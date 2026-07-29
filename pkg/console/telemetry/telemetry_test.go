package telemetry

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/console-operator/pkg/api"
)

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
