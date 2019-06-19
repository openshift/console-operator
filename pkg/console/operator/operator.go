package operator

import (
	// standard lib
	"fmt"
	"reflect"
	"time"

	// kube
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1 "k8s.io/client-go/informers/core/v1"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	oauthclientv1 "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	oauthinformersv1 "github.com/openshift/client-go/oauth/informers/externalversions/oauth/v1"
	routeclientv1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/openshift/console-operator/pkg/api"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/status"

	"monis.app/go/openshift/operator"

	// informers
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	operatorinformerv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"

	routesinformersv1 "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	appsinformersv1 "k8s.io/client-go/informers/apps/v1"

	// clients
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"

	// operator
	customerrors "github.com/openshift/console-operator/pkg/console/errors"
	"github.com/openshift/console-operator/pkg/console/subresource/configmap"
	"github.com/openshift/console-operator/pkg/console/subresource/deployment"
	"github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	"github.com/openshift/console-operator/pkg/console/subresource/route"
	"github.com/openshift/console-operator/pkg/console/subresource/secret"
	"github.com/openshift/console-operator/pkg/console/subresource/service"
)

const (
	controllerName = "Console"
)

type consoleOperator struct {
	operatorConfigClient operatorclientv1.ConsoleInterface
	consoleConfigClient  configclientv1.ConsoleInterface
	// core kube
	secretsClient    coreclientv1.SecretsGetter
	configMapClient  coreclientv1.ConfigMapsGetter
	serviceClient    coreclientv1.ServicesGetter
	deploymentClient appsv1.DeploymentsGetter
	// openshift
	routeClient                routeclientv1.RoutesGetter
	oauthClient                oauthclientv1.OAuthClientsGetter
	infrastructureConfigClient configclientv1.InfrastructureInterface
	versionGetter              status.VersionGetter
	// recorder
	recorder       events.Recorder
	resourceSyncer resourcesynccontroller.ResourceSyncer
}

func NewConsoleOperator(
	// informers
	operatorConfigInformer operatorinformerv1.ConsoleInformer,
	configInformer configinformer.SharedInformerFactory,

	coreV1 corev1.Interface,
	managedCoreV1 corev1.Interface,
	deployments appsinformersv1.DeploymentInformer,
	routes routesinformersv1.RouteInformer,
	oauthClients oauthinformersv1.OAuthClientInformer,

	// clients
	operatorConfigClient operatorclientv1.OperatorV1Interface,
	configClient configclientv1.ConfigV1Interface,
	corev1Client coreclientv1.CoreV1Interface,
	deploymentClient appsv1.DeploymentsGetter,
	routev1Client routeclientv1.RoutesGetter,
	oauthv1Client oauthclientv1.OAuthClientsGetter,
	versionGetter status.VersionGetter,
	// recorder
	recorder events.Recorder,
	resourceSyncer resourcesynccontroller.ResourceSyncer,
) operator.Runner {
	c := &consoleOperator{
		// configs
		operatorConfigClient:       operatorConfigClient.Consoles(),
		consoleConfigClient:        configClient.Consoles(),
		infrastructureConfigClient: configClient.Infrastructures(),
		// console resources
		// core kube
		secretsClient:    corev1Client,
		configMapClient:  corev1Client,
		serviceClient:    corev1Client,
		deploymentClient: deploymentClient,
		// openshift
		routeClient:   routev1Client,
		oauthClient:   oauthv1Client,
		versionGetter: versionGetter,
		// recorder
		recorder:       recorder,
		resourceSyncer: resourceSyncer,
	}

	secretsInformer := coreV1.Secrets()
	configMapInformer := coreV1.ConfigMaps()
	managedConfigMapInformer := managedCoreV1.ConfigMaps()
	serviceInformer := coreV1.Services()
	configV1Informers := configInformer.Config().V1()

	configNameFilter := operator.FilterByNames(api.ConfigResourceName)
	targetNameFilter := operator.FilterByNames(api.OpenShiftConsoleName)

	return operator.New(controllerName, c,
		// configs
		operator.WithInformer(configV1Informers.Consoles(), configNameFilter),
		operator.WithInformer(operatorConfigInformer, configNameFilter),
		operator.WithInformer(configV1Informers.Infrastructures(), configNameFilter),
		// console resources
		operator.WithInformer(deployments, targetNameFilter),
		operator.WithInformer(routes, targetNameFilter),
		operator.WithInformer(serviceInformer, targetNameFilter),
		operator.WithInformer(oauthClients, targetNameFilter),
		// special resources with unique names
		operator.WithInformer(configMapInformer, operator.FilterByNames(api.OpenShiftConsoleConfigMapName, api.ServiceCAConfigMapName, api.OpenShiftCustomLogoConfigMap)),
		operator.WithInformer(managedConfigMapInformer, operator.FilterByNames(api.OpenShiftConsoleConfigMapName, api.OpenShiftConsolePublicConfigMapName)),
		operator.WithInformer(secretsInformer, operator.FilterByNames(deployment.ConsoleOauthConfigName)),
	)
}

