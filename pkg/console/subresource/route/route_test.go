package route

import (
	"github.com/openshift/console-operator/pkg/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"reflect"
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
)

func TestDefaultRoute(t *testing.T) {
	var (
		weight int32 = 100
	)
	type args struct {
		cr *v1alpha1.Console
	}
	tests := []struct {
		name string
		args args
		want *routev1.Route
	}{
		{
			name: "Test default route",
			args: args{
				cr: &v1alpha1.Console{
					TypeMeta:   v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{},
					Spec:       v1alpha1.ConsoleSpec{},
					Status:     v1alpha1.ConsoleStatus{},
				},
			},
			want: &routev1.Route{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name:      api.OpenShiftConsoleShortName,
					Namespace: api.OpenShiftConsoleName,
					Labels:    map[string]string{"app": api.OpenShiftConsoleName},
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
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name:                       api.OpenShiftConsoleShortName,
					GenerateName:               "",
					Namespace:                  api.OpenShiftConsoleName,
					SelfLink:                   "",
					UID:                        "",
					ResourceVersion:            "",
					Generation:                 0,
					CreationTimestamp:          v1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Labels:          map[string]string{"app": api.OpenShiftConsoleName},
					Annotations:     nil,
					OwnerReferences: nil,
					Initializers:    nil,
					Finalizers:      nil,
					ClusterName:     "",
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
