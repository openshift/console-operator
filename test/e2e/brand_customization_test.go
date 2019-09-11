package e2e

import (
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
	customLogoVolumeName = "custom-logo"
	customLogoMountPath  = "/var/logo/"
)

func setupCustomBrandTest(t *testing.T, cmName string, fileName string) (*framework.ClientSet, *operatorsv1.Console) {
	clientSet, operatorConfig := framework.StandardSetup(t)
	// ensure it doesn't exist already for some reason
	err := deleteCustomLogoConfigMap(clientSet, cmName)
	if err != nil && !apiErrors.IsNotFound(err) {
		t.Fatalf("could not cleanup previous custom logo configmap, %v", err)
	}
	_, err = createCustomLogoConfigMap(clientSet, cmName, fileName)
	if err != nil && !apiErrors.IsAlreadyExists(err) {
		t.Fatalf("could not create custom logo configmap, %v", err)
	}

	return clientSet, operatorConfig
}
func cleanupCustomBrandTest(t *testing.T, client *framework.ClientSet, cmName string) {
	err := deleteCustomLogoConfigMap(client, cmName)
	if err != nil {
		t.Fatalf("could not delete custom logo configmap, %v", err)
	}
	framework.StandardCleanup(t, client)
}

// TODO: consider break this into several different tests.
// - setup should be same setup across them
// - check for just one thing
// - call cleanup
// - too many things in series here, probably
//
//
// TestBrandCustomization() tests that changing the customization values on the operator-config
// will result in the customization being set on the console-config in openshift-console.
// Implicitly it ensures that the operator-config customization overrides customization set on
// console-config in openshift-config-managed, if the managed configmap exists.
func TestCustomBrand(t *testing.T) {
	// create a configmap with the new logo
	customProductName := "custom name"
	customLogoConfigMapName := "custom-logo"
	customLogoFileName := "pic.png"
	pollFrequency := 1 * time.Second
	pollStandardMax := 30 * time.Second // TODO: maybe longer is all that was needed.
	pollLongMax := 120 * time.Second

	client, operatorConfig := setupCustomBrandTest(t, customLogoConfigMapName, customLogoFileName)
	// cleanup, defer deletion of the configmap to ensure it happens even if another part of the test fails
	defer cleanupCustomBrandTest(t, client, customLogoConfigMapName)

	originalConfig := operatorConfig.DeepCopy()
	operatorConfigWithCustomLogo := withCustomBrand(*operatorConfig, customProductName, customLogoConfigMapName, customLogoFileName)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := client.Operator.Consoles().Update(operatorConfigWithCustomLogo)
		return err
	})
	if err != nil {
		t.Fatalf("could not update operator config with custom product name %v and logo %v via configmap %v (%v)", customProductName, customLogoFileName, customLogoConfigMapName, err)
	}

	// check console-config in openshift-console and verify the config has made it through
	err = wait.Poll(pollFrequency, pollStandardMax, func() (stop bool, err error) {
		cm, err := framework.GetConsoleConfigMap(client)
		if hasCustomBranding(cm, customProductName, customLogoFileName) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatalf("custom branding not found in console config in openshift-console namespace")
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
		t.Fatalf("configmap custom-logo not found in openshift-console")
	}

	// ensure the volume mounts have been added to the deployment
	err = wait.Poll(pollFrequency, pollLongMax, func() (stop bool, err error) {
		deployment, err := framework.GetConsoleDeployment(client)
		volume := findCustomLogoVolume(deployment)
		volumeMount := findCustomLogoVolumeMount(deployment)
		return volume && volumeMount, nil
	})
	if err != nil {
		t.Fatalf("error: customization values not on deployment, %v", err)
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		operatorConfig, err := client.Operator.Consoles().Get(consoleapi.ConfigResourceName, metav1.GetOptions{})
		operatorConfig.Spec = originalConfig.Spec
		_, err = client.Operator.Consoles().Update(operatorConfig)
		return err
	})
	if err != nil {
		t.Fatalf("could not clear customizations from operator config: %v", err)
	}

	// ensure that the custom-logo configmap in openshift-console has been removed
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

