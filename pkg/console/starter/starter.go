package starter

import (
	"fmt"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/status"

	// clients
	configclient "github.com/openshift/client-go/config/clientset/versioned"

	authclient "github.com/openshift/client-go/oauth/clientset/versioned"
	routesclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/console-operator/pkg/generated/clientset/versioned"
	// informers
	oauthinformers "github.com/openshift/client-go/oauth/informers/externalversions"
	routesinformers "github.com/openshift/client-go/route/informers/externalversions"
	"github.com/openshift/console-operator/pkg/generated/informers/externalversions"

	// operator
	"github.com/openshift/console-operator/pkg/controller"
	"github.com/openshift/console-operator/pkg/console/operator"
	"github.com/openshift/console-operator/pkg/console/subresource/configmap"
	"github.com/openshift/console-operator/pkg/console/subresource/deployment"
	"github.com/openshift/console-operator/pkg/console/subresource/secret"
	"github.com/openshift/console-operator/pkg/console/subresource/service"
)

func RunOperator(ctx *controllercmd.ControllerContext) error {
	clientConfig := ctx.KubeConfig
	stopCh := ctx.StopCh

	// for the OperatorStatus
	configClient, err := configclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

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

	tweakOAuthListOptions := func(options *v1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("metadata.name", controller.OAuthClientName).String()
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
		oauthinformers.WithTweakListOptions(tweakOAuthListOptions),
	)

	// event recorders
	configMapEventRecorder := events.NewRecorder(
		kubeClient.CoreV1().Events(controller.TargetNamespace),
		controller.ResourceName,
		configmap.Ref(),
	)
	serviceEventRecorder := events.NewRecorder(
		kubeClient.CoreV1().Events(controller.TargetNamespace),
		controller.ResourceName,
		service.Ref(),
	)
	secretEventRecorder := events.NewRecorder(
		kubeClient.CoreV1().Events(controller.TargetNamespace),
		controller.ResourceName,
		secret.Ref(),
	)
	deploymentEventRecorder := events.NewRecorder(
		kubeClient.CoreV1().Events(controller.TargetNamespace),
		controller.ResourceName,
		deployment.Ref(),
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
		// recorders
		configMapEventRecorder,
		deploymentEventRecorder,
		serviceEventRecorder,
		secretEventRecorder,
	)

	kubeInformersNamespaced.Start(stopCh)
	consoleOperatorInformers.Start(stopCh)
	routesInformersNamespaced.Start(stopCh)
	oauthInformers.Start(stopCh)

	go consoleOperator.Run(stopCh)

	clusterOperatorStatus := status.NewClusterOperatorStatusController(
		controller.ResourceName,
		configClient.ConfigV1(),
		&operatorStatusProvider{informers: consoleOperatorInformers},
	)
	//// TODO: will have a series of Run() funcs here
	go clusterOperatorStatus.Run(1, stopCh)

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

func (p *operatorStatusProvider) CurrentStatus() (operatorv1.OperatorStatus, error) {
	instance, err := p.informers.Console().V1alpha1().Consoles().Lister().Consoles(controller.TargetNamespace).Get(controller.ResourceName)
	if err != nil {
		return operatorv1.OperatorStatus{}, err
	}
	return instance.Status.OperatorStatus, nil
}
