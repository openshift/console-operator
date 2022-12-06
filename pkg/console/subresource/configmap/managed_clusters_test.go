package configmap

import (
	"testing"

	"github.com/go-test/deep"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
)

var (
	testManagedClusterConfig consoleserver.ManagedClusterConfig = consoleserver.ManagedClusterConfig{
		Name: "test-cluster",
		APIServer: consoleserver.ManagedClusterAPIServerConfig{
			URL:    "test-url",
			CAFile: "/var/api/ca",
		},
		Oauth: consoleserver.ManagedClusterOAuthConfig{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			CAFile:       "/var/oauth/ca",
		},
	}
	testManagedClusterConfigYaml = `- name: test-cluster
  apiServer:
    url: test-url
    caFile: /var/api/ca
  oauth:
    clientID: test-client-id
    clientSecret: test-client-secret
    caFile: /var/oauth/ca
  copiedCSVsDisabled: false
`
)

func TestDefaultManagedClustersConfigMap(t *testing.T) {
	type args struct {
		managedClusters []consoleserver.ManagedClusterConfig
		cr              *operatorv1.Console
	}
	tests := []struct {
		name string
		args args
		want *corev1.ConfigMap
	}{
		{
			name: "Test default managed clusters config map",
			args: args{
				managedClusters: []consoleserver.ManagedClusterConfig{testManagedClusterConfig},
				cr: &operatorv1.Console{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec:       operatorv1.ConsoleSpec{},
					Status:     operatorv1.ConsoleStatus{},
				},
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:                       api.ManagedClusterConfigMapName,
					Namespace:                  api.OpenShiftConsoleNamespace,
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:                     map[string]string{"app": "console", api.ManagedClusterLabel: ""},
					Annotations:                map[string]string{},
					OwnerReferences:            nil,
					Finalizers:                 nil,
				},
				Data: map[string]string{
					api.ManagedClusterConfigKey: testManagedClusterConfigYaml,
				},
				BinaryData: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, _ := DefaultManagedClustersConfigMap(tt.args.cr, tt.args.managedClusters)
			if diff := deep.Equal(cm, tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestManagedClusterStub(t *testing.T) {
	tests := []struct {
		name string
		want *corev1.ConfigMap
	}{
		{
			name: "Test stubbing managed clusters config map",
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:                       api.ManagedClusterConfigMapName,
					Namespace:                  api.OpenShiftConsoleNamespace,
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:                     map[string]string{"app": "console", api.ManagedClusterLabel: ""},
					Annotations:                map[string]string{},
					OwnerReferences:            nil,
					Finalizers:                 nil,
				},
				Data:       nil,
				BinaryData: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(ManagedClustersConfigMapStub(), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}
