package operator

import (
	// standard lib
	"fmt"
	"time"

	// 3rd party
	"github.com/golang/glog"
	"github.com/sirupsen/logrus"

	// kube
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	corev1 "k8s.io/client-go/informers/core/v1"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	oauthclientv1 "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	oauthinformersv1 "github.com/openshift/client-go/oauth/informers/externalversions/oauth/v1"
	routeclientv1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/boilerplate/operator"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	// informers
	configinformerv1 "github.com/openshift/client-go/config/informers/externalversions/config/v1"
	operatorinformerv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"

	routesinformersv1 "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	appsinformersv1 "k8s.io/client-go/informers/apps/v1"

	// clients
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"

	// operator
	"github.com/openshift/console-operator/pkg/console/subresource/configmap"
	"github.com/openshift/console-operator/pkg/console/subresource/deployment"
	"github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	"github.com/openshift/console-operator/pkg/console/subresource/route"
	"github.com/openshift/console-operator/pkg/console/subresource/secret"
	"github.com/openshift/console-operator/pkg/console/subresource/service"
)

const (
	controllerName           = "Console"
	workloadFailingCondition = "WorkloadFailing"
)

var CreateDefaultConsoleFlag bool

type consoleOperator struct {
	operatorConfigClient operatorclientv1.ConsoleInterface
	consoleConfigClient  configclientv1.ConsoleInterface
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

func NewConsoleOperator(
	// informers
	operatorConfigInformer operatorinformerv1.ConsoleInformer,
	consoleConfigInformer configinformerv1.ConsoleInformer,
	coreV1 corev1.Interface,
	deployments appsinformersv1.DeploymentInformer,
	routes routesinformersv1.RouteInformer,
	oauthClients oauthinformersv1.OAuthClientInformer,

	// clients
	operatorConfigClient operatorclientv1.OperatorV1Interface,
	consoleConfigClient configclientv1.ConfigV1Interface,
	corev1Client coreclientv1.CoreV1Interface,
	deploymentClient appsv1.DeploymentsGetter,
	routev1Client routeclientv1.RoutesGetter,
	oauthv1Client oauthclientv1.OAuthClientsGetter,
	// recorder
	recorder events.Recorder,
) operator.Runner {
	c := &consoleOperator{
		// operator
		operatorConfigClient: operatorConfigClient.Consoles(),
		consoleConfigClient:  consoleConfigClient.Consoles(),
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
		operator.WithInformer(operatorConfigInformer, operator.FilterByNames(api.ConfigResourceName)),
		operator.WithInformer(consoleConfigInformer, operator.FilterByNames(api.ConfigResourceName)),
		operator.WithInformer(deployments, operator.FilterByNames(api.OpenShiftConsoleName)),
		operator.WithInformer(configMapInformer, operator.FilterByNames(configmap.ConsoleConfigMapName, configmap.ServiceCAConfigMapName)),
		operator.WithInformer(secretsInformer, operator.FilterByNames(deployment.ConsoleOauthConfigName)),
		operator.WithInformer(routes, operator.FilterByNames(api.OpenShiftConsoleName)),
		operator.WithInformer(serviceInformer, operator.FilterByNames(api.OpenShiftConsoleName)),
		operator.WithInformer(oauthClients, operator.FilterByNames(api.OAuthClientName)),
	)
}

// key is actually the pivot point for the operator, which is our Console custom resource
func (c *consoleOperator) Key() (metav1.Object, error) {
	operatorConfig, err := c.operatorConfigClient.Get(api.ConfigResourceName, metav1.GetOptions{})
	if errors.IsNotFound(err) && CreateDefaultConsoleFlag {
		if _, err := c.operatorConfigClient.Create(c.defaultConsoleOperatorConfig()); err != nil {
			logrus.Errorf("No console operator config found. Creating. %v \n", err)
			return nil, err
		}
	}

	return operatorConfig, err
}

func (c *consoleOperator) Sync(obj metav1.Object) error {
	startTime := time.Now()
	logrus.Infof("started syncing operator %q (%v)", obj.GetName(), startTime)
	defer logrus.Infof("finished syncing operator %q (%v) \n\n", obj.GetName(), time.Since(startTime))

	operatorConfig := obj.(*operatorsv1.Console)

	consoleConfig, err := c.consoleConfigClient.Get(api.ConfigResourceName, metav1.GetOptions{})
	if errors.IsNotFound(err) && CreateDefaultConsoleFlag {
		if _, err := c.consoleConfigClient.Create(c.defaultConsoleConfig()); err != nil {
			logrus.Errorf("No console config found. Creating. %v \n", err)
			return err
		}
	}

	if err := c.handleSync(operatorConfig, consoleConfig); err != nil {
		v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
			Type:               operatorsv1.OperatorStatusTypeFailing,
			Status:             operatorsv1.ConditionTrue,
			Reason:             "OperatorSyncLoopError",
			Message:            err.Error(),
			LastTransitionTime: metav1.Now(),
		})
		if _, updateErr := c.operatorConfigClient.UpdateStatus(operatorConfig); updateErr != nil {
			glog.Errorf("error updating status: %s", err)
		}
		return err
	}

	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeAvailable,
		Status:             operatorsv1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	})
	return nil
}

