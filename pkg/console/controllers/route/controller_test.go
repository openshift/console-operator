package route

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"testing"

	"github.com/go-test/deep"

	// k8s
	corev1 "k8s.io/api/core/v1"

	// console-operator
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
)

const (
	validCertificate = `-----BEGIN CERTIFICATE-----
MIICRzCCAfGgAwIBAgIJAIydTIADd+yqMA0GCSqGSIb3DQEBCwUAMH4xCzAJBgNV
BAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNVBAcMBkxvbmRvbjEYMBYGA1UE
CgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1JVCBEZXBhcnRtZW50MRswGQYD
VQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTIwIBcNMTcwNDI2MjMyNDU4WhgPMjExNzA0
MDIyMzI0NThaMH4xCzAJBgNVBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNV
BAcMBkxvbmRvbjEYMBYGA1UECgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1J
VCBEZXBhcnRtZW50MRswGQYDVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTIwXDANBgkq
hkiG9w0BAQEFAANLADBIAkEAuiRet28DV68Dk4A8eqCaqgXmymamUEjW/DxvIQqH
3lbhtm8BwSnS9wUAajSLSWiq3fci2RbRgaSPjUrnbOHCLQIDAQABo1AwTjAdBgNV
HQ4EFgQU0vhI4OPGEOqT+VAWwxdhVvcmgdIwHwYDVR0jBBgwFoAU0vhI4OPGEOqT
+VAWwxdhVvcmgdIwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAANBALNeJGDe
nV5cXbp9W1bC12Tc8nnNXn4ypLE2JTQAvyp51zoZ8hQoSnRVx/VCY55Yu+br8gQZ
+tW+O/PoE7B3tuY=
-----END CERTIFICATE-----`
	validKey = `-----BEGIN RSA PRIVATE KEY-----
MIIBVgIBADANBgkqhkiG9w0BAQEFAASCAUAwggE8AgEAAkEAuiRet28DV68Dk4A8
eqCaqgXmymamUEjW/DxvIQqH3lbhtm8BwSnS9wUAajSLSWiq3fci2RbRgaSPjUrn
bOHCLQIDAQABAkEArDR1g9IqD3aUImNikDgAngbzqpAokOGyMoxeavzpEaFOgCzi
gi7HF7yHRmZkUt8CzdEvnHSqRjFuaaB0gGA+AQIhAOc8Z1h8ElLRSqaZGgI3jCTp
Izx9HNY//U5NGrXD2+ttAiEAzhOqkqI4+nDab7FpiD7MXI6fO549mEXeVBPvPtsS
OcECIQCIfkpOm+ZBBpO3JXaJynoqK4gGI6ALA/ik6LSUiIlfPQIhAISjd9hlfZME
bDQT1r8Q3Gx+h9LRqQeHgPBQ3F5ylqqBAiBaJ0hkYvrIdWxNlcLqD3065bJpHQ4S
WQkuZUQN1M/Xvg==
-----END RSA PRIVATE KEY-----`
	invalidCertificate = `
-----BEGIN CERTIFICATE-----
MIIEBDCCAuygAwIBAgIDAjppMA0GCSqGSIb3DQEBBQUAMEIxCzAJBgNVBAYTAlVT
WHPbqCRiOwY1nQ2pM714A5AuTHhdUDqB1O6gyHA43LL5Z/qHQF1hwFGPa4NrzQU6
yuGnBXj8ytqU0CwIPX4WecigUCAkVDNx
-----END CERTIFICATE-----`
	invalidKey = `
-----BEGIN RSA PRIVATE KEY-----
MIIJKQIBAAKCAgEAw2jtDhf4X2W8182vtAiwXUk/Zr7mruiiFt3y4l7YRBXaazmI
eIWaEkvN9O90gL09Cx5jgq6mP1pjCzHsEFhnICziFd1r+M3cMeb4EAqwMZ84
-----END RSA PRIVATE KEY-----`
)

type certificateData struct {
	keyPEM         []byte
	certificatePEM []byte
	certificate    *tls.Certificate
}

