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
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1 "k8s.io/client-go/informers/core/v1"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

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
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	// informers
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	consoleinformersv1 "github.com/openshift/client-go/console/informers/externalversions/console/v1"
	operatorinformerv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"

	routesinformersv1 "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	appsinformersv1 "k8s.io/client-go/informers/apps/v1"

	// clients
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	consoleclientv1 "github.com/openshift/client-go/console/clientset/versioned/typed/console/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"

	// operator
	"github.com/openshift/console-operator/pkg/console/subresource/configmap"
	"github.com/openshift/console-operator/pkg/console/subresource/deployment"
	"github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	"github.com/openshift/console-operator/pkg/console/subresource/secret"
)

const (
	controllerName         = "Console"
	controllerWorkQueueKey = "console-sync--work-queue-key"
)

type consoleOperator struct {
	// configs
	operatorClient             v1helpers.OperatorClient
	operatorConfigClient       operatorclientv1.ConsoleInterface
	consoleConfigClient        configclientv1.ConsoleInterface
	infrastructureConfigClient configclientv1.InfrastructureInterface
	proxyConfigClient          configclientv1.ProxyInterface
	oauthConfigClient          configclientv1.OAuthInterface
	consolePluginClient        consoleclientv1.ConsolePluginInterface
	// core kube
	secretsClient    coreclientv1.SecretsGetter
	configMapClient  coreclientv1.ConfigMapsGetter
	serviceClient    coreclientv1.ServicesGetter
	deploymentClient appsv1.DeploymentsGetter
	// openshift
	routeClient   routeclientv1.RoutesGetter
	oauthClient   oauthclientv1.OAuthClientsGetter
	versionGetter status.VersionGetter
	// events
	cachesToSync   []cache.InformerSynced
	queue          workqueue.RateLimitingInterface
	recorder       events.Recorder
	resourceSyncer resourcesynccontroller.ResourceSyncer
	// context
	ctx context.Context
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
	deploymentClient appsv1.DeploymentsGetter,
	deployments appsinformersv1.DeploymentInformer,
	// routes
	routev1Client routeclientv1.RoutesGetter,
	routesInformer routesinformersv1.RouteInformer,
	// oauth
	oauthv1Client oauthclientv1.OAuthClientsGetter,
	oauthInformer oauthinformersv1.OAuthClientInformer,
	//plugins
	consolePluginClient consoleclientv1.ConsolePluginInterface,
	consolePluginInformer consoleinformersv1.ConsolePluginInformer,
	// openshift managed
	managedCoreV1 corev1.Interface,
	// event handling
	versionGetter status.VersionGetter,
	recorder events.Recorder,
	resourceSyncer resourcesynccontroller.ResourceSyncer,
	// context
	ctx context.Context,
) *consoleOperator {
	c := &consoleOperator{
		// configs
		operatorClient:             operatorClient,
		operatorConfigClient:       operatorConfigClient.Consoles(),
		consoleConfigClient:        configClient.Consoles(),
		infrastructureConfigClient: configClient.Infrastructures(),
		proxyConfigClient:          configClient.Proxies(),
		oauthConfigClient:          configClient.OAuths(),
		// console resources
		// core kube
		secretsClient:    corev1Client,
		configMapClient:  corev1Client,
		serviceClient:    corev1Client,
		deploymentClient: deploymentClient,
		// openshift
		routeClient:         routev1Client,
		oauthClient:         oauthv1Client,
		consolePluginClient: consolePluginClient,
		versionGetter:       versionGetter,
		// recorder
		recorder:       recorder,
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ConsoleCliDownloadsSyncer"),
		resourceSyncer: resourceSyncer,
		ctx:            ctx,
	}

	oauthInformer.Informer().AddEventHandler(c.newEventHandler())
	operatorClient.Informer().AddEventHandler(c.newEventHandler())
	operatorConfigInformer.Informer().AddEventHandler(c.newEventHandler())
	consolePluginInformer.Informer().AddEventHandler(c.newEventHandler())
	routesInformer.Informer().AddEventHandler(c.newEventHandler())

	c.cachesToSync = append(c.cachesToSync,
		oauthInformer.Informer().HasSynced,
		operatorClient.Informer().HasSynced,
		operatorConfigInformer.Informer().HasSynced,
		consolePluginInformer.Informer().HasSynced,
		routesInformer.Informer().HasSynced,
	)

	return c
}

