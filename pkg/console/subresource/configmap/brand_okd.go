//go:build !ocp
// +build !ocp

package configmap

const (
	DEFAULT_BRAND   = "okd"
	DEFAULT_DOC_URL = "https://docs.okd.io/latest/"
)

// DefaultDocURL returns the documentation base URL for OKD.
// OKD always uses "latest", so no version derivation is needed.
func DefaultDocURL() string {
	return DEFAULT_DOC_URL
}
