package route

import (
	"fmt"

	"github.com/openshift/console-operator/pkg/console/subresource/route"

	operatorsv1 "github.com/openshift/api/operator/v1"
	klog "github.com/openshift/console-operator/_output/tools/src/pkg/mod/k8s.io/klog@v0.2.0"

	// k8s
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// 3rd party
	"monis.app/go/openshift/operator"

	// openshift
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	v1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	routeclientv1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/openshift/library-go/pkg/operator/events"

	// console-operator
	"github.com/openshift/console-operator/pkg/api"
)

const (
	// key is basically irrelevant
	controllerWorkQueueKey = "route-sync-work-queue-key"
	controllerName         = "ConsoleRouteSyncController"
)

type RouteSyncController struct {
	// clients
	operatorConfigClient operatorclientv1.ConsoleInterface
	routeClient          routeclientv1.RoutesGetter
	// names
	targetNamespace string
	routeName       string
	// events
	recorder events.Recorder
}

func NewRouteSyncController(
	operatorConfigClient operatorclientv1.ConsoleInterface,
	operatorConfigInformer v1.ConsoleInformer,
	// TODO: route client...
	// names
	targetNamespace string,
	routeName string,
	// events
	recorder events.Recorder,
) operator.Runner {
	c := &RouteSyncController{
		operatorConfigClient: operatorConfigClient,
		routeClient:          nil, // TODO
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
func (c *RouteSyncController) Key() (metav1.Object, error) {
	return c.operatorConfigClient.Get(api.ConfigResourceName, metav1.GetOptions{})
}

func (c *RouteSyncController) Sync(obj metav1.Object) error {
	startTime := time.Now()
	klog.V(4).Infof("started syncing route %q (%v)", c.routeName, startTime)
	defer klog.V(4).Infof("finished syncing route %q (%v)", c.routeName, time.Since(startTime))

	// we need to cast the operator config
	operatorConfig := obj.(*operatorsv1.Console)
	// TODO!
	if err := c.handleSync(operatorConfig); err != nil {
		return err
	}

	return nil
}

func (c *RouteSyncController) handleSync(config *operatorsv1.Console) error {

	switch config.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infoln("console is in a managed state: syncing service")
	case operatorsv1.Unmanaged:
		klog.V(4).Infoln("console is in an unmanaged state: skipping service sync")
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infoln("console is in a removed state: deleting service")
		return c.removeRoute()
	default:
		return fmt.Errorf("unknown state: %v", config.Spec.ManagementState)
	}

	// TODO: now sync the route!
}

func (c *RouteSyncController) removeRoute() error {
	klog.V(2).Info("deleting console route")
	defer klog.V(2).Info("finished deleting console route")
	return c.routeClient.Routes(c.targetNamespace).Delete(route.Stub().Name, &metav1.DeleteOptions{})
}
