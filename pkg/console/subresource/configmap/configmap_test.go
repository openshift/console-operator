package configmap

import (
	"fmt"
	"sort"
	"strconv"
	"testing"

	yaml "gopkg.in/yaml.v2"

	"github.com/go-test/deep"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	configv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
)

const (
	host               = "localhost"
	customHostname     = "custom-route-hostname.openshift.com"
	mockAPIServer      = "https://api.some.cluster.openshift.com:6443"
	mockConsoleURL     = "https://console-openshift-console.apps.some.cluster.openshift.com"
	configKey          = "console-config.yaml"
	mockOperatorDocURL = "https://operator.config/doc/link/"
	testReleaseVersion = "testReleaseVersion"
	test               = 123

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
)

// To manually run these tests: go test -v ./pkg/console/subresource/configmap/...
func TestDefaultConfigMap(t *testing.T) {
	type args struct {
		operatorConfig               *operatorv1.Console
		authConfig                   *configv1.Authentication
		consoleConfig                *configv1.Console
		managedConfig                *corev1.ConfigMap
		monitoringSharedConfig       *corev1.ConfigMap
		authServerCAConfig           *corev1.ConfigMap
		infrastructureConfig         *configv1.Infrastructure
		rt                           *routev1.Route
		inactivityTimeoutSeconds     int
		availablePlugins             []*consolev1.ConsolePlugin
		nodeArchitectures            []string
		nodeOperatingSystems         []string
		copiedCSVsDisabled           bool
		contentSecurityPolicyEnabled bool
		telemetryConfig              map[string]string
	}
	t.Setenv("OPERATOR_IMAGE_VERSION", testReleaseVersion)
	tests := []struct {
		name string
		args args
		want *corev1.ConfigMap
	}{
		{
			name: "Test default configmap, no customization",
			args: args{
				authConfig:     &configv1.Authentication{},
				operatorConfig: &operatorv1.Console{},
				consoleConfig:  &configv1.Console{},
				managedConfig:  &corev1.ConfigMap{},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL:         mockAPIServer,
						ControlPlaneTopology: configv1.HighlyAvailableTopologyMode,
					},
				},
				rt: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleName,
					},
					Spec: routev1.RouteSpec{
						Host: host,
					},
				},
				inactivityTimeoutSeconds: 0,
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1",
						Kind:       "Console",
						Controller: ptr.To(true),
					}},
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
  controlPlaneTopology: HighlyAvailable
  releaseVersion: ` + testReleaseVersion + `
