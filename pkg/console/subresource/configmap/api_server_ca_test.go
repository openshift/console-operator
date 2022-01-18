package configmap

import (
	"testing"

	"github.com/go-test/deep"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
)

func TestDefaultAPIServerCAConfigMap(t *testing.T) {
	type args struct {
		clusterName string
		caBundle    []byte
		cr          *operatorv1.Console
	}
	tests := []struct {
		name string
		args args
		want *corev1.ConfigMap
	}{
		{
			name: "Test default API server CA config map",
			args: args{
				clusterName: "test-cluster",
				caBundle:    []byte("test-bundle"),
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
					Name:                       APIServerCAConfigMapName("test-cluster"),
					Namespace:                  api.OpenShiftConsoleNamespace,
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:                     map[string]string{api.ManagedClusterLabel: "test-cluster", api.ManagedClusterAPIServerCertName: "", "app": "console"},
					Annotations:                map[string]string{},
					OwnerReferences:            nil,
					Finalizers:                 nil,
				},
				Data: map[string]string{
					"ca-bundle.crt": "test-bundle",
				},
				BinaryData: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(DefaultAPIServerCAConfigMap(tt.args.clusterName, tt.args.caBundle, tt.args.cr), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestAPIServerCAStub(t *testing.T) {
	type args struct {
		clusterName string
	}
	tests := []struct {
		name string
		args args
		want *corev1.ConfigMap
	}{
		{
			name: "Test stubbing API server CA config map",
			args: args{
				clusterName: "test-cluster",
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:                       APIServerCAConfigMapName("test-cluster"),
					Namespace:                  api.OpenShiftConsoleNamespace,
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:                     map[string]string{api.ManagedClusterLabel: "test-cluster", api.ManagedClusterAPIServerCertName: "", "app": "console"},
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
			if diff := deep.Equal(APIServerCAConfigMapStub(tt.args.clusterName), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}
