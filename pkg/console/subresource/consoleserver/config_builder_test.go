package consoleserver

import (
	"strconv"
	"testing"

	"github.com/go-test/deep"
	"github.com/google/go-cmp/cmp"
	configv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
)

// Tests that the builder will return a correctly structured
// Console Server Config struct when builder.Config() is called
func TestConsoleServerCLIConfigBuilder(t *testing.T) {
	tests := []struct {
		name   string
		input  func() Config
		output Config
	}{
		{
			name: "Config builder should return default config if given no inputs",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				return b.Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Customization: Customization{
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{},
			},
		}, {
			name: "Config builder should handle customization with LightspeedButton capability",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				return b.
					Capabilities([]v1.Capability{
						{
							Name: v1.LightspeedButton,
							Visibility: v1.CapabilityVisibility{
								State: v1.CapabilityEnabled,
							},
						},
					}).
					Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Customization: Customization{
					Capabilities: []v1.Capability{
						{
							Name: v1.LightspeedButton,
							Visibility: v1.CapabilityVisibility{
								State: v1.CapabilityEnabled,
							},
						},
					},
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{},
			},
		}, {
			name: "Config builder should handle cluster info with internal OAuth",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				return b.
					APIServerURL("https://foobar.com/api").
					Host("https://foobar.com/host").
					LogoutURL("https://foobar.com/logout").
					AuthConfig(&configv1.Authentication{Spec: configv1.AuthenticationSpec{Type: ""}}, "").
					Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath:    "",
					ConsoleBaseAddress: "https://foobar.com/host",
					MasterPublicURL:    "https://foobar.com/api",
				},
				Auth: Auth{
					AuthType:            "openshift",
					ClientID:            api.OpenShiftConsoleName,
					ClientSecretFile:    clientSecretFilePath,
					OAuthEndpointCAFile: "/var/oauth-serving-cert/ca-bundle.crt",
					LogoutRedirect:      "https://foobar.com/logout",
				},
				Customization: Customization{
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{},
			},
		}, {
			name: "Config builder should handle cluster info with external OIDC",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				return b.
					APIServerURL("https://foobar.com/api").
					Host("https://foobar.com/host").
					LogoutURL("https://foobar.com/logout").
					AuthConfig(&configv1.Authentication{
						Spec: configv1.AuthenticationSpec{
							Type: "OIDC",
							OIDCProviders: []configv1.OIDCProvider{
								{
									Issuer: configv1.TokenIssuer{
										CertificateAuthority: configv1.ConfigMapNameReference{
											Name: "auth-server-ca",
										},
									},
									OIDCClients: []configv1.OIDCClientConfig{
										{ComponentName: "console", ComponentNamespace: "openshift-console"},
									},
								},
							},
						},
					},
						"").
					Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath:    "",
					ConsoleBaseAddress: "https://foobar.com/host",
					MasterPublicURL:    "https://foobar.com/api",
				},
				Auth: Auth{
					AuthType:            "oidc",
					ClientID:            api.OpenShiftConsoleName,
					ClientSecretFile:    clientSecretFilePath,
					OAuthEndpointCAFile: "/var/auth-server-ca/ca-bundle.crt",
					LogoutRedirect:      "https://foobar.com/logout",
				},
				Session: Session{
					CookieEncryptionKeyFile:     "/var/session-secret/sessionEncryptionKey",
					CookieAuthenticationKeyFile: "/var/session-secret/sessionAuthenticationKey",
				},
				Customization: Customization{
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{},
			},
		}, {
			name: "Config builder should handle monitoring and info",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				return b.
					APIServerURL("https://foobar.com/api").
					Host("https://foobar.com/host").
					LogoutURL("https://foobar.com/logout").
					AuthConfig(&configv1.Authentication{
						Spec: configv1.AuthenticationSpec{
							Type: "OIDC",
							OIDCProviders: []configv1.OIDCProvider{
								{
									Issuer: configv1.TokenIssuer{
										CertificateAuthority: configv1.ConfigMapNameReference{
											Name: "auth-server-ca",
										},
									},
								},
							},
						},
					}, "").
					Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath:    "",
					ConsoleBaseAddress: "https://foobar.com/host",
					MasterPublicURL:    "https://foobar.com/api",
				},
				Auth: Auth{
					AuthType:         "disabled",
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
					LogoutRedirect:   "https://foobar.com/logout",
				},
				Customization: Customization{
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{},
			},
		}, {
			name: "Config builder should handle StatuspageID",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				return b.StatusPageID("status-12345").Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Customization: Customization{
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{
					StatuspageID: "status-12345",
				},
			},
		},
		{
			name: "Config builder should handle custom dev catalog without categories and types",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				b.CustomDeveloperCatalog(v1.DeveloperConsoleCatalogCustomization{})
				return b.Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Customization: Customization{
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{},
			},
		},
		{
			name: "Config builder should handle custom dev catalog with empty (zero) categories",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				b.CustomDeveloperCatalog(v1.DeveloperConsoleCatalogCustomization{
					Categories: []v1.DeveloperConsoleCatalogCategory{},
				})
				return b.Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Session: Session{},
				Customization: Customization{
					DeveloperCatalog: &DeveloperConsoleCatalogCustomization{
						Categories: &[]DeveloperConsoleCatalogCategory{},
					},
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{},
			},
		},
		{
			name: "Config builder should handle custom dev catalog with some categories and subcategories",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				b.CustomDeveloperCatalog(v1.DeveloperConsoleCatalogCustomization{
					Categories: []v1.DeveloperConsoleCatalogCategory{
						{
							DeveloperConsoleCatalogCategoryMeta: v1.DeveloperConsoleCatalogCategoryMeta{
								ID:    "java",
								Label: "Java",
								Tags:  []string{"java", "jvm", "quarkus"},
							},
							Subcategories: []v1.DeveloperConsoleCatalogCategoryMeta{
								{
									ID:    "quarkus",
									Label: "Quarkus",
									Tags:  []string{"quarkus"},
								},
							},
						},
						{
							DeveloperConsoleCatalogCategoryMeta: v1.DeveloperConsoleCatalogCategoryMeta{
								ID:    "notagsorsubcategory",
								Label: "No tags or subcategory",
							},
						},
					},
				})
				return b.Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Session: Session{},
				Customization: Customization{
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
					DeveloperCatalog: &DeveloperConsoleCatalogCustomization{
						Categories: &[]DeveloperConsoleCatalogCategory{
							{
								DeveloperConsoleCatalogCategoryMeta: DeveloperConsoleCatalogCategoryMeta{
									ID:    "java",
									Label: "Java",
									Tags:  []string{"java", "jvm", "quarkus"},
								},
								Subcategories: []DeveloperConsoleCatalogCategoryMeta{
									{
										ID:    "quarkus",
										Label: "Quarkus",
										Tags:  []string{"quarkus"},
									},
								},
							},
							{
								DeveloperConsoleCatalogCategoryMeta: DeveloperConsoleCatalogCategoryMeta{
									ID:    "notagsorsubcategory",
									Label: "No tags or subcategory",
								},
							},
						},
					},
				},
				Providers: Providers{},
			},
		},
		{
			name: "Config builder should handle dev catalog types",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				b.CustomDeveloperCatalog(v1.DeveloperConsoleCatalogCustomization{
					Types: v1.DeveloperConsoleCatalogTypes{State: v1.CatalogTypeDisabled, Disabled: &[]string{"type1", "type2"}},
				})
				return b.Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Session: Session{},
				Customization: Customization{
					DeveloperCatalog: &DeveloperConsoleCatalogCustomization{
						Categories: nil,
						Types:      DeveloperConsoleCatalogTypes{State: CatalogTypeDisabled, Disabled: &[]string{"type1", "type2"}},
					},
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{},
			},
		},
		{
			name: "Config builder should handle dev catalog types with empty enabled array",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				b.CustomDeveloperCatalog(v1.DeveloperConsoleCatalogCustomization{
					Types: v1.DeveloperConsoleCatalogTypes{State: v1.CatalogTypeEnabled, Enabled: &[]string{}},
				})
				return b.Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Session: Session{},
				Customization: Customization{
					DeveloperCatalog: &DeveloperConsoleCatalogCustomization{
						Categories: nil,
						Types:      DeveloperConsoleCatalogTypes{State: CatalogTypeEnabled, Enabled: &[]string{}},
					},
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{},
			},
		},
		{
			name: "Config builder should handle project access options",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				b.ProjectAccess(v1.ProjectAccess{
					AvailableClusterRoles: []string{"View", "Edit", "Admin"},
				})
				return b.Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Session: Session{},
				Customization: Customization{
					ProjectAccess: ProjectAccess{
						AvailableClusterRoles: []string{"View", "Edit", "Admin"},
					},
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{},
			},
		},
		{
			name: "Config builder should handle quick starts options",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				b.QuickStarts(v1.QuickStarts{
					Disabled: []string{"quick-start0", "quick-start1", "quick-start2"},
				})
				return b.Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Session: Session{},
				Customization: Customization{
					QuickStarts: QuickStarts{
						Disabled: []string{"quick-start0", "quick-start1", "quick-start2"},
					},
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{},
			},
		},
		{
			name: "Config builder should handle perspectives",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				b.Perspectives([]v1.Perspective{
					{
						ID: "perspective1",
						Visibility: v1.PerspectiveVisibility{
							State: v1.PerspectiveAccessReview,
							AccessReview: &v1.ResourceAttributesAccessReview{
								Required: []authorizationv1.ResourceAttributes{{Resource: "namespaces", Verb: "list"}},
								Missing:  []authorizationv1.ResourceAttributes{{Resource: "clusterroles", Verb: "list"}},
							},
						},
					}, {
						ID: "perspective2",
						Visibility: v1.PerspectiveVisibility{
							State: v1.PerspectiveDisabled,
						},
					},
				})
				return b.Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Session: Session{},
				Customization: Customization{
					Perspectives: []Perspective{
						{
							ID: "perspective1",
							Visibility: PerspectiveVisibility{
								State: PerspectiveAccessReview,
								AccessReview: &ResourceAttributesAccessReview{
									Required: []authorizationv1.ResourceAttributes{
										{Resource: "namespaces", Verb: "list"},
									},
									Missing: []authorizationv1.ResourceAttributes{
										{Resource: "clusterroles", Verb: "list"},
									},
								},
							},
						},
						{ID: "perspective2", Visibility: PerspectiveVisibility{State: PerspectiveDisabled}},
					},
				},
				Providers: Providers{},
			},
		},
		{
			name: "Config builder should handle perspectives with Pinned resources",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				b.Perspectives([]v1.Perspective{
					{
						ID: "perspective1",
						Visibility: v1.PerspectiveVisibility{
							State: v1.PerspectiveAccessReview,
							AccessReview: &v1.ResourceAttributesAccessReview{
								Required: []authorizationv1.ResourceAttributes{{Resource: "namespaces", Verb: "list"}},
								Missing:  []authorizationv1.ResourceAttributes{{Resource: "clusterroles", Verb: "list"}},
							},
						},
						PinnedResources: &[]v1.PinnedResourceReference{
							{Group: "apps", Version: "v1", Resource: "deployments"},
							{Group: "", Version: "v1", Resource: "configmaps"},
						},
					}, {
						ID: "perspective2",
						Visibility: v1.PerspectiveVisibility{
							State: v1.PerspectiveDisabled,
						},
					},
				})
				return b.Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Session: Session{},
				Customization: Customization{
					Perspectives: []Perspective{
						{ID: "perspective1",
							Visibility: PerspectiveVisibility{
								State: PerspectiveAccessReview, AccessReview: &ResourceAttributesAccessReview{
									Required: []authorizationv1.ResourceAttributes{
										{Resource: "namespaces", Verb: "list"},
									},
									Missing: []authorizationv1.ResourceAttributes{
										{Resource: "clusterroles", Verb: "list"},
									},
								},
							},
							PinnedResources: &[]v1.PinnedResourceReference{
								{Group: "apps", Version: "v1", Resource: "deployments"},
								{Group: "", Version: "v1", Resource: "configmaps"},
							},
						},
						{ID: "perspective2", Visibility: PerspectiveVisibility{State: PerspectiveDisabled}},
					},
				},
				Providers: Providers{},
			},
		},
		{
			name: "Config builder should handle perspectives with empty Pinned resources",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				b.Perspectives([]v1.Perspective{
					{
						ID: "perspective1",
						Visibility: v1.PerspectiveVisibility{
							State: v1.PerspectiveAccessReview,
							AccessReview: &v1.ResourceAttributesAccessReview{
								Required: []authorizationv1.ResourceAttributes{{Resource: "namespaces", Verb: "list"}},
								Missing:  []authorizationv1.ResourceAttributes{{Resource: "clusterroles", Verb: "list"}},
							},
						},
						PinnedResources: &[]v1.PinnedResourceReference{},
					}, {
						ID: "perspective2",
						Visibility: v1.PerspectiveVisibility{
							State: v1.PerspectiveDisabled,
						},
					},
				})
				return b.Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Session: Session{},
				Customization: Customization{
					Perspectives: []Perspective{
						{ID: "perspective1",
							Visibility: PerspectiveVisibility{
								State: PerspectiveAccessReview,
								AccessReview: &ResourceAttributesAccessReview{
									Required: []authorizationv1.ResourceAttributes{
										{Resource: "namespaces", Verb: "list"},
									},
									Missing: []authorizationv1.ResourceAttributes{
										{Resource: "clusterroles", Verb: "list"},
									},
								},
							},
							PinnedResources: &[]v1.PinnedResourceReference{},
						},
						{ID: "perspective2", Visibility: PerspectiveVisibility{State: PerspectiveDisabled}},
					},
				},
				Providers: Providers{},
			},
		},
		{
			name: "Config builder should handle all inputs",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				b.Host("").
					LogoutURL("https://foobar.com/logout").
					Brand(v1.BrandOKD).
					DocURL("https://foobar.com/docs").
					APIServerURL("https://foobar.com/api").
					StatusPageID("status-12345").
					Plugins(map[string]string{
						"plugin1": "plugin1_url",
						"plugin2": "plugin2_url",
					}).
					I18nNamespaces([]string{"plugin__plugin1"})
				return b.Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
					MasterPublicURL: "https://foobar.com/api",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
					LogoutRedirect:   "https://foobar.com/logout",
				},
				Session: Session{},
				Customization: Customization{
					Branding:             "okd",
					DocumentationBaseURL: "https://foobar.com/docs",
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{
					StatuspageID: "status-12345",
				},
				Plugins: map[string]string{
					"plugin1": "plugin1_url",
					"plugin2": "plugin2_url",
				},
				I18nNamespaces: []string{"plugin__plugin1"},
			},
		},
		{
			name: "Config builder should pass telemetry configuration",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				b.TelemetryConfiguration(map[string]string{
					"a-key": "a-value",
				})
				return b.Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Customization: Customization{
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{},
				Telemetry: map[string]string{
					"a-key": "a-value",
				},
			},
		},
		{
			name: "Config builder should pass monitoring info",
			input: func() Config {
				b := &ConsoleServerCLIConfigBuilder{}
				b.Monitoring(&corev1.ConfigMap{
					Data: map[string]string{
						"alertmanagerUserWorkloadHost": "alertmanager-user-workload.openshift-user-workload-monitoring.svc:9094",
						"alertmanagerTenancyHost":      "alertmanager-user-workload.openshift-user-workload-monitoring.svc:9092",
					},
				})
				return b.Config()
			},
			output: Config{
				Kind:       "ConsoleConfig",
				APIVersion: "console.openshift.io/v1",
				ServingInfo: ServingInfo{
					BindAddress: "https://[::]:8443",
					CertFile:    certFilePath,
					KeyFile:     keyFilePath,
				},
				ClusterInfo: ClusterInfo{
					ConsoleBasePath: "",
				},
				Auth: Auth{
					ClientID:         api.OpenShiftConsoleName,
					ClientSecretFile: clientSecretFilePath,
				},
				Customization: Customization{
					Perspectives: []Perspective{
						{
							ID:         "dev",
							Visibility: PerspectiveVisibility{State: PerspectiveDisabled},
						},
					},
				},
				Providers: Providers{},
				MonitoringInfo: MonitoringInfo{
					AlertmanagerUserWorkloadHost: "alertmanager-user-workload.openshift-user-workload-monitoring.svc:9094",
					AlertmanagerTenancyHost:      "alertmanager-user-workload.openshift-user-workload-monitoring.svc:9092",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(tt.input(), tt.output); diff != nil {
				t.Error(diff)
			}
		})
	}
}

