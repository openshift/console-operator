package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/api/operator/v1alpha1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleOperatorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConsoleOperatorConfigSpec   `json:"spec,omitempty"`
	Status ConsoleOperatorConfigStatus `json:"status,omitempty"`
}

type ConsoleOperatorConfigSpec struct {
	v1alpha1.OperatorSpec
	// Count is the number of ConsoleOperatorConfig replicas
	Count int32 `json:"count,omitempty"`
	// take a look @:
	// https://github.com/openshift/cluster-image-registry-operator/blob/master/pkg/apis/imageregistry/v1alpha1/types.go#L91-L92
	// DefaultRoute: T|F
	// additional routes config w/secrets
	// Route[]
}

type ConsoleOperatorConfigStatus struct {
	v1alpha1.OperatorStatus
	// set once the router has a default host name
	DefaultHostName string `json:"defaultHostName"`
	OAuthSecret     string `json:"oauthSecret"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ConsoleOperatorConfig `json:"items"`
}
