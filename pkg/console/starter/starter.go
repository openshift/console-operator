package starter

import (
	"context"
	"fmt"
	"os"
	"time"

	// kube
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/api/oauth"
	operatorv1 "github.com/openshift/api/operator"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/clidownloads"
	"github.com/openshift/console-operator/pkg/console/controllers/downloadsdeployment"
	"github.com/openshift/console-operator/pkg/console/controllers/healthcheck"
	"github.com/openshift/console-operator/pkg/console/controllers/route"
	"github.com/openshift/console-operator/pkg/console/controllers/service"
	"github.com/openshift/console-operator/pkg/console/operatorclient"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/status"
	"github.com/openshift/library-go/pkg/operator/unsupportedconfigoverridescontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	// clients
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"

	authclient "github.com/openshift/client-go/oauth/clientset/versioned"
	oauthinformers "github.com/openshift/client-go/oauth/informers/externalversions"

	operatorversionedclient "github.com/openshift/client-go/operator/clientset/versioned"
	operatorinformers "github.com/openshift/client-go/operator/informers/externalversions"

	routesclient "github.com/openshift/client-go/route/clientset/versioned"
	routesinformers "github.com/openshift/client-go/route/informers/externalversions"

	consolev1client "github.com/openshift/client-go/console/clientset/versioned"
	// consolev1client "github.com/openshift/client-go/console/clientset/versioned/typed/console/v1"
	consoleinformers "github.com/openshift/client-go/console/informers/externalversions"

	"github.com/openshift/console-operator/pkg/console/clientwrapper"
	"github.com/openshift/console-operator/pkg/console/operator"
	"github.com/openshift/library-go/pkg/operator/loglevel"
)

