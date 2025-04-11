package consoleserver

import (
	configv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/api/console/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
)

// This file is a copy of the struct within the console itself:
//   https://github.com/openshift/console/blob/master/pkg/serverconfig/types.go
// These structs need to remain in sync.
//
// `yaml:",omitempty"` has not been applied to any of the properties currently
// in use by the operator.  This is for backwards compatibilty purposes. If
// we have been sending an empty string value, we will continue to send it.
// Anything we have not been explicitly setting should have the `yaml:",omitempty"` tag.

// Config is the top-level console server cli configuration.
type Config struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`

	Auth                         `yaml:"auth"`
	ClusterInfo                  `yaml:"clusterInfo"`
	ContentSecurityPolicy        map[v1.DirectiveType][]string `yaml:"contentSecurityPolicy,omitempty"`
	ContentSecurityPolicyEnabled bool                          `yaml:"contentSecurityPolicyEnabled,omitempty"`
	Customization                `yaml:"customization"`
	I18nNamespaces               []string `yaml:"i18nNamespaces,omitempty"`
	MonitoringInfo               `yaml:"monitoringInfo,omitempty"`
	Plugins                      map[string]string `yaml:"plugins,omitempty"`
	Providers                    `yaml:"providers"`
	Proxy                        Proxy `yaml:"proxy,omitempty"`
	ServingInfo                  `yaml:"servingInfo"`
	Session                      `yaml:"session"`
	Telemetry                    map[string]string `yaml:"telemetry,omitempty"`
}

type Proxy struct {
	Services []ProxyService `yaml:"services,omitempty"`
}

type ProxyService struct {
	Authorize      bool   `yaml:"authorize"`
	CACertificate  string `yaml:"caCertificate"`
	ConsoleAPIPath string `yaml:"consoleAPIPath"`
	Endpoint       string `yaml:"endpoint"`
}

// ServingInfo holds configuration for serving HTTP.
type ServingInfo struct {
	BindAddress string `yaml:"bindAddress,omitempty"`

	// These fields are defined in `HTTPServingInfo`, but are not supported for console. Fail if any are specified.
	// https://github.com/openshift/api/blob/0cb4131a7636e1ada6b2769edc9118f0fe6844c8/config/v1/types.go#L7-L38
	BindNetwork           string        `yaml:"bindNetwork,omitempty"`
	CertFile              string        `yaml:"certFile,omitempty"`
	CipherSuites          []string      `yaml:"cipherSuites,omitempty"`
	ClientCA              string        `yaml:"clientCA,omitempty"`
	KeyFile               string        `yaml:"keyFile,omitempty"`
	MaxRequestsInFlight   int64         `yaml:"maxRequestsInFlight,omitempty"`
	MinTLSVersion         string        `yaml:"minTLSVersion,omitempty"`
	NamedCertificates     []interface{} `yaml:"namedCertificates,omitempty"`
	RedirectPort          int           `yaml:"redirectPort,omitempty"`
	RequestTimeoutSeconds int64         `yaml:"requestTimeoutSeconds,omitempty"`
}

// ClusterInfo holds information the about the cluster such as master public URL and console public URL.
type ClusterInfo struct {
	ConsoleBaseAddress   string                `yaml:"consoleBaseAddress,omitempty"`
	ConsoleBasePath      string                `yaml:"consoleBasePath,omitempty"`
	ControlPlaneToplogy  configv1.TopologyMode `yaml:"controlPlaneTopology,omitempty"`
	CopiedCSVsDisabled   bool                  `yaml:"copiedCSVsDisabled,omitempty"`
	MasterPublicURL      string                `yaml:"masterPublicURL,omitempty"`
	NodeArchitectures    []string              `yaml:"nodeArchitectures,omitempty"`
	NodeOperatingSystems []string              `yaml:"nodeOperatingSystems,omitempty"`
	ReleaseVersion       string                `yaml:"releaseVersion,omitempty"`
}

// MonitoringInfo holds configuration for monitoring related services
type MonitoringInfo struct {
	AlertmanagerTenancyHost      string `yaml:"alertmanagerTenancyHost,omitempty"`
	AlertmanagerUserWorkloadHost string `yaml:"alertmanagerUserWorkloadHost,omitempty"`
}

// Auth holds configuration for authenticating with OpenShift. The auth method is assumed to be "openshift".
type Auth struct {
	AuthType                 string   `yaml:"authType,omitempty"`
	ClientID                 string   `yaml:"clientID,omitempty"`
	ClientSecretFile         string   `yaml:"clientSecretFile,omitempty"`
	InactivityTimeoutSeconds int      `yaml:"inactivityTimeoutSeconds,omitempty"`
	LogoutRedirect           string   `yaml:"logoutRedirect,omitempty"`
	OAuthEndpointCAFile      string   `yaml:"oauthEndpointCAFile,omitempty"`
	OIDCExtraScopes          []string `yaml:"oidcExtraScopes,omitempty"`
	OIDCIssuer               string   `yaml:"oidcIssuer,omitempty"`
	OIDCOCLoginCommand       string   `yaml:"oidcOCLoginCommand,omitempty"`
}

// Session holds configuration for web-session related configuration
type Session struct {
	CookieAuthenticationKeyFile string `yaml:"cookieAuthenticationKeyFile,omitempty"`
	CookieEncryptionKeyFile     string `yaml:"cookieEncryptionKeyFile,omitempty"`
	// TODO: move InactivityTimeoutSeconds here
}

// Customization holds configuration such as what logo to use.
type Customization struct {
	// addPage allows customizing actions on the Add page in developer perspective.
	AddPage           AddPage                 `yaml:"addPage,omitempty"`
	Branding          string                  `yaml:"branding,omitempty"`
	Capabilities      []operatorv1.Capability `yaml:"capabilities,omitempty"`
	CustomProductName string                  `yaml:"customProductName,omitempty"`
	// developerCatalog allows to configure the shown developer catalog categories.
	DeveloperCatalog     *DeveloperConsoleCatalogCustomization `yaml:"developerCatalog,omitempty"`
	DocumentationBaseURL string                                `yaml:"documentationBaseURL,omitempty"`
	Logos                []operatorv1.Logo                     `yaml:"logos,omitempty"`
	// perspectives allows enabling/disabling of perspective(s) that user can see in the Perspective switcher dropdown.
	Perspectives  []Perspective `yaml:"perspectives,omitempty"`
	ProjectAccess ProjectAccess `yaml:"projectAccess,omitempty"`
	QuickStarts   QuickStarts   `yaml:"quickStarts,omitempty"`
}

// QuickStarts contains options for quick starts
type QuickStarts struct {
	Disabled []string `json:"disabled,omitempty"`
}

// ProjectAccess contains options for project access roles
type ProjectAccess struct {
	AvailableClusterRoles []string `yaml:"availableClusterRoles,omitempty"`
}

// CatalogTypesState defines the state of the catalog types based on which the types will be enabled or disabled.
type CatalogTypesState string

const (
	CatalogTypeDisabled CatalogTypesState = "Disabled"
	CatalogTypeEnabled  CatalogTypesState = "Enabled"
)

// DeveloperConsoleCatalogTypes defines the state of the sub-catalog types.
type DeveloperConsoleCatalogTypes struct {
	// disabled is a list of developer catalog types (sub-catalogs IDs) that are not shown to users.
	// Types (sub-catalogs) are added via console plugins, the available types (sub-catalog IDs) are available
	// in the console on the cluster configuration page, or when editing the YAML in the console.
	// Example: "Devfile", "HelmChart", "BuilderImage"
	// If the list is empty or all the available sub-catalog types are added, then the complete developer catalog should be hidden.
	Disabled *[]string `yaml:"disabled,omitempty"`
	// enabled is a list of developer catalog types (sub-catalogs IDs) that will be shown to users.
	// Types (sub-catalogs) are added via console plugins, the available types (sub-catalog IDs) are available
	// in the console on the cluster configuration page, or when editing the YAML in the console.
	// Example: "Devfile", "HelmChart", "BuilderImage"
	// If the list is non-empty, a new type will not be shown to the user until it is added to list.
	// If the list is empty the complete developer catalog will be shown.
	Enabled *[]string `yaml:"enabled,omitempty"`
	// state defines if a list of catalog types should be enabled or disabled.
	State CatalogTypesState `yaml:"state,omitempty"`
}

// DeveloperConsoleCatalogCustomization allow cluster admin to configure developer catalog.
type DeveloperConsoleCatalogCustomization struct {
	// categories which are shown in the developer catalog.
	Categories *[]DeveloperConsoleCatalogCategory `yaml:"categories"`
	// types allows enabling or disabling of sub-catalog types that user can see in the Developer catalog.
	// When omitted, all the sub-catalog types will be shown.
	Types DeveloperConsoleCatalogTypes `yaml:"types"`
}

// DeveloperConsoleCatalogCategoryMeta are the key identifiers of a developer catalog category.
type DeveloperConsoleCatalogCategoryMeta struct {
	// ID is an identifier used in the URL to enable deep linking in console.
	// ID is required and must have 1-32 URL safe (A-Z, a-z, 0-9, - and _) characters.
	ID string `yaml:"id"`
	// label defines a category display label. It is required and must have 1-64 characters.
	Label string `yaml:"label"`
	// tags is a list of strings that will match the category. A selected category
	// show all items which has at least one overlapping tag between category and item.
	Tags []string `yaml:"tags,omitempty"`
}

// DeveloperConsoleCatalogCategory for the developer console catalog.
type DeveloperConsoleCatalogCategory struct {
	// defines top level category ID, label and filter tags.
	DeveloperConsoleCatalogCategoryMeta `yaml:",inline"`
	// subcategories defines a list of child categories.
	Subcategories []DeveloperConsoleCatalogCategoryMeta `yaml:"subcategories,omitempty"`
}

// AddPage allows customizing actions on the Add page in developer perspective.
type AddPage struct {
	// disabledActions is a list of actions that are not shown to users.
	// Each action in the list is represented by its ID.
	DisabledActions []string `yaml:"disabledActions,omitempty"`
}

// PerspectiveState defines the visibility state of the perspective. "Enabled" means the perspective is shown.
// "Disabled" means the Perspective is hidden.
// "AccessReview" means access review check is required to show or hide a Perspective.
type PerspectiveState string

const (
	PerspectiveAccessReview PerspectiveState = "AccessReview"
	PerspectiveDisabled     PerspectiveState = "Disabled"
	PerspectiveEnabled      PerspectiveState = "Enabled"
)

// PerspectiveID defines the id of the perspective.
// "admin" is the id of the admin perspective.
// "dev" is the id of the developer perspective.
type PerspectiveID string

const (
	PerspectiveIDAdmin PerspectiveID = "admin"
	PerspectiveIDDev   PerspectiveID = "dev"
)

// ResourceAttributesAccessReview defines the visibility of the perspective depending on the access review checks.
// `required` and  `missing` can work together esp. in the case where the cluster admin
// wants to show another perspective to users without specific permissions. Out of `required` and `missing` atleast one property should be non-empty.
type ResourceAttributesAccessReview struct {
	// missing defines a list of permission checks. The perspective will only be shown when at least one check fails. When omitted, the access review is skipped and the perspective will not be shown unless it is required to do so based on the configuration of the required access review list.
	Missing []authorizationv1.ResourceAttributes `yaml:"missing,omitempty"`
	// required defines a list of permission checks. The perspective will only be shown when all checks are successful. When omitted, the access review is skipped and the perspective will not be shown unless it is required to do so based on the configuration of the missing access review list.
	Required []authorizationv1.ResourceAttributes `yaml:"required,omitempty"`
}

// PerspectiveVisibility defines the criteria to show/hide a perspective.
type PerspectiveVisibility struct {
	// accessReview defines required and missing access review checks.
	AccessReview *ResourceAttributesAccessReview `yaml:"accessReview,omitempty"`
	// state defines the perspective is enabled or disabled or access review check is required.
	// state is required
	State PerspectiveState `yaml:"state"`
}

// Perspective defines a perspective that cluster admins want to show/hide in the perspective switcher dropdown
type Perspective struct {
	// id defines the id of the perspective.
	// Example: "dev", "admin".
	// The available perspective ids can be found in the code snippet section next to the yaml editor.
	// Incorrect or unknown ids will be ignored.
	ID string `yaml:"id"`
	// pinnedResources defines the list of default pinned resources that users will see on the perspective navigation if they have not customized these pinned resources themselves.
	// The list of available Kubernetes resources could be read via `kubectl api-resources`.
	// The console will also provide a configuration UI and a YAML snippet that will list the available resources that can be pinned to the navigation.
	// Incorrect or unknown resources will be ignored.
	PinnedResources *[]operatorv1.PinnedResourceReference `yaml:"pinnedResources,omitempty"`
	// visibility defines the state of perspective along with access review checks if needed for that perspective.
	// visibility is required
	Visibility PerspectiveVisibility `yaml:"visibility"`
}

type Providers struct {
	StatuspageID string `yaml:"statuspageID,omitempty"`
}

type HelmChartRepo struct {
	CAFile string `yaml:"caFile,omitempty"`
	URL    string `yaml:"url,omitempty"`
}

type Helm struct {
	ChartRepo HelmChartRepo `yaml:"chartRepository"`
}
