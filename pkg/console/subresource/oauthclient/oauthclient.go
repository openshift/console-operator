package oauthclient

import (
	oauthv1 "github.com/openshift/api/oauth/v1"
	"github.com/openshift/api/route/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/console-operator/pkg/controller"
	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: ApplyOauth should be a generic Apply that could be used for any oauth-client
// - should look like resourceapply.ApplyService and the other Apply funcs
//   once its in a trustworthy state, PR to library-go so it can live with
//   the other Apply funcs
func ApplyOAuth(client oauthclient.OAuthClientsGetter, required *oauthv1.OAuthClient) (*oauthv1.OAuthClient, bool, error) {
	existing, err := client.OAuthClients().Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.OAuthClients().Create(required)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}
	// Unfortunately data is all top level so its a little more
	// tedious to manually copy things over
	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	existing.Secret = required.Secret
	existing.AdditionalSecrets = required.AdditionalSecrets
	existing.RespondWithChallenges = required.RespondWithChallenges
	existing.RedirectURIs = required.RedirectURIs
	existing.GrantMethod = required.GrantMethod
	existing.ScopeRestrictions = required.ScopeRestrictions
	existing.AccessTokenMaxAgeSeconds = required.AccessTokenMaxAgeSeconds
	existing.AccessTokenInactivityTimeoutSeconds = required.AccessTokenInactivityTimeoutSeconds

	actual, err := client.OAuthClients().Update(existing)
	return actual, true, err
}

// registers the console on the oauth client as a valid application
func RegisterConsoleToOAuthClient(client *oauthv1.OAuthClient, route *v1.Route, randomBits string) *oauthv1.OAuthClient {
	// without a route, we cannot create a usable oauth client
	if route == nil {
		return nil
	}
	// we are the only application for this client
	// in the future we may accept multiple routes
	client.RedirectURIs = []string{}
	client.RedirectURIs = append(client.RedirectURIs, util.HTTPS(route.Spec.Host))
	// client.Secret = randomBits
	client.Secret = string(randomBits)
	return client
}

// for ManagementState.Removed
// Console does not have create/delete priviledges on oauth clients, only update
func DeRegisterConsoleFromOAuthClient(client *oauthv1.OAuthClient) *oauthv1.OAuthClient {
	client.RedirectURIs = []string{}
	// changing the string to anything else will invalidate the client
	client.Secret = crypto.Random256BitsString()
	return client
}

func DefaultOauthClient() *oauthv1.OAuthClient {
	return Stub()
}

func Stub() *oauthv1.OAuthClient {
	// we cannot set an ownerRef on the OAuthClient as it is cluster scoped
	return &oauthv1.OAuthClient{
		ObjectMeta: metav1.ObjectMeta{
			Name: controller.OpenShiftConsoleName,
		},
	}
}

func GetSecretString(client *oauthv1.OAuthClient) string {
	return client.Secret
}
