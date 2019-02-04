package service

import (
	"github.com/openshift/console-operator/pkg/api"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"reflect"
	"testing"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"k8s.io/api/core/v1"
)

func TestDefaultService(t *testing.T) {
	type args struct {
		cr *v1alpha1.Console
	}
	tests := []struct {
		name string
		args args
		want *v1.Service
	}{
		{
			name: "Test default service generation",
			args: args{
				cr: &v1alpha1.Console{},
			},
			want: &v1.Service{
				TypeMeta: v12.TypeMeta{},
				ObjectMeta: v12.ObjectMeta{
					Name:      api.OpenShiftConsoleShortName,
					Namespace: api.OpenShiftConsoleNamespace,
					Labels:    map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{
						ServingCertSecretAnnotation: ConsoleServingCertName},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:       consolePortName,
							Protocol:   v1.ProtocolTCP,
							Port:       consolePort,
							TargetPort: intstr.FromInt(consoleTargetPort),
						},
					},
					Selector:        map[string]string{"app": api.OpenShiftConsoleName, "component": "ui"},
					Type:            "ClusterIP",
					SessionAffinity: "None",
				},
				Status: v1.ServiceStatus{},
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
		want *v1.Service
	}{
		{
			name: "Test stubbing out service",
			want: &v1.Service{
				TypeMeta: v12.TypeMeta{},
				ObjectMeta: v12.ObjectMeta{
					Name:      api.OpenShiftConsoleShortName,
					Namespace: api.OpenShiftConsoleNamespace,
					Labels:    map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{
						ServingCertSecretAnnotation: ConsoleServingCertName},
				},
				Spec:   v1.ServiceSpec{},
				Status: v1.ServiceStatus{},
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
