package operator

import (
	"fmt"
	// TODO: use when swapping up to client from Handler
	// "k8s.io/apimachinery/pkg/api/errors"
	// "k8s.io/client-go/kubernetes/typed/core/v1"
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
		logrus.Infof("%v set to https://%v", host, host)
		return fmt.Sprintf("https://%s", host)
	}
	return ""
}

func addRedirectURI(oauth *oauthv1.OAuthClient, rt *routev1.Route) {
	if rt != nil {
		oauth.RedirectURIs = []string{}
		oauth.RedirectURIs = append(oauth.RedirectURIs, https(rt.Spec.Host))
		// TODO: remove, only here for dev
		oauth.RedirectURIs = append(oauth.RedirectURIs, "https://127.0.0.1:8443/console/", "https://localhost:9000")
		fmt.Printf("oauth redirect uris TODO, %v", oauth.RedirectURIs)
	}
}

func hasRedirectUri(oauth *oauthv1.OAuthClient, rt *routev1.Route) bool {
	logrus.Infof("hasRedirectUril() %s", rt.Spec.Host)
	for _, uri := range oauth.RedirectURIs {
		logrus.Infof("matches: %v", uri == https(rt.Spec.Host))
		logrus.Infof("matches?  %v %v", uri, rt.Spec.Host)
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

func stringSecretsMatch(str1 string, str2 string) bool {
	if str1 == "" {
		return false
	}
	if str2 == "" {
		return false
	}
	return strings.Compare(str1, str2) == 0
}

// The console-operator actually does not have permissions to create oauthclients,
// only to get & update.  this is used to generate the values to get.
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

// TODO: update to use new client from Handler
// to improve Get,Update,etc
// func getOrCreateSecret(defaultSecret *corev1.Secret, secretInterface v1.SecretInterface) (*corev1.Secret, error) {
func getOrCreateSecret(secret *corev1.Secret) (*corev1.Secret, error) {
	err := sdk.Get(secret)
	if err != nil {
		err = sdk.Create(secret)
		if err != nil {
			return nil, err
		}
	}
	return secret, nil
	//secret, err := secretInterface.Get(ConsoleOauthConfigName, metav1.GetOptions{})
	//if errors.IsNotFound(err) {
	//	return secretInterface.Create(defaultSecret)
	//}
	//return secret, err
}

// Sync of oauth client + secret consists of the following:
// - get client & secret
//   - if secret doesn't exist
//     - create secret
//   - if both exist
//     - test secret strings match
//     - if they do not match
//       - update secrets
// func UpdateOauthClientIfNotInSync(cr *v1alpha1.Console, rt *routev1.Route, secretInterface v1.SecretInterface) (*oauthv1.OAuthClient, *corev1.Secret, error) {
func UpdateOauthClientIfNotInSync(cr *v1alpha1.Console, rt *routev1.Route) (*oauthv1.OAuthClient, *corev1.Secret, error) {

	authClient := newConsoleOauthClient(cr)
	_ = sdk.Get(authClient)
	if !hasRedirectUri(authClient, rt) {
		addRedirectURI(authClient, rt)
		sdk.Update(authClient)
	}
	authClientSecretString := authClient.Secret

	authConfigSecret := newOauthConfigSecret(cr, "")
	// TODO: update to use new client from Handler
	// authConfigSecret, err := getOrCreateSecret(authConfigSecret, secretInterface)
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
			logrus.Printf("oauth client update error %v \n", err)
			return nil, nil, err
		}
		logrus.Infof("updated oauth client %s", authClient.ObjectMeta.Name)
		err = sdk.Update(authConfigSecret)
		if err != nil {
			logrus.Printf("oauth client secret update error %v \n", err)
			return nil, nil, err
		}
		logrus.Infof("updated oauth client %s", authConfigSecret.ObjectMeta.Name)
		return authClient, authConfigSecret, nil
	}


	return authClient, authConfigSecret, nil
}
