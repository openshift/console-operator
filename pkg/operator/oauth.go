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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// OAuthClientName = "console-oauth-client"
	// OAuthClientName = "openshift-web-console"
	OAuthClientName = OpenShiftConsoleName
)

func ApplyOAuth(cr *v1alpha1.Console, rt *routev1.Route) (*oauthv1.OAuthClient, *corev1.Secret, error) {

	authClient := defaultOAuthClient(cr)
	if err := sdk.Get(authClient); err != nil {
		return nil, nil, err
	}
	if !hasRedirectUri(authClient, rt) {
		addRedirectURI(authClient, rt)
		if err := sdk.Update(authClient); err != nil {
			return nil, nil, err
		}
	}

	authClientSecretString := authClient.Secret
	authConfigSecret := newOauthConfigSecret(cr, "")
	// ensure we have created a secret before we compare the values for updates
	authConfigSecret, err := getOrCreateSecret(authConfigSecret)
	if err != nil {
		return nil, nil, err
	}

	authConfigSecretData := authConfigSecret.Data["clientSecret"]
	authConfigSecretString := string(authConfigSecretData)

	if !stringSecretsMatch(authClientSecretString, authConfigSecretString) {
		randomBits := randomStringForSecret()
		addSecretToOauthClient(authClient, &randomBits)
		updateOauthConfigSecret(authConfigSecret, randomBits)
		if err := sdk.Update(authClient); err != nil {
			logrus.Printf("oauthclient \"%v\" update error %v \n", authClient.ObjectMeta.Name, err)
			return nil, nil, err
		}
		logrus.Infof("oauthclient \"%s\" updated", authClient.ObjectMeta.Name)

		if err = sdk.Update(authConfigSecret); err != nil {
			logrus.Printf("secret \"%v\" update error %v \n", authConfigSecret.ObjectMeta.Name, err)
			return nil, nil, err
		}
		logrus.Infof("secret \"%s\" updated", authConfigSecret.ObjectMeta.Name)
		return authClient, authConfigSecret, nil
	}

	return authClient, authConfigSecret, nil
}

// Deletes the Console Auth Secret when the Console ManagementState is set to Removed
func DeleteOAuthSecret(cr *v1alpha1.Console) error {
	authConfigSecret := newOauthConfigSecret(cr, "")
	return sdk.Delete(authConfigSecret)
}

// deletes secret & eliminates redirectUris
func NeutralizeOAuthClient(cr *v1alpha1.Console) error {
	authClient := defaultOAuthClient(cr)
	if err := sdk.Get(authClient); err != nil {
		return err
	}
	unusedSecret := randomStringForSecret()
	addSecretToOauthClient(authClient, &unusedSecret)
	authClient.RedirectURIs = []string{}

	if err := sdk.Update(authClient); err != nil {
		logrus.Printf("oauthclient \"%v\" update error %v \n", authClient.ObjectMeta.Name, err)
		return err
	}
	return nil
}

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
	meta.Name = ConsoleOauthConfigName
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
	if host != "" {
		logrus.Infof("oauth redirect URI set to https://%v", host)
		return fmt.Sprintf("https://%s", host)
	}
	logrus.Infof("route host is not yet available for oauth callback")
	return ""
}

func addRedirectURI(oauth *oauthv1.OAuthClient, rt *routev1.Route) {
	if rt != nil {
		oauth.RedirectURIs = []string{}
		oauth.RedirectURIs = append(oauth.RedirectURIs, https(rt.Spec.Host))
		// TODO: remove, only here for dev
		oauth.RedirectURIs = append(oauth.RedirectURIs, "https://127.0.0.1:8443/console/", "https://localhost:9000")
	}
}

func hasRedirectUri(oauth *oauthv1.OAuthClient, rt *routev1.Route) bool {
	for _, uri := range oauth.RedirectURIs {
		if uri == https(rt.Spec.Host) {
			return true
		}
	}
	return false
}

func addSecretToOauthClient(client *oauthv1.OAuthClient, randomBits *string) {
	if randomBits != nil {
		client.Secret = *randomBits
	}
}

func defaultOAuthClient(cr *v1alpha1.Console) *oauthv1.OAuthClient {
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

func getOrCreateSecret(secret *corev1.Secret) (*corev1.Secret, error) {
	err := sdk.Get(secret)
	if err != nil {
		err = sdk.Create(secret)
		if err != nil {
			return nil, err
		}
	}
	return secret, nil
}

func stringSecretsMatch(str1 string, str2 string) bool {
	if str1 == "" {
		return false
	}
	return str1 == str2
}
