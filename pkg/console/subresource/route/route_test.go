package route

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-test/deep"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
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

func TestNewRouteConfig(t *testing.T) {
	tests := []struct {
		name           string
		operatorConfig *operatorv1.Console
		ingressConfig  *configv1.Ingress
		routeName      string
		expectedConfig *RouteConfig
	}{
		{
			name: "Custom hostname and TLS secret set",
			operatorConfig: &operatorv1.Console{
				Spec: operatorv1.ConsoleSpec{
					Route: operatorv1.ConsoleConfigRoute{
						Hostname: "custom.example.com",
						Secret:   configv1.SecretNameReference{Name: "custom-tls-secret"},
					},
				},
			},
			ingressConfig: &configv1.Ingress{Spec: configv1.IngressSpec{Domain: "example.com"}},
			routeName:     api.OpenShiftConsoleRouteName,
			expectedConfig: &RouteConfig{
				customRoute: RouteControllerSpec{
					Hostname:   "custom.example.com",
					SecretName: "custom-tls-secret",
				},
				defaultRoute: RouteControllerSpec{
					Hostname: "console-openshift-console.example.com",
				},
				domain:    "example.com",
				routeName: api.OpenShiftConsoleRouteName,
			},
		},
		{
			name:           "No custom hostname or TLS secret",
			operatorConfig: &operatorv1.Console{},
			ingressConfig:  &configv1.Ingress{Spec: configv1.IngressSpec{Domain: "example.com"}},
			routeName:      api.OpenShiftConsoleRouteName,
			expectedConfig: &RouteConfig{
				customRoute:  RouteControllerSpec{},
				defaultRoute: RouteControllerSpec{Hostname: "console-openshift-console.example.com"},
				domain:       "example.com",
				routeName:    api.OpenShiftConsoleRouteName,
			},
		},
		{
			name: "Custom route matches default route",
			operatorConfig: &operatorv1.Console{
				Spec: operatorv1.ConsoleSpec{
					Route: operatorv1.ConsoleConfigRoute{Hostname: "console-openshift-console.example.com"},
				},
			},
			ingressConfig: &configv1.Ingress{Spec: configv1.IngressSpec{Domain: "example.com"}},
			routeName:     api.OpenShiftConsoleRouteName,
			expectedConfig: &RouteConfig{
				customRoute:  RouteControllerSpec{},
				defaultRoute: RouteControllerSpec{Hostname: "console-openshift-console.example.com"},
				domain:       "example.com",
				routeName:    api.OpenShiftConsoleRouteName,
			},
		},
		{
			name:           "Empty Ingress Domain",
			operatorConfig: &operatorv1.Console{},
			ingressConfig:  &configv1.Ingress{Spec: configv1.IngressSpec{Domain: "example.com"}},
			routeName:      api.OpenShiftConsoleRouteName,
			expectedConfig: &RouteConfig{
				domain:       "example.com",
				routeName:    api.OpenShiftConsoleRouteName,
				defaultRoute: RouteControllerSpec{Hostname: "console-openshift-console.example.com"},
			},
		},
		{
			name:           "Operator Config Without Hostname But Ingress Config Provides a Custom Route",
			operatorConfig: &operatorv1.Console{},
			ingressConfig: &configv1.Ingress{
				Spec: configv1.IngressSpec{
					Domain: "example.com",
					ComponentRoutes: []configv1.ComponentRouteSpec{
						{Name: api.OpenShiftConsoleRouteName, Hostname: "ingress-provided.example.com"},
					},
				},
			},
			routeName: api.OpenShiftConsoleRouteName,
			expectedConfig: &RouteConfig{
				domain:       "example.com",
				routeName:    api.OpenShiftConsoleRouteName,
				defaultRoute: RouteControllerSpec{Hostname: "console-openshift-console.example.com"},
			},
		},
	}

	for _, tt := range tests {
		var errors []string
		t.Run(tt.name, func(t *testing.T) {
			config := NewRouteConfig(tt.operatorConfig, tt.ingressConfig, tt.routeName)

			if diff := deep.Equal(tt.expectedConfig.domain, config.GetDomain()); diff != nil {
				errors = append(errors, fmt.Sprintf("Domain mismatch: %v\n", diff))
			}
			if diff := deep.Equal(tt.expectedConfig.routeName, config.GetRouteName()); diff != nil {
				errors = append(errors, fmt.Sprintf("Route domain mismatch: %v\n", diff))
			}

			if diff := deep.Equal(tt.expectedConfig.defaultRoute, config.GetDefaultRoute()); diff != nil {
				errors = append(errors, fmt.Sprintf("Default route mismatch: %v\n", diff))
			}

			if diff := deep.Equal(tt.expectedConfig.customRoute, config.GetCustomRoute()); diff != nil {
				errors = append(errors, fmt.Sprintf("Custom route mismatch: %v\n", diff))
			}

			if len(errors) > 0 {
				t.Errorf("RouteConfig mismatch in %s test case: %v", tt.name, strings.Join(errors, ", "))
			}
		})
	}
}
