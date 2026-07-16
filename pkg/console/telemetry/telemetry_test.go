package telemetry

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/console-operator/pkg/api"
)

// Helpers to set rate-limit timestamps and restore after test
func withLastAttemptTime(t *testing.T, ts time.Time) {
	prev := lastAttemptTime
	lastAttemptTime = ts
	t.Cleanup(func() { lastAttemptTime = prev })
}

func withLastSuccessTime(t *testing.T, ts time.Time) {
	prev := lastSuccessTime
	lastSuccessTime = ts
	t.Cleanup(func() { lastSuccessTime = prev })
}

func newFakeSecretLister(t *testing.T, secrets ...*corev1.Secret) corev1listers.SecretLister {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, s := range secrets {
		if err := indexer.Add(s.DeepCopy()); err != nil {
			t.Fatalf("failed to add secret to indexer: %v", err)
		}
	}
	return corev1listers.NewSecretLister(indexer)
}

func TestGetAccessToken_MissingCloudEntry(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: PullSecretName, Namespace: api.OpenShiftConfigNamespace},
		Data:       map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)},
	}
	lister := newFakeSecretLister(t, secret)

	token, err := GetAccessToken(lister)
	if err != nil {
		t.Fatalf("expected no error for missing cloud.openshift.com, got: %v", err)
	}
	if token != "" {
		t.Fatalf("expected empty token, got %q", token)
	}
}

func TestGetAccessToken_PresentCloudEntry(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: PullSecretName, Namespace: api.OpenShiftConfigNamespace},
		Data:       map[string][]byte{".dockerconfigjson": []byte(`{"auths":{"cloud.openshift.com":{"auth":"my-token"}}}`)},
	}
	lister := newFakeSecretLister(t, secret)

	token, err := GetAccessToken(lister)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "my-token" {
		t.Fatalf("expected %q, got %q", "my-token", token)
	}
}

func TestGetOrganizationMeta_UsesCustomOverrides(t *testing.T) {
	telemetryConfig := map[string]string{
		"ORGANIZATION_ID": "org-custom",
		"ACCOUNT_MAIL":    "user@example.com",
	}
	orgID, account, refresh := GetOrganizationMeta(telemetryConfig, "", "", "cluster-id", "token")

	if orgID != "org-custom" || account != "user@example.com" {
		t.Fatalf("expected custom values, got orgID=%q account=%q", orgID, account)
	}
	if refresh {
		t.Fatalf("expected refresh=false when using custom overrides")
	}
}

func TestGetOrganizationMeta_UsesCachedWhenAvailable(t *testing.T) {
	telemetryConfig := map[string]string{}
	orgID, account, refresh := GetOrganizationMeta(telemetryConfig, "org-cached", "cached@example.com", "cluster-id", "token")

	if orgID != "org-cached" || account != "cached@example.com" {
		t.Fatalf("expected cached values, got orgID=%q account=%q", orgID, account)
	}
	if refresh {
		t.Fatalf("expected refresh=false when using cached values")
	}
}

func TestGetOrganizationMeta_SkipsFetch_WhenSuccessRecent_OneMissing(t *testing.T) {
	telemetryConfig := map[string]string{}

	// Emulate recent successful fetch within 24h
	withLastSuccessTime(t, time.Now())

	orgID, account, refresh := GetOrganizationMeta(telemetryConfig, "org-cached", "", "cluster-id", "token")

	if orgID != "org-cached" || account != "" {
		t.Fatalf("expected to return cached org and empty account without fetching, got orgID=%q account=%q", orgID, account)
	}
	if refresh {
		t.Fatalf("expected refresh=false when fetch is rate-limited by last success")
	}
}

func TestGetOrganizationMeta_SkipsFetch_WhenAttemptRecent_NoCache(t *testing.T) {
	telemetryConfig := map[string]string{}

	// Emulate recent attempt within backoff window (no success)
	withLastAttemptTime(t, time.Now())

	orgID, account, refresh := GetOrganizationMeta(telemetryConfig, "", "", "cluster-id", "token")

	if orgID != "" || account != "" {
		t.Fatalf("expected empty values when no cache and fetch is rate-limited by last attempt, got orgID=%q account=%q", orgID, account)
	}
	if refresh {
		t.Fatalf("expected refresh=false when fetch is rate-limited by last attempt")
	}
}
