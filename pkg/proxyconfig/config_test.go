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
			name:      "HTTP only",
			component: Config{HTTPProxy: "http://component.example:3128"},
			want: Config{
				HTTPProxy: "http://component.example:3128", NoProxy: defaults,
			},
		},
		{
			name:      "HTTPS only with service host",
			component: Config{HTTPSProxy: "http://component.example:3128"},
			host:      "10.0.0.1",
			want: Config{
				HTTPSProxy: "http://component.example:3128",
				NoProxy:    []string{".cluster.local", ".cluster.local.", ".svc", ".svc.", "10.0.0.1", "127.0.0.1", "localhost", "localhost."},
			},
		},
		{
			name: "both URLs and administrator bypass entries",
			component: Config{
				HTTPProxy: "http://http.example:3128", HTTPSProxy: "http://https.example:3128",
				NoProxy: []string{"idp.example", ".svc", "idp.example", "10.0.0.1"},
			},
			host: "10.0.0.1",
			want: Config{
				HTTPProxy: "http://http.example:3128", HTTPSProxy: "http://https.example:3128",
				NoProxy: []string{".cluster.local", ".cluster.local.", ".svc", ".svc.", "10.0.0.1", "127.0.0.1", "idp.example", "localhost", "localhost."},
			},
		},
		{
			name: "service host already in defaults", component: Config{HTTPProxy: "http://component.example:3128"},
			host: "127.0.0.1",
			want: Config{
				HTTPProxy: "http://component.example:3128", NoProxy: defaults,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KUBERNETES_SERVICE_HOST", tc.host)
			got := tc.component.Finalize()
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("finalized = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestFinalizeTrustedCA(t *testing.T) {
	for _, tc := range []struct {
		name      string
		component Config
		wantCA    string
	}{
		{
			name: "component trust", component: Config{HTTPProxy: "http://component.example:3128", TrustedCAName: "component-ca"},
			wantCA: "component-ca",
		},
		{
			name: "component without trust", component: Config{HTTPProxy: "http://component.example:3128"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.component.Finalize()
			if got.TrustedCAName != tc.wantCA {
				t.Fatalf("finalized = %#v, want CA %q", got, tc.wantCA)
			}
		})
	}
}
