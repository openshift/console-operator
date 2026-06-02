package route

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kubefake "k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/library-go/pkg/crypto"
)

func TestGenerateHTTP2CertSelfSigned(t *testing.T) {
	hostname := "console-openshift-console.apps.example.com"
	cert, err := GenerateHTTP2Cert(hostname, nil)
	if err != nil {
		t.Fatalf("GenerateHTTP2Cert() error: %v", err)
	}
	if cert.Certificate == "" {
		t.Fatal("expected non-empty certificate")
	}
	if cert.Key == "" {
		t.Fatal("expected non-empty key")
	}

	parsed := parseCert(t, cert.Certificate)

	if err := parsed.VerifyHostname(hostname); err != nil {
		t.Errorf("certificate does not verify for hostname %q: %v", hostname, err)
	}
	if time.Now().After(parsed.NotAfter) {
		t.Error("certificate is already expired")
	}
	if time.Now().Before(parsed.NotBefore) {
		t.Error("certificate is not yet valid")
	}

	if privateKeyVerifier([]byte(cert.Key)) != nil {
		t.Fatal("key is not a valid private key")
	}
}

func TestGenerateHTTP2CertWithCA(t *testing.T) {
	ca := makeTestCA(t)
	hostname := "console-openshift-console.apps.example.com"

	cert, err := GenerateHTTP2Cert(hostname, ca)
	if err != nil {
		t.Fatalf("GenerateHTTP2Cert() error: %v", err)
	}

	parsed := parseCert(t, cert.Certificate)

	if err := parsed.VerifyHostname(hostname); err != nil {
		t.Errorf("certificate does not verify for hostname %q: %v", hostname, err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(ca.Config.Certs[0])
	if _, err := parsed.Verify(x509.VerifyOptions{Roots: roots}); err != nil {
		t.Errorf("certificate does not chain to CA: %v", err)
	}
}

func TestValidHTTP2Cert(t *testing.T) {
	hostname := "console-openshift-console.apps.example.com"

	cert, err := GenerateHTTP2Cert(hostname, nil)
	if err != nil {
		t.Fatalf("GenerateHTTP2Cert() error: %v", err)
	}
	secret := makeSecretFromCert(cert)

	tests := []struct {
		name      string
		secret    *corev1.Secret
		hostname  string
		ca        *crypto.CA
		wantValid bool
	}{
		{
			name:      "valid cert",
			secret:    secret,
			hostname:  hostname,
			wantValid: true,
		},
		{
			name:      "wrong hostname",
			secret:    secret,
			hostname:  "other.apps.example.com",
			wantValid: false,
		},
		{
			name:      "CA mismatch via SubjectKeyId",
			secret:    secret,
			hostname:  hostname,
			ca:        makeTestCA(t),
			wantValid: false,
		},
		{
			name:      "missing tls.crt",
			secret:    &corev1.Secret{Data: map[string][]byte{"tls.key": []byte("data")}},
			hostname:  hostname,
			wantValid: false,
		},
		{
			name:      "missing tls.key",
			secret:    &corev1.Secret{Data: map[string][]byte{"tls.crt": []byte("data")}},
			hostname:  hostname,
			wantValid: false,
		},
		{
			name:      "invalid PEM",
			secret:    &corev1.Secret{Data: map[string][]byte{"tls.crt": []byte("not-pem"), "tls.key": []byte("not-pem")}},
			hostname:  hostname,
			wantValid: false,
		},
		{
			name: "corrupt key",
			secret: &corev1.Secret{Data: map[string][]byte{
				"tls.crt": []byte(cert.Certificate),
				"tls.key": []byte("-----BEGIN RSA PRIVATE KEY-----\nbaddata\n-----END RSA PRIVATE KEY-----\n"),
			}},
			hostname:  hostname,
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, valid := validHTTP2Cert(tt.secret, tt.hostname, tt.ca)
			if valid != tt.wantValid {
				t.Errorf("validHTTP2Cert() valid = %v, want %v", valid, tt.wantValid)
			}
			if tt.wantValid && result == nil {
				t.Error("expected non-nil cert when valid")
			}
		})
	}

	t.Run("CA signed cert validates with same CA", func(t *testing.T) {
		ca := makeTestCA(t)
		caCert, err := GenerateHTTP2Cert(hostname, ca)
		if err != nil {
			t.Fatalf("GenerateHTTP2Cert() error: %v", err)
		}
		caSecret := makeSecretFromCert(caCert)
		result, valid := validHTTP2Cert(caSecret, hostname, ca)
		if !valid {
			t.Error("expected CA-signed cert to be valid with same CA")
		}
		if result == nil {
			t.Error("expected non-nil cert")
		}
	})
}

func TestEnsureHTTP2Cert_CreatePath(t *testing.T) {
	hostname := "console-openshift-console.apps.example.com"
	fakeClient := kubefake.NewSimpleClientset()
	lister := newFakeSecretLister(t)

	cert, err := EnsureHTTP2Cert(context.Background(), fakeClient.CoreV1(), lister, hostname, nil)
	if err != nil {
		t.Fatalf("EnsureHTTP2Cert() error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected non-nil cert")
	}
	if cert.Certificate == "" || cert.Key == "" {
		t.Fatal("expected non-empty cert and key")
	}

	created, err := fakeClient.CoreV1().Secrets(api.OpenShiftConsoleNamespace).Get(context.Background(), api.ConsoleHTTP2CertSecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected secret to be created: %v", err)
	}
	if created.Type != corev1.SecretTypeTLS {
		t.Errorf("expected secret type %s, got %s", corev1.SecretTypeTLS, created.Type)
	}
	if len(created.Data["tls.crt"]) == 0 || len(created.Data["tls.key"]) == 0 {
		t.Error("expected non-empty cert data in secret")
	}
	if created.Labels["app"] != "console" {
		t.Errorf("expected app=console label, got %v", created.Labels)
	}
}

func TestEnsureHTTP2Cert_UpdatePath(t *testing.T) {
	hostname := "console-openshift-console.apps.example.com"

	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            api.ConsoleHTTP2CertSecretName,
			Namespace:       api.OpenShiftConsoleNamespace,
			ResourceVersion: "123",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("old-invalid-cert"),
			"tls.key": []byte("old-invalid-key"),
		},
	}

	fakeClient := kubefake.NewSimpleClientset(existingSecret)
	lister := newFakeSecretLister(t, existingSecret)

	cert, err := EnsureHTTP2Cert(context.Background(), fakeClient.CoreV1(), lister, hostname, nil)
	if err != nil {
		t.Fatalf("EnsureHTTP2Cert() error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected non-nil cert")
	}

	updated, err := fakeClient.CoreV1().Secrets(api.OpenShiftConsoleNamespace).Get(context.Background(), api.ConsoleHTTP2CertSecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected secret to exist: %v", err)
	}
	if string(updated.Data["tls.crt"]) == "old-invalid-cert" {
		t.Error("expected secret to be updated with new cert")
	}

	parseCert(t, string(updated.Data["tls.crt"]))
}

