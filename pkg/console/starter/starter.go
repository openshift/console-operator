package starter

import (
	"fmt"
	"time"

	// kube
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	// "k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/operatorclient"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/status"

	// informers

	// clients
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"

	authclient "github.com/openshift/client-go/oauth/clientset/versioned"
	oauthinformers "github.com/openshift/client-go/oauth/informers/externalversions"

	operatorversionedclient "github.com/openshift/client-go/operator/clientset/versioned"
	operatorinformers "github.com/openshift/client-go/operator/informers/externalversions"

	routesclient "github.com/openshift/client-go/route/clientset/versioned"
	routesinformers "github.com/openshift/client-go/route/informers/externalversions"

	"github.com/openshift/console-operator/pkg/console/operator"
)

func RunOperator(ctx *controllercmd.ControllerContext) error {
	// TODO: reenable this after upgradeing library-go
	// only for the ClusterStatus, everything else has a specific client
	//dynamicClient, err := dynamic.NewForConfig(ctx.KubeConfig)
	//if err != nil {
	//	return err
	//}

	kubeClient, err := kubernetes.NewForConfig(ctx.ProtoKubeConfig)
	if err != nil {
		return err
	}

	consoleConfigClient, err := configclient.NewForConfig(ctx.KubeConfig)
	if err != nil {
		return err
	}

	consoleOperatorConfigClient, err := operatorversionedclient.NewForConfig(ctx.KubeConfig)
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

	// NOOP for now
	// TODO: can perhaps put this back the way it was, but may
	// need to create a couple different version for
	// resources w/different names
	tweakListOptions := func(options *v1.ListOptions) {
		// options.FieldSelector = fields.OneTermEqualSelector("metadata.name", operator.ConfigResourceName).String()
	}

	tweakListOptionsForConfigs := func(options *v1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("metadata.name", api.ConfigResourceName).String()
	}

	tweakListOptionsForClusterOperator := func(options *v1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("metadata.name", api.OpenShiftConsoleName).String()
	}

	tweakListOptionsForOAuth := func(options *v1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("metadata.name", api.OAuthClientName).String()
	}

	tweakListOptionsForRoute := func(options *v1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("metadata.name", api.OpenShiftConsoleRouteName).String()
	}

	kubeInformersNamespaced := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resync,
		informers.WithNamespace(api.TargetNamespace),
		informers.WithTweakListOptions(tweakListOptions),
	)

	consoleConfigInformers := configinformers.NewSharedInformerFactoryWithOptions(
		consoleConfigClient,
		resync,
		configinformers.WithTweakListOptions(tweakListOptionsForConfigs),
	)

	consoleOperatorConfigInformers := operatorinformers.NewSharedInformerFactoryWithOptions(
		consoleOperatorConfigClient,
		resync,
		operatorinformers.WithTweakListOptions(tweakListOptionsForConfigs),
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

	clusterOperatorInformers := configinformers.NewSharedInformerFactoryWithOptions(
		consoleConfigClient,
		resync,
		configinformers.WithTweakListOptions(tweakListOptionsForClusterOperator),
	)

	// TODO: Replace this with real event recorder (use ControllerContext).
	recorder := ctx.EventRecorder

	operatorClient := &operatorclient.OperatorClient{
		Informers: consoleOperatorConfigInformers,
		Client:    consoleOperatorConfigClient.OperatorV1(),
	}

	// TODO: rearrange these into informer,client pairs, NOT separated.
	// We need the clusteroperator/console watched here...
	consoleOperator := operator.NewConsoleOperator(
		// informers
		consoleOperatorConfigInformers.Operator().V1().Consoles(), // OperatorConfig
		consoleConfigInformers.Config().V1().Consoles(),           // ConsoleConfig

		kubeInformersNamespaced.Core().V1(),               // Secrets, ConfigMaps, Service
		kubeInformersNamespaced.Apps().V1().Deployments(), // Deployments
		routesInformersNamespaced.Route().V1().Routes(),   // Route
		oauthInformers.Oauth().V1().OAuthClients(),        // OAuth clients
		// clients
		consoleOperatorConfigClient.OperatorV1(),
		consoleConfigClient.ConfigV1(),

		kubeClient.CoreV1(), // Secrets, ConfigMaps, Service
		kubeClient.AppsV1(),
		routesClient.RouteV1(),
		oauthClient.OauthV1(),

		clusterOperatorInformers.Config().V1().ClusterOperators(),

		recorder,
	)

	clusterOperatorStatus := status.NewClusterOperatorStatusController(
		"console",
		[]configv1.ObjectReference{
			{Group: "operator.openshift.io", Resource: "consoles", Name: api.ConfigResourceName},
			{Group: "config.openshift.io", Resource: "consoles", Name: api.ConfigResourceName},
			{Group: "oauth.openshift.io", Resource: "oauthclients", Name: api.OAuthClientName},
			{Resource: "namespaces", Name: api.OpenShiftConsoleOperatorNamespace},
			{Resource: "namespaces", Name: api.OpenShiftConsoleNamespace},
		},
		consoleConfigClient.ConfigV1(),
		operatorClient,
		status.NewVersionGetter(),
		ctx.EventRecorder,
	)

	kubeInformersNamespaced.Start(ctx.Done())
	consoleOperatorConfigInformers.Start(ctx.Done())
	consoleConfigInformers.Start(ctx.Done())
	routesInformersNamespaced.Start(ctx.Done())
	oauthInformers.Start(ctx.Done())
	clusterOperatorInformers.Start(ctx.Done())

	go consoleOperator.Run(ctx.Done())
	go clusterOperatorStatus.Run(1, ctx.Done())
	<-ctx.Done()

	return fmt.Errorf("stopped")
}
