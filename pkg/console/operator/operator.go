package operator

import (
	// standard lib
	"fmt"
	"time"

	// 3rd party
	"github.com/sirupsen/logrus"

	// kube
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1 "k8s.io/client-go/informers/core/v1"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	// openshift
	operatorv1 "github.com/openshift/api/operator/v1"
	oauthclientv1 "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	oauthinformersv1 "github.com/openshift/client-go/oauth/informers/externalversions/oauth/v1"
	routeclientv1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/boilerplate/operator"
	"github.com/openshift/library-go/pkg/operator/events"

	// informers
	routesinformersv1 "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	consolev1 "github.com/openshift/console-operator/pkg/apis/console/v1"
	consoleinformers "github.com/openshift/console-operator/pkg/generated/informers/externalversions/console/v1"
	appsinformersv1 "k8s.io/client-go/informers/apps/v1"

	// clients
	"github.com/openshift/console-operator/pkg/console/subresource/configmap"
	"github.com/openshift/console-operator/pkg/console/subresource/deployment"
	"github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	"github.com/openshift/console-operator/pkg/console/subresource/route"
	"github.com/openshift/console-operator/pkg/console/subresource/secret"
	"github.com/openshift/console-operator/pkg/console/subresource/service"

	// operator
	v1 "github.com/openshift/console-operator/pkg/generated/clientset/versioned/typed/console/v1"
)

const (
	controllerName = "Console"
)

const (
	minimumConsoleReplicas = 3
)

var CreateDefaultConsoleFlag bool

type consoleOperator struct {
	// for a performance sensitive operator, it would make sense to use informers
	// to handle reads and clients to handle writes.  since this operator works
	// on a singleton resource, it has no performance requirements.
	operatorClient v1.ConsoleInterface
	// core kube
	secretsClient    coreclientv1.SecretsGetter
	configMapClient  coreclientv1.ConfigMapsGetter
	serviceClient    coreclientv1.ServicesGetter
	deploymentClient appsv1.DeploymentsGetter
	// openshift
	routeClient routeclientv1.RoutesGetter
	oauthClient oauthclientv1.OAuthClientsGetter
	// recorder
	recorder events.Recorder
}

// the consoleOperator uses specific, strongly-typed clients
// for each resource that it interacts with.
func NewConsoleOperator(
	// informers
	consoles consoleinformers.ConsoleInformer,
	coreV1 corev1.Interface,
	deployments appsinformersv1.DeploymentInformer,
	routes routesinformersv1.RouteInformer,
	oauthClients oauthinformersv1.OAuthClientInformer,
	// clients
	operatorClient v1.ConsolesGetter,
	corev1Client coreclientv1.CoreV1Interface,
	deploymentClient appsv1.DeploymentsGetter,
	routev1Client routeclientv1.RoutesGetter,
	oauthv1Client oauthclientv1.OAuthClientsGetter,
	// recorder
	recorder events.Recorder,
) operator.Runner {
	c := &consoleOperator{
		// operator
		operatorClient: operatorClient.Consoles(api.TargetNamespace),
		// core kube
		secretsClient:    corev1Client,
		configMapClient:  corev1Client,
		serviceClient:    corev1Client,
		deploymentClient: deploymentClient,
		// openshift
		routeClient: routev1Client,
		oauthClient: oauthv1Client,
		// recorder
		recorder: recorder,
	}

	secretsInformer := coreV1.Secrets()
	configMapInformer := coreV1.ConfigMaps()
	serviceInformer := coreV1.Services()

	return operator.New(controllerName, c,
		operator.WithInformer(consoles, operator.FilterByNames(api.ResourceName)),
		operator.WithInformer(deployments, operator.FilterByNames(api.OpenShiftConsoleName)),
		operator.WithInformer(configMapInformer, operator.FilterByNames(configmap.ConsoleConfigMapName, configmap.ServiceCAConfigMapName)),
		operator.WithInformer(secretsInformer, operator.FilterByNames(deployment.ConsoleOauthConfigName)),
		operator.WithInformer(routes, operator.FilterByNames(api.OpenShiftConsoleShortName)),
		operator.WithInformer(serviceInformer, operator.FilterByNames(api.OpenShiftConsoleShortName)),
		operator.WithInformer(oauthClients, operator.FilterByNames(api.OAuthClientName)),
	)
}

