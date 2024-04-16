package authentication

import (
	"testing"

	"github.com/go-test/deep"
	config "github.com/openshift/api/config/v1"
	"github.com/openshift/console-operator/pkg/api"
)

func TestGetOIDCOCLoginCommand(t *testing.T) {
	type args struct {
		authConfig   *config.Authentication
		apiServerURL string
	}

	tests := []struct {
		name   string
		args   args
		output string
	}{
		{
			name:   "No client",
			args:   args{&config.Authentication{}, ""},
			output: "",
		}, {
			name: "No CLI client",
			args: args{
				&config.Authentication{
					Spec: config.AuthenticationSpec{
						OIDCProviders: []config.OIDCProvider{
							{
								OIDCClients: []config.OIDCClientConfig{
									{
										ComponentNamespace: "some-other-namespace",
										ComponentName:      "some-other-component",
										ClientID:           "some-client-id",
									},
								},
							},
						},
					},
				},
				"",
			},
			output: "",
		}, {
			name: "No issuer URL",
			args: args{
				&config.Authentication{
					Spec: config.AuthenticationSpec{
						OIDCProviders: []config.OIDCProvider{
							{
								OIDCClients: []config.OIDCClientConfig{
									{
										ComponentNamespace: api.TargetNamespace,
										ComponentName:      api.CLIOIDCClientComponentName,
										ClientID:           "some-client-id",
									},
								},
							},
						},
					},
				},
				"",
			},
			output: "",
		}, {
			name: "With CLI client",
			args: args{
				&config.Authentication{
					Spec: config.AuthenticationSpec{
						OIDCProviders: []config.OIDCProvider{
							{
								Issuer: config.TokenIssuer{
									URL: "https://some-issuer-url",
								},
								Name: "some-oidc-provider",
								OIDCClients: []config.OIDCClientConfig{
									{
										ComponentNamespace: api.TargetNamespace,
										ComponentName:      api.CLIOIDCClientComponentName,
										ClientID:           "some-client-id",
									},
								},
							},
						},
					},
				},
				"https://some-api-url",
			},
			output: "oc login https://some-api-url --issuer-url https://some-issuer-url --exec-plugin oc-oidc --client-id some-client-id",
		}, {
			name: "With extra scopes",
			args: args{
				&config.Authentication{
					Spec: config.AuthenticationSpec{
						OIDCProviders: []config.OIDCProvider{
							{
								Issuer: config.TokenIssuer{
									URL: "https://some-issuer-url",
								},
								Name: "some-oidc-provider",
								OIDCClients: []config.OIDCClientConfig{
									{
										ComponentNamespace: api.TargetNamespace,
										ComponentName:      api.CLIOIDCClientComponentName,
										ClientID:           "some-client-id",
										ExtraScopes:        []string{"foo", "bar"},
									},
								},
							},
						},
					},
				},
				"https://some-api-url",
			},
			output: "oc login https://some-api-url --issuer-url https://some-issuer-url --exec-plugin oc-oidc --client-id some-client-id --extra-scopes foo,bar",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := GetOIDCOCLoginCommand(test.args.authConfig, test.args.apiServerURL)
			if diff := deep.Equal(actual, test.output); diff != nil {
				t.Errorf("GetCLILoginCommands() %s", diff)
			}
		})
	}
}
