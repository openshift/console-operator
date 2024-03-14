package configmap

import (
	"github.com/openshift/console-operator/pkg/api"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// Partial of https://github.com/openshift/cluster-monitoring-operator/blob/255482d7f08a8327da24d45343a87fbe9773eeb0/pkg/manifests/types.go#L263
type TelemeterClientConfig struct {
	ClusterID string `yaml:"clusterID,omitempty"`
	Enabled   *bool  `yaml:"enabled,omitempty"`
	Token     string `yaml:"token,omitempty"`
}

// Partial of https://github.com/openshift/cluster-monitoring-operator/blob/255482d7f08a8327da24d45343a87fbe9773eeb0/pkg/manifests/types.go#L36C6-L36C36
type ClusterMonitoringConfiguration struct {
	TelemeterClientConfig *TelemeterClientConfig `yaml:"telemeterClient,omitempty"`
}

func GetTelemeterClientConfig(cm *v1.ConfigMap) *TelemeterClientConfig {
	cfg := &ClusterMonitoringConfiguration{}

	if cm == nil || cm.Data == nil {
		return nil
	}

	content, ok := cm.Data[api.ClusterMonitoringConfigMapKey]
	if !ok || content == "" {
		return nil
	}

	if err := yaml.Unmarshal([]byte(content), cfg); err != nil {
		klog.Errorf("failed to unmarshal cluster monitoring config: %v.", err)
		return nil
	}

	return cfg.TelemeterClientConfig
}

func TelemeterClientIsEnabled(cm *v1.ConfigMap) bool {
	c := GetTelemeterClientConfig(cm)
	if c == nil {
		return false
	}

	if (c.Enabled != nil && !*c.Enabled) ||
		c.ClusterID == "" ||
		c.Token == "" {
		return false
	}
	return true
}