func TestEnsureHTTP2Cert_ExistingValidCert(t *testing.T) {
	hostname := "console-openshift-console.apps.example.com"

	validCert, err := GenerateHTTP2Cert(hostname, nil)
	if err != nil {
		t.Fatalf("GenerateHTTP2Cert() error: %v", err)
	}

	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            api.ConsoleHTTP2CertSecretName,
			Namespace:       api.OpenShiftConsoleNamespace,
			ResourceVersion: "123",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte(validCert.Certificate),
			"tls.key": []byte(validCert.Key),
		},
	}

	fakeClient := kubefake.NewSimpleClientset(existingSecret)
	lister := newFakeSecretLister(t, existingSecret)

	cert, err := EnsureHTTP2Cert(context.Background(), fakeClient.CoreV1(), lister, hostname, nil)
	if err != nil {
		t.Fatalf("EnsureHTTP2Cert() error: %v", err)
	}
	if cert.Certificate != validCert.Certificate {
		t.Error("expected to return existing valid cert, not generate a new one")
	}
	if cert.Key != validCert.Key {
		t.Error("expected to return existing valid key, not generate a new one")
	}
}

func TestLoadCAFromSecret(t *testing.T) {
	t.Run("valid CA secret", func(t *testing.T) {
		ca := makeTestCA(t)
		certPEM, keyPEM, err := ca.Config.GetPEMBytes()
		if err != nil {
			t.Fatalf("failed to get CA PEM bytes: %v", err)
		}
		secret := makeSecretFromCert(&CustomTLSCert{
			Certificate: string(certPEM),
			Key:         string(keyPEM),
		})
		loaded, err := LoadCAFromSecret(secret)
		if err != nil {
			t.Fatalf("LoadCAFromSecret() error: %v", err)
		}
		if loaded.Config.Certs[0].Subject.CommonName != "test-ca" {
			t.Errorf("expected CN=test-ca, got CN=%s", loaded.Config.Certs[0].Subject.CommonName)
		}
	})

	t.Run("missing tls.crt", func(t *testing.T) {
		secret := &corev1.Secret{Data: map[string][]byte{"tls.key": []byte("data")}}
		_, err := LoadCAFromSecret(secret)
		if err == nil {
			t.Error("expected error for missing tls.crt")
		}
	})

	t.Run("missing tls.key", func(t *testing.T) {
		secret := &corev1.Secret{Data: map[string][]byte{"tls.crt": []byte("data")}}
		_, err := LoadCAFromSecret(secret)
		if err == nil {
			t.Error("expected error for missing tls.key")
		}
	})

	t.Run("invalid PEM data", func(t *testing.T) {
		secret := &corev1.Secret{Data: map[string][]byte{
			"tls.crt": []byte("not-valid-pem"),
			"tls.key": []byte("not-valid-pem"),
		}}
		_, err := LoadCAFromSecret(secret)
		if err == nil {
			t.Error("expected error for invalid PEM")
		}
	})
}

