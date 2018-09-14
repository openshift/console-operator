package console

import (
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	oauthClientName = "console-oauth-client"
)


func newOauthConfigSecret(randomSecret string) *corev1.Secret {
	meta := sharedMeta()
	meta.Name = consoleOauthConfigName
	oauthConfigSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind: "Secret",
		},
		ObjectMeta: meta,
		StringData: map[string]string{
			"clientSecret": randomSecret,
		},
	}
	return oauthConfigSecret
}

func addRedirectURI(oauth *oauthv1.OAuthClient, rt *routev1.Route) {
	if rt != nil {
		if oauth.RedirectURIs != nil {
			oauth.RedirectURIs = []string{}
		}
		oauth.RedirectURIs = append(oauth.RedirectURIs, rt.Spec.Host)
	}
}

// NOTE: this also crates the oauth-config-secret, which seems a little
// fishy but works.  perhaps it should be split out.
func newConsoleOauthClient(cr *v1alpha1.Console, rt *routev1.Route) (*oauthv1.OAuthClient, *corev1.Secret) {
	randomBits := crypto.RandomBitsString(256)
	oauthConfigSecret := newOauthConfigSecret(randomBits)
	oauthclient := &oauthv1.OAuthClient{
		TypeMeta: metav1.TypeMeta{
			APIVersion: oauthv1.GroupVersion.String(),
			Kind: "OAuthClient",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: oauthClientName,
		},
		Secret: randomBits,
	}
	addRedirectURI(oauthclient, rt);
	// NOTE: oauth clients are cluster scoped, OwnerRef will be ingored
	addOwnerRef(oauthConfigSecret, ownerRefFrom(cr))
	return oauthclient, oauthConfigSecret
}

func UpdateOauthClient(cr *v1alpha1.Console, rt *routev1.Route) {
	oauthClient, _ := newConsoleOauthClient(cr, rt)
	addOwnerRef(oauthClient, ownerRefFrom(cr))
	sdk.Update(oauthClient)
}

func DeleteOauthClient() {
	oauthClient, _ := newConsoleOauthClient(nil, nil)
	err := sdk.Delete(oauthClient)
	if err != nil {
		logrus.Error("Failed to delete oauth client")
	}
}