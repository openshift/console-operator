package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConsoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Console `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Console struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ConsoleSpec   `json:"spec"`
	Status            ConsoleStatus `json:"status,omitempty"`
}

// if/when changed, be sure to regenerate generated code:
// 	operator-sdk generate k8s
type ConsoleSpec struct {
	// Count is the number of Console replicas
	// Default: 1
	Count int32 `json:"count,omitempty"`
	Image string `json:"image"`
}
type ConsoleStatus struct {
	// Fill me
}

// good idea to set the defaults
// https://github.com/operator-framework/operator-sdk-samples/blob/master/vault-operator/pkg/apis/vault/v1alpha1/types.go#L39
func (c *Console) SetDefaults() bool {
	changed := false
	if c.Spec.Count == 0 {
		c.Spec.Count = 1
		changed = true
	}
	return changed
}

// may want to create secret names here, etc.
// https://github.com/operator-framework/operator-sdk-samples/blob/master/vault-operator/pkg/apis/vault/v1alpha1/types.go#L65