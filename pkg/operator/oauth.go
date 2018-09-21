package operator

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
	OAuthClientName = "console-oauth-client"
)

func randomStringForSecret() string {
	return crypto.RandomBitsString(256)
}

func updateOauthConfigSecret(configSecret *corev1.Secret, randomSecret string) {
	configSecret.StringData = map[string]string{
		"clientSecret": randomSecret,
	}
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
	}
	updateOauthConfigSecret(oauthConfigSecret, randomSecret)
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
			Name: OAuthClientName,
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
	}

	if err := sdk.Create(authSecret); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console oauth client secret : %v", err)
		return err
	} else {
		logrus.Info("created console oauth secret ", randomBits)
	}
	return nil
}

func fetchAndUpdateOauthClient(cr *v1alpha1.Console, rt *routev1.Route, randomBits string) error {
	authClient := newConsoleOauthClient(cr)
	err := sdk.Get(authClient)
	if err != nil {
		logrus.Errorf("failed to retrieve oauth client in order to update callback url")
		return err
	}
	addSecretToOauthClient(authClient, &randomBits)
	addRedirectURI(authClient, rt)

	err = sdk.Update(authClient)
	if err != nil {
		logrus.Errorf("failed to update oauth client with secret & callback url")
	}
	return err
}

func fetchAndUpdateOauthSecret(cr *v1alpha1.Console, randomBits string) error {
	authSecret := newOauthConfigSecret(cr, "")
	err := sdk.Get(authSecret)
	if err != nil {
		logrus.Errorf("failed to retrieve oauth secret in order to update callback url")
		return err
	}
	updateOauthConfigSecret(authSecret, randomBits)

	err = sdk.Update(authSecret)
	if err != nil {
		logrus.Errorf("failed to update oauth secret with client secret")
	}
	return err
}

func UpdateOauthClient(cr *v1alpha1.Console, rt *routev1.Route) error {
	randomBits := randomStringForSecret()
	err := fetchAndUpdateOauthClient(cr, rt, randomBits)
	if err != nil {
		return err
	}
	err = fetchAndUpdateOauthSecret(cr, randomBits)
	if err != nil {
		return err
	}
	return nil
}

func DeleteOauthClient() error {
	oauthClient := newConsoleOauthClient(nil)
	err := sdk.Delete(oauthClient)
	if err != nil {
		logrus.Infof("Failed to delete oauth client %v", err)
	}
	return err
}
