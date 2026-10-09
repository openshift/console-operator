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
	resolvedProxy := &Config{HTTPProxy: "http://component.example:3128"}
	delegateErr := errors.New("proxy resolution failed")
	gateErr := errors.New("feature gates not yet observed")

	for _, tc := range []struct {
		name                        string
		enabled, disabled, required []configv1.FeatureGateName
		gateErr                     error
		delegateConfig              *Config
		delegateErr                 error
		wantErr                     error
		wantCalls                   int
	}{
		{name: "all required gates enabled, delegate resolves", enabled: required, required: required, delegateConfig: resolvedProxy, wantCalls: 1},
		{name: "a required gate disabled, delegate skipped", enabled: required[:1], disabled: required[1:], required: required},
		{name: "gate observation error propagates", required: required, gateErr: gateErr, wantErr: gateErr},
		{name: "no required gates configured bypasses the check", gateErr: gateErr, delegateConfig: resolvedProxy, wantCalls: 1},
		{name: "delegate error propagates", enabled: required, required: required, delegateErr: delegateErr, wantErr: delegateErr, wantCalls: 1},
		{name: "delegate config absent", enabled: required, required: required, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			accessor := featuregates.NewHardcodedFeatureGateAccessForTesting(tc.enabled, tc.disabled, nil, tc.gateErr)
			delegate := &recordingProxyResolver{config: tc.delegateConfig, err: tc.delegateErr}
			operatorConfig := &operatorv1.Console{}
			got, err := NewGatedProxyResolver(delegate, accessor, tc.required...).ResolveProxy(operatorConfig)

			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if delegate.calls != tc.wantCalls {
				t.Fatalf("delegate calls = %d, want %d", delegate.calls, tc.wantCalls)
			}
			if tc.wantCalls == 0 {
				if got != nil {
					t.Fatalf("gated resolver returned a config without calling the delegate: %#v", got)
				}
				return
			}
			if got != delegate.config || delegate.operatorConfig != operatorConfig {
				t.Fatal("delegate input or result was not passed through unchanged")
			}
		})
	}
}

func TestConsoleProxyResolverReadsCurrentConfig(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
	input := &operatorv1.Console{}
	input.Spec.AuthProxy = operatorv1.ConsoleAuthProxyConfig{
		HTTPProxy: "http://component.example:3128",
		NoProxy:   []string{"idp.example", ".svc"},
		TrustedCA: operatorv1.ConsoleAuthProxyTrustedCAConfigMapReference{Name: "proxy-ca"},
	}
	original := input.DeepCopy()

	got, err := ConsoleProxyResolver{}.ResolveProxy(input)
	want := &Config{
		HTTPProxy:     "http://component.example:3128",
		NoProxy:       []string{".cluster.local", ".cluster.local.", ".svc", ".svc.", "10.0.0.1", "127.0.0.1", "idp.example", "localhost", "localhost."},
		TrustedCAName: "proxy-ca",
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("proxy = %#v, error = %v; want %#v", got, err, want)
	}

	// The resolved config must not alias or mutate the operator configuration.
	got.NoProxy[0] = "changed.example"
	if !reflect.DeepEqual(input, original) {
		t.Fatal("resolver mutated or shared storage with the operator configuration")
	}

	// A spec without an HTTP(S) proxy yields no override.
	input.Spec.AuthProxy = operatorv1.ConsoleAuthProxyConfig{NoProxy: []string{"unused.example"}}
	if got, err := (ConsoleProxyResolver{}).ResolveProxy(input); got != nil || err != nil {
		t.Fatalf("disabled override returned proxy = %#v, error = %v", got, err)
	}
}
