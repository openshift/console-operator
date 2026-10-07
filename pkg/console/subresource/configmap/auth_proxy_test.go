package configmap

import (
	"reflect"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
	"github.com/openshift/console-operator/pkg/proxyconfig"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
)

func TestAuthProxyConfigReplacement(t *testing.T) {
	for _, tc := range []struct {
		name             string
		proxy            *proxyconfig.Config
		previousProxy    *proxyconfig.Config
		managedAuthProxy bool
		authType         configv1.AuthenticationType
		noClient         bool
		overrides        []byte
		want             *consoleserver.AuthProxy
	}{
		{name: "HTTP only replaces previous HTTPS and trust", previousProxy: &proxyconfig.Config{HTTPSProxy: "https://previous.example:3128", TrustedCAName: "previous-ca"}, proxy: &proxyconfig.Config{HTTPProxy: "http://component.example:3128", NoProxy: []string{".svc", "idp.example"}}, want: &consoleserver.AuthProxy{HTTPProxy: "http://component.example:3128", NoProxy: []string{".svc", "idp.example"}}},
		{name: "HTTPS and proxy trust", proxy: &proxyconfig.Config{HTTPSProxy: "https://component.example:3128", TrustedCAName: "proxy-ca"}, want: &consoleserver.AuthProxy{HTTPSProxy: "https://component.example:3128", TrustedCAFile: api.AuthProxyCAMountDir + "/" + api.AuthProxyCAFileName}},
		{name: "absent override removes previous block", previousProxy: &proxyconfig.Config{HTTPProxy: "http://previous.example:3128"}},
		{name: "managed proxy fields retain existing merge behavior", managedAuthProxy: true, proxy: &proxyconfig.Config{HTTPProxy: "http://component.example:3128"}, want: &consoleserver.AuthProxy{HTTPProxy: "http://component.example:3128", HTTPSProxy: "http://managed.example:3128"}},
		{name: "integrated auth omits block", authType: configv1.AuthenticationTypeIntegratedOAuth, proxy: &proxyconfig.Config{HTTPProxy: "http://component.example:3128"}},
		{name: "disabled OIDC omits block", noClient: true, proxy: &proxyconfig.Config{HTTPProxy: "http://component.example:3128"}},
		{name: "unsupported overrides remain last", proxy: &proxyconfig.Config{HTTPProxy: "http://component.example:3128"}, overrides: []byte(`{"auth":{"proxy":{"httpsProxy":"http://unsupported.example:3128"}}}`), want: &consoleserver.AuthProxy{HTTPProxy: "http://component.example:3128", HTTPSProxy: "http://unsupported.example:3128"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			op := minimalOperatorConfig()
			op.Spec.UnsupportedConfigOverrides.Raw = tc.overrides
			auth := minimalAuthConfig()
			auth.Spec.Type = configv1.AuthenticationTypeOIDC
			auth.Spec.OIDCProviders = []configv1.OIDCProvider{{Issuer: configv1.TokenIssuer{URL: "https://idp.example", CertificateAuthority: configv1.ConfigMapNameReference{Name: "issuer-ca"}}, OIDCClients: []configv1.OIDCClientConfig{{ComponentNamespace: api.TargetNamespace, ComponentName: api.OpenShiftConsoleName, ClientID: "console"}}}}
			if tc.authType != "" {
				auth.Spec.Type = tc.authType
			}
			if tc.noClient {
				auth.Spec.OIDCProviders[0].OIDCClients = nil
			}
			managed := &corev1.ConfigMap{Data: map[string]string{consoleConfigYamlFile: `proxy:
  services:
    - consoleAPIPath: /api/proxy/plugin/test/
      endpoint: https://plugin.example
`}}
			if tc.managedAuthProxy {
				managed.Data[consoleConfigYamlFile] = "auth:\n  proxy:\n    httpsProxy: http://managed.example:3128\n" + managed.Data[consoleConfigYamlFile]
			}
			before := managed.DeepCopy()
			build := func(proxy *proxyconfig.Config) *corev1.ConfigMap {
				cm, _, err := DefaultConfigMap(op, minimalConsoleConfig(), auth, managed, &corev1.ConfigMap{}, minimalInfrastructureConfig(), minimalRoute(), 0, nil, nil, nil, false, nil, "console.example", false, false, nil, "", nil, proxy)
				if err != nil {
					t.Fatal(err)
				}
				return cm
			}
			if tc.previousProxy != nil {
				previous := build(tc.previousProxy)
				var previousConfig consoleserver.Config
				if err := yaml.Unmarshal([]byte(previous.Data[consoleConfigYamlFile]), &previousConfig); err != nil {
					t.Fatal(err)
				}
				if previousConfig.Auth.Proxy == nil {
					t.Fatal("previous configuration did not contain a proxy override")
				}
			}
			cm := build(tc.proxy)
			var config consoleserver.Config
			if err := yaml.Unmarshal([]byte(cm.Data[consoleConfigYamlFile]), &config); err != nil {
				t.Fatal(err)
			}
			got := config.Auth.Proxy
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("auth.proxy = %#v, want %#v", got, tc.want)
			}
			if tc.authType == "" && !tc.noClient && config.Auth.OAuthEndpointCAFile != api.AuthServerCAMountDir+"/"+api.AuthServerCAFileName {
				t.Fatal("proxy configuration changed issuer trust")
			}
			if len(config.Proxy.Services) != 1 {
				t.Fatal("proxy update removed plugin proxy configuration")
			}
			if !reflect.DeepEqual(managed, before) {
				t.Fatal("config generation modified managed informer data")
			}
			if rebuilt := build(tc.proxy); rebuilt.Data[consoleConfigYamlFile] != cm.Data[consoleConfigYamlFile] {
				t.Fatal("reconciliation is not idempotent")
			}
		})
	}
}

func TestAuthProxyBuilderSnapshotsAndClears(t *testing.T) {
	proxy := &proxyconfig.Config{HTTPProxy: "http://component.example:3128", NoProxy: []string{"idp.example"}}
	builder := &consoleserver.ConsoleServerCLIConfigBuilder{}
	builder.AuthProxy(proxy)
	proxy.NoProxy[0] = "changed.example"
	// The full YAML test above verifies mode gating; this verifies ownership and
	// clearing when the caller reuses a builder during reconciliation.
	auth := &configv1.Authentication{Spec: configv1.AuthenticationSpec{
		Type: configv1.AuthenticationTypeOIDC,
		OIDCProviders: []configv1.OIDCProvider{{OIDCClients: []configv1.OIDCClientConfig{{
			ComponentNamespace: api.TargetNamespace, ComponentName: api.OpenShiftConsoleName,
		}}}},
	}}
	config := builder.AuthConfig(auth, "").Config()
	if config.Auth.Proxy == nil || config.Auth.Proxy.NoProxy[0] != "idp.example" {
		t.Fatal("builder retained mutable input")
	}
	if builder.AuthProxy(nil).Config().Auth.Proxy != nil {
		t.Fatal("reused builder retained proxy override")
	}
}
