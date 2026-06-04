package e2e

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	api "github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/test/e2e/framework"
)

func TestHTTP2CertAutoGeneration(t *testing.T) {
	client, _ := framework.StandardSetup(t)
	defer framework.StandardCleanup(t, client)

	var secret *corev1.Secret
	err := wait.Poll(1*time.Second, framework.AsyncOperationTimeout, func() (bool, error) {
		var getErr error
		secret, getErr = client.Core.Secrets(api.OpenShiftConsoleNamespace).Get(context.TODO(), api.ConsoleHTTP2CertSecretName, metav1.GetOptions{})
		if getErr != nil {
			if apierrors.IsNotFound(getErr) {
				return false, nil
			}
			return false, getErr
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("Verify that the operator created the HTTP/2 cert secret: %v", err)
	}

	if secret.Type != corev1.SecretTypeTLS {
		t.Errorf("expected secret type %s, got %s", corev1.SecretTypeTLS, secret.Type)
	}
	if len(secret.Data["tls.crt"]) == 0 {
		t.Error("expected non-empty tls.crt in HTTP/2 cert secret")
	}
	if len(secret.Data["tls.key"]) == 0 {
		t.Error("expected non-empty tls.key in HTTP/2 cert secret")
	}

	route, err := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), api.OpenShiftConsoleRouteName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("could not get console route: %v", err)
	}
	if route.Spec.TLS == nil || len(route.Spec.TLS.Certificate) == 0 || len(route.Spec.TLS.Key) == 0 {
		t.Error("expected the console route to have the auto-generated HTTP/2 TLS cert and key set")
	}
}

func TestHTTP2CertRegeneration(t *testing.T) {
	client, _ := framework.StandardSetup(t)
	defer framework.StandardCleanup(t, client)

	err := wait.Poll(1*time.Second, framework.AsyncOperationTimeout, func() (bool, error) {
		_, getErr := client.Core.Secrets(api.OpenShiftConsoleNamespace).Get(context.TODO(), api.ConsoleHTTP2CertSecretName, metav1.GetOptions{})
		if getErr != nil {
			if apierrors.IsNotFound(getErr) {
				return false, nil
			}
			return false, getErr
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("HTTP/2 cert secret was not created initially: %v", err)
	}

	t.Log("Deleting the HTTP/2 cert secret to verify that the operator regenerates it")
	err = client.Core.Secrets(api.OpenShiftConsoleNamespace).Delete(context.TODO(), api.ConsoleHTTP2CertSecretName, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("could not delete HTTP/2 cert secret: %v", err)
	}

	err = wait.Poll(1*time.Second, framework.AsyncOperationTimeout, func() (bool, error) {
		_, getErr := client.Core.Secrets(api.OpenShiftConsoleNamespace).Get(context.TODO(), api.ConsoleHTTP2CertSecretName, metav1.GetOptions{})
		if getErr != nil {
			if apierrors.IsNotFound(getErr) {
				return false, nil
			}
			return false, getErr
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("HTTP/2 cert secret was not regenerated after deletion: %v", err)
	}

	route, err := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), api.OpenShiftConsoleRouteName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("could not get console route: %v", err)
	}
	if route.Spec.TLS == nil || len(route.Spec.TLS.Certificate) == 0 {
		t.Error("expected the console route to have a TLS cert set after regeneration")
	}
}

func TestHTTP2CertAdminOverride(t *testing.T) {
	client, _ := framework.StandardSetup(t)
	defer cleanupCustomURLTestCase(t, client)

	tlsSecretName := "http2-test-custom-tls"
	componentRouteSpec := getComponentRouteSpec(t, client, api.OpenShiftConsoleRouteName, tlsSecretName, api.OpenShiftConsoleRouteName)
	createTLSSecret(t, client, tlsSecretName, string(componentRouteSpec.Hostname))
	defer func() {
		_ = client.Core.Secrets(api.OpenShiftConfigNamespace).Delete(context.TODO(), tlsSecretName, metav1.DeleteOptions{})
	}()
	setIngressConfigComponentRoute(t, client, componentRouteSpec)

	checkCustomTLSWasSet(t, client, api.OpenShiftConsoleRouteName, tlsSecretName)

	t.Log("Removing the admin custom cert to verify fallback to the auto-generated cert")
	unsetIngressConfigComponentRoute(t, client)

	err := wait.Poll(1*time.Second, framework.AsyncOperationTimeout, func() (bool, error) {
		route, getErr := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), api.OpenShiftConsoleRouteName, metav1.GetOptions{})
		if getErr != nil {
			return false, getErr
		}
		if route.Spec.TLS == nil || len(route.Spec.TLS.Certificate) == 0 {
			return false, nil
		}
		adminSecret, secretErr := client.Core.Secrets(api.OpenShiftConfigNamespace).Get(context.TODO(), tlsSecretName, metav1.GetOptions{})
		if secretErr != nil {
			if apierrors.IsNotFound(secretErr) {
				return true, nil
			}
			return false, secretErr
		}
		if route.Spec.TLS.Certificate != string(adminSecret.Data["tls.crt"]) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("the route did not fall back to the auto-generated cert after removing the admin cert: %v", err)
	}

}

func TestHTTP2CertNotOnDownloadsRoute(t *testing.T) {
	client, _ := framework.StandardSetup(t)
	defer framework.StandardCleanup(t, client)

	route, err := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), api.OpenShiftConsoleDownloadsRouteName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("could not get the downloads route: %v", err)
	}
	if route.Spec.TLS != nil && len(route.Spec.TLS.Certificate) > 0 {
		t.Error("expected the downloads route to have no TLS cert set")
	}
}
