package plugins

import (
	"context"
	"encoding/json"
	"fmt"

	// k8s
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	// openshift
	operatorv1 "github.com/openshift/api/operator/v1"
	consoleinformersv1alpha1 "github.com/openshift/client-go/console/informers/externalversions/console/v1alpha1"
	listerv1alpha1 "github.com/openshift/client-go/console/listers/console/v1alpha1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	// console-operator
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
)

const (
	migratedPluginsAnnotation = "console.openshift.io/migrated-plugins"
)

type PluginsMigrationController struct {
	// configs
	operatorClient       v1helpers.OperatorClient
	operatorConfigClient operatorclientv1.ConsoleInterface
	// lister
	consolePluginLister   listerv1alpha1.ConsolePluginLister
	consolePluginInformer consoleinformersv1alpha1.ConsolePluginInformer
}

func NewPluginsMigrationController(
	// operator
	operatorClient v1helpers.OperatorClient,
	operatorConfigClient operatorclientv1.ConsoleInterface,
	// operatorConfigInformer operatorinformerv1.ConsoleInformer,
	// plugins
	consolePluginInformer consoleinformersv1alpha1.ConsolePluginInformer,
	// events
	recorder events.Recorder,
) factory.Controller {

	c := &PluginsMigrationController{
		// configs
		operatorClient:       operatorClient,
		operatorConfigClient: operatorConfigClient,
		// plugins
		consolePluginLister: consolePluginInformer.Lister(),
	}

	return factory.New().
		WithInformers(
			consolePluginInformer.Informer(),
		).WithSync(c.Sync).
		ToController("PluginsMigrationController", recorder.WithComponentSuffix("plugins-migration-controller"))
}

func (c *PluginsMigrationController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {

	operatorConfig, err := c.operatorConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	switch operatorConfig.Spec.ManagementState {
	case operatorv1.Managed:
		klog.V(4).Infof("console-operator is in a managed state: migrating available plugins")
	case operatorv1.Unmanaged:
		klog.V(4).Infof("console-operator is in an unmanaged state: skipping available plugins migration")
		return nil
	case operatorv1.Removed:
		klog.V(4).Infof("console-operator is in a removed state: removing migrated plugins")
		return c.removeMigratedPlugins(ctx, operatorConfig)
	default:
		return fmt.Errorf("unknown state: %v", operatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	availablePluginsName, err := c.GetAvailablePluginsName()
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("AvailablePluginMigration", "FailedAvailablePluginsGet", err))
	if err != nil {
		return statusHandler.FlushAndReturn(err)
	}
	migratedPluginsArray, err := GetMigratedPlugins(operatorConfig)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("AvailablePluginMigration", "FailedMigratedPluginsGet", err))
	if err != nil {
		return statusHandler.FlushAndReturn(err)
	}

	pluginsToEnable := GetPluginsToEnable(operatorConfig, availablePluginsName, migratedPluginsArray)

	if len(pluginsToEnable) == 0 {
		return statusHandler.FlushAndReturn(nil)
	}

	err = c.UpdateOperatorConfig(ctx, operatorConfig, pluginsToEnable, migratedPluginsArray)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("AvailablePluginMigration", "FailedOperatorConfigUpdate", err))
	if err != nil {
		return statusHandler.FlushAndReturn(err)
	}

	return statusHandler.FlushAndReturn(err)
}

func (c *PluginsMigrationController) GetAvailablePluginsName() ([]string, error) {
	availablePlugins, err := c.consolePluginLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var availablePluginsName []string
	for _, plugin := range availablePlugins {
		availablePluginsName = append(availablePluginsName, plugin.Name)
	}
	return availablePluginsName, nil
}

func GetMigratedPlugins(operatorConfig *operatorv1.Console) ([]string, error) {
	migratedPluginsAnnotation, migratedPluginsAnnotationExists := operatorConfig.Annotations[migratedPluginsAnnotation]
	migratedPluginsArray := []string{}
	if migratedPluginsAnnotationExists {
		err := json.Unmarshal([]byte(migratedPluginsAnnotation), &migratedPluginsArray)
		if err != nil {
			return nil, err
		}
	}

	return migratedPluginsArray, nil
}

func GetPluginsToEnable(
	operatorConfig *operatorv1.Console,
	availablePluginsName []string,
	migratedPluginsArray []string,
) []string {
	var pluginsToEnable []string
	for _, availablePluginName := range availablePluginsName {
		if !util.ContainsString(migratedPluginsArray, availablePluginName) && !util.ContainsString(operatorConfig.Spec.Plugins, availablePluginName) {
			pluginsToEnable = append(pluginsToEnable, availablePluginName)
		}
	}

	return pluginsToEnable
}

func (c *PluginsMigrationController) UpdateOperatorConfig(
	ctx context.Context,
	operatorConfig *operatorv1.Console,
	pluginsToEnable []string,
	migratedPluginsArray []string,
) error {
	if len(pluginsToEnable) == 0 {
		return nil
	}

	updatedOperatorConfig := operatorConfig.DeepCopy()
	updatedOperatorConfig.Spec.Plugins = append(updatedOperatorConfig.Spec.Plugins, pluginsToEnable...)

	pluginsToAnnotate := append(migratedPluginsArray, pluginsToEnable...)
	marshaledPluginsToAnnotate, err := json.Marshal(pluginsToAnnotate)
	if err != nil {
		return err
	}
	updatedOperatorConfig.Annotations[migratedPluginsAnnotation] = string(marshaledPluginsToAnnotate)

	_, err = c.operatorConfigClient.Update(ctx, updatedOperatorConfig, metav1.UpdateOptions{})
	return err
}

func (c *PluginsMigrationController) removeMigratedPlugins(ctx context.Context, operatorConfig *operatorv1.Console) error {
	updatedOperatorConfig := operatorConfig.DeepCopy()
	updatedOperatorConfig.Spec.Plugins = []string{}
	marshaledPluginsToAnnotate, err := json.Marshal([]string{})
	if err != nil {
		return err
	}
	updatedOperatorConfig.Annotations[migratedPluginsAnnotation] = string(marshaledPluginsToAnnotate)
	_, err = c.operatorConfigClient.Update(ctx, updatedOperatorConfig, metav1.UpdateOptions{})
	return nil
}
