package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	operatorsv1 "github.com/openshift/api/operator/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	consoleapi "github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/test/e2e/framework"
	"github.com/openshift/library-go/pkg/crypto"
)

const (
	tlsSecretName = "custom-route-tls"
)

func setupCustomURLTestCase(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	return framework.StandardSetup(t)
}

func cleanupCustomURLTestCase(t *testing.T, client *framework.ClientSet) {
	framework.StandardCleanup(t, client)
	err := client.Core.Secrets(api.OpenShiftConfigNamespace).Delete(context.TODO(), tlsSecretName, metav1.DeleteOptions{})
	if err != nil && !apiErrors.IsNotFound(err) {
		t.Fatalf("could not delete cleanup %q secret, %v", tlsSecretName, err)
	}
	framework.StandardCleanup(t, client)
}

func TestCustomURL(t *testing.T) {
	client, _ := setupCustomURLTestCase(t)
	defer cleanupCustomURLTestCase(t, client)

	customRouteConfig := getCustomRouteConfig(t, client, "")

	setOperatorConfigRoute(t, client, customRouteConfig)
	err := wait.Poll(1*time.Second, pollTimeout, func() (stop bool, err error) {
		_, err = client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), api.OpenshiftConsoleCustomRouteName, v1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return true, err
		}
		return true, nil
	})
	if err != nil {
		t.Errorf("error: %s", err)
	}

	// remove the custom custom hostname from the console operator config
	unsetOperatorConfigRoute(t, client)
	err = wait.Poll(1*time.Second, pollTimeout, func() (stop bool, err error) {
		_, err = client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), api.OpenshiftConsoleCustomRouteName, v1.GetOptions{})
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

func TestCustomURLWithTLS(t *testing.T) {
	client, _ := setupCustomURLTestCase(t)
	defer cleanupCustomURLTestCase(t, client)

	customRouteConfig := getCustomRouteConfig(t, client, tlsSecretName)
	createTLSSecret(t, client, customRouteConfig.Hostname)
	setOperatorConfigRoute(t, client, customRouteConfig)
	err := wait.Poll(1*time.Second, pollTimeout, func() (stop bool, err error) {
		_, err = client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), api.OpenshiftConsoleCustomRouteName, v1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return true, err
		}
		return true, nil
	})
	if err != nil {
		t.Errorf("error: %s", err)
	}

	// remove the custom custom hostname from the console operator config
	unsetOperatorConfigRoute(t, client)
	err = wait.Poll(1*time.Second, pollTimeout, func() (stop bool, err error) {
		_, err = client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), api.OpenshiftConsoleCustomRouteName, v1.GetOptions{})
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

func getCustomRouteConfig(t *testing.T, client *framework.ClientSet, secretName string) operatorsv1.ConsoleConfigRoute {
	route, err := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), api.OpenShiftConsoleName, v1.GetOptions{})
	if err != nil {
		t.Fatalf("could not get route: %s", err)
	}
	customRouteHostname := getCustomHostname(t, route)
	customRouteConfig := operatorsv1.ConsoleConfigRoute{
		Hostname: customRouteHostname,
		Secret: configv1.SecretNameReference{
			Name: secretName,
		},
	}

	return customRouteConfig
}

func createTLSSecret(t *testing.T, client *framework.ClientSet, hostname string) {
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
	if err != nil {
		t.Errorf("error creating custom TLS Secret: %s", err)
	}
}

func setOperatorConfigRoute(t *testing.T, client *framework.ClientSet, routeConfig operatorsv1.ConsoleConfigRoute) {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		operatorConfig, err := client.Operator.Consoles().Get(context.TODO(), consoleapi.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("could not get operator config, %v", err)
		}

		t.Logf("setting custom URL to '%s'", routeConfig.Hostname)
		operatorConfig.Spec.Route = routeConfig

		_, err = client.Operator.Consoles().Update(context.TODO(), operatorConfig, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		t.Fatalf("could not update operator config to unset custom route: %v", err)
	}
}

// replace console-openshift-console.apps.user.devcluster.openshift.com
// with    console-custom-openshift-console.apps.user.devcluster.openshift.com
func getCustomHostname(t *testing.T, route *routev1.Route) string {
	defaultHost := route.Spec.Host
	return strings.Replace(defaultHost, "console", "console-custom", 1)
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