// key is actually the pivot point for the operator, which is our Console custom resource
func (c *consoleOperator) Key() (metav1.Object, error) {
	return c.operatorConfigClient.Get(api.ConfigResourceName, metav1.GetOptions{})
}

func (c *consoleOperator) Sync(obj metav1.Object) error {
	startTime := time.Now()
	klog.V(4).Infof("started syncing operator %q (%v)", obj.GetName(), startTime)
	defer klog.V(4).Infof("finished syncing operator %q (%v)", obj.GetName(), time.Since(startTime))

	// we need to cast the operator config
	operatorConfig := obj.(*operatorsv1.Console)

	// ensure we have top level console config
	consoleConfig, err := c.consoleConfigClient.Get(api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("console config error: %v", err)
		return err
	}

	// we need infrastructure config for apiServerURL
	infrastructureConfig, err := c.infrastructureConfigClient.Get(api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("infrastructure config error: %v", err)
		return err
	}

	if err := c.handleSync(operatorConfig, consoleConfig, infrastructureConfig); err != nil {
		return err
	}

	return nil
}

func (c *consoleOperator) handleSync(operatorConfig *operatorsv1.Console, consoleConfig *configv1.Console, infrastructureConfig *configv1.Infrastructure) error {
	operatorConfigCopy := operatorConfig.DeepCopy()

	switch operatorConfigCopy.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infoln("console is in a managed state.")
		// handled below
	case operatorsv1.Unmanaged:
		klog.V(4).Infoln("console is in an unmanaged state.")
		c.ConditionsManagementStateUnmanaged(operatorConfigCopy)
		if !reflect.DeepEqual(operatorConfigCopy, operatorConfig) {
			c.SyncStatus(operatorConfigCopy)
		}
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infoln("console has been removed.")
		c.ConditionsManagementStateRemoved(operatorConfigCopy)
		if !reflect.DeepEqual(operatorConfigCopy, operatorConfig) {
			c.SyncStatus(operatorConfigCopy)
		}
		return c.removeConsole(operatorConfigCopy)
	default:
		c.ConditionsManagementStateInvalid(operatorConfigCopy)
		if !reflect.DeepEqual(operatorConfigCopy, operatorConfig) {
			c.SyncStatus(operatorConfigCopy)
		}
		return fmt.Errorf("console is in an unknown state: %v", operatorConfigCopy.Spec.ManagementState)
	}

	// we can default to not failing, and wait to see if sync returns an error
	c.ConditionNotDegraded(operatorConfigCopy)
	err := sync_v400(c, operatorConfigCopy, consoleConfig, infrastructureConfig)
	if err != nil {
		if !customerrors.IsSyncError(err) {
			c.SyncStatus(c.ConditionResourceSyncDegraded(operatorConfigCopy, err.Error()))
			return err
		} else {
			c.SyncStatus(operatorConfigCopy)
			return nil
		}
	}
	// finally write out the set of conditions currently set if anything has changed
	// to avoid a hot loop
	if !reflect.DeepEqual(operatorConfigCopy, operatorConfig) {
		c.SyncStatus(operatorConfigCopy)
	}
	return nil
}

