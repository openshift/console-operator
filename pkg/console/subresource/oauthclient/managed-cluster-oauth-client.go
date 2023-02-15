package oauthclient

import (
	oauthv1 "github.com/openshift/api/oauth/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

func DefaultManagedClusterOauthClient(secret string, redirectUris []string) *oauthv1.OAuthClient {
	client := ManagedClusterOAuthClientStub()
	SetGrantMethod(client, oauthv1.GrantHandlerAuto)
	SetRedirectURIs(client, redirectUris)
	SetSecretString(client, secret)
	return client
}

func ManagedClusterOAuthClientStub() *oauthv1.OAuthClient {
	return &oauthv1.OAuthClient{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OAuthClient",
			APIVersion: oauthv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   api.ManagedClusterOAuthClientName,
			Labels: util.LabelsForManagedClusterResources(""),
		},
	}
}
