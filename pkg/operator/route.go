package operator

import (
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	routev1 "github.com/openshift/api/route/v1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
)

func newConsoleRoute(cr *v1alpha1.Console) *routev1.Route {
	meta := sharedMeta()
	meta.Name = OpenShiftConsoleShortName
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
		},
	}
	addOwnerRef(route, ownerRefFrom(cr))
	return route
}

func CreateRoute(cr *v1alpha1.Console) (*routev1.Route, error) {
	rt := newConsoleRoute(cr)
	if err := sdk.Create(rt); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console route : %v", err)
		return nil, err
	} else {
		logrus.Infof("created console service '%s'", rt.ObjectMeta.Name)
		return rt, nil
	}
}

func ApplyRoute(cr *v1alpha1.Console) (*routev1.Route, error) {
	rt := newConsoleRoute(cr)
	err := sdk.Get(rt)

	if err != nil {
		return CreateRoute(cr)
	}
	return rt, nil
}

// Deletes the Console Route when the Console ManagementState is set to Removed
func DeleteRoute(cr *v1alpha1.Console) error {
	rt := newConsoleRoute(cr)
	return sdk.Delete(rt)
}
