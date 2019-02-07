package route

import (
	"reflect"
	"testing"

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
					Name:        api.OpenShiftConsoleShortName,
					Namespace:   api.OpenShiftConsoleNamespace,
					Labels:      map[string]string{"app": api.OpenShiftConsoleName},
					Annotations: map[string]string{},
				},
				Spec: routev1.RouteSpec{
					To: routev1.RouteTargetReference{
						Kind:   "Service",
						Name:   api.OpenShiftConsoleShortName,
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
			if got := DefaultRoute(tt.args.cr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DefaultRoute() = \n%v\n want \n%v", got, tt.want)
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
					Name:                       api.OpenShiftConsoleShortName,
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
			if got := Stub(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Stub() = %v, want %v", got, tt.want)
			}
		})
	}
}
