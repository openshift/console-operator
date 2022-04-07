package route

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"
	"time"

	// k8s
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformersv1 "k8s.io/client-go/informers/core/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"

	// openshift
	operatorsv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	v1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	routeclientv1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	routesinformersv1 "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	// console-operator
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
)

type RouteSyncController struct {
	routeName            string
	isHealthCheckEnabled bool
	// clients
	operatorClient       v1helpers.OperatorClient
	operatorConfigClient operatorclientv1.ConsoleInterface
	ingressClient        configclientv1.IngressInterface
	routeClient          routeclientv1.RoutesGetter
	configMapClient      coreclientv1.ConfigMapsGetter
	secretClient         coreclientv1.SecretsGetter
	// events
	resourceSyncer resourcesynccontroller.ResourceSyncer
}

func NewRouteSyncController(
	routeName string,
	isHealthCheckEnabled bool,
	// top level config
	configClient configclientv1.ConfigV1Interface,
	configInformer configinformer.SharedInformerFactory,
	// clients
	operatorClient v1helpers.OperatorClient,
	operatorConfigClient operatorclientv1.ConsoleInterface,
	routev1Client routeclientv1.RoutesGetter,
	configMapClient coreclientv1.ConfigMapsGetter,
	secretClient coreclientv1.SecretsGetter,
	// informers
	operatorConfigInformer v1.ConsoleInformer,
	coreInformer coreinformersv1.Interface,
	secretInformer coreinformersv1.SecretInformer,
	routeInformer routesinformersv1.RouteInformer,
	// events
	recorder events.Recorder,
	resourceSyncer resourcesynccontroller.ResourceSyncer,
) factory.Controller {
	ctrl := &RouteSyncController{
		routeName:            routeName,
		isHealthCheckEnabled: isHealthCheckEnabled,
		operatorClient:       operatorClient,
		operatorConfigClient: operatorConfigClient,
		ingressClient:        configClient.Ingresses(),
		routeClient:          routev1Client,
		configMapClient:      configMapClient,
		secretClient:         secretClient,
		// events
		resourceSyncer: resourceSyncer,
	}

	configMapInformer := coreInformer.ConfigMaps()
	configV1Informers := configInformer.Config().V1()

	return factory.New().
		WithFilteredEventsInformers( // configs
			util.NamesFilter(api.ConfigResourceName),
			configV1Informers.Consoles().Informer(),
			operatorConfigInformer.Informer(),
			configV1Informers.Ingresses().Informer(),
		).WithFilteredEventsInformers( // service
		util.NamesFilter(api.TrustedCAConfigMapName, api.DefaultIngressCertConfigMapName),
		configMapInformer.Informer(),
	).WithInformers(
		secretInformer.Informer(),
	).WithFilteredEventsInformers( // route
		util.NamesFilter(routeName, routesub.GetCustomRouteName(routeName)),
		routeInformer.Informer(),
	).WithSync(ctrl.Sync).
		ToController(fmt.Sprintf("%sRouteController", strings.Title(routeName)), recorder.WithComponentSuffix(fmt.Sprintf("%s-route-controller", routeName)))
}

func (c *RouteSyncController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.operatorConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	updatedOperatorConfig := operatorConfig.DeepCopy()

	switch updatedOperatorConfig.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infof("console-operator is in a managed state: syncing %q route", c.routeName)
	case operatorsv1.Unmanaged:
		klog.V(4).Infof("console-operator is in an unmanaged state: skipping %q route sync", c.routeName)
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infof("console-operator is in a removed state: deleting %q route", c.routeName)
		if err = c.removeRoute(ctx, routesub.GetCustomRouteName(c.routeName)); err != nil {
			return err
		}
		return c.removeRoute(ctx, c.routeName)
	default:
		return fmt.Errorf("unknown state: %v", updatedOperatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	ingressConfig, err := c.ingressClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return statusHandler.FlushAndReturn(err)
	}
	routeConfig := routesub.NewRouteConfig(updatedOperatorConfig, ingressConfig, c.routeName)

	// try to sync the custom route first. If the sync fails for any reason, error
	// out the sync loop and inform about this fact instead of putting default
	// route into inaccessible state.
	_, customRouteErrReason, customRouteErr := c.SyncCustomRoute(ctx, routeConfig, controllerContext)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("CustomRouteSync", customRouteErrReason, customRouteErr))
	if customRouteErr != nil {
		return statusHandler.FlushAndReturn(customRouteErr)
	}

	_, defaultRouteErrReason, defaultRouteErr := c.SyncDefaultRoute(ctx, routeConfig, controllerContext)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("DefaultRouteSync", defaultRouteErrReason, defaultRouteErr))

	// warn if deprecated configuration of custom domain for 'console' route is set on the console-operator config
	if (len(operatorConfig.Spec.Route.Hostname) != 0 || len(operatorConfig.Spec.Route.Secret.Name) != 0) && c.routeName == api.OpenShiftConsoleRouteName {
		klog.Warning(deprecationMessage(operatorConfig))
	}

	return statusHandler.FlushAndReturn(defaultRouteErr)
}

