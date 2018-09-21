package operator

import (
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
)

// operator.Reconcile(cr)
// at this point we need to do the following:
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
func ReconcileConsole(cr *v1alpha1.Console) {

	CreateService(cr)
	rt, _ := CreateRoute(cr)

	// fetching the route to get it with a host annotation
	_ = sdk.Get(rt)

	CreateConsoleConfigMap(cr, rt)
	CreateOAuthClient(cr, rt)

	CreateConsoleDeployment(cr)

	// ensure these stay in sync.
	// can probably dedupe some work here
	UpdateOauthClient(cr, rt)

}
