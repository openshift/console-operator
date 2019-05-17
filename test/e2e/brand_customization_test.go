package e2e

import (
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	consoleapi "github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/testframework"
)

const (
	testCustomProductName = "test-e2e-product-name"
	testCustomLogoName    = "test-e2e-configmap"
	testCustomLogoKey     = "pic.jpg"
	testMountPath         = "/var/logo/"
)

// Test prep - setup the client used by each test
func setupCustomTestCase(t *testing.T) (*testframework.Clientset, operatorsv1.ConsoleCustomization) {
	client := testframework.MustNewClientset(t, nil)
	// Get the original operator config
	originalConfig, err := client.Consoles().Get(consoleapi.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("could not get operator config, %v", err)
	}
	originalConfigCustomization := originalConfig.Spec.Customization

	return client, originalConfigCustomization
}

func cleanupCustomizationTestCase(t *testing.T, client *testframework.Clientset, originalConfigCustomization operatorsv1.ConsoleCustomization) {

	setOperatorConfigCustomization(t, client, originalConfigCustomization)

	err := client.ConfigMaps("openshift-config").Delete(testCustomLogoName, &metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("could not delete test configmap, %v", err)
	}
	testframework.WaitForSettledState(t, client)
}

// TestOperatorConfigCustomization() tests that changing the customization values on the operator-config
// will result in the customization being set on the console-config in openshift-console.
// Implicitly it ensures that the operator-config customization overrides customization set on
// console-config in openshift-config-managed, if the managed configmap exists.
func TestOperatorConfigCustomization(t *testing.T) {
	client, originalConfigCustomization := setupCustomTestCase(t)
	defer cleanupCustomizationTestCase(t, client, originalConfigCustomization)

	// Create configmap with logo in namespace openshift-config
	// Manual cmd: oc create configmap test-configmap --from-file=pic.jpg -n openshift-config
	_, err := createLogoConfigMap(client)
	if err != nil {
		t.Fatalf("error: could not create logo configmap, %v", err)
	}
	// Set customization options on the operatorConfig
	customData := operatorsv1.ConsoleCustomization{
		CustomProductName: testCustomProductName,
		CustomLogoFile: configv1.ConfigMapFileReference{
			Name: testCustomLogoName,
			Key:  testCustomLogoKey,
		},
	}
	setOperatorConfigCustomization(t, client, customData)

	// Verify options appear in the console-config in openshift-console
	err = wait.Poll(1*time.Second, pollTimeout, func() (stop bool, err error) {
		productName, logo := getConsoleCustomization(t, client)
		return (productName == testCustomProductName) && (logo == testMountPath+testCustomLogoKey), nil
	})
	if err != nil {
		t.Fatalf("error: customization values not found  %v", err)
	}

	// Verify mounts and volumes appear correctly on the deployment console in openshift-console
	err = wait.Poll(1*time.Second, pollTimeout, func() (stop bool, err error) {
		deployment, err := testframework.GetConsoleDeployment(client)
		foundCustomizationVolume := findCustomizationVolume(deployment)
		foundCustomizationMount := findCustomizationVolumeMount(deployment)
		return foundCustomizationVolume && foundCustomizationMount, nil
	})
	if err != nil {
		t.Fatalf("error: customization values not on deployment, %v", err)
	}
}

func findCustomizationVolume(deployment *appsv1.Deployment) bool {
	volumes := deployment.Spec.Template.Spec.Volumes
	for _, volume := range volumes {
		if volume.Name == testCustomLogoName {
			return true
		}
	}
	return false
}

func findCustomizationVolumeMount(deployment *appsv1.Deployment) bool {
	mounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
	for _, mount := range mounts {
		if (mount.Name == testCustomLogoName) && (mount.MountPath == testMountPath) {
			return true
		}
	}
	return false
}

// Set Customization on the operator config
func setOperatorConfigCustomization(t *testing.T, client *testframework.Clientset, cust operatorsv1.ConsoleCustomization) {
	operatorConfig, err := client.Consoles().Get(consoleapi.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("could not get operator config, %v", err)
	}
	spec := operatorsv1.ConsoleSpec{
		OperatorSpec: operatorsv1.OperatorSpec{
			ManagementState: "Managed",
		},
		Customization: cust,
	}
	operatorConfig.Spec = spec
	_, err = client.Consoles().Update(operatorConfig)
	if err != nil {
		t.Fatalf("could not update operator config with customization=%v, %v", cust, err)
	}
}

// Get the brand from the console-config in the data of the console CM
func getConsoleCustomization(t *testing.T, client *testframework.Clientset) (string, string) {
	cm, err := testframework.GetConsoleConfigMap(client)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	data := cm.Data["console-config.yaml"]
	logoValue := ""
	customProductName := ""
	temp := strings.Split(data, "\n")
	for _, item := range temp {
		if strings.Contains(item, "customLogoFile") {
			logoValue = strings.Split(strings.TrimSpace(item), ":")[1]
		}
		if strings.Contains(item, "customProductName") {
			customProductName = strings.Split(strings.TrimSpace(item), ":")[1]
		}
	}
	t.Logf("productName:%s, logo:%s", customProductName, logoValue)
	return strings.TrimSpace(customProductName), strings.TrimSpace(logoValue)
}

// Helper function that creates a test configmap with binarydata
func createLogoConfigMap(client *testframework.Clientset) (*v1.ConfigMap, error) {
	var data = make(map[string][]byte)
	data[testCustomLogoKey] = []byte("TESTING")

	cm := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              testCustomLogoName,
			Namespace:         "openshift-config",
			CreationTimestamp: metav1.Time{},
		},
		BinaryData: data,
	}
	return client.ConfigMaps("openshift-config").Create(cm)

}
