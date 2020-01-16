package oauthconnect

import (
	"fmt"
	"time"

	// 3rd party
	"monis.app/go/openshift/operator"

	// k8s
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	// openshift
	operatorsv1 "github.com/openshift/api/operator/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	v1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"

	// console-operator
	"github.com/openshift/console-operator/pkg/api"
)

const (
	controllerName = "ConsoleOauthConnectController"
)

// This controller is responsible for monitoring the ability for the
// console pods to connect to the oauth client.  It does this simply
// by hitting the console health/oauthconnect endpoint.  The console
// will make a request to the oauthclient /healthz endpoint.  It expects
// a 200 ok response.  The operator will report status based in the response.
//
// The operator will hit two endpoints to provide clarity around the
// oauth connection:
// - console /health (verify the route to console is ok and the pod responds)
// - console /health/oauthconnect (verifies the route to oauth is ok and the pod responds via its /healthz endpoint)
//
// The console-operator could directly request the oauth /healthz, but the proxy
// through console ensures that pods running on different nodes do not deliver a
// false positive.
//
// TODO: what is needed for this to work correctly?
// - we should watch the console deployment for changes
// - we should watch our oauth client for changes
// - anything else?
type ConsoleOauthConnectController struct {
	// clients
	operatorConfigClient operatorclientv1.ConsoleInterface
	// names
	targetNamespace string
	routeName       string
	// events
	recorder events.Recorder
}

func NewConsoleOauthConnectController(
	operatorConfigClient operatorclientv1.ConsoleInterface,
	operatorConfigInformer v1.ConsoleInformer,
	// names
	targetNamespace string,
	routeName string,
	// events
	recorder events.Recorder,
) operator.Runner {

	c := &ConsoleOauthConnectController{
		operatorConfigClient: operatorConfigClient,
		targetNamespace:      targetNamespace,
		routeName:            routeName,
		recorder:             recorder,
	}

	configNameFilter := operator.FilterByNames(api.ConfigResourceName)

	return operator.New(controllerName, c,

		operator.WithInformer(operatorConfigInformer, configNameFilter),
	)
}

// key is actually the pivot point for the operator, which is our Console custom resource
func (c *ConsoleOauthConnectController) Key() (metav1.Object, error) {
	return c.operatorConfigClient.Get(api.ConfigResourceName, metav1.GetOptions{})
}

func (c *ConsoleOauthConnectController) Sync(obj metav1.Object) error {
	startTime := time.Now()
	klog.V(4).Infof("started syncing route %q (%v)", c.routeName, startTime)
	defer klog.V(4).Infof("finished syncing route %q (%v)", c.routeName, time.Since(startTime))

	// we need to cast the operator config
	operatorConfig := obj.(*operatorsv1.Console)
	if err := c.handleSync(operatorConfig); err != nil {
		return err
	}

	return nil
}

func (c *ConsoleOauthConnectController) handleSync(config *operatorsv1.Console) error {

	switch config.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infoln("console is in a managed state: syncing route")
	case operatorsv1.Unmanaged:
		klog.V(4).Infoln("console is in an unmanaged state: skipping route sync")
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infoln("console is in a removed state: deleting route")
		// TODO: anything we need to do here? this controller may have no actual managed resources...
		return nil
	default:
		return fmt.Errorf("unknown state: %v", config.Spec.ManagementState)
	}

	updatedOperatorConfig := config.DeepCopy()

	// TODO: we need to do something here!
	// foo, err := c.SyncSomethingHealthRelated()

	// TODO: then we need to update health.
	// status.HandleProgressingOrDegraded(updatedOperatorConfig, "RouteSync", rtErrReason, rtErr)
	// status.SyncStatus(c.operatorConfigClient, updatedOperatorConfig)
}
