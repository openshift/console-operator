package proxyconfig

import (
	"reflect"
	"testing"
)

func TestFinalize(t *testing.T) {
	defaults := []string{".cluster.local", ".cluster.local.", ".svc", ".svc.", "127.0.0.1", "localhost", "localhost."}
	for _, tc := range []struct {
		name      string
		component Config
		host      string
		want      Config
	}{
		{
			name:      "adds internal bypass defaults",
			component: Config{HTTPProxy: "http://component.example:3128"},
			want:      Config{HTTPProxy: "http://component.example:3128", NoProxy: defaults},
		},
		{
			name: "merges service host and caller entries, sorted and deduplicated",
			component: Config{
				HTTPSProxy:    "http://component.example:3128",
				NoProxy:       []string{"idp.example", ".svc", "idp.example", "10.0.0.1"},
				TrustedCAName: "proxy-ca",
			},
			host: "10.0.0.1",
			want: Config{
				HTTPSProxy:    "http://component.example:3128",
				NoProxy:       []string{".cluster.local", ".cluster.local.", ".svc", ".svc.", "10.0.0.1", "127.0.0.1", "idp.example", "localhost", "localhost."},
				TrustedCAName: "proxy-ca",
			},
		},
		{
			name:      "service host already among defaults is deduplicated",
			component: Config{HTTPProxy: "http://component.example:3128"},
			host:      "127.0.0.1",
			want:      Config{HTTPProxy: "http://component.example:3128", NoProxy: defaults},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KUBERNETES_SERVICE_HOST", tc.host)
			if got := tc.component.Finalize(); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("finalized = %#v, want %#v", got, tc.want)
			}
		})
	}
}
