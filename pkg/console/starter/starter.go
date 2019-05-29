package starter

import (
	"fmt"

	"github.com/openshift/api/oauth"

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
	operatorv1 "github.com/openshift/api/operator"
	"github.com/openshift/console-operator/pkg/api"
	operatorclient "github.com/openshift/console-operator/pkg/console/operatorclient"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/status"
	"github.com/openshift/library-go/pkg/operator/unsupportedconfigoverridescontroller"

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
	// configs are all named "cluster", but our clusteroperator is named "console"
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

	operatorClient := &operatorclient.OperatorClient{
		Informers: operatorConfigInformers,
		Client:    operatorConfigClient.OperatorV1(),
	}

	recorder := ctx.EventRecorder

	versionGetter := status.NewVersionGetter()

	// TODO: rearrange these into informer,client pairs, NOT separated.
	consoleOperator := operator.NewConsoleOperator(
		// informers
		operatorConfigInformers.Operator().V1().Consoles(), // OperatorConfig
		configInformers,                                   // ConsoleConfig
		kubeInformersNamespaced.Core().V1(),               // Secrets, ConfigMaps, Service
		kubeInformersManagedNamespaced.Core().V1(),        // Managed ConfigMaps
		kubeInformersNamespaced.Apps().V1().Deployments(), // Deployments
		routesInformersNamespaced.Route().V1().Routes(),   // Route
		oauthInformers.Oauth().V1().OAuthClients(),        // OAuth clients
		// clients
		operatorConfigClient.OperatorV1(),
		configClient.ConfigV1(),

		kubeClient.CoreV1(), // Secrets, ConfigMaps, Service
		kubeClient.AppsV1(),
		routesClient.RouteV1(),
		oauthClient.OauthV1(),
		versionGetter,
		recorder,
	)

	versionRecorder := status.NewVersionGetter()
	versionRecorder.SetVersion("operator", os.Getenv("RELEASE_VERSION"))

	clusterOperatorStatus := status.NewClusterOperatorStatusController(
		"console",
		[]configv1.ObjectReference{
			{Group: operatorv1.GroupName, Resource: "consoles", Name: api.ConfigResourceName},
			{Group: configv1.GroupName, Resource: "consoles", Name: api.ConfigResourceName},
			{Group: configv1.GroupName, Resource: "infrastructures", Name: api.ConfigResourceName},
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

	configUpgradeableController := unsupportedconfigoverridescontroller.NewUnsupportedConfigOverridesController(operatorClient, ctx.EventRecorder)

	for _, informer := range []interface {
		Start(stopCh <-chan struct{})
	}{
		kubeInformersNamespaced,
		kubeInformersManagedNamespaced,
		operatorConfigInformers,
		configInformers,
		routesInformersNamespaced,
		oauthInformers,
	} {
		informer.Start(ctx.Done())
	}

	go consoleOperator.Run(ctx.Done())
	go clusterOperatorStatus.Run(1, ctx.Done())
	go configUpgradeableController.Run(1, ctx.Done())

	<-ctx.Done()
	return fmt.Errorf("stopped")
}