type configSet struct {
	Console        *configv1.Console
	Operator       *operatorsv1.Console
	Infrastructure *configv1.Infrastructure
	Proxy          *configv1.Proxy
	OAuth          *configv1.OAuth
}

func (c *consoleOperator) sync() error {
	operatorConfig, err := c.operatorConfigClient.Get(c.ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("operator config error: %v", err)
		return err
	}

	consoleConfig, err := c.consoleConfigClient.Get(c.ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("console config error: %v", err)
		return err
	}

	// we need infrastructure config for apiServerURL
	infrastructureConfig, err := c.infrastructureConfigClient.Get(c.ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("infrastructure config error: %v", err)
		return err
	}

	proxyConfig, err := c.proxyConfigClient.Get(c.ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("proxy config error: %v", err)
		return err
	}

	oauthConfig, err := c.oauthConfigClient.Get(c.ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("oauth config error: %v", err)
		return err
	}

	configs := configSet{
		Console:        consoleConfig,
		Operator:       operatorConfig,
		Infrastructure: infrastructureConfig,
		Proxy:          proxyConfig,
		OAuth:          oauthConfig,
	}

	if err := c.handleSync(configs); err != nil {
		return err
	}

	return nil
}

func (c *consoleOperator) handleSync(configs configSet) error {
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
		return c.removeConsole()
	default:
		return fmt.Errorf("console is in an unknown state: %v", updatedStatus.Spec.ManagementState)
	}

	return c.sync_v400(updatedStatus, configs)
}

// this may need to move to sync_v400 if versions ever have custom delete logic
func (c *consoleOperator) removeConsole() error {
	klog.V(2).Info("deleting console resources")
	defer klog.V(2).Info("finished deleting console resources")
	var errs []error
	// configmaps
	errs = append(errs, c.configMapClient.ConfigMaps(api.TargetNamespace).Delete(c.ctx, configmap.Stub().Name, metav1.DeleteOptions{}))
	errs = append(errs, c.configMapClient.ConfigMaps(api.TargetNamespace).Delete(c.ctx, configmap.ServiceCAStub().Name, metav1.DeleteOptions{}))
	// secret
	errs = append(errs, c.secretsClient.Secrets(api.TargetNamespace).Delete(c.ctx, secret.Stub().Name, metav1.DeleteOptions{}))
	// existingOAuthClient is not a delete, it is a deregister/neutralize
	existingOAuthClient, getAuthErr := c.oauthClient.OAuthClients().Get(c.ctx, oauthclient.Stub().Name, metav1.GetOptions{})
	errs = append(errs, getAuthErr)
	if len(existingOAuthClient.RedirectURIs) != 0 {
		_, updateAuthErr := c.oauthClient.OAuthClients().Update(c.ctx, oauthclient.DeRegisterConsoleFromOAuthClient(existingOAuthClient), metav1.UpdateOptions{})
		errs = append(errs, updateAuthErr)
	}
	// deployment
	// NOTE: CVO controls the deployment for downloads, console-operator cannot delete it.
	errs = append(errs, c.deploymentClient.Deployments(api.TargetNamespace).Delete(c.ctx, deployment.Stub().Name, metav1.DeleteOptions{}))
	// clear the console URL from the public config map in openshift-config-managed
	_, _, updateConfigErr := resourceapply.ApplyConfigMap(c.configMapClient, c.recorder, configmap.EmptyPublicConfig())
	errs = append(errs, updateConfigErr)

	return utilerrors.FilterOut(utilerrors.NewAggregate(errs), errors.IsNotFound)
}

func (c *consoleOperator) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()
	klog.V(4).Infof("Starting %v", controllerName)
	defer klog.V(4).Infof("Shutting down %v", controllerName)
	if !cache.WaitForCacheSync(stopCh, c.cachesToSync...) {
		klog.Infoln("caches did not sync")
		runtime.HandleError(fmt.Errorf("caches did not sync"))
		return
	}
	// only start one worker
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *consoleOperator) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *consoleOperator) processNextWorkItem() bool {
	processKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(processKey)
	err := c.sync()
	if err == nil {
		c.queue.Forget(processKey)
		return true
	}
	runtime.HandleError(fmt.Errorf("%v failed with : %v", processKey, err))
	c.queue.AddRateLimited(processKey)
	return true
}

func (c *consoleOperator) newEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(controllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
	}
}
