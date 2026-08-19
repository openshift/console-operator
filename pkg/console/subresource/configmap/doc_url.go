package configmap

import (
	"fmt"
	"strconv"
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
// Returns "latest" if the version is empty, does not contain a dot separator,
// or if the major/minor components are not valid decimal numbers.
func extractMajorMinor(version string) string {
	if version == "" {
		return "latest"
	}
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return "latest"
	}
	major := parts[0]
	minor := parts[1]
	if _, err := strconv.Atoi(major); err != nil {
		return "latest"
	}
	if _, err := strconv.Atoi(minor); err != nil {
		return "latest"
	}
	return major + "." + minor
}