session: {}
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
  perspectives:
    - id: dev
      visibility:
        state: Disabled
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers: {}
`,
				},
			},
		},
		{
			name: "Test configmap with oauth-serving-cert",
			args: args{
				authConfig:     &configv1.Authentication{},
				operatorConfig: &operatorv1.Console{},
				consoleConfig:  &configv1.Console{},
				managedConfig:  &corev1.ConfigMap{},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL: mockAPIServer,
					},
				},
				rt: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleName,
					},
					Spec: routev1.RouteSpec{
						Host: host,
					},
				},
				inactivityTimeoutSeconds: 0,
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1",
						Kind:       "Console",
						Controller: ptr.To(true),
					}},
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
  releaseVersion: ` + testReleaseVersion + `
session: {}
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
  perspectives:
    - id: dev
      visibility:
        state: Disabled
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers: {}
`,
				},
			},
		},
		{
			name: "Test managed config to override default config",
			args: args{
				authConfig:     &configv1.Authentication{},
				operatorConfig: &operatorv1.Console{},
				consoleConfig:  &configv1.Console{},
				managedConfig: &corev1.ConfigMap{
					Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
session: {}
customization:
  branding: online
  documentationBaseURL: https://docs.okd.io/4.4/
`,
					},
				},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL: mockAPIServer,
					},
				},
				rt: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleName,
					},
					Spec: routev1.RouteSpec{
						Host: host,
					},
				},
				inactivityTimeoutSeconds: 0,
				nodeArchitectures:        []string{"amd64", "arm64"},
				telemetryConfig: map[string]string{
					"telemetry.console.openshift.io/TELEMETER_CLIENT_DISABLED": "true",
				},
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1",
						Kind:       "Console",
						Controller: ptr.To(true),
					}},
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
  releaseVersion: ` + testReleaseVersion + `
  nodeArchitectures:
  - amd64
  - arm64
session: {}
customization:
  branding: online
  documentationBaseURL: https://docs.okd.io/4.4/
  perspectives:
    - id: dev
      visibility:
        state: Disabled
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers: {}
telemetry:
  telemetry.console.openshift.io/TELEMETER_CLIENT_DISABLED: "true"
`,
				},
			},
		},
		{
			name: "Test nodeOperatingSystems config",
			args: args{
				authConfig:     &configv1.Authentication{},
				operatorConfig: &operatorv1.Console{},
				consoleConfig:  &configv1.Console{},
				managedConfig: &corev1.ConfigMap{
					Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
session: {}
customization:
  branding: online
  documentationBaseURL: https://docs.okd.io/4.4/
`,
					},
				},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL: mockAPIServer,
					},
				},
				rt: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleName,
					},
					Spec: routev1.RouteSpec{
						Host: host,
					},
				},
				inactivityTimeoutSeconds: 0,
				nodeOperatingSystems:     []string{"foo", "bar"},
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1",
						Kind:       "Console",
						Controller: ptr.To(true),
					}},
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
  releaseVersion: ` + testReleaseVersion + `
  nodeOperatingSystems:
  - foo
  - bar
session: {}
customization:
  branding: online
  documentationBaseURL: https://docs.okd.io/4.4/
  perspectives:
    - id: dev
      visibility:
        state: Disabled
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers: {}
`,
				},
			},
		},
		{
			name: "Test operator config overriding default config and managed config",
			args: args{
				authConfig: &configv1.Authentication{},
				operatorConfig: &operatorv1.Console{
					Spec: operatorv1.ConsoleSpec{
						OperatorSpec: operatorv1.OperatorSpec{},
						Customization: operatorv1.ConsoleCustomization{
							Brand:                operatorv1.BrandDedicated,
							DocumentationBaseURL: mockOperatorDocURL,
						},
					},
					Status: operatorv1.ConsoleStatus{},
				},
				consoleConfig: &configv1.Console{},
				managedConfig: &corev1.ConfigMap{
					Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
session: {}
customization:
  branding: online
  documentationBaseURL: https://docs.okd.io/4.4/
`,
					},
				},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL: mockAPIServer,
					},
				},
				rt: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleName,
					},
					Spec: routev1.RouteSpec{
						Host: host,
					},
				},
				inactivityTimeoutSeconds: 0,
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1",
						Kind:       "Console",
						Controller: ptr.To(true),
					}},
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
  releaseVersion: ` + testReleaseVersion + `
session: {}
customization:
  branding: ` + string(operatorv1.BrandDedicatedLegacy) + `
  documentationBaseURL: ` + mockOperatorDocURL + `
  perspectives:
    - id: dev
      visibility:
        state: Disabled
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers: {}
`,
				},
			},
		},
		// TODO remove deprecated CustomLogoFile API
		{
			name: "Test operator config with custom branding values (CustomLogoFile)",
			args: args{
				authConfig: &configv1.Authentication{},
				operatorConfig: &operatorv1.Console{
					Spec: operatorv1.ConsoleSpec{
						OperatorSpec: operatorv1.OperatorSpec{},
						Customization: operatorv1.ConsoleCustomization{
							Brand:                operatorv1.BrandDedicated,
							DocumentationBaseURL: mockOperatorDocURL,
							CustomProductName:    "custom-product-name",
							CustomLogoFile: configv1.ConfigMapFileReference{
								Name: "custom-logo-file",
								Key:  "logo.svg",
							},
						},
					},
					Status: operatorv1.ConsoleStatus{},
				},
				consoleConfig: &configv1.Console{},
				managedConfig: &corev1.ConfigMap{
					Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
session: {}
customization:
  branding: online
  documentationBaseURL: https://docs.okd.io/4.4/
`,
					},
				},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL: mockAPIServer,
					},
				},
				rt: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleName,
					},
					Spec: routev1.RouteSpec{
						Host: host,
					},
				},
				inactivityTimeoutSeconds: 0,
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1",
						Kind:       "Console",
						Controller: ptr.To(true),
					}},
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
  releaseVersion: ` + testReleaseVersion + `
session: {}
customization:
  branding: ` + string(operatorv1.BrandDedicatedLegacy) + `
  documentationBaseURL: ` + mockOperatorDocURL + `
  customProductName: custom-product-name
  logos:
    - themes:
        - mode: ` + string(operatorv1.ThemeModeDark) + `
          source:
            from: ConfigMap
            configmap:
              name: custom-logo-file
              key: logo.svg
        - mode: ` + string(operatorv1.ThemeModeLight) + `
          source:
            from: ConfigMap
            configmap:
              name: custom-logo-file
              key: logo.svg
      type: ` + string(operatorv1.LogoTypeMasthead) + `
  perspectives:
    - id: dev
      visibility:
        state: Disabled
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers: {}
`,
				},
			},
		},
		{
			name: "Test operator config with custom branding values (Logos)",
			args: args{
				authConfig: &configv1.Authentication{},
				operatorConfig: &operatorv1.Console{
					Spec: operatorv1.ConsoleSpec{
						OperatorSpec: operatorv1.OperatorSpec{},
						Customization: operatorv1.ConsoleCustomization{
							Brand:                operatorv1.BrandDedicated,
							DocumentationBaseURL: mockOperatorDocURL,
							CustomProductName:    "custom-product-name",
							Logos: []operatorv1.Logo{
								{
									Type: operatorv1.LogoTypeMasthead,
									Themes: []operatorv1.Theme{
										{
											Mode: operatorv1.ThemeModeDark,
											Source: operatorv1.FileReferenceSource{
												From: "ConfigMap",
												ConfigMap: &operatorv1.ConfigMapFileReference{
													Name: "masthead-dark",
													Key:  "masthead-dark-logo.png",
												},
											},
										},
										{
											Mode: operatorv1.ThemeModeLight,
											Source: operatorv1.FileReferenceSource{
												From: "ConfigMap",
												ConfigMap: &operatorv1.ConfigMapFileReference{
													Name: "masthead-light",
													Key:  "masthead-light-logo.png",
												},
											},
										},
									},
								},
								{
									Type: operatorv1.LogoTypeFavicon,
									Themes: []operatorv1.Theme{
										{
											Mode: operatorv1.ThemeModeDark,
											Source: operatorv1.FileReferenceSource{
												From: "ConfigMap",
												ConfigMap: &operatorv1.ConfigMapFileReference{
													Name: "favicon-dark",
													Key:  "favicon-dark-logo.png",
												},
											},
										},
										{
											Mode: operatorv1.ThemeModeLight,
											Source: operatorv1.FileReferenceSource{
												From: "ConfigMap",
												ConfigMap: &operatorv1.ConfigMapFileReference{
													Name: "favicon-light",
													Key:  "favicon-light-logo.png",
												},
											},
										},
									},
								},
							},
						},
					},
					Status: operatorv1.ConsoleStatus{},
				},
				consoleConfig: &configv1.Console{},
				managedConfig: &corev1.ConfigMap{
					Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
session: {}
customization:
  branding: online
  documentationBaseURL: https://docs.okd.io/4.4/
`,
					},
				},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL: mockAPIServer,
					},
				},
				rt: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleName,
					},
					Spec: routev1.RouteSpec{
						Host: host,
					},
				},
				inactivityTimeoutSeconds: 0,
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1",
						Kind:       "Console",
						Controller: ptr.To(true),
					}},
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
  releaseVersion: ` + testReleaseVersion + `
session: {}
customization:
  branding: ` + string(operatorv1.BrandDedicatedLegacy) + `
  documentationBaseURL: ` + mockOperatorDocURL + `
  customProductName: custom-product-name
  logos:
    - themes:
        - mode: ` + string(operatorv1.ThemeModeDark) + `
          source:
            from: ConfigMap
            configmap:
              name: masthead-dark
              key: masthead-dark-logo.png
        - mode: ` + string(operatorv1.ThemeModeLight) + `
          source:
            from: ConfigMap
            configmap:
              name: masthead-light
              key: masthead-light-logo.png
      type: ` + string(operatorv1.LogoTypeMasthead) + `
    - themes:
        - mode: ` + string(operatorv1.ThemeModeDark) + `
          source:
            from: ConfigMap
            configmap:
              name: favicon-dark
              key: favicon-dark-logo.png
        - mode: ` + string(operatorv1.ThemeModeLight) + `
          source:
            from: ConfigMap
            configmap:
              name: favicon-light
              key: favicon-light-logo.png
      type: ` + string(operatorv1.LogoTypeFavicon) + `
  perspectives:
    - id: dev
      visibility:
        state: Disabled
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers: {}
`,
				},
			},
		},
		{
			name: "Test operator config with Statuspage pageID",
			args: args{
				authConfig: &configv1.Authentication{},
				operatorConfig: &operatorv1.Console{
					Spec: operatorv1.ConsoleSpec{
						OperatorSpec: operatorv1.OperatorSpec{},
						Customization: operatorv1.ConsoleCustomization{
							Brand:                operatorv1.BrandDedicated,
							DocumentationBaseURL: mockOperatorDocURL,
						},
						Providers: operatorv1.ConsoleProviders{
							Statuspage: &operatorv1.StatuspageProvider{
								PageID: "id-1234",
							},
						},
					},
					Status: operatorv1.ConsoleStatus{},
				},
				consoleConfig: &configv1.Console{},
				managedConfig: &corev1.ConfigMap{
					Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
session: {}
customization:
  branding: online
  documentationBaseURL: https://docs.okd.io/4.4/
`,
					},
				},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL: mockAPIServer,
					},
				},
				rt: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleName,
					},
					Spec: routev1.RouteSpec{
						Host: host,
					},
				},
				inactivityTimeoutSeconds: 0,
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1",
						Kind:       "Console",
						Controller: ptr.To(true),
					}},
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
  releaseVersion: ` + testReleaseVersion + `
session: {}
customization:
  branding: ` + string(operatorv1.BrandDedicatedLegacy) + `
  documentationBaseURL: ` + mockOperatorDocURL + `
  perspectives:
    - id: dev
      visibility:
        state: Disabled
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers:
  statuspageID: id-1234
`,
				},
			},
		},
		{
			name: "Test operator config with custom route hostname",
			args: args{
				authConfig: &configv1.Authentication{},
				operatorConfig: &operatorv1.Console{
					Spec: operatorv1.ConsoleSpec{
						Route: operatorv1.ConsoleConfigRoute{
							Hostname: customHostname,
						},
					},
				},
				consoleConfig: &configv1.Console{},
				managedConfig: &corev1.ConfigMap{},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL: mockAPIServer,
					},
				},
				rt: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenshiftConsoleCustomRouteName,
					},
					Spec: routev1.RouteSpec{
						Host: customHostname,
					},
				},
				inactivityTimeoutSeconds: 0,
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1",
						Kind:       "Console",
						Controller: ptr.To(true),
					}},
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + customHostname + `
  masterPublicURL: ` + mockAPIServer + `
  releaseVersion: ` + testReleaseVersion + `
session: {}
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
  perspectives:
    - id: dev
      visibility:
        state: Disabled
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
  redirectPort: ` + strconv.Itoa(api.RedirectContainerPort) + `
providers: {}
`,
				},
			},
		},
		{
			name: "Test operator config, with inactivityTimeoutSeconds set",
			args: args{
				authConfig:     &configv1.Authentication{},
				operatorConfig: &operatorv1.Console{},
				consoleConfig:  &configv1.Console{},
				managedConfig:  &corev1.ConfigMap{},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL: mockAPIServer,
					},
				},
				rt: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleName,
					},
					Spec: routev1.RouteSpec{
						Host: host,
					},
				},
				inactivityTimeoutSeconds: 60,
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1",
						Kind:       "Console",
						Controller: ptr.To(true),
					}},
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  inactivityTimeoutSeconds: 60
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
  releaseVersion: ` + testReleaseVersion + `
