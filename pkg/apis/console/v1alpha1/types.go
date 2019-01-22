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
	Customization Customization `json:"customization,omitempty"`
	Overrides     Overrides     `json:"overrides,omitempty"`
}

type ConsoleStatus struct {
	v1alpha1.OperatorStatus
	// set once the router has a default host name
	DefaultHostName string `json:"defaultHostName"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Console `json:"items"`
}

type Overrides struct {
	// NodeSelector allows the console to be scheduled on nodes besides master nodes.
	// +optional
	NodeSelector metav1.LabelSelector `json:"nodeSelector,omitempty"`
}

type Customization struct {
	Branding             Brand  `json:"branding"`
	DocumentationBaseURL string `json:"documentationBaseURL"`
}

type Brand string

const (
	BrandOKD       Brand = "okd"
	BrandOnline    Brand = "online"
	BrandOCP       Brand = "ocp"
	BrandDedicated Brand = "dedicated"
)
