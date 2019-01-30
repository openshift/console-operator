package v1alpha1

import (
	"github.com/openshift/api/operator/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleOperatorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConsoleOperatorConfigSpec   `json:"spec,omitempty"`
	Status ConsoleOperatorConfigStatus `json:"status,omitempty"`
}

type ConsoleOperatorConfigSpec struct {
	v1alpha1.OperatorSpec `json:",inline"`
	Customization         Customization `json:"customization,omitempty"`
}

type ConsoleOperatorConfigStatus struct {
	v1alpha1.OperatorStatus `json:",inline"`
}

type Customization struct {
	Brand                Brand  `json:"brand,omitempty"`
	DocumentationBaseURL string `json:"documentationBaseURL,omitempty"`
}

type Brand string

const (
	BrandOpenShift Brand = "openshift"
	BrandOKD       Brand = "okd"
	BrandOnline    Brand = "online"
	BrandOCP       Brand = "ocp"
	BrandDedicated Brand = "dedicated"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ConsoleOperatorConfig `json:"items"`
}