func RunOperator(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {

	kubeClient, err := kubernetes.NewForConfig(controllerContext.ProtoKubeConfig)
	if err != nil {
		return err
	}

	configClient, err := configclient.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	operatorConfigClient, err := operatorversionedclient.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	consoleClient, err := consolev1client.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	routesClient, err := routesclient.NewForConfig(controllerContext.ProtoKubeConfig)
	if err != nil {
		return err
	}

	oauthClient, err := authclient.NewForConfig(controllerContext.ProtoKubeConfig)
	if err != nil {
		return err
	}

	const resync = 10 * time.Minute

	tweakListOptionsForOAuth := func(options *metav1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("metadata.name", api.OAuthClientName).String()
	}

	kubeInformers := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resync,
	);

	kubeInformersConsoleNamespaced := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resync,
		informers.WithNamespace(api.OpenShiftConsoleNamespace),
	)

	kubeInformersManagedNamespaced := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resync,
		informers.WithNamespace(api.OpenShiftConfigManagedNamespace),
	)

	kubeInformersConfigNamespaced := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resync,
		informers.WithNamespace(api.OpenShiftConfigNamespace),
	)

	//configs are all named "cluster", but our clusteroperator is named "console"
	configInformers := configinformers.NewSharedInformerFactoryWithOptions(
		configClient,
		resync,
	)

	operatorConfigInformers := operatorinformers.NewSharedInformerFactoryWithOptions(
		operatorConfigClient,
		resync,
	)

	routesInformersNamespaced := routesinformers.NewSharedInformerFactoryWithOptions(
		routesClient,
		resync,
		routesinformers.WithNamespace(api.TargetNamespace),
	)

	// oauthclients are not namespaced
	oauthInformers := oauthinformers.NewSharedInformerFactoryWithOptions(
		oauthClient,
		resync,
		oauthinformers.WithTweakListOptions(tweakListOptionsForOAuth),
	)

	consoleInformers := consoleinformers.NewSharedInformerFactory(
		consoleClient,
		resync,
	)

	operatorClient := &operatorclient.OperatorClient{
		Informers: operatorConfigInformers,
		Client:    operatorConfigClient.OperatorV1(),
		Context:   ctx,
	}

	recorder := controllerContext.EventRecorder

	versionGetter := status.NewVersionGetter()

	resourceSyncerInformers, resourceSyncer := getResourceSyncer(controllerContext, clientwrapper.WithoutSecret(kubeClient), operatorClient)

	err = startStaticResourceSyncing(resourceSyncer)
	if err != nil {
		return err
	}

	// TODO: rearrange these into informer,client pairs, NOT separated.
	consoleOperator := operator.NewConsoleOperator(
		// top level config
		configClient.ConfigV1(),
		configInformers,
		// operator
		operatorClient,
		operatorConfigClient.OperatorV1(),
		operatorConfigInformers.Operator().V1().Consoles(), // OperatorConfig

		// core resources
		kubeClient.CoreV1(),                        // Namespaces, Secrets, ConfigMaps, Service
		kubeInformersConsoleNamespaced.Core().V1(), // Secrets, ConfigMaps, Service
		kubeInformers.Core().V1().Namespaces(),     // Namespaces
		// deployments
		kubeClient.AppsV1(),
		kubeInformersConsoleNamespaced.Apps().V1().Deployments(), // Deployments
		// routes
		routesClient.RouteV1(),
		routesInformersNamespaced.Route().V1().Routes(), // Route
		// oauth
		oauthClient.OauthV1(),
		oauthInformers.Oauth().V1().OAuthClients(), // OAuth clients
		// plugins
		consoleInformers.Console().V1alpha1().ConsolePlugins(),
		// openshift managed
		kubeInformersManagedNamespaced.Core().V1(), // Managed ConfigMaps
		// event handling
		versionGetter,
		recorder,
		resourceSyncer,
	)

	downloadsDeploymentController := downloadsdeployment.NewDownloadsDeploymentSyncController(
		// top level config
		configClient.ConfigV1(),
		// clients
		operatorClient,
		operatorConfigClient.OperatorV1().Consoles(),
		configInformers,
		// operator
		operatorConfigInformers.Operator().V1().Consoles(),

		kubeClient.AppsV1(), // Deployments
		kubeInformersConsoleNamespaced.Apps().V1().Deployments(), // Deployments
		recorder,
		resourceSyncer,
	)

	cliDownloadsController := clidownloads.NewCLIDownloadsSyncController(
		// clients
		operatorClient,
		operatorConfigClient.OperatorV1(),
		consoleClient.ConsoleV1().ConsoleCLIDownloads(),
		routesClient.RouteV1(),
		// informers
		operatorConfigInformers.Operator().V1().Consoles(),    // OperatorConfig
		consoleInformers.Console().V1().ConsoleCLIDownloads(), // ConsoleCliDownloads
		routesInformersNamespaced.Route().V1().Routes(),       // Routes
		// events
		recorder,
		resourceSyncer,
	)

	consoleServiceController := service.NewServiceSyncController(
		api.OpenShiftConsoleServiceName,
		// top level config
		configClient.ConfigV1(),
		configInformers,
		// clients
		operatorClient,
		operatorConfigClient.OperatorV1().Consoles(), // operator config so we can update status
		kubeClient.CoreV1(),                          // only needs to interact with the service resource
		// informers
		operatorConfigInformers.Operator().V1().Consoles(), // OperatorConfig
		kubeInformersConsoleNamespaced.Core().V1().Services(),     // Services
		// events
		recorder,
		resourceSyncer,
	)

	downloadsServiceController := service.NewServiceSyncController(
		api.DownloadsResourceName,
		// top level config
		configClient.ConfigV1(),
		configInformers,
		operatorClient,
		// clients
		operatorConfigClient.OperatorV1().Consoles(), // operator config so we can update status
		kubeClient.CoreV1(),                          // only needs to interact with the service resource
		// informers
		operatorConfigInformers.Operator().V1().Consoles(), // OperatorConfig
		kubeInformersConsoleNamespaced.Core().V1().Services(),     // Services
		// events
		recorder,
		resourceSyncer,
	)

	consoleRouteController := route.NewRouteSyncController(
		api.OpenShiftConsoleRouteName,
		// enable health check for console route
		true,
		// top level config
		configClient.ConfigV1(),
		configInformers,
		// clients
		operatorClient,
		operatorConfigClient.OperatorV1().Consoles(),
		routesClient.RouteV1(),
		kubeClient.CoreV1(),
		kubeClient.CoreV1(),
		// route
		operatorConfigInformers.Operator().V1().Consoles(),
		kubeInformersConsoleNamespaced.Core().V1(),                 // `openshift-console` namespace informers
		kubeInformersConfigNamespaced.Core().V1().Secrets(), // `openshift-config` namespace informers
		routesInformersNamespaced.Route().V1().Routes(),
		// events
		recorder,
		resourceSyncer,
	)

	downloadsRouteController := route.NewRouteSyncController(
		api.OpenShiftConsoleDownloadsRouteName,
		// disable health check for console route
		false,
		// top level config
		configClient.ConfigV1(),
		configInformers,
		// clients
		operatorClient,
		operatorConfigClient.OperatorV1().Consoles(),
		routesClient.RouteV1(),
		kubeClient.CoreV1(),
		kubeClient.CoreV1(),
		// route
		operatorConfigInformers.Operator().V1().Consoles(),
		kubeInformersConsoleNamespaced.Core().V1(),                 // `openshift-console` namespace informers
		kubeInformersConfigNamespaced.Core().V1().Secrets(), // `openshift-config` namespace informers
		routesInformersNamespaced.Route().V1().Routes(),
		// events
		recorder,
		resourceSyncer,
	)

	consoleRouteHealthCheckController := healthcheck.NewHealthCheckController(
		// top level config
		configClient.ConfigV1(),
		// clients
		operatorClient,
		operatorConfigClient.OperatorV1().Consoles(),
		routesClient.RouteV1(),
		kubeClient.CoreV1(),
		// route
		operatorConfigInformers.Operator().V1().Consoles(),
		kubeInformersConsoleNamespaced.Core().V1(), // `openshift-console` namespace informers
		routesInformersNamespaced.Route().V1().Routes(),
		// events
		recorder,
		resourceSyncer,
	)

	versionRecorder := status.NewVersionGetter()
	versionRecorder.SetVersion("operator", os.Getenv("RELEASE_VERSION"))

	clusterOperatorStatus := status.NewClusterOperatorStatusController(
		api.ClusterOperatorName,
		[]configv1.ObjectReference{
			{Group: operatorv1.GroupName, Resource: "consoles", Name: api.ConfigResourceName},
			{Group: configv1.GroupName, Resource: "consoles", Name: api.ConfigResourceName},
			{Group: configv1.GroupName, Resource: "infrastructures", Name: api.ConfigResourceName},
			{Group: configv1.GroupName, Resource: "proxies", Name: api.ConfigResourceName},
			{Group: configv1.GroupName, Resource: "oauths", Name: api.ConfigResourceName},
			{Group: oauth.GroupName, Resource: "oauthclients", Name: api.OAuthClientName},
			{Group: corev1.GroupName, Resource: "namespaces", Name: api.OpenShiftConsoleOperatorNamespace},
			{Group: corev1.GroupName, Resource: "namespaces", Name: api.OpenShiftConsoleNamespace},
			{Group: corev1.GroupName, Resource: "configmaps", Name: api.OpenShiftConsolePublicConfigMapName, Namespace: api.OpenShiftConfigManagedNamespace},
		},
		// clusteroperator client
		configClient.ConfigV1(),
		// cluster operator informer
		configInformers.Config().V1().ClusterOperators(),
		// operator client
		operatorClient,
		versionRecorder,
		controllerContext.EventRecorder,
	)

	// NOTE: be sure to uncomment the .Run() below if using this
	//staleConditionsController := staleconditions.NewRemoveStaleConditions(
	//	[]string{
	//		// If a condition is removed, we need to add it here for at least
	//		// one release to ensure the operator does not permanently wedge.
	//		// Please do something like the following:
	//		//
	//		// example: in 4.x.x we removed FooDegraded condition and can remove
	//		// this in 4.x+1:
	//		// "FooDegraded",
	//	},
	//	operatorClient,
	//	controllerContext.EventRecorder,
	//)

	configUpgradeableController := unsupportedconfigoverridescontroller.NewUnsupportedConfigOverridesController(operatorClient, controllerContext.EventRecorder)
	logLevelController := loglevel.NewClusterOperatorLoggingController(operatorClient, controllerContext.EventRecorder)
	managementStateController := management.NewOperatorManagementStateController(api.ClusterOperatorName, operatorClient, controllerContext.EventRecorder)

	for _, informer := range []interface {
		Start(stopCh <-chan struct{})
	}{
		kubeInformers,
		kubeInformersConsoleNamespaced,
		kubeInformersConfigNamespaced,
		kubeInformersManagedNamespaced,
		resourceSyncerInformers,
		operatorConfigInformers,
		consoleInformers,
		configInformers,
		routesInformersNamespaced,
		oauthInformers,
	} {
		informer.Start(ctx.Done())
	}

	for _, controller := range []interface {
		Run(ctx context.Context, workers int)
	}{
		resourceSyncer,
		clusterOperatorStatus,
		logLevelController,
		managementStateController,
		configUpgradeableController,
		consoleServiceController,
		consoleRouteController,
		downloadsServiceController,
		downloadsRouteController,
		consoleOperator,
		cliDownloadsController,
		downloadsDeploymentController,
		consoleRouteHealthCheckController,
	} {
		go controller.Run(ctx, 1)
	}

	// go staleConditionsController.Run(1, ctx.Done())

	<-ctx.Done()
	return fmt.Errorf("stopped")
}