session: {}
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
  perspectives:
    - id: dev
      visibility:
        state: Disabled
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers: {}
`,
				},
			},
		},
		{
			name: "Test operator config, with enabledPlugins set",
			args: args{
				authConfig:     &configv1.Authentication{},
				operatorConfig: &operatorv1.Console{},
				consoleConfig:  &configv1.Console{},
				managedConfig:  &corev1.ConfigMap{},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL: mockAPIServer,
					},
				},
				rt: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleName,
					},
					Spec: routev1.RouteSpec{
						Host: host,
					},
				},
				inactivityTimeoutSeconds: 0,
				availablePlugins: []*consolev1.ConsolePlugin{
					testPluginsWithProxy("plugin1", "service1", "service-namespace1"),
					testPluginsWithProxy("plugin2", "service2", "service-namespace2"),
					testPluginsWithI18nPreloadType("plugin3", "service3", "service-namespace3"),
				},
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1",
						Kind:       "Console",
						Controller: ptr.To(true),
					}},
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
  releaseVersion: ` + testReleaseVersion + `
session: {}
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
  perspectives:
    - id: dev
      visibility:
        state: Disabled
i18nNamespaces:
- plugin__plugin3
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers: {}
plugins:
  plugin1: https://service1.service-namespace1.svc.cluster.local:8443/
  plugin2: https://service2.service-namespace2.svc.cluster.local:8443/
  plugin3: https://service3.service-namespace3.svc.cluster.local:8443/
proxy:
  services:
  - authorize: true
    caCertificate: '-----BEGIN CERTIFICATE-----` + "\n" + `
MIICRzCCAfGgAwIBAgIJAIydTIADd+yqMA0GCSqGSIb3DQEBCwUAMH4xCzAJBgNV` + "\n" + `
BAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNVBAcMBkxvbmRvbjEYMBYGA1UE` + "\n" + `
CgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1JVCBEZXBhcnRtZW50MRswGQYD` + "\n" + `
VQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTIwIBcNMTcwNDI2MjMyNDU4WhgPMjExNzA0` + "\n" + `
MDIyMzI0NThaMH4xCzAJBgNVBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNV` + "\n" + `
BAcMBkxvbmRvbjEYMBYGA1UECgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1J` + "\n" + `
VCBEZXBhcnRtZW50MRswGQYDVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTIwXDANBgkq` + "\n" + `
hkiG9w0BAQEFAANLADBIAkEAuiRet28DV68Dk4A8eqCaqgXmymamUEjW/DxvIQqH` + "\n" + `
3lbhtm8BwSnS9wUAajSLSWiq3fci2RbRgaSPjUrnbOHCLQIDAQABo1AwTjAdBgNV` + "\n" + `
HQ4EFgQU0vhI4OPGEOqT+VAWwxdhVvcmgdIwHwYDVR0jBBgwFoAU0vhI4OPGEOqT` + "\n" + `
+VAWwxdhVvcmgdIwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAANBALNeJGDe` + "\n" + `
nV5cXbp9W1bC12Tc8nnNXn4ypLE2JTQAvyp51zoZ8hQoSnRVx/VCY55Yu+br8gQZ` + "\n" + `
+tW+O/PoE7B3tuY=` + "\n" + `
-----END CERTIFICATE-----'
    consoleAPIPath: /api/proxy/plugin/plugin1/plugin1-alias/
    endpoint: https://proxy-service1.proxy-service-namespace1.svc.cluster.local:9991
  - authorize: true
    caCertificate: '-----BEGIN CERTIFICATE-----` + "\n" + `
MIICRzCCAfGgAwIBAgIJAIydTIADd+yqMA0GCSqGSIb3DQEBCwUAMH4xCzAJBgNV` + "\n" + `
BAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNVBAcMBkxvbmRvbjEYMBYGA1UE` + "\n" + `
CgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1JVCBEZXBhcnRtZW50MRswGQYD` + "\n" + `
VQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTIwIBcNMTcwNDI2MjMyNDU4WhgPMjExNzA0` + "\n" + `
MDIyMzI0NThaMH4xCzAJBgNVBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNV` + "\n" + `
BAcMBkxvbmRvbjEYMBYGA1UECgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1J` + "\n" + `
VCBEZXBhcnRtZW50MRswGQYDVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTIwXDANBgkq` + "\n" + `
hkiG9w0BAQEFAANLADBIAkEAuiRet28DV68Dk4A8eqCaqgXmymamUEjW/DxvIQqH` + "\n" + `
3lbhtm8BwSnS9wUAajSLSWiq3fci2RbRgaSPjUrnbOHCLQIDAQABo1AwTjAdBgNV` + "\n" + `
HQ4EFgQU0vhI4OPGEOqT+VAWwxdhVvcmgdIwHwYDVR0jBBgwFoAU0vhI4OPGEOqT` + "\n" + `
+VAWwxdhVvcmgdIwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAANBALNeJGDe` + "\n" + `
nV5cXbp9W1bC12Tc8nnNXn4ypLE2JTQAvyp51zoZ8hQoSnRVx/VCY55Yu+br8gQZ` + "\n" + `
+tW+O/PoE7B3tuY=` + "\n" + `
-----END CERTIFICATE-----'
    consoleAPIPath: /api/proxy/plugin/plugin2/plugin2-alias/
    endpoint: https://proxy-service2.proxy-service-namespace2.svc.cluster.local:9991
`,
				},
			},
		},
		{
			name: "Test operator config, with 'External' ControlPlaneTopology",
			args: args{
				authConfig:     &configv1.Authentication{},
				operatorConfig: &operatorv1.Console{},
				consoleConfig:  &configv1.Console{},
				managedConfig:  &corev1.ConfigMap{},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL:         mockAPIServer,
						ControlPlaneTopology: configv1.ExternalTopologyMode,
					},
				},
				rt: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleName,
					},
					Spec: routev1.RouteSpec{
						Host: host,
					},
				},
				inactivityTimeoutSeconds: 0,
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1",
						Kind:       "Console",
						Controller: ptr.To(true),
					}},
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
  controlPlaneTopology: External
  releaseVersion: ` + testReleaseVersion + `
