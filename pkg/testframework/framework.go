package testframework

import (
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	consoleapi "github.com/openshift/console-operator/pkg/api"
)

var (
	// AsyncOperationTimeout is how long we want to wait for asynchronous
	// operations to complete. ForeverTestTimeout is not long enough to create
	// several replicas and get them available on a slow machine.
	// Setting this to 5 minutes:w

	AsyncOperationTimeout = 5 * time.Minute
)

func DeleteAll(t *testing.T, client *Clientset) {
	resources := []string{"Deployment", "Service", "Route", "ConfigMap"}

	for _, resource := range resources {
		t.Logf("deleting console %s...", resource)
		if err := DeleteCompletely(
			func() (metav1.Object, error) {
				return getResource(client, resource)
			},
			func(*metav1.DeleteOptions) error {
				return deleteResource(client, resource)
			},
		); err != nil {
			t.Fatalf("unable to delete console %s: %s", resource, err)
		}
	}
}

func getResource(client *Clientset, resource string) (metav1.Object, error) {
	var res metav1.Object
	var err error
	switch resource {
	case "ConfigMap":
		res, err = client.ConfigMaps(consoleapi.OpenShiftConsoleOperatorNamespace).Get(consoleapi.OpenShiftConsoleConfigMapName, metav1.GetOptions{})
	case "Service":
		res, err = client.Services(consoleapi.OpenShiftConsoleOperatorNamespace).Get(consoleapi.OpenShiftConsoleServiceName, metav1.GetOptions{})
	case "Route":
		res, err = client.Routes(consoleapi.OpenShiftConsoleOperatorNamespace).Get(consoleapi.OpenShiftConsoleRouteName, metav1.GetOptions{})
	case "Deployment":
		fallthrough
	default:
		res, err = client.Deployments(consoleapi.OpenShiftConsoleOperatorNamespace).Get(consoleapi.OpenShiftConsoleDeploymentName, metav1.GetOptions{})
	}
	return res, err
}

func deleteResource(client *Clientset, resource string) error {
	var err error
	switch resource {
	case "ConfigMap":
		err = client.ConfigMaps(consoleapi.OpenShiftConsoleOperatorNamespace).Delete(consoleapi.OpenShiftConsoleConfigMapName, &metav1.DeleteOptions{})
	case "Service":
		err = client.Services(consoleapi.OpenShiftConsoleOperatorNamespace).Delete(consoleapi.OpenShiftConsoleServiceName, &metav1.DeleteOptions{})
	case "Route":
		err = client.Routes(consoleapi.OpenShiftConsoleOperatorNamespace).Delete(consoleapi.OpenShiftConsoleRouteName, &metav1.DeleteOptions{})
	case "Deployment":
		fallthrough
	default:
		err = client.Deployments(consoleapi.OpenShiftConsoleOperatorNamespace).Delete(consoleapi.OpenShiftConsoleDeploymentName, &metav1.DeleteOptions{})
	}
	return err
}

// DeleteCompletely sends a delete request and waits until the resource and
// its dependents are deleted.
func DeleteCompletely(getObject func() (metav1.Object, error), deleteObject func(*metav1.DeleteOptions) error) error {
	obj, err := getObject()
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	uid := obj.GetUID()

	policy := metav1.DeletePropagationForeground
	if err := deleteObject(&metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{
			UID: &uid,
		},
		PropagationPolicy: &policy,
	}); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return wait.Poll(1*time.Second, AsyncOperationTimeout, func() (stop bool, err error) {
		obj, err = getObject()
		if err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}

		return obj.GetUID() != uid, nil
	})
}

// IsResourceAvailable checks if tested resource is available(recreated by console-operator),
// during 10 second period. If not error will be returned.
func IsResourceAvailable(errChan chan error, client *Clientset, resource string) {
	counter := 0
	err := wait.Poll(1*time.Second, AsyncOperationTimeout, func() (stop bool, err error) {
		_, err = getResource(client, resource)
		if err == nil {
			return true, nil
		}
		if counter == 10 {
			if err != nil {
				return true, fmt.Errorf("deleted console %s was not recreated", resource)
			}
			return true, nil
		}
		counter++
		return false, nil
	})
	errChan <- err
}

// IsResourceUnavailable checks if tested resource is unavailable(not recreated by console-operator),
// during 10 second period. If not error will be returned.
func IsResourceUnavailable(errChan chan error, client *Clientset, resource string) {
	counter := 0
	err := wait.Poll(1*time.Second, AsyncOperationTimeout, func() (stop bool, err error) {
		_, err = getResource(client, resource)
		if err == nil {
			return true, fmt.Errorf("deleted console %s was recreated", resource)
		}
		if !errors.IsNotFound(err) {
			return true, err
		}
		counter++
		if counter == 10 {
			return true, nil
		}
		return false, nil
	})
	errChan <- err
}
