package healthcheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"time"

	// k8s

	coreinformersv1 "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	v1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	operatorv1listers "github.com/openshift/client-go/operator/listers/operator/v1"
	routesinformersv1 "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	routev1listers "github.com/openshift/client-go/route/listers/route/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	"github.com/openshift/library-go/pkg/route/routeapihelpers"

	// console-operator
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
)

type HealthCheckController struct {
	// clients
	operatorClient             v1helpers.OperatorClient
	infrastructureConfigLister configlistersv1.InfrastructureLister
	configMapLister            corev1listers.ConfigMapLister
	routeLister                routev1listers.RouteLister
	ingressConfigLister        configlistersv1.IngressLister
	operatorConfigLister       operatorv1listers.ConsoleLister
}

func NewHealthCheckController(
	// top level config
	configClient configclientv1.ConfigV1Interface,
	// clients
	operatorClient v1helpers.OperatorClient,
	// informers
	operatorConfigInformer v1.ConsoleInformer,
	configInformer configinformer.SharedInformerFactory,
	coreInformer coreinformersv1.Interface,
	routeInformer routesinformersv1.RouteInformer,
	// events
	recorder events.Recorder,
) factory.Controller {
	ctrl := &HealthCheckController{
		operatorClient:             operatorClient,
		operatorConfigLister:       operatorConfigInformer.Lister(),
		infrastructureConfigLister: configInformer.Config().V1().Infrastructures().Lister(),
		ingressConfigLister:        configInformer.Config().V1().Ingresses().Lister(),
		routeLister:                routeInformer.Lister(),
		configMapLister:            coreInformer.ConfigMaps().Lister(),
	}

	configMapInformer := coreInformer.ConfigMaps()
	configV1Informers := configInformer.Config().V1()

	return factory.New().
		WithFilteredEventsInformers( // configs
			util.IncludeNamesFilter(api.ConfigResourceName),
			operatorConfigInformer.Informer(),
			configV1Informers.Ingresses().Informer(),
		).WithFilteredEventsInformers( // service
		util.IncludeNamesFilter(api.TrustedCAConfigMapName, api.OAuthServingCertConfigMapName),
		configMapInformer.Informer(),
	).WithFilteredEventsInformers( // route
		util.IncludeNamesFilter(api.OpenShiftConsoleRouteName, api.OpenshiftConsoleCustomRouteName),
		routeInformer.Informer(),
	).ResyncEvery(30*time.Second).WithSync(ctrl.Sync).
		ToController("HealthCheckController", recorder.WithComponentSuffix("health-check-controller"))
}

func (c *HealthCheckController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	statusHandler := status.NewStatusHandler(c.operatorClient)
	operatorConfig, err := c.operatorConfigLister.Get(api.ConfigResourceName)
	if err != nil {
		klog.Errorf("operator config error: %v", err)
		return statusHandler.FlushAndReturn(err)
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
	ingressConfig, err := c.ingressConfigLister.Get(api.ConfigResourceName)
	if err != nil {
		klog.Errorf("ingress config error: %v", err)
		return statusHandler.FlushAndReturn(err)
	}
	infrastructureConfig, err := c.infrastructureConfigLister.Get(api.ConfigResourceName)
	if err != nil {
		klog.Errorf("infrastructure config error: %v", err)
		return statusHandler.FlushAndReturn(err)
	}

	// Disable the health check for external control plane topology (hypershift) and ingress NLB.
	// This is to avoid an issue with internal NLB see https://issues.redhat.com/browse/OCPBUGS-23300
	if isExternalControlPlaneWithNLB(infrastructureConfig, ingressConfig) {
		return nil
	}

	activeRouteName := api.OpenShiftConsoleRouteName
	routeConfig := routesub.NewRouteConfig(updatedOperatorConfig, ingressConfig, activeRouteName)
	if routeConfig.IsCustomHostnameSet() {
		activeRouteName = api.OpenshiftConsoleCustomRouteName
	}

	activeRoute, activeRouteErr := c.routeLister.Routes(api.OpenShiftConsoleNamespace).Get(activeRouteName)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("RouteHealth", "FailedRouteGet", activeRouteErr))
	if activeRouteErr != nil {
		return statusHandler.FlushAndReturn(activeRouteErr)
	}

	routeHealthCheckErrReason, routeHealthCheckErr := c.CheckRouteHealth(ctx, updatedOperatorConfig, activeRoute)
	statusHandler.AddCondition(status.HandleDegraded("RouteHealth", routeHealthCheckErrReason, routeHealthCheckErr))
	statusHandler.AddCondition(status.HandleAvailable("RouteHealth", routeHealthCheckErrReason, routeHealthCheckErr))

	return statusHandler.FlushAndReturn(routeHealthCheckErr)
}

func (c *HealthCheckController) CheckRouteHealth(ctx context.Context, operatorConfig *operatorsv1.Console, route *routev1.Route) (string, error) {
	var reason string
	err := retry.OnError(
		retry.DefaultRetry,
		func(err error) bool { return err != nil },
		func() error {
			var (
				url *url.URL
				err error
			)
			if len(operatorConfig.Spec.Ingress.ConsoleURL) == 0 {
				url, _, err = routeapihelpers.IngressURI(route, route.Spec.Host)
				if err != nil {
					reason = "RouteNotAdmitted"
					return fmt.Errorf("console route is not admitted")
				}
			} else {
				url, err = url.Parse(operatorConfig.Spec.Ingress.ConsoleURL)
				if err != nil {
					reason = "FailedParseConsoleURL"
					return fmt.Errorf("failed to parse console url: %w", err)
				}
			}

			caPool, err := c.getCA(ctx, route.Spec.TLS)
			if err != nil {
				reason = "FailedLoadCA"
				return fmt.Errorf("failed to read CA to check route health: %v", err)
			}
			client := clientWithCA(caPool)

			req, err := http.NewRequest(http.MethodGet, url.String(), nil)
			if err != nil {
				reason = "FailedRequest"
				return fmt.Errorf("failed to build request to route (%s): %v", url, err)
			}
			resp, err := client.Do(req)
			if err != nil {
				reason = "FailedGet"
				return fmt.Errorf("failed to GET route (%s): %v", url, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				reason = "StatusError"
				return fmt.Errorf("route not yet available, %s returns '%s'", url, resp.Status)
			}
			reason = ""
			return nil
		},
	)
	return reason, err
}

func (c *HealthCheckController) getCA(ctx context.Context, tls *routev1.TLSConfig) (*x509.CertPool, error) {
	caCertPool := x509.NewCertPool()

	if tls != nil && len(tls.Certificate) != 0 {
		if ok := caCertPool.AppendCertsFromPEM([]byte(tls.Certificate)); !ok {
			klog.V(4).Infof("failed to parse custom tls.crt")
		}
	}

	for _, cmName := range []string{api.TrustedCAConfigMapName, api.DefaultIngressCertConfigMapName} {
		cm, err := c.configMapLister.ConfigMaps(api.OpenShiftConsoleNamespace).Get(cmName)
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

func isExternalControlPlaneWithNLB(infrastructureConfig *configv1.Infrastructure, ingressConfig *configv1.Ingress) bool {
	return infrastructureConfig.Status.ControlPlaneTopology == configv1.ExternalTopologyMode &&
		infrastructureConfig.Status.PlatformStatus.Type == configv1.AWSPlatformType &&
		ingressConfig.Spec.LoadBalancer.Platform.Type == configv1.AWSPlatformType &&
		ingressConfig.Spec.LoadBalancer.Platform.AWS.Type == configv1.NLB
}
