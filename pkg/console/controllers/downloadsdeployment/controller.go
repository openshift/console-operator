package downloadsdeployment

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsinformersv1 "k8s.io/client-go/informers/apps/v1"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	operatorinformerv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	operatorlistersv1 "github.com/openshift/client-go/operator/listers/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/status"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/openshift/console-operator/pkg/console/controllers/util"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
)

type DownloadsDeploymentSyncController struct {
	operatorClient v1helpers.OperatorClient
	// configs
	consoleOperatorLister operatorlistersv1.ConsoleLister
	infrastructureLister  configlistersv1.InfrastructureLister
	// core kube
	deploymentClient appsclientv1.DeploymentsGetter
}

func NewDownloadsDeploymentSyncController(
	// clients
	operatorClient v1helpers.OperatorClient,
	// informer
	configInformer configinformer.SharedInformerFactory,
	operatorConfigInformer operatorinformerv1.ConsoleInformer,
	// core kube
	deploymentClient appsclientv1.DeploymentsGetter,
	deploymentInformer appsinformersv1.DeploymentInformer,
	// events
	recorder events.Recorder,
) factory.Controller {
	configV1Informers := configInformer.Config().V1()

	ctrl := &DownloadsDeploymentSyncController{
		// configs
		operatorClient:        operatorClient,
		consoleOperatorLister: operatorConfigInformer.Lister(),
		infrastructureLister:  configInformer.Config().V1().Infrastructures().Lister(),
		// client
		deploymentClient: deploymentClient,
	}

	configNameFilter := util.IncludeNamesFilter(api.ConfigResourceName)
	downloadsNameFilter := util.IncludeNamesFilter(api.OpenShiftConsoleDownloadsDeploymentName)

	return factory.New().
		WithFilteredEventsInformers( // infrastructure configs
			configNameFilter,
			operatorConfigInformer.Informer(),
			configV1Informers.Infrastructures().Informer(),
		).WithFilteredEventsInformers( // downloads deployment
		downloadsNameFilter,
		deploymentInformer.Informer(),
	).ResyncEvery(time.Minute).WithSync(ctrl.Sync).
		ToController("ConsoleDownloadsDeploymentSyncController", recorder.WithComponentSuffix("console-downloads-deployment-controller"))
}

func (c *DownloadsDeploymentSyncController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.consoleOperatorLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}
	operatorConfigCopy := operatorConfig.DeepCopy()

	switch operatorConfigCopy.Spec.ManagementState {
	case operatorv1.Managed:
		klog.V(4).Infoln("console is in a managed state: syncing downloads deployment")
	case operatorv1.Unmanaged:
		klog.V(4).Infoln("console is in an unmanaged state: skipping downloads deployment sync")
		return nil
	case operatorv1.Removed:
		klog.V(4).Infoln("console is in an removed state: removing synced downloads deployment")
		return c.removeDownloadsDeployment(ctx)
	default:
		return fmt.Errorf("unknown state: %v", operatorConfigCopy.Spec.ManagementState)
	}
	statusHandler := status.NewStatusHandler(c.operatorClient)

	infrastructureConfig, err := c.infrastructureLister.Get(api.ConfigResourceName)
	statusHandler.AddCondition(status.HandleDegraded("DownloadsDeploymentSync", "FailedInfrastructureConfigGet", err))
	if err != nil {
		return statusHandler.FlushAndReturn(err)
	}

	actualDownloadsDownloadsDeployment, _, downloadsDeploymentErr := c.SyncDownloadsDeployment(ctx, operatorConfigCopy, infrastructureConfig, controllerContext)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("DownloadsDeploymentSync", "FailedApply", downloadsDeploymentErr))
	if downloadsDeploymentErr != nil {
		return statusHandler.FlushAndReturn(downloadsDeploymentErr)
	}
	statusHandler.UpdateDeploymentGeneration(actualDownloadsDownloadsDeployment)

	return statusHandler.FlushAndReturn(nil)
}

func (c *DownloadsDeploymentSyncController) SyncDownloadsDeployment(ctx context.Context, operatorConfigCopy *operatorv1.Console, infrastructureConfig *configv1.Infrastructure, controllerContext factory.SyncContext) (*appsv1.Deployment, bool, error) {

	requiredDownloadsDeployment := deploymentsub.DefaultDownloadsDeployment(operatorConfigCopy, infrastructureConfig)

	return resourceapply.ApplyDeployment(ctx,
		c.deploymentClient,
		controllerContext.Recorder(),
		requiredDownloadsDeployment,
		resourcemerge.ExpectedDeploymentGeneration(requiredDownloadsDeployment, operatorConfigCopy.Status.Generations),
	)
}

func (c *DownloadsDeploymentSyncController) removeDownloadsDeployment(ctx context.Context) error {
	err := c.deploymentClient.Deployments(api.OpenShiftConsoleNamespace).Delete(ctx, api.OpenShiftConsoleDownloadsDeploymentName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
