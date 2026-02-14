package downloadsserviceaccount

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformersv1 "k8s.io/client-go/informers/core/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"

	operatorv1 "github.com/openshift/api/operator/v1"
	operatorinformerv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	operatorlistersv1 "github.com/openshift/client-go/operator/listers/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	serviceaccountsub "github.com/openshift/console-operator/pkg/console/subresource/serviceaccount"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

type DownloadsServiceAccountSyncController struct {
	operatorClient v1helpers.OperatorClient
	// configs
	consoleOperatorLister operatorlistersv1.ConsoleLister
	// core kube
	serviceAccountClient coreclientv1.ServiceAccountsGetter
}

func NewDownloadsServiceAccountSyncController(
	// clients
	operatorClient v1helpers.OperatorClient,
	// informer
	operatorConfigInformer operatorinformerv1.ConsoleInformer,
	// core kube
	serviceAccountClient coreclientv1.ServiceAccountsGetter,
	serviceAccountInformer coreinformersv1.ServiceAccountInformer,
	// events
	recorder events.Recorder,
) factory.Controller {
	ctrl := &DownloadsServiceAccountSyncController{
		// configs
		operatorClient:        operatorClient,
		consoleOperatorLister: operatorConfigInformer.Lister(),
		// client
		serviceAccountClient: serviceAccountClient,
	}

	configNameFilter := util.IncludeNamesFilter(api.ConfigResourceName)
	downloadsNameFilter := util.IncludeNamesFilter(api.DownloadsResourceName)

	return factory.New().
		WithFilteredEventsInformers( // configs
			configNameFilter,
			operatorConfigInformer.Informer(),
		).WithFilteredEventsInformers( // downloads service account
		downloadsNameFilter,
		serviceAccountInformer.Informer(),
	).ResyncEvery(time.Minute).WithSync(ctrl.Sync).
		ToController("ConsoleDownloadsServiceAccountSyncController", recorder.WithComponentSuffix("console-downloads-service-account-controller"))
}

func (c *DownloadsServiceAccountSyncController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.consoleOperatorLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}
	operatorConfigCopy := operatorConfig.DeepCopy()

	switch operatorConfigCopy.Spec.ManagementState {
	case operatorv1.Managed:
		klog.V(4).Infoln("console is in a managed state: syncing downloads service account")
	case operatorv1.Unmanaged:
		klog.V(4).Infoln("console is in an unmanaged state: skipping downloads service account sync")
		return nil
	case operatorv1.Removed:
		klog.V(4).Infoln("console is in a removed state: removing downloads service account")
		return c.removeDownloadsServiceAccount(ctx)
	default:
		return fmt.Errorf("unknown state: %v", operatorConfigCopy.Spec.ManagementState)
	}
	statusHandler := status.NewStatusHandler(c.operatorClient)

	_, _, serviceAccountErr := c.SyncDownloadsServiceAccount(ctx, operatorConfigCopy, controllerContext)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("DownloadsServiceAccountSync", "FailedApply", serviceAccountErr))
	if serviceAccountErr != nil {
		return statusHandler.FlushAndReturn(serviceAccountErr)
	}

	return statusHandler.FlushAndReturn(nil)
}

func (c *DownloadsServiceAccountSyncController) SyncDownloadsServiceAccount(ctx context.Context, operatorConfigCopy *operatorv1.Console, controllerContext factory.SyncContext) (*corev1.ServiceAccount, bool, error) {
	requiredDownloadsServiceAccount := serviceaccountsub.DefaultDownloadsServiceAccount(operatorConfigCopy)

	return resourceapply.ApplyServiceAccount(ctx,
		c.serviceAccountClient,
		controllerContext.Recorder(),
		requiredDownloadsServiceAccount,
	)
}

func (c *DownloadsServiceAccountSyncController) removeDownloadsServiceAccount(ctx context.Context) error {
	err := c.serviceAccountClient.ServiceAccounts(api.OpenShiftConsoleNamespace).Delete(ctx, api.DownloadsResourceName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
