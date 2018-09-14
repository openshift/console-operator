package console

import (
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)


func newConsoleRoute(cr *v1alpha1.Console) *routev1.Route {
	meta := sharedMeta()
	weight := int32(100)
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			// APIVersion: "route.openshift.io/v1",
			APIVersion: routev1.GroupVersion.String(),
			Kind:       "Route",
		},
		ObjectMeta: meta,
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: meta.Name,
				Weight: &weight,
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("https"),
			},
			TLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
		},
	}
	logrus.Info("Creating console route manifest")
	addOwnerRef(route, ownerRefFrom(cr))
	return route
}