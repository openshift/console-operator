package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	operatorsv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/telemetry"
	"github.com/openshift/console-operator/test/e2e/framework"
)

const (
	SEGMENT_API_HOST = "SEGMENT_API_HOST"
)

func setupTelemetryConfigTestCase(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	return framework.StandardSetup(t)
}

func cleanupTelemetryConfigTestCase(t *testing.T, client *framework.ClientSet) {
	framework.StandardCleanup(t, client)
}

func TestTelemetryConfig(t *testing.T) {
	client, _ := setupDownloadsTestCase(t)
	defer cleanupDownloadsTestCase(t, client)
	telemetryConfigMap, err := client.Core.ConfigMaps(api.OpenShiftConsoleOperatorNamespace).Get(context.TODO(), telemetry.TelemetryConfigMapName, v1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(telemetryConfigMap.Data) == 0 {
		t.Fatal("telemetry-config configmap is empty")
	}

	// check default value for SEGMENT_API_HOST key
	value, ok := telemetryConfigMap.Data[SEGMENT_API_HOST]
	if !ok {
		t.Fatalf("telemetry-config configmap does not contain SEGMENT_JS_HOST data key. Instead contains: %v", telemetryConfigMap.Data)
	}
	if value != "console.redhat.com/connections/api/v1" {
		t.Fatalf("telemetry-config configmap does not contain SEGMENT_API_HOST key with value 'console.redhat.com/connections/api/v1'. Instead contains: %q", value)
	}

	// update the defaul value for SEGMENT_API_HOST key
	_, err = client.Core.ConfigMaps(api.OpenShiftConsoleOperatorNamespace).Patch(context.TODO(), telemetry.TelemetryConfigMapName, types.MergePatchType, []byte(`{"data": {"SEGMENT_API_HOST": "test"}}`), metav1.PatchOptions{})
	if err != nil {
		t.Fatal(err)
	}

	err = wait.Poll(1*time.Second, framework.AsyncOperationTimeout, func() (stop bool, err error) {
		telemetryConfigMap, err := client.Core.ConfigMaps(api.OpenShiftConsoleOperatorNamespace).Get(context.TODO(), telemetry.TelemetryConfigMapName, v1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		value, ok := telemetryConfigMap.Data[SEGMENT_API_HOST]
		if !ok {
			return false, fmt.Errorf("updated telemetry-config configmap does not contain SEGMENT_JS_HOST data key. Instead contains: %v", telemetryConfigMap.Data)
		}
		if value != "test" {
			return false, fmt.Errorf("update telemetry-config configmap does not contain SEGMENT_API_HOST key with value 'console.redhat.com/connections/api/v1'. Instead contains: %q", value)
		}

		return true, nil
	})

}
