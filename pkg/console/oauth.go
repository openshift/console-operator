package console

import (
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/crypto"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

// the oauth client can be created after the route, once we have a hostname
// - will create a client secret
//   - reference by configmap/deployment
func newConsoleOauthClient(cr *v1alpha1.Console, rt *routev1.Route) *oauthv1.OAuthClient {
	return &oauthv1.OAuthClient{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "oauth.openshift.io/v1",
			Kind: "OAuthClient",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:            "console-oauth-client",
			// logically we own,
			// but namespaced resources cannot own
			// cluster scoped resources
			// OwnerReferences:            nil,
		},
		Secret:                              crypto.RandomBitsString(256),
		// TODO: we need to fill this in from our Route, whenever
		// it gets a .Spec.Host
		//redirectURIs:
		//- http://localhost:9000/auth/callback
		RedirectURIs:                        []string{
			"",
		},
	}
}