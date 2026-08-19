//go:build ocp
// +build ocp

package configmap

import "os"

const DEFAULT_BRAND = "ocp"

// DefaultDocURL returns the documentation base URL for OCP, dynamically
// deriving the version from the OPERATOR_IMAGE_VERSION environment variable.
// Falls back to "latest" if the version is unavailable or cannot be parsed.
func DefaultDocURL() string {
	return formatOCPDocURL(os.Getenv("OPERATOR_IMAGE_VERSION"))
}
