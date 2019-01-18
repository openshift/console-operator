package e2e_test

import (
	"testing"

	"github.com/openshift/console-operator/pkg/testframework"
)

// TestManaged sets console-operator to Managed state. After that "openshift-console
// deployment is deleted after which the deployment is tested for availability, together
// with other resources from the 'openshift-console' namespace - ConfigMap, Router, Service
func TestManaged(t *testing.T) {
	client := testframework.MustNewClientset(t, nil)
	defer testframework.MustManageConsole(t, client)
	testframework.MustManageConsole(t, client)
	testframework.DeleteAll(t, client)

	t.Logf("waiting the operator to recreate console deployment...")

	errChan := make(chan error)
	go testframework.IsResourceAvailable(errChan, client, "ConfigMap")
	go testframework.IsResourceAvailable(errChan, client, "Route")
	go testframework.IsResourceAvailable(errChan, client, "Service")
	go testframework.IsResourceAvailable(errChan, client, "Deployment")
	checkErr := <-errChan

	if checkErr != nil {
		t.Fatal(checkErr)
	}
}
