package e2e

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	configv1 "github.com/openshift/api/config/v1"

	"github.com/openshift/console-operator/pkg/api"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	"github.com/openshift/console-operator/test/e2e/framework"
)

var additionalRoutePollTimeout = 60 * time.Second

func TestAdditionalRouteLifecycle(t *testing.T) {
	client, _ := framework.StandardSetup(t)
	defer cleanupAdditionalRoutes(t, client, []string{"console-e2e-a", "console-e2e-b"})

	domain := getClusterAppsDomain(t, client)
	nameA := "console-e2e-a"
	nameB := "console-e2e-b"
	hostnameA := additionalRouteHostname(nameA, domain)
	hostnameB := additionalRouteHostname(nameB, domain)

	addComponentRoute(t, client, nameA, hostnameA, nil)
	addComponentRoute(t, client, nameB, hostnameB, nil)

	t.Run("RoutesCreated", func(t *testing.T) {
		waitForAdditionalRoute(t, client, nameA, hostnameA)
		waitForAdditionalRoute(t, client, nameB, hostnameB)
	})

	t.Run("OAuthRedirectURIs", func(t *testing.T) {
		checkOAuthRedirectURI(t, client, hostnameA, true)
		checkOAuthRedirectURI(t, client, hostnameB, true)
	})

	t.Run("ConfigMapHosts", func(t *testing.T) {
		checkConfigMapAdditionalHost(t, client, hostnameA, true)
		checkConfigMapAdditionalHost(t, client, hostnameB, true)
	})

	t.Run("SelectiveRemoval", func(t *testing.T) {
		removeComponentRoute(t, client, nameA)
		waitForAdditionalRouteRemoved(t, client, nameA)

		// Route B should still exist
		route, err := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), nameB, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("expected route %s to still exist: %v", nameB, err)
		}
		if route.Spec.Host != hostnameB {
			t.Errorf("expected route %s host %q, got %q", nameB, hostnameB, route.Spec.Host)
		}

		checkOAuthRedirectURI(t, client, hostnameA, false)
		checkOAuthRedirectURI(t, client, hostnameB, true)
		checkConfigMapAdditionalHost(t, client, hostnameA, false)
		checkConfigMapAdditionalHost(t, client, hostnameB, true)
	})

	t.Run("RouteAdmissionAndRedirect", func(t *testing.T) {
		// Wait for route to be admitted
		err := wait.Poll(2*time.Second, additionalRoutePollTimeout, func() (bool, error) {
			route, err := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), nameB, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			for _, ingress := range route.Status.Ingress {
				for _, cond := range ingress.Conditions {
					if cond.Type == "Admitted" && cond.Status == "True" {
						return true, nil
					}
				}
			}
			return false, nil
		})
		if err != nil {
			t.Errorf("route %s was not admitted within %s, skipping HTTP check", nameB, additionalRoutePollTimeout)
			return
		}

		// Verify /auth/login redirects with redirect_uri matching this hostname
		noRedirectClient := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
		loginURL := fmt.Sprintf("https://%s/auth/login", hostnameB)
		err = wait.Poll(2*time.Second, additionalRoutePollTimeout, func() (bool, error) {
			resp, err := noRedirectClient.Get(loginURL)
			if err != nil {
				return false, nil
			}
			defer resp.Body.Close()
			location := resp.Header.Get("Location")
			if location == "" {
				return false, nil
			}
			parsed, err := url.Parse(location)
			if err != nil {
				return false, nil
			}
			redirectURI := parsed.Query().Get("redirect_uri")
			if strings.Contains(redirectURI, hostnameB) {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			t.Logf("login redirect_uri does not contain hostname %s within %s (requires console PR #16686)", hostnameB, additionalRoutePollTimeout)
		}
	})
}

