package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/api/operator/v1alpha1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Console struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConsoleSpec   `json:"spec,omitempty"`
	Status ConsoleStatus `json:"status,omitempty"`
}

type ConsoleSpec struct {
	v1alpha1.OperatorSpec
	// Replicas is the number of Console replicas
	Replicas int32 `json:"count,omitempty"`
	// default generated route
	DefaultRoute bool `json:"defaultRoute,omitempty"`
	// there may potentially be musltiple custom host names
	Routes []ConsoleConfigRoute `json:"routes,omitempty"`
	// if authenticating with Request Header, OAuth or OpenID identity providers,
	// an external URL is required to destroy single sign-on sessions
	LogoutPublicURL string `json:logoutPublicURL,omitempty`
}

type ConsoleStatus struct {
	v1alpha1.OperatorStatus
	// set once the router has a default host name
	DefaultHostName string `json:"defaultHostName"`
	// list of all hostnames as there may be mulitple serving the console
	HostNames        []ConsoleConfigRoute
	OAuthSecretValid OAuthSecretValidationStatus `json:"oauthSecretValid"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Console `json:"items"`
}

// For each custom host name, we will need a config.
// a route/host will reference a secret in the `openshift-managed` namespace,
// which will contain all of the tls cert information.  The certs will
// need to be read out of the secret and inlined on the route created for
// the specific hostname.
type ConsoleConfigRoute struct {
	Name       string `json:"name"`
	Hostname   string `json:"hostname"`
	SecretName string `json:"secretName"`
}

type OAuthSecretValidationStatus string

const (
	OAuthSecretValid   OAuthSecretValidationStatus = "valid"
	OAuthSecretInvalid OAuthSecretValidationStatus = "invalid"
)
