package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	operatorsv1 "github.com/openshift/api/operator/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	consoleapi "github.com/openshift/console-operator/pkg/api"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	"github.com/openshift/console-operator/test/e2e/framework"
	"github.com/openshift/library-go/pkg/crypto"
)

const (
	consoleRouteCustomTLSSecretName   = "console-route-custom-tls"
	downloadsRouteCustomTLSSecretName = "downloads-route-custom-tls"
)

type testCaseConfig struct {
	RouteTestConfigs []routeTestConfig
}

type routeTestConfig struct {
	DefaultRouteName          string
	CustomRouteName           string
	CustomRouteHostname       string
	CustomRouteHostnamePrefix string
	LegacySetup               bool
	SkipRouteCheck            bool
	CustomTLSSecretName       string
}

func (tc *testCaseConfig) setup(t *testing.T, client *framework.ClientSet) {
	for i, routeTestConfig := range tc.RouteTestConfigs {
		if routeTestConfig.LegacySetup {
			customRouteConfig := getCustomRouteConfig(t, client, routeTestConfig.CustomTLSSecretName, routeTestConfig.CustomRouteHostnamePrefix)
			tc.RouteTestConfigs[i].CustomRouteHostname = customRouteConfig.Hostname
			createTLSSecret(t, client, routeTestConfig.CustomTLSSecretName, customRouteConfig.Hostname)
			setOperatorConfigRoute(t, client, customRouteConfig)
		} else {
			componentRouteSpec := getComponentRouteSpec(t, client, routeTestConfig.DefaultRouteName, routeTestConfig.CustomTLSSecretName, routeTestConfig.CustomRouteHostnamePrefix)
			tc.RouteTestConfigs[i].CustomRouteHostname = string(componentRouteSpec.Hostname)
			createTLSSecret(t, client, routeTestConfig.CustomTLSSecretName, string(componentRouteSpec.Hostname))
			setIngressConfigComponentRoute(t, client, componentRouteSpec)
		}
	}
}

func (tc *testCaseConfig) checkCustomRouteWasCreated(t *testing.T, client *framework.ClientSet) {
	for _, routeTestConfig := range tc.RouteTestConfigs {
		if routeTestConfig.SkipRouteCheck {
			continue
		}
		checkCustomRouteWasCreated(t, client, routeTestConfig.CustomRouteName, routeTestConfig.CustomRouteHostname)
	}
}

func (tc *testCaseConfig) checkCustomRouteWasRemoved(t *testing.T, client *framework.ClientSet) {
	for _, routeTestConfig := range tc.RouteTestConfigs {
		checkCustomRouteWasRemoved(t, client, routeTestConfig.CustomRouteName)
	}
}

func (tc *testCaseConfig) checkRouteCustomTLSWasSet(t *testing.T, client *framework.ClientSet) {
	for _, routeTestConfig := range tc.RouteTestConfigs {
		checkCustomTLSWasSet(t, client, routeTestConfig.DefaultRouteName, routeTestConfig.CustomTLSSecretName)
	}
}

func (tc *testCaseConfig) checkRouteCustomTLSWasUnset(t *testing.T, client *framework.ClientSet) {
	for _, routeTestConfig := range tc.RouteTestConfigs {
		checkCustomTLSWasUnset(t, client, routeTestConfig.DefaultRouteName)
	}
}

func setupCustomURLTestCase(t *testing.T, testCaseConfig *testCaseConfig) (*framework.ClientSet, *operatorsv1.Console) {
	client, operatorConfig := framework.StandardSetup(t)
	if testCaseConfig != nil {
		testCaseConfig.setup(t, client)
	}
	return client, operatorConfig
}

func cleanupCustomURLTestCase(t *testing.T, client *framework.ClientSet) {
	unsetOperatorConfigRoute(t, client)
	unsetIngressConfigComponentRoute(t, client)
	for _, secretName := range []string{consoleRouteCustomTLSSecretName, downloadsRouteCustomTLSSecretName} {
		err := client.Core.Secrets(api.OpenShiftConfigNamespace).Delete(context.TODO(), secretName, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			t.Fatalf("could not delete cleanup %q secret, %v", secretName, err)
		}
	}
	framework.StandardCleanup(t, client)
}

