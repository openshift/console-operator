package route

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/url"
	"time"

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
	routev1listers "github.com/openshift/client-go/route/listers/route/v1"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/library-go/pkg/route/routeapihelpers"

	"github.com/openshift/console-operator/bindata"
	"github.com/openshift/console-operator/pkg/api"
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

	// if hostname for custom route is the same as the hostname for the default route
	// OR if the custom route hostname is not set:
	// - if the custom route TLS secret is set and set it for the default route
	// - unset hostname and TLS secret for the custom route
	if defaultRoute.hostname == customRoute.hostname || len(customRoute.hostname) == 0 {
		if len(customRoute.secretName) != 0 {
			defaultRoute.secretName = customRoute.secretName
		}
		customRoute.hostname = ""
		customRoute.secretName = ""
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
func (rc *RouteConfig) DefaultRoute(tlsConfig *CustomTLSCert, ingressConfig *configv1.Ingress) *routev1.Route {
	route := &routev1.Route{}
	if rc.IsCustomHostnameSet() && rc.routeName == api.OpenShiftConsoleRouteName {
		route = resourceread.ReadRouteV1OrDie(bindata.MustAsset(fmt.Sprintf("assets/routes/%s-redirect-route.yaml", rc.routeName)))
	} else {
		route = resourceread.ReadRouteV1OrDie(bindata.MustAsset(fmt.Sprintf("assets/routes/%s-route.yaml", rc.routeName)))
	}
	route.Spec.Host = GetDefaultRouteHost(rc.routeName, ingressConfig)
	setTLS(tlsConfig, route)
	return route
}

func (rc *RouteConfig) CustomRoute(tlsConfig *CustomTLSCert, routeName string) *routev1.Route {
	route := resourceread.ReadRouteV1OrDie(bindata.MustAsset(fmt.Sprintf("assets/routes/%s-custom-route.yaml", rc.routeName)))
	route.Spec.Host = rc.customRoute.hostname
	setTLS(tlsConfig, route)
	return route
}

func GetDefaultRouteHost(routeName string, ingressConfig *configv1.Ingress) string {
	return fmt.Sprintf("%s-%s.%s", routeName, api.OpenShiftConsoleNamespace, ingressConfig.Spec.Domain)
}

func ApplyRoute(client routeclient.RoutesGetter, required *routev1.Route) (*routev1.Route, bool, error) {
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

func GetActiveRouteInfo(routeClient routev1listers.RouteLister, activeRouteName string) (route *routev1.Route, routeURL *url.URL, reason string, err error) {
	route, routeErr := routeClient.Routes(api.TargetNamespace).Get(activeRouteName)
	if routeErr != nil {
		return nil, nil, "FailedGet", routeErr
	}
	uri, _, uriErr := routeapihelpers.IngressURI(route, route.Spec.Host)
	if uriErr != nil {
		return nil, nil, "FailedIngress", uriErr
	}

	return route, uri, "", nil
}

func GetCustomTLS(customCertSecret *corev1.Secret) (*CustomTLSCert, error) {
	customTLS := &CustomTLSCert{}
	cert, certExist := customCertSecret.Data["tls.crt"]
	if !certExist {
		return nil, fmt.Errorf("custom cert secret data doesn't contain 'tls.crt' entry")
	}
	certificateVerifyErr := certificateVerifier(cert)
	if certificateVerifyErr != nil {
		return nil, fmt.Errorf("failed to verify custom certificate PEM: " + certificateVerifyErr.Error())
	}
	customTLS.Certificate = string(cert)

	key, keyExist := customCertSecret.Data["tls.key"]
	if !keyExist {
		return nil, fmt.Errorf("custom cert secret data doesn't contain 'tls.key' entry")
	}

	privateKeyVerifyErr := privateKeyVerifier(key)
	if privateKeyVerifyErr != nil {
		return nil, fmt.Errorf("failed to verify custom key PEM: " + privateKeyVerifyErr.Error())
	}
	customTLS.Key = string(key)

	return customTLS, nil
}

func certificateVerifier(customCert []byte) error {
	block, _ := pem.Decode([]byte(customCert))
	if block == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	now := time.Now()
	if now.After(certificate.NotAfter) {
		return fmt.Errorf("custom TLS certificate is expired")
	}
	if now.Before(certificate.NotBefore) {
		return fmt.Errorf("custom TLS certificate is not valid yet")
	}
	return nil
}

func privateKeyVerifier(customKey []byte) error {
	block, _ := pem.Decode([]byte(customKey))
	if block == nil {
		return fmt.Errorf("failed to decode key PEM")
	}
	if _, err := x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
		if _, err = x509.ParsePKCS1PrivateKey(block.Bytes); err != nil {
			if _, err = x509.ParseECPrivateKey(block.Bytes); err != nil {
				return fmt.Errorf("block %s is not valid key PEM", block.Type)
			}
		}
	}
	return nil
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
