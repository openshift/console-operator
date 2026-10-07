package proxyconfig

import (
	"errors"
	"reflect"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/api/features"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/configobserver/featuregates"
)

type recordingProxyResolver struct {
	config         *Config
	err            error
	calls          int
	operatorConfig *operatorv1.Console
}

func (r *recordingProxyResolver) ResolveProxy(operatorConfig *operatorv1.Console) (*Config, error) {
	r.calls++
	r.operatorConfig = operatorConfig
	return r.config, r.err
}

func TestGatedProxyResolver(t *testing.T) {
	required := []configv1.FeatureGateName{features.FeatureGateExternalOIDC, features.FeatureGateAuthenticationComponentProxyExternalOIDC}
	delegateErr := errors.New("proxy resolution failed")
	gateErr := errors.New("feature gates not yet observed")
	for _, tc := range []struct {
		name              string
		enabled, disabled []configv1.FeatureGateName
		gateErr           error
		noRequiredGates   bool
		delegateNil       bool
		delegateErr       error
		wantErr           error
		wantCalls         int
	}{
		{name: "all required gates", enabled: required, wantCalls: 1},
		{name: "unrelated gates disabled", enabled: required, disabled: []configv1.FeatureGateName{features.FeatureGateAuthenticationComponentProxy, features.FeatureGateExternalOIDCAsWebhook}, wantCalls: 1},
		{name: "OIDC gate disabled in current version", enabled: required[1:], disabled: required[:1]},
		{name: "proxy gate disabled in current version", enabled: required[:1], disabled: required[1:]},
		{name: "gate observation error", gateErr: gateErr, wantErr: gateErr},
		{name: "no required gates", noRequiredGates: true, gateErr: gateErr, wantCalls: 1},
		{name: "delegate returns no override", enabled: required, delegateNil: true, wantCalls: 1},
		{name: "delegate error", enabled: required, delegateErr: delegateErr, wantErr: delegateErr, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			accessor := featuregates.NewHardcodedFeatureGateAccessForTesting(tc.enabled, tc.disabled, nil, tc.gateErr)
			delegate := &recordingProxyResolver{err: tc.delegateErr}
			if !tc.delegateNil && tc.delegateErr == nil {
				delegate.config = &Config{HTTPProxy: "http://component.example:3128"}
			}
			gates := required
			if tc.noRequiredGates {
				gates = nil
			}
			resolver := NewGatedProxyResolver(delegate, accessor, gates...)
			operatorConfig := &operatorv1.Console{}
			got, err := resolver.ResolveProxy(operatorConfig)
			if !errors.Is(err, tc.wantErr) || delegate.calls != tc.wantCalls {
				t.Fatalf("error = %v, calls = %d; want error %v, calls %d", err, delegate.calls, tc.wantErr, tc.wantCalls)
			}
			if delegate.calls == 0 {
				if got != nil {
					t.Fatalf("gated resolver returned settings: %#v", got)
				}
			} else if got != delegate.config || delegate.operatorConfig != operatorConfig {
				t.Fatal("delegate input or result was not preserved")
			}
		})
	}
}

type mutableFeatureGateAccess struct {
	featuregates.FeatureGateAccess
	gates featuregates.FeatureGate
}

func (a *mutableFeatureGateAccess) CurrentFeatureGates() (featuregates.FeatureGate, error) {
	return a.gates, nil
}

func TestGatedProxyResolverObservesChanges(t *testing.T) {
	gate := features.FeatureGateAuthenticationComponentProxyExternalOIDC
	accessor := &mutableFeatureGateAccess{}
	delegate := &recordingProxyResolver{config: &Config{HTTPProxy: "http://component.example:3128"}}
	resolver := NewGatedProxyResolver(delegate, accessor, gate)
	for _, enabled := range []bool{true, false, true} {
		wantCalls := 0
		if enabled {
			accessor.gates = featuregates.NewFeatureGate([]configv1.FeatureGateName{gate}, nil)
			wantCalls = 1
		} else {
			accessor.gates = featuregates.NewFeatureGate(nil, []configv1.FeatureGateName{gate})
		}
		delegate.calls = 0
		got, err := resolver.ResolveProxy(&operatorv1.Console{})
		if err != nil || (got != nil) != enabled || delegate.calls != wantCalls {
			t.Fatalf("enabled = %t: proxy = %#v, calls = %d, error = %v", enabled, got, delegate.calls, err)
		}
	}
}

func TestConsoleProxyResolverReadsCurrentConfig(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
	input := &operatorv1.Console{}
	input.Spec.AuthProxy = operatorv1.ConsoleAuthProxyConfig{
		HTTPProxy: "http://component.example:3128", NoProxy: []string{"idp.example", ".svc"},
		TrustedCA: operatorv1.ConsoleAuthProxyTrustedCAConfigMapReference{Name: "proxy-ca"},
	}
	original := input.DeepCopy()
	resolver := ConsoleProxyResolver{}
	got, err := resolver.ResolveProxy(input)
	want := &Config{
		HTTPProxy: "http://component.example:3128", TrustedCAName: "proxy-ca",
		NoProxy: []string{".cluster.local", ".cluster.local.", ".svc", ".svc.", "10.0.0.1", "127.0.0.1", "idp.example", "localhost", "localhost."},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("proxy = %#v, error = %v; want %#v", got, err, want)
	}
	if !reflect.DeepEqual(input, original) {
		t.Fatal("resolver mutated the operator configuration")
	}
	got.NoProxy[0] = "changed.example"
	if !reflect.DeepEqual(input, original) {
		t.Fatal("result shares bypass entries with the operator configuration")
	}
	input.Spec.AuthProxy = operatorv1.ConsoleAuthProxyConfig{NoProxy: []string{"unused.example"}}
	if got, err := resolver.ResolveProxy(input); got != nil || err != nil {
		t.Fatalf("removed override returned proxy = %#v, error = %v", got, err)
	}
}
