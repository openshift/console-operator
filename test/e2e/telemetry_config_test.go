package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	operatorsv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
	"github.com/openshift/console-operator/pkg/console/telemetry"
	"github.com/openshift/console-operator/test/e2e/framework"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	SEGMENT_API_HOST = "SEGMENT_API_HOST"
	SEGMENT_CDN_URL  = "SEGMENT_CDN_URL"

	defaultSegmentAPIHost = "console.redhat.com/connections/api/v1"
	defaultSegmentCDNURL  = "https://console.redhat.com/connections/cdn"
	patchedSegmentCDNURL  = "https://test.example.com/cdn"
)

func setupTelemetryConfigTestCase(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	return framework.StandardSetup(t)
}

func cleanupTelemetryConfigTestCase(t *testing.T, client *framework.ClientSet) {
	framework.StandardCleanup(t, client)
}

func TestTelemetryConfig(t *testing.T) {
	client, _ := setupTelemetryConfigTestCase(t)
	defer cleanupTelemetryConfigTestCase(t, client)
	telemetryConfigMap, err := client.Core.ConfigMaps(api.OpenShiftConsoleOperatorNamespace).Get(context.TODO(), telemetry.TelemetryConfigMapName, v1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(telemetryConfigMap.Data) == 0 {
		t.Fatal("telemetry-config configmap is empty")
	}

	// check default value for SEGMENT_API_HOST key
	apiHostValue, ok := telemetryConfigMap.Data[SEGMENT_API_HOST]
	if !ok {
		t.Fatalf("telemetry-config configmap does not contain SEGMENT_API_HOST data key. Instead contains: %v", telemetryConfigMap.Data)
	}
	if apiHostValue != defaultSegmentAPIHost {
		t.Fatalf("telemetry-config configmap does not contain SEGMENT_API_HOST key with value '%s'. Instead contains: %q", defaultSegmentAPIHost, apiHostValue)
	}

	// check default value for SEGMENT_CDN_URL key
	cdnURLValue, ok := telemetryConfigMap.Data[SEGMENT_CDN_URL]
	if !ok {
		t.Fatalf("telemetry-config configmap does not contain SEGMENT_CDN_URL data key. Instead contains: %v", telemetryConfigMap.Data)
	}
	if cdnURLValue != defaultSegmentCDNURL {
		t.Fatalf("telemetry-config configmap does not contain SEGMENT_CDN_URL key with value '%s'. Instead contains: %q", defaultSegmentCDNURL, cdnURLValue)
	}

	if _, ok := telemetryConfigMap.Data["SEGMENT_JS_HOST"]; ok {
		t.Fatal("telemetry-config configmap should not contain deprecated SEGMENT_JS_HOST key")
	}

	// update the default value for SEGMENT_API_HOST key
	_, err = client.Core.ConfigMaps(api.OpenShiftConsoleOperatorNamespace).Patch(context.TODO(), telemetry.TelemetryConfigMapName, types.MergePatchType, []byte(`{"data": {"SEGMENT_API_HOST": "test"}}`), metav1.PatchOptions{})
	if err != nil {
		t.Fatal(err)
	}

	err = wait.Poll(1*time.Second, framework.AsyncOperationTimeout, func() (stop bool, err error) {
		telemetryConfigMap, err := client.Core.ConfigMaps(api.OpenShiftConsoleOperatorNamespace).Get(context.TODO(), telemetry.TelemetryConfigMapName, v1.GetOptions{})
		if err != nil {
			return false, err
		}
		value, ok := telemetryConfigMap.Data[SEGMENT_API_HOST]
		if !ok {
			return false, fmt.Errorf("updated telemetry-config configmap does not contain SEGMENT_API_HOST data key. Instead contains: %v", telemetryConfigMap.Data)
		}
		if value != "test" {
			return false, fmt.Errorf("updated telemetry-config configmap does not contain SEGMENT_API_HOST key with value 'test'. Instead contains: %q", value)
		}

		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// update the default value for SEGMENT_CDN_URL key
	_, err = client.Core.ConfigMaps(api.OpenShiftConsoleOperatorNamespace).Patch(context.TODO(), telemetry.TelemetryConfigMapName, types.MergePatchType, []byte(fmt.Sprintf(`{"data": {"SEGMENT_CDN_URL": "%s"}}`, patchedSegmentCDNURL)), metav1.PatchOptions{})
	if err != nil {
		t.Fatal(err)
	}

	err = wait.Poll(1*time.Second, framework.AsyncOperationTimeout, func() (stop bool, err error) {
		consoleConfigMap, err := framework.GetConsoleConfigMap(client)
		if err != nil {
			return false, err
		}
		configYAML, ok := consoleConfigMap.Data["console-config.yaml"]
		if !ok {
			return false, fmt.Errorf("console-config configmap does not contain console-config.yaml data key")
		}
		var consoleConfig consoleserver.Config
		if err := yaml.Unmarshal([]byte(configYAML), &consoleConfig); err != nil {
			return false, fmt.Errorf("failed to unmarshal console-config.yaml: %w", err)
		}
		if consoleConfig.Telemetry == nil {
			return false, fmt.Errorf("console-config.yaml telemetry section is missing")
		}
		value, ok := consoleConfig.Telemetry[SEGMENT_CDN_URL]
		if !ok {
			return false, fmt.Errorf("console-config.yaml telemetry does not contain SEGMENT_CDN_URL key. Instead contains: %v", consoleConfig.Telemetry)
		}
		if value != patchedSegmentCDNURL {
			return false, fmt.Errorf("console-config.yaml telemetry SEGMENT_CDN_URL expected '%s', got %q", patchedSegmentCDNURL, value)
		}

		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
