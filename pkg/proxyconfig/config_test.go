package proxyconfig

import (
	"reflect"
	"testing"
)

func TestFinalize(t *testing.T) {
	defaults := []string{".cluster.local", ".cluster.local.", ".svc", ".svc.", "127.0.0.1", "localhost", "localhost."}
	for _, tc := range []struct {
		name                  string
		sourceSonfig          Config
		kubernetesServiceHost string
		wantConfig            Config
	}{
		{
			name:         "adds internal bypass defaults",
			sourceSonfig: Config{HTTPProxy: "http://component.example:3128"},
			wantConfig:   Config{HTTPProxy: "http://component.example:3128", NoProxy: defaults},
		},
		{
			name: "merges service host and caller entries, sorted and deduplicated",
			sourceSonfig: Config{
				HTTPSProxy:    "http://component.example:3128",
				NoProxy:       []string{"idp.example", ".svc", "idp.example", "10.0.0.1"},
				TrustedCAName: "proxy-ca",
			},
			kubernetesServiceHost: "10.0.0.1",
			wantConfig: Config{
				HTTPSProxy:    "http://component.example:3128",
				NoProxy:       []string{".cluster.local", ".cluster.local.", ".svc", ".svc.", "10.0.0.1", "127.0.0.1", "idp.example", "localhost", "localhost."},
				TrustedCAName: "proxy-ca",
			},
		},
		{
			name:                  "service host already among defaults is deduplicated",
			sourceSonfig:          Config{HTTPProxy: "http://component.example:3128"},
			kubernetesServiceHost: "127.0.0.1",
			wantConfig:            Config{HTTPProxy: "http://component.example:3128", NoProxy: defaults},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KUBERNETES_SERVICE_HOST", tc.kubernetesServiceHost)
			if got := tc.sourceSonfig.Finalize(); !reflect.DeepEqual(got, tc.wantConfig) {
				t.Fatalf("finalized = %#v, want %#v", got, tc.wantConfig)
			}
		})
	}
}