// startResourceSyncing should start syncing process of all secrets and configmaps that need to be synced.
func startStaticResourceSyncing(resourceSyncer *resourcesynccontroller.ResourceSyncController) error {
	// sync: 'default-ingress-cert' configmap
	// from: 'openshift-config-managed' namespace
	// to:   'openshift-console' namespace
	return resourceSyncer.SyncConfigMap(
		resourcesynccontroller.ResourceLocation{Name: api.DefaultIngressCertConfigMapName, Namespace: api.OpenShiftConsoleNamespace},
		resourcesynccontroller.ResourceLocation{Name: api.DefaultIngressCertConfigMapName, Namespace: api.OpenShiftConfigManagedNamespace},
	)
}

func getResourceSyncer(controllerContext *controllercmd.ControllerContext, kubeClient kubernetes.Interface, operatorClient v1helpers.OperatorClient) (v1helpers.KubeInformersForNamespaces, *resourcesynccontroller.ResourceSyncController) {
	resourceSyncerInformers := v1helpers.NewKubeInformersForNamespaces(
		kubeClient,
		api.OpenShiftConfigNamespace,
		api.OpenShiftConsoleNamespace,
		api.OpenShiftConfigManagedNamespace,
	)
	resourceSyncer := resourcesynccontroller.NewResourceSyncController(
		operatorClient,
		resourceSyncerInformers,
		v1helpers.CachedSecretGetter(kubeClient.CoreV1(), resourceSyncerInformers),
		v1helpers.CachedConfigMapGetter(kubeClient.CoreV1(), resourceSyncerInformers),
		controllerContext.EventRecorder,
	)
	return resourceSyncerInformers, resourceSyncer
}
