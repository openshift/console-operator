package e2e

import (
	"context"
	"reflect"
	"testing"
	"time"

	consolev1alpha "github.com/openshift/api/console/v1alpha1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	yaml "gopkg.in/yaml.v2"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	consoleapi "github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
	"github.com/openshift/console-operator/test/e2e/framework"
)

const (
	availablePluginName   = "test-plugin"
	unavailablePluginName = "missing-test-plugin"
)

func setupPluginsTestCase(t *testing.T) (*framework.ClientSet, *operatorsv1.Console) {
	return framework.StandardSetup(t)
}

func cleanupPluginsTestCase(t *testing.T, client *framework.ClientSet) {
	framework.StandardCleanup(t, client)

	err := client.ConsolePlugin.Delete(context.TODO(), availablePluginName, metav1.DeleteOptions{})
	if err != nil && !apiErrors.IsNotFound(err) {
		t.Fatalf("could not delete cleanup %q plugin, %v", availablePluginName, err)
	}
	framework.StandardCleanup(t, client)
}

// TestAddPlugins is adding available and unavailable plugin names to the console-operator config.
// Only plugin that is available on the cluster will be set with his endpoint into the console-config ConfigMap.
func TestAddPlugins(t *testing.T) {
	expectedPlugins := map[string]string{availablePluginName: "https://test-plugin-service-name.test-plugin-service-namespace.svc.cluster.local:8443/manifest"}
	currentPlugins := map[string]string{}
	client, _ := setupPluginsTestCase(t)
	defer cleanupPluginsTestCase(t, client)

	plugin := &consolev1alpha.ConsolePlugin{
		ObjectMeta: v1.ObjectMeta{
			Name: availablePluginName,
		},
		Spec: consolev1alpha.ConsolePluginSpec{
			DisplayName: "TestPlugin",
			Service: consolev1alpha.ConsolePluginService{
				Name:      "test-plugin-service-name",
				Namespace: "test-plugin-service-namespace",
				Port:      8443,
				BasePath:  "/manifest",
			},
		},
	}

	_, err := client.ConsolePlugin.Create(context.TODO(), plugin, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("could not create ConsolePlugin custom resource: %s", err)
	}
	enabledPlugins := []string{availablePluginName, unavailablePluginName}
	setOperatorConfigPlugins(t, client, enabledPlugins)

	err = wait.Poll(1*time.Second, pollTimeout, func() (stop bool, err error) {
		currentPlugins = getConsolePluginsField(t, client)
		if reflect.DeepEqual(expectedPlugins, currentPlugins) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Errorf("error: expected '%v' plugins, got '%v': '%v'", expectedPlugins, currentPlugins, err)
	}
}

func setOperatorConfigPlugins(t *testing.T, client *framework.ClientSet, pluginNames []string) {
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

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err = client.Operator.Consoles().Update(context.TODO(), operatorConfig, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		t.Fatalf("could not update operator config plugins: %v", err)
	}
}

func getConsolePluginsField(t *testing.T, client *framework.ClientSet) map[string]string {
	cm, err := framework.GetConsoleConfigMap(client)
	if err != nil {
		t.Fatalf("error: %s", err)
	}
	consoleConfig := consoleserver.Config{}
	err = yaml.Unmarshal([]byte(cm.Data["console-config.yaml"]), &consoleConfig)
	if err != nil {
		t.Fatalf("could not unmarshal console-config.yaml: %v", err)
	}

	return consoleConfig.Plugins
}