func (c *consoleOperator) handleSync(operatorConfig *operatorsv1.Console, consoleConfig *configv1.Console) error {

	originalOperatorConfig := operatorConfig.DeepCopy()
	switch operatorConfig.Spec.ManagementState {
	case operatorsv1.Managed:
		logrus.Println("console is in a managed state.")
		// handled below
	case operatorsv1.Unmanaged:
		logrus.Println("console is in an unmanaged state.")
		v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
			Type:               operatorsv1.OperatorStatusTypeAvailable,
			Status:             operatorsv1.ConditionUnknown,
			Reason:             "Unmanaged",
			Message:            "the controller manager is in an unmanaged state, therefore its availability is unknown.",
			LastTransitionTime: metav1.Now(),
		})
		v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
			Type:               operatorsv1.OperatorStatusTypeProgressing,
			Status:             operatorsv1.ConditionFalse,
			Reason:             "Unmanaged",
			Message:            "the controller manager is in an unmanaged state, therefore no changes are being applied.",
			LastTransitionTime: metav1.Now(),
		})
		v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
			Type:               operatorsv1.OperatorStatusTypeFailing,
			Status:             operatorsv1.ConditionFalse,
			Reason:             "Unmanaged",
			Message:            "the controller manager is in an unmanaged state, therefore no operator actions are failing.",
			LastTransitionTime: metav1.Now(),
		})
		if !equality.Semantic.DeepEqual(operatorConfig.Status, originalOperatorConfig.Status) {
			if _, err := c.operatorConfigClient.UpdateStatus(operatorConfig); err != nil {
				return err
			}
		}
		return nil
	case operatorsv1.Removed:
		logrus.Println("console has been removed.")
		return c.deleteAllResources(operatorConfig)
	default:
		// TODO should update status
		return fmt.Errorf("unknown state: %v", operatorConfig.Spec.ManagementState)
	}

	_, _, _, err := sync_v400(c, operatorConfig, consoleConfig)
	if err != nil {
		return err
	}

	// TODO: these should probably be handled separately
	// if configChanged {
	// 	// TODO: this should do better apply logic or similar, maybe use SetStatusFromAvailability
	// 	if _, err = c.operatorConfigClient.Update(operatorConfigOut); err != nil {
	// 		return err
	// 	}

	// 	if _, err = c.consoleConfigClient.Update(consoleConfigOut); err != nil {
	// 		return err
	// 	}
	// }
	return nil
}

// this may need to move to sync_v400 if versions ever have custom delete logic
func (c *consoleOperator) deleteAllResources(cr *operatorsv1.Console) error {
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

// see https://github.com/openshift/api/blob/master/config/v1/types_console.go
func (c *consoleOperator) defaultConsoleConfig() *configv1.Console {
	return &configv1.Console{
		ObjectMeta: metav1.ObjectMeta{
			Name: api.ConfigResourceName,
		},
	}
}

// see https://github.com/openshift/api/blob/master/operator/v1/types_console.go
func (c *consoleOperator) defaultConsoleOperatorConfig() *operatorsv1.Console {
	return &operatorsv1.Console{
		ObjectMeta: metav1.ObjectMeta{
			Name: api.ConfigResourceName,
		},
		Spec: operatorsv1.ConsoleSpec{
			OperatorSpec: operatorsv1.OperatorSpec{
				ManagementState: operatorsv1.Managed,
			},
		},
	}
}