// Tests that the builder will return correctly formatted YAML representing a
// Console Server Config struct when builder.ConfigYAML() is called.
// This YAML should be an exact representation of the struct from builder.Config()
// and the output should make use of the YAML tags embedded in the structs
// in types.go
func TestConsoleServerCLIConfigBuilderYAML(t *testing.T) {
	tests := []struct {
		name  string
		input func() ([]byte, error)
		// tests the YAML conversion output of the configmap
		output string
	}{
		{
			name: "Config builder should return default config if given no inputs",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				return b.ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
session: {}
customization:
  perspectives:
  - id: dev
    visibility:
      state: Disabled
providers: {}
`,
		},
		{
			name: "Config builder should return modified client ID if overriden by user",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				return b.AuthConfig(
					&configv1.Authentication{
						Spec: configv1.AuthenticationSpec{
							Type: configv1.AuthenticationTypeOIDC,
							OIDCProviders: []configv1.OIDCProvider{
								{
									OIDCClients: []configv1.OIDCClientConfig{
										{
											ComponentNamespace: "openshift-console",
											ComponentName:      "console",
											ClientID:           "testing-id",
											ClientSecret: configv1.SecretNameReference{
												Name: "testing-oidc-client-secret",
											},
										},
									},
								},
							},
						},
					}, "",
				).ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  authType: oidc
  clientID: testing-id
  clientSecretFile: /var/oauth-config/clientSecret
session:
  cookieEncryptionKeyFile: /var/session-secret/sessionEncryptionKey
  cookieAuthenticationKeyFile: /var/session-secret/sessionAuthenticationKey
customization:
  perspectives:
  - id: dev
    visibility:
      state: Disabled
providers: {}
`,
		},
		{
			name: "Config builder should handle cluster info",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				return b.
					APIServerURL("https://foobar.com/api").
					Host("https://foobar.com/host").
					LogoutURL("https://foobar.com/logout").
					AuthConfig(&configv1.Authentication{Spec: configv1.AuthenticationSpec{Type: configv1.AuthenticationTypeIntegratedOAuth}}, "").
					ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo:
  consoleBaseAddress: https://foobar.com/host
  masterPublicURL: https://foobar.com/api
auth:
  authType: openshift
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  oauthEndpointCAFile: /var/oauth-serving-cert/ca-bundle.crt
  logoutRedirect: https://foobar.com/logout
session: {}
customization:
  perspectives:
  - id: dev
    visibility:
      state: Disabled
providers: {}
`,
		},
		{
			name: "Config builder should handle StatuspageID",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				return b.StatusPageID("status-12345").ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
session: {}
customization:
  perspectives:
  - id: dev
    visibility:
      state: Disabled
providers:
  statuspageID: status-12345
`,
		},
		{
			name: "Config builder should handle custom dev catalog without categories and types",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				b.CustomDeveloperCatalog(v1.DeveloperConsoleCatalogCustomization{})
				return b.ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
session: {}
customization:
  perspectives:
  - id: dev
    visibility:
      state: Disabled
providers: {}
`,
		},
		{
			name: "Config builder should handle custom dev catalog with empty (zero) categories",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				b.CustomDeveloperCatalog(v1.DeveloperConsoleCatalogCustomization{
					Categories: []v1.DeveloperConsoleCatalogCategory{},
				})
				return b.ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
session: {}
customization:
  developerCatalog:
    categories: []
    types: {}
  perspectives:
  - id: dev
    visibility:
      state: Disabled
providers: {}
`,
		},
		{
			name: "Config builder should handle custom dev catalog with some categories and subcategories",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				b.CustomDeveloperCatalog(v1.DeveloperConsoleCatalogCustomization{
					Categories: []v1.DeveloperConsoleCatalogCategory{
						{
							DeveloperConsoleCatalogCategoryMeta: v1.DeveloperConsoleCatalogCategoryMeta{
								ID:    "java",
								Label: "Java",
								Tags:  []string{"java", "jvm", "quarkus"},
							},
							Subcategories: []v1.DeveloperConsoleCatalogCategoryMeta{
								{
									ID:    "quarkus",
									Label: "Quarkus",
									Tags:  []string{"quarkus"},
								},
							},
						},
						{
							DeveloperConsoleCatalogCategoryMeta: v1.DeveloperConsoleCatalogCategoryMeta{
								ID:    "notagsorsubcategory",
								Label: "No tags or subcategory",
							},
						},
					},
				})
				return b.ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
session: {}
customization:
  developerCatalog:
    categories:
    - id: java
      label: Java
      tags:
      - java
      - jvm
      - quarkus
      subcategories:
      - id: quarkus
        label: Quarkus
        tags:
        - quarkus
    - id: notagsorsubcategory
      label: No tags or subcategory
    types: {}
  perspectives:
  - id: dev
    visibility:
      state: Disabled
providers: {}
`,
		},
		{
			name: "Config builder should handle dev catalog types",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				b.CustomDeveloperCatalog(v1.DeveloperConsoleCatalogCustomization{
					Types: v1.DeveloperConsoleCatalogTypes{State: v1.CatalogTypeDisabled, Disabled: &[]string{"type1", "type2"}},
				})
				return b.ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
session: {}
customization:
  developerCatalog:
    categories: null
    types:
      state: Disabled
      disabled:
      - type1
      - type2
  perspectives:
  - id: dev
    visibility:
      state: Disabled
providers: {}
`,
		},
		{
			name: "Config builder should handle dev catalog types with empty enabled array",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				b.CustomDeveloperCatalog(v1.DeveloperConsoleCatalogCustomization{
					Types: v1.DeveloperConsoleCatalogTypes{State: v1.CatalogTypeEnabled, Enabled: &[]string{}},
				})
				return b.ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
session: {}
customization:
  developerCatalog:
    categories: null
    types:
      state: Enabled
      enabled: []
  perspectives:
  - id: dev
    visibility:
      state: Disabled
providers: {}
`,
		},
		{
			name: "Config builder should handle custom add page with disabled actions",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				b.AddPage(v1.AddPage{
					DisabledActions: []string{
						"git",
						"tekton.dev/pipelines",
					},
				})
				return b.ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
session: {}
customization:
  addPage:
    disabledActions:
    - git
    - tekton.dev/pipelines
  perspectives:
  - id: dev
    visibility:
      state: Disabled
providers: {}
`,
		},
		{
			name: "Config builder should handle all inputs",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				b.Host("").
					LogoutURL("https://foobar.com/logout").
					Brand(v1.BrandOKD).
					DocURL("https://foobar.com/docs").
					APIServerURL("https://foobar.com/api").
					CustomHostnameRedirectPort(true).
					StatusPageID("status-12345").
					Plugins(map[string]string{
						"plugin1": "plugin1_url",
						"plugin2": "plugin2_url",
					}).
					I18nNamespaces([]string{"plugin__plugin1"}).
					AuthConfig(&configv1.Authentication{Spec: configv1.AuthenticationSpec{Type: "OIDC"}}, "")
				return b.ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
  redirectPort: ` + strconv.Itoa(api.RedirectContainerPort) + `
clusterInfo:
  masterPublicURL: https://foobar.com/api
auth:
  authType: disabled
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
  logoutRedirect: https://foobar.com/logout
session: {}
customization:
  branding: okd
  documentationBaseURL: https://foobar.com/docs
  perspectives:
  - id: dev
    visibility:
      state: Disabled
providers:
  statuspageID: status-12345
plugins:
  plugin1: plugin1_url
  plugin2: plugin2_url
i18nNamespaces:
- plugin__plugin1
`,
		},
		{
			name: "Config builder should handle inputs for quick starts options",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				b.QuickStarts(v1.QuickStarts{
					Disabled: []string{
						"quickStarts0",
						"quickStarts1",
					},
				})
				return b.ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
session: {}
customization:
  quickStarts:
    disabled:
    - quickStarts0
    - quickStarts1
  perspectives:
  - id: dev
    visibility:
      state: Disabled
providers: {}
`,
		},
		{
			name: "Config builder should handle perspectives",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				b.Perspectives([]v1.Perspective{
					{
						ID: "perspective1",
						Visibility: v1.PerspectiveVisibility{
							State: v1.PerspectiveAccessReview,
							AccessReview: &v1.ResourceAttributesAccessReview{
								Required: []authorizationv1.ResourceAttributes{{Resource: "namespaces", Verb: "list"}},
								Missing:  []authorizationv1.ResourceAttributes{{Resource: "clusterroles", Verb: "list"}},
							},
						},
					}, {
						ID: "perspective2",
						Visibility: v1.PerspectiveVisibility{
							State: v1.PerspectiveDisabled,
						},
					},
				})
				return b.ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
session: {}
customization:
  perspectives:
  - id: perspective1
    visibility:
      state: AccessReview
      accessReview:
        required:
        - namespace: ""
          verb: list
          group: ""
          version: ""
          resource: namespaces
          subresource: ""
          name: ""
          fieldselector: null
          labelselector: null
        missing:
        - namespace: ""
          verb: list
          group: ""
          version: ""
          resource: clusterroles
          subresource: ""
          name: ""
          fieldselector: null
          labelselector: null
  - id: perspective2
    visibility:
      state: Disabled
providers: {}
`,
		},
		{
			name: "Config builder should handle perspectives with Pinned resources",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				b.Perspectives([]v1.Perspective{
					{
						ID: "perspective1",
						Visibility: v1.PerspectiveVisibility{
							State: v1.PerspectiveAccessReview,
							AccessReview: &v1.ResourceAttributesAccessReview{
								Required: []authorizationv1.ResourceAttributes{{Resource: "namespaces", Verb: "list"}},
								Missing:  []authorizationv1.ResourceAttributes{{Resource: "clusterroles", Verb: "list"}},
							},
						},
						PinnedResources: &[]v1.PinnedResourceReference{
							{Group: "apps", Version: "v1", Resource: "deployments"},
							{Group: "", Version: "v1", Resource: "configmaps"},
						},
					}, {
						ID: "perspective2",
						Visibility: v1.PerspectiveVisibility{
							State: v1.PerspectiveDisabled,
						},
					},
				})
				return b.ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
session: {}
customization:
  perspectives:
  - id: perspective1
    visibility:
      state: AccessReview
      accessReview:
        required:
        - namespace: ""
          verb: list
          group: ""
          version: ""
          resource: namespaces
          subresource: ""
          name: ""
          fieldselector: null
          labelselector: null
        missing:
        - namespace: ""
          verb: list
          group: ""
          version: ""
          resource: clusterroles
          subresource: ""
          name: ""
          fieldselector: null
          labelselector: null
    pinnedResources:
    - group: apps
      version: v1
      resource: deployments
    - group: ""
      version: v1
      resource: configmaps
  - id: perspective2
    visibility:
      state: Disabled
providers: {}
`,
		},
		{
			name: "Config builder should handle perspectives with empty Pinned resources",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				b.Perspectives([]v1.Perspective{
					{
						ID: "perspective1",
						Visibility: v1.PerspectiveVisibility{
							State: v1.PerspectiveAccessReview,
							AccessReview: &v1.ResourceAttributesAccessReview{
								Required: []authorizationv1.ResourceAttributes{{Resource: "namespaces", Verb: "list"}},
								Missing:  []authorizationv1.ResourceAttributes{{Resource: "clusterroles", Verb: "list"}},
							},
						},
						PinnedResources: &[]v1.PinnedResourceReference{},
					}, {
						ID: "perspective2",
						Visibility: v1.PerspectiveVisibility{
							State: v1.PerspectiveDisabled,
						},
					},
				})
				return b.ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
session: {}
customization:
  perspectives:
  - id: perspective1
    visibility:
      state: AccessReview
      accessReview:
        required:
        - namespace: ""
          verb: list
          group: ""
          version: ""
          resource: namespaces
          subresource: ""
          name: ""
          fieldselector: null
          labelselector: null
        missing:
        - namespace: ""
          verb: list
          group: ""
          version: ""
          resource: clusterroles
          subresource: ""
          name: ""
          fieldselector: null
          labelselector: null
    pinnedResources: []
  - id: perspective2
    visibility:
      state: Disabled
providers: {}
`,
		},
		{
			name: "Config builder should pass telemetry configuration",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				b.TelemetryConfiguration(map[string]string{
					"a-key":               "a-value",
					"a-boolean-as-string": "false",
				})
				return b.ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
session: {}
customization:
  perspectives:
  - id: dev
    visibility:
      state: Disabled
providers: {}
telemetry:
  a-boolean-as-string: "false"
  a-key: a-value
`,
		},
		{
			name: "Config builder should pass customization Capabilities",
			input: func() ([]byte, error) {
				b := &ConsoleServerCLIConfigBuilder{}
				b.Capabilities([]v1.Capability{
					{
						Name: v1.LightspeedButton,
						Visibility: v1.CapabilityVisibility{
							State: v1.CapabilityEnabled,
						},
					},
				})
				return b.ConfigYAML()
			},
			output: `apiVersion: console.openshift.io/v1
kind: ConsoleConfig
servingInfo:
  bindAddress: https://[::]:8443
  certFile: /var/serving-cert/tls.crt
  keyFile: /var/serving-cert/tls.key
clusterInfo: {}
auth:
  clientID: console
  clientSecretFile: /var/oauth-config/clientSecret
session: {}
customization:
  perspectives:
  - id: dev
    visibility:
      state: Disabled
  capabilities:
  - name: LightspeedButton
    visibility:
      state: Enabled
providers: {}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, _ := tt.input()
			if diff := cmp.Diff(tt.output, string(input)); len(diff) > 0 {
				t.Error(diff)
			}
		})
	}
}
