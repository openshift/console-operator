package route

import (
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/go-test/deep"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	customerrors "github.com/openshift/console-operator/pkg/console/errors"
)

const (
	tlsKey         = "key"
	tlsCertificate = "certificate"
)

func TestGetCanonicalHost(t *testing.T) {
	var (
		validHost = "validhost.com"

		customControllerIngress = []routev1.RouteIngress{
			{
				Host:       validHost,
				RouterName: "custom",
				Conditions: []routev1.RouteIngressCondition{{
					Type:   routev1.RouteAdmitted,
					Status: v1.ConditionTrue,
				}},
			},
		}
		notAdmittedIngress = []routev1.RouteIngress{
			{
				Host:       validHost,
				RouterName: "default",
				Conditions: []routev1.RouteIngressCondition{{
					Type:   routev1.RouteAdmitted,
					Status: v1.ConditionFalse,
				}},
			},
		}
	)
	type args struct {
		route *routev1.Route
	}
	type want struct {
		host string
		err  error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Test admitted route with default controller",
			args: args{
				route: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleRouteName,
					},
					Status: routev1.RouteStatus{
						Ingress: []routev1.RouteIngress{
							{
								Host:       validHost,
								RouterName: "default",
								Conditions: []routev1.RouteIngressCondition{{
									Type:   routev1.RouteAdmitted,
									Status: v1.ConditionTrue,
								}},
							},
						},
					},
				},
			},
			want: want{
				host: "validhost.com",
				err:  nil,
			},
		},
		{
			name: "Test admitted route with custom controller",
			args: args{
				route: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleRouteName,
					},
					Status: routev1.RouteStatus{
						Ingress: customControllerIngress,
					},
				},
			},
			want: want{
				host: "",
				err:  customerrors.NewSyncError(fmt.Sprintf("route %q is not available at canonical host %s", api.OpenShiftConsoleRouteName, customControllerIngress)),
			},
		},
		{
			name: "Test not admitted route with default controller",
			args: args{
				route: &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: api.OpenShiftConsoleRouteName,
					},
					Status: routev1.RouteStatus{
						Ingress: notAdmittedIngress,
					},
				},
			},
			want: want{
				host: "",
				err:  customerrors.NewSyncError(fmt.Sprintf("route %q is not available at canonical host %s", api.OpenShiftConsoleRouteName, notAdmittedIngress)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, err := GetCanonicalHost(tt.args.route)
			if diff := deep.Equal(host, tt.want.host); diff != nil {
				t.Error(diff)
			}
			if diff := deep.Equal(err, tt.want.err); diff != nil {
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
