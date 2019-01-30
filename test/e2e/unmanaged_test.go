package e2e

import (
	"testing"

	"github.com/openshift/console-operator/pkg/testframework"
)

// TestUnmanaged sets console-operator to Unmanaged state. After that "openshift-console
// deployment is deleted after which the deploymnet is tested for unavailability, to
// check that it wasn't recreated byt the console operator. Other resources from the
// 'openshift-console' namespace (ConfigMap, Router, Service) are tested for availability
// since they have not been deleted.
func TestUnmanaged(t *testing.T) {
	client := testframework.MustNewClientset(t, nil)
	defer testframework.MustManageConsole(t, client)
	testframework.MustUnmanageConsole(t, client)
	testframework.DeleteAll(t, client)

	t.Logf("waiting to check if the operator has not recreate deleted resources...")
	errChan := make(chan error)
	go testframework.IsResourceUnavailable(errChan, client, "ConfigMap")
	go testframework.IsResourceUnavailable(errChan, client, "Route")
	go testframework.IsResourceUnavailable(errChan, client, "Service")
	go testframework.IsResourceUnavailable(errChan, client, "Deployment")
	checkErr := <-errChan

	if checkErr != nil {
		t.Fatal(checkErr)
	}
}

func TestEditUnmanagedConfigMap(t *testing.T) {
	client := testframework.MustNewClientset(t, nil)
	defer testframework.MustManageConsole(t, client)
	testframework.MustUnmanageConsole(t, client)

	err := patchAndCheckConfigMap(t, client, false)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}

func TestEditUnmanagedService(t *testing.T) {
	client := testframework.MustNewClientset(t, nil)
	defer testframework.MustManageConsole(t, client)
	testframework.MustUnmanageConsole(t, client)

	err := patchAndCheckService(t, client, false)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}

func TestEditUnmanagedRoute(t *testing.T) {
	client := testframework.MustNewClientset(t, nil)
	defer testframework.MustManageConsole(t, client)
	testframework.MustUnmanageConsole(t, client)

	err := patchAndCheckRoute(t, client, false)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}
