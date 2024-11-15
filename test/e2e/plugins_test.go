package e2e

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	consolev1 "github.com/openshift/api/console/v1"
	consoleapi "github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
	"github.com/openshift/console-operator/test/e2e/framework"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

const (
	availablePluginName           = "test-plugin"
	unavailablePluginName         = "missing-test-plugin"
	expectedPluginServiceEndpoint = "https://test-plugin-service-name.test-plugin-service-namespace.svc.cluster.local:8443/manifest"
	expectedPluginNamespace       = "plugin__test-plugin"
)

var (
	expectedCSP = map[consolev1.DirectiveType][]string{
		consolev1.DefaultSrc: {"source1", "source2", "source3"},
		consolev1.StyleSrc:   {"style1", "style2"},
		consolev1.ImgSrc:     {"image1"},
	}

	pluginCSPs = map[string][]consolev1.ConsolePluginCSP{
		fmt.Sprintf("%s-1", availablePluginName): {
			{Directive: consolev1.DefaultSrc, Values: []consolev1.CSPDirectiveValue{"source1", "source2"}},
			{Directive: consolev1.StyleSrc, Values: []consolev1.CSPDirectiveValue{"style1", "style2"}},
		},
		fmt.Sprintf("%s-2", availablePluginName): {
			{Directive: consolev1.DefaultSrc, Values: []consolev1.CSPDirectiveValue{"source2", "source3"}},
			{Directive: consolev1.StyleSrc, Values: []consolev1.CSPDirectiveValue{"style1"}},
			{Directive: consolev1.ImgSrc, Values: []consolev1.CSPDirectiveValue{"image1"}},
		},
	}
)

// setupTestCase initializes test case dependencies and returns a client and list of default plugins.
func setupTestCase(t *testing.T) (*framework.ClientSet, []string) {
	client, _ := framework.StandardSetup(t)
	defaultPlugins := getOperatorConfigPlugins(t, client)
	return client, defaultPlugins
}

// cleanupTestCase resets any modifications to plugins and cleans up resources.
func cleanupTestCase(t *testing.T, client *framework.ClientSet, defaultPlugins, testPlugins []string) {
	for _, plugin := range testPlugins {
		deleteConsolePlugin(t, client, plugin)
	}
	setOperatorConfigPlugins(t, client, defaultPlugins)
	framework.StandardCleanup(t, client)
}

// TestPluginsCSPAggregation verifies correct CSP aggregation from multiple plugins.
// Uncomment this test once the ConsoleContentSecurityPolicy is not behind feature gate.
// func TestPluginsCSPAggregation(t *testing.T) {
// 	client, defaultPlugins := setupTestCase(t)
// 	defer cleanupTestCase(t, client, defaultPlugins, maps.Keys(pluginCSPs))

// 	for name, csp := range pluginCSPs {
// 		createConsolePlugin(t, client, getPlugin(name, csp))
// 	}
// 	setOperatorConfigPlugins(t, client, maps.Keys(pluginCSPs))
// 	verifyConsoleConfigCSP(t, client, expectedCSP)
// }

// TestAddPlugins tests addition of available and unavailable plugins.
func TestAddPlugins(t *testing.T) {
	expectedPlugins := map[string]string{availablePluginName: expectedPluginServiceEndpoint}
	expectedI18nNamespaces := []string{expectedPluginNamespace}

	client, defaultPlugins := setupTestCase(t)
	defer cleanupTestCase(t, client, defaultPlugins, []string{availablePluginName})

	createConsolePlugin(t, client, getPlugin(availablePluginName, nil))
	setOperatorConfigPlugins(t, client, []string{availablePluginName, unavailablePluginName})
	verifyConsoleConfigPluginsAndNamespaces(t, client, expectedPlugins, expectedI18nNamespaces)
}

