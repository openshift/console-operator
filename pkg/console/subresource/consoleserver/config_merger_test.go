package consoleserver

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/openshift/api/operator/v1"
)

func TestConfigMerger(t *testing.T) {
	tests := []struct {
		name   string
		input  func() ([]byte, error)
		output string
	}{
		{
			name: "Merger should accept several configs and return a single merged config",
			input: func() ([]byte, error) {
				b1 := &ConsoleServerCLIConfigBuilder{}
				conf1, _ := b1.ConfigYAML()

				b2 := &ConsoleServerCLIConfigBuilder{}
				conf2, _ := b2.
					APIServerURL("https://shizzlepop.com/api").
					Host("https://console-openshift-console.apps.shizzlepop.com").
					LogoutURL("https://shizzlepop.com/logout").
					ConfigYAML()

				b3 := &ConsoleServerCLIConfigBuilder{}
				b3.
					Host("https://console-openshift-console.apps.foobar.com").
					LogoutURL("https://foobar.com/logout").
					Brand(v1.BrandOKD).
					DocURL("https://foobar.com/docs").
					APIServerURL("https://foobar.com/api").
					StatusPageID("status-12345")
				conf3, _ := b3.ConfigYAML()

				merger := ConsoleYAMLMerger{}
				return merger.Merge(conf1, conf2, conf3)
			},
			output: `apiVersion: console.openshift.io/v1
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  logoutRedirect: https://foobar.com/logout
clusterInfo:
  consoleBaseAddress: https://console-openshift-console.apps.foobar.com
  masterPublicURL: https://foobar.com/api
customization:
  branding: okd
  documentationBaseURL: https://foobar.com/docs
kind: ConsoleConfig
providers:
  statuspageID: status-12345
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
session: {}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, _ := tt.input()
			if diff := cmp.Diff(tt.output, string(input)); len(diff) > 0 {
				t.Error(diff)
			}
		})
	}
}
