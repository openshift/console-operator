package route

import (
	"testing"

	"github.com/go-test/deep"

	configv1 "github.com/openshift/api/config/v1"
)

func TestGetDefaultRouteHost(t *testing.T) {
	type args struct {
		routeName     string
		ingressConfig *configv1.Ingress
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test assembling linux amd64 specific URL",
			args: args{
				routeName: "console",
				ingressConfig: &configv1.Ingress{
					Spec: configv1.IngressSpec{
						Domain: "apps.devcluster.openshift.com",
					},
				},
			},
			want: "console-openshift-console.apps.devcluster.openshift.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(GetDefaultRouteHost(tt.args.routeName, tt.args.ingressConfig), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}
