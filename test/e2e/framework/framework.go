package framework

import (
	"fmt"
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func DeleteAll(t *testing.T, client *ClientSet) {
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

func GetResource(client *ClientSet, resource string) (runtime.Object, error) {
	var res runtime.Object
	var err error
	switch resource {
	case "ConfigMap":
		res, err = GetConsoleConfigMap(client)
	case "ConfigMapPublic":
		res, err = GetPublicConsoleConfigMap(client)
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

func GetTopLevelConfig(client *ClientSet) (*configv1.Console, error) {
	return client.Console.Consoles().Get(consoleapi.ConfigResourceName, metav1.GetOptions{})
}

func GetOperatorConfig(client *ClientSet) (*operatorv1.Console, error) {
	return client.Operator.Consoles().Get(consoleapi.ConfigResourceName, metav1.GetOptions{})
}

// custom-logo in openshift-console should exist when custom branding is used
func GetCustomLogoConfigMap(client *ClientSet) (*corev1.ConfigMap, error) {
	return client.Core.ConfigMaps(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.OpenShiftCustomLogoConfigMapName, metav1.GetOptions{})
}

func GetConsoleConfigMap(client *ClientSet) (*corev1.ConfigMap, error) {
	return client.Core.ConfigMaps(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.OpenShiftConsoleConfigMapName, metav1.GetOptions{})
}

func GetPublicConsoleConfigMap(client *ClientSet) (*corev1.ConfigMap, error) {
	return client.Core.ConfigMaps(consoleapi.OpenShiftConfigManagedNamespace).Get(consoleapi.OpenShiftConsolePublicConfigMapName, metav1.GetOptions{})
}

func GetConsoleService(client *ClientSet) (*corev1.Service, error) {
	return client.Core.Services(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.OpenShiftConsoleServiceName, metav1.GetOptions{})
}

func GetConsoleRoute(client *ClientSet) (*routev1.Route, error) {
	return client.Routes.Routes(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.OpenShiftConsoleRouteName, metav1.GetOptions{})
}

func GetConsoleDeployment(client *ClientSet) (*appv1.Deployment, error) {
	deployment, err := client.Apps.Deployments(consoleapi.OpenShiftConsoleNamespace).Get(consoleapi.OpenShiftConsoleDeploymentName, metav1.GetOptions{})
	return deployment, err
}

func deleteResource(client *ClientSet, resource string) error {
	var err error
	switch resource {
	case "ConfigMap":
		err = client.Core.ConfigMaps(consoleapi.OpenShiftConsoleNamespace).Delete(consoleapi.OpenShiftConsoleConfigMapName, &metav1.DeleteOptions{})
	case "Service":
		err = client.Core.Services(consoleapi.OpenShiftConsoleNamespace).Delete(consoleapi.OpenShiftConsoleServiceName, &metav1.DeleteOptions{})
	case "Route":
		err = client.Routes.Routes(consoleapi.OpenShiftConsoleNamespace).Delete(consoleapi.OpenShiftConsoleRouteName, &metav1.DeleteOptions{})
	case "Deployment":
		fallthrough
	default:
		err = client.Apps.Deployments(consoleapi.OpenShiftConsoleNamespace).Delete(consoleapi.OpenShiftConsoleDeploymentName, &metav1.DeleteOptions{})
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

func ConsoleResourcesAvailable(client *ClientSet) error {
	errChan := make(chan error)
	go IsResourceAvailable(errChan, client, "ConfigMap")
	go IsResourceAvailable(errChan, client, "ConfigMapPublic")
	go IsResourceAvailable(errChan, client, "Route")
	go IsResourceAvailable(errChan, client, "Service")
	go IsResourceAvailable(errChan, client, "Deployment")
	checkErr := <-errChan

	return checkErr
}

// IsResourceAvailable checks if tested resource is available during a 30 second period.
// if the resource does not exist by the end of the period, an error will be returned.
func IsResourceAvailable(errChan chan error, client *ClientSet, resource string) {
	counter := 0
	maxCount := 30
	err := wait.Poll(1*time.Second, AsyncOperationTimeout, func() (stop bool, err error) {
		_, err = GetResource(client, resource)
		if err == nil {
			return true, nil
		}
		if counter == maxCount {
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

func ConsoleResourcesUnavailable(client *ClientSet) error {
	errChan := make(chan error)
	go IsResourceUnavailable(errChan, client, "ConfigMap")
	go IsResourceUnavailable(errChan, client, "Route")
	go IsResourceUnavailable(errChan, client, "Service")
	go IsResourceUnavailable(errChan, client, "Deployment")
	checkErr := <-errChan

	return checkErr
}

// IsResourceUnavailable checks if tested resource is unavailable during a 15 second period.
// If the resource exists during that time, an error will be returned.
func IsResourceUnavailable(errChan chan error, client *ClientSet, resourceType string) {
	counter := 0
	maxCount := 15
	err := wait.Poll(1*time.Second, AsyncOperationTimeout, func() (stop bool, err error) {
		resource, err := GetResource(client, resourceType)
		if err == nil {
			fmt.Printf("%s : %#v \n", resourceType, resource)
			return true, fmt.Errorf("deleted console %s was recreated: %#v", resourceType, resource)
		}
		if !errors.IsNotFound(err) {
			return true, err
		}
		counter++
		if counter == maxCount {
			return true, nil
		}
		return false, nil
	})
	errChan <- err
}
