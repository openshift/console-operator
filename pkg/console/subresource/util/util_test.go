package util

import (
	"testing"

	"github.com/go-test/deep"

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
			if diff := deep.Equal(SharedLabels(), tt.want); diff != nil {
				t.Error(diff)
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
			if diff := deep.Equal(LabelsForConsole(), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestLabelsForDownloads(t *testing.T) {
	tests := []struct {
		name string
		want map[string]string
	}{
		{
			name: "Test labels for downloads",
			want: map[string]string{"app": api.OpenShiftConsoleName, "component": api.DownloadsResourceName},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(LabelsForDownloads(), tt.want); diff != nil {
				t.Error(diff)
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
			if diff := deep.Equal(SharedMeta(), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

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
					ObjectMeta: metav1.ObjectMeta{
						Name: "Test",
						UID:  "50",
					},
					Spec:   operatorv1.ConsoleSpec{},
					Status: operatorv1.ConsoleStatus{},
				},
			},
			want: &metav1.OwnerReference{
				APIVersion: "operator.openshift.io/v1",
				Kind:       "Console",
				Name:       "Test",
				UID:        "50",
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
			if diff := deep.Equal(OwnerRefFrom(tt.args.cr), tt.want); diff != nil {
				t.Error(diff)
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
			if diff := deep.Equal(HTTPS(tt.args.host), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestRemoveDuplicateStr(t *testing.T) {
	tests := []struct {
		strSlice []string
		want     []string
	}{
		{
			strSlice: []string{"plugin1", "plugin2", "plugin3", "plugin1", "plugin2"},
			want:     []string{"plugin1", "plugin2", "plugin3"},
		},
	}
	for _, tt := range tests {
		t.Run("RemoveDuplicateStr unit test", func(t *testing.T) {
			if diff := deep.Equal(RemoveDuplicateStr(tt.strSlice), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}
