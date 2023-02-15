package oauthclient

import (
	oauthv1 "github.com/openshift/api/oauth/v1"
	"github.com/openshift/console-operator/pkg/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DefaultOauthClient() *oauthv1.OAuthClient {
	return Stub()
}

func Stub() *oauthv1.OAuthClient {
	// we cannot set an ownerRef on the OAuthClient as it is cluster scoped
	return &oauthv1.OAuthClient{
		ObjectMeta: metav1.ObjectMeta{
			Name: api.OAuthClientName,
		},
	}
}
