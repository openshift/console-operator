package e2e_test

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	consoleapi "github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/testframework"
)

func TestManaged(t *testing.T) {
	client := testframework.MustNewClientset(t, nil)

	t.Logf("deleting console deployment...")
	if err := testframework.DeleteCompletely(
		func() (metav1.Object, error) {
			return client.Deployments(consoleapi.OpenShiftConsoleOperatorNamespace).Get(consoleapi.OpenShiftConsoleName, metav1.GetOptions{})
		},
		func(deleteOptions *metav1.DeleteOptions) error {
			return client.Deployments(consoleapi.OpenShiftConsoleOperatorNamespace).Delete(consoleapi.OpenShiftConsoleName, deleteOptions)
		},
	); err != nil {
		t.Fatalf("unable to delete console deployment: %s", err)
	}

	t.Logf("waiting the operator to recreate console deployment...")
	err := wait.Poll(1*time.Second, testframework.AsyncOperationTimeout, func() (stop bool, err error) {
		_, err = client.Deployments(consoleapi.OpenShiftConsoleOperatorNamespace).Get(consoleapi.OpenShiftConsoleName, metav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		t.Logf("get deployment: %s", err)
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal(err)
	}
}
