package starter

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	// "k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	authclient "github.com/openshift/client-go/oauth/clientset/versioned"
	// clients
	routesclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/console-operator/pkg/generated/clientset/versioned"
	// informers
	oauthinformers "github.com/openshift/client-go/oauth/informers/externalversions"
	routesinformers "github.com/openshift/client-go/route/informers/externalversions"
	"github.com/openshift/console-operator/pkg/generated/informers/externalversions"

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/console-operator/pkg/console/operator"
	"github.com/openshift/console-operator/pkg/controller"
)

func RunOperator(clientConfig *rest.Config, stopCh <-chan struct{}) error {
	// TODO: reenable this after upgradeing library-go
	// only for the ClusterStatus, everything else has a specific client
	//dynamicClient, err := dynamic.NewForConfig(clientConfig)
	//if err != nil {
	//	return err
	//}

	// creates a new kube clientset
	// clientConfig is a REST config
	// a clientSet contains clients for groups.
	// each group has one version included in the set.
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	// pkg/apis/console/v1alpha1/types.go has a `genclient` annotation,
	// that creates the expected functions for the type.
	consoleOperatorClient, err := versioned.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	routesClient, err := routesclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	oauthClient, err := authclient.NewForConfig(clientConfig)
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

	kubeInformersNamespaced := informers.NewSharedInformerFactoryWithOptions(
		// takes a client
		kubeClient,
		resync,
		// takes an unlimited number of additional "options" arguments, which are functions,
		// that take a sharedInformerFactory and return a sharedInformerFactory
		informers.WithNamespace(controller.TargetNamespace),
		informers.WithTweakListOptions(tweakListOptions),
	)

	consoleOperatorInformers := externalversions.NewSharedInformerFactoryWithOptions(
		// this is our generated client
		consoleOperatorClient,
		resync,
		// and the same set of optional transform functions
		externalversions.WithNamespace(controller.TargetNamespace),
		externalversions.WithTweakListOptions(tweakListOptions),
	)

	routesInformersNamespaced := routesinformers.NewSharedInformerFactoryWithOptions(
		routesClient,
		resync,
		routesinformers.WithNamespace(controller.TargetNamespace),
		routesinformers.WithTweakListOptions(tweakListOptions),
	)

	// oauthclients are not namespaced
	oauthInformers := oauthinformers.NewSharedInformerFactoryWithOptions(
		oauthClient,
		resync,
		oauthinformers.WithTweakListOptions(tweakListOptions),
	)

	consoleOperator := operator.NewConsoleOperator(
		// informers
		consoleOperatorInformers.Console().V1alpha1().Consoles(), // Console
		kubeInformersNamespaced.Core().V1(),                      // Secrets, ConfigMaps, Service
		kubeInformersNamespaced.Apps().V1().Deployments(),        // Deployments
		routesInformersNamespaced.Route().V1().Routes(),          // Route
		oauthInformers.Oauth().V1().OAuthClients(),               // OAuth clients
		// clients
		consoleOperatorClient.ConsoleV1alpha1(),
		kubeClient.CoreV1(), // Secrets, ConfigMaps, Service
		kubeClient.AppsV1(),
		routesClient.RouteV1(),
		oauthClient.OauthV1(),
	)

	kubeInformersNamespaced.Start(stopCh)
	consoleOperatorInformers.Start(stopCh)
	routesInformersNamespaced.Start(stopCh)
	oauthInformers.Start(stopCh)

	go consoleOperator.Run(stopCh)

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

	<-stopCh

	return fmt.Errorf("stopped")
}

// I'd prefer this in a /console/status/ package, but other operators keep it here.
type operatorStatusProvider struct {
	informers externalversions.SharedInformerFactory
}

func (p *operatorStatusProvider) Informer() cache.SharedIndexInformer {
	return p.informers.Console().V1alpha1().Consoles().Informer()
}

func (p *operatorStatusProvider) CurrentStatus() (operatorv1alpha1.OperatorStatus, error) {
	instance, err := p.informers.Console().V1alpha1().Consoles().Lister().Consoles(controller.TargetNamespace).Get(controller.ResourceName)
	if err != nil {
		return operatorv1alpha1.OperatorStatus{}, err
	}

	return instance.Status.OperatorStatus, nil
}