session: {}
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
  perspectives:
    - id: dev
      visibility:
        state: Disabled
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers: {}
`,
				},
			},
		},
		{
			name: "Test operator config, with CopiedCSVsDisabled",
			args: args{
				authConfig:     &configv1.Authentication{},
				operatorConfig: &operatorv1.Console{},
				consoleConfig:  &configv1.Console{},
				managedConfig:  &corev1.ConfigMap{},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL:         mockAPIServer,
						ControlPlaneTopology: configv1.ExternalTopologyMode,
					},
				},
				copiedCSVsDisabled: true,
				rt: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleName,
					},
					Spec: routev1.RouteSpec{
						Host: host,
					},
				},
				inactivityTimeoutSeconds: 0,
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1",
						Kind:       "Console",
						Controller: ptr.To(true),
					}},
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
  controlPlaneTopology: External
  releaseVersion: ` + testReleaseVersion + `
  copiedCSVsDisabled: true
session: {}
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
  perspectives:
    - id: dev
      visibility:
        state: Disabled
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers: {}
`,
				},
			},
		},
		{
			name: "Test default configmap with monitoring config",
			args: args{
				authConfig:     &configv1.Authentication{},
				operatorConfig: &operatorv1.Console{},
				consoleConfig:  &configv1.Console{},
				managedConfig:  &corev1.ConfigMap{},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL:         mockAPIServer,
						ControlPlaneTopology: configv1.HighlyAvailableTopologyMode,
					},
				},
				rt: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleName,
					},
					Spec: routev1.RouteSpec{
						Host: host,
					},
				},
				inactivityTimeoutSeconds: 0,
				monitoringSharedConfig: &corev1.ConfigMap{
					Data: map[string]string{
						"alertmanagerUserWorkloadHost": "alertmanager-user-workload.openshift-user-workload-monitoring.svc:9094",
						"alertmanagerTenancyHost":      "alertmanager-user-workload.openshift-user-workload-monitoring.svc:9092",
					},
				},
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      api.OpenShiftConsoleConfigMapName,
					Namespace: api.OpenShiftConsoleNamespace,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1",
						Kind:       "Console",
						Controller: ptr.To(true),
					}},
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
  controlPlaneTopology: HighlyAvailable
  releaseVersion: ` + testReleaseVersion + `
