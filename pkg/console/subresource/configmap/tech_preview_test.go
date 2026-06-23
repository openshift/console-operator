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
			name: "Technology Preview enabled",
			args: args{
				techPreviewEnabled: true,
			},
			want: true,
		},
		{
			name: "Technology Preview disabled",
			args: args{
				techPreviewEnabled: false,
			},
			want: false,
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
				tt.args.techPreviewEnabled,
				false, // olmLifecycleMetadataEnabled
			)

			if err != nil {
				t.Errorf("DefaultConfigMap() error = %v.", err)
				return
			}

			var config consoleserver.Config
			err = yaml.Unmarshal([]byte(cm.Data["console-config.yaml"]), &config)
			if err != nil {
				t.Errorf("Failed to unmarshal config: %v.", err)
				return
			}

			if config.ClusterInfo.TechPreviewEnabled != tt.want {
				t.Errorf("TechPreviewEnabled: got %t, want %t (case %q, techPreviewEnabled input=%t).", config.ClusterInfo.TechPreviewEnabled, tt.want, tt.name, tt.args.techPreviewEnabled)
			}
		})
	}
}

func TestOLMLifecycleMetadataEnabled(t *testing.T) {
	type args struct {
		olmLifecycleMetadataEnabled bool
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "OLM Lifecycle Metadata enabled",
			args: args{
				olmLifecycleMetadataEnabled: true,
			},
			want: true,
		},
		{
			name: "OLM Lifecycle Metadata disabled",
			args: args{
				olmLifecycleMetadataEnabled: false,
			},
			want: false,
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
				tt.args.olmLifecycleMetadataEnabled,
			)

			if err != nil {
				t.Errorf("DefaultConfigMap() error = %v.", err)
				return
			}

			var config consoleserver.Config
			err = yaml.Unmarshal([]byte(cm.Data["console-config.yaml"]), &config)
			if err != nil {
				t.Errorf("Failed to unmarshal config: %v.", err)
				return
			}

			if config.ClusterInfo.OLMLifecycleMetadataEnabled != tt.want {
				t.Errorf("OLMLifecycleMetadataEnabled: got %t, want %t (case %q, olmLifecycleMetadataEnabled input=%t).", config.ClusterInfo.OLMLifecycleMetadataEnabled, tt.want, tt.name, tt.args.olmLifecycleMetadataEnabled)
			}
		})
	}
}

func minimalOperatorConfig() *operatorv1.Console {
	return &operatorv1.Console{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec:       operatorv1.ConsoleSpec{},
	}
}

func minimalConsoleConfig() *configv1.Console {
	return &configv1.Console{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}
}

func minimalAuthConfig() *configv1.Authentication {
	return &configv1.Authentication{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}
}

func minimalInfrastructureConfig() *configv1.Infrastructure {
	return &configv1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Status: configv1.InfrastructureStatus{
			APIServerURL: "https://api.test.cluster:6443",
		},
	}
}

func minimalRoute() *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "console"},
		Spec:       routev1.RouteSpec{Host: "console.test.cluster"},
	}
}
