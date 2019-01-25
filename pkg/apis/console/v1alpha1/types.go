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
	Customization Customization `json:"customization,omitempty"`
	Overrides     Overrides     `json:"overrides,omitempty"`
}

type ConsoleOperatorConfigStatus struct {
	v1alpha1.OperatorStatus
	// set once the router has a default host name
	DefaultHostName string `json:"defaultHostName"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ConsoleOperatorConfig `json:"items"`
}

type Overrides struct {
	// NodeSelector allows the console to be scheduled on nodes besides master nodes.
	// +optional
	NodeSelector metav1.LabelSelector `json:"nodeSelector,omitempty"`
}

type Customization struct {
	Brand                Brand  `json:"brand"`
	DocumentationBaseURL string `json:"documentationBaseURL"`
}

type Brand string

const (
	BrandOpenshift Brand = "openshift"
	BrandOKD       Brand = "okd"
	BrandOnline    Brand = "online"
	BrandOCP       Brand = "ocp"
	BrandDedicated Brand = "dedicated"
)
