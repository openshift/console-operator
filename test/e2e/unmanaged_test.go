package e2e

import (
	"reflect"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/console-operator/pkg/api"

	"github.com/openshift/console-operator/pkg/testframework"
)

// TestUnmanaged sets console-operator to Unmanaged state. After that "openshift-console
// deployment is deleted after which the deploymnet is tested for unavailability, to
// check that it wasn't recreated byt the console operator. Other resources from the
// 'openshift-console' namespace (ConfigMap, Router, Service) are tested for availability
// since they have not been deleted.
//func TestUnmanaged(t *testing.T) {
//	client := testframework.MustNewClientset(t, nil)
//	defer testframework.MustManageConsole(t, client)
//	testframework.MustUnmanageConsole(t, client)
//	testframework.DeleteAll(t, client)
//
//	t.Logf("waiting to check if the operator has not recreate deleted resources...")
//	errChan := make(chan error)
//	go testframework.IsResourceUnavailable(errChan, client, "ConfigMap")
//	go testframework.IsResourceUnavailable(errChan, client, "Route")
//	go testframework.IsResourceUnavailable(errChan, client, "Service")
//	go testframework.IsResourceUnavailable(errChan, client, "Deployment")
//	checkErr := <-errChan
//
//	if checkErr != nil {
//		t.Fatal(checkErr)
//	}
//}
//
//func TestEditUnmanagedConfigMap(t *testing.T) {
//	client := testframework.MustNewClientset(t, nil)
//	defer testframework.MustManageConsole(t, client)
//	testframework.MustUnmanageConsole(t, client)
//
//	err := patchAndCheckConfigMap(t, client, false)
//	if err != nil {
//		t.Fatalf("error: %s", err)
//	}
//}
//
//func TestEditUnmanagedService(t *testing.T) {
//	client := testframework.MustNewClientset(t, nil)
//	defer testframework.MustManageConsole(t, client)
//	testframework.MustUnmanageConsole(t, client)
//
//	err := patchAndCheckService(t, client, false)
//	if err != nil {
//		t.Fatalf("error: %s", err)
//	}
//}
//
//func TestEditUnmanagedRoute(t *testing.T) {
//	client := testframework.MustNewClientset(t, nil)
//	defer testframework.MustManageConsole(t, client)
//	testframework.MustUnmanageConsole(t, client)
//
//	err := patchAndCheckRoute(t, client, false)
//	if err != nil {
//		t.Fatalf("error: %s", err)
//	}
//}

// TEMP:  fiddling with the tests to see why things are failing in CI
func TestEditUnmanagedConfigMapAgain(t *testing.T) {
	frequency := 1 * time.Second
	timeout := 10 * time.Second
	client := testframework.MustNewClientset(t, nil)
	// in the end, put the console back to "Manged"
	defer testframework.MustManageConsole(t, client)
	// kick us off with "Unmanaged"
	testframework.MustUnmanageConsole(t, client)

	originalCM, err := client.ConfigMaps(api.TargetNamespace).Get(api.OpenShiftConsoleConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	logrus.Printf("retrieved: %v \n", originalCM.SelfLink)

	// copy for good measure
	modifiedCM := originalCM.DeepCopy()
	// modify it to check if it gets put back
	modifiedCM.Data = map[string]string{}
	modifiedCM.Data["this-is-not-the-right-yaml"] = string([]byte(`{"data": {"console-config.yaml": "test"}}`))

	// issue the update
	if _, err := client.ConfigMaps(api.TargetNamespace).Update(modifiedCM); err != nil {
		t.Fatalf("error: %s", err)
	}

	// using this bool to test if the operator stomps the cm back into place
	operatorHasReconciled := false
	count := 0
	err = wait.Poll(frequency, timeout, func() (stop bool, err error) {
		count++
		logrus.Printf("Polling: %v \n", count)

		testCM, err := client.ConfigMaps(api.TargetNamespace).Get(api.OpenShiftConsoleConfigMapName, metav1.GetOptions{})
		if err != nil {
			// if we err, stop the loop, something went wrong
			return true, err
		}
		logrus.Printf("cm Data: %v matches? %v \n", testCM.Data, !reflect.DeepEqual(originalCM.Data, testCM.Data))
		// this should stay false. if it flips to true we are broken.
		operatorHasReconciled = !reflect.DeepEqual(originalCM.Data, testCM.Data)
		// let it run the full 10 cycles
		return false, err
	})
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	if operatorHasReconciled {
		t.Fatalf("error: operator reconciled the configmap while in an unmanaged state.")
	}
}
