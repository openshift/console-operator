package service

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
)

func TestDefaultService(t *testing.T) {
	type args struct {
		cr *operatorv1.Console
	}
	tests := []struct {
		name string
		args args
		want *corev1.Service
	}{
		{
			name: "Test default service generation",
			args: args{
				cr: &operatorv1.Console{},
			},
			want: &corev1.Service{
				TypeMeta: v12.TypeMeta{},
				ObjectMeta: v12.ObjectMeta{
					Name:      api.OpenShiftConsoleName,
					Namespace: api.OpenShiftConsoleNamespace,
					Labels:    map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{
						ServingCertSecretAnnotation: ConsoleServingCertName},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       consolePortName,
							Protocol:   corev1.ProtocolTCP,
							Port:       consolePort,
							TargetPort: intstr.FromInt(consoleTargetPort),
						},
					},
					Selector:        map[string]string{"app": api.OpenShiftConsoleName, "component": "ui"},
					Type:            "ClusterIP",
					SessionAffinity: "None",
				},
				Status: corev1.ServiceStatus{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultService(tt.args.cr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DefaultService() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStub(t *testing.T) {
	tests := []struct {
		name string
		want *corev1.Service
	}{
		{
			name: "Test stubbing out service",
			want: &corev1.Service{
				TypeMeta: v12.TypeMeta{},
				ObjectMeta: v12.ObjectMeta{
					Name:      api.OpenShiftConsoleName,
					Namespace: api.OpenShiftConsoleNamespace,
					Labels:    map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{
						ServingCertSecretAnnotation: ConsoleServingCertName},
				},
				Spec:   corev1.ServiceSpec{},
				Status: corev1.ServiceStatus{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Stub(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Stub() = %v, want %v", got, tt.want)
			}
		})
	}
}
