package testframework

import (
	"fmt"
	"testing"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
		res, err = client.ConfigMaps(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.OpenShiftConsoleConfigMapName, metav1.GetOptions{})
	case "Service":
		res, err = client.Services(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.OpenShiftConsoleServiceName, metav1.GetOptions{})
	case "Route":
		res, err = client.Routes(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.OpenShiftConsoleRouteName, metav1.GetOptions{})
	case "Deployment":
		fallthrough
	default:
		res, err = client.Deployments(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.OpenShiftConsoleDeploymentName, metav1.GetOptions{})
	}
	return res, err
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

	uid := getUID(obj)

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

		return getUID(obj) != uid, nil
	})
}

func getUID(obj runtime.Object) types.UID {
	configMap, ok := obj.(*corev1.ConfigMap)
	if ok {
		return configMap.ObjectMeta.GetUID()
	}
	service, ok := obj.(*corev1.Service)
	if ok {
		return service.ObjectMeta.GetUID()
	}
	route, ok := obj.(*routev1.Route)
	if ok {
		return route.ObjectMeta.GetUID()
	}
	deployment, _ := obj.(*appv1.Deployment)
	return deployment.ObjectMeta.GetUID()
}

// IsResourceAvailable checks if tested resource is available(recreated by console-operator),
// during 10 second period. If not error will be returned.
func IsResourceAvailable(errChan chan error, client *Clientset, resource string) {
	counter := 0
	err := wait.Poll(1*time.Second, AsyncOperationTimeout, func() (stop bool, err error) {
		_, err = GetResource(client, resource)
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

func IsResourceAvailable_(client *Clientset, resource string) error {
	counter := 0
	err := wait.Poll(1*time.Second, AsyncOperationTimeout, func() (stop bool, err error) {
		_, err = GetResource(client, resource)
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
	return err
}

// IsResourceUnavailable checks if tested resource is unavailable(not recreated by console-operator),
// during 10 second period. If not error will be returned.
func IsResourceUnavailable(errChan chan error, client *Clientset, resource string) {
	counter := 0
	err := wait.Poll(1*time.Second, AsyncOperationTimeout, func() (stop bool, err error) {
		_, err = GetResource(client, resource)
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