func TestAdditionalRouteLabelPropagation(t *testing.T) {
	client, _ := framework.StandardSetup(t)

	// The labels field on ComponentRouteSpec is gated behind the
	// IngressComponentRouteLabels feature gate. Check the cluster's
	// resolved feature gates to see if it's actually enabled — the
	// gate must be present in the FeatureGate status, not just
	// TechPreview being on (the CRD may predate the field).
	if !isFeatureGateEnabled(t, client, "IngressComponentRouteLabels") {
		t.Skip("IngressComponentRouteLabels feature gate not enabled, skipping label propagation test")
	}

	name := "console-e2e-labels"
	defer cleanupAdditionalRoutes(t, client, []string{name})

	domain := getClusterAppsDomain(t, client)
	hostname := additionalRouteHostname(name, domain)

	initialLabels := map[string]configv1.LabelValue{
		"ingress": "shard-test",
		"env":     "ci",
	}

	addComponentRoute(t, client, name, hostname, initialLabels)

	t.Run("LabelsApplied", func(t *testing.T) {
		err := wait.Poll(1*time.Second, additionalRoutePollTimeout, func() (bool, error) {
			route, err := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			if route.Labels["ingress"] != "shard-test" || route.Labels["env"] != "ci" {
				return false, nil
			}
			if route.Labels[routesub.AdditionalRouteLabel] != "true" {
				return false, nil
			}
			if route.Labels["app"] != "console" {
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			route, _ := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), name, metav1.GetOptions{})
			t.Fatalf("labels not propagated to route within %s, current labels: %v", additionalRoutePollTimeout, route.Labels)
		}
	})

	t.Run("LabelsUpdated", func(t *testing.T) {
		updatedLabels := map[string]configv1.LabelValue{
			"ingress": "shard-test",
			"tier":    "frontend",
		}
		updateComponentRouteLabels(t, client, name, updatedLabels)

		err := wait.Poll(1*time.Second, additionalRoutePollTimeout, func() (bool, error) {
			route, err := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			if route.Labels["ingress"] != "shard-test" {
				return false, nil
			}
			if route.Labels["tier"] != "frontend" {
				return false, nil
			}
			if _, hasEnv := route.Labels["env"]; hasEnv {
				return false, nil
			}
			if route.Labels[routesub.AdditionalRouteLabel] != "true" {
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			route, _ := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), name, metav1.GetOptions{})
			t.Fatalf("labels not reconciled on route within %s, current labels: %v", additionalRoutePollTimeout, route.Labels)
		}
	})
}

// --- Helpers ---

func getClusterAppsDomain(t *testing.T, client *framework.ClientSet) string {
	t.Helper()
	ingressConfig, err := client.Ingress.Ingresses().Get(context.TODO(), api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get ingress config: %v", err)
	}
	if ingressConfig.Spec.Domain == "" {
		t.Fatal("ingress config has empty domain")
	}
	return ingressConfig.Spec.Domain
}

func additionalRouteHostname(name, domain string) string {
	return fmt.Sprintf("%s-%s.%s", name, api.OpenShiftConsoleNamespace, domain)
}

