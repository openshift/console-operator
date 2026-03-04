package serviceaccounts

import (
	"context"
	"fmt"
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
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

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
		statusHandler := status.NewStatusHandler(c.operatorClient)
		removeErr := c.removeServiceAccount(ctx)
		statusHandler.AddConditions(status.HandleProgressingOrDegraded(c.conditionType, "FailedRemove", removeErr))
		return statusHandler.FlushAndReturn(removeErr)
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
	serviceAccount := resourceread.ReadServiceAccountV1OrDie(
		bindata.MustAsset(fmt.Sprintf("assets/serviceaccounts/%s-sa.yaml", c.serviceAccountName)),
	)
	if serviceAccount.Name == "" {
		return fmt.Errorf("No service account found for name %v .", c.serviceAccountName)
	}

	// Fetch the live SA to get its actual ownerRefs (e.g. ClusterVersion/version
	// set by CVO from the old manifests/06-sa.yaml, or any previously set
	// Console/cluster ref). We need these on the *required* object so that
	// MergeOwnerRefs will update them — it only modifies refs that appear in
	// required (matched by Name+Kind+APIVersion.Group).
	existing, err := c.serviceAccountClient.ServiceAccounts(api.OpenShiftConsoleNamespace).Get(ctx, c.serviceAccountName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get service account %s: %w", c.serviceAccountName, err)
	}
	if err == nil {
		// Clear controller:true from all existing ownerRefs to satisfy the
		// Kubernetes invariant that only one ownerRef may have controller:true.
		// https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
		falseBool := false
		ownerRefs := existing.DeepCopy().GetOwnerReferences()
		for i := range ownerRefs {
			ownerRefs[i].Controller = &falseBool
		}
		serviceAccount.SetOwnerReferences(ownerRefs)
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
