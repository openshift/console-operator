package console

import (
	"fmt"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	oauthClientName = "console-oauth-client"
)

func randomStringForSecret() string {
	return crypto.RandomBitsString(256)
}

func newOauthConfigSecret(cr *v1alpha1.Console, randomSecret string) *corev1.Secret {
	meta := sharedMeta()
	meta.Name = consoleOauthConfigName
	oauthConfigSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: meta,
		StringData: map[string]string{
			"clientSecret": randomSecret,
		},
	}
	addOwnerRef(oauthConfigSecret, ownerRefFrom(cr))
	return oauthConfigSecret
}

func https(host string) string {
	return fmt.Sprintf("https://%s", host)
}

func addRedirectURI(oauth *oauthv1.OAuthClient, rt *routev1.Route) {
	if rt != nil {
		if oauth.RedirectURIs != nil {
			oauth.RedirectURIs = []string{}
		}
		oauth.RedirectURIs = append(oauth.RedirectURIs, https(rt.Spec.Host))
	}
}

func addSecretToOauthClient(client *oauthv1.OAuthClient, randomBits *string) {
	if randomBits != nil {
		client.Secret = *randomBits
	}
}

func newConsoleOauthClient(cr *v1alpha1.Console) *oauthv1.OAuthClient {
	oauthclient := &oauthv1.OAuthClient{
		TypeMeta: metav1.TypeMeta{
			APIVersion: oauthv1.GroupVersion.String(),
			Kind:       "OAuthClient",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: oauthClientName,
		},
	}
	return oauthclient
}

func CreateOAuthClient(cr *v1alpha1.Console, rt *routev1.Route) error {
	randomBits := randomStringForSecret()

	authClient := newConsoleOauthClient(cr)
	addSecretToOauthClient(authClient, &randomBits)

	authSecret := newOauthConfigSecret(cr, randomBits)

	if err := sdk.Create(authClient); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console oauth client : %v", err)
		return err
	} else {
		logrus.Info("created console oauth client with secret ", randomBits)
		// logYaml(oauthc)
	}

	if err := sdk.Create(authSecret); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console oauth client secret : %v", err)
		return err
	} else {
		logrus.Info("created console oauth client secret ", randomBits)
		// logYaml(oauths)
	}
	return nil
}

func UpdateOauthClient(cr *v1alpha1.Console, rt *routev1.Route) {
	authClient := newConsoleOauthClient(cr)
	err := sdk.Get(authClient)
	fmt.Printf("Updating OAUTH client, using existing secret >>> %s", authClient.Secret)
	if err != nil {
		logrus.Errorf("failed to retrieve oauth client in order to update callback url")
	}
	addRedirectURI(authClient, rt)
	err = sdk.Update(authClient)
	if err != nil {
		logrus.Errorf("failed to update oauth client callback url")
	}
}

func DeleteOauthClient() {
	oauthClient := newConsoleOauthClient(nil)
	err := sdk.Delete(oauthClient)
	if err != nil {
		logrus.Error("Failed to delete oauth client")
	}
}