func (c *RouteSyncController) removeRoute(ctx context.Context, routeName string) error {
	err := c.routeClient.Routes(api.OpenShiftConsoleNamespace).Delete(ctx, routeName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *RouteSyncController) SyncDefaultRoute(ctx context.Context, routeConfig *routesub.RouteConfig, controllerContext factory.SyncContext) (*routev1.Route, string, error) {
	customTLSSecret, configErr := c.GetDefaultRouteTLSSecret(ctx, routeConfig)
	if configErr != nil {
		return nil, "InvalidDefaultRouteConfig", configErr
	}
	customTLSCert, secretValidationErr := ValidateCustomCertSecret(customTLSSecret)
	if secretValidationErr != nil {
		return nil, "InvalidCustomTLSSecret", secretValidationErr
	}

	requiredDefaultRoute := routeConfig.DefaultRoute(customTLSCert)

	defaultRoute, _, defaultRouteError := routesub.ApplyRoute(c.routeClient, controllerContext.Recorder(), requiredDefaultRoute)
	if defaultRouteError != nil {
		return nil, "FailedDefaultRouteApply", defaultRouteError
	}

	if _, defaultRouteError = routesub.GetCanonicalHost(defaultRoute); defaultRouteError != nil {
		return nil, "FailedAdmitDefaultRoute", defaultRouteError
	}

	return defaultRoute, "", defaultRouteError
}

// Custom route sync needs to:
// 1. validate if the reference for secret with TLS certificate and key is defined in operator config(in case a non-openshift cluster domain is used)
// 2. if secret is defined, verify the TLS certificate and key
// 4. create the custom console route, if custom TLS certificate and key are defined use them
// 5. apply the custom route
func (c *RouteSyncController) SyncCustomRoute(ctx context.Context, routeConfig *routesub.RouteConfig, controllerContext factory.SyncContext) (*routev1.Route, string, error) {
	if !routeConfig.IsCustomHostnameSet() {
		if err := c.removeRoute(ctx, routesub.GetCustomRouteName(c.routeName)); err != nil {
			return nil, "FailedDeleteCustomRoutes", err
		}
		return nil, "", nil
	}

	// Check if the custom route hostname is not same as the default one.
	// If it is, dont create a custom route but rather update the default one.
	if routeConfig.HostnameMatch() {
		return nil, "", nil
	}

	if configErr := c.ValidateCustomRouteConfig(ctx, routeConfig); configErr != nil {
		return nil, "InvalidCustomRouteConfig", configErr
	}

	customTLSSecret, customTLSSecretErr := c.GetCustomRouteTLSSecret(ctx, routeConfig)
	if customTLSSecretErr != nil {
		return nil, "FailedCustomTLSSecretGet", fmt.Errorf("failed to GET custom route TLS secret: %s", customTLSSecretErr)
	}

	customTLSCert, secretValidationErr := ValidateCustomCertSecret(customTLSSecret)
	if secretValidationErr != nil {
		return nil, "InvalidCustomTLSSecret", secretValidationErr
	}

	requiredCustomRoute := routeConfig.CustomRoute(customTLSCert, c.routeName)
	customRoute, _, customRouteError := routesub.ApplyRoute(c.routeClient, controllerContext.Recorder(), requiredCustomRoute)
	if customRouteError != nil {
		return nil, "FailedCustomRouteApply", customRouteError
	}

	if _, customRouteError = routesub.GetCanonicalHost(customRoute); customRouteError != nil {
		return nil, "FailedAdmitCustomRoute", customRouteError
	}

	return customRoute, "", customRouteError
}

func (c *RouteSyncController) GetCustomRouteTLSSecret(ctx context.Context, routeConfig *routesub.RouteConfig) (*corev1.Secret, error) {
	if routeConfig.IsCustomTLSSecretSet() {
		customTLSSecret, customTLSSecretErr := c.secretClient.Secrets(api.OpenShiftConfigNamespace).Get(ctx, routeConfig.GetCustomTLSSecretName(), metav1.GetOptions{})
		if customTLSSecretErr != nil {
			return nil, fmt.Errorf("failed to GET custom route TLS secret: %s", customTLSSecretErr)
		}
		return customTLSSecret, nil
	}
	return nil, nil
}

func (c *RouteSyncController) GetDefaultRouteTLSSecret(ctx context.Context, routeConfig *routesub.RouteConfig) (*corev1.Secret, error) {
	// if custom route is set, we don't need to validate the config
	// since it will be used for the custom route, not the default one
	if routeConfig.IsCustomHostnameSet() {
		return nil, nil
	}

	if !routeConfig.IsDefaultTLSSecretSet() {
		return nil, nil
	}

	secret, secretErr := c.secretClient.Secrets(api.OpenShiftConfigNamespace).Get(ctx, routeConfig.GetDefaultTLSSecretName(), metav1.GetOptions{})
	if secretErr != nil {
		return nil, fmt.Errorf("failed to GET default route TLS secret: %s", secretErr)
	}
	return secret, nil
}

func (c *RouteSyncController) ValidateCustomRouteConfig(ctx context.Context, routeConfig *routesub.RouteConfig) error {
	// Check if the custom hostname has cluster domain suffix, which indicates
	// if a secret that contains TLS certificate and key needs to exist in the
	// `openshift-config` namespace and referenced in  the operator config.
	// If the suffix matches the cluster domain, then the secret is optional.
	// If the suffix doesn't matches the cluster domain, then the secret is mandatory.
	if !routeConfig.IsCustomTLSSecretSet() {
		if !strings.HasSuffix(routeConfig.GetCustomRouteHostname(), routeConfig.GetDomain()) {
			return fmt.Errorf("secret reference for custom route TLS secret is not defined")
		}
	}
	return nil
}

// Validate secret that holds custom TLS certificate and key.
// Secret has to contain `tls.crt` and `tls.key` data keys
// where the certificate and key are stored and both need
// to be in valid format.
// Return the custom TLS certificate and key
func ValidateCustomCertSecret(customCertSecret *corev1.Secret) (*routesub.CustomTLSCert, error) {
	if customCertSecret == nil {
		return nil, nil
	}
	if customCertSecret.Type != corev1.SecretTypeTLS {
		return nil, fmt.Errorf("custom cert secret is not in %q type, instead uses %q type", corev1.SecretTypeTLS, customCertSecret.Type)
	}

	return routesub.GetCustomTLS(customCertSecret)
}

func (c *RouteSyncController) CheckRouteHealth(ctx context.Context, route *routev1.Route) (string, error) {
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

func (c *RouteSyncController) getCA(ctx context.Context, tls *routev1.TLSConfig) (*x509.CertPool, error) {
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

func deprecationMessage(operatorConfig *operatorsv1.Console) string {
	msg := `Deprecated: custom domain is being configured on console-operator config for the 'console' route.
Please remove that entry from console-operator config and instead configure ingress config spec with following custom domain entry for 'console' route:
----
spec:
  componentRoutes:
  - name: console
    namespace: openshift-console
`
	if len(operatorConfig.Spec.Route.Hostname) != 0 {
		msg += fmt.Sprintf("    hostname: %s\n", operatorConfig.Spec.Route.Hostname)
	}

	if len(operatorConfig.Spec.Route.Secret.Name) != 0 {
		msg += "    servingCertKeyPairSecret:\n"
		msg += fmt.Sprintf("      name: %s\n", operatorConfig.Spec.Route.Secret.Name)
	}
	msg += "----"

	return msg
}
