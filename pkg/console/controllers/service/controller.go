package service

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformersv1 "k8s.io/client-go/informers/core/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"

	operatorsv1 "github.com/openshift/api/operator/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	operatorinformersv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	"github.com/openshift/console-operator/pkg/console/subresource/service"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

// ctrl just needs the clients so it can make requests
// the informers will automatically notify it of changes
// and kick the sync loop
type ServiceSyncController struct {
	operatorClient       v1helpers.OperatorClient
	operatorConfigClient operatorclientv1.ConsoleInterface
	// live clients, we dont need listers w/caches
	serviceClient coreclientv1.ServicesGetter
	// events
	resourceSyncer resourcesynccontroller.ResourceSyncer
}

// factory func needs clients and informers
// informers to start them up, clients to pass
func NewServiceSyncController(
	// clients
	operatorClient v1helpers.OperatorClient,
	operatorConfigClient operatorclientv1.ConsoleInterface,
	corev1Client coreclientv1.CoreV1Interface,
	// informers
	operatorConfigInformer operatorinformersv1.ConsoleInformer,
	serviceInformer coreinformersv1.ServiceInformer,
	// events
	recorder events.Recorder,
	resourceSyncer resourcesynccontroller.ResourceSyncer,
) factory.Controller {

	ctrl := &ServiceSyncController{
		operatorClient:       operatorClient,
		operatorConfigClient: operatorConfigClient,
		serviceClient:        corev1Client,
		resourceSyncer:       resourceSyncer,
	}

	return factory.New().
		WithFilteredEventsInformers( // configs
			util.NamesFilter(api.ConfigResourceName),
			operatorConfigInformer.Informer(),
		).WithFilteredEventsInformers( // console resources
		util.NamesFilter(api.OpenShiftConsoleServiceName, api.OpenshiftConsoleRedirectServiceName),
		serviceInformer.Informer(),
	).WithSync(ctrl.Sync).
		ToController("ConsoleServiceController", recorder.WithComponentSuffix("console-service-controller"))
}

func (c *ServiceSyncController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.operatorConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	updatedOperatorConfig := operatorConfig.DeepCopy()

	switch updatedOperatorConfig.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infoln("console is in a managed state: syncing service")
	case operatorsv1.Unmanaged:
		klog.V(4).Infoln("console is in an unmanaged state: skipping service sync")
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infoln("console is in a removed state: deleting service")
		if err = c.removeService(ctx, api.OpenshiftConsoleRedirectServiceName); err != nil {
			return err
		}
		return c.removeService(ctx, api.OpenShiftConsoleServiceName)
	default:
		return fmt.Errorf("unknown state: %v", updatedOperatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	requiredSvc := service.DefaultService(updatedOperatorConfig)
	_, _, svcErr := resourceapply.ApplyService(c.serviceClient, controllerContext.Recorder(), requiredSvc)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ServiceSync", "FailedApply", svcErr))
	if svcErr != nil {
		return statusHandler.FlushAndReturn(svcErr)
	}

	redirectSvcErrReason, redirectSvcErr := c.SyncRedirectService(ctx, updatedOperatorConfig, controllerContext)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("RedirectServiceSync", redirectSvcErrReason, redirectSvcErr))

	return statusHandler.FlushAndReturn(redirectSvcErr)
}

func (c *ServiceSyncController) SyncRedirectService(ctx context.Context, operatorConfig *operatorsv1.Console, controllerContext factory.SyncContext) (string, error) {
	if !routesub.IsCustomRouteSet(operatorConfig) {
		if err := c.removeService(ctx, api.OpenshiftConsoleRedirectServiceName); err != nil {
			return "FailedDelete", err
		}
		return "", nil
	}
	requiredRedirectService := service.RedirectService(operatorConfig)
	_, _, redirectSvcErr := resourceapply.ApplyService(c.serviceClient, controllerContext.Recorder(), requiredRedirectService)
	if redirectSvcErr != nil {
		return "FailedApply", redirectSvcErr
	}
	return "", redirectSvcErr
}

func (c *ServiceSyncController) removeService(ctx context.Context, serviceName string) error {
	err := c.serviceClient.Services(api.OpenShiftConsoleNamespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
