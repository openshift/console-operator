package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	yaml "gopkg.in/yaml.v2"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	consoleapi "github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
	"github.com/openshift/console-operator/test/e2e/framework"
)

const (
	customLogoVolumeName       = "custom-logo"
	customLogoMountPath        = "/var/logo/"
	customProductName1         = "custom name one"
	customProductName2         = "custom name two"
	customLogoConfigMapNamePng = "custom-logo-png"
	customLogoFileNamePng      = "pic.png"
	customLogoConfigMapNameSvg = "custom-logo-svg"
	customLogoFileNameSvg      = "pic.svg"

	pollFrequency   = 1 * time.Second
	pollStandardMax = 30 * time.Second
	pollLongMax     = 120 * time.Second
)

func setupCustomBrandTest(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	clientSet, operatorConfig := framework.StandardSetup(t)
	deleteAndCreateCustomLogoConfigMap(t, clientSet, customLogoConfigMapNamePng, customLogoFileNamePng)
	deleteAndCreateCustomLogoConfigMap(t, clientSet, customLogoConfigMapNameSvg, customLogoFileNameSvg)
	return clientSet, operatorConfig
}

func cleanupCustomBrandTest(t *testing.T, clientSet *framework.ClientSet) {
	cleanupCustomLogoConfigMap(t, clientSet, customLogoConfigMapNamePng)
	cleanupCustomLogoConfigMap(t, clientSet, customLogoConfigMapNameSvg)
	framework.StandardCleanup(t, clientSet)
}

func deleteAndCreateCustomLogoConfigMap(t *testing.T, clientSet *framework.ClientSet, customLogoConfigMapName string, customLogoFileName string) {
	// ensure it doesn't exist already for some reason
	err := deleteCustomLogoConfigMap(clientSet, customLogoConfigMapName)
	if err != nil && !apiErrors.IsNotFound(err) {
		t.Fatalf("could not delete cleanup previous %q configmap, %v", customLogoConfigMapName, err)
	}
	_, err = createCustomLogoConfigMap(clientSet, customLogoConfigMapName, customLogoFileName)
	if err != nil && !apiErrors.IsAlreadyExists(err) {
		t.Fatalf("could not create %q configmap, %v", customLogoConfigMapName, err)
	}
}

func cleanupCustomLogoConfigMap(t *testing.T, clientSet *framework.ClientSet, customLogoConfigMapName string) {
	err := deleteCustomLogoConfigMap(clientSet, customLogoConfigMapName)
	if err != nil {
		t.Fatalf("could not delete %q configmap, %v", customLogoConfigMapName, err)
	}
}

// TestBrandCustomization() tests that changing the customization values on the operator config
// will result in the customization being set on the console-config configmap in openshift-console.
// The test covers following cases, in given order:
//  - image with binary representation (.png) is set as a custom-logo
//  - custom-logo gets changed
//  - image with string representation (.svg) is set as a custom-logo
//  - custom-logo gets unset
func TestCustomBrand(t *testing.T) {
	// create a configmaps with binary and string image type representation
	client, operatorConfig := setupCustomBrandTest(t)
	// cleanup, defer deletion of the configmaps to ensure it happens even if another part of the test fails
	defer cleanupCustomBrandTest(t, client)

	originalConfig := operatorConfig.DeepCopy()

	customizationSuites := []struct {
		customProductName       string
		customLogoConfigMapName string
		customLogoFileName      string
	}{
		{customProductName1, customLogoConfigMapNamePng, customLogoFileNamePng},
		{customProductName2, customLogoConfigMapNameSvg, customLogoFileNameSvg},
	}
	// set customization suites, to test custom-logo edits(in following order):
	//  - with '.png' image type - binary
	//  - with '.svg' image type - string
	for _, suite := range customizationSuites {
		err := setAndCheckCustomLogo(t, client, suite.customProductName, suite.customLogoConfigMapName, suite.customLogoFileName)
		if err != nil {
			t.Fatalf("error: %s", err)
		}
	}

	// remove custom configuration from console config
	t.Log("removing custom-product-name and custom-logo")
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		operatorConfig, err := client.Operator.Consoles().Get(context.TODO(), consoleapi.ConfigResourceName, metav1.GetOptions{})
		operatorConfig.Spec = originalConfig.Spec
		_, err = client.Operator.Consoles().Update(context.TODO(), operatorConfig, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		t.Fatalf("could not clear customizations from operator config (%s)", err)
	}

	// ensure that the custom-product-name configmap in openshift-console has been removed
	err = wait.Poll(pollFrequency, pollStandardMax, func() (stop bool, err error) {
		_, err = framework.GetCustomLogoConfigMap(client)
		if apiErrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		// Try until timeout
		return false, nil
	})
	if err != nil {
		t.Fatalf("configmap custom-logo not found in openshift-console")
	}
}

