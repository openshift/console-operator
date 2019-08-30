package configmap

import (
	"testing"

	v1 "github.com/openshift/api/config/v1"

	operator "github.com/openshift/api/operator/v1"

	"github.com/go-test/deep"
)

func TestOnlyFileOrKeySet(t *testing.T) {
	tests := []struct {
		name   string
		input  *operator.Console
		output bool
	}{
		{
			name:   "No custom logo file or key set",
			input:  &operator.Console{},
			output: false,
		}, {
			name: "Both custom logo file and key set",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Name: "custom-logo-file",
							Key:  "img.png",
						},
					},
				},
			},
			output: false,
		}, {
			name: "Custom logo file set but not key",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Name: "custom-logo-file",
						},
					},
				},
			},
			output: true,
		}, {
			name: "Custom logo key set but not file",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Key: "img.png",
						},
					},
				},
			},
			output: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(FileNameOrKeyInconsistentlySet(tt.input), tt.output); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestFileNameNotSet(t *testing.T) {
	tests := []struct {
		name   string
		input  *operator.Console
		output bool
	}{
		{
			name:   "No custom logo file data",
			input:  &operator.Console{},
			output: true,
		}, {
			name: "Custom logo name and key set",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Name: "custom-logo-file",
							Key:  "img.png",
						},
					},
				},
			},
			output: false,
		}, {
			name: "Custom logo name set",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Name: "custom-logo-file",
						},
					},
				},
			},
			output: false,
		}, {
			name: "Custom logo key set",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Key: "img.png",
						},
					},
				},
			},
			output: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(FileNameNotSet(tt.input), tt.output); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestIsRemoved(t *testing.T) {
	tests := []struct {
		name   string
		input  *operator.Console
		output bool
	}{
		{
			name:   "Custom logo has been removed if there is no custom logo file on config",
			input:  &operator.Console{},
			output: true,
		}, {
			name: "Custom logo has not been removed if there is custom logo file config",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Name: "custom-logo-file",
							Key:  "img.png",
						},
					},
				},
			},
			output: false,
		}, {
			name: "Custom logo has not been removed if custom logo file config is partially provided via name",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Name: "custom-logo-file",
						},
					},
				},
			},
			output: false,
		}, {
			name: "Custom logo has not been removed if custom logo file config is partially provided via key",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Key: "img.png",
						},
					},
				},
			},
			output: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(IsRemoved(tt.input), tt.output); diff != nil {
				t.Error(diff)
			}
		})
	}
}
