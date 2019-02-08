package util

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
)

func TestSharedLabels(t *testing.T) {
	tests := []struct {
		name string
		want map[string]string
	}{
		{
			name: "Test generating shared labels",
			want: map[string]string{
				"app": api.OpenShiftConsoleName,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SharedLabels(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SharedLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLabelsForConsole(t *testing.T) {
	tests := []struct {
		name string
		want map[string]string
	}{
		{
			name: "Test labels for console",
			want: map[string]string{"app": api.OpenShiftConsoleName, "component": "ui"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LabelsForConsole(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LabelsForConsole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSharedMeta(t *testing.T) {
	tests := []struct {
		name string
		want metav1.ObjectMeta
	}{
		{
			name: "Test shared metadata",
			want: metav1.ObjectMeta{
				Name:        api.OpenShiftConsoleName,
				Namespace:   api.OpenShiftConsoleNamespace,
				Labels:      map[string]string{"app": api.OpenShiftConsoleName},
				Annotations: map[string]string{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SharedMeta(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SharedMeta() = %v, want %v", got, tt.want)
			}
		})
	}
}

/*
func TestAddOwnerRef(t *testing.T) {
	type args struct {
		obj      metav1Object
		ownerRef *metav1OwnerReference
	}
	tests := []struct {
		name string
		args args
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddOwnerRef(tt.args.obj, tt.args.ownerRef)
		})
	}
}
*/

func TestOwnerRefFrom(t *testing.T) {
	var truthy = true
	type args struct {
		cr *operatorv1.Console
	}
	tests := []struct {
		name string
		args args
		want *metav1.OwnerReference
	}{
		{
			name: "Test owner ref from when cr is not null",
			args: args{
				cr: &operatorv1.Console{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Console",
						APIVersion: "4.0",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "Test",
						UID:  "",
					},
					Spec:   operatorv1.ConsoleSpec{},
					Status: operatorv1.ConsoleStatus{},
				},
			},
			want: &metav1.OwnerReference{
				APIVersion: "4.0",
				Kind:       "Console",
				Name:       "Test",
				UID:        "",
				Controller: &truthy,
			},
		},
		{
			name: "Test owner ref from when cr is nil",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OwnerRefFrom(tt.args.cr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OwnerRefFrom() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHTTPS(t *testing.T) {
	type args struct {
		host string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test HTTPS when host is empty",
			args: args{
				host: "",
			},
			want: "",
		},
		{
			name: "Test HTTPS when host already has secure protocol",
			args: args{
				host: "https://localhost",
			},
			want: "https://localhost",
		},
		{
			name: "Test HTTPS when secure protocol is prepended to host",
			args: args{
				host: "localhost",
			},
			want: "https://localhost",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HTTPS(tt.args.host); got != tt.want {
				t.Errorf("HTTPS() = %v, want %v", got, tt.want)
			}
		})
	}
}
