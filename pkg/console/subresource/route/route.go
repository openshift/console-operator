package route

import (
	"context"
	"fmt"

	// kube
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/assets"
	customerrors "github.com/openshift/console-operator/pkg/console/errors"
)

const (
	// ingress instance named "default" is the OOTB ingresscontroller
	// this is an implicit stable API
	defaultIngressController = "default"
)

// holds information about custom TLS certificate and its key
type CustomTLSCert struct {
	Certificate string
	Key         string
}

type RouteConfig struct {
	defaultRoute RouteControllerSpec
	customRoute  RouteControllerSpec
	domain       string
	routeName    string
}

type RouteControllerSpec struct {
	hostname   string
	secretName string
}

func getComponentRouteSpec(ingressConfig *configv1.Ingress, componentName string) *configv1.ComponentRouteSpec {
	for i, componentRoute := range ingressConfig.Spec.ComponentRoutes {
		if componentRoute.Name == componentName && componentRoute.Namespace == api.OpenShiftConsoleNamespace {
			return ingressConfig.Spec.ComponentRoutes[i].DeepCopy()
		}
	}
	return nil
}

func getComponentRouteStatus(ingressConfig *configv1.Ingress, componentName string) *configv1.ComponentRouteStatus {
	for i, componentRoute := range ingressConfig.Status.ComponentRoutes {
		if componentRoute.Name == componentName && componentRoute.Namespace == api.OpenShiftConsoleNamespace {
			return ingressConfig.Status.ComponentRoutes[i].DeepCopy()
		}
	}
	return nil
}

func NewRouteConfig(operatorConfig *operatorv1.Console, ingressConfig *configv1.Ingress, routeName string) *RouteConfig {
	defaultRoute := RouteControllerSpec{
		hostname: GetDefaultRouteHost(routeName, ingressConfig),
	}
	var customRoute RouteControllerSpec
	var isIngressConfigCustomHostnameSet bool

	// Custom hostname in ingress config takes precedent over console operator's config
	componentRouteSpec := getComponentRouteSpec(ingressConfig, routeName)
	if componentRouteSpec != nil {
		customRoute.hostname = string(componentRouteSpec.Hostname)
		if componentRouteSpec.ServingCertKeyPairSecret.Name != "" {
			customRoute.secretName = componentRouteSpec.ServingCertKeyPairSecret.Name
		}
		isIngressConfigCustomHostnameSet = true
	}

	// Legacy behaviour, operatorConfig only configures custom hostname for "console" route.
	// The custom route hostname doesn't have to be set if admin just wants to set the custom
	// TLS for the default route.
	if !isIngressConfigCustomHostnameSet && routeName == api.OpenShiftConsoleRouteName {
		if len(operatorConfig.Spec.Route.Secret.Name) != 0 {
			customRoute.secretName = operatorConfig.Spec.Route.Secret.Name
		}
		customRoute.hostname = operatorConfig.Spec.Route.Hostname
	}

	// if hostname is not set and secret is, set the secret for the default route
	if len(customRoute.hostname) == 0 && len(customRoute.secretName) == 0 {
		defaultRoute.secretName = customRoute.secretName
	}

	customHostnameSpec := &RouteConfig{
		defaultRoute: defaultRoute,
		customRoute:  customRoute,
		domain:       ingressConfig.Spec.Domain,
		routeName:    routeName,
	}
	return customHostnameSpec
}

func (rc *RouteConfig) HostnameMatch() bool {
	return rc.customRoute.hostname == rc.defaultRoute.hostname
}

func (rc *RouteConfig) IsCustomHostnameSet() bool {
	return len(rc.customRoute.hostname) != 0
}

func (rc *RouteConfig) GetCustomRouteHostname() string {
	return rc.customRoute.hostname
}

func (rc *RouteConfig) IsCustomTLSSecretSet() bool {
	return len(rc.customRoute.secretName) != 0
}

func (rc *RouteConfig) IsDefaultTLSSecretSet() bool {
	return len(rc.defaultRoute.secretName) != 0
}

func (rc *RouteConfig) GetCustomTLSSecretName() string {
	return rc.customRoute.secretName
}

func (rc *RouteConfig) GetDefaultTLSSecretName() string {
	return rc.defaultRoute.secretName
}

func (rc *RouteConfig) GetDomain() string {
	return rc.domain
}

