package secret

import (
	"reflect"
	"testing"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/deployment"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestDefaultSecret(t *testing.T) {
	type args struct {
		cr         *v1alpha1.ConsoleOperatorConfig
		randomBits string
	}
	tests := []struct {
		name string
		args args
		want *corev1.Secret
	}{
		{
			name: "Test default secret",
			args: args{
				cr: &v1alpha1.ConsoleOperatorConfig{
					TypeMeta:   v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{},
					Spec:       v1alpha1.ConsoleOperatorConfigSpec{},
					Status:     v1alpha1.ConsoleOperatorConfigStatus{},
				},
				randomBits: ClientSecretKey,
			},
			want: &corev1.Secret{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name:      deployment.ConsoleOauthConfigName,
					Namespace: api.OpenShiftConsoleName,
					Labels:    map[string]string{"app": api.OpenShiftConsoleName},
				},
				Data:       map[string][]byte{"clientSecret": {99, 108, 105, 101, 110, 116, 83, 101, 99, 114, 101, 116}},
				StringData: nil,
				Type:       "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultSecret(tt.args.cr, tt.args.randomBits); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DefaultSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStub(t *testing.T) {
	tests := []struct {
		name string
		want *corev1.Secret
	}{
		{
			name: "Test stubbing secret",
			want: &corev1.Secret{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name:      deployment.ConsoleOauthConfigName,
					Namespace: api.OpenShiftConsoleName,
					Labels:    map[string]string{"app": api.OpenShiftConsoleName},
				},
				Data:       nil,
				StringData: nil,
				Type:       "",
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

func TestGetSecretString(t *testing.T) {
	type args struct {
		secret *corev1.Secret
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test get secret string",
			args: args{
				secret: &corev1.Secret{
					Data: map[string][]byte{"clientSecret": {99, 108, 105, 101, 110, 116, 83, 101, 99, 114, 101, 116}},
				},
			},
			want: ClientSecretKey,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetSecretString(tt.args.secret); got != tt.want {
				t.Errorf("GetSecretString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetSecretString(t *testing.T) {
	type args struct {
		secret     *corev1.Secret
		randomBits string
	}
	tests := []struct {
		name string
		args args
		want *corev1.Secret
	}{
		{
			name: "Test set secret string",
			args: args{
				secret:     &corev1.Secret{},
				randomBits: ClientSecretKey,
			},
			want: &corev1.Secret{
				Data: map[string][]byte{"clientSecret": {99, 108, 105, 101, 110, 116, 83, 101, 99, 114, 101, 116}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SetSecretString(tt.args.secret, tt.args.randomBits); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetSecretString() = %v, want %v", got, tt.want)
			}
		})
	}
}
