// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1

import (
	consolev1 "github.com/openshift/api/console/v1"
)

// ConsoleSampleSourceApplyConfiguration represents a declarative configuration of the ConsoleSampleSource type for use
// with apply.
type ConsoleSampleSourceApplyConfiguration struct {
	Type            *consolev1.ConsoleSampleSourceType                    `json:"type,omitempty"`
	GitImport       *ConsoleSampleGitImportSourceApplyConfiguration       `json:"gitImport,omitempty"`
	ContainerImport *ConsoleSampleContainerImportSourceApplyConfiguration `json:"containerImport,omitempty"`
}

// ConsoleSampleSourceApplyConfiguration constructs a declarative configuration of the ConsoleSampleSource type for use with
// apply.
func ConsoleSampleSource() *ConsoleSampleSourceApplyConfiguration {
	return &ConsoleSampleSourceApplyConfiguration{}
}

// WithType sets the Type field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Type field is set to the value of the last call.
func (b *ConsoleSampleSourceApplyConfiguration) WithType(value consolev1.ConsoleSampleSourceType) *ConsoleSampleSourceApplyConfiguration {
	b.Type = &value
	return b
}

// WithGitImport sets the GitImport field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the GitImport field is set to the value of the last call.
func (b *ConsoleSampleSourceApplyConfiguration) WithGitImport(value *ConsoleSampleGitImportSourceApplyConfiguration) *ConsoleSampleSourceApplyConfiguration {
	b.GitImport = value
	return b
}

// WithContainerImport sets the ContainerImport field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ContainerImport field is set to the value of the last call.
func (b *ConsoleSampleSourceApplyConfiguration) WithContainerImport(value *ConsoleSampleContainerImportSourceApplyConfiguration) *ConsoleSampleSourceApplyConfiguration {
	b.ContainerImport = value
	return b
}
