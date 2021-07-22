package consoleserver

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
	APIVersion    string `yaml:"apiVersion"`
	Kind          string `yaml:"kind"`
	ServingInfo   `yaml:"servingInfo"`
	ClusterInfo   `yaml:"clusterInfo"`
	Auth          `yaml:"auth"`
	Customization `yaml:"customization"`
	Providers     `yaml:"providers"`
	Plugins       map[string]string `yaml:"plugins,omitempty"`
}

// ServingInfo holds configuration for serving HTTP.
type ServingInfo struct {
	BindAddress  string `yaml:"bindAddress,omitempty"`
	CertFile     string `yaml:"certFile,omitempty"`
	KeyFile      string `yaml:"keyFile,omitempty"`
	RedirectPort int    `yaml:"redirectPort,omitempty"`

	// These fields are defined in `HTTPServingInfo`, but are not supported for console. Fail if any are specified.
	// https://github.com/openshift/api/blob/0cb4131a7636e1ada6b2769edc9118f0fe6844c8/config/v1/types.go#L7-L38
	BindNetwork           string        `yaml:"bindNetwork,omitempty"`
	ClientCA              string        `yaml:"clientCA,omitempty"`
	NamedCertificates     []interface{} `yaml:"namedCertificates,omitempty"`
	MinTLSVersion         string        `yaml:"minTLSVersion,omitempty"`
	CipherSuites          []string      `yaml:"cipherSuites,omitempty"`
	MaxRequestsInFlight   int64         `yaml:"maxRequestsInFlight,omitempty"`
	RequestTimeoutSeconds int64         `yaml:"requestTimeoutSeconds,omitempty"`
}

// ClusterInfo holds information the about the cluster such as master public URL and console public URL.
type ClusterInfo struct {
	ConsoleBaseAddress string `yaml:"consoleBaseAddress,omitempty"`
	ConsoleBasePath    string `yaml:"consoleBasePath,omitempty"`
	MasterPublicURL    string `yaml:"masterPublicURL,omitempty"`
}

// Auth holds configuration for authenticating with OpenShift. The auth method is assumed to be "openshift".
type Auth struct {
	ClientID                 string `yaml:"clientID,omitempty"`
	ClientSecretFile         string `yaml:"clientSecretFile,omitempty"`
	OAuthEndpointCAFile      string `yaml:"oauthEndpointCAFile,omitempty"`
	LogoutRedirect           string `yaml:"logoutRedirect,omitempty"`
	InactivityTimeoutSeconds int    `yaml:"inactivityTimeoutSeconds,omitempty"`
}

// Customization holds configuration such as what logo to use.
type Customization struct {
	Branding             string `yaml:"branding,omitempty"`
	DocumentationBaseURL string `yaml:"documentationBaseURL,omitempty"`
	CustomProductName    string `yaml:"customProductName,omitempty"`
	CustomLogoFile       string `yaml:"customLogoFile,omitempty"`
	// developerCatalog allows to configure the shown developer catalog categories.
	DeveloperCatalog *DeveloperConsoleCatalogCustomization `yaml:"developerCatalog,omitempty"`
	ProjectAccess    ProjectAccess                         `yaml:"projectAccess,omitempty"`
	QuickStarts      QuickStarts                           `yaml:"quickStarts,omitempty"`
	// addPage allows customizing actions on the Add page in developer perspective.
	AddPage AddPage `yaml:"addPage,omitempty"`
}

// QuickStarts contains options for quick starts
type QuickStarts struct {
	Disabled []string `json:"disabled,omitempty"`
}

// ProjectAccess contains options for project access roles
type ProjectAccess struct {
	AvailableClusterRoles []string `yaml:"availableClusterRoles,omitempty"`
}

// DeveloperConsoleCatalogCustomization allow cluster admin to configure developer catalog.
type DeveloperConsoleCatalogCustomization struct {
	// categories which are shown the in developer catalog.
	Categories *[]DeveloperConsoleCatalogCategory `yaml:"categories"`
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

type Providers struct {
	StatuspageID string `yaml:"statuspageID,omitempty"`
}

type HelmChartRepo struct {
	URL    string `yaml:"url,omitempty"`
	CAFile string `yaml:"caFile,omitempty"`
}

type Helm struct {
	ChartRepo HelmChartRepo `yaml:"chartRepository"`
}
