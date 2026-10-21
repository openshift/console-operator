package proxyconfig

import (
	"os"
	"slices"
)

// Config contains proxy routing settings and an optional source CA reference.
type Config struct {
	HTTPProxy     string
	HTTPSProxy    string
	NoProxy       []string
	TrustedCAName string
}

// IsProxyEnabled returns true when either HTTPProxy or HTTPSProxy is set.
func (p Config) IsProxyEnabled() bool {
	return len(p.HTTPProxy) > 0 || len(p.HTTPSProxy) > 0
}

// Finalize adds internal bypass defaults to an active component proxy override.
// It includes KUBERNETES_SERVICE_HOST from the environment when set.
// The resulting NoProxy entries are sorted and deduplicated.
func (p Config) Finalize() Config {
	defaults := []string{".cluster.local", ".cluster.local.", ".svc", ".svc.", "localhost", "localhost.", "127.0.0.1"}
	if kubernetesServiceHost := os.Getenv("KUBERNETES_SERVICE_HOST"); kubernetesServiceHost != "" {
		defaults = append(defaults, kubernetesServiceHost)
	}
	p.NoProxy = slices.Concat(p.NoProxy, defaults)
	slices.Sort(p.NoProxy)
	p.NoProxy = slices.Compact(p.NoProxy)
	return p
}
