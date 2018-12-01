package operator

import (
	// standard lib
	"fmt"

	"github.com/sirupsen/logrus"
	// kube
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/informers/core/v1"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	// openshift
	operatorv1 "github.com/openshift/api/operator/v1"
	oauthclientv1 "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	oauthinformersv1 "github.com/openshift/client-go/oauth/informers/externalversions/oauth/v1"
	routeclientv1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/openshift/console-operator/pkg/controller"
	// informers
	routesinformersv1 "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	consolev1alpha1 "github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	consoleinformers "github.com/openshift/console-operator/pkg/generated/informers/externalversions/console/v1alpha1"
	appsinformersv1 "k8s.io/client-go/informers/apps/v1"
	// clients
	"github.com/openshift/console-operator/pkg/generated/clientset/versioned/typed/console/v1alpha1"
	// operator
	"github.com/openshift/console-operator/pkg/console/subresource/configmap"
	"github.com/openshift/console-operator/pkg/console/subresource/deployment"
	"github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	"github.com/openshift/console-operator/pkg/console/subresource/route"
	"github.com/openshift/console-operator/pkg/console/subresource/secret"
	"github.com/openshift/console-operator/pkg/console/subresource/service"
)

const (
	// workQueueKey is the singleton key shared by all events
	// the value is irrelevant
	workQueueKey   = "sync-queue"
	controllerName = "Console"
)

var CreateDefaultConsoleFlag bool

// the ConsoleOperator uses specific, strongly-typed clients
// for each resource that it interacts with.
func NewConsoleOperator(
	// informers
	consoles consoleinformers.ConsoleInformer,
	coreV1 v1.Interface,
	deployments appsinformersv1.DeploymentInformer,
	routes routesinformersv1.RouteInformer,
	oauthClients oauthinformersv1.OAuthClientInformer,
	// clients
	operatorClient v1alpha1.ConsolesGetter,
	corev1Client coreclientv1.CoreV1Interface,
	deploymentClient appsv1.DeploymentsGetter,
	routev1Client routeclientv1.RoutesGetter,
	oauthv1Client oauthclientv1.OAuthClientsGetter,
) *ConsoleOperator {
	c := &ConsoleOperator{
		// operator
		operatorClient: operatorClient.Consoles(controller.TargetNamespace),
		// core kube
		secretsClient:    corev1Client,
		configMapClient:  corev1Client,
		serviceClient:    corev1Client,
		deploymentClient: deploymentClient,
		// openshift
		routeClient: routev1Client,
		oauthClient: oauthv1Client,
	}

	operatorInformer := consoles.Informer()

	secretsInformer := coreV1.Secrets().Informer()
	configMapInformer := coreV1.ConfigMaps().Informer()
	serviceInformer := coreV1.Services().Informer()
	deploymentInformer := deployments.Informer()

	routeInformer := routes.Informer()
	oauthInformer := oauthClients.Informer()

	// we do not really need to wait for our informers to sync since we only watch a single resource
	// and make live reads but it does not hurt anything and guarantees we have the correct behavior
	internalController, queue := controller.New(
		controllerName,
		c.sync,
		operatorInformer.HasSynced,
		secretsInformer.HasSynced,
		deploymentInformer.HasSynced,
		configMapInformer.HasSynced,
		serviceInformer.HasSynced,
		routeInformer.HasSynced,
		oauthInformer.HasSynced)

	c.controller = internalController

	operatorInformer.AddEventHandler(eventHandler(queue))
	secretsInformer.AddEventHandler(eventHandler(queue))
	deploymentInformer.AddEventHandler(eventHandler(queue))
	configMapInformer.AddEventHandler(eventHandler(queue))
	serviceInformer.AddEventHandler(eventHandler(queue))
	routeInformer.AddEventHandler(eventHandler(queue))
	oauthInformer.AddEventHandler(eventHandler(queue))

	return c
}

// eventHandler queues the operator to check spec and status
// TODO add filtering and more nuanced logic
// each informer's event handler could have specific logic based on the resource
// for now just rekicking the sync loop is enough since we only watch a single resource by name
func eventHandler(queue workqueue.RateLimitingInterface) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { queue.Add(workQueueKey) },
		UpdateFunc: func(old, new interface{}) { queue.Add(workQueueKey) },
		DeleteFunc: func(obj interface{}) { queue.Add(workQueueKey) },
	}
}

