package e2e

import (
	"testing"

	"github.com/openshift/console-operator/pkg/testframework"
)

// TestRemoved sets console-operator to Removed state. After that all the resources
// from the 'openshift-console' namespace (Deployment, ConfigMap, Router, Service),
// are tested for unavailability since the operator should delete them.
func TestRemoved(t *testing.T) {
	client := testframework.MustNewClientset(t, nil)
	defer testframework.MustManageConsole(t, client)
	testframework.MustRemoveConsole(t, client)

	t.Logf("waiting to check if the operator has not recreate removed console resources...")

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
