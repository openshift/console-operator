package configmap

import (
	"fmt"
	"testing"

	"github.com/go-test/deep"
	v1 "k8s.io/api/core/v1"
)

const monitoringConfigKey string = "config.yaml"

func getMockClusterMonitoringConfigMap(content string) *v1.ConfigMap {
	return &v1.ConfigMap{
		Data: map[string]string{monitoringConfigKey: content},
	}
}

var (
	f                              = false
	enabledClusterMonitoringConfig = getMockClusterMonitoringConfigMap(`telemeterClient:
  clusterID: clusterID
  token: abc123`)
	disabledClusterMonitoringConfig = getMockClusterMonitoringConfigMap(`telemeterClient:
  clusterID: clusterID
  enabled: false
  token: abc123`)
	missingTokenClusterMonitoringConfig = getMockClusterMonitoringConfigMap(`telemeterClient:
  clusterID: clusterID`)
	missingClusterIDClusterMonitoringConfig = getMockClusterMonitoringConfigMap(`telemeterClient:
  token: abc123`)
	missingConfigClusterMonitoringConfig = &v1.ConfigMap{Data: map[string]string{"foo": "bar"}}
)

func TestGetTelemeterClientConfig(t *testing.T) {
	tests := []struct {
		name      string
		configMap *v1.ConfigMap
		expected  *TelemeterClientConfig
	}{
		{
			name:      "telemeter client is enabled",
			configMap: enabledClusterMonitoringConfig,
			expected:  &TelemeterClientConfig{ClusterID: "clusterID", Token: "abc123"},
		},
		{
			name:      "telemeter client is disabled",
			configMap: disabledClusterMonitoringConfig,
			expected:  &TelemeterClientConfig{ClusterID: "clusterID", Token: "abc123", Enabled: &f},
		},
		{
			name:      "telemeter client is missing token",
			configMap: missingTokenClusterMonitoringConfig,
			expected:  &TelemeterClientConfig{ClusterID: "clusterID"},
		},
		{
			name:      "telemeter client is missing clusterID",
			configMap: missingClusterIDClusterMonitoringConfig,
			expected:  &TelemeterClientConfig{Token: "abc123"},
		},
		{
			name:      "telemeter client is missing config.yaml",
			configMap: missingConfigClusterMonitoringConfig,
			expected:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := GetTelemeterClientConfig(tt.configMap)
			diff := deep.Equal(actual, tt.expected)
			if diff != nil {
				fmt.Printf("expected: %+v\nactual: %+v\n", tt.expected, actual)
				t.Error(diff)
			}
		})
	}
}

func TestTelemeterClientIsEnabled(t *testing.T) {
	tests := []struct {
		name      string
		configMap *v1.ConfigMap
		expected  bool
	}{
		{
			name:      "telemeter client is enabled",
			configMap: enabledClusterMonitoringConfig,
			expected:  true,
		},
		{
			name:      "telemeter client is disabled",
			configMap: disabledClusterMonitoringConfig,
			expected:  false,
		},
		{
			name:      "telemeter client is missing token",
			configMap: missingTokenClusterMonitoringConfig,
			expected:  false,
		},
		{
			name:      "telemeter client is missing clusterID",
			configMap: missingClusterIDClusterMonitoringConfig,
			expected:  false,
		},
		{
			name:      "telemeter client is missing config.yaml",
			configMap: missingConfigClusterMonitoringConfig,
			expected:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := TelemeterClientIsEnabled(tt.configMap)
			if actual != tt.expected {
				t.Errorf("%s: actual = %v, desired %v", tt.name, actual, tt.expected)
			}
		})
	}
}
