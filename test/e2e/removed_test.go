package e2e

import (
	"testing"

	"github.com/openshift/console-operator/pkg/testframework"
)

// TestRemoved() sets ManagementState:Removed and verifies that all
// console resources are deleted.
func TestRemoved(t *testing.T) {
	client := testframework.MustNewClientset(t, nil)
	defer testframework.MustManageConsole(t, client)
	testframework.MustRemoveConsole(t, client)

	t.Logf("validating that the operator does not recreate removed resources when ManagementState:Removed...")

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