func addComponentRoute(t *testing.T, client *framework.ClientSet, name, hostname string, labels map[string]configv1.LabelValue) {
	t.Helper()
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		ingressConfig, err := client.Ingress.Ingresses().Get(context.TODO(), api.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		spec := configv1.ComponentRouteSpec{
			Namespace: api.OpenShiftConsoleNamespace,
			Name:      name,
			Hostname:  configv1.Hostname(hostname),
		}
		if labels != nil {
			spec.Labels = labels
		}
		ingressConfig.Spec.ComponentRoutes = append(ingressConfig.Spec.ComponentRoutes, spec)
		_, err = client.Ingress.Ingresses().Update(context.TODO(), ingressConfig, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		t.Fatalf("failed to add componentRoute %s: %v", name, err)
	}
}

func removeComponentRoute(t *testing.T, client *framework.ClientSet, name string) {
	t.Helper()
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		ingressConfig, err := client.Ingress.Ingresses().Get(context.TODO(), api.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		var filtered []configv1.ComponentRouteSpec
		for _, cr := range ingressConfig.Spec.ComponentRoutes {
			if string(cr.Name) != name {
				filtered = append(filtered, cr)
			}
		}
		ingressConfig.Spec.ComponentRoutes = filtered
		_, err = client.Ingress.Ingresses().Update(context.TODO(), ingressConfig, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		t.Fatalf("failed to remove componentRoute %s: %v", name, err)
	}
}

func updateComponentRouteLabels(t *testing.T, client *framework.ClientSet, name string, labels map[string]configv1.LabelValue) {
	t.Helper()
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		ingressConfig, err := client.Ingress.Ingresses().Get(context.TODO(), api.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		for i, cr := range ingressConfig.Spec.ComponentRoutes {
			if string(cr.Name) == name {
				ingressConfig.Spec.ComponentRoutes[i].Labels = labels
				break
			}
		}
		_, err = client.Ingress.Ingresses().Update(context.TODO(), ingressConfig, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		t.Fatalf("failed to update labels on componentRoute %s: %v", name, err)
	}
}

func waitForAdditionalRoute(t *testing.T, client *framework.ClientSet, name, hostname string) {
	t.Helper()
	err := wait.Poll(1*time.Second, additionalRoutePollTimeout, func() (bool, error) {
		route, err := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if route.Spec.Host != hostname {
			return false, nil
		}
		if route.Labels[routesub.AdditionalRouteLabel] != "true" {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("route %s with hostname %s not created within %s", name, hostname, additionalRoutePollTimeout)
	}
}

func waitForAdditionalRouteRemoved(t *testing.T, client *framework.ClientSet, name string) {
	t.Helper()
	err := wait.Poll(1*time.Second, additionalRoutePollTimeout, func() (bool, error) {
		_, err := client.Routes.Routes(api.OpenShiftConsoleNamespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("route %s not removed within %s", name, additionalRoutePollTimeout)
	}
}

// checkOAuthRedirectURI verifies the OAuthClient has (or doesn't have) a redirect
// URI for the given hostname. On OIDC clusters, the OAuthClient doesn't exist
// because the operator skips OAuthClient management entirely — redirect URIs are
// registered externally on the OIDC provider (e.g. Keycloak). In that case, the
// check is skipped.
func checkOAuthRedirectURI(t *testing.T, client *framework.ClientSet, hostname string, shouldExist bool) {
	t.Helper()
	// On OIDC clusters the operator does not create or manage an OAuthClient.
	// If the OAuthClient doesn't exist, skip the check silently.
	_, err := client.OAuth.OAuthClients().Get(context.TODO(), api.OAuthClientName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		t.Logf("OAuthClient %s not found (OIDC mode), skipping redirect URI check for %s", api.OAuthClientName, hostname)
		return
	}

	expectedURI := fmt.Sprintf("https://%s/auth/callback", hostname)
	err = wait.Poll(2*time.Second, additionalRoutePollTimeout, func() (bool, error) {
		oauthClient, err := client.OAuth.OAuthClients().Get(context.TODO(), api.OAuthClientName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		found := false
		for _, uri := range oauthClient.RedirectURIs {
			if uri == expectedURI {
				found = true
				break
			}
		}
		return found == shouldExist, nil
	})
	if err != nil {
		if shouldExist {
			t.Errorf("OAuthClient %s missing redirect URI for %s within %s", api.OAuthClientName, hostname, additionalRoutePollTimeout)
		} else {
			t.Errorf("OAuthClient %s still has redirect URI for %s after %s", api.OAuthClientName, hostname, additionalRoutePollTimeout)
		}
	}
}

func checkConfigMapAdditionalHost(t *testing.T, client *framework.ClientSet, hostname string, shouldExist bool) {
	t.Helper()
	expectedAddr := fmt.Sprintf("https://%s", hostname)
	err := wait.Poll(2*time.Second, additionalRoutePollTimeout, func() (bool, error) {
		cm, err := client.Core.ConfigMaps(api.OpenShiftConsoleNamespace).Get(context.TODO(), api.OpenShiftConsoleConfigMapName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		configYAML, ok := cm.Data["console-config.yaml"]
		if !ok {
			return false, nil
		}
		found := strings.Contains(configYAML, expectedAddr)
		return found == shouldExist, nil
	})
	if err != nil {
		if shouldExist {
			t.Errorf("ConfigMap %s missing additionalConsoleBaseAddresses entry for %s within %s", api.OpenShiftConsoleConfigMapName, hostname, additionalRoutePollTimeout)
		} else {
			t.Errorf("ConfigMap %s still has additionalConsoleBaseAddresses entry for %s after %s", api.OpenShiftConsoleConfigMapName, hostname, additionalRoutePollTimeout)
		}
	}
}

// isFeatureGateEnabled checks whether a specific feature gate is enabled on the
// cluster by inspecting the FeatureGate status. This is more precise than
// framework.IsFeatureGateSet which only checks if TechPreview/DevPreview is active
// — a cluster can have TechPreview enabled but still lack a specific gate if
// the CRD or API predates the feature.
func isFeatureGateEnabled(t *testing.T, client *framework.ClientSet, gateName string) bool {
	t.Helper()
	fg, err := client.FeatureGate.FeatureGates().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get FeatureGate: %v", err)
	}
	for _, version := range fg.Status.FeatureGates {
		for _, enabled := range version.Enabled {
			if string(enabled.Name) == gateName {
				return true
			}
		}
	}
	return false
}

func cleanupAdditionalRoutes(t *testing.T, client *framework.ClientSet, names []string) {
	t.Helper()
	for _, name := range names {
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			ingressConfig, err := client.Ingress.Ingresses().Get(context.TODO(), api.ConfigResourceName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			var filtered []configv1.ComponentRouteSpec
			for _, cr := range ingressConfig.Spec.ComponentRoutes {
				if string(cr.Name) != name {
					filtered = append(filtered, cr)
				}
			}
			if len(filtered) == len(ingressConfig.Spec.ComponentRoutes) {
				return nil
			}
			ingressConfig.Spec.ComponentRoutes = filtered
			_, err = client.Ingress.Ingresses().Update(context.TODO(), ingressConfig, metav1.UpdateOptions{})
			return err
		})
		if err != nil {
			t.Logf("warning: failed to remove componentRoute %s during cleanup: %v", name, err)
		}
	}
	framework.StandardCleanup(t, client)
}
