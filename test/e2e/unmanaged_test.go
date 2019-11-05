package e2e

import (
	"testing"

	operatorsv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/test/e2e/framework"
)

func setupUnmanagedTestCase(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	client, operatorConfig := framework.StandardSetup(t)
	framework.MustUnmanageConsole(t, client)
	return client, operatorConfig
}

func cleanUpUnmanagedTestCase(t *testing.T, client *framework.ClientSet) {
	framework.StandardCleanup(t, client)
}

// TestUnmanaged() sets ManagementState:Unmanaged then deletes a set of console
// resources and verifies that the operator does not recreate them.
func TestUnmanaged(t *testing.T) {
	client, _ := setupUnmanagedTestCase(t)
	defer cleanUpUnmanagedTestCase(t, client)

	framework.DeleteAll(t, client)
	t.Logf("validating that the operator does not recreate deleted resources when ManagementState:Unmanaged...")
	err := framework.ConsoleResourcesUnavailable(client)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEditUnmanagedConfigMap(t *testing.T) {
	client, _ := setupUnmanagedTestCase(t)
	defer cleanUpUnmanagedTestCase(t, client)

	err := patchAndCheckConfigMap(t, client, false)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}

func TestEditUnmanagedService(t *testing.T) {
	client, _ := setupUnmanagedTestCase(t)
	defer cleanUpUnmanagedTestCase(t, client)

	err := patchAndCheckService(t, client, false)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}

func TestEditUnmanagedRoute(t *testing.T) {
	client, _ := setupUnmanagedTestCase(t)
	defer cleanUpUnmanagedTestCase(t, client)

	err := patchAndCheckRoute(t, client, false)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}

func TestEditUnmanagedConsoleCLIDownloads(t *testing.T) {
	client, _ := setupUnmanagedTestCase(t)
	defer cleanUpUnmanagedTestCase(t, client)

	err := patchAndCheckConsoleCLIDownloads(t, client, false, api.OCCLIDownloadsCustomResourceName)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	err = patchAndCheckConsoleCLIDownloads(t, client, false, api.ODOCLIDownloadsCustomResourceName)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}
