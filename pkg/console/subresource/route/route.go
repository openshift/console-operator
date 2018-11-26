package route

import (
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
	"github.com/openshift/console-operator/pkg/controller"
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
	meta := util.SharedMeta()
	meta.Name = controller.OpenShiftConsoleShortName
	weight := int32(100)
	route := Stub()
	route.Spec = routev1.RouteSpec{
		To: routev1.RouteTargetReference{
			Kind:   "Service",
			Name:   meta.Name,
			Weight: &weight,
		},
		Port: &routev1.RoutePort{
			TargetPort: intstr.FromString("https"),
		},
		TLS: &routev1.TLSConfig{
			Termination:                   routev1.TLSTerminationReencrypt,
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
		},
		WildcardPolicy: routev1.WildcardPolicyNone,
	}
	util.AddOwnerRef(route, util.OwnerRefFrom(cr))
	return route
}

func Stub() *routev1.Route {
	meta := util.SharedMeta()
	meta.Name = controller.OpenShiftConsoleShortName
	return &routev1.Route{
		ObjectMeta: meta,
	}
}
