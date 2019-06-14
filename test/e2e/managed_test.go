package e2e

import (
	"testing"

	"github.com/openshift/console-operator/pkg/testframework"
)

func setupManagedTestCase(t *testing.T) *testframework.Clientset {
	client := testframework.MustNewClientset(t, nil)
	testframework.MustManageConsole(t, client)
	return client
}

func cleanupManagedTestCase(t *testing.T, client *testframework.Clientset) {
	testframework.WaitForSettledState(t, client)
}

// TestManaged() sets ManagementState:Managed then deletes a set of console
// resources and verifies that the operator recreates them.
func TestManaged(t *testing.T) {
	client := setupManagedTestCase(t)
	defer testframework.MustManageConsole(t, client)
	testframework.DeleteAll(t, client)

	t.Logf("validating that the operator recreates resources when ManagementState:Managed...")

	err := testframework.ConsoleResourcesAvailable(client)
	if err != nil {
		t.Fatal(err)
	}
	cleanupManagedTestCase(t, client)
}

func TestEditManagedConfigMap(t *testing.T) {
	client := setupManagedTestCase(t)
	defer testframework.MustManageConsole(t, client)

	err := patchAndCheckConfigMap(t, client, true)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	cleanupManagedTestCase(t, client)
}

func TestEditManagedService(t *testing.T) {
	client := setupManagedTestCase(t)
	defer testframework.MustManageConsole(t, client)

	err := patchAndCheckService(t, client, true)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	cleanupManagedTestCase(t, client)
}

func TestEditManagedRoute(t *testing.T) {
	client := setupManagedTestCase(t)
	defer testframework.MustManageConsole(t, client)

	err := patchAndCheckRoute(t, client, true)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	cleanupManagedTestCase(t, client)
}