// dont pass a pointer, we want a copy
func withCustomBrand(operatorConfig operatorsv1.Console, productName string, configMapName string, fileName string) *operatorsv1.Console {
	operatorConfig.Spec.Customization = operatorsv1.ConsoleCustomization{
		CustomProductName: productName,
		CustomLogoFile: configv1.ConfigMapFileReference{
			Name: configMapName,
			Key:  fileName,
		},
	}
	return &operatorConfig
}

func customLogoConfigmap(configMapName string, imageKey string) *v1.ConfigMap {
	var data = make(map[string][]byte)
	data[imageKey] = []byte("iVBORw0KGgoAAAANSUhEUgAAAGQAAABkCAYAAABw4pVUAAADmklEQVR4Xu2bv0tyURzGv1KLDoUgLoIShEODDbaHf0JbuNfSkIlR2NAf4NTgoJtQkoODq4OWlVu0NUWzoERQpmDhywm6vJh6L3bu5SmeM8bx3Od+Pvfx/jLX9vb2UDhgCLgoBMbFZxAKwfJBIWA+KIRC0AiA5eE5hELACIDFYUMoBIwAWBw2hELACIDFYUMoBIwAWBw2hELACIDFYUMoBIwAWBw2hELACIDFYUMoBIwAWBw25C8I8Xg8srCwYOxKp9OR9/d3LbvmdrtlcXHRlrW1BLR5kZkacnR0JMFg0IhWq9WkVCppiXp4eChLS0vGWpeXl1IsFrWs/RsW0SKkXq/L+fm5lv0dFXJ1dSWnp6da1v4Ni1AImCUKoZDpBKLRqKysrBiTbm5u5PHxEQybfXHgGmLfrv6OlSkEzJN2IaFQSFZXVyUcDovX65XX11d5enoSdbV0f39vuvt+v18CgYAx7+HhQV5eXkw/91cmaBNyfX0tqVRK1I3dpPH29ibZbFYU5EmDl70z/MPO6I1hu90Wn88nLpfL9EAdDoeSy+Xk7u5u7FwK0SBkHNmPjw+Zm5sbC30wGEgikRj7uIVCNApRR//FxYVUKhXp9XoyPz8vsVhMNjY2vskpFArSbDa/CaMQTUKmfRWp517qa+7/oU7wJycnFDJCQMtJXa2pHi6qh4yTRjqdFnUF9jVarZYcHx9TiB1C+v2+7O7uTj2hb25ufn59fY3n52c5ODigEDuEqMvYTCYzVcj6+rrE43FjTrfblWQySSF2CLm9vZV8Pj9VyNrammxtbRlz1D3J3t4ehdghpNFoyNnZGYWY3oWZT9ByUrfygooNMZehZlCINU6OzaIQx1Bb2xCFWOPk2CwKcQy1tQ1RiDVOjs2iEMdQW9sQhVjj5NgsCnEMtbUNUYg1To7NmknI6EukarUq5XJ5auhIJCI7OzvGHPXDBfUOfnTs7+/L8vKy8Wedvxt2jOoPNjSTkB9sjx81IUAhYIcIhVAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhoAJ+QeTS82niTWiVwAAAABJRU5ErkJggg==")

	cm := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              configMapName,
			Namespace:         consoleapi.OpenShiftConfigNamespace,
			CreationTimestamp: metav1.Time{},
		},
		BinaryData: data,
	}
	return cm
}

func createCustomLogoConfigMap(client *framework.ClientSet, configMapName string, imageKey string) (*v1.ConfigMap, error) {
	return client.Core.ConfigMaps(consoleapi.OpenShiftConfigNamespace).Create(customLogoConfigmap(configMapName, imageKey))
}

func deleteCustomLogoConfigMap(client *framework.ClientSet, configMapName string) error {
	return client.Core.ConfigMaps(consoleapi.OpenShiftConfigNamespace).Delete(configMapName, &metav1.DeleteOptions{})
}
