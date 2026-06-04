package route

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/library-go/pkg/crypto"
)

const (
	http2CertValidity      = 365 * 24 * time.Hour
	http2CertRenewalBuffer = 30 * 24 * time.Hour
)

// GenerateHTTP2Cert creates a TLS certificate for the given hostname to enable
// HTTP/2 on the console route. The cert's only purpose is to be unique in the
// router's cert_config.map so that per-cert ALPN negotiation kicks in.
//
// If ca is provided, the cert is signed by that CA (the ingress controller's
// CA, so the trust chain matches the wildcard cert). Otherwise a self-signed
// CA is created and used.
func GenerateHTTP2Cert(hostname string, ca *crypto.CA) (*CustomTLSCert, error) {
	if ca == nil {
		caConfig, err := crypto.MakeSelfSignedCAConfigForDuration("console-http2-ca", http2CertValidity)
		if err != nil {
			return nil, fmt.Errorf("failed to create self-signed CA: %w", err)
		}
		ca = &crypto.CA{
			SerialGenerator: &crypto.RandomSerialGenerator{},
			Config:          caConfig,
		}
	}

	certConfig, err := ca.MakeServerCertForDuration(sets.New(hostname), http2CertValidity)
	if err != nil {
		return nil, fmt.Errorf("failed to create server certificate: %w", err)
	}

	certPEM, keyPEM, err := certConfig.GetPEMBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to encode certificate PEM: %w", err)
	}

	return &CustomTLSCert{
		Certificate: string(certPEM),
		Key:         string(keyPEM),
	}, nil
}

// EnsureHTTP2Cert returns a TLS cert for HTTP/2 enablement, creating or
// regenerating one as needed. The cert is persisted in a Secret in the
// openshift-console namespace so it survives operator restarts without
// triggering unnecessary route updates.
func EnsureHTTP2Cert(ctx context.Context, secretClient corev1client.SecretsGetter, secretLister corev1listers.SecretLister, hostname string, ca *crypto.CA) (*CustomTLSCert, error) {
	existing, listerErr := secretLister.Secrets(api.OpenShiftConsoleNamespace).Get(api.ConsoleHTTP2CertSecretName)
	if listerErr != nil && !apierrors.IsNotFound(listerErr) {
		return nil, fmt.Errorf("failed to get HTTP/2 cert secret: %w", listerErr)
	}

	if listerErr == nil {
		if cert, valid := validHTTP2Cert(existing, hostname, ca); valid {
			return cert, nil
		}
		klog.V(4).Info("HTTP/2 cert needs regeneration")
	}

	newCert, err := GenerateHTTP2Cert(hostname, ca)
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.ConsoleHTTP2CertSecretName,
			Namespace: api.OpenShiftConsoleNamespace,
			Labels: map[string]string{
				"app": "console",
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte(newCert.Certificate),
			"tls.key": []byte(newCert.Key),
		},
	}

	if apierrors.IsNotFound(listerErr) {
		klog.V(2).Infof("Creating HTTP/2 cert secret %s/%s", api.OpenShiftConsoleNamespace, api.ConsoleHTTP2CertSecretName)
		_, err = secretClient.Secrets(api.OpenShiftConsoleNamespace).Create(ctx, secret, metav1.CreateOptions{})
	} else {
		secret.ResourceVersion = existing.ResourceVersion
		klog.V(2).Infof("Updating HTTP/2 cert secret %s/%s", api.OpenShiftConsoleNamespace, api.ConsoleHTTP2CertSecretName)
		_, err = secretClient.Secrets(api.OpenShiftConsoleNamespace).Update(ctx, secret, metav1.UpdateOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to persist HTTP/2 cert secret: %w", err)
	}

	return newCert, nil
}

// LoadCAFromSecret parses a kubernetes.io/tls Secret into a crypto.CA,
// suitable for signing serving certs.
func LoadCAFromSecret(secret *corev1.Secret) (*crypto.CA, error) {
	certPEM, ok := secret.Data["tls.crt"]
	if !ok {
		return nil, fmt.Errorf("secret missing tls.crt")
	}
	keyPEM, ok := secret.Data["tls.key"]
	if !ok {
		return nil, fmt.Errorf("secret missing tls.key")
	}
	return crypto.GetCAFromBytes(certPEM, keyPEM)
}

// validHTTP2Cert checks whether the cert in the Secret is still usable:
// not expired (with 30-day buffer), hostname matches, and issuer matches
// the current CA (verified cryptographically via SubjectKeyId).
func validHTTP2Cert(secret *corev1.Secret, hostname string, ca *crypto.CA) (*CustomTLSCert, bool) {
	certPEM, ok := secret.Data["tls.crt"]
	if !ok {
		return nil, false
	}
	keyPEM, ok := secret.Data["tls.key"]
	if !ok {
		return nil, false
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, false
	}

	if time.Now().Add(http2CertRenewalBuffer).After(cert.NotAfter) {
		return nil, false
	}
	if time.Now().Before(cert.NotBefore) {
		return nil, false
	}

	if err := cert.VerifyHostname(hostname); err != nil {
		return nil, false
	}

	if ca != nil && len(ca.Config.Certs) > 0 {
		if !bytes.Equal(cert.AuthorityKeyId, ca.Config.Certs[0].SubjectKeyId) {
			return nil, false
		}
	}

	if privateKeyVerifier(keyPEM) != nil {
		return nil, false
	}

	return &CustomTLSCert{
		Certificate: string(certPEM),
		Key:         string(keyPEM),
	}, true
}
