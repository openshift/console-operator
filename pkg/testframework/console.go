package testframework

import (
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	// operatorapi "github.com/openshift/api/operator/v1"
	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"

	consoleapi "github.com/openshift/console-operator/pkg/api"
	v1alpha1 "github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
)

func isOperatorManaged(cr *v1alpha1.Console) bool {
	return cr.Spec.ManagementState == operatorsv1alpha1.Managed
}

func isOperatorUnmanaged(cr *v1alpha1.Console) bool {
	return cr.Spec.ManagementState == operatorsv1alpha1.Unmanaged
}

func isOperatorRemoved(cr *v1alpha1.Console) bool {
	return cr.Spec.ManagementState == operatorsv1alpha1.Removed
}

type operatorStateReactionFn func(cr *v1alpha1.Console) bool

func ensureConsoleIsInDesiredState(t *testing.T, client *Clientset, state operatorsv1alpha1.ManagementState) error {
	var cr *v1alpha1.Console
	// var checkFunc func()
	var checkFunc operatorStateReactionFn

	switch state {
	case operatorsv1alpha1.Managed:
		checkFunc = isOperatorManaged
	case operatorsv1alpha1.Unmanaged:
		checkFunc = isOperatorUnmanaged
	case operatorsv1alpha1.Removed:
		checkFunc = isOperatorRemoved
	}

	err := wait.Poll(1*time.Second, AsyncOperationTimeout, func() (stop bool, err error) {
		cr, err = client.ConsoleV1alpha1Interface.Consoles(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.ResourceName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return checkFunc(cr), nil
	})
	if err != nil {
		DumpObject(t, "the latest observed state of the console resource", cr)
		DumpOperatorLogs(t, client)
		return fmt.Errorf("failed to wait to change console operator state to 'Removed': %s", err)
	}
	return nil
}

func ManageConsole(t *testing.T, client *Clientset) error {
	cr, err := client.ConsoleV1alpha1Interface.Consoles(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.ResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if isOperatorManaged(cr) {
		t.Logf("console operator already in 'Managed' state")
		return nil
	}

	t.Logf("changing console operator state to 'Managed'...")

	_, err = client.ConsoleV1alpha1Interface.Consoles(consoleapi.OpenShiftConsoleNamespace).Patch(consoleapi.ResourceName, types.MergePatchType, []byte(`{"spec": {"managementState": "Managed"}}`))
	if err != nil {
		return err
	}
	if err := ensureConsoleIsInDesiredState(t, client, operatorsv1alpha1.Managed); err != nil {
		return fmt.Errorf("unable to change console operator state to 'Managed': %s", err)
	}

	return nil
}

func UnmanageConsole(t *testing.T, client *Clientset) error {
	cr, err := client.ConsoleV1alpha1Interface.Consoles(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.ResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if isOperatorUnmanaged(cr) {
		t.Logf("console operator already in 'Unmanaged' state")
		return nil
	}

	t.Logf("changing console operator state to 'Unmanaged'...")

	_, err = client.ConsoleV1alpha1Interface.Consoles(consoleapi.OpenShiftConsoleNamespace).Patch(consoleapi.ResourceName, types.MergePatchType, []byte(`{"spec": {"managementState": "Unmanaged"}}`))
	if err != nil {
		return err
	}
	if err := ensureConsoleIsInDesiredState(t, client, operatorsv1alpha1.Unmanaged); err != nil {
		return fmt.Errorf("unable to change console operator state to 'Unmanaged': %s", err)
	}

	return nil
}

func RemoveConsole(t *testing.T, client *Clientset) error {
	cr, err := client.ConsoleV1alpha1Interface.Consoles(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.ResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if isOperatorRemoved(cr) {
		t.Logf("console operator already in 'Removed' state")
		return nil
	}

	t.Logf("changing console operator state to 'Removed'...")

	_, err = client.ConsoleV1alpha1Interface.Consoles(consoleapi.OpenShiftConsoleNamespace).Patch(consoleapi.ResourceName, types.MergePatchType, []byte(`{"spec": {"managementState": "Removed"}}`))
	if err != nil {
		return err
	}
	if err := ensureConsoleIsInDesiredState(t, client, operatorsv1alpha1.Removed); err != nil {
		return fmt.Errorf("unable to change console operator state to 'Removed': %s", err)
	}

	return nil
}
func MustManageConsole(t *testing.T, client *Clientset) error {
	if err := ManageConsole(t, client); err != nil {
		t.Fatal(err)
	}
	return nil
}

func MustUnmanageConsole(t *testing.T, client *Clientset) error {
	if err := UnmanageConsole(t, client); err != nil {
		t.Fatal(err)
	}
	return nil
}

func MustRemoveConsole(t *testing.T, client *Clientset) error {
	if err := RemoveConsole(t, client); err != nil {
		t.Fatal(err)
	}
	return nil
}
