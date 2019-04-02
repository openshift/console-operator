package testframework

import (
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	routev1 "github.com/openshift/api/route/v1"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
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
			func() (runtime.Object, error) {
				return GetResource(client, resource)
			},
			func(*metav1.DeleteOptions) error {
				return deleteResource(client, resource)
			},
		); err != nil {
			t.Fatalf("unable to delete console %s: %s", resource, err)
		}
	}
}

func GetResource(client *Clientset, resource string) (runtime.Object, error) {
	var res runtime.Object
	var err error
	switch resource {
	case "ConfigMap":
		res, err = GetConsoleConfigMap(client)
	case "Service":
		res, err = GetConsoleService(client)
	case "Route":
		res, err = GetConsoleRoute(client)
	case "Deployment":
		fallthrough
	default:
		res, err = GetConsoleDeployment(client)
	}
	return res, err
}

func GetConsoleConfigMap(client *Clientset) (*corev1.ConfigMap, error) {
	return client.ConfigMaps(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.OpenShiftConsoleConfigMapName, metav1.GetOptions{})
}

func GetConsoleService(client *Clientset) (*corev1.Service, error) {
	return client.Services(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.OpenShiftConsoleServiceName, metav1.GetOptions{})
}

func GetConsoleRoute(client *Clientset) (*routev1.Route, error) {
	return client.Routes(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.OpenShiftConsoleRouteName, metav1.GetOptions{})
}

func GetConsoleDeployment(client *Clientset) (*appv1.Deployment, error) {
	return client.Deployments(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.OpenShiftConsoleDeploymentName, metav1.GetOptions{})
}

func deleteResource(client *Clientset, resource string) error {
	var err error
	switch resource {
	case "ConfigMap":
		err = client.ConfigMaps(consoleapi.OpenShiftConsoleNamespace).Delete(consoleapi.OpenShiftConsoleConfigMapName, &metav1.DeleteOptions{})
	case "Service":
		err = client.Services(consoleapi.OpenShiftConsoleNamespace).Delete(consoleapi.OpenShiftConsoleServiceName, &metav1.DeleteOptions{})
	case "Route":
		err = client.Routes(consoleapi.OpenShiftConsoleNamespace).Delete(consoleapi.OpenShiftConsoleRouteName, &metav1.DeleteOptions{})
	case "Deployment":
		fallthrough
	default:
		err = client.Deployments(consoleapi.OpenShiftConsoleNamespace).Delete(consoleapi.OpenShiftConsoleDeploymentName, &metav1.DeleteOptions{})
	}
	return err
}

// DeleteCompletely sends a delete request and waits until the resource and
// its dependents are deleted.
func DeleteCompletely(getObject func() (runtime.Object, error), deleteObject func(*metav1.DeleteOptions) error) error {
	obj, err := getObject()
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	accessor, err := meta.Accessor(obj)
	uid := accessor.GetUID()

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

		accessor, err := meta.Accessor(obj)

		return accessor.GetUID() != uid, nil
	})
}

// IsResourceAvailable checks if tested resource is available(recreated by console-operator),
// during 10 second period. If not error will be returned.
func IsResourceAvailable(errChan chan error, client *Clientset, resource string) {
	var myObj runtime.Object
	counter := 0
	err := wait.Poll(1*time.Second, AsyncOperationTimeout, func() (stop bool, err error) {
		logrus.Printf("polling (%v) for resource %v... \n", counter, resource)
		myObj, err = GetResource(client, resource)
		//logrus.Printf("%v", myObj.)
		if err == nil {
			logrus.Printf("%v found, recreated. \n", resource)
			return true, nil
		}
		if counter == 10 {
			if err != nil {
				return true, fmt.Errorf("deleted console %s was not recreated", resource)
			}
			logrus.Printf("max retries for %v \n", resource)
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
	var myObj runtime.Object
	counter := 0
	err := wait.Poll(1*time.Second, AsyncOperationTimeout, func() (stop bool, err error) {
		logrus.Printf("polling (%v) for resource %v... \n", counter, resource)
		myObj, err = GetResource(client, resource)
		//logrus.Printf("%v", myObj)
		if err == nil {
			logrus.Printf("%v found, incorrectly recreated. \n", resource)
			return true, fmt.Errorf("deleted console %s was recreated", resource)
		}
		if !errors.IsNotFound(err) {
			logrus.Printf("%v \n", err)
			return true, err
		}
		counter++
		if counter == 10 {
			logrus.Printf("max retries for %v \n", resource)
			return true, nil
		}
		return false, nil
	})
	errChan <- err
}
