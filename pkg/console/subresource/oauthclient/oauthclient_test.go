package oauthclient

import (
	"testing"

	"github.com/go-test/deep"
	"github.com/openshift/console-operator/pkg/api"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	oauthv1 "github.com/openshift/api/oauth/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStub(t *testing.T) {
	tests := []struct {
		name string
		want *oauthv1.OAuthClient
	}{
		{
			name: "Test Stub oauth client",
			want: &oauthv1.OAuthClient{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: api.OAuthClientName,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(Stub(), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}
