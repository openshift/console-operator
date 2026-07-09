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

func TestGetAdditionalComponentRouteSpecs(t *testing.T) {
	tests := []struct {
		name          string
		ingressConfig *configv1.Ingress
		wantNames     []string
	}{
		{
			name: "no component routes",
			ingressConfig: &configv1.Ingress{
				Spec: configv1.IngressSpec{},
			},
			wantNames: nil,
		},
		{
			name: "only known routes excluded",
			ingressConfig: &configv1.Ingress{
				Spec: configv1.IngressSpec{
					ComponentRoutes: []configv1.ComponentRouteSpec{
						{Namespace: api.OpenShiftConsoleNamespace, Name: "console", Hostname: "console.example.com"},
						{Namespace: api.OpenShiftConsoleNamespace, Name: "downloads", Hostname: "downloads.example.com"},
					},
				},
			},
			wantNames: nil,
		},
		{
			name: "additional routes returned",
			ingressConfig: &configv1.Ingress{
				Spec: configv1.IngressSpec{
					ComponentRoutes: []configv1.ComponentRouteSpec{
						{Namespace: api.OpenShiftConsoleNamespace, Name: "console", Hostname: "console.example.com"},
						{Namespace: api.OpenShiftConsoleNamespace, Name: "console-web", Hostname: "console-web.example.com"},
						{Namespace: api.OpenShiftConsoleNamespace, Name: "console-internal", Hostname: "console.internal.example.com"},
					},
				},
			},
			wantNames: []string{"console-web", "console-internal"},
		},
		{
			name: "different namespace excluded",
			ingressConfig: &configv1.Ingress{
				Spec: configv1.IngressSpec{
					ComponentRoutes: []configv1.ComponentRouteSpec{
						{Namespace: "openshift-authentication", Name: "oauth-openshift", Hostname: "oauth.example.com"},
						{Namespace: api.OpenShiftConsoleNamespace, Name: "console-extra", Hostname: "extra.example.com"},
					},
				},
			},
			wantNames: []string{"console-extra"},
		},
		{
			name: "custom route names excluded",
			ingressConfig: &configv1.Ingress{
				Spec: configv1.IngressSpec{
					ComponentRoutes: []configv1.ComponentRouteSpec{
						{Namespace: api.OpenShiftConsoleNamespace, Name: "console-custom", Hostname: "custom.example.com"},
						{Namespace: api.OpenShiftConsoleNamespace, Name: "downloads-custom", Hostname: "dl-custom.example.com"},
						{Namespace: api.OpenShiftConsoleNamespace, Name: "console-secondary", Hostname: "secondary.example.com"},
					},
				},
			},
			wantNames: []string{"console-secondary"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs := GetAdditionalComponentRouteSpecs(tt.ingressConfig)
			var gotNames []string
			for _, s := range specs {
				gotNames = append(gotNames, string(s.Name))
			}
			if diff := deep.Equal(gotNames, tt.wantNames); diff != nil {
				t.Errorf("GetAdditionalComponentRouteSpecs() names mismatch: %v", diff)
			}
		})
	}
}

func TestGetAdditionalRouteHostnames(t *testing.T) {
	ingressConfig := &configv1.Ingress{
		Spec: configv1.IngressSpec{
			ComponentRoutes: []configv1.ComponentRouteSpec{
				{Namespace: api.OpenShiftConsoleNamespace, Name: "console", Hostname: "console.example.com"},
				{Namespace: api.OpenShiftConsoleNamespace, Name: "console-web", Hostname: "console-web.example.com"},
				{Namespace: api.OpenShiftConsoleNamespace, Name: "console-internal", Hostname: "console.internal.example.com"},
			},
		},
	}
	got := GetAdditionalRouteHostnames(ingressConfig)
	want := []string{"console-web.example.com", "console.internal.example.com"}
	if diff := deep.Equal(got, want); diff != nil {
		t.Errorf("GetAdditionalRouteHostnames() mismatch: %v", diff)
	}
}

func TestAdditionalRoute(t *testing.T) {
	spec := configv1.ComponentRouteSpec{
		Name:      "console-secondary",
		Namespace: api.OpenShiftConsoleNamespace,
		Hostname:  "console-secondary.example.com",
	}
	route := AdditionalRoute(spec, nil)

	if route.Name != "console-secondary" {
		t.Errorf("expected route name %q, got %q", "console-secondary", route.Name)
	}
	if route.Spec.Host != "console-secondary.example.com" {
		t.Errorf("expected route host %q, got %q", "console-secondary.example.com", route.Spec.Host)
	}
	if route.Labels[AdditionalRouteLabel] != "true" {
		t.Errorf("expected label %q to be %q, got %q", AdditionalRouteLabel, "true", route.Labels[AdditionalRouteLabel])
	}
	if route.Spec.To.Name != api.OpenShiftConsoleServiceName {
		t.Errorf("expected route to point to service %q, got %q", api.OpenShiftConsoleServiceName, route.Spec.To.Name)
	}
	if route.Spec.TLS.Termination != "reencrypt" {
		t.Errorf("expected TLS termination reencrypt, got %q", route.Spec.TLS.Termination)
	}
}
