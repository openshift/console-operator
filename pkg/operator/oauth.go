package operator

import (
	"fmt"
	"strings"

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
	if str2 == "" {
		return false
	}
	return strings.Compare(str1, str2) == 0
}

func UpdateOauthClientIfNotInSync(cr *v1alpha1.Console, rt *routev1.Route) (*oauthv1.OAuthClient, *corev1.Secret, error) {
	authClient := newConsoleOauthClient(cr)
	_ = sdk.Get(authClient)
	if !hasRedirectUri(authClient, rt) {
		addRedirectURI(authClient, rt)
		err := sdk.Update(authClient)
		if err != nil {
			return nil, nil, err
		}
	}

	authClientSecretString := authClient.Secret

	authConfigSecret := newOauthConfigSecret(cr, "")
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
		err := sdk.Update(authClient)
		if err != nil {
			logrus.Printf("oauthclient \"%v\" update error %v \n", authClient.ObjectMeta.Name, err)
			return nil, nil, err
		}
		logrus.Infof("oauthclient \"%s\" updated", authClient.ObjectMeta.Name)
		err = sdk.Update(authConfigSecret)
		if err != nil {
			logrus.Printf("secret \"%v\" update error %v \n", authConfigSecret.ObjectMeta.Name, err)
			return nil, nil, err
		}
		logrus.Infof("secret \"%s\" updated", authConfigSecret.ObjectMeta.Name)
		return authClient, authConfigSecret, nil
	}

	return authClient, authConfigSecret, nil
}

// NOTE: the oauthclient is now created via manifests, the operator
// should likely no longer be responsible for deleting it.
func DeleteOauthClient() error {
	oauthClient := newConsoleOauthClient(nil)
	err := sdk.Delete(oauthClient)
	if err != nil {
		logrus.Infof("Failed to delete oauth client %v", err)
	}
	return err
}
