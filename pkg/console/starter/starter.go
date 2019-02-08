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
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/library-go/pkg/controller/controllercmd"

	// clients
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"

	authclient "github.com/openshift/client-go/oauth/clientset/versioned"
	oauthinformers "github.com/openshift/client-go/oauth/informers/externalversions"

	operatorclient "github.com/openshift/client-go/operator/clientset/versioned"
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

	kubeClient, err := kubernetes.NewForConfig(ctx.KubeConfig)
	if err != nil {
		return err
	}

	consoleConfigClient, err := configclient.NewForConfig(ctx.KubeConfig)
	if err != nil {
		return err
	}

	consoleOperatorConfigClient, err := operatorclient.NewForConfig(ctx.KubeConfig)
	if err != nil {
		return err
	}

	routesClient, err := routesclient.NewForConfig(ctx.KubeConfig)
	if err != nil {
		return err
	}

	oauthClient, err := authclient.NewForConfig(ctx.KubeConfig)
	if err != nil {
		return err
	}

	const resync = 10 * time.Minute

	// NOOP for now
	// TODO: can perhaps put this back the way it was, but may
	// need to create a couple different version for
	// resources w/different names
	tweakListOptions := func(options *v1.ListOptions) {
		// options.FieldSelector = fields.OneTermEqualSelector("metadata.name", operator.ResourceName).String()
	}

	tweakOAuthListOptions := func(options *v1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("metadata.name", api.OAuthClientName).String()
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
		configinformers.WithTweakListOptions(tweakListOptions),
	)

	consoleOperatorConfigInformers := operatorinformers.NewSharedInformerFactoryWithOptions(
		consoleOperatorConfigClient,
		resync,
		operatorinformers.WithTweakListOptions(tweakListOptions),
	)

	routesInformersNamespaced := routesinformers.NewSharedInformerFactoryWithOptions(
		routesClient,
		resync,
		routesinformers.WithNamespace(api.TargetNamespace),
		routesinformers.WithTweakListOptions(tweakListOptions),
	)

	// oauthclients are not namespaced
	oauthInformers := oauthinformers.NewSharedInformerFactoryWithOptions(
		oauthClient,
		resync,
		oauthinformers.WithTweakListOptions(tweakOAuthListOptions),
	)

	// TODO: Replace this with real event recorder (use ControllerContext).
	recorder := ctx.EventRecorder

	// TODO: rearrange these into informer,client pairs, NOT separated.
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
		recorder,
	)

	kubeInformersNamespaced.Start(ctx.Context.Done())
	consoleOperatorConfigInformers.Start(ctx.Context.Done())
	consoleConfigInformers.Start(ctx.Context.Done())
	routesInformersNamespaced.Start(ctx.Context.Done())
	oauthInformers.Start(ctx.Context.Done())

	go consoleOperator.Run(ctx.Context.Done())

	// TODO: turn this back on!
	// for now its just creating noise.... as we need to update library-go for it to work correctly
	// our version of library-go has the old group
	//clusterOperatorStatus := status.NewClusterOperatorStatusController(
	//	controller.TargetNamespace,
	//	controller.ResourceName,
	//	// no idea why this is dynamic & not a strongly typed client.
	//	dynamicClient,
	//	&operatorStatusProvider{informers: consoleOperatorInformers},
	//)
	//// TODO: will have a series of Run() funcs here
	//go clusterOperatorStatus.Run(1, stopCh)

	<-ctx.Context.Done()

	return fmt.Errorf("stopped")
}

// I'd prefer this in a /console/status/ package, but other operators keep it here.
//type operatorStatusProvider struct {
//	informers externalversions.SharedInformerFactory
//}
//
//func (p *operatorStatusProvider) Informer() cache.SharedIndexInformer {
//	return p.informers.Console().V1().Consoles().Informer()
//}
//
//func (p *operatorStatusProvider) CurrentStatus() (operatorv1.OperatorStatus, error) {
//	instance, err := p.informers.Console().V1().Consoles().Lister().Consoles(api.TargetNamespace).Get(api.ResourceName)
//	if err != nil {
//		return operatorv1.OperatorStatus{}, err
//	}
//
//	return instance.Status.OperatorStatus, nil
//}
