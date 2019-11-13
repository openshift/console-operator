package route

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
	routev1 "github.com/openshift/api/route/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	v1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	routeclientv1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/openshift/library-go/pkg/operator/events"

	// console-operator
	"github.com/openshift/console-operator/pkg/api"
	customerrors "github.com/openshift/console-operator/pkg/console/errors"
	"github.com/openshift/console-operator/pkg/console/status"
	"github.com/openshift/console-operator/pkg/console/subresource/route"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
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
		klog.V(4).Infoln("console is in a managed state: syncing route")
	case operatorsv1.Unmanaged:
		klog.V(4).Infoln("console is in an unmanaged state: skipping route sync")
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infoln("console is in a removed state: deleting route")
		return c.removeRoute()
	default:
		return fmt.Errorf("unknown state: %v", config.Spec.ManagementState)
	}

	// TODO: now sync the route!
	updatedOperatorConfig := config.DeepCopy()

	_, _, rtErrReason, rtErr := c.SyncRoute(updatedOperatorConfig)

	// TODO: do we need the "toUpdate" bool?

	status.HandleProgressingOrDegraded(updatedOperatorConfig, "RouteSync", rtErrReason, rtErr)
	status.SyncStatus(c.operatorConfigClient, updatedOperatorConfig)

}

func (c *RouteSyncController) removeRoute() error {
	klog.V(2).Info("deleting console route")
	defer klog.V(2).Info("finished deleting console route")
	return c.routeClient.Routes(c.targetNamespace).Delete(route.Stub().Name, &metav1.DeleteOptions{})
}

// apply route
// - be sure to test that we don't trigger an infinite loop by stomping on the
//   default host name set by the server, or any other values. The ApplyRoute()
//   logic will have to be sound.
// - update to ApplyRoute() once the logic is settled
func (co *RouteSyncController) SyncRoute(operatorConfig *operatorsv1.Console) (consoleRoute *routev1.Route, isNew bool, reason string, err error) {
	// ensure we have a route. any error returned is a non-404 error

	rt, rtIsNew, rtErr := routesub.GetOrCreate(co.routeClient, routesub.DefaultRoute(operatorConfig))
	if rtErr != nil {
		return nil, false, "FailedCreate", rtErr
	}

	// we will not proceed until the route is valid. this eliminates complexity with the
	// configmap, secret & oauth client as they can be certain they have a host if we pass this point.
	host := routesub.GetCanonicalHost(rt)
	if len(host) == 0 {
		return nil, false, "FailedHost", customerrors.NewSyncError(fmt.Sprintf("route is not available at canonical host %s", rt.Status.Ingress))
	}

	if validatedRoute, changed := routesub.Validate(rt); changed {
		// if validation changed the route, issue an update
		if _, err := co.routeClient.Routes(api.TargetNamespace).Update(validatedRoute); err != nil {
			// error is unexpected, this is a real error
			return nil, false, "InvalidRouteCorrection", err
		}
		// abort sync, route changed, let it settle & retry
		return nil, true, "InvalidRoute", customerrors.NewSyncError("route is invalid, correcting route state")
	}
	// only return the route if it is valid with a host
	return rt, rtIsNew, "", rtErr
}
