package configmap

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
)

func TestTLSConfigInjection(t *testing.T) {
	tests := []struct {
		name          string
		tlsMinVersion configv1.TLSProtocolVersion
		tlsCiphers    []string
		tlsGroups     []string
		wantMinTLS    string
		wantGroups    []string
	}{
		{
			name:          "TLS 1.2 with ciphers and groups",
			tlsMinVersion: configv1.VersionTLS12,
			tlsCiphers:    []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"},
			tlsGroups:     []string{"X25519", "secp256r1"},
			wantMinTLS:    "VersionTLS12",
			wantGroups:    []string{"X25519", "secp256r1"},
		},
		{
			name:          "TLS 1.3 with no custom ciphers",
			tlsMinVersion: configv1.VersionTLS13,
			tlsCiphers:    []string{},
			wantMinTLS:    "VersionTLS13",
		},
		{
			name:          "Intermediate profile ciphers and groups",
			tlsMinVersion: configv1.TLSProfiles[configv1.TLSProfileIntermediateType].MinTLSVersion,
			tlsCiphers:    configv1.TLSProfiles[configv1.TLSProfileIntermediateType].Ciphers,
			tlsGroups:     []string{"X25519MLKEM768", "X25519", "secp256r1", "secp384r1"},
			wantMinTLS:    "VersionTLS12",
			wantGroups:    []string{"X25519MLKEM768", "X25519", "secp256r1", "secp384r1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, _, err := DefaultConfigMap(
				minimalOperatorConfig(),
				minimalConsoleConfig(),
				minimalAuthConfig(),
				&corev1.ConfigMap{},
				&corev1.ConfigMap{},
				minimalInfrastructureConfig(),
				minimalRoute(),
				0,                            // inactivityTimeoutSeconds
				[]*consolev1.ConsolePlugin{}, // availablePlugins
				[]string{"amd64"},            // nodeArchitectures
				[]string{"linux"},            // nodeOperatingSystems
				false,                        // copiedCSVsDisabled
				map[string]string{},          // telemetryConfig
				"console.test.cluster",       // consoleHost
				false,                        // techPreviewEnabled
				false,                        // olmLifecycleMetadataEnabled
				nil,                          // additionalHosts
				tt.tlsMinVersion,
				tt.tlsCiphers,
				tt.tlsGroups,
			)

			if err != nil {
				t.Errorf("DefaultConfigMap() error = %v", err)
				return
			}

			var config consoleserver.Config
			err = yaml.Unmarshal([]byte(cm.Data["console-config.yaml"]), &config)
			if err != nil {
				t.Errorf("Failed to unmarshal config: %v", err)
				return
			}

			// Check MinTLSVersion
			if config.ServingInfo.MinTLSVersion != tt.wantMinTLS {
				t.Errorf("MinTLSVersion = %v, want %v", config.ServingInfo.MinTLSVersion, tt.wantMinTLS)
			}

			// Check CipherSuites
			if len(config.ServingInfo.CipherSuites) != len(tt.tlsCiphers) {
				t.Errorf("CipherSuites count = %v, want %v. Got: %v",
					len(config.ServingInfo.CipherSuites), len(tt.tlsCiphers), config.ServingInfo.CipherSuites)
			}

			// Verify the actual cipher values match
			for i, cipher := range config.ServingInfo.CipherSuites {
				if i < len(tt.tlsCiphers) && cipher != tt.tlsCiphers[i] {
					t.Errorf("CipherSuites[%d] = %v, want %v", i, cipher, tt.tlsCiphers[i])
				}
			}

			if len(config.ServingInfo.Groups) != len(tt.wantGroups) {
				t.Errorf("Groups count = %v, want %v. Got: %v",
					len(config.ServingInfo.Groups), len(tt.wantGroups), config.ServingInfo.Groups)
			}

			for i, group := range config.ServingInfo.Groups {
				if i < len(tt.wantGroups) && group != tt.wantGroups[i] {
					t.Errorf("Groups[%d] = %v, want %v", i, group, tt.wantGroups[i])
				}
			}
		})
	}
}
