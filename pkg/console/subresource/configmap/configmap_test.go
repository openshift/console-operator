package configmap

import (
	"fmt"
	"github.com/openshift/api/route/v1"
	"reflect"
	"testing"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	host        = "localhost"
	exampleYaml = `kind: ConsoleConfig
apiVersion: console.openshift.io/v1beta1
auth:
  clientID: openshift-console
  clientSecretFile: /var/oauth-config/clientSecret
  logoutRedirect: ""
  oauthEndpointCAFile: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
clusterInfo:
  consoleBaseAddress: https://` + host + `
  consoleBasePath: ""
customization:
  branding: okd
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
		cr  *v1alpha1.Console
		rt  *v1.Route
		opt v1alpha1.FlagOptions
	}
	tests := []struct {
		name string
		args args
		want *corev1.ConfigMap
	}{
		{
			name: "Test Default Config Map",
			args: args{
				cr: &v1alpha1.Console{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec:       v1alpha1.ConsoleSpec{},
					Status:     v1alpha1.ConsoleStatus{},
				},
				rt: &v1.Route{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: v1.RouteSpec{
						Host:              host,
						Path:              "",
						To:                v1.RouteTargetReference{},
						AlternateBackends: nil,
						Port:              nil,
						TLS:               nil,
						WildcardPolicy:    "",
					},
					Status: v1.RouteStatus{},
				},
				opt: v1alpha1.FlagOptions{
					Brand: brandingDefault,
				},
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:                       ConsoleConfigMapName,
					GenerateName:               "",
					Namespace:                  api.OpenShiftConsoleName,
					SelfLink:                   "",
					UID:                        "",
					ResourceVersion:            "",
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:          map[string]string{"app": api.OpenShiftConsoleName},
					Annotations:     nil,
					OwnerReferences: nil,
					Initializers:    nil,
					Finalizers:      nil,
					ClusterName:     "",
				},
				Data:       map[string]string{"console-config.yaml": exampleYaml},
				BinaryData: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultConfigMap(tt.args.cr, tt.args.rt, tt.args.opt); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DefaultConfigMap() = %v\n ----------- want %v", got, tt.want)
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
					Namespace:                  api.OpenShiftConsoleName,
					SelfLink:                   "",
					UID:                        "",
					ResourceVersion:            "",
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:          map[string]string{"app": api.OpenShiftConsoleName},
					Annotations:     nil,
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
			if got := Stub(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("\nStub() = %v\n -------- want %v", got, tt.want)
			}
		})
	}
}

// This unit test relies on both NewYamlConfig and NewYamlConfigString
// to ensure the serialized data is created from host name
// TODO: remove - This unit test is probably not useful since it is just testing yaml methods slice and marshal with no logic
func TestNewYamlConfig(t *testing.T) {
	type args struct {
		host    string
		options v1alpha1.FlagOptions
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "TestNewYamlConfig",
			args: args{
				host:    host,
				options: v1alpha1.FlagOptions{Brand: brandingDefault},
			},
			want: exampleYaml,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewYamlConfigString(tt.args.host, tt.args.options); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewYamlConfig() = \n%v\n ----> want\n%v", got, tt.want)
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
			if got := consoleBaseAddr(tt.args.host); got != tt.want {
				t.Errorf("consoleBaseAddr() = %v, want %v", got, tt.want)
			}
		})
	}
}
