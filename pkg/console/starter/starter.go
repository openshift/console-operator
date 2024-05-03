package starter

import (
	"context"
	"fmt"
	"os"
	"time"

	// kube
	corev1 "k8s.io/api/core/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiexensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	policyv1client "k8s.io/client-go/kubernetes/typed/policy/v1"
	"k8s.io/klog/v2"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/api/oauth"
	operatorv1 "github.com/openshift/api/operator"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/clientwrapper"
	"github.com/openshift/console-operator/pkg/console/controllers/clidownloads"
	"github.com/openshift/console-operator/pkg/console/controllers/clioidcclientstatus"
	"github.com/openshift/console-operator/pkg/console/controllers/downloadsdeployment"
	"github.com/openshift/console-operator/pkg/console/controllers/healthcheck"
	"github.com/openshift/console-operator/pkg/console/controllers/oauthclients"
	"github.com/openshift/console-operator/pkg/console/controllers/oauthclientsecret"
	"github.com/openshift/console-operator/pkg/console/controllers/oidcsetup"
	pdb "github.com/openshift/console-operator/pkg/console/controllers/poddisruptionbudget"
	"github.com/openshift/console-operator/pkg/console/controllers/route"
	"github.com/openshift/console-operator/pkg/console/controllers/service"
	upgradenotification "github.com/openshift/console-operator/pkg/console/controllers/upgradenotification"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/operatorclient"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/configobserver/featuregates"
	"github.com/openshift/library-go/pkg/operator/managementstatecontroller"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/staleconditions"
	"github.com/openshift/library-go/pkg/operator/status"
	"github.com/openshift/library-go/pkg/operator/unsupportedconfigoverridescontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	// clients
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"

	authclient "github.com/openshift/client-go/oauth/clientset/versioned"

	operatorversionedclient "github.com/openshift/client-go/operator/clientset/versioned"
	operatorinformers "github.com/openshift/client-go/operator/informers/externalversions"

	routesclient "github.com/openshift/client-go/route/clientset/versioned"
	routesinformers "github.com/openshift/client-go/route/informers/externalversions"

	consolev1client "github.com/openshift/client-go/console/clientset/versioned"
	consoleinformers "github.com/openshift/client-go/console/informers/externalversions"

	telemetry "github.com/openshift/console-operator/pkg/console/telemetry"

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

	dynamicClient, err := dynamic.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	const resync = 10 * time.Minute

	kubeInformersNamespaced := informers.NewSharedInformerFactoryWithOptions(
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

	kubeInformersMonitoringNamespaced := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resync,
		informers.WithNamespace(telemetry.TelemeterClientDeploymentNamespace),
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

	consoleInformers := consoleinformers.NewSharedInformerFactory(
		consoleClient,
		resync,
	)

	dynamicInformers := dynamicinformer.NewDynamicSharedInformerFactory(
		dynamicClient,
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

	oauthClientsSwitchedInformer := util.NewSwitchedInformer(ctx,
		oauthClient,
		resync,
		configInformers.Config().V1().Authentications(),
		recorder,
	)

	err = startStaticResourceSyncing(resourceSyncer)
	if err != nil {
		return err
	}

	desiredVersion := status.VersionForOperatorFromEnv()
	missingVersion := "0.0.1-snapshot"

	featureGateAccessor := featuregates.NewFeatureGateAccess(
		desiredVersion, missingVersion,
		configInformers.Config().V1().ClusterVersions(),
		configInformers.Config().V1().FeatureGates(),
		controllerContext.EventRecorder,
	)

	configInformers.Start(ctx.Done())
	go featureGateAccessor.Run(ctx)

	select {
	case <-featureGateAccessor.InitialFeatureGatesObserved():
		featureGates, _ := featureGateAccessor.CurrentFeatureGates()
		klog.Infof("FeatureGates initialized: knownFeatureGates=%v", featureGates.KnownFeatures())
	case <-time.After(1 * time.Minute):
		klog.Errorf("timed out waiting for FeatureGate detection")
		return fmt.Errorf("timed out waiting for FeatureGate detection")
	}

	featureGates, err := featureGateAccessor.CurrentFeatureGates()
	if err != nil {
		return err
	}

	// TODO: rearrange these into informer,client pairs, NOT separated.
	consoleOperator := operator.NewConsoleOperator(
		ctx,
		// top level config
		configClient.ConfigV1(),
		configInformers,
		dynamicClient,
		dynamicInformers,
		// operator
		operatorClient,
		operatorConfigClient.OperatorV1(),
		operatorConfigInformers.Operator().V1().Consoles(), // OperatorConfig
		// core resources
		kubeClient.CoreV1(),                 // Secrets, ConfigMaps, Service
		kubeInformersNamespaced.Core().V1(), // Secrets, ConfigMaps, Service
		// deployments
		kubeClient.AppsV1(),
		kubeInformersNamespaced.Apps().V1().Deployments(), // Deployments
		kubeInformersMonitoringNamespaced.Apps().V1().Deployments(),
		// oauth
		oauthClientsSwitchedInformer,
		// routes
		routesInformersNamespaced.Route().V1().Routes(), // Route
		// plugins
		consoleInformers.Console().V1().ConsolePlugins(),
		// openshift
		kubeInformersConfigNamespaced.Core().V1().ConfigMaps(), // openshift-config configMaps
		kubeInformersConfigNamespaced.Core().V1().Secrets(),    // openshift-config secrets
		// openshift managed
		kubeInformersManagedNamespaced.Core().V1(), // Managed ConfigMaps
		// event handling
		versionGetter,
		recorder,
		resourceSyncer,
	)

	apiextensionsClient, err := apiextensionsclient.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}
	apiextensionsInformers := apiexensionsinformers.NewSharedInformerFactory(apiextensionsClient, resync)

	oauthClientController := oauthclients.NewOAuthClientsController(
		operatorClient,
		oauthClient,
		configInformers.Config().V1().Authentications(),
		operatorConfigInformers.Operator().V1().Consoles(),
		routesInformersNamespaced.Route().V1().Routes(),
		configInformers.Config().V1().Ingresses(),
		kubeInformersNamespaced.Core().V1().Secrets(),
		oauthClientsSwitchedInformer,
		recorder,
	)

	oauthClientSecretController := oauthclientsecret.NewOAuthClientSecretController(
		operatorClient,
		kubeClient.CoreV1(),
		configInformers.Config().V1().Authentications(),
		operatorConfigInformers.Operator().V1().Consoles(),
		kubeInformersConfigNamespaced.Core().V1().Secrets(),
		kubeInformersNamespaced.Core().V1().Secrets(),
		recorder,
	)

	externalOIDCEnabled := featureGates.Enabled("ExternalOIDC")
	oidcSetupController := oidcsetup.NewOIDCSetupController(
		operatorClient,
		kubeClient.CoreV1(),
		configInformers.Config().V1().Authentications(),
		configClient.ConfigV1().Authentications(),
		operatorConfigInformers.Operator().V1().Consoles(),
		kubeInformersConfigNamespaced.Core().V1().ConfigMaps(),
		kubeInformersNamespaced.Core().V1().Secrets(),
		kubeInformersNamespaced.Core().V1().ConfigMaps(),
		kubeInformersNamespaced.Apps().V1().Deployments(),
		externalOIDCEnabled,
		recorder,
	)

	cliOIDCClientStatusController := clioidcclientstatus.NewCLIOIDCClientStatusController(
		operatorClient,
		configInformers.Config().V1().Authentications(),
		configClient.ConfigV1().Authentications(),
		operatorConfigInformers.Operator().V1().Consoles(),
		externalOIDCEnabled,
		recorder,
	)

	downloadsDeploymentController := downloadsdeployment.NewDownloadsDeploymentSyncController(
		// clients
		operatorClient,
		configInformers,
		// operator
		operatorConfigInformers.Operator().V1().Consoles(),

		kubeClient.AppsV1(), // Deployments
		kubeInformersNamespaced.Apps().V1().Deployments(), // Deployments
		recorder,
	)

	cliDownloadsController := clidownloads.NewCLIDownloadsSyncController(
		// top level config
		configClient.ConfigV1(),
		// clients
		operatorClient,
		consoleClient.ConsoleV1().ConsoleCLIDownloads(),
		// informers
		operatorConfigInformers.Operator().V1().Consoles(), // OperatorConfig
		configInformers, // Config
		consoleInformers.Console().V1().ConsoleCLIDownloads(), // ConsoleCliDownloads
		routesInformersNamespaced.Route().V1().Routes(),       // Routes
		// events
		recorder,
	)

	consoleServiceController := service.NewServiceSyncController(
		api.OpenShiftConsoleServiceName,
		// top level config
		configClient.ConfigV1(),
		configInformers,
		// clients
		operatorClient,
		kubeClient.CoreV1(), // only needs to interact with the service resource
		// informers
		operatorConfigInformers.Operator().V1().Consoles(), // OperatorConfig
		kubeInformersNamespaced.Core().V1().Services(),     // Services
		// events
		recorder,
	)

	downloadsServiceController := service.NewServiceSyncController(
		api.DownloadsResourceName,
		// top level config
		configClient.ConfigV1(),
		configInformers,
		operatorClient,
		// clients
		kubeClient.CoreV1(), // only needs to interact with the service resource
		// informers
		operatorConfigInformers.Operator().V1().Consoles(), // OperatorConfig
		kubeInformersNamespaced.Core().V1().Services(),     // Services
		// events
		recorder,
	)

	consoleRouteController := route.NewRouteSyncController(
		api.OpenShiftConsoleRouteName,
		// enable health check for console route
		true,
		// top level config
		configInformers,
		// clients
		operatorClient,
		routesClient.RouteV1(),
		// route
		operatorConfigInformers.Operator().V1().Consoles(),
		kubeInformersConfigNamespaced.Core().V1().Secrets(), // `openshift-config` namespace informers
		routesInformersNamespaced.Route().V1().Routes(),
		// events
		recorder,
	)

	downloadsRouteController := route.NewRouteSyncController(
		api.OpenShiftConsoleDownloadsRouteName,
		// disable health check for console route
		false,
		// top level config
		configInformers,
		// clients
		operatorClient,
		routesClient.RouteV1(),
		// route
		operatorConfigInformers.Operator().V1().Consoles(),
		kubeInformersConfigNamespaced.Core().V1().Secrets(), // `openshift-config` namespace informers
		routesInformersNamespaced.Route().V1().Routes(),
		// events
		recorder,
	)

	consoleRouteHealthCheckController := healthcheck.NewHealthCheckController(
		// top level config
		configClient.ConfigV1(),
		// clients
		operatorClient,
		// route
		operatorConfigInformers.Operator().V1().Consoles(),
		configInformers,                     // Config
		kubeInformersNamespaced.Core().V1(), // `openshift-console` namespace informers
		routesInformersNamespaced.Route().V1().Routes(),
		// events
		recorder,
	)

	upgradeNotificationController := upgradenotification.NewUpgradeNotificationController(
		// top level config
		configInformers,
		// clients
		operatorClient,
		operatorConfigInformers.Operator().V1().Consoles(),
		consoleClient.ConsoleV1().ConsoleNotifications(),
		//events
		recorder,
	)

	versionRecorder := status.NewVersionGetter()
	versionRecorder.SetVersion("operator", os.Getenv("OPERATOR_IMAGE_VERSION"))

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

	// Show all ConsolePlugin instances as related objects
	clusterOperatorStatus.WithRelatedObjectsFunc(func() (bool, []configv1.ObjectReference) {
		relatedObjects := []configv1.ObjectReference{}
		consolePlugins, err := consoleClient.ConsoleV1().ConsolePlugins().List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, nil
		}
		for _, plugin := range consolePlugins.Items {
			relatadPlugin := configv1.ObjectReference{
				Group:    "console.openshift.io",
				Resource: "consoleplugins",
				Name:     plugin.GetName(),
			}
			relatedObjects = append(relatedObjects, relatadPlugin)
		}
		return true, relatedObjects
	})

	// NOTE: be sure to uncomment the .Run() below if using this
	staleConditionsController := staleconditions.NewRemoveStaleConditionsController(
		[]string{
			// If a condition is removed, we need to add it here for at least
			// one release to ensure the operator does not permanently wedge.
			// Please do something like the following:
			//
			// example: in 4.x.x we removed FooDegraded condition and can remove
			// this in 4.x+1:
			// "FooDegraded",
			//
			// in 4.8 we removed DonwloadsDeploymentSyncDegraded and can remove this in 4.9
			// "DonwloadsDeploymentSyncDegraded",
			// in 4.9 we replaced DefaultIngressCertValidation with OAuthServingCertValidation and can remove this in 4.10
			// "DefaultIngressCertValidation",
			// in 4.13 we are removing CustomRouteSync and DefaultRouteSync
			// we are need to backport the removal of these conditions all the way down to 4.10
			// since we only remove them in https://github.com/openshift/console-operator/pull/662
			// and its causing upgrade issues to the customers
			"CustomRouteSyncDegraded",
			"CustomRouteSyncProgressing",
			"DefaultRouteSyncDegraded",
			"DefaultRouteSyncProgressing",
		},
		operatorClient,
		controllerContext.EventRecorder,
	)

	// instantiate pdb client
	policyClient, err := policyv1client.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	consolePDBController := pdb.NewPodDisruptionBudgetController(
		api.OpenShiftConsoleName,
		// clients
		operatorClient,
		operatorConfigInformers.Operator().V1().Consoles(),
		policyClient,
		// informers
		kubeInformersNamespaced.Policy().V1().PodDisruptionBudgets(),
		//events
		recorder,
	)

	downloadsPDBController := pdb.NewPodDisruptionBudgetController(
		api.DownloadsResourceName,
		// clients
		operatorClient,
		operatorConfigInformers.Operator().V1().Consoles(),
		policyClient,
		// informers
		kubeInformersNamespaced.Policy().V1().PodDisruptionBudgets(),
		//events
		recorder,
	)

	configUpgradeableController := unsupportedconfigoverridescontroller.NewUnsupportedConfigOverridesController(operatorClient, controllerContext.EventRecorder)
	logLevelController := loglevel.NewClusterOperatorLoggingController(operatorClient, controllerContext.EventRecorder)
	managementStateController := managementstatecontroller.NewOperatorManagementStateController(api.ClusterOperatorName, operatorClient, controllerContext.EventRecorder)

	for _, informer := range []interface {
		Start(stopCh <-chan struct{})
	}{
		apiextensionsInformers,
		configInformers,
		kubeInformersNamespaced,
		kubeInformersConfigNamespaced,
		kubeInformersManagedNamespaced,
		kubeInformersMonitoringNamespaced,
		resourceSyncerInformers,
		operatorConfigInformers,
		consoleInformers,
		routesInformersNamespaced,
		dynamicInformers,
		oauthClientsSwitchedInformer,
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
		consolePDBController,
		downloadsPDBController,
		oauthClientController,
		oauthClientSecretController,
		oidcSetupController,
		cliOIDCClientStatusController,
		upgradeNotificationController,
		staleConditionsController,
	} {
		go controller.Run(ctx, 1)
	}

	<-ctx.Done()
	return fmt.Errorf("stopped")
}

// startResourceSyncing should start syncing process of all secrets and configmaps that need to be synced.
func startStaticResourceSyncing(resourceSyncer *resourcesynccontroller.ResourceSyncController) error {
	// sync: 'oauth-serving-cert' configmap
	// from: 'openshift-config-managed' namespace
	// to:   'openshift-console' namespace
	err := resourceSyncer.SyncConfigMap(
		resourcesynccontroller.ResourceLocation{Name: api.OAuthServingCertConfigMapName, Namespace: api.OpenShiftConsoleNamespace},
		resourcesynccontroller.ResourceLocation{Name: api.OAuthServingCertConfigMapName, Namespace: api.OpenShiftConfigManagedNamespace},
	)
	if err != nil {
		return err
	}

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