func TestValidHTTP2Cert_ExpiredCert(t *testing.T) {
	hostname := "console-openshift-console.apps.example.com"
	ca := makeTestCA(t)

	certConfig, err := ca.MakeServerCertForDuration(sets.New(hostname), 29*24*time.Hour)
	if err != nil {
		t.Fatalf("MakeServerCertForDuration() error: %v", err)
	}
	certPEM, keyPEM, err := certConfig.GetPEMBytes()
	if err != nil {
		t.Fatalf("GetPEMBytes() error: %v", err)
	}
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": keyPEM,
		},
	}

	_, valid := validHTTP2Cert(secret, hostname, nil)
	if valid {
		t.Error("expected cert within 30-day renewal buffer to be invalid")
	}
}

func TestEnsureHTTP2Cert_CARotation(t *testing.T) {
	hostname := "console-openshift-console.apps.example.com"
	ca1 := makeTestCA(t)
	ca2 := makeTestCA(t)

	certSignedByCA1, err := GenerateHTTP2Cert(hostname, ca1)
	if err != nil {
		t.Fatalf("GenerateHTTP2Cert() error: %v", err)
	}

	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            api.ConsoleHTTP2CertSecretName,
			Namespace:       api.OpenShiftConsoleNamespace,
			ResourceVersion: "100",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte(certSignedByCA1.Certificate),
			"tls.key": []byte(certSignedByCA1.Key),
		},
	}

	fakeClient := kubefake.NewSimpleClientset(existingSecret)
	lister := newFakeSecretLister(t, existingSecret)

	newCert, err := EnsureHTTP2Cert(context.Background(), fakeClient.CoreV1(), lister, hostname, ca2)
	if err != nil {
		t.Fatalf("EnsureHTTP2Cert() error: %v", err)
	}
	if newCert.Certificate == certSignedByCA1.Certificate {
		t.Error("expected cert to be regenerated when CA changed")
	}

	parsed := parseCert(t, newCert.Certificate)
	roots := x509.NewCertPool()
	roots.AddCert(ca2.Config.Certs[0])
	if _, err := parsed.Verify(x509.VerifyOptions{Roots: roots}); err != nil {
		t.Errorf("new cert does not chain to CA-2: %v", err)
	}

	updated, err := fakeClient.CoreV1().Secrets(api.OpenShiftConsoleNamespace).Get(context.Background(), api.ConsoleHTTP2CertSecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected secret to exist: %v", err)
	}
	if string(updated.Data["tls.crt"]) == certSignedByCA1.Certificate {
		t.Error("expected secret to be updated with new cert")
	}
}

func TestEnsureHTTP2Cert_HostnameChange(t *testing.T) {
	hostnameA := "console-a.apps.example.com"
	hostnameB := "console-b.apps.example.com"

	certForA, err := GenerateHTTP2Cert(hostnameA, nil)
	if err != nil {
		t.Fatalf("GenerateHTTP2Cert() error: %v", err)
	}

	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            api.ConsoleHTTP2CertSecretName,
			Namespace:       api.OpenShiftConsoleNamespace,
			ResourceVersion: "100",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte(certForA.Certificate),
			"tls.key": []byte(certForA.Key),
		},
	}

	fakeClient := kubefake.NewSimpleClientset(existingSecret)
	lister := newFakeSecretLister(t, existingSecret)

	newCert, err := EnsureHTTP2Cert(context.Background(), fakeClient.CoreV1(), lister, hostnameB, nil)
	if err != nil {
		t.Fatalf("EnsureHTTP2Cert() error: %v", err)
	}
	if newCert.Certificate == certForA.Certificate {
		t.Error("expected cert to be regenerated for new hostname")
	}

	parsed := parseCert(t, newCert.Certificate)
	if err := parsed.VerifyHostname(hostnameB); err != nil {
		t.Errorf("new cert does not verify for hostname %q: %v", hostnameB, err)
	}
	if err := parsed.VerifyHostname(hostnameA); err == nil {
		t.Error("new cert should not verify for old hostname")
	}

	updated, err := fakeClient.CoreV1().Secrets(api.OpenShiftConsoleNamespace).Get(context.Background(), api.ConsoleHTTP2CertSecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected secret to exist: %v", err)
	}
	if string(updated.Data["tls.crt"]) == certForA.Certificate {
		t.Error("expected secret to be updated with new cert")
	}
}

func makeTestCA(t *testing.T) *crypto.CA {
	t.Helper()
	caConfig, err := crypto.MakeSelfSignedCAConfigForDuration("test-ca", 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to create test CA: %v", err)
	}
	return &crypto.CA{
		SerialGenerator: &crypto.RandomSerialGenerator{},
		Config:          caConfig,
	}
}

func makeSecretFromCert(cert *CustomTLSCert) *corev1.Secret {
	return &corev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte(cert.Certificate),
			"tls.key": []byte(cert.Key),
		},
	}
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

func parseCert(t *testing.T, certPEM string) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		t.Fatal("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}
	return cert
}
