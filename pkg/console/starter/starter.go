package starter

import (
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
	"github.com/openshift/console-operator/pkg/console/operatorclient"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/staleconditions"
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
	"github.com/openshift/console-operator/pkg/console/controllers/service"
	"github.com/openshift/console-operator/pkg/console/operator"
	"github.com/openshift/library-go/pkg/operator/loglevel"
)

func RunOperator(ctx *controllercmd.ControllerContext) error {

	kubeClient, err := kubernetes.NewForConfig(ctx.ProtoKubeConfig)
	if err != nil {
		return err
	}

	configClient, err := configclient.NewForConfig(ctx.KubeConfig)
	if err != nil {
		return err
	}

	operatorConfigClient, err := operatorversionedclient.NewForConfig(ctx.KubeConfig)
	if err != nil {
		return err
	}

	consoleClient, err := consolev1client.NewForConfig(ctx.KubeConfig)
	if err != nil {
		return err
	}

	routesClient, err := routesclient.NewForConfig(ctx.ProtoKubeConfig)
	if err != nil {
		return err
	}

	oauthClient, err := authclient.NewForConfig(ctx.ProtoKubeConfig)
	if err != nil {
		return err
	}

	const resync = 10 * time.Minute

	tweakListOptionsForOAuth := func(options *metav1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("metadata.name", api.OAuthClientName).String()
	}

	tweakListOptionsForRoute := func(options *metav1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("metadata.name", api.OpenShiftConsoleRouteName).String()
	}

	kubeInformersNamespaced := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resync,
		informers.WithNamespace(api.TargetNamespace),
	)

	kubeInformersManagedNamespaced := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resync,
		informers.WithNamespace(api.OpenShiftConfigManagedNamespace),
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
		routesinformers.WithTweakListOptions(tweakListOptionsForRoute),
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
	}

	recorder := ctx.EventRecorder

	versionGetter := status.NewVersionGetter()

	resourceSyncerInformers, resourceSyncer := getResourceSyncer(ctx, clientwrapper.WithoutSecret(kubeClient), operatorClient)

	// TODO: rearrange these into informer,client pairs, NOT separated.
	consoleOperator := operator.NewConsoleOperator(
		// top level config
		configClient.ConfigV1(),
		configInformers,
		// operator
		operatorConfigClient.OperatorV1(),
		operatorConfigInformers.Operator().V1().Consoles(), // OperatorConfig

		// core resources
		kubeClient.CoreV1(),                 // Secrets, ConfigMaps, Service
		kubeInformersNamespaced.Core().V1(), // Secrets, ConfigMaps, Service
		// deployments
		kubeClient.AppsV1(),
		kubeInformersNamespaced.Apps().V1().Deployments(), // Deployments
		// routes
		routesClient.RouteV1(),
		routesInformersNamespaced.Route().V1().Routes(), // Route
		// oauth
		oauthClient.OauthV1(),
		oauthInformers.Oauth().V1().OAuthClients(), // OAuth clients
		// openshift managed
		kubeInformersManagedNamespaced.Core().V1(), // Managed ConfigMaps
		// event handling
		versionGetter,
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
		// recorder
		recorder,
	)

	consoleServiceController := service.NewServiceSyncController(
		// operator config so we can update status
		operatorConfigClient.OperatorV1().Consoles(),
		// only needs to interact with the service resource
		kubeClient.CoreV1(),
		kubeInformersNamespaced.Core().V1().Services(),
		// names
		api.OpenShiftConsoleNamespace,
		api.OpenShiftConsoleName,
		// events
		recorder,
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
		ctx.EventRecorder,
	)

	staleConditionsController := staleconditions.NewRemoveStaleConditions(
		[]string{
			// in 4.1.0 we directly set this set of conditions. We should no longer do this.
			// in 4.3.0+ we can remove this
			"Degraded",
			// in follow-up PRs, we should stop using the rest directly:
			"Progressing",
			"Available",
			// "Faililng"
		},
		operatorClient,
		ctx.EventRecorder,
	)

	configUpgradeableController := unsupportedconfigoverridescontroller.NewUnsupportedConfigOverridesController(operatorClient, ctx.EventRecorder)
	logLevelController := loglevel.NewClusterOperatorLoggingController(operatorClient, ctx.EventRecorder)
	managementStateController := management.NewOperatorManagementStateController(api.ClusterOperatorName, operatorClient, ctx.EventRecorder)

	for _, informer := range []interface {
		Start(stopCh <-chan struct{})
	}{
		kubeInformersNamespaced,
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

	go consoleServiceController.Run(1, ctx.Done())
	go consoleOperator.Run(ctx.Done())
	go resourceSyncer.Run(1, ctx.Done())
	go clusterOperatorStatus.Run(1, ctx.Done())
	go configUpgradeableController.Run(1, ctx.Done())
	go logLevelController.Run(1, ctx.Done())
	go managementStateController.Run(1, ctx.Done())
	go cliDownloadsController.Run(1, ctx.Done())
	go staleConditionsController.Run(1, ctx.Done())

	<-ctx.Done()
	return fmt.Errorf("stopped")
}

func getResourceSyncer(ctx *controllercmd.ControllerContext, kubeClient kubernetes.Interface, operatorClient v1helpers.OperatorClient) (v1helpers.KubeInformersForNamespaces, *resourcesynccontroller.ResourceSyncController) {
	resourceSyncerInformers := v1helpers.NewKubeInformersForNamespaces(
		kubeClient,
		api.OpenShiftConfigNamespace,
		api.OpenShiftConsoleNamespace,
	)
	resourceSyncer := resourcesynccontroller.NewResourceSyncController(
		operatorClient,
		resourceSyncerInformers,
		v1helpers.CachedSecretGetter(kubeClient.CoreV1(), resourceSyncerInformers),
		v1helpers.CachedConfigMapGetter(kubeClient.CoreV1(), resourceSyncerInformers),
		ctx.EventRecorder,
	)
	return resourceSyncerInformers, resourceSyncer
}
