package configmap

import (
	"testing"

	"github.com/go-test/deep"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
)

func TestDefaultServiceCAConfigMap(t *testing.T) {
	type args struct {
		cr *operatorv1.Console
	}
	tests := []struct {
		name string
		args args
		want *corev1.ConfigMap
	}{
		{
			name: "Test default service CA config map",
			args: args{
				cr: &operatorv1.Console{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec:       operatorv1.ConsoleSpec{},
					Status:     operatorv1.ConsoleStatus{},
				},
			},
			want: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:                       api.ServiceCAConfigMapName,
					Namespace:                  api.OpenShiftConsoleNamespace,
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:                     map[string]string{"app": api.OpenShiftConsoleName},
					Annotations:                map[string]string{injectCABundleAnnotation: "true"},
					OwnerReferences:            nil,
					Finalizers:                 nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(DefaultServiceCAConfigMap(tt.args.cr), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestServiceCAStub(t *testing.T) {
	tests := []struct {
		name string
		want *corev1.ConfigMap
	}{
		{
			name: "Test stubbing Service CA",
			want: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:                       api.ServiceCAConfigMapName,
					Namespace:                  api.OpenShiftConsoleNamespace,
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:                     map[string]string{"app": api.OpenShiftConsoleName},
					Annotations:                map[string]string{injectCABundleAnnotation: "true"},
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
			if diff := deep.Equal(ServiceCAStub(), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}
