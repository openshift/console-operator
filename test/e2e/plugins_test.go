package e2e

import (
	"context"
	"reflect"
	"testing"
	"time"

	consolev1 "github.com/openshift/api/console/v1"
	consolev1alpha1 "github.com/openshift/api/console/v1alpha1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	yaml "gopkg.in/yaml.v2"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/openshift/console-operator/pkg/api"
	consoleapi "github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
	"github.com/openshift/console-operator/test/e2e/framework"
)

const (
	availablePluginName     = "test-plugin"
	unavailablePluginName   = "missing-test-plugin"
	pluginServiceEndpoint   = "https://test-plugin-service-name.test-plugin-service-namespace.svc.cluster.local:8443/manifest"
	expectedPluginNamespace = "plugin__test-plugin"
)

func setupPluginsTestCase(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	return framework.StandardSetup(t)
}

func cleanupPluginsTestCase(t *testing.T, client *framework.ClientSet) {
	err := client.ConsolePluginV1.Delete(context.TODO(), availablePluginName, metav1.DeleteOptions{})
	if err != nil && !apiErrors.IsNotFound(err) {
		t.Fatalf("could not delete cleanup %q plugin, %v", availablePluginName, err)
	}
	framework.StandardCleanup(t, client)
}

// TestAddPlugin is adding available and unavailable plugin names to the console-operator config.
// Only plugin that is available on the cluster will be set with his endpoint into the console-config ConfigMap.
func TestAddV1Plugins(t *testing.T) {
	expectedPlugins := map[string]string{availablePluginName: pluginServiceEndpoint}
	expertedI18nNamespaces := []string{expectedPluginNamespace}
	client, _ := setupPluginsTestCase(t)
	defer cleanupPluginsTestCase(t, client)

	plugin := &consolev1.ConsolePlugin{
		ObjectMeta: v1.ObjectMeta{
			Name: availablePluginName,
		},
		Spec: consolev1.ConsolePluginSpec{
			DisplayName: "TestPlugin",
			I18n: consolev1.ConsolePluginI18n{
				LoadType: consolev1.Preload,
			},
			Backend: consolev1.ConsolePluginBackend{
				Type: consolev1.Service,
				Service: &consolev1.ConsolePluginService{
					Name:      "test-plugin-service-name",
					Namespace: "test-plugin-service-namespace",
					Port:      8443,
					BasePath:  "/manifest",
				},
			},
		},
	}

	_, err := client.ConsolePluginV1.Create(context.TODO(), plugin, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("could not create v1 ConsolePlugin custom resource: %s", err)
	}
	enabledPlugins := []string{availablePluginName, unavailablePluginName}
	setOperatorConfigPlugins(t, client, enabledPlugins)

	err = wait.Poll(1*time.Second, pollTimeout, func() (stop bool, err error) {
		consoleConfig := getConsoleConfig(t, client)
		if reflect.DeepEqual(expectedPlugins, consoleConfig.Plugins) && reflect.DeepEqual(expertedI18nNamespaces, consoleConfig.I18nNamespaces) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Errorf("error verifying v1alpha1 ConsolePlugin %q was enabled: %s", availablePluginName, err)
	}
}

// when a v1alpha1 ConsolePLugin is creating console-operator should be able to get his converted
// v1 version, due to the conversion webhook.
func TestAddV1Alpha1Plugins(t *testing.T) {
	expectedPlugins := map[string]string{availablePluginName: pluginServiceEndpoint}
	expertedI18nNamespaces := []string{expectedPluginNamespace}
	client, _ := setupPluginsTestCase(t)
	defer cleanupPluginsTestCase(t, client)

	plugin := &consolev1alpha1.ConsolePlugin{
		ObjectMeta: v1.ObjectMeta{
			Name:        availablePluginName,
			Annotations: map[string]string{api.V1Alpha1PluginI18nAnnotation: "true"},
		},
		Spec: consolev1alpha1.ConsolePluginSpec{
			DisplayName: "TestPlugin",
			Service: consolev1alpha1.ConsolePluginService{
				Name:      "test-plugin-service-name",
				Namespace: "test-plugin-service-namespace",
				Port:      8443,
				BasePath:  "/manifest",
			},
		},
	}

	_, err := client.ConsolePluginV1Alpha1.Create(context.TODO(), plugin, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("could not create v1alpha1 ConsolePlugin custom resource: %s", err)
	}
	enabledPlugins := []string{availablePluginName, unavailablePluginName}
	setOperatorConfigPlugins(t, client, enabledPlugins)

	err = wait.Poll(1*time.Second, pollTimeout, func() (stop bool, err error) {
		consoleConfig := getConsoleConfig(t, client)
		if reflect.DeepEqual(expectedPlugins, consoleConfig.Plugins) && reflect.DeepEqual(expertedI18nNamespaces, consoleConfig.I18nNamespaces) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Errorf("error verifying v1 ConsolePlugin %q was enabled: %s", availablePluginName, err)
	}
}

func setOperatorConfigPlugins(t *testing.T, client *framework.ClientSet, pluginNames []string) {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		operatorConfig, err := client.Operator.Consoles().Get(context.TODO(), consoleapi.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("could not get operator config, %v", err)
		}
		t.Logf("setting plugins to '%v'", pluginNames)
		operatorConfig.Spec = operatorsv1.ConsoleSpec{
			OperatorSpec: operatorsv1.OperatorSpec{
				ManagementState: "Managed",
			},
			Plugins: pluginNames,
		}

		_, err = client.Operator.Consoles().Update(context.TODO(), operatorConfig, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		t.Fatalf("could not update operator config plugins: %v", err)
	}
}

func getConsoleConfig(t *testing.T, client *framework.ClientSet) consoleserver.Config {
	cm, err := framework.GetConsoleConfigMap(client)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	consoleConfig := consoleserver.Config{}
	err = yaml.Unmarshal([]byte(cm.Data["console-config.yaml"]), &consoleConfig)
	if err != nil {
		t.Fatalf("could not unmarshal console-config.yaml: %v", err)
	}

	return consoleConfig
}
