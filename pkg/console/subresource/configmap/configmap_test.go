package configmap

import (
	"fmt"
	"strconv"
	"testing"

	yaml "gopkg.in/yaml.v2"

	"github.com/go-test/deep"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/api/console/v1alpha1"
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
		operatorConfig           *operatorv1.Console
		consoleConfig            *configv1.Console
		managedConfig            *corev1.ConfigMap
		infrastructureConfig     *configv1.Infrastructure
		rt                       *routev1.Route
		useDefaultCAFile         bool
		inactivityTimeoutSeconds int
		availablePlugins         []*v1alpha1.ConsolePlugin
		managedClusterConfigFile string
	}
	tests := []struct {
		name string
		args args
		want *corev1.ConfigMap
	}{
		{
			name: "Test default configmap, no customization",
			args: args{
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
				useDefaultCAFile:         true,
				inactivityTimeoutSeconds: 0,
				managedClusterConfigFile: "",
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
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
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
				useDefaultCAFile:         false,
				inactivityTimeoutSeconds: 0,
				managedClusterConfigFile: "",
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
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
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
				operatorConfig: &operatorv1.Console{},
				consoleConfig:  &configv1.Console{},
				managedConfig: &corev1.ConfigMap{
					Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
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
				useDefaultCAFile:         true,
				inactivityTimeoutSeconds: 0,
				managedClusterConfigFile: "",
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
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
customization:
  branding: online
  documentationBaseURL: https://docs.okd.io/4.4/
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
				useDefaultCAFile:         true,
				inactivityTimeoutSeconds: 0,
				managedClusterConfigFile: "",
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
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
customization:
  branding: ` + string(operatorv1.BrandDedicated) + `
  documentationBaseURL: ` + mockOperatorDocURL + `
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
			name: "Test operator config with Custom Branding Values",
			args: args{
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
				useDefaultCAFile:         true,
				inactivityTimeoutSeconds: 0,
				managedClusterConfigFile: "",
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
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
customization:
  branding: ` + string(operatorv1.BrandDedicated) + `
  documentationBaseURL: ` + mockOperatorDocURL + `
  customLogoFile: /var/logo/logo.svg
  customProductName: custom-product-name
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
				useDefaultCAFile:         true,
				inactivityTimeoutSeconds: 0,
				managedClusterConfigFile: "",
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
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
customization:
  branding: ` + string(operatorv1.BrandDedicated) + `
  documentationBaseURL: ` + mockOperatorDocURL + `
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
				useDefaultCAFile:         false,
				inactivityTimeoutSeconds: 0,
				managedClusterConfigFile: "",
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
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
clusterInfo:
  consoleBaseAddress: https://` + customHostname + `
  masterPublicURL: ` + mockAPIServer + `
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
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
				useDefaultCAFile:         true,
				inactivityTimeoutSeconds: 60,
				managedClusterConfigFile: "",
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
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  inactivityTimeoutSeconds: 60
  oauthEndpointCAFile: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
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
			name: "Test operator config, with managedClusterConfigFile set",
			args: args{
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
				useDefaultCAFile:         true,
				inactivityTimeoutSeconds: 0,
				managedClusterConfigFile: "test",
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
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers: {}
managedClusterConfigFile: 'test'
`,
				},
			},
		},
		{
			name: "Test operator config, with enabledPlugins set",
			args: args{
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
				useDefaultCAFile:         true,
				inactivityTimeoutSeconds: 0,
				availablePlugins: []*v1alpha1.ConsolePlugin{
					testPlugins("pluginName1", "serviceName1", "serviceNamespace1"),
					testPluginsWithProxy("pluginName2", "serviceName2", "serviceNamespace2"),
				},
				managedClusterConfigFile: "",
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
				},
				Data: map[string]string{configKey: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  masterPublicURL: ` + mockAPIServer + `
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
providers: {}
plugins:
  pluginName1: https://serviceName1.serviceNamespace1.svc.cluster.local:8443/
  pluginName2: https://serviceName2.serviceNamespace2.svc.cluster.local:8443/
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
    consoleAPIPath: /api/proxy/plugin/pluginName2/test-alias/
    endpoint: https://proxy-serviceName2.proxy-serviceNamespace2.svc.cluster.local:9991
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
				tt.args.managedConfig,
				tt.args.infrastructureConfig,
				tt.args.rt,
				tt.args.useDefaultCAFile,
				tt.args.inactivityTimeoutSeconds,
				tt.args.availablePlugins,
				tt.args.managedClusterConfigFile,
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

func testPlugins(pluginName, serviceName, serviceNamespace string) *v1alpha1.ConsolePlugin {
	return &v1alpha1.ConsolePlugin{
		ObjectMeta: metav1.ObjectMeta{
			Name: pluginName,
		},
		Spec: v1alpha1.ConsolePluginSpec{
			Service: v1alpha1.ConsolePluginService{
				Name:      serviceName,
				Namespace: serviceNamespace,
				Port:      8443,
				BasePath:  "/",
			},
		},
	}
}

func testPluginsWithProxy(pluginName, serviceName, serviceNamespace string) *v1alpha1.ConsolePlugin {
	plugin := testPlugins(pluginName, serviceName, serviceNamespace)
	plugin.Spec.Proxy = []v1alpha1.ConsolePluginProxy{
		{
			Alias:         "test-alias",
			Type:          v1alpha1.ProxyTypeService,
			CACertificate: validCertificate,
			Authorize:     true,
			Service: v1alpha1.ConsolePluginProxyServiceConfig{
				Name:      fmt.Sprintf("proxy-%s", serviceName),
				Namespace: fmt.Sprintf("proxy-%s", serviceNamespace),
				Port:      9991,
			},
		},
	}

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
