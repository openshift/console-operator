package consoleserver

import (
	"testing"

	"github.com/go-test/deep"
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
				conf1, _ := ConfigYAML()

				b2 := &ConsoleServerCLIConfigBuilder{}
				conf2, _ := ConfigYAML()

				b3 := &ConsoleServerCLIConfigBuilder{}
				StatusPageID("status-12345")
				conf3, _ := ConfigYAML()

				merger := ConsoleYAMLMerger{}
				return Merge(conf1, conf2, conf3)
			},
			output: `apiVersion: console.openshift.io/v1
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  logoutRedirect: https://foobar.com/logout
  oauthEndpointCAFile: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
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
  bindAddress: https://0.0.0.0:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, _ := tt.input()
			if diff := deep.Equal(string(input), tt.output); diff != nil {
				t.Error(diff)
			}
		})
	}
}