// key is actually the pivot point for the operator, which is our Console custom resource
func (c *consoleOperator) Key() (metav1.Object, error) {
	operatorConfig, err := c.operatorClient.Get(api.ResourceName, metav1.GetOptions{})

	if errors.IsNotFound(err) && CreateDefaultConsoleFlag {
		return c.operatorClient.Create(c.defaultConsole())
	}

	return operatorConfig, err
}

func (c *consoleOperator) Sync(obj metav1.Object) error {
	startTime := time.Now()
	logrus.Infof("started syncing operator %q (%v)", obj.GetName(), startTime)
	defer logrus.Infof("finished syncing operator %q (%v) \n\n", obj.GetName(), time.Since(startTime))

	operatorConfig := obj.(*consolev1.Console)

	if err := c.handleSync(operatorConfig); err != nil {
		return err
	}
	return nil
}

func (c *consoleOperator) handleSync(config *consolev1.Console) error {

	switch config.Spec.ManagementState {
	case operatorv1.Managed:
		logrus.Println("console is in a managed state.")
		// handled below
	case operatorv1.Unmanaged:
		logrus.Println("console is in an unmanaged state.")
		return nil
		// take a look @ https://github.com/openshift/service-serving-cert-signer/blob/master/pkg/operator/operator.go#L86
	case operatorv1.Removed:
		logrus.Println("console has been removed.")
		return c.deleteAllResources(config)
	default:
		// TODO should update status
		return fmt.Errorf("unknown state: %v", config.Spec.ManagementState)
	}

	// do we need the if(configChanged) update bits?
	outConfig, configChanged, err := sync_v400(c, config)
	if err != nil {
		return err
	}
	if configChanged {
		// TODO: this should do better apply logic or similar, maybe use SetStatusFromAvailability
		_, err = c.operatorClient.Update(outConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

// this may need to move to sync_v400 if versions ever have custom delete logic
func (c *consoleOperator) deleteAllResources(cr *consolev1.Console) error {
	logrus.Info("deleting console resources")
	defer logrus.Info("finished deleting console resources")
	var errs []error
	// service
	errs = append(errs, c.serviceClient.Services(api.TargetNamespace).Delete(service.Stub().Name, &metav1.DeleteOptions{}))
	// route
	errs = append(errs, c.routeClient.Routes(api.TargetNamespace).Delete(route.Stub().Name, &metav1.DeleteOptions{}))
	// configmap
	errs = append(errs, c.configMapClient.ConfigMaps(api.TargetNamespace).Delete(configmap.Stub().Name, &metav1.DeleteOptions{}))
	// secret
	errs = append(errs, c.secretsClient.Secrets(api.TargetNamespace).Delete(secret.Stub().Name, &metav1.DeleteOptions{}))
	// existingOAuthClient is not a delete, it is a deregister/neutralize
	existingOAuthClient, getAuthErr := c.oauthClient.OAuthClients().Get(oauthclient.Stub().Name, metav1.GetOptions{})
	errs = append(errs, getAuthErr)
	_, updateAuthErr := c.oauthClient.OAuthClients().Update(oauthclient.DeRegisterConsoleFromOAuthClient(existingOAuthClient))
	errs = append(errs, updateAuthErr)
	// deployment
	errs = append(errs, c.deploymentClient.Deployments(api.TargetNamespace).Delete(deployment.Stub().Name, &metav1.DeleteOptions{}))

	return utilerrors.FilterOut(utilerrors.NewAggregate(errs), errors.IsNotFound)
}

// this may need to eventually live under each sync version, depending on if there is
// custom sync logic
func (c *consoleOperator) defaultConsole() *consolev1.Console {
	logrus.Info("creating console CR with default values")

	return &consolev1.Console{
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.ResourceName,
			Namespace: api.OpenShiftConsoleNamespace,
		},
		Spec: consolev1.ConsoleSpec{
			OperatorSpec: operatorv1.OperatorSpec{
				// by default the console is managed
				ManagementState: operatorv1.Managed,
			},
			Count: minimumConsoleReplicas,
		},
	}
}
