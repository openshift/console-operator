package route

import (
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/go-test/deep"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
)

func TestDefaultRoute(t *testing.T) {
	var (
		weight int32 = 100
	)
	type args struct {
		cr *operatorv1.Console
	}
	tests := []struct {
		name string
		args args
		want *routev1.Route
	}{
		{
			name: "Test default route",
			args: args{
				cr: &operatorv1.Console{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec:       operatorv1.ConsoleSpec{},
					Status:     operatorv1.ConsoleStatus{},
				},
			},
			want: &routev1.Route{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					To: routev1.RouteTargetReference{
						Kind:   "Service",
						Name:   api.OpenShiftConsoleName,
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
				Status: routev1.RouteStatus{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(DefaultRoute(tt.args.cr), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestStub(t *testing.T) {
	tests := []struct {
		name string
		want *routev1.Route
	}{
		{
			name: "Test stubbing out route",
			want: &routev1.Route{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:                       api.OpenShiftConsoleName,
					GenerateName:               "",
					Namespace:                  api.OpenShiftConsoleNamespace,
					SelfLink:                   "",
					UID:                        "",
					ResourceVersion:            "",
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:                     map[string]string{"app": api.OpenShiftConsoleName},
					Annotations:                map[string]string{},
					OwnerReferences:            nil,
					Initializers:               nil,
					Finalizers:                 nil,
					ClusterName:                "",
				},
				Spec:   routev1.RouteSpec{},
				Status: routev1.RouteStatus{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(Stub(), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestIsAdmitted(t *testing.T) {
	type args struct {
		route *routev1.Route
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Test IsAdmitted(): Route has an ingress with a matching host with an admission status of true.",
			args: args{
				route: &routev1.Route{
					Spec: routev1.RouteSpec{
						Host: "usethisone.com",
					},
					Status: routev1.RouteStatus{
						Ingress: []routev1.RouteIngress{
							{
								Host: "usethisone.com",
								Conditions: []routev1.RouteIngressCondition{{
									Type:   routev1.RouteAdmitted,
									Status: v1.ConditionTrue,
								}},
							},
							{
								Host: "notthisone.com",
								Conditions: []routev1.RouteIngressCondition{{
									Type:   routev1.RouteAdmitted,
									Status: v1.ConditionFalse,
								}},
							},
						},
					},
				},
			},
			// we do have a matching ingress that has been admitted
			want: true,
		},
		{
			name: "Test IsAdmitted(): Route has an ingress with a matching host but with an admission status of false.",
			args: args{
				route: &routev1.Route{
					Spec: routev1.RouteSpec{
						Host: "usethisone.com",
					},
					Status: routev1.RouteStatus{
						Ingress: []routev1.RouteIngress{
							{
								Host: "usethisone.com",
								Conditions: []routev1.RouteIngressCondition{{
									Type:   routev1.RouteAdmitted,
									Status: v1.ConditionFalse,
								}},
							},
							{
								Host: "notthisone.com",
								Conditions: []routev1.RouteIngressCondition{{
									Type:   routev1.RouteAdmitted,
									Status: v1.ConditionFalse,
								}},
							},
						},
					},
				},
			},
			// in this case, the function should return false
			want: false,
		}, {
			name: "Test IsAdmitted(): Route has no matching ingress.",
			args: args{
				route: &routev1.Route{
					Spec: routev1.RouteSpec{
						Host: "usethisone.com",
					},
					Status: routev1.RouteStatus{
						Ingress: []routev1.RouteIngress{},
					},
				},
			},
			// no matching ingress, so we should return false
			want: false,
		}, {
			name: "Test IsAdmitted(): Route has no ingress with a matching host.",
			args: args{
				route: &routev1.Route{
					Spec: routev1.RouteSpec{
						Host: "usethisone.com",
					},
					Status: routev1.RouteStatus{
						Ingress: []routev1.RouteIngress{
							{
								Host: "nope.com",
								Conditions: []routev1.RouteIngressCondition{{
									Type:   routev1.RouteAdmitted,
									Status: v1.ConditionFalse,
								}},
							},
							{
								Host: "nopenope.com",
								Conditions: []routev1.RouteIngressCondition{{
									Type:   routev1.RouteAdmitted,
									Status: v1.ConditionFalse,
								}},
							},
						},
					},
				},
			},
			// no ingress with a matching host, so we should return false
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAdmitted(tt.args.route); got != tt.want {
				t.Errorf("IsAdmitted() = \n%v\n want \n%v", got, tt.want)
			}
		})
	}
}
