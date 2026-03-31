package serviceaccounts

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	operatorv1 "github.com/openshift/api/operator/v1"
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	operatorinformerv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	operatorlistersv1 "github.com/openshift/client-go/operator/listers/operator/v1"

	"github.com/openshift/console-operator/bindata"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	subresourceutil "github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformersv1 "k8s.io/client-go/informers/core/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/klog/v2"
)

type ServiceAccountSyncController struct {
	serviceAccountName string
	conditionType      string
	operatorClient     v1helpers.OperatorClient
	// configs
	consoleOperatorLister operatorlistersv1.ConsoleLister
	infrastructureLister  configlistersv1.InfrastructureLister
	// core kube
	serviceAccountClient coreclientv1.ServiceAccountsGetter
}

func NewServiceAccountSyncController(
	// clients
	operatorClient v1helpers.OperatorClient,
	// informer
	configInformer configinformer.SharedInformerFactory,
	operatorConfigInformer operatorinformerv1.ConsoleInformer,
	// core kube
	serviceAccountClient coreclientv1.ServiceAccountsGetter,
	serviceAccountInformer coreinformersv1.ServiceAccountInformer,
	// events
	recorder events.Recorder,
	// serviceAccountName
	serviceAccountName string,
	// controllerName,
	controllerName string,
) factory.Controller {
	configV1Informers := configInformer.Config().V1()

	ctrl := &ServiceAccountSyncController{
		serviceAccountName: serviceAccountName,
		conditionType:      fmt.Sprintf("%sServiceAccountSync", controllerName),
		// configs
		operatorClient:        operatorClient,
		consoleOperatorLister: operatorConfigInformer.Lister(),
		infrastructureLister:  configInformer.Config().V1().Infrastructures().Lister(),
		// clients
		serviceAccountClient: serviceAccountClient,
	}

	configNameFilter := util.IncludeNamesFilter(api.ConfigResourceName)
	serviceAccountNameFilter := util.IncludeNamesFilter(serviceAccountName)

	return factory.New().
		WithFilteredEventsInformers( // infrastructure configs
			configNameFilter,
			operatorConfigInformer.Informer(),
			configV1Informers.Infrastructures().Informer(),
		).WithFilteredEventsInformers( // service account
		serviceAccountNameFilter,
		serviceAccountInformer.Informer(),
	).ResyncEvery(time.Minute).WithSync(ctrl.Sync).
		ToController(fmt.Sprintf("%sServiceAccountController", strings.Title(controllerName)), recorder.WithComponentSuffix(fmt.Sprintf("%s-service-account-controller", controllerName)))
}

func (c *ServiceAccountSyncController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.consoleOperatorLister.Get(api.ConfigResourceName)
	if err != nil {
		return fmt.Errorf("failed to get console operator config %s: %w", api.ConfigResourceName, err)
	}
	operatorConfigCopy := operatorConfig.DeepCopy()

	switch operatorConfigCopy.Spec.ManagementState {
	case operatorv1.Managed:
		klog.V(4).Infoln("console is in a managed state: syncing serviceaccount")
	case operatorv1.Unmanaged:
		klog.V(4).Infoln("console is in an unmanaged state: skipping serviceaccount sync")
		return nil
	case operatorv1.Removed:
		klog.V(4).Infoln("console is in a removed state: removing synced serviceaccount")
		return c.removeServiceAccount(ctx)
	default:
		return fmt.Errorf("unknown state: %v", operatorConfigCopy.Spec.ManagementState)
	}
	statusHandler := status.NewStatusHandler(c.operatorClient)

	serviceAccountErr := c.SyncServiceAccount(ctx, operatorConfigCopy, controllerContext)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded(c.conditionType, "FailedApply", serviceAccountErr))
	if serviceAccountErr != nil {
		return statusHandler.FlushAndReturn(serviceAccountErr)
	}

	return statusHandler.FlushAndReturn(nil)
}

func (c *ServiceAccountSyncController) removeServiceAccount(ctx context.Context) error {
	err := c.serviceAccountClient.ServiceAccounts(api.OpenShiftConsoleNamespace).Delete(ctx, c.serviceAccountName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *ServiceAccountSyncController) SyncServiceAccount(ctx context.Context, operatorConfigCopy *operatorv1.Console, controllerContext factory.SyncContext) error {
	serviceAccount, err := c.DefaultServiceAccount(operatorConfigCopy)

	if err != nil {
		return err
	}

	// check for service account existence

	existingServiceAccount, err := c.serviceAccountClient.ServiceAccounts(serviceAccount.Namespace).Get(ctx, serviceAccount.Name, metav1.GetOptions{})

	if err == nil {
		for _, oR := range existingServiceAccount.OwnerReferences {
			// mark ownerref for deletion. we cannot have multiple owner refs
			// https://github.com/openshift/library-go/blob/master/pkg/operator/resource/resourcemerge/object_merger.go#L214-L219
			if reflect.DeepEqual(oR, *subresourceutil.OwnerRefFrom(operatorConfigCopy)) {
				continue // we want to keep this controller=true
			}
			removalRef := oR.DeepCopy()
			removalRef.UID = removalRef.UID + "-"
			serviceAccount.OwnerReferences = append(serviceAccount.OwnerReferences, *removalRef)
		}
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get existing service account %s: %w", c.serviceAccountName, err)
	}

	_, _, err = resourceapply.ApplyServiceAccount(ctx,
		c.serviceAccountClient,
		controllerContext.Recorder(),
		serviceAccount,
	)

	if err != nil {
		return fmt.Errorf("failed to apply service account %s: %w", c.serviceAccountName, err)
	}

	return nil
}

func (c *ServiceAccountSyncController) DefaultServiceAccount(cr *operatorv1.Console) (*corev1.ServiceAccount, error) {
	serviceAccount := resourceread.ReadServiceAccountV1OrDie(
		bindata.MustAsset(fmt.Sprintf("assets/serviceaccounts/%s-sa.yaml", c.serviceAccountName)),
	)
	if serviceAccount.Name == "" {
		return nil, fmt.Errorf("No service account found for name %v .", c.serviceAccountName)
	}
	serviceAccount.SetOwnerReferences([]metav1.OwnerReference{*subresourceutil.OwnerRefFrom(cr)})
	return serviceAccount, nil
}
