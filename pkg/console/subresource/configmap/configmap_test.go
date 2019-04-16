package configmap

import (
	"fmt"
	"testing"

	yaml "gopkg.in/yaml.v2"

	"github.com/go-test/deep"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
)

const (
	host          = "localhost"
	mockAPIServer = "https://api.some.cluster.openshift.com:6443"
	configKey     = "console-config.yaml"
	exampleYaml   = `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  logoutRedirect: ""
  oauthEndpointCAFile: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  consoleBasePath: ""
  masterPublicURL: ` + mockAPIServer + `
customization:
  branding: ` + DEFAULT_BRAND + `
  documentationBaseURL: ` + DEFAULT_DOC_URL + `
servingInfo:
  bindAddress: https://0.0.0.0:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
`
	exampleManagedConfigMapData = `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
customization:
  branding: online
  documentationBaseURL: https://docs.okd.io/4.0/
`
	exampleYamlWithManagedConfig = `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  logoutRedirect: ""
  oauthEndpointCAFile: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  consoleBasePath: ""
  masterPublicURL: ` + mockAPIServer + `
customization:
  branding: online 
  documentationBaseURL: https://docs.okd.io/4.0/
servingInfo:
  bindAddress: https://0.0.0.0:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
`
)

// To manually run these tests: go test -v ./pkg/console/subresource/configmap/...
func TestDefaultConfigMap(t *testing.T) {
	type args struct {
		operatorConfig       *operatorv1.Console
		consoleConfig        *configv1.Console
		managedConfig        *corev1.ConfigMap
		infrastructureConfig *configv1.Infrastructure
		rt                   *routev1.Route
	}
	tests := []struct {
		name string
		args args
		want *corev1.ConfigMap
	}{
		{
			name: "Test generating default configmap without managed config override",
			args: args{
				operatorConfig: &operatorv1.Console{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec:       operatorv1.ConsoleSpec{},
					Status:     operatorv1.ConsoleStatus{},
				},
				consoleConfig: &configv1.Console{
					Spec:   configv1.ConsoleSpec{},
					Status: configv1.ConsoleStatus{},
				},
				managedConfig: &corev1.ConfigMap{},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL: mockAPIServer,
					},
				},
				rt: &routev1.Route{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: routev1.RouteSpec{
						Host: host,
					},
					Status: routev1.RouteStatus{},
				},
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:        ConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
				},
				Data: map[string]string{configKey: exampleYaml},
			},
		},
		{
			name: "Test generating configmap with managed config to override branding",
			args: args{
				operatorConfig: &operatorv1.Console{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec:       operatorv1.ConsoleSpec{},
					Status:     operatorv1.ConsoleStatus{},
				},
				consoleConfig: &configv1.Console{
					Spec:   configv1.ConsoleSpec{},
					Status: configv1.ConsoleStatus{},
				},
				managedConfig: &corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "console-config",
						Namespace: "openshift-config-managed",
					},
					Data:       map[string]string{configKey: exampleManagedConfigMapData},
					BinaryData: nil,
				},
				infrastructureConfig: &configv1.Infrastructure{
					Status: configv1.InfrastructureStatus{
						APIServerURL: mockAPIServer,
					},
				},
				rt: &routev1.Route{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: routev1.RouteSpec{
						Host: host,
					},
					Status: routev1.RouteStatus{},
				},
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:        ConsoleConfigMapName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
				},
				Data: map[string]string{configKey: exampleYamlWithManagedConfig},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, _, _ := DefaultConfigMap(tt.args.operatorConfig, tt.args.consoleConfig, tt.args.managedConfig, tt.args.infrastructureConfig, tt.args.rt)

			// marshall the exampleYaml to map[string]interface{} so we can use it in diff below
			var exampleConfig map[string]interface{}
			exampleBytes := []byte(tt.want.Data[configKey])
			err := yaml.Unmarshal(exampleBytes, &exampleConfig)
			if err != nil {
				t.Error(err)
				fmt.Printf("%v\n", exampleConfig)
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

func TestStub(t *testing.T) {
	tests := []struct {
		name string
		want *corev1.ConfigMap
	}{
		{
			name: "Testing Stub function configmap",
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:                       ConsoleConfigMapName,
					GenerateName:               "",
					Namespace:                  api.OpenShiftConsoleNamespace,
					SelfLink:                   "",
					UID:                        "",
					ResourceVersion:            "",
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:          map[string]string{"app": api.OpenShiftConsoleName},
					Annotations:     map[string]string{},
					OwnerReferences: nil,
					Initializers:    nil,
					Finalizers:      nil,
					ClusterName:     "",
				},
				BinaryData: nil,
				Data:       nil,
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

// This unit test relies on both NewYamlConfig and NewYamlConfigString
// to ensure the serialized data is created from host name
func TestNewYamlConfig(t *testing.T) {
	type args struct {
		host           string
		logoutRedirect string
		brand          operatorv1.Brand
		docURL         string
		apiServerURL   string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "TestNewYamlConfig",
			args: args{
				host:           host,
				logoutRedirect: "",
				brand:          DEFAULT_BRAND,
				docURL:         DEFAULT_DOC_URL,
				apiServerURL:   mockAPIServer,
			},
			want: exampleYaml,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(string(NewYamlConfig(tt.args.host, tt.args.logoutRedirect, tt.args.brand, tt.args.docURL, tt.args.apiServerURL)), tt.want); diff != nil {
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
					Data:       map[string]string{configKey: exampleManagedConfigMapData},
					BinaryData: nil,
				},
			},
			want: `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
customization:
  branding: online
  documentationBaseURL: https://docs.okd.io/4.0/
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
