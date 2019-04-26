package e2e

import (
	"testing"

	"github.com/openshift/console-operator/pkg/testframework"
)

// TestUnmanaged() sets ManagementState:Unmanaged then deletes a set of console
// resources and verifies that the operator does not recreate them.
func TestUnmanaged(t *testing.T) {
	client := testframework.MustNewClientset(t, nil)
	defer testframework.MustManageConsole(t, client)
	testframework.MustUnmanageConsole(t, client)
	testframework.DeleteAll(t, client)

	t.Logf("validating that the operator does not recreate deleted resources when ManagementState:Unmanaged...")
	err := testframework.ConsoleResourcesUnavailable(client)
	if err != nil {
		t.Fatal(err)
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
