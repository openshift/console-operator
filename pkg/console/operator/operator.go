package operator

import (
	// standard lib
	"context"
	"fmt"
	"time"

	// kube
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	corev1 "k8s.io/client-go/informers/core/v1"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"

	// openshift

	clusterclientv1 "github.com/open-cluster-management/api/client/cluster/clientset/versioned/typed/cluster/v1"
	clusterinformersv1 "github.com/open-cluster-management/api/client/cluster/informers/externalversions/cluster/v1"
	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	consoleinformersv1alpha1 "github.com/openshift/client-go/console/informers/externalversions/console/v1alpha1"
	listerv1alpha1 "github.com/openshift/client-go/console/listers/console/v1alpha1"
	oauthclientv1 "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	oauthinformersv1 "github.com/openshift/client-go/oauth/informers/externalversions/oauth/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	operatorinformerv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	routeclientv1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	routesinformersv1 "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/status"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	appsinformersv1 "k8s.io/client-go/informers/apps/v1"

	// operator
	"github.com/openshift/console-operator/pkg/console/subresource/configmap"
	"github.com/openshift/console-operator/pkg/console/subresource/deployment"
	"github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	"github.com/openshift/console-operator/pkg/console/subresource/secret"
)

type consoleOperator struct {
	// configs
	operatorClient             v1helpers.OperatorClient
	operatorConfigClient       operatorclientv1.ConsoleInterface
	consoleConfigClient        configclientv1.ConsoleInterface
	infrastructureConfigClient configclientv1.InfrastructureInterface
	ingressConfigClient        configclientv1.IngressInterface
	proxyConfigClient          configclientv1.ProxyInterface
	oauthConfigClient          configclientv1.OAuthInterface
	// core kube
	secretsClient    coreclientv1.SecretsGetter
	configMapClient  coreclientv1.ConfigMapsGetter
	serviceClient    coreclientv1.ServicesGetter
	deploymentClient appsclientv1.DeploymentsGetter
	// openshift
	routeClient          routeclientv1.RoutesGetter
	oauthClient          oauthclientv1.OAuthClientsGetter
	managedClusterClient clusterclientv1.ManagedClustersGetter
	dynamicClient        dynamic.Interface
	versionGetter        status.VersionGetter
	// lister
	consolePluginLister listerv1alpha1.ConsolePluginLister

	resourceSyncer resourcesynccontroller.ResourceSyncer
}

func NewConsoleOperator(
	// top level config
	configClient configclientv1.ConfigV1Interface,
	configInformer configinformer.SharedInformerFactory,
	// operator
	operatorClient v1helpers.OperatorClient,
	operatorConfigClient operatorclientv1.OperatorV1Interface,
	operatorConfigInformer operatorinformerv1.ConsoleInformer,
	// core resources
	corev1Client coreclientv1.CoreV1Interface,
	coreV1 corev1.Interface,
	// deployments
	deploymentClient appsclientv1.DeploymentsGetter,
	deploymentInformer appsinformersv1.DeploymentInformer,
	// routes
	routev1Client routeclientv1.RoutesGetter,
	routeInformer routesinformersv1.RouteInformer,
	// oauth
	oauthv1Client oauthclientv1.OAuthClientsGetter,
	oauthClients oauthinformersv1.OAuthClientInformer,
	// plugins
	consolePluginInformer consoleinformersv1alpha1.ConsolePluginInformer,
	// openshift managed
	managedCoreV1 corev1.Interface,
	// ManagedClusters
	managedClusterClient clusterclientv1.ManagedClustersGetter,
	managedClusterInformers clusterinformersv1.ManagedClusterInformer,
	dynamicClient dynamic.Interface,
	// event handling
	versionGetter status.VersionGetter,
	recorder events.Recorder,
	resourceSyncer resourcesynccontroller.ResourceSyncer,
) factory.Controller {
	c := &consoleOperator{
		// configs
		operatorClient:             operatorClient,
		operatorConfigClient:       operatorConfigClient.Consoles(),
		consoleConfigClient:        configClient.Consoles(),
		infrastructureConfigClient: configClient.Infrastructures(),
		ingressConfigClient:        configClient.Ingresses(),
		proxyConfigClient:          configClient.Proxies(),
		oauthConfigClient:          configClient.OAuths(),
		// console resources
		// core kube
		secretsClient:    corev1Client,
		configMapClient:  corev1Client,
		serviceClient:    corev1Client,
		deploymentClient: deploymentClient,
		// openshift
		routeClient:          routev1Client,
		oauthClient:          oauthv1Client,
		managedClusterClient: managedClusterClient,
		dynamicClient:        dynamicClient,
		versionGetter:        versionGetter,
		// plugins
		consolePluginLister: consolePluginInformer.Lister(),

		resourceSyncer: resourceSyncer,
	}

	secretsInformer := coreV1.Secrets()
	configMapInformer := coreV1.ConfigMaps()
	managedConfigMapInformer := managedCoreV1.ConfigMaps()
	serviceInformer := coreV1.Services()
	configV1Informers := configInformer.Config().V1()
	configNameFilter := util.NamesFilter(api.ConfigResourceName)
	targetNameFilter := util.NamesFilter(api.OpenShiftConsoleName)

	return factory.New().
		WithFilteredEventsInformers( // configs
			configNameFilter,
			configV1Informers.Consoles().Informer(),
			operatorConfigInformer.Informer(),
			configV1Informers.Infrastructures().Informer(),
			configV1Informers.Ingresses().Informer(),
			configV1Informers.Proxies().Informer(),
			configV1Informers.OAuths().Informer(),
		).WithFilteredEventsInformers( // console resources
		targetNameFilter,
		deploymentInformer.Informer(),
		routeInformer.Informer(),
		serviceInformer.Informer(),
		oauthClients.Informer(),
	).WithInformers(
		consolePluginInformer.Informer(),
	).WithFilteredEventsInformers(
		util.NamesFilter(api.OpenShiftConsoleConfigMapName, api.ServiceCAConfigMapName, api.OpenShiftCustomLogoConfigMapName, api.TrustedCAConfigMapName),
		configMapInformer.Informer(),
	).WithFilteredEventsInformers(
		util.NamesFilter(api.OpenShiftConsoleConfigMapName, api.OpenShiftConsolePublicConfigMapName),
		managedConfigMapInformer.Informer(),
	).WithFilteredEventsInformers(
		util.NamesFilter(deployment.ConsoleOauthConfigName),
		secretsInformer.Informer(),
	).WithFilteredEventsInformers(
		util.ExcludeName(api.HubClusterName),
		managedClusterInformers.Informer(),
	).WithSync(c.Sync).
		ToController("ConsoleOperator", recorder.WithComponentSuffix("console-operator"))
}