func TestIngressConsoleComponentRoute(t *testing.T) {
	testConfig := &testCaseConfig{
		RouteTestConfigs: []routeTestConfig{
			{
				DefaultRouteName:          api.OpenShiftConsoleRouteName,
				CustomRouteName:           routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomRouteHostnamePrefix: routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomTLSSecretName:       "",
				LegacySetup:               false,
			},
		},
	}
	client, _ := setupCustomURLTestCase(t, testConfig)
	defer cleanupCustomURLTestCase(t, client)

	testConfig.checkCustomRouteWasCreated(t, client)
	unsetIngressConfigComponentRoute(t, client)
	testConfig.checkCustomRouteWasRemoved(t, client)
}

func TestIngressConsoleComponentRouteWithTLS(t *testing.T) {
	testConfig := &testCaseConfig{
		RouteTestConfigs: []routeTestConfig{
			{
				DefaultRouteName:          api.OpenShiftConsoleRouteName,
				CustomRouteName:           routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomRouteHostnamePrefix: routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomTLSSecretName:       consoleRouteCustomTLSSecretName,
				LegacySetup:               false,
			},
		},
	}
	client, _ := setupCustomURLTestCase(t, testConfig)
	defer cleanupCustomURLTestCase(t, client)

	testConfig.checkCustomRouteWasCreated(t, client)
	unsetIngressConfigComponentRoute(t, client)
	testConfig.checkCustomRouteWasRemoved(t, client)
}

// Tests default route hostname set on the Ingress config with only custom TLS
func TestIngressConsoleComponentRouteWithCustomTLS(t *testing.T) {
	testConfig := &testCaseConfig{
		RouteTestConfigs: []routeTestConfig{
			{
				DefaultRouteName:          api.OpenShiftConsoleRouteName,
				CustomRouteName:           api.OpenShiftConsoleRouteName,
				CustomRouteHostnamePrefix: api.OpenShiftConsoleRouteName,
				CustomTLSSecretName:       consoleRouteCustomTLSSecretName,
				LegacySetup:               false,
			},
		},
	}
	client, _ := setupCustomURLTestCase(t, testConfig)
	defer cleanupCustomURLTestCase(t, client)

	testConfig.checkRouteCustomTLSWasSet(t, client)
	unsetIngressConfigComponentRoute(t, client)
	testConfig.checkRouteCustomTLSWasUnset(t, client)
}

func TestIngressDownloadsComponentRoute(t *testing.T) {
	testConfig := &testCaseConfig{
		RouteTestConfigs: []routeTestConfig{
			{
				DefaultRouteName:          api.DownloadsResourceName,
				CustomRouteName:           routesub.GetCustomRouteName(api.DownloadsResourceName),
				CustomRouteHostnamePrefix: routesub.GetCustomRouteName(api.DownloadsResourceName),
				CustomTLSSecretName:       "",
				LegacySetup:               false,
			},
		},
	}
	client, _ := setupCustomURLTestCase(t, testConfig)
	defer cleanupCustomURLTestCase(t, client)

	testConfig.checkCustomRouteWasCreated(t, client)
	unsetIngressConfigComponentRoute(t, client)
	testConfig.checkCustomRouteWasRemoved(t, client)
}

func TestIngressDownloadsComponentRouteWithTLS(t *testing.T) {
	testConfig := &testCaseConfig{
		RouteTestConfigs: []routeTestConfig{
			{
				DefaultRouteName:          api.DownloadsResourceName,
				CustomRouteName:           routesub.GetCustomRouteName(api.DownloadsResourceName),
				CustomRouteHostnamePrefix: routesub.GetCustomRouteName(api.DownloadsResourceName),
				CustomTLSSecretName:       downloadsRouteCustomTLSSecretName,
				LegacySetup:               false,
			},
		},
	}
	client, _ := setupCustomURLTestCase(t, testConfig)
	defer cleanupCustomURLTestCase(t, client)

	testConfig.checkCustomRouteWasCreated(t, client)
	unsetIngressConfigComponentRoute(t, client)
	testConfig.checkCustomRouteWasRemoved(t, client)
}