// verifyConsoleConfigCSP checks if the aggregated CSP in the console configuration matches expectations.
func verifyConsoleConfigCSP(t *testing.T, client *framework.ClientSet, expectedCSP map[consolev1.DirectiveType][]string) {
	err := wait.Poll(pollFrequency, pollStandardMax, func() (bool, error) {
		consoleConfig := getConsoleConfig(t, client)
		return reflect.DeepEqual(consoleConfig.ContentSecurityPolicy, expectedCSP), nil
	})
	if err != nil {
		t.Errorf("error verifying aggregated CSP configuration: %v", err)
	}
}

// verifyConsoleConfigPluginsAndNamespaces checks if the plugins and namespaces in the console configuration match expectations.
func verifyConsoleConfigPluginsAndNamespaces(t *testing.T, client *framework.ClientSet, expectedPlugins map[string]string, expectedNamespaces []string) {
	err := wait.Poll(pollFrequency, pollStandardMax, func() (bool, error) {
		consoleConfig := getConsoleConfig(t, client)
		return reflect.DeepEqual(consoleConfig.Plugins, expectedPlugins) &&
			reflect.DeepEqual(consoleConfig.I18nNamespaces, expectedNamespaces), nil
	})
	if err != nil {
		t.Errorf("error verifying ConsolePlugin %q was enabled: %v", availablePluginName, err)
	}
}

// setOperatorConfigPlugins updates the console operator's config with enabled plugins.
func setOperatorConfigPlugins(t *testing.T, client *framework.ClientSet, plugins []string) {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		operatorConfig, err := client.Operator.Consoles().Get(context.TODO(), consoleapi.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("could not get operator config: %v", err)
		}
		operatorConfig.Spec.Plugins = plugins
		_, err = client.Operator.Consoles().Update(context.TODO(), operatorConfig, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		t.Fatalf("could not update operator config plugins: %v", err)
	}
}

// getOperatorConfigPlugins retrieves the current plugins from the operator config.
func getOperatorConfigPlugins(t *testing.T, client *framework.ClientSet) []string {
	var plugins []string
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		config, err := client.Operator.Consoles().Get(context.TODO(), consoleapi.ConfigResourceName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("could not get operator config: %v", err)
		}
		plugins = config.Spec.Plugins
		return nil
	})
	if err != nil {
		t.Fatalf("could not retrieve operator config plugins: %v", err)
	}
	return plugins
}

// getConsoleConfig unmarshals and returns the console configuration.
func getConsoleConfig(t *testing.T, client *framework.ClientSet) consoleserver.Config {
	cm, err := framework.GetConsoleConfigMap(client)
	if err != nil {
		t.Fatalf("could not retrieve console config map: %v", err)
	}
	var consoleConfig consoleserver.Config
	if err := yaml.Unmarshal([]byte(cm.Data["console-config.yaml"]), &consoleConfig); err != nil {
		t.Fatalf("could not unmarshal console config: %v", err)
	}
	return consoleConfig
}

// getPlugin constructs a ConsolePlugin resource.
func getPlugin(name string, csp []consolev1.ConsolePluginCSP) *consolev1.ConsolePlugin {
	return &consolev1.ConsolePlugin{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: consolev1.ConsolePluginSpec{
			DisplayName:           name,
			ContentSecurityPolicy: csp,
			I18n:                  consolev1.ConsolePluginI18n{LoadType: consolev1.Preload},
			Backend: consolev1.ConsolePluginBackend{
				Type: consolev1.Service,
				Service: &consolev1.ConsolePluginService{
					Name: "test-plugin-service-name", Namespace: "test-plugin-service-namespace",
					Port: 8443, BasePath: "/manifest",
				},
			},
		},
	}
}

// createConsolePlugin creates a ConsolePlugin resource in the cluster.
func createConsolePlugin(t *testing.T, client *framework.ClientSet, plugin *consolev1.ConsolePlugin) {
	if _, err := client.ConsolePluginV1.Create(context.TODO(), plugin, metav1.CreateOptions{}); err != nil {
		t.Fatalf("could not create ConsolePlugin: %v", err)
	}
}

// deleteConsolePlugin removes a ConsolePlugin resource from the cluster.
func deleteConsolePlugin(t *testing.T, client *framework.ClientSet, name string) {
	if err := client.ConsolePluginV1.Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("could not delete ConsolePlugin: %v", err)
	}
}
