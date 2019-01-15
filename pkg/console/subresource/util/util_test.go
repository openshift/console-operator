package util

import (
	"github.com/openshift/console-operator/pkg/api"
	"reflect"
	"testing"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
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
		want v1.ObjectMeta
	}{
		{
			name: "Test shared metadata",
			want: v1.ObjectMeta{
				Name:      api.OpenShiftConsoleName,
				Namespace: api.OpenShiftConsoleName,
				Labels:    map[string]string{"app": api.OpenShiftConsoleName},
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
		obj      v1.Object
		ownerRef *v1.OwnerReference
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
		cr *v1alpha1.Console
	}
	tests := []struct {
		name string
		args args
		want *v1.OwnerReference
	}{
		{
			name: "Test owner ref from when cr is not null",
			args: args{
				cr: &v1alpha1.Console{
					TypeMeta: v1.TypeMeta{
						Kind:       "Console",
						APIVersion: "4.0",
					},
					ObjectMeta: v1.ObjectMeta{
						Name: "Test",
						UID:  "",
					},
					Spec:   v1alpha1.ConsoleSpec{},
					Status: v1alpha1.ConsoleStatus{},
				},
			},
			want: &v1.OwnerReference{
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
