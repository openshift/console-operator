package configmap

import (
	"fmt"
	"strings"
)

const ocpDocURLFormat = "https://access.redhat.com/documentation/en-us/openshift_container_platform/%s/"

// formatOCPDocURL returns the OCP documentation base URL for the given version.
// Extracts major.minor from version (e.g., "5.0.3" → "5.0"). Falls back to
// "latest" when the version is empty or cannot be parsed.
func formatOCPDocURL(version string) string {
	return fmt.Sprintf(ocpDocURLFormat, extractMajorMinor(version))
}

// extractMajorMinor extracts the major.minor portion from a version string.
// Returns "latest" if the version is empty or does not contain a dot separator.
func extractMajorMinor(version string) string {
	if version == "" {
		return "latest"
	}
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return "latest"
	}
	return parts[0] + "." + parts[1]
}
