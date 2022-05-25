package e2e

import (
	"testing"

	operatorsv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/test/e2e/framework"
)

func setupManagedTestCase(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	return framework.StandardSetup(t)
}

func cleanupManagedTestCase(t *testing.T, client *framework.ClientSet) {
	framework.StandardCleanup(t, client)
}

// TestManaged() sets ManagementState:Managed then deletes a set of console
// resources and verifies that the operator recreates them.
func TestManaged(t *testing.T) {
	client, _ := setupManagedTestCase(t)
	defer cleanupManagedTestCase(t, client)
	framework.DeleteAll(t, client)

	t.Logf("validating that the operator recreates resources when ManagementState:Managed...")
	err := framework.ConsoleResourcesAvailable(client)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEditManagedConfigMap(t *testing.T) {
	client, _ := setupManagedTestCase(t)
	defer cleanupManagedTestCase(t, client)

	err := patchAndCheckConfigMap(t, client, true)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}

func TestEditManagedService(t *testing.T) {
	client, _ := setupManagedTestCase(t)
	defer cleanupManagedTestCase(t, client)

	err := patchAndCheckService(t, client, true)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}

func TestEditManagedRoute(t *testing.T) {
	client, _ := setupManagedTestCase(t)
	defer cleanupManagedTestCase(t, client)

	err := patchAndCheckRoute(t, client, true)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}

func TestEditManagedConsoleCLIDownloads(t *testing.T) {
	client, _ := setupManagedTestCase(t)
	defer cleanupManagedTestCase(t, client)

	err := patchAndCheckConsoleCLIDownloads(t, client, true, api.OCCLIDownloadsCustomResourceName)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	err = patchAndCheckConsoleCLIDownloads(t, client, true, api.ODOCLIDownloadsCustomResourceName)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}

func TestEditManagedPodDisruptionBudget(t *testing.T) {
	client, _ := setupManagedTestCase(t)
	defer cleanupManagedTestCase(t, client)

	err := patchAndCheckPodDisruptionBudget(t, client, true, api.OpenShiftConsolePDBName)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	err = patchAndCheckPodDisruptionBudget(t, client, true, api.OpenShiftConsoleDownloadsPDBName)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
}