session: {}
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
  perspectives:
    - id: dev
      visibility:
        state: Disabled
monitoringInfo:
  alertmanagerTenancyHost: alertmanager-user-workload.openshift-user-workload-monitoring.svc:9092
  alertmanagerUserWorkloadHost: alertmanager-user-workload.openshift-user-workload-monitoring.svc:9094
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers: {}
`,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, _, _ := DefaultConfigMap(
				tt.args.operatorConfig,
				tt.args.consoleConfig,
				tt.args.authConfig,
				tt.args.managedConfig,
				tt.args.monitoringSharedConfig,
				tt.args.infrastructureConfig,
				tt.args.rt,
				tt.args.inactivityTimeoutSeconds,
				tt.args.availablePlugins,
				tt.args.nodeArchitectures,
				tt.args.nodeOperatingSystems,
				tt.args.copiedCSVsDisabled,
				tt.args.contentSecurityPolicyEnabled,
				tt.args.telemetryConfig,
				tt.args.rt.Spec.Host,
			)

			// marshall the exampleYaml to map[string]interface{} so we can use it in diff below
			var exampleConfig map[string]interface{}
			exampleBytes := []byte(tt.want.Data[configKey])
			err := yaml.Unmarshal(exampleBytes, &exampleConfig)
			if err != nil {
				t.Error(err)
			}

			// the reason we have to marshall blindly into map[string]interface{}
			// is that we don't have the definition for the console config struct.
			// it exists in the console repo under cmd/bridge/config.go and is not
			// available as an api object
			var actualConfig map[string]interface{}
			// convert the string back into a []byte
			configBytes := []byte(cm.Data[configKey])

			err = yaml.Unmarshal(configBytes, &actualConfig)
			if err != nil {
				t.Error("Problem with consoleConfig.Data[console-config.yaml]", err)
			}

			// compare the configs
			if diff := deep.Equal(exampleConfig, actualConfig); diff != nil {
				fmt.Printf("\n EXAMPLE: %#v\n\n ACTUAL: %#v\n", exampleConfig, actualConfig)
				t.Error(diff)
			}

			// nil them out, we already compared them, and unfortunately we can't trust
			// that the ordering will be stable. this avoids a flaky test.
			cm.Data = nil
			tt.want.Data = nil

			// and then we can test the rest of the struct
			if diff := deep.Equal(cm, tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func testPlugins(pluginName, serviceName, serviceNamespace string) *consolev1.ConsolePlugin {
	return &consolev1.ConsolePlugin{
		ObjectMeta: metav1.ObjectMeta{
			Name: pluginName,
		},
		Spec: consolev1.ConsolePluginSpec{
			Backend: consolev1.ConsolePluginBackend{
				Type: consolev1.Service,
				Service: &consolev1.ConsolePluginService{
					Name:      serviceName,
					Namespace: serviceNamespace,
					Port:      8443,
					BasePath:  "/",
				},
			},
		},
	}
}

func testPluginsWithProxy(pluginName, serviceName, serviceNamespace string) *consolev1.ConsolePlugin {
	plugin := testPlugins(pluginName, serviceName, serviceNamespace)
	plugin.Spec.Proxy = []consolev1.ConsolePluginProxy{
		{
			Alias:         fmt.Sprintf("%s-alias", pluginName),
			CACertificate: validCertificate,
			Authorization: consolev1.UserToken,
			Endpoint: consolev1.ConsolePluginProxyEndpoint{
				Type: consolev1.ProxyTypeService,
				Service: &consolev1.ConsolePluginProxyServiceConfig{
					Name:      fmt.Sprintf("proxy-%s", serviceName),
					Namespace: fmt.Sprintf("proxy-%s", serviceNamespace),
					Port:      9991,
				},
			},
		},
	}

	return plugin
}

func testPluginsWithI18nPreloadType(pluginName, serviceName, serviceNamespace string) *consolev1.ConsolePlugin {
	plugin := testPlugins(pluginName, serviceName, serviceNamespace)
	plugin.Spec.I18n.LoadType = consolev1.Preload
	return plugin
}

func TestStub(t *testing.T) {
	tests := []struct {
		name string
		want *corev1.ConfigMap
	}{
		{
			name: "Testing Stub function configmap",
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(Stub(), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestDefaultPublicConfigMap(t *testing.T) {
	tests := []struct {
		name string
		want *corev1.ConfigMap
	}{
		{
			name: "Test generating default public configmap with console URL",
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      api.OpenShiftConsolePublicConfigMapName,
					Namespace: api.OpenShiftConfigManagedNamespace,
				},
				Data: map[string]string{"consoleURL": mockConsoleURL},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(DefaultPublicConfig(mockConsoleURL), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func Test_consoleBaseAddr(t *testing.T) {
	type args struct {
		host string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test Console Base Addr",
			args: args{
				host: host,
			},
			want: fmt.Sprintf("https://%s", host),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(consoleBaseAddr(tt.args.host), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func Test_extractYAML(t *testing.T) {
	type args struct {
		newConfig *corev1.ConfigMap
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test getting data from configmap as yaml",
			args: args{
				newConfig: &corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "console-config",
						Namespace: "openshift-config-managed",
					},
					Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
session: {}
customization:
  branding: online
  documentationBaseURL: https://docs.okd.io/4.4/
`,
					},
					BinaryData: nil,
				},
			},
			want: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