// this may need to move to sync_v400 if versions ever have custom delete logic
func (c *consoleOperator) removeConsole(cr *operatorsv1.Console) error {
	klog.V(2).Info("deleting console resources")
	defer klog.V(2).Info("finished deleting console resources")
	var errs []error
	// service
	errs = append(errs, c.serviceClient.Services(api.TargetNamespace).Delete(service.Stub().Name, &metav1.DeleteOptions{}))
	// route
	errs = append(errs, c.routeClient.Routes(api.TargetNamespace).Delete(route.Stub().Name, &metav1.DeleteOptions{}))
	// configmaps
	errs = append(errs, c.configMapClient.ConfigMaps(api.TargetNamespace).Delete(configmap.Stub().Name, &metav1.DeleteOptions{}))
	errs = append(errs, c.configMapClient.ConfigMaps(api.TargetNamespace).Delete(configmap.ServiceCAStub().Name, &metav1.DeleteOptions{}))
	// secret
	errs = append(errs, c.secretsClient.Secrets(api.TargetNamespace).Delete(secret.Stub().Name, &metav1.DeleteOptions{}))
	// existingOAuthClient is not a delete, it is a deregister/neutralize
	existingOAuthClient, getAuthErr := c.oauthClient.OAuthClients().Get(oauthclient.Stub().Name, metav1.GetOptions{})
	errs = append(errs, getAuthErr)
	if len(existingOAuthClient.RedirectURIs) != 0 {
		_, updateAuthErr := c.oauthClient.OAuthClients().Update(oauthclient.DeRegisterConsoleFromOAuthClient(existingOAuthClient))
		errs = append(errs, updateAuthErr)
	}
	// deployment
	// NOTE: CVO controls the deployment for downloads, console-operator cannot delete it.
	errs = append(errs, c.deploymentClient.Deployments(api.TargetNamespace).Delete(deployment.Stub().Name, &metav1.DeleteOptions{}))
	// clear the console URL from the public config map in openshift-config-managed
	_, _, updateConfigErr := resourceapply.ApplyConfigMap(c.configMapClient, c.recorder, configmap.EmptyPublicConfig())
	errs = append(errs, updateConfigErr)

	return utilerrors.FilterOut(utilerrors.NewAggregate(errs), errors.IsNotFound)
}

func (c *consoleOperator) SyncCustomLogoConfigMap(operatorConfig *operatorsv1.Console) error {
	c.CheckCustomLogoImageStatus(operatorConfig)
	logoName := operatorConfig.Spec.Customization.CustomLogoFile.Name
	// add syncing for the custom logo config map
	if logoName != "" {
		return c.resourceSyncer.SyncConfigMap(
			resourcesynccontroller.ResourceLocation{Namespace: api.OpenShiftConsoleNamespace, Name: api.OpenShiftCustomLogoConfigMap},
			resourcesynccontroller.ResourceLocation{Namespace: api.OpenShiftConfigNamespace, Name: logoName},
		)
	}
	return nil
}

func (c *consoleOperator) CheckCustomLogoImageStatus(operatorConfig *operatorsv1.Console) {
	logo := operatorConfig.Spec.Customization.CustomLogoFile
	if logo.Name != "" && logo.Key != "" {
		//Check that configmap for custom logo exists
		logoConfigMap, err := c.configMapClient.ConfigMaps(api.OpenShiftConfigNamespace).Get(logo.Name, metav1.GetOptions{})
		if err != nil {
			// Set Operator Status to CustomLogoInvalid
			c.SetStatusCondition(operatorConfig, operatorsv1.OperatorStatusTypeDegraded, operatorsv1.ConditionTrue, reasonCustomLogoInvalid, "custom logo config map does not exist or has error")
			klog.Errorf("customLogo configmap not valid, %v", err)
		} else {
			if logoConfigMap.BinaryData[logo.Key] == nil {
				//Key does not exist so activate status condition
				c.SetStatusCondition(operatorConfig, operatorsv1.OperatorStatusTypeDegraded, operatorsv1.ConditionTrue, reasonCustomLogoInvalid, "custom logo file does not exist ")
				klog.Errorf("customLogo key is null")
			}
		}
	}
}
