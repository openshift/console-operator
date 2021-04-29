package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformersv1 "k8s.io/client-go/informers/core/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"

	operatorsv1 "github.com/openshift/api/operator/v1"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	operatorinformersv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/assets"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

// ctrl just needs the clients so it can make requests
// the informers will automatically notify it of changes
// and kick the sync loop
type ServiceSyncController struct {
	serviceName          string
	operatorClient       v1helpers.OperatorClient
	operatorConfigClient operatorclientv1.ConsoleInterface
	ingressClient        configclientv1.IngressInterface
	// live clients, we dont need listers w/caches
	serviceClient coreclientv1.ServicesGetter
	// events
	resourceSyncer resourcesynccontroller.ResourceSyncer
}

// factory func needs clients and informers
// informers to start them up, clients to pass
func NewServiceSyncController(
	serviceName string,
	// top level config
	configClient configclientv1.ConfigV1Interface,
	configInformer configinformer.SharedInformerFactory,
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
		serviceName:          serviceName,
		operatorClient:       operatorClient,
		operatorConfigClient: operatorConfigClient,
		ingressClient:        configClient.Ingresses(),
		serviceClient:        corev1Client,
		resourceSyncer:       resourceSyncer,
	}

	configV1Informers := configInformer.Config().V1()

	return factory.New().
		WithFilteredEventsInformers( // configs
			util.NamesFilter(api.ConfigResourceName),
			operatorConfigInformer.Informer(),
			configV1Informers.Ingresses().Informer(),
		).WithFilteredEventsInformers( // console resources
		util.NamesFilter(serviceName, ctrl.getRedirectServiceName()),
		serviceInformer.Informer(),
	).ResyncEvery(time.Minute).WithSync(ctrl.Sync).
		ToController(fmt.Sprintf("%sServiceController", strings.Title(serviceName)), recorder.WithComponentSuffix(fmt.Sprintf("%s-service-controller", serviceName)))
}

func (c *ServiceSyncController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.operatorConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	updatedOperatorConfig := operatorConfig.DeepCopy()

	switch updatedOperatorConfig.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infof("console-operator is in a managed state: syncing %q service", c.serviceName)
	case operatorsv1.Unmanaged:
		klog.V(4).Infof("console-operator is in an unmanaged state: skipping service %q sync", c.serviceName)
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infof("console-operator is in a removed state: deleting %q service", c.serviceName)
		if err = c.removeService(ctx, c.getRedirectServiceName()); err != nil {
			return err
		}
		return c.removeService(ctx, c.serviceName)
	default:
		return fmt.Errorf("unknown state: %v", updatedOperatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	ingressConfig, err := c.ingressClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return statusHandler.FlushAndReturn(err)
	}
	// Service name matches the Route's so it can be used as well, for creating RouteConfig
	routeConfig := routesub.NewRouteConfig(updatedOperatorConfig, ingressConfig, c.serviceName)

	requiredSvc := c.getDefaultService()
	_, _, svcErr := resourceapply.ApplyService(c.serviceClient, controllerContext.Recorder(), requiredSvc)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ServiceSync", "FailedApply", svcErr))
	if svcErr != nil {
		return statusHandler.FlushAndReturn(svcErr)
	}

	// we are only creating redirect service for the `console` route
	if c.serviceName == api.OpenShiftConsoleServiceName {
		redirectSvcErrReason, svcErr := c.SyncRedirectService(ctx, routeConfig, controllerContext)
		statusHandler.AddConditions(status.HandleProgressingOrDegraded("RedirectServiceSync", redirectSvcErrReason, svcErr))
	}

	return statusHandler.FlushAndReturn(svcErr)
}

func (c *ServiceSyncController) SyncRedirectService(ctx context.Context, routeConfig *routesub.RouteConfig, controllerContext factory.SyncContext) (string, error) {
	if !routeConfig.IsCustomHostnameSet() {
		if err := c.removeService(ctx, c.getRedirectServiceName()); err != nil {
			return "FailedDelete", err
		}
		return "", nil
	}
	requiredRedirectService := c.getRedirectService()
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

func (c *ServiceSyncController) getDefaultService() *corev1.Service {
	service := resourceread.ReadServiceV1OrDie(assets.MustAsset(fmt.Sprintf("services/%s-service.yaml", c.serviceName)))

	return service
}

func (c *ServiceSyncController) getRedirectService() *corev1.Service {
	service := resourceread.ReadServiceV1OrDie(assets.MustAsset(fmt.Sprintf("services/%s-redirect-service.yaml", c.serviceName)))

	return service
}

func (c *ServiceSyncController) getRedirectServiceName() string {
	return fmt.Sprintf("%s-redirect", c.serviceName)
}
