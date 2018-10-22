package operator

import (
	"k8s.io/apimachinery/pkg/api/errors"
	errutil "k8s.io/apimachinery/pkg/util/errors"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
)

// operator.Reconcile(cr)
// so ignore "resource exists", GET the resource, diff against expected, if not, UPDATE resource, loop.
//   it shouldn't loop infintely, however, at some point it ought to idle if things aren't changing
//   (until the next watch event fires)
// process should:
//   - burst when it is first reconciling to get everything into correct state
//   - update & reconcile only when things change. if no monkey business, should be idle
//   - wake up every <resyncPeriod> in main.go and do a reconcile again, just as a check
//   - note that API calls are expensive, so don't make them without good reason
// reconcile ought to do the following:
//   create deployment if not exists
//   create service if not exists
//   create route if not exists
//   create configmap if not exists
//   create oauthclient if not exists
// 		which will look something like this:
//        sdk.Get(the-client)
//        if !exists
//          sdk.Get(the-route)
//          addRouteHostIfWeGotIt(the-client)
//          sdk.Create(the-client)
//        else
//          sdk.Get(the-route)
//          addRouteHostIfWeGotIt(the-client)
//          sdk.Update(the-client)
//   create oauthclient-secret if not exists
// but also
//   sync random secret between oauthclient & oauthclient-secret
//   sync route.host between route, oauthclient.redirectURIs & configmap.baseAddress
func ReconcileConsole(cr *v1alpha1.Console) error {

	if _, err := ApplyService(cr); err != nil {
		return err
	}

	rt, err := ApplyRoute(cr)
	if err != nil {
		return err
	}

	if _, err := ApplyConfigMap(cr, rt); err != nil {
		return err
	}

	if _, err := ApplyDeployment(cr); err != nil {
		return err
	}

	if _, _, err := ApplyOAuth(cr, rt); err != nil {
		return err
	}
	return nil
}

func DeleteAllResources(cr *v1alpha1.Console) error {
	var errs []error
	for _, fn := range []func(*v1alpha1.Console) error{
		DeleteService,
		DeleteRoute,
		DeleteConfigMap,
		DeleteDeployment,
		DeleteOAuthSecret,
		// we don't own it and can't create or delete it. however, we can update it
		NeutralizeOAuthClient,
	} {
		errs = append(errs, fn(cr))
	}
	return errutil.FilterOut(errutil.NewAggregate(errs), errors.IsNotFound)
}
