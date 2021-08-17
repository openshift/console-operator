package configmap

import (
	"testing"

	"github.com/go-test/deep"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
)

func TestDefaultClusterCAConfigMap(t *testing.T) {
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
			name: "Test default cluster CA config map",
			args: args{
				clusterName: "test-cluster",
				caBundle:    nil,
				cr: &operatorv1.Console{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec:       operatorv1.ConsoleSpec{},
					Status:     operatorv1.ConsoleStatus{},
				},
			},
			want: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:                       ClusterCAConfigMapName("test-cluster"),
					Namespace:                  api.OpenShiftConsoleNamespace,
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:                     map[string]string{ClusterCAConfigMapLabel: ""},
					Annotations:                map[string]string{},
					OwnerReferences:            nil,
					Finalizers:                 nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(DefaultClusterCAConfigMap(tt.args.clusterName, tt.args.caBundle, tt.args.cr), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestClusterCAStub(t *testing.T) {
	type args struct {
		clusterName string
		caBundle    []byte
	}
	tests := []struct {
		name string
		args args
		want *corev1.ConfigMap
	}{
		{
			name: "Test stubbing Cluster CA",
			args: args{
				clusterName: "test-cluster",
				caBundle:    []byte("test-bundle"),
			},
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:                       ClusterCAConfigMapName("test-cluster"),
					Namespace:                  api.OpenShiftConsoleNamespace,
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:                     map[string]string{ClusterCAConfigMapLabel: ""},
					Annotations:                map[string]string{},
					OwnerReferences:            nil,
					Finalizers:                 nil,
				},
				Data: map[string]string{
					"ca.crt": "test-bundle",
				},
				BinaryData: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(ClusterCAStub(tt.args.clusterName, tt.args.caBundle), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}
