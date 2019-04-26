package testframework

import (
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	operatorsv1 "github.com/openshift/api/operator/v1"
	consoleapi "github.com/openshift/console-operator/pkg/api"
)

func isOperatorManaged(cr *operatorsv1.Console) bool {
	return cr.Spec.ManagementState == operatorsv1.Managed
}

func isOperatorUnmanaged(cr *operatorsv1.Console) bool {
	return cr.Spec.ManagementState == operatorsv1.Unmanaged
}

func isOperatorRemoved(cr *operatorsv1.Console) bool {
	return cr.Spec.ManagementState == operatorsv1.Removed
}

type operatorStateReactionFn func(cr *operatorsv1.Console) bool

func ensureConsoleIsInDesiredState(t *testing.T, client *Clientset, state operatorsv1.ManagementState) error {
	var operatorConfig *operatorsv1.Console
	// var checkFunc func()
	var checkFunc operatorStateReactionFn

	switch state {
	case operatorsv1.Managed:
		checkFunc = isOperatorManaged
	case operatorsv1.Unmanaged:
		checkFunc = isOperatorUnmanaged
	case operatorsv1.Removed:
		checkFunc = isOperatorRemoved
	}

	err := wait.Poll(1*time.Second, AsyncOperationTimeout, func() (stop bool, err error) {
		operatorConfig, err = client.Consoles().Get(consoleapi.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return checkFunc(operatorConfig), nil
	})
	if err != nil {
		DumpObject(t, "the latest observed state of the console resource", operatorConfig)
		DumpOperatorLogs(t, client)
		return fmt.Errorf("failed to wait to change console operator state to 'Removed': %s", err)
	}
	return nil
}

func ManageConsole(t *testing.T, client *Clientset) error {
	operatorConfig, err := client.Consoles().Get(consoleapi.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if isOperatorManaged(operatorConfig) {
		t.Logf("console operator already in 'Managed' state")
		return nil
	}

	t.Logf("changing console operator state to 'Managed'...")

	_, err = client.Consoles().Patch(consoleapi.ConfigResourceName, types.MergePatchType, []byte(`{"spec": {"managementState": "Managed"}}`))
	if err != nil {
		return err
	}
	if err := ensureConsoleIsInDesiredState(t, client, operatorsv1.Managed); err != nil {
		return fmt.Errorf("unable to change console operator state to 'Managed': %s", err)
	}

	err = ConsoleResourcesAvailable(client)
	if err != nil {
		t.Fatal(err)
	}

	return nil
}

func UnmanageConsole(t *testing.T, client *Clientset) error {
	operatorConfig, err := client.Consoles().Get(consoleapi.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if isOperatorUnmanaged(operatorConfig) {
		t.Logf("console operator already in 'Unmanaged' state")
		return nil
	}

	t.Logf("changing console operator state to 'Unmanaged'...")

	_, err = client.Consoles().Patch(consoleapi.ConfigResourceName, types.MergePatchType, []byte(`{"spec": {"managementState": "Unmanaged"}}`))
	if err != nil {
		return err
	}
	if err := ensureConsoleIsInDesiredState(t, client, operatorsv1.Unmanaged); err != nil {
		return fmt.Errorf("unable to change console operator state to 'Unmanaged': %s", err)
	}

	return nil
}

func RemoveConsole(t *testing.T, client *Clientset) error {
	operatorConfig, err := client.Consoles().Get(consoleapi.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if isOperatorRemoved(operatorConfig) {
		t.Logf("console operator already in 'Removed' state")
		return nil
	}

	t.Logf("changing console operator state to 'Removed'...")

	_, err = client.Consoles().Patch(consoleapi.ConfigResourceName, types.MergePatchType, []byte(`{"spec": {"managementState": "Removed"}}`))
	if err != nil {
		return err
	}
	if err := ensureConsoleIsInDesiredState(t, client, operatorsv1.Removed); err != nil {
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

func MustNormalLogLevel(t *testing.T, client *Clientset) error {
	operatorConfig, err := client.Consoles().Get(consoleapi.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("checking if console operator LogLevel is set to 'Normal'...")
	if operatorConfig.Spec.LogLevel == operatorsv1.Normal {
		return nil
	}
	err = SetLogLevel(t, client, operatorsv1.Normal)
	if err != nil {
		t.Fatal(err)
	}
	return nil
}

func SetLogLevel(t *testing.T, client *Clientset, logLevel operatorsv1.LogLevel) error {
	operatorConfig, err := client.Consoles().Get(consoleapi.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	deployment, err := GetConsoleDeployment(client)
	if err != nil {
		return err
	}
	currentDeploymentGeneration := deployment.ObjectMeta.Generation
	currentOperatorConfigGeneration := operatorConfig.ObjectMeta.Generation

	t.Logf("setting console operator to '%s' LogLevel ...", logLevel)
	_, err = client.Consoles().Patch(consoleapi.ConfigResourceName, types.MergePatchType, []byte(fmt.Sprintf(`{"spec": {"logLevel": "%s"}}`, logLevel)))
	if err != nil {
		return err
	}

	err = wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
		newOperatorConfig, err := client.Consoles().Get(consoleapi.ConfigResourceName, metav1.GetOptions{})
		newDeployment, err := GetConsoleDeployment(client)
		if err != nil {
			return false, nil
		}
		if GenerationChanged(newOperatorConfig.ObjectMeta.Generation, currentOperatorConfigGeneration) {
			return false, nil
		}
		if GenerationChanged(newDeployment.ObjectMeta.Generation, currentDeploymentGeneration) {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	return nil
}

func GenerationChanged(oldGeneration, newGeneration int64) bool {
	return oldGeneration == newGeneration
}
