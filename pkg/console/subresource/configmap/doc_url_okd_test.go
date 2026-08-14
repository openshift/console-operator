//go:build !ocp
// +build !ocp

package configmap

import "testing"

func TestDefaultDocURL(t *testing.T) {
	// OKD build: DefaultDocURL always returns the static OKD docs URL,
	// regardless of OPERATOR_IMAGE_VERSION.
	// The OCP code path is covered by TestFormatOCPDocURL and
	// TestExtractMajorMinor in doc_url_test.go.
	const expectedOKDDocURL = "https://docs.okd.io/latest/"

	tests := []struct {
		name                 string
		operatorImageVersion string
	}{
		{name: "returns expected OKD documentation URL"},
		{
			name:                 "OPERATOR_IMAGE_VERSION does not affect OKD build",
			operatorImageVersion: "5.0.3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.operatorImageVersion != "" {
				t.Setenv("OPERATOR_IMAGE_VERSION", tt.operatorImageVersion)
			}
			got := DefaultDocURL()
			if got != expectedOKDDocURL {
				t.Errorf("DefaultDocURL() = %q, want %q", got, expectedOKDDocURL)
			}
		})
	}
}
