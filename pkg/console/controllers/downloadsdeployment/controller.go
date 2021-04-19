package downloadsdeployment

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsinformersv1 "k8s.io/client-go/informers/apps/v1"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	operatorinformerv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/status"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/openshift/console-operator/pkg/console/controllers/util"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
)

type DownloadsDeploymentSyncController struct {
	operatorClient v1helpers.OperatorClient
	// configs
	operatorConfigClient       operatorclientv1.ConsoleInterface
	infrastructureConfigClient configclientv1.InfrastructureInterface
	// core kube
	deploymentClient appsclientv1.DeploymentsGetter
	// events
	resourceSyncer resourcesynccontroller.ResourceSyncer
}

func NewDownloadsDeploymentSyncController(
	// top level config
	configClient configclientv1.ConfigV1Interface,
	// clients
	operatorClient v1helpers.OperatorClient,
	operatorConfigClient operatorclientv1.ConsoleInterface,
	// informer
	configInformer configinformer.SharedInformerFactory,
	operatorConfigInformer operatorinformerv1.ConsoleInformer,
	// core kube
	deploymentClient appsclientv1.DeploymentsGetter,
	deploymentInformer appsinformersv1.DeploymentInformer,
	// events
	recorder events.Recorder,
	resourceSyncer resourcesynccontroller.ResourceSyncer,
) factory.Controller {
	configV1Informers := configInformer.Config().V1()

	ctrl := &DownloadsDeploymentSyncController{
		// configs
		operatorClient:             operatorClient,
		operatorConfigClient:       operatorConfigClient,
		infrastructureConfigClient: configClient.Infrastructures(),
		// client
		deploymentClient: deploymentClient,
		// events
		resourceSyncer: resourceSyncer,
	}

	configNameFilter := util.NamesFilter(api.ConfigResourceName)
	downloadsNameFilter := util.NamesFilter(api.OpenShiftConsoleDownloadsDeploymentName)

	return factory.New().
		WithFilteredEventsInformers( // infrastructure configs
			configNameFilter,
			operatorConfigInformer.Informer(),
			configV1Informers.Infrastructures().Informer(),
		).WithFilteredEventsInformers( // downloads deployment
		downloadsNameFilter,
		deploymentInformer.Informer(),
	).WithSync(ctrl.Sync).
		ToController("ConsoleDownloadsDeploymentSyncController", recorder.WithComponentSuffix("console-downloads-deployment-controller"))
}

func (c *DownloadsDeploymentSyncController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.operatorConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	updatedOperatorConfig := operatorConfig.DeepCopy()

	switch updatedOperatorConfig.Spec.ManagementState {
	case operatorv1.Managed:
		klog.V(4).Infoln("console is in a managed state: syncing downloads deployment")
	case operatorv1.Unmanaged:
		klog.V(4).Infoln("console is in an unmanaged state: skipping downloads deployment sync")
		return nil
	case operatorv1.Removed:
		klog.V(4).Infoln("console is in an removed state: removing synced downloads deployment")
		return c.removeDownloadsDeployment(ctx)
	default:
		return fmt.Errorf("unknown state: %v", updatedOperatorConfig.Spec.ManagementState)
	}
	statusHandler := status.NewStatusHandler(c.operatorClient)

	infrastructureConfig, err := c.infrastructureConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	statusHandler.AddCondition(status.HandleDegraded("DonwloadsDeploymentSync", "FailedInfrastructureConfigGet", err))
	if err != nil {
		return statusHandler.FlushAndReturn(err)
	}

	actualDownloadsDownloadsDeployment, _, downloadsDeploymentErr := c.SyncDownloadsDeployment(ctx, updatedOperatorConfig, infrastructureConfig, controllerContext)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("DownloadsDeploymentSync", "FailedApply", downloadsDeploymentErr))
	if downloadsDeploymentErr != nil {
		return statusHandler.FlushAndReturn(downloadsDeploymentErr)
	}
	statusHandler.UpdateDeploymentGeneration(actualDownloadsDownloadsDeployment)

	return statusHandler.FlushAndReturn(nil)
}

func (c *DownloadsDeploymentSyncController) SyncDownloadsDeployment(ctx context.Context, operatorConfig *operatorv1.Console, infrastructureConfig *configv1.Infrastructure, controllerContext factory.SyncContext) (*appsv1.Deployment, bool, error) {

	updatedOperatorConfig := operatorConfig.DeepCopy()
	requiredDownloadsDeployment := deploymentsub.DefaultDownloadsDeployment(updatedOperatorConfig, infrastructureConfig)

	return resourceapply.ApplyDeployment(
		c.deploymentClient,
		controllerContext.Recorder(),
		requiredDownloadsDeployment,
		resourcemerge.ExpectedDeploymentGeneration(requiredDownloadsDeployment, updatedOperatorConfig.Status.Generations),
	)
}

func (c *DownloadsDeploymentSyncController) removeDownloadsDeployment(ctx context.Context) error {
	return c.deploymentClient.Deployments(api.OpenShiftConsoleNamespace).Delete(ctx, api.OpenShiftConsoleDownloadsDeploymentName, metav1.DeleteOptions{})
}
