package configmap

import (
	"reflect"
	"testing"

	"github.com/openshift/console-operator/pkg/api"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDefaultServiceCAConfigMap(t *testing.T) {
	type args struct {
		cr *v1alpha1.ConsoleOperatorConfig
	}
	tests := []struct {
		name string
		args args
		want *corev1.ConfigMap
	}{
		{
			name: "Test default service CA config map",
			args: args{
				cr: &v1alpha1.ConsoleOperatorConfig{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec:       v1alpha1.ConsoleOperatorConfigSpec{},
					Status:     v1alpha1.ConsoleOperatorConfigStatus{},
				},
			},
			want: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:                       ServiceCAConfigMapName,
					Namespace:                  api.OpenShiftConsoleName,
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:          map[string]string{"app": api.OpenShiftConsoleName},
					Annotations:     map[string]string{injectCABundleAnnotation: "true"},
					OwnerReferences: nil,
					Initializers:    nil,
					Finalizers:      nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultServiceCAConfigMap(tt.args.cr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DefaultServiceCAConfigMap() = %v, want %v", got, tt.want)
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
					Name:                       ServiceCAConfigMapName,
					Namespace:                  api.OpenShiftConsoleName,
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:          map[string]string{"app": api.OpenShiftConsoleName},
					Annotations:     map[string]string{injectCABundleAnnotation: "true"},
					OwnerReferences: nil,
					Initializers:    nil,
					Finalizers:      nil,
				},
				Data:       nil,
				BinaryData: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ServiceCAStub(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ServiceCAStub() = %v, want %v", got, tt.want)
			}
		})
	}
}
