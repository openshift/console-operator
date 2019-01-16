package starter

import (
	// standard
	"fmt"
	"time"

	// k8s
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	// openshift
	v1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/status"

	// clients
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	authclient "github.com/openshift/client-go/oauth/clientset/versioned"
	routesclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/console-operator/pkg/generated/clientset/versioned"

	// informers
	oauthinformers "github.com/openshift/client-go/oauth/informers/externalversions"
	routesinformers "github.com/openshift/client-go/route/informers/externalversions"
	"github.com/openshift/console-operator/pkg/generated/informers/externalversions"

	// operator
	"github.com/openshift/console-operator/pkg/console/operator"
)

func RunOperator(ctx *controllercmd.ControllerContext) error {
	configClient, err := configv1client.NewForConfig(ctx.KubeConfig)
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(ctx.KubeConfig)
	if err != nil {
		return err
	}

	consoleOperatorClient, err := versioned.NewForConfig(ctx.KubeConfig)
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
	tweakListOptions := func(options *metav1.ListOptions) {
		// options.FieldSelector = fields.OneTermEqualSelector("metadata.name", operator.ResourceName).String()
	}

	tweakOAuthListOptions := func(options *metav1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("metadata.name", api.OAuthClientName).String()
	}

	kubeInformersNamespaced := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resync,
		// takes an unlimited number of additional "options" arguments, which are functions,
		// that take a sharedInformerFactory and return a sharedInformerFactory
		informers.WithNamespace(api.TargetNamespace),
		informers.WithTweakListOptions(tweakListOptions),
	)

	consoleOperatorInformers := externalversions.NewSharedInformerFactoryWithOptions(
		// this is our generated client
		consoleOperatorClient,
		resync,
		// and the same set of optional transform functions
		externalversions.WithNamespace(api.TargetNamespace),
		externalversions.WithTweakListOptions(tweakListOptions),
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

	consoleOperator := operator.NewConsoleOperator(
		// informers
		consoleOperatorInformers.Console().V1().Consoles(), // Console
		kubeInformersNamespaced.Core().V1(),                // Secrets, ConfigMaps, Service
		kubeInformersNamespaced.Apps().V1().Deployments(),  // Deployments
		routesInformersNamespaced.Route().V1().Routes(),    // Route
		oauthInformers.Oauth().V1().OAuthClients(),         // OAuth clients
		// clients
		consoleOperatorClient.ConsoleV1(),
		kubeClient.CoreV1(), // Secrets, ConfigMaps, Service
		kubeClient.AppsV1(),
		routesClient.RouteV1(),
		oauthClient.OauthV1(),
		recorder,
	)

	kubeInformersNamespaced.Start(ctx.StopCh)
	consoleOperatorInformers.Start(ctx.StopCh)
	routesInformersNamespaced.Start(ctx.StopCh)
	oauthInformers.Start(ctx.StopCh)

	go consoleOperator.Run(ctx.StopCh)

	clusterOperatorStatus := status.NewClusterOperatorStatusController(
		api.ResourceName,
		configClient.ConfigV1(),
		operatorStatusProvider{informers: consoleOperatorInformers},
		ctx.EventRecorder,
	)

	go clusterOperatorStatus.Run(1, ctx.StopCh)

	<-ctx.StopCh

	return fmt.Errorf("stopped")
}

// I'd prefer this in a /console/status/ package, but other operators keep it here.
type operatorStatusProvider struct {
	informers externalversions.SharedInformerFactory
}

func (p operatorStatusProvider) Informer() cache.SharedIndexInformer {
	return p.informers.Console().V1().Consoles().Informer()
}

func (p operatorStatusProvider) CurrentStatus() (v1.OperatorStatus, error) {
	instance, err := p.informers.Console().V1().Consoles().Lister().Consoles(api.TargetNamespace).Get(api.ResourceName)
	if err != nil {
		return v1.OperatorStatus{}, err
	}

	return instance.Status.OperatorStatus, nil
}
