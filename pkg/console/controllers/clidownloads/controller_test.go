package clidownloads

import (
	"testing"

	"github.com/go-test/deep"
	v1 "github.com/openshift/api/console/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetPlatformURL(t *testing.T) {
	type args struct {
		baseURL     string
		platform    string
		archiveType string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test assembling linux amd64 specific URL",
			args: args{
				baseURL:     "https://www.example.com/amd64",
				platform:    "linux",
				archiveType: "oc.tar",
			},
			want: "https://www.example.com/amd64/linux/oc.tar",
		},
		{
			name: "Test assembling linux arm64 specific URL",
			args: args{
				baseURL:     "https://www.example.com/arm64",
				platform:    "linux",
				archiveType: "oc.tar",
			},
			want: "https://www.example.com/arm64/linux/oc.tar",
		},
		{
			name: "Test assembling linux ppc64le specific URL",
			args: args{
				baseURL:     "https://www.example.com/ppc64le",
				platform:    "linux",
				archiveType: "oc.tar",
			},
			want: "https://www.example.com/ppc64le/linux/oc.tar",
		},
		{
			name: "Test assembling linux s390x specific URL",
			args: args{
				baseURL:     "https://www.example.com/s390x",
				platform:    "linux",
				archiveType: "oc.tar",
			},
			want: "https://www.example.com/s390x/linux/oc.tar",
		},
		{
			name: "Test assembling mac specific URL",
			args: args{
				baseURL:     "https://www.example.com/amd64",
				platform:    "mac",
				archiveType: "oc.zip",
			},
			want: "https://www.example.com/amd64/mac/oc.zip",
		},
		{
			name: "Test assembling windows 64-bit specific URL",
			args: args{
				baseURL:     "https://www.example.com/amd64",
				platform:    "windows",
				archiveType: "oc.zip",
			},
			want: "https://www.example.com/amd64/windows/oc.zip",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(GetPlatformURL(tt.args.baseURL, tt.args.platform, tt.args.archiveType), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestPlatformBasedOCConsoleCLIDownloads(t *testing.T) {
	type args struct {
		host             string
		arch             string
		cliDownloadsName string
	}
	tests := []struct {
		name string
		args args
		want *v1.ConsoleCLIDownload
	}{
		{
			name: "Test assembling platform specific URL",
			args: args{
				host:             "www.example.com",
				cliDownloadsName: "amd64/oc-cli-downloads",
			},
			want: &v1.ConsoleCLIDownload{
				ObjectMeta: metav1.ObjectMeta{
					Name: "amd64/oc-cli-downloads",
				},
				Spec: v1.ConsoleCLIDownloadSpec{
					Description: `With the OpenShift command line interface, you can create applications and manage OpenShift projects from a terminal.

The oc binary offers the same capabilities as the kubectl binary, but it is further extended to natively support OpenShift Container Platform features.
`,
					DisplayName: "oc - OpenShift Command Line Interface (CLI)",
					Links: []v1.CLIDownloadLink{
						{
							Href: "https://www.example.com/amd64/linux/oc.tar",
							Text: "Download oc for Linux for x86_64",
						},
						{
							Href: "https://www.example.com/arm64/linux/oc.tar",
							Text: "Download oc for Linux for ARM 64",
						},
						{
							Href: "https://www.example.com/ppc64le/linux/oc.tar",
							Text: "Download oc for Linux for IBM Power, little endian",
						},
						{
							Href: "https://www.example.com/s390x/linux/oc.tar",
							Text: "Download oc for Linux for IBM Z",
						},
						{
							Href: "https://www.example.com/amd64/mac/oc.zip",
							Text: "Download oc for Mac for x86_64",
						},
						{
							Href: "https://www.example.com/amd64/windows/oc.zip",
							Text: "Download oc for Windows for x86_64",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(PlatformBasedOCConsoleCLIDownloads(tt.args.host, tt.args.cliDownloadsName), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestGetLicenseURL(t *testing.T) {
	type args struct {
		baseURL     string
		platform    string
		archiveType string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test assembling linux amd64 specific URL for License",
			args: args{
				baseURL:     "https://www.example.com/amd64",
				platform:    "linux",
				archiveType: "LICENSE",
			},
			want: "https://www.example.com/amd64/linux/LICENSE",
		},
		{
			name: "Test assembling linux arm64 specific URL for License",
			args: args{
				baseURL:     "https://www.example.com/arm64",
				platform:    "linux",
				archiveType: "LICENSE",
			},
			want: "https://www.example.com/arm64/linux/LICENSE",
		},
		{
			name: "Test assembling linux ppc64le specific URL for License",
			args: args{
				baseURL:     "https://www.example.com/ppc64le",
				platform:    "linux",
				archiveType: "LICENSE",
			},
			want: "https://www.example.com/ppc64le/linux/LICENSE",
		},
		{
			name: "Test assembling linux s390x specific URL for License",
			args: args{
				baseURL:     "https://www.example.com/s390x",
				platform:    "linux",
				archiveType: "LICENSE",
			},
			want: "https://www.example.com/s390x/linux/LICENSE",
		},
		{
			name: "Test assembling mac specific URL for License",
			args: args{
				baseURL:     "https://www.example.com/amd64",
				platform:    "mac",
				archiveType: "LICENSE",
			},
			want: "https://www.example.com/amd64/mac/LICENSE",
		},
		{
			name: "Test assembling windows 64-bit specific URL for License",
			args: args{
				baseURL:     "https://www.example.com/amd64",
				platform:    "windows",
				archiveType: "LICENSE",
			},
			want: "https://www.example.com/amd64/windows/LICENSE",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(GetPlatformURL(tt.args.baseURL, tt.args.platform, tt.args.archiveType), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestLicenseDownloads(t *testing.T) {
	type args struct {
		host             string
		arch             string
		cliDownloadsName string
	}
	tests := []struct {
		name string
		args args
		want *v1.ConsoleCLIDownload
	}{
		{
			name: "Test assembling platform specific URL for LICENSE",
			args: args{
				host:             "www.example.com",
				cliDownloadsName: "amd64/oc-cli-downloads",
			},
			want: &v1.ConsoleCLIDownload{
				ObjectMeta: metav1.ObjectMeta{
					Name: "amd64/oc-cli-downloads",
				},
				Spec: v1.ConsoleCLIDownloadSpec{
					Description: `Apache License v2.0 for the OpenShift command line interface.`,
					DisplayName: "LICENSE - license of OpenShift Command Line Interface",
					Links: []v1.CLIDownloadLink{
						{
							Href: "https://www.example.com/amd64/linux/LICENSE",
							Text: "Download LICENSE at Linux for x86_64",
						},
						{
							Href: "https://www.example.com/arm64/linux/LICENSE",
							Text: "Download LICENSE at Linux for ARM 64",
						},
						{
							Href: "https://www.example.com/ppc64le/linux/LICENSE",
							Text: "Download LICENSE at Linux for IBM Power, little endian",
						},
						{
							Href: "https://www.example.com/s390x/linux/LICENSE",
							Text: "Download LICENSE at Linux for IBM Z",
						},
						{
							Href: "https://www.example.com/amd64/mac/LICENSE",
							Text: "Download LICENSE at Mac for x86_64",
						},
						{
							Href: "https://www.example.com/amd64/windows/LICENSE",
							Text: "Download LICENSE at Windows for x86_64",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(LicenseDownloads(tt.args.host, tt.args.cliDownloadsName), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}
