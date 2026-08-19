package configmap

import "testing"

func TestFormatOCPDocURL(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "full release version",
			version: "5.0.3",
			want:    "https://access.redhat.com/documentation/en-us/openshift_container_platform/5.0/",
		},
		{
			name:    "two-part version",
			version: "4.21",
			want:    "https://access.redhat.com/documentation/en-us/openshift_container_platform/4.21/",
		},
		{
			name:    "nightly pre-release version",
			version: "5.1.0-0.nightly-2026-01-01-000000",
			want:    "https://access.redhat.com/documentation/en-us/openshift_container_platform/5.1/",
		},
		{
			name:    "rc version",
			version: "4.19.0-rc.1",
			want:    "https://access.redhat.com/documentation/en-us/openshift_container_platform/4.19/",
		},
		{
			name:    "empty version falls back to latest",
			version: "",
			want:    "https://access.redhat.com/documentation/en-us/openshift_container_platform/latest/",
		},
		{
			name:    "single number without dot falls back to latest",
			version: "5",
			want:    "https://access.redhat.com/documentation/en-us/openshift_container_platform/latest/",
		},
		{
			name:    "non-numeric dotted version falls back to latest",
			version: "invalid.version",
			want:    "https://access.redhat.com/documentation/en-us/openshift_container_platform/latest/",
		},
		{
			name:    "trailing dot falls back to latest",
			version: "5.",
			want:    "https://access.redhat.com/documentation/en-us/openshift_container_platform/latest/",
		},
		{
			name:    "non-numeric minor falls back to latest",
			version: "5.x",
			want:    "https://access.redhat.com/documentation/en-us/openshift_container_platform/latest/",
		},
		{
			name:    "leading dot falls back to latest",
			version: ".5",
			want:    "https://access.redhat.com/documentation/en-us/openshift_container_platform/latest/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatOCPDocURL(tt.version)
			if got != tt.want {
				t.Errorf("formatOCPDocURL(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestExtractMajorMinor(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "standard three-part version",
			version: "5.0.3",
			want:    "5.0",
		},
		{
			name:    "two-part version",
			version: "4.21",
			want:    "4.21",
		},
		{
			name:    "nightly build version",
			version: "5.1.0-0.nightly-2026-01-01-000000",
			want:    "5.1",
		},
		{
			name:    "release candidate",
			version: "4.19.0-rc.1",
			want:    "4.19",
		},
		{
			name:    "empty string",
			version: "",
			want:    "latest",
		},
		{
			name:    "single number without dot",
			version: "5",
			want:    "latest",
		},
		{
			name:    "non-numeric dotted version",
			version: "invalid.version",
			want:    "latest",
		},
		{
			name:    "trailing dot with empty minor",
			version: "5.",
			want:    "latest",
		},
		{
			name:    "non-numeric minor component",
			version: "5.x",
			want:    "latest",
		},
		{
			name:    "leading dot with empty major",
			version: ".5",
			want:    "latest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMajorMinor(tt.version)
			if got != tt.want {
				t.Errorf("extractMajorMinor(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}
