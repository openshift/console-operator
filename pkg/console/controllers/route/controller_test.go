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
	noSANCertificate = `-----BEGIN CERTIFICATE-----
MIICrDCCAZQCCQDLyfaAArSAUTANBgkqhkiG9w0BAQsFADAXMRUwEwYDVQQDDAx4
eGlhX3Rlc3RfY2EwIBcNMjIwMjIxMTcwNTU2WhgPMjI5NTEyMDcxNzA1NTZaMBcx
FTATBgNVBAMMDHh4aWFfdGVzdF9jYTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCC
AQoCggEBAKcOSfl9dvOfcQcUl5k9vMzL1UhqHOgs3vmQIF0Ht4tzxSo9gB2rHMhq
uiOBtoA7BdhEA72Bp04Rx7sGrD0SntWgGEYBE3LaXiBgonKB+AwwC7svNxUVakW4
3BDSogktgUo2njclex+Bjm3sNVrZm7JYPJ27MqHhtPBqeO28HGHTLizeNIkz1A5u
vl3mzotQeacduCcLn3kFcT1hjzcHh89AqaMplHzJzeNnsIUMsYud0D3vxfhmoh17
gzU2LJTMGmRDPsoV1DwRnDHWIaxGcz8tVox0M3ocTHjoyvMwKv23JqinURVqHnVg
UZETxuNDrHEFaIhyWzFIOEAIaiJ9pO8CAwEAATANBgkqhkiG9w0BAQsFAAOCAQEA
pB+4nh22itJpBVJ4qgZ8HrI/t4PBW+gf+4XgnslfmvvE1tbuucADvu8b8LA1CwIa
+jqv9HM5jy2DdflwCOjMwkPK7TTq8WmaRM3x5Ys3mI0AMuZqXDch/A3tzVRKPdlo
wo1jCYynYDQF2bHgeAIMAHVGUSZmOBBMGPYKV+CptYGm42Lel/9awC2IToyj1grA
sbm/L8bzE4vkpMQPvcGnj2SEiEVFFAdKsrLe57HzKlJsaMH8Ow59CpYdQGqYTOew
6UiC6O+gjsEV6Q6xcCoOow05ANsjo7S0y9u8UhxwiEtOE4D479b3tKnJp5CnkbUJ
PSV0AWbZpYFnB2GMMXObPg==
-----END CERTIFICATE-----`
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
		{
			name: "Test custom cert secret with no SAN in certificate",
			args: args{
				secret: &corev1.Secret{
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						corev1.TLSCertKey:       []byte(noSANCertificate),
						corev1.TLSPrivateKeyKey: []byte(validKey),
					},
				},
			},
			want: want{
				customTLSCert: nil,
				err:           fmt.Errorf("failed to verify custom certificate PEM: custom TLS certificate has no SAN"),
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
