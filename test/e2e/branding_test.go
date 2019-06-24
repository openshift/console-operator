package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	operatorsv1 "github.com/openshift/api/operator/v1"
	consoleapi "github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/testframework"
)

var validOperatorConfigBrands = []operatorsv1.Brand{operatorsv1.BrandOKD, operatorsv1.BrandOCP, operatorsv1.BrandOnline, operatorsv1.BrandDedicated, operatorsv1.BrandAzure}
var validManagedConfigMapBrands = []operatorsv1.Brand{operatorsv1.BrandOKD, operatorsv1.BrandOCP}

// Test prep - setup the client used by each test
func setupBrandingTestCase(t *testing.T) (*testframework.Clientset, operatorsv1.Brand, map[string]string) {
	client := testframework.MustNewClientset(t, nil)
	// Get the original operator config
	originalConfig, err := client.Consoles().Get(consoleapi.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("could not get operator config, %v", err)
	}
	originalConfigBrand := originalConfig.Spec.Customization.Brand
	// Get the original Managed Config Map
	originalManagedConfigMap, err := client.ConfigMaps(consoleapi.OpenShiftConfigManagedNamespace).Get(consoleapi.OpenShiftConsoleConfigMapName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		t.Fatalf("could not get console-config configmap, %v", err)
	}

	return client, originalConfigBrand, originalManagedConfigMap.Data
}

func cleanupBrandingTestCase(t *testing.T, client *testframework.Clientset, originalConfigBrand operatorsv1.Brand, originalManagedConfigMapData map[string]string) {
	err := setOperatorConfigBrand(client, "")
	if err != nil {
		t.Fatalf("could not get operator config, %v", err)
	}

	managedConfigMap := generateTestConfigMap(operatorsv1.BrandOKD)
	if originalManagedConfigMapData != nil {
		managedConfigMap.Data = originalManagedConfigMapData
	}
	_, err = client.ConfigMaps(consoleapi.OpenShiftConfigManagedNamespace).Update(managedConfigMap)
	if err != nil {
		t.Fatalf("could not reset managed config map  %v", err)
	}
	testframework.WaitForSettledState(t, client)
}

// TestOperatorConfigBranding() tests that changing the brand value on the operator-config
// will result in the brand being set on the console-config in openshift-console.
// Implicitly it ensures that the operator-config brand overrides brand set on
// console-config in openshift-config-managed, if the managed configmap exists.
func TestOperatorConfigBranding(t *testing.T) {
	client, originalConfigBrand, originalManagedConfigMapData := setupBrandingTestCase(t)
	defer cleanupBrandingTestCase(t, client, originalConfigBrand, originalManagedConfigMapData)
	// Set a temporary managed config to test it does not override the operator config values
	_, err := updateOrCreateConsoleConfigMap(client, generateTestConfigMap(operatorsv1.BrandOKD))
	if err != nil {
		t.Fatalf("error: could not apply managed config map %v", err)
	}

	for _, expectedBrand := range validOperatorConfigBrands {
		t.Logf("update operator with %v", expectedBrand)
		// now check if it has set the brand
		err = wait.Poll(1*time.Second, testframework.AsyncOperationTimeout, func() (stop bool, err error) {
			// helper to update the operator config
			err = setOperatorConfigBrand(client, expectedBrand)
			if err != nil {
				return false, nil
			}
			gotBrand := getConsoleBrand(t, client)
			return (gotBrand == expectedBrand), nil
		})
		if err != nil {
			t.Fatalf("error: brand was never updated, %v", err)
		}
	}
}

// Test setting brand via the config map in the openshift-config-managed namespace, this requires the operator config not be set
func TestBrandingFromManagedConfigMap(t *testing.T) {
	client, originalConfigBrand, originalManagedConfigMapData := setupBrandingTestCase(t)
	defer cleanupBrandingTestCase(t, client, originalConfigBrand, originalManagedConfigMapData)
	// Set operator config to empty so it does not override the managed config map values
	err := setOperatorConfigBrand(client, "")
	if err != nil {
		t.Fatalf("could not get operator config, %v", err)
	}

	for _, expectedBrand := range validManagedConfigMapBrands {
		t.Logf("update data for the config map in openshift-config-managed namespace with %v", expectedBrand)
		// Create new configmap for test
		_, err := updateOrCreateConsoleConfigMap(client, generateTestConfigMap(expectedBrand))
		if err != nil {
			t.Fatalf("error: could not apply managed config map %v", err)
		}

		err = wait.Poll(1*time.Second, pollTimeout, func() (stop bool, err error) {
			gotBrand := getConsoleBrand(t, client)
			return (gotBrand == expectedBrand), nil
		})
		if err != nil {
			t.Fatalf("error: brand was never updated, %v", err)
		}
	}
}

func generateTestConfigMap(brand operatorsv1.Brand) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "console-config",
			Namespace: "openshift-config-managed",
		},
		Data: map[string]string{
			"console-config.yaml": `kind: ConsoleConfig
apiVersion: console.openshift.io/v1
customization:
  branding: ` + string(brand) + `
  documentationBaseURL: https://docs.okd.io/4.0/
`,
		},
		BinaryData: nil,
	}
}

// Set Brand on the operator config
func setOperatorConfigBrand(client *testframework.Clientset, brand operatorsv1.Brand) (err error) {
	operatorConfig, err := client.Consoles().Get(consoleapi.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	spec := operatorsv1.ConsoleSpec{
		OperatorSpec: operatorsv1.OperatorSpec{
			ManagementState: "Managed",
		},
		Customization: operatorsv1.ConsoleCustomization{
			Brand: brand,
		},
	}
	operatorConfig.Spec = spec
	_, err = client.Consoles().Update(operatorConfig)
	if err != nil {
		return err
	}
	return nil
}

// Get the brand from the console-config in the data of the console CM
func getConsoleBrand(t *testing.T, client *testframework.Clientset) operatorsv1.Brand {
	cm, err := testframework.GetConsoleConfigMap(client)
	if err != nil {
		t.Fatalf("error: %s", err)
	}

	data := cm.Data["console-config.yaml"]
	brandingValue := ""
	temp := strings.Split(data, "\n")
	for _, item := range temp {
		if strings.Contains(item, "branding") {
			brandingValue = strings.Split(strings.TrimSpace(item), ":")[1]
		}
	}
	brand, _ := stringToBrand(strings.TrimSpace(brandingValue))
	return brand
}

// Helper function to convert string version of valid brands to their corresponding constant name
func stringToBrand(brandstr string) (b operatorsv1.Brand, ok bool) {
	switch found := operatorsv1.Brand(brandstr); found {
	case
		operatorsv1.BrandDedicated,
		operatorsv1.BrandOnline,
		operatorsv1.BrandOKD,
		operatorsv1.BrandOCP,
		operatorsv1.BrandOpenShift,
		operatorsv1.BrandAzure:
		return found, true
	default:
		fmt.Printf("invalid brand (%s), defaulting to %s \n", brandstr, operatorsv1.BrandOKD)
		return operatorsv1.BrandOKD, false
	}
}

// Helper function that decides whether to update a config map (if it exists) or create a new one
func updateOrCreateConsoleConfigMap(client *testframework.Clientset, cm *v1.ConfigMap) (*v1.ConfigMap, error) {
	// Check if configMap exist so we know whether to update or create
	_, err := client.ConfigMaps(cm.ObjectMeta.Namespace).Get(cm.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return client.ConfigMaps(cm.ObjectMeta.Namespace).Create(cm)
	} else if err != nil {
		return nil, err
	}
	return client.ConfigMaps(cm.ObjectMeta.Namespace).Update(cm)

}