session: {}
customization:
  branding: online
  documentationBaseURL: https://docs.okd.io/4.4/
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractYAML(tt.args.newConfig)
			if diff := deep.Equal(result, []byte(tt.want)); diff != nil {
				t.Error(diff)
				t.Errorf("Got: %v \n", result)
				t.Errorf("Want: %v \n", []byte(tt.want))
			}
		})
	}
}

func TestAggregateCSPDirectives(t *testing.T) {
	tests := []struct {
		name   string
		input  []*consolev1.ConsolePlugin
		output map[consolev1.DirectiveType][]string
	}{
		{
			name: "Test aggregate CSP directives for multiple ConsolePlugins",
			input: []*consolev1.ConsolePlugin{
				{
					Spec: consolev1.ConsolePluginSpec{
						ContentSecurityPolicy: []consolev1.ConsolePluginCSP{
							{
								Directive: consolev1.DefaultSrc,
								Values:    []consolev1.CSPDirectiveValue{"source1", "source2"},
							},
							{
								Directive: consolev1.ScriptSrc,
								Values:    []consolev1.CSPDirectiveValue{"script1"},
							},
						},
					},
				},
				{
					Spec: consolev1.ConsolePluginSpec{
						ContentSecurityPolicy: []consolev1.ConsolePluginCSP{
							{
								Directive: consolev1.DefaultSrc,
								Values:    []consolev1.CSPDirectiveValue{"source2", "source3"},
							},
							{
								Directive: consolev1.StyleSrc,
								Values:    []consolev1.CSPDirectiveValue{"style1", "style2"},
							},
						},
					},
				},
			},
			output: map[consolev1.DirectiveType][]string{
				consolev1.DefaultSrc: {"source1", "source2", "source3"},
				consolev1.ScriptSrc:  {"script1"},
				consolev1.StyleSrc:   {"style1", "style2"},
			},
		},
		{
			name: "Test aggregate CSP directives for a single ConsolePlugin",
			input: []*consolev1.ConsolePlugin{
				{
					Spec: consolev1.ConsolePluginSpec{
						ContentSecurityPolicy: []consolev1.ConsolePluginCSP{
							{
								Directive: consolev1.DefaultSrc,
								Values:    []consolev1.CSPDirectiveValue{"source1", "source2"},
							},
							{
								Directive: consolev1.StyleSrc,
								Values:    []consolev1.CSPDirectiveValue{"style1", "style2"},
							},
						},
					},
				},
			},
			output: map[consolev1.DirectiveType][]string{
				consolev1.DefaultSrc: {"source1", "source2"},
				consolev1.StyleSrc:   {"style1", "style2"},
			},
		},
		{
			name: "Test a single ConsolePlugin without CSP directives",
			input: []*consolev1.ConsolePlugin{
				{
					Spec: consolev1.ConsolePluginSpec{},
				},
			},
			output: nil,
		},
	}

	// Anonymous function to sort each slice in a directive map
	sortDirectives := func(directives map[consolev1.DirectiveType][]string) {
		for _, values := range directives {
			sort.Strings(values)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aggregateCSPDirectives(tt.input)
			sortDirectives(result)
			if diff := deep.Equal(tt.output, result); diff != nil {
				t.Error(diff)
				t.Errorf("Got: %v \n", result)
				t.Errorf("Want: %v \n", tt.output)
			}
		})
	}
}
