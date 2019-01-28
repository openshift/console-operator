package e2e

import (
	"reflect"
	"testing"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/testframework"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	consoleapi "github.com/openshift/console-operator/pkg/api"
)

// Basically each of these tests helpers are similar, they only vary in the
// resource they are GETting, PATCHing and patch itself.
// Since after the patch is done, the test needs to wait, till it can GET
// patched object(or not, if the operator status is set to Managed).

func patchAndCheckConfigMap(t *testing.T, client *testframework.Clientset) bool {
	res, err := testframework.GetResource(client, "ConfigMap")
	errorCheck(t, err)
	configMap, ok := res.(*corev1.ConfigMap)
	if !ok {
		t.Fatalf("unable to type received object as ConfigMap")
	}
	originalData := configMap.Data

	t.Logf("patching Data on the console ConfigMap")
	_, err = client.ConfigMaps(consoleapi.OpenShiftConsoleOperatorNamespace).Patch(consoleapi.OpenShiftConsoleConfigMapName, types.MergePatchType, []byte(`{"data": {"console-config.yaml": "test"}}`))
	errorCheck(t, err)

	time.Sleep(5 * time.Second)

	res, err = testframework.GetResource(client, "ConfigMap")
	errorCheck(t, err)
	configMap, ok = res.(*corev1.ConfigMap)
	if !ok {
		t.Fatalf("unable to type received object as ConfigMap")
	}
	newData := configMap.Data

	return reflect.DeepEqual(originalData, newData)
}

func patchAndCheckService(t *testing.T, client *testframework.Clientset) bool {
	res, err := testframework.GetResource(client, "Service")
	errorCheck(t, err)
	service, ok := res.(*corev1.Service)
	if !ok {
		t.Fatalf("unable to type received object as Service")
	}
	originalData := service.GetAnnotations()

	t.Logf("patching Annotations on the console Service")
	_, err = client.Services(consoleapi.OpenShiftConsoleOperatorNamespace).Patch(consoleapi.OpenShiftConsoleServiceName, types.MergePatchType, []byte(`{"metadata": {"annotations": {"service.alpha.openshift.io/serving-cert-secret-name": "test"}}}`))
	errorCheck(t, err)

	time.Sleep(5 * time.Second)

	res, err = testframework.GetResource(client, "Service")
	errorCheck(t, err)
	service, ok = res.(*corev1.Service)
	if !ok {
		t.Fatalf("unable to type received object as Service")
	}
	newData := service.GetAnnotations()

	return reflect.DeepEqual(originalData, newData)
}

func patchAndCheckRoute(t *testing.T, client *testframework.Clientset) bool {
	res, err := testframework.GetResource(client, "Route")
	errorCheck(t, err)
	route, ok := res.(*routev1.Route)
	if !ok {
		t.Fatalf("unable to type received object as Route")
	}
	originalData := route.Spec.Port.TargetPort

	t.Logf("patching TargetPort on the console Route")
	_, err = client.Routes(consoleapi.OpenShiftConsoleOperatorNamespace).Patch(consoleapi.OpenShiftConsoleRouteName, types.MergePatchType, []byte(`{"spec": {"port": {"targetPort": "http"}}}`))
	errorCheck(t, err)

	time.Sleep(5 * time.Second)

	res, err = testframework.GetResource(client, "Route")
	errorCheck(t, err)
	route, ok = res.(*routev1.Route)
	if !ok {
		t.Fatalf("unable to type received object as Route")
	}
	newData := route.Spec.Port.TargetPort

	return reflect.DeepEqual(originalData, newData)
}

func errorCheck(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("Fatal error: %s", err)
	}
}
