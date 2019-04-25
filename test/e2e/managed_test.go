package e2e

import (
	"testing"

	"github.com/openshift/console-operator/pkg/testframework"
)

// TestManaged() sets ManagementState:Managed then deletes a set of console
// resources and verifies that the operator recreates them.
func TestManaged(t *testing.T) {
	client := testframework.MustNewClientset(t, nil)
	defer testframework.MustManageConsole(t, client)
	testframework.MustManageConsole(t, client)
	testframework.DeleteAll(t, client)

	t.Logf("validating that the operator recreates resources when ManagementState:Managed...")

	err := testframework.ConsoleResourcesAvailable(client)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEditManagedConfigMap(t *testing.T) {
	client := testframework.MustNewClientset(t, nil)
	defer testframework.MustManageConsole(t, client)
	testframework.MustManageConsole(t, client)

	err := patchAndCheckConfigMap(t, client, true)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}

func TestEditManagedService(t *testing.T) {
	client := testframework.MustNewClientset(t, nil)
	defer testframework.MustManageConsole(t, client)
	testframework.MustManageConsole(t, client)

	err := patchAndCheckService(t, client, true)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}

func TestEditManagedRoute(t *testing.T) {
	client := testframework.MustNewClientset(t, nil)
	defer testframework.MustManageConsole(t, client)
	testframework.MustManageConsole(t, client)

	err := patchAndCheckRoute(t, client, true)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}