type configSet struct {
	Console        *configv1.Console
	Operator       *operatorsv1.Console
	Infrastructure *configv1.Infrastructure
	Proxy          *configv1.Proxy
	OAuth          *configv1.OAuth
	Ingress        *configv1.Ingress
}

func (c *consoleOperator) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.operatorConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		klog.Error("failed to retrieve operator config: %v", err)
		return err
	}

	startTime := time.Now()
	klog.V(4).Infof("started syncing operator %q (%v)", operatorConfig.Name, startTime)
	defer klog.V(4).Infof("finished syncing operator %q (%v)", operatorConfig.Name, time.Since(startTime))

	// ensure we have top level console config
	consoleConfig, err := c.consoleConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("console config error: %v", err)
		return err
	}

	// we need infrastructure config for apiServerURL
	infrastructureConfig, err := c.infrastructureConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("infrastructure config error: %v", err)
		return err
	}

	proxyConfig, err := c.proxyConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("proxy config error: %v", err)
		return err
	}

	oauthConfig, err := c.oauthConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("oauth config error: %v", err)
		return err
	}

	ingressConfig, err := c.ingressConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("ingress config error: %v", err)
		return err
	}

	configs := configSet{
		Console:        consoleConfig,
		Operator:       operatorConfig,
		Infrastructure: infrastructureConfig,
		Proxy:          proxyConfig,
		OAuth:          oauthConfig,
		Ingress:        ingressConfig,
	}

	if err := c.handleSync(ctx, controllerContext, configs); err != nil {
		return err
	}

	return nil
}

func (c *consoleOperator) handleSync(ctx context.Context, controllerContext factory.SyncContext, configs configSet) error {
	updatedStatus := configs.Operator.DeepCopy()

	switch updatedStatus.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infoln("console is in a managed state.")
		// handled below
	case operatorsv1.Unmanaged:
		klog.V(4).Infoln("console is in an unmanaged state.")
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infoln("console has been removed.")
		return c.removeConsole(ctx, controllerContext.Recorder())
	default:
		return fmt.Errorf("console is in an unknown state: %v", updatedStatus.Spec.ManagementState)
	}

	return c.sync_v400(ctx, controllerContext, updatedStatus, configs)
}

// this may need to move to sync_v400 if versions ever have custom delete logic
func (c *consoleOperator) removeConsole(ctx context.Context, recorder events.Recorder) error {
	klog.V(2).Info("deleting console resources")
	defer klog.V(2).Info("finished deleting console resources")
	var errs []error
	// configmaps
	errs = append(errs, c.configMapClient.ConfigMaps(api.TargetNamespace).Delete(ctx, configmap.Stub().Name, metav1.DeleteOptions{}))
	errs = append(errs, c.configMapClient.ConfigMaps(api.TargetNamespace).Delete(ctx, configmap.ServiceCAStub().Name, metav1.DeleteOptions{}))
	// secret
	errs = append(errs, c.secretsClient.Secrets(api.TargetNamespace).Delete(ctx, secret.Stub().Name, metav1.DeleteOptions{}))
	// existingOAuthClient is not a delete, it is a deregister/neutralize
	existingOAuthClient, getAuthErr := c.oauthClient.OAuthClients().Get(ctx, oauthclient.Stub().Name, metav1.GetOptions{})
	errs = append(errs, getAuthErr)
	if len(existingOAuthClient.RedirectURIs) != 0 {
		_, updateAuthErr := c.oauthClient.OAuthClients().Update(ctx, oauthclient.DeRegisterConsoleFromOAuthClient(existingOAuthClient), metav1.UpdateOptions{})
		errs = append(errs, updateAuthErr)
	}
	// deployment
	// NOTE: CVO controls the deployment for downloads, console-operator cannot delete it.
	errs = append(errs, c.deploymentClient.Deployments(api.TargetNamespace).Delete(ctx, deployment.Stub().Name, metav1.DeleteOptions{}))
	// clear the console URL from the public config map in openshift-config-managed
	_, _, updateConfigErr := resourceapply.ApplyConfigMap(c.configMapClient, recorder, configmap.EmptyPublicConfig())
	errs = append(errs, updateConfigErr)

	return utilerrors.FilterOut(utilerrors.NewAggregate(errs), errors.IsNotFound)
}
