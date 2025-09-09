package configmap

import (
	"testing"

	"gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
)

func TestTechPreviewEnabled(t *testing.T) {
	type args struct {
		techPreviewEnabled bool
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Tech preview enabled",
			args: args{
				techPreviewEnabled: true,
			},
			want: true,
		},
		{
			name: "Tech preview disabled",
			args: args{
				techPreviewEnabled: false,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create minimal test configuration
			operatorConfig := &operatorv1.Console{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: operatorv1.ConsoleSpec{},
			}

			consoleConfig := &configv1.Console{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
			}

			authConfig := &configv1.Authentication{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
			}

			infrastructureConfig := &configv1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Status: configv1.InfrastructureStatus{
					APIServerURL: "https://api.test.cluster:6443",
				},
			}

			route := &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name: "console",
				},
				Spec: routev1.RouteSpec{
					Host: "console.test.cluster",
				},
			}

			// Generate configmap with tech preview setting
			cm, _, err := DefaultConfigMap(
				operatorConfig,
				consoleConfig,
				authConfig,
				&corev1.ConfigMap{},
				&corev1.ConfigMap{},
				infrastructureConfig,
				route,
				0,                            // inactivityTimeoutSeconds
				[]*consolev1.ConsolePlugin{}, // availablePlugins
				[]string{"amd64"},            // nodeArchitectures
				[]string{"linux"},            // nodeOperatingSystems
				false,                        // copiedCSVsDisabled
				false,                        // contentSecurityPolicyEnabled
				map[string]string{},          // telemetryConfig
				"console.test.cluster",       // consoleHost
				tt.args.techPreviewEnabled,
			)

			if err != nil {
				t.Errorf("DefaultConfigMap() error = %v", err)
				return
			}

			// Parse the generated config
			var config consoleserver.Config
			err = yaml.Unmarshal([]byte(cm.Data["console-config.yaml"]), &config)
			if err != nil {
				t.Errorf("Failed to unmarshal config: %v", err)
				return
			}

			// Verify tech preview setting
			if config.ClusterInfo.TechPreviewEnabled != tt.want {
				t.Errorf("TechPreviewEnabled = %v, want %v", config.ClusterInfo.TechPreviewEnabled, tt.want)
			}
		})
	}
}