type ConsoleOperator struct {
	// for a performance sensitive operator, it would make sense to use informers
	// to handle reads and clients to handle writes.  since this operator works
	// on a singleton resource, it has no performance requirements.
	operatorClient v1alpha1.ConsoleInterface
	// core kube
	secretsClient    coreclientv1.SecretsGetter
	configMapClient  coreclientv1.ConfigMapsGetter
	serviceClient    coreclientv1.ServicesGetter
	deploymentClient appsv1.DeploymentsGetter
	// openshift
	routeClient routeclientv1.RoutesGetter
	oauthClient oauthclientv1.OAuthClientsGetter
	// controller
	controller *controller.Controller
}

func (c *ConsoleOperator) Run(stopCh <-chan struct{}) {
	// only start one worker because we only have one key name in our queue
	// since this operator works on a singleton, it does not make sense to ever run more than one worker
	c.controller.Run(1, stopCh)
}

// sync() is the handler() function equivalent from the sdk
// this is the big switch statement.
// TODO: clean this up a bit, its messy
func (c *ConsoleOperator) sync(_ interface{}) error {
	// we ignore the passed in key because it will always be workQueueKey
	// it does not matter how the sync loop was triggered
	// all we need to worry about is reconciling the state back to what we expect
	operatorConfig, err := c.operatorClient.Get(controller.ResourceName, metav1.GetOptions{})

	if errors.IsNotFound(err) && CreateDefaultConsoleFlag {
		_, err := c.operatorClient.Create(c.defaultConsole())
		return err
	}
	if err != nil {
		return err
	}

	switch operatorConfig.Spec.ManagementState {
	case operatorv1.Managed:
		fmt.Println("Console is in a managed state.")
		// handled below
	case operatorv1.Unmanaged:
		fmt.Println("Console is in an unmanaged state.")
		return nil
	// take a look @ https://github.com/openshift/service-serving-cert-signer/blob/master/pkg/operator/operator.go#L86
	case operatorv1.Removed:
		fmt.Println("Console has been removed.")
		return c.deleteAllResources(operatorConfig)
	// TODO:
	// case operatorv1.Force
	default:
		// TODO should update status
		return fmt.Errorf("unknown state: %v", operatorConfig.Spec.ManagementState)
	}

	outConfig := operatorConfig.DeepCopy()
	var errs []error
	logrus.Println("Sync-4.0.0")
	outConfig, err = sync_v400(c, outConfig)
	errs = append(errs, err)

	// TODO: this should do better apply logic or similar, maybe use SetStatusFromAvailability
	_, err = c.operatorClient.Update(outConfig)
	errs = append(errs, err)

	return utilerrors.NewAggregate(errs)
}

// this may need to move to sync_v400 if versions ever have custom delete logic
func (c *ConsoleOperator) deleteAllResources(cr *consolev1alpha1.Console) error {
	var errs []error
	// service
	errs = append(errs, c.serviceClient.Services(controller.TargetNamespace).Delete(service.Stub().Name, &metav1.DeleteOptions{}))
	// route
	errs = append(errs, c.routeClient.Routes(controller.TargetNamespace).Delete(route.Stub().Name, &metav1.DeleteOptions{}))
	// configmap
	errs = append(errs, c.configMapClient.ConfigMaps(controller.TargetNamespace).Delete(configmap.Stub().Name, &metav1.DeleteOptions{}))
	// secret
	errs = append(errs, c.secretsClient.Secrets(controller.TargetNamespace).Delete(secret.Stub().Name, &metav1.DeleteOptions{}))
	// existingOAuthClient is not a delete, it is a deregister/neutralize
	existingOAuthClient, getAuthErr := c.oauthClient.OAuthClients().Get(oauthclient.Stub().Name, metav1.GetOptions{})
	errs = append(errs, getAuthErr)
	_, updateAuthErr := c.oauthClient.OAuthClients().Update(oauthclient.DeRegisterConsoleFromOAuthClient(existingOAuthClient))
	errs = append(errs, updateAuthErr)
	// deployment
	errs = append(errs, c.deploymentClient.Deployments(controller.TargetNamespace).Delete(deployment.Stub().Name, &metav1.DeleteOptions{}))

	return utilerrors.FilterOut(utilerrors.NewAggregate(errs), errors.IsNotFound)
}

// this may need to eventually live under each sync version, depending on if there is
// custom sync logic
func (c *ConsoleOperator) defaultConsole() *consolev1alpha1.Console {
	return &consolev1alpha1.Console{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.ResourceName,
			Namespace: controller.OpenShiftConsoleNamespace,
		},
		Spec: consolev1alpha1.ConsoleSpec{
			OperatorSpec: operatorv1.OperatorSpec{
				// by default the console is managed
				ManagementState: "Managed",
			},
			// one replica is created
			Count: 1,
		},
	}
}