func setAndCheckCustomLogo(t *testing.T, client *framework.ClientSet, customProductName string, customLogoConfigMapName string, customLogoFileName string) error {
	// set operator config with the custom-logo and custom-product-name
	t.Logf("setting custom-product-name and custom-logo in %q format", strings.Split(customLogoFileName, ".")[1])
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		operatorConfig, err := client.Operator.Consoles().Get(context.TODO(), consoleapi.ConfigResourceName, metav1.GetOptions{})
		operatorConfigWithCustomLogo := withCustomBrand(operatorConfig, customProductName, customLogoConfigMapName, customLogoFileName)
		_, err = client.Operator.Consoles().Update(context.TODO(), operatorConfigWithCustomLogo, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return fmt.Errorf("could not update operator config with custom product name %q and logo %q via configmap %q (%s)", customProductName, customLogoFileName, customLogoConfigMapName, err)
	}

	// check console-config in openshift-console and verify the config has made it through
	err = wait.Poll(pollFrequency, pollStandardMax, func() (stop bool, err error) {
		configMap, err := framework.GetConsoleConfigMap(client)
		if hasCustomBranding(configMap, customProductName, customLogoFileName) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("custom branding for %q file not found in console config in openshift-console namespace", customLogoFileName)
	}

	// ensure that custom-logo in openshift-console has been created
	err = wait.Poll(pollFrequency, pollStandardMax, func() (stop bool, err error) {
		_, err = framework.GetCustomLogoConfigMap(client)
		if apiErrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("configmap custom-logo not found in openshift-console")
	}

	// ensure the volume mounts have been added to the deployment
	err = wait.Poll(pollFrequency, pollLongMax, func() (stop bool, err error) {
		deployment, err := framework.GetConsoleDeployment(client)
		volume := findCustomLogoVolume(deployment)
		volumeMount := findCustomLogoVolumeMount(deployment)
		return volume && volumeMount, nil
	})
	if err != nil {
		return fmt.Errorf("customization values for %q file not on deployment (%s)", customLogoFileName, err)
	}
	return nil
}

func hasCustomBranding(cm *v1.ConfigMap, desiredProductName string, desiredLogoFileName string) bool {
	consoleConfig := consoleserver.Config{}
	yaml.Unmarshal([]byte(cm.Data["console-config.yaml"]), &consoleConfig)
	actualProductName := consoleConfig.Customization.CustomProductName
	actualLogoFile := consoleConfig.Customization.CustomLogoFile
	return (desiredProductName == actualProductName) && ("/var/logo/"+desiredLogoFileName == actualLogoFile)
}

func findCustomLogoVolume(deployment *appsv1.Deployment) bool {
	volumes := deployment.Spec.Template.Spec.Volumes
	for _, volume := range volumes {
		if volume.Name == customLogoVolumeName {
			return true
		}
	}
	return false
}

func findCustomLogoVolumeMount(deployment *appsv1.Deployment) bool {
	mounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
	for _, mount := range mounts {
		if (mount.Name == customLogoVolumeName) && (mount.MountPath == customLogoMountPath) {
			return true
		}
	}
	return false
}

func withCustomBrand(operatorConfig *operatorsv1.Console, customProductName string, customLogoConfigMapName string, customLogoFileName string) *operatorsv1.Console {
	operatorConfig.Spec.Customization = operatorsv1.ConsoleCustomization{
		CustomProductName: customProductName,
		CustomLogoFile: configv1.ConfigMapFileReference{
			Name: customLogoConfigMapName,
			Key:  customLogoFileName,
		},
	}
	return operatorConfig
}

func customLogoConfigmap(customLogoConfigMapName string, imageKey string) *v1.ConfigMap {
	configMap := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              customLogoConfigMapName,
			Namespace:         consoleapi.OpenShiftConfigNamespace,
			CreationTimestamp: metav1.Time{},
		},
	}
	if strings.HasSuffix(imageKey, "png") {
		binaryData := make(map[string][]byte)
		binaryData[imageKey] = []byte("iVBORw0KGgoAAAANSUhEUgAAAGQAAABkCAYAAABw4pVUAAADmklEQVR4Xu2bv0tyURzGv1KLDoUgLoIShEODDbaHf0JbuNfSkIlR2NAf4NTgoJtQkoODq4OWlVu0NUWzoERQpmDhywm6vJh6L3bu5SmeM8bx3Od+Pvfx/jLX9vb2UDhgCLgoBMbFZxAKwfJBIWA+KIRC0AiA5eE5hELACIDFYUMoBIwAWBw2hELACIDFYUMoBIwAWBw2hELACIDFYUMoBIwAWBw2hELACIDFYUMoBIwAWBw25C8I8Xg8srCwYOxKp9OR9/d3LbvmdrtlcXHRlrW1BLR5kZkacnR0JMFg0IhWq9WkVCppiXp4eChLS0vGWpeXl1IsFrWs/RsW0SKkXq/L+fm5lv0dFXJ1dSWnp6da1v4Ni1AImCUKoZDpBKLRqKysrBiTbm5u5PHxEQybfXHgGmLfrv6OlSkEzJN2IaFQSFZXVyUcDovX65XX11d5enoSdbV0f39vuvt+v18CgYAx7+HhQV5eXkw/91cmaBNyfX0tqVRK1I3dpPH29ibZbFYU5EmDl70z/MPO6I1hu90Wn88nLpfL9EAdDoeSy+Xk7u5u7FwK0SBkHNmPjw+Zm5sbC30wGEgikRj7uIVCNApRR//FxYVUKhXp9XoyPz8vsVhMNjY2vskpFArSbDa/CaMQTUKmfRWp517qa+7/oU7wJycnFDJCQMtJXa2pHi6qh4yTRjqdFnUF9jVarZYcHx9TiB1C+v2+7O7uTj2hb25ufn59fY3n52c5ODigEDuEqMvYTCYzVcj6+rrE43FjTrfblWQySSF2CLm9vZV8Pj9VyNrammxtbRlz1D3J3t4ehdghpNFoyNnZGYWY3oWZT9ByUrfygooNMZehZlCINU6OzaIQx1Bb2xCFWOPk2CwKcQy1tQ1RiDVOjs2iEMdQW9sQhVjj5NgsCnEMtbUNUYg1To7NmknI6EukarUq5XJ5auhIJCI7OzvGHPXDBfUOfnTs7+/L8vKy8Wedvxt2jOoPNjSTkB9sjx81IUAhYIcIhVAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhoAJ+QeTS82niTWiVwAAAABJRU5ErkJggg==")
		configMap.BinaryData = binaryData
		return configMap
	}
	data := make(map[string]string)
	data[imageKey] = `<svg width="50" height="50" xmlns="http://www.w3.org/2000/svg"><circle cx="25" cy="25" r="20"/></svg>`
	configMap.Data = data
	return configMap
}

func createCustomLogoConfigMap(client *framework.ClientSet, customLogoConfigMapName string, imageKey string) (*v1.ConfigMap, error) {
	return client.Core.ConfigMaps(consoleapi.OpenShiftConfigNamespace).Create(context.TODO(), customLogoConfigmap(customLogoConfigMapName, imageKey), metav1.CreateOptions{})
}

func deleteCustomLogoConfigMap(client *framework.ClientSet, customLogoConfigMapName string) error {
	return client.Core.ConfigMaps(consoleapi.OpenShiftConfigNamespace).Delete(context.TODO(), customLogoConfigMapName, metav1.DeleteOptions{})
}