func TestIngressConsoleAndDownloadsComponentRoute(t *testing.T) {
	testConfig := &testCaseConfig{
		RouteTestConfigs: []routeTestConfig{
			{
				DefaultRouteName:          api.OpenShiftConsoleRouteName,
				CustomRouteName:           routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomRouteHostnamePrefix: routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomTLSSecretName:       "",
				LegacySetup:               false,
			},
			{
				DefaultRouteName:          api.DownloadsResourceName,
				CustomRouteName:           routesub.GetCustomRouteName(api.DownloadsResourceName),
				CustomRouteHostnamePrefix: routesub.GetCustomRouteName(api.DownloadsResourceName),
				CustomTLSSecretName:       "",
				LegacySetup:               false,
			},
		},
	}
	client, _ := setupCustomURLTestCase(t, testConfig)
	defer cleanupCustomURLTestCase(t, client)

	testConfig.checkCustomRouteWasCreated(t, client)
	unsetIngressConfigComponentRoute(t, client)
	testConfig.checkCustomRouteWasRemoved(t, client)
}

func TestIngressConsoleAndDownloadsComponentRouteWithTLS(t *testing.T) {
	testConfig := &testCaseConfig{
		RouteTestConfigs: []routeTestConfig{
			{
				DefaultRouteName:          api.OpenShiftConsoleRouteName,
				CustomRouteName:           routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomRouteHostnamePrefix: routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomTLSSecretName:       consoleRouteCustomTLSSecretName,
				LegacySetup:               false,
			},
			{
				DefaultRouteName:          api.DownloadsResourceName,
				CustomRouteName:           routesub.GetCustomRouteName(api.DownloadsResourceName),
				CustomRouteHostnamePrefix: routesub.GetCustomRouteName(api.DownloadsResourceName),
				CustomTLSSecretName:       downloadsRouteCustomTLSSecretName,
				LegacySetup:               false,
			},
		},
	}
	client, _ := setupCustomURLTestCase(t, testConfig)
	defer cleanupCustomURLTestCase(t, client)

	testConfig.checkCustomRouteWasCreated(t, client)
	unsetIngressConfigComponentRoute(t, client)
	testConfig.checkCustomRouteWasRemoved(t, client)
}

func TestLegacyCustomURL(t *testing.T) {
	testConfig := &testCaseConfig{
		RouteTestConfigs: []routeTestConfig{
			{
				DefaultRouteName:          api.OpenShiftConsoleRouteName,
				CustomRouteName:           routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomRouteHostnamePrefix: routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomTLSSecretName:       "",
				LegacySetup:               true,
			},
		},
	}
	client, _ := setupCustomURLTestCase(t, testConfig)
	defer cleanupCustomURLTestCase(t, client)

	testConfig.checkCustomRouteWasCreated(t, client)
	unsetOperatorConfigRoute(t, client)
	testConfig.checkCustomRouteWasRemoved(t, client)
}

func TestLegacyCustomURLWithTLS(t *testing.T) {
	testConfig := &testCaseConfig{
		RouteTestConfigs: []routeTestConfig{
			{
				DefaultRouteName:          api.OpenShiftConsoleRouteName,
				CustomRouteName:           routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomRouteHostnamePrefix: routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomTLSSecretName:       consoleRouteCustomTLSSecretName,
				LegacySetup:               true,
			},
		},
	}
	client, _ := setupCustomURLTestCase(t, testConfig)
	defer cleanupCustomURLTestCase(t, client)

	testConfig.checkCustomRouteWasCreated(t, client)
	unsetOperatorConfigRoute(t, client)
	testConfig.checkCustomRouteWasRemoved(t, client)
}

func TestLegacyConsoleComponentRouteWithCustomTLS(t *testing.T) {
	testConfig := &testCaseConfig{
		RouteTestConfigs: []routeTestConfig{
			{
				DefaultRouteName:          api.OpenShiftConsoleRouteName,
				CustomRouteName:           api.OpenShiftConsoleRouteName,
				CustomRouteHostnamePrefix: api.OpenShiftConsoleRouteName,
				CustomTLSSecretName:       consoleRouteCustomTLSSecretName,
				LegacySetup:               true,
			},
		},
	}
	client, _ := setupCustomURLTestCase(t, testConfig)
	defer cleanupCustomURLTestCase(t, client)

	testConfig.checkRouteCustomTLSWasSet(t, client)
	unsetOperatorConfigRoute(t, client)
	testConfig.checkRouteCustomTLSWasUnset(t, client)
}

