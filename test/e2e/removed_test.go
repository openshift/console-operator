package e2e

import (
	"testing"

	operatorsv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/test/e2e/framework"
)

func setupRemovedTestCase(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	return framework.StandardSetup(t)
}

func cleanupRemovedTestCase(t *testing.T, client *framework.ClientSet) {
	framework.StandardCleanup(t, client)
}

// TestRemoved() sets ManagementState:Removed and verifies that all
// console resources are deleted.
// NOTE: this does not apply to the Downloads resources.  The Downloads
// resources are managed by CVO and thus always exist.
func TestRemoved(t *testing.T) {
	client, _ := setupRemovedTestCase(t)
	defer cleanupRemovedTestCase(t, client)

	framework.MustRemoveConsole(t, client)
	t.Logf("validating that the operator does not recreate removed resources when ManagementState:Removed...")
	err := framework.ConsoleResourcesUnavailable(client)
	if err != nil {
		t.Fatal(err)
	}
	framework.MustManageConsole(t, client)
	t.Logf("validating that the operator does recreate removed resources when ManagementState:Managed...")
	framework.ConsoleResourcesAvailable(client)
	if err != nil {
		t.Fatal(err)
	}
}
