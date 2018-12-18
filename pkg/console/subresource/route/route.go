package route

import (
	"github.com/openshift/console-operator/pkg/api"
	// kube
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	// openshift
	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"

	// operator
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

// We can't blindly ApplyRoute() as we need the server to annotate the
// route.Spec.Host, so we need this func
func GetOrCreate(client routeclient.RoutesGetter, required *routev1.Route) (*routev1.Route, bool, error) {
	isNew := false
	existing, err := client.Routes(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		isNew = true
		actual, err := client.Routes(required.Namespace).Create(required)
		return actual, isNew, err
	}
	if err != nil {
		return nil, isNew, err
	}
	return existing, isNew, nil
}

// TODO: ApplyRoute
// - Handle the nuance of ApplyRoute(), noting that Host and perhaps other
//   fields are provided later by the server.  Once we know its correct,
//   PR to library-go so it can live with the other Apply* funcs
func ApplyRoute(client routeclient.RoutesGetter, required *routev1.Route) (*routev1.Route, bool, error) {
	// first, get or create
	existing, err := client.Routes(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.Routes(required.Namespace).Create(required)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	// possibly this should just be a DeepEqual on Spec?
	hostSame := equality.Semantic.DeepEqual(existing.Spec.Host, required.Spec.Host)
	portSame := equality.Semantic.DeepEqual(existing.Spec.Port, required.Spec.Port)
	tlsSame := equality.Semantic.DeepEqual(existing.Spec.TLS, required.Spec.TLS)
	targetSame := equality.Semantic.DeepEqual(existing.Spec.To, required.Spec.To)
	wildcardSame := equality.Semantic.DeepEqual(existing.Spec.WildcardPolicy, required.Spec.WildcardPolicy)
	// if nothing we care about changed, do nothing.  this would be good to
	// PR to library-go and ensure we get it right
	if hostSame && portSame && tlsSame && targetSame && wildcardSame {
		return existing, false, nil
	}

	// TODO:
	// - we dont want to squash host, which is assigned by the server
	// - figure out how to handle this properly, some props are assigned later
	toWrite := existing
	// - CAN we just squash the .Spec here? or is that incorrect? Apply should
	//   be careful, but simple, know nothing about the business logic of the
	//   operator itself.  Therefore, if one does ApplyRoute(someRoute) would they
	//   expect it simply to set this, regardless of what is on the server already?
	//   at this point probably should assume the caller already did a .Get(route)
	//   and merged properties, if that path was desired.
	toWrite.Spec = *required.Spec.DeepCopy()

	actual, err := client.Routes(required.Namespace).Update(toWrite)
	return actual, true, err
}

func DefaultRoute(cr *v1alpha1.Console) *routev1.Route {
	route := Stub()
	route.Spec = routev1.RouteSpec{
		To:             toService(),
		Port:           port(),
		TLS:            tls(),
		WildcardPolicy: wildcard(),
	}
	util.AddOwnerRef(route, util.OwnerRefFrom(cr))
	return route
}

func Stub() *routev1.Route {
	meta := util.SharedMeta()
	meta.Name = api.OpenShiftConsoleShortName
	return &routev1.Route{
		ObjectMeta: meta,
	}
}

// we can't just blindly apply the route, we need the route.Spec.Host
// and we don't want to trigger a sync loop.
// TODO: evaluate metadata.annotations to see what will affect our route in
// an undesirable way:
// - https://docs.openshift.com/container-platform/3.9/architecture/networking/routes.html#alternateBackends
func Validate(route *routev1.Route) (*routev1.Route, bool) {
	changed := false

	if toServiceSame := equality.Semantic.DeepEqual(route.Spec.To, toService()); !toServiceSame {
		changed = true
		route.Spec.To = toService()
	}

	if portSame := equality.Semantic.DeepEqual(route.Spec.Port, port()); !portSame {
		changed = true
		route.Spec.Port = port()
	}

	if tlsSame := equality.Semantic.DeepEqual(route.Spec.TLS, tls()); !tlsSame {
		changed = true
		route.Spec.TLS = tls()
	}

	if wildcardSame := equality.Semantic.DeepEqual(route.Spec.WildcardPolicy, wildcard()); !wildcardSame {
		changed = true
		route.Spec.WildcardPolicy = wildcard()
	}

	return route, changed
}

func routeMeta() metav1.ObjectMeta {
	meta := util.SharedMeta()
	meta.Name = api.OpenShiftConsoleShortName
	return meta
}

func toService() routev1.RouteTargetReference {
	weight := int32(100)
	return routev1.RouteTargetReference{
		Kind:   "Service",
		Name:   routeMeta().Name,
		Weight: &weight,
	}
}

func port() *routev1.RoutePort {
	return &routev1.RoutePort{
		TargetPort: intstr.FromString("https"),
	}
}

func tls() *routev1.TLSConfig {
	return &routev1.TLSConfig{
		Termination:                   routev1.TLSTerminationReencrypt,
		InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
	}
}

func wildcard() routev1.WildcardPolicyType {
	return routev1.WildcardPolicyNone
}
