package healthcheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"time"

	// k8s

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformersv1 "k8s.io/client-go/informers/core/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"

	// openshift
	operatorsv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	v1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	routeclientv1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	routesinformersv1 "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	// console-operator
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
)

type HealthCheckController struct {
	// clients
	operatorClient       v1helpers.OperatorClient
	operatorConfigClient operatorclientv1.ConsoleInterface
	ingressClient        configclientv1.IngressInterface
	routeClient          routeclientv1.RoutesGetter
	configMapClient      coreclientv1.ConfigMapsGetter
}

func NewHealthCheckController(
	// top level config
	configClient configclientv1.ConfigV1Interface,
	// clients
	operatorClient v1helpers.OperatorClient,
	operatorConfigClient operatorclientv1.ConsoleInterface,
	routev1Client routeclientv1.RoutesGetter,
	configMapClient coreclientv1.ConfigMapsGetter,
	// informers
	operatorConfigInformer v1.ConsoleInformer,
	coreInformer coreinformersv1.Interface,
	routeInformer routesinformersv1.RouteInformer,
	// events
	recorder events.Recorder,
) factory.Controller {
	ctrl := &HealthCheckController{
		operatorClient:       operatorClient,
		operatorConfigClient: operatorConfigClient,
		ingressClient:        configClient.Ingresses(),
		routeClient:          routev1Client,
		configMapClient:      configMapClient,
	}

	configMapInformer := coreInformer.ConfigMaps()

	return factory.New().
		WithFilteredEventsInformers( // service
			util.NamesFilter(api.TrustedCAConfigMapName, api.DefaultIngressCertConfigMapName),
			configMapInformer.Informer(),
		).WithFilteredEventsInformers( // route
		util.NamesFilter(api.OpenShiftConsoleRouteName, api.OpenshiftConsoleCustomRouteName),
		routeInformer.Informer(),
	).ResyncEvery(time.Minute).WithSync(ctrl.Sync).
		ToController("HealthCheckController", recorder.WithComponentSuffix("health-check-controller"))
}

func (c *HealthCheckController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.operatorConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	updatedOperatorConfig := operatorConfig.DeepCopy()

	switch updatedOperatorConfig.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infoln("console-operator is in a managed state: starting health checks")
	case operatorsv1.Unmanaged:
		klog.V(4).Infoln("console-operator is in an unmanaged state: skipping health checks")
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infoln("console-operator is in a removed state: skipping health checks")
		return nil
	default:
		return fmt.Errorf("unknown state: %v", updatedOperatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	ingressConfig, err := c.ingressClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return statusHandler.FlushAndReturn(err)
	}

	activeRouteName := api.OpenShiftConsoleRouteName
	routeConfig := routesub.NewRouteConfig(updatedOperatorConfig, ingressConfig, activeRouteName)
	if routeConfig.IsCustomHostnameSet() {
		activeRouteName = api.OpenshiftConsoleCustomRouteName
	}

	activeRoute, activeRouteErr := c.routeClient.Routes(api.OpenShiftConsoleNamespace).Get(ctx, activeRouteName, metav1.GetOptions{})
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("RouteHealth", "FailedRouteGet", activeRouteErr))
	if activeRouteErr != nil {
		statusHandler.FlushAndReturn(activeRouteErr)
	}

	routeHealthCheckErrReason, routeHealthCheckErr := c.CheckRouteHealth(ctx, updatedOperatorConfig, activeRoute)
	statusHandler.AddCondition(status.HandleDegraded("RouteHealth", routeHealthCheckErrReason, routeHealthCheckErr))
	statusHandler.AddCondition(status.HandleAvailable("RouteHealth", routeHealthCheckErrReason, routeHealthCheckErr))

	return statusHandler.FlushAndReturn(routeHealthCheckErr)
}

func (c *HealthCheckController) CheckRouteHealth(ctx context.Context, operatorConfig *operatorsv1.Console, route *routev1.Route) (string, error) {
	if !routesub.IsAdmitted(route) {
		return "RouteNotAdmitted", fmt.Errorf("console route is not admitted")
	}

	caPool, err := c.getCA(ctx, route.Spec.TLS)
	if err != nil {
		return "FailedLoadCA", fmt.Errorf("failed to read CA to check route health: %v", err)
	}
	client := clientWithCA(caPool)

	if len(route.Spec.Host) == 0 {
		return "RouteHostError", fmt.Errorf("route does not have host specified")
	}
	url := "https://" + route.Spec.Host + "/health"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "FailedRequest", fmt.Errorf("failed to build request to route (%s): %v", url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "FailedGet", fmt.Errorf("failed to GET route (%s): %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "StatusError", fmt.Errorf("route not yet available, %s returns '%s'", url, resp.Status)
	}

	return "", nil
}

func (c *HealthCheckController) getCA(ctx context.Context, tls *routev1.TLSConfig) (*x509.CertPool, error) {
	caCertPool := x509.NewCertPool()

	if tls != nil && len(tls.Certificate) != 0 {
		if ok := caCertPool.AppendCertsFromPEM([]byte(tls.Certificate)); !ok {
			klog.V(4).Infof("failed to parse custom tls.crt")
		}
	}

	for _, cmName := range []string{api.TrustedCAConfigMapName, api.DefaultIngressCertConfigMapName} {
		cm, err := c.configMapClient.ConfigMaps(api.OpenShiftConsoleNamespace).Get(ctx, cmName, metav1.GetOptions{})
		if err != nil {
			klog.V(4).Infof("failed to GET configmap %s / %s ", api.OpenShiftConsoleNamespace, cmName)
			return nil, err
		}
		if ok := caCertPool.AppendCertsFromPEM([]byte(cm.Data["ca-bundle.crt"])); !ok {
			klog.V(4).Infof("failed to parse %s ca-bundle.crt", cmName)
		}
	}

	return caCertPool, nil
}

func clientWithCA(caPool *x509.CertPool) *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{
				RootCAs: caPool,
			},
		},
	}
}
