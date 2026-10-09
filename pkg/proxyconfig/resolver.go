package proxyconfig

import (
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/configobserver/featuregates"
)

// ProxyResolver resolves an optional proxy override from the operator configuration.
// A nil result means no component override applies.
type ProxyResolver interface {
	ResolveProxy(operatorConfig *operatorv1.Console) (*Config, error)
}

// ConsoleProxyResolver resolves Console's authentication proxy configuration.
type ConsoleProxyResolver struct{}

func (ConsoleProxyResolver) ResolveProxy(operatorConfig *operatorv1.Console) (*Config, error) {
	input := operatorConfig.Spec.AuthProxy
	proxy := Config{
		HTTPProxy:     input.HTTPProxy,
		HTTPSProxy:    input.HTTPSProxy,
		NoProxy:       input.NoProxy,
		TrustedCAName: input.TrustedCA.Name,
	}
	if !proxy.IsProxyEnabled() {
		return nil, nil
	}
	proxy = proxy.Finalize()
	return &proxy, nil
}

// GatedProxyResolver resolves an override only when every required gate is enabled
// in the current feature-gate state. Gate observation errors propagate to the caller.
type GatedProxyResolver struct {
	delegate          ProxyResolver
	gates             []configv1.FeatureGateName
	featureGateAccess featuregates.FeatureGateAccess
}

func NewGatedProxyResolver(delegate ProxyResolver, featureGateAccess featuregates.FeatureGateAccess, gates ...configv1.FeatureGateName) *GatedProxyResolver {
	return &GatedProxyResolver{
		delegate: delegate, gates: gates, featureGateAccess: featureGateAccess,
	}
}

func (r *GatedProxyResolver) ResolveProxy(operatorConfig *operatorv1.Console) (*Config, error) {
	if len(r.gates) == 0 {
		return r.delegate.ResolveProxy(operatorConfig)
	}
	gates, err := r.featureGateAccess.CurrentFeatureGates()
	if err != nil {
		return nil, err
	}
	for _, gate := range r.gates {
		if !gates.Enabled(gate) {
			return nil, nil
		}
	}
	return r.delegate.ResolveProxy(operatorConfig)
}