// Default `console` route points by default to the `console` service.
// If custom hostname for the console is set, then the default route
// should point to the redirect `console-redirect` service and the
// created custom route should be pointing to the `console` service.
func (rc *RouteConfig) DefaultRoute(tlsConfig *CustomTLSCert) *routev1.Route {
	route := &routev1.Route{}
	if rc.IsCustomHostnameSet() && rc.routeName == api.OpenShiftConsoleRouteName {
		route = resourceread.ReadRouteV1OrDie(assets.MustAsset(fmt.Sprintf("routes/%s-redirect-route.yaml", rc.routeName)))
	} else {
		route = resourceread.ReadRouteV1OrDie(assets.MustAsset(fmt.Sprintf("routes/%s-route.yaml", rc.routeName)))
	}
	setTLS(tlsConfig, route)
	return route
}

func (rc *RouteConfig) CustomRoute(tlsConfig *CustomTLSCert, routeName string) *routev1.Route {
	route := resourceread.ReadRouteV1OrDie(assets.MustAsset(fmt.Sprintf("routes/%s-custom-route.yaml", rc.routeName)))
	route.Spec.Host = rc.customRoute.hostname
	setTLS(tlsConfig, route)
	return route
}

func GetDefaultRouteHost(routeName string, ingressConfig *configv1.Ingress) string {
	return fmt.Sprintf("%s-%s.%s", routeName, api.OpenShiftConsoleNamespace, ingressConfig.Spec.Domain)
}

func ApplyRoute(client routeclient.RoutesGetter, recorder events.Recorder, required *routev1.Route) (*routev1.Route, bool, error) {
	existing, err := client.Routes(required.Namespace).Get(context.TODO(), required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		requiredCopy := required.DeepCopy()
		actual, err := client.Routes(requiredCopy.Namespace).Create(context.TODO(), resourcemerge.WithCleanLabelsAndAnnotations(requiredCopy).(*routev1.Route), metav1.CreateOptions{})
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	existingCopy := existing.DeepCopy()
	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	specSame := equality.Semantic.DeepEqual(existingCopy.Spec, required.Spec)

	if specSame && !*modified {
		klog.V(4).Infof("%s route exists and is in the correct state", existingCopy.ObjectMeta.Name)
		return existingCopy, false, nil
	}

	existingCopy.Spec = required.Spec
	actual, err := client.Routes(required.Namespace).Update(context.TODO(), existingCopy, metav1.UpdateOptions{})
	return actual, true, err
}

func setTLS(tlsConfig *CustomTLSCert, route *routev1.Route) {
	if tlsConfig != nil {
		route.Spec.TLS.Certificate = tlsConfig.Certificate
		route.Spec.TLS.Key = tlsConfig.Key
	}
}

func GetCustomRouteName(routeName string) string {
	return fmt.Sprintf("%s-custom", routeName)
}

func GetCanonicalHost(route *routev1.Route) (string, error) {
	for _, ingress := range route.Status.Ingress {
		if ingress.RouterName != defaultIngressController {
			klog.V(4).Infof("ignoring route %q ingress '%v'", route.ObjectMeta.Name, ingress.RouterName)
			continue
		}
		// ingress must be admitted before it is useful to us
		if !isIngressAdmitted(ingress) {
			klog.V(4).Infof("route %q ingress '%v' not admitted", route.ObjectMeta.Name, ingress.RouterName)
			continue
		}
		klog.V(4).Infof("route %q ingress '%v' found and admitted, host: %v", route.ObjectMeta.Name, defaultIngressController, ingress.Host)
		return ingress.Host, nil
	}
	klog.V(4).Infof("route %q ingress not yet ready for console", route.ObjectMeta.Name)
	return "", customerrors.NewSyncError(fmt.Sprintf("route %q is not available at canonical host %s", route.ObjectMeta.Name, route.Status.Ingress))
}

func IsAdmitted(route *routev1.Route) bool {
	for _, ingress := range route.Status.Ingress {
		if isIngressAdmitted(ingress) {
			return true
		}
	}
	return false
}

func isIngressAdmitted(ingress routev1.RouteIngress) bool {
	admitted := false
	for _, condition := range ingress.Conditions {
		if condition.Type == routev1.RouteAdmitted && condition.Status == corev1.ConditionTrue {
			admitted = true
		}
	}
	return admitted
}
