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
)

const (
	consoleRouteCustomTLSSecretName   = "console-route-custom-tls"
	downloadsRouteCustomTLSSecretName = "downloads-route-custom-tls"
)

// bash script for genereting valid certificate and key is placed
// scripts/gencert.sh
// Usage of this script is:
// $ CRT_CN="client.com" CRT_SAN="DNS.1:www.client.com,DNS.2:admin.client.com,IP.1:192.168.1.10,IP.2:10.0.0.234" ./gencert.sh

const (
	validCertificate = `-----BEGIN CERTIFICATE-----
MIIFeDCCA2CgAwIBAgIJALcdZxainrkZMA0GCSqGSIb3DQEBCwUAMFMxCzAJBgNV
BAYTAkZSMQ4wDAYDVQQHDAVQYXJpczEOMAwGA1UECgwFRnJudG4xDzANBgNVBAsM
BkRldk9wczETMBEGA1UEAwwKY2xpZW50LmNvbTAeFw0yMjAyMjEyMDMwNDBaFw0z
MjAyMTkyMDMwNDBaMFMxCzAJBgNVBAYTAkZSMQ4wDAYDVQQHDAVQYXJpczEOMAwG
A1UECgwFRnJudG4xDzANBgNVBAsMBkRldk9wczETMBEGA1UEAwwKY2xpZW50LmNv
bTCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBAL67+rJxMnDUAR/hflT2
am5iMWhs1pQ3s6K0kxyqN1FcF+CzrjPEwO+qcuDw1j2UILxMqcEm+dQ3v8S6/Yaj
l2wigsyk8Hvknle1qpZSYM7ODKZsQTcW/Qiu+xGTetdoik4/ij1tcxB/2Bfdjp0f
EzxtH+sp6KDNkT4UqMKWyGZG2/UNuTB+Tgj+RrSoBfjJ/kngDm43rBi7yLTK33SJ
xfCmsqDpYLKfKKL0z2pu1YuurehHkwsg/hGojCOchlao2+C1XR8wwR6X8orggRs3
prX7rahychyja4xOxoNHPuByKLF5oHgd0ev4li4B3a3JFiF3hEQ6RGL/1DeykRHp
5RBmajxVatUK8ioFTy6ZO6e+DOFV93CZLjer+gzgyir9V40aKx/WFaeXXB0K61ji
U1UAXKPv6x4uvAftX69gOqQiqL351/FcuGzyIvII93/dj/8yEsNnR+TRVrtMBwVT
wxKD2GOjpyBOFK/IDG4xDTJrL0Hofp/weOVqBC1ND9r2zIt4XYcrOS10ozHEKijP
wRWvDlmJP9+gF4O9S/RGdDnNf5Esz5geMWgak0lpgtnPWZLMWF/MIOAT1SQcTeXg
j7U8c0xN+xvRz5DetR86Ogpk5ISct/9r5QjL4I+U5kXhQ6EFOsJvOtC5AjoPD/xE
oZ52LfBOafifJCjZeHTlIqc1AgMBAAGjTzBNMBIGA1UdEwEB/wQIMAYBAf8CAQAw
NwYDVR0RBDAwLoIOd3d3LmNsaWVudC5jb22CEGFkbWluLmNsaWVudC5jb22HBMCo
AQqHBAoAAOowDQYJKoZIhvcNAQELBQADggIBAANCCg6FQGzDZtpOuQ/LVkBqoPoN
QGexM4V+0S6V/UprSj/uTSAYMzLrZ2fvQUEWXkov62PypuzEKxSgmFEFKZM+JUIg
X8Dq/ZV96zEbBw8HeOdh+mvy/xgDUfD/omOKrcKDPMsELsnH2mfkF76M7muMG81Y
JdO+286ANwkNqgrSKTOq3f3Uj+DG43VI3YntDpd06/Xx2nlUQHVvmX9kr8/LhFQN
IsHrRnGGKCX8VQjLu0nFpelEXY5hHwtkv14xVcaB/6d7PhecIp5YyaTJ+XvAfJHJ
ddJ7F9W1gzIuGxBiwnX71y5xhPGVn77EaqFk16mkOlwDNY1g5JerufYnp9WxtOUL
qMNGFyYByx0KAzUyLcku2jSF60km4amDfNrNp5orYAF15O2kMbRWJpne9XHMWlD0
GkfUHJ1vTQas9WLOLkhU3yrPfu6eCeD0ZApFZCCwCDwxQe6tYYZxTtY62PsbJSj/
0z+nwxDw5Xz6S04cLeHrMm256YMZ8wstqYfAhBP/ebsBpWhfmy7OyqVrklIUvhAJ
Hsae7UGaTMb1TT4y4AHqC+RIl110qktG+3zGDY8Eldo5QSHmhJEWfpi1bIYPMhJe
nx86qQawaIL9ybNc++xnFnfx8LAGoTe/SVP7ZjbRsklbcPbRS8Cm76ZCm1yG+3By
HyvPeYndFocYvy0v
-----END CERTIFICATE-----`
	validKey = `-----BEGIN PRIVATE KEY-----
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
Ry2ru3EBXmrEr5O2Hzhz4FdZrpDeNA==
-----END PRIVATE KEY-----`
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
			createTLSSecret(t, client, routeTestConfig.CustomTLSSecretName)
			setOperatorConfigRoute(t, client, customRouteConfig)
		} else {
			componentRouteSpec := getComponentRouteSpec(t, client, routeTestConfig.DefaultRouteName, routeTestConfig.CustomTLSSecretName, routeTestConfig.CustomRouteHostnamePrefix)
			tc.RouteTestConfigs[i].CustomRouteHostname = string(componentRouteSpec.Hostname)
			createTLSSecret(t, client, routeTestConfig.CustomTLSSecretName)
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

func createTLSSecret(t *testing.T, client *framework.ClientSet, tlsSecretName string) {
	if tlsSecretName == "" {
		return
	}

	customTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tlsSecretName,
			Namespace: api.OpenShiftConfigNamespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte(validCertificate),
			"tls.key": []byte(validKey),
		},
	}

	_, err := client.Core.Secrets(api.OpenShiftConfigNamespace).Create(context.TODO(), customTLSSecret, metav1.CreateOptions{})
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