func TestLegacyCustomURLWithIngressConsoleComponentRoute(t *testing.T) {
	testConfig := &testCaseConfig{
		RouteTestConfigs: []routeTestConfig{
			{
				DefaultRouteName:          api.OpenShiftConsoleRouteName,
				CustomRouteName:           routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomRouteHostnamePrefix: routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomTLSSecretName:       consoleRouteCustomTLSSecretName,
				LegacySetup:               true,
				SkipRouteCheck:            true,
			},
			{
				DefaultRouteName:          api.OpenShiftConsoleRouteName,
				CustomRouteName:           routesub.GetCustomRouteName(api.OpenShiftConsoleRouteName),
				CustomRouteHostnamePrefix: fmt.Sprintf("%s-custom-ingress", api.OpenShiftConsoleRouteName),
				CustomTLSSecretName:       consoleRouteCustomTLSSecretName,
				LegacySetup:               false,
			},
		},
	}
	client, _ := setupCustomURLTestCase(t, testConfig)
	defer cleanupCustomURLTestCase(t, client)

	testConfig.checkCustomRouteWasCreated(t, client)
	unsetOperatorConfigRoute(t, client)
	unsetIngressConfigComponentRoute(t, client)
	testConfig.checkCustomRouteWasRemoved(t, client)
}

func checkCustomRouteWasCreated(t *testing.T, client *framework.ClientSet, routeName, hostname string) {
	err := wait.Poll(1*time.Second, pollTimeout, func() (stop bool, err error) {
		route, err := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), routeName, v1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return true, err
		}
		if route.Spec.Host == hostname {
			return true, nil
		}
		// it's better to wait for timeout then error out prematurely without waiting for operator to consilidate the route
		return false, nil
	})
	if err != nil {
		t.Errorf("error: %s", err)
	}
}

func checkCustomRouteWasRemoved(t *testing.T, client *framework.ClientSet, routeName string) {
	err := wait.Poll(1*time.Second, pollTimeout, func() (stop bool, err error) {
		_, err = client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), routeName, v1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return true, err
		}
		return false, nil
	})
	if err != nil {
		t.Errorf("error: %s", err)
	}
}

func checkCustomTLSWasSet(t *testing.T, client *framework.ClientSet, routeName string, customSecretName string) {
	route := &routev1.Route{}
	customSecret, err := client.Core.Secrets(api.OpenShiftConfigNamespace).Get(context.TODO(), customSecretName, v1.GetOptions{})
	if err != nil {
		t.Fatalf("could not get custom TLS secret, %v", err)
	}
	err = wait.Poll(1*time.Second, pollTimeout, func() (stop bool, err error) {
		route, err = client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), routeName, v1.GetOptions{})
		if err != nil {
			return true, err
		}

		customTLS, err := routesub.GetCustomTLS(customSecret)
		if err != nil {
			return true, err
		}

		if route.Spec.TLS.Certificate == customTLS.Certificate && route.Spec.TLS.Key == customTLS.Key {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}

func checkCustomTLSWasUnset(t *testing.T, client *framework.ClientSet, routeName string) {
	err := wait.Poll(1*time.Second, 20*time.Second, func() (stop bool, err error) {
		route, err := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), routeName, v1.GetOptions{})
		if err != nil {
			return true, err
		}
		if len(route.Spec.TLS.Certificate) == 0 && len(route.Spec.TLS.Key) == 0 {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		t.Errorf("error: %s", err)
	}
}

func getCustomRouteConfig(t *testing.T, client *framework.ClientSet, secretName string, customHostnamePrefix string) operatorsv1.ConsoleConfigRoute {
	ingressConfig, err := client.Ingress.Ingresses().Get(context.TODO(), consoleapi.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("could not get ingress config, %v", err)
	}
	customRouteHostname := fmt.Sprintf("%s-%s.%s", customHostnamePrefix, api.OpenShiftConsoleNamespace, ingressConfig.Spec.Domain)
	customRouteConfig := operatorsv1.ConsoleConfigRoute{
		Hostname: customRouteHostname,
		Secret: configv1.SecretNameReference{
			Name: secretName,
		},
	}

	return customRouteConfig
}

func getComponentRouteSpec(t *testing.T, client *framework.ClientSet, routeName string, secretName string, customHostnamePrefix string) configv1.ComponentRouteSpec {
	ingressConfig, err := client.Ingress.Ingresses().Get(context.TODO(), consoleapi.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("could not get ingress config, %v", err)
	}
	customRouteHostname := fmt.Sprintf("%s-%s.%s", customHostnamePrefix, api.OpenShiftConsoleNamespace, ingressConfig.Spec.Domain)
	componentRouteSpec := configv1.ComponentRouteSpec{
		Namespace: api.OpenShiftConsoleNamespace,
		Name:      routeName,
		Hostname:  configv1.Hostname(customRouteHostname),
		ServingCertKeyPairSecret: configv1.SecretNameReference{
			Name: secretName,
		},
	}

	return componentRouteSpec
}

func createTLSSecret(t *testing.T, client *framework.ClientSet, tlsSecretName, hostname string) {
	if tlsSecretName == "" {
		return
	}
	tlsCert, err := crypto.MakeSelfSignedCAConfig(hostname, 1)
	if err != nil {
		t.Errorf("error: %s", err)
	}
	certBytes, keyBytes, err := tlsCert.GetPEMBytes()
	if err != nil {
		t.Errorf("error: %s", err)
	}

	customTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tlsSecretName,
			Namespace: api.OpenShiftConfigNamespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certBytes,
			"tls.key": keyBytes,
		},
	}

	_, err = client.Core.Secrets(api.OpenShiftConfigNamespace).Create(context.TODO(), customTLSSecret, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		t.Errorf("error creating custom TLS Secret: %s", err)
	}
}