func newCertificateData(certificatePEM string, keyPEM string) *certificateData {
	certificate, err := tls.X509KeyPair([]byte(certificatePEM), []byte(keyPEM))
	if err != nil {
		panic(fmt.Sprintf("Unable to initialize certificate: %v", err))
	}
	certs, err := x509.ParseCertificates(certificate.Certificate[0])
	if err != nil {
		panic(fmt.Sprintf("Unable to initialize certificate leaf: %v", err))
	}
	certificate.Leaf = certs[0]
	return &certificateData{
		keyPEM:         []byte(keyPEM),
		certificatePEM: []byte(certificatePEM),
		certificate:    &certificate,
	}
}

func TestValidateCustomCertSecret(t *testing.T) {
	type args struct {
		secret *corev1.Secret
	}
	type want struct {
		customTLSCert *routesub.CustomTLSCert
		err           error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Test valid custom cert secret",
			args: args{
				secret: &corev1.Secret{
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						corev1.TLSCertKey:       []byte(validCertificate),
						corev1.TLSPrivateKeyKey: []byte(validKey),
					},
				},
			},
			want: want{
				customTLSCert: &routesub.CustomTLSCert{
					Certificate: validCertificate,
					Key:         validKey,
				},
				err: nil,
			},
		},
		{
			name: "Test custom cert secret with invalid type",
			args: args{
				secret: &corev1.Secret{
					Type: corev1.SecretTypeSSHAuth,
					Data: map[string][]byte{
						corev1.TLSCertKey:       []byte(validCertificate),
						corev1.TLSPrivateKeyKey: []byte(validKey),
					},
				},
			},
			want: want{
				customTLSCert: nil,
				err:           fmt.Errorf("custom cert secret is not in %q type, instead uses %q type", corev1.SecretTypeTLS, corev1.SecretTypeSSHAuth),
			},
		},
		{
			name: "Test custom cert secret missing 'tls.key' data field",
			args: args{
				secret: &corev1.Secret{
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						corev1.TLSCertKey: []byte(validCertificate),
					},
				},
			},
			want: want{
				customTLSCert: nil,
				err:           fmt.Errorf("custom cert secret data doesn't contain '%s' entry", corev1.TLSPrivateKeyKey),
			},
		},
		{
			name: "Test custom cert secret missing 'tls.crt' data field",
			args: args{
				secret: &corev1.Secret{
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						corev1.TLSPrivateKeyKey: []byte(validKey),
					},
				},
			},
			want: want{
				customTLSCert: nil,
				err:           fmt.Errorf("custom cert secret data doesn't contain '%s' entry", corev1.TLSCertKey),
			},
		},
		{
			name: "Test custom cert secret with invalid TLS cert",
			args: args{
				secret: &corev1.Secret{
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						corev1.TLSCertKey:       []byte(invalidCertificate),
						corev1.TLSPrivateKeyKey: []byte(validKey),
					},
				},
			},
			want: want{
				customTLSCert: nil,
				err:           fmt.Errorf("failed to verify custom certificate PEM: asn1: syntax error: data truncated"),
			},
		},
		{
			name: "Test custom cert secret with invalid TLS key",
			args: args{
				secret: &corev1.Secret{
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						corev1.TLSCertKey:       []byte(validCertificate),
						corev1.TLSPrivateKeyKey: []byte(invalidKey),
					},
				},
			},
			want: want{
				customTLSCert: nil,
				err:           fmt.Errorf("failed to verify custom key PEM: block RSA PRIVATE KEY is not valid key PEM"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			customTLSCert, err := ValidateCustomCertSecret(tt.args.secret)
			if diff := deep.Equal(customTLSCert, tt.want.customTLSCert); diff != nil {
				t.Error(diff)
			}
			if diff := deep.Equal(err, tt.want.err); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestGetDefaultRouteHost(t *testing.T) {
	type args struct {
		ingressDomain string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test assembling linux amd64 specific URL",
			args: args{
				ingressDomain: "apps.devcluster.openshift.com",
			},
			want: "console-openshift-console.apps.devcluster.openshift.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(GetDefaultRouteHost(tt.args.ingressDomain), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}