func setOperatorConfigRoute(t *testing.T, client *framework.ClientSet, routeConfig operatorsv1.ConsoleConfigRoute) {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		operatorConfig, err := client.Operator.Consoles().Get(context.TODO(), consoleapi.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("could not get operator config, %v", err)
		}

		t.Logf("setting custom URL on console-operator config to %q", routeConfig.Hostname)
		operatorConfig.Spec.Route = routeConfig

		_, err = client.Operator.Consoles().Update(context.TODO(), operatorConfig, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		t.Fatalf("could not update operator config to set custom route: %v", err)
	}
}

func setIngressConfigComponentRoute(t *testing.T, client *framework.ClientSet, componentRouteSpec configv1.ComponentRouteSpec) {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		ingressConfig, err := client.Ingress.Ingresses().Get(context.TODO(), consoleapi.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("could not get ingress config, %v", err)
		}

		t.Logf("setting custom URL on ingress config for %q to %q", componentRouteSpec.Name, componentRouteSpec.Hostname)
		ingressConfig.Spec.ComponentRoutes = append(ingressConfig.Spec.ComponentRoutes, componentRouteSpec)

		_, err = client.Ingress.Ingresses().Update(context.TODO(), ingressConfig, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		t.Fatalf("could not update ingress config to set custom route: %v", err)
	}
}

func unsetIngressConfigComponentRoute(t *testing.T, client *framework.ClientSet) {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		ingressConfig, err := client.Ingress.Ingresses().Get(context.TODO(), consoleapi.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("could not get ingress config, %v", err)
		}

		t.Logf("unsetting ingress config's component routes")
		ingressConfig.Spec.ComponentRoutes = []configv1.ComponentRouteSpec{}

		_, err = client.Ingress.Ingresses().Update(context.TODO(), ingressConfig, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		t.Fatalf("could not update ingress config to unset component routes: %v", err)
	}
}

// replace console-openshift-console.apps.user.devcluster.openshift.com
// with    console-custom-openshift-console.apps.user.devcluster.openshift.com
func getCustomHostname(t *testing.T, routeName string, route *routev1.Route) string {
	defaultHost := route.Spec.Host
	return strings.Replace(defaultHost, routeName, fmt.Sprintf("%s-custom", routeName), 1)
}

func unsetOperatorConfigRoute(t *testing.T, client *framework.ClientSet) {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		operatorConfig, err := client.Operator.Consoles().Get(context.TODO(), consoleapi.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("could not get operator config, %v", err)
		}
		t.Logf("unsetting custom URL")
		operatorConfig.Spec.Route = operatorsv1.ConsoleConfigRoute{}

		_, err = client.Operator.Consoles().Update(context.TODO(), operatorConfig, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		t.Fatalf("could not update operator config with unset custom route: %v", err)
	}
}
