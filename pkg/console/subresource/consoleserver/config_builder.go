package consoleserver

import (
	"os"
	"reflect"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"gopkg.in/yaml.v2"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const (
	clientSecretFilePath     = "/var/oauth-config/clientSecret"
	oauthServingCertFilePath = "/var/oauth-serving-cert/ca-bundle.crt"
	oauthEndpointCAFilePath  = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	// serving info
	certFilePath = "/var/serving-cert/tls.crt"
	keyFilePath  = "/var/serving-cert/tls.key"
)

// ConsoleServerCLIConfigBuilder
// Director will be DefaultConfigMap()
//
// b := ConsoleYamlConfigBuilder{}
// return the default config value immediately:
//
//	b.Config()
//	b.ConfigYAML()
//
// set all the values:
//
//	b.Host(host).LogoutURL("").Brand("").DocURL("").APIServerURL("").Config()
//
// set only some values:
//
//	b.Host().Brand("").Config()
type ConsoleServerCLIConfigBuilder struct {
	host                       string
	logoutRedirectURL          string
	brand                      operatorv1.Brand
	docURL                     string
	apiServerURL               string
	controlPlaneToplogy        configv1.TopologyMode
	statusPageID               string
	customProductName          string
	devCatalogCustomization    operatorv1.DeveloperConsoleCatalogCustomization
	projectAccess              operatorv1.ProjectAccess
	quickStarts                operatorv1.QuickStarts
	addPage                    operatorv1.AddPage
	perspectives               []operatorv1.Perspective
	customLogoFile             string
	CAFile                     string
	monitoring                 map[string]string
	customHostnameRedirectPort int
	inactivityTimeoutSeconds   int
	pluginsList                map[string]string
	i18nNamespaceList          []string
	proxyServices              []ProxyService
	managedClusterConfigFile   string
	telemetry                  map[string]string
	releaseVersion             string
	nodeArchitectures          []string
}

func (b *ConsoleServerCLIConfigBuilder) Host(host string) *ConsoleServerCLIConfigBuilder {
	b.host = host
	return b
}
func (b *ConsoleServerCLIConfigBuilder) LogoutURL(logoutRedirectURL string) *ConsoleServerCLIConfigBuilder {
	b.logoutRedirectURL = logoutRedirectURL
	return b
}
func (b *ConsoleServerCLIConfigBuilder) Brand(brand operatorv1.Brand) *ConsoleServerCLIConfigBuilder {
	b.brand = brand
	return b
}
func (b *ConsoleServerCLIConfigBuilder) DocURL(docURL string) *ConsoleServerCLIConfigBuilder {
	b.docURL = docURL
	return b
}
func (b *ConsoleServerCLIConfigBuilder) APIServerURL(apiServerURL string) *ConsoleServerCLIConfigBuilder {
	b.apiServerURL = apiServerURL
	return b
}
func (b *ConsoleServerCLIConfigBuilder) TopologyMode(topologyMode configv1.TopologyMode) *ConsoleServerCLIConfigBuilder {
	b.controlPlaneToplogy = topologyMode
	return b
}
func (b *ConsoleServerCLIConfigBuilder) CustomProductName(customProductName string) *ConsoleServerCLIConfigBuilder {
	b.customProductName = customProductName
	return b
}
func (b *ConsoleServerCLIConfigBuilder) CustomDeveloperCatalog(devCatalogCustomization operatorv1.DeveloperConsoleCatalogCustomization) *ConsoleServerCLIConfigBuilder {
	b.devCatalogCustomization = devCatalogCustomization
	return b
}
func (b *ConsoleServerCLIConfigBuilder) ProjectAccess(projectAccess operatorv1.ProjectAccess) *ConsoleServerCLIConfigBuilder {
	b.projectAccess = projectAccess
	return b
}
func (b *ConsoleServerCLIConfigBuilder) QuickStarts(quickStarts operatorv1.QuickStarts) *ConsoleServerCLIConfigBuilder {
	b.quickStarts = quickStarts
	return b
}
func (b *ConsoleServerCLIConfigBuilder) AddPage(addPage operatorv1.AddPage) *ConsoleServerCLIConfigBuilder {
	b.addPage = addPage
	return b
}
func (b *ConsoleServerCLIConfigBuilder) Perspectives(perspectives []operatorv1.Perspective) *ConsoleServerCLIConfigBuilder {
	b.perspectives = perspectives
	return b
}
func (b *ConsoleServerCLIConfigBuilder) CustomLogoFile(customLogoFile string) *ConsoleServerCLIConfigBuilder {
	if customLogoFile != "" {
		b.customLogoFile = "/var/logo/" + customLogoFile // append path here to prevent customLogoFile from always being just /var/logo/
	}
	return b
}
func (b *ConsoleServerCLIConfigBuilder) CustomHostnameRedirectPort(redirect bool) *ConsoleServerCLIConfigBuilder {
	// If custom hostname is set on the console operator config,
	// set the port under which the console backend will listen
	// for redirect.
	if redirect {
		b.customHostnameRedirectPort = api.RedirectContainerTargetPort
	}
	return b
}
func (b *ConsoleServerCLIConfigBuilder) StatusPageID(id string) *ConsoleServerCLIConfigBuilder {
	b.statusPageID = id
	return b
}

func (b *ConsoleServerCLIConfigBuilder) OAuthServingCert(useDefaultCAFile bool) *ConsoleServerCLIConfigBuilder {
	if useDefaultCAFile {
		b.CAFile = oauthEndpointCAFilePath
		return b
	}
	b.CAFile = oauthServingCertFilePath
	return b
}

func (b *ConsoleServerCLIConfigBuilder) Monitoring(monitoringConfig *corev1.ConfigMap) *ConsoleServerCLIConfigBuilder {
	if monitoringConfig != nil {
		b.monitoring = monitoringConfig.Data
	}
	return b
}

func (b *ConsoleServerCLIConfigBuilder) InactivityTimeout(timeout int) *ConsoleServerCLIConfigBuilder {
	b.inactivityTimeoutSeconds = timeout
	return b
}

func (b *ConsoleServerCLIConfigBuilder) Plugins(plugins map[string]string) *ConsoleServerCLIConfigBuilder {
	b.pluginsList = plugins
	return b
}

func (b *ConsoleServerCLIConfigBuilder) I18nNamespaces(i18nNamespaces []string) *ConsoleServerCLIConfigBuilder {
	b.i18nNamespaceList = i18nNamespaces
	return b
}

func (b *ConsoleServerCLIConfigBuilder) Proxy(proxyServices []ProxyService) *ConsoleServerCLIConfigBuilder {
	b.proxyServices = proxyServices
	return b
}

func (b *ConsoleServerCLIConfigBuilder) ManagedClusterConfigFile(file string) *ConsoleServerCLIConfigBuilder {
	b.managedClusterConfigFile = file
	return b
}

func (b *ConsoleServerCLIConfigBuilder) TelemetryConfiguration(telemetry map[string]string) *ConsoleServerCLIConfigBuilder {
	b.telemetry = telemetry
	return b
}

func (b *ConsoleServerCLIConfigBuilder) ReleaseVersion() *ConsoleServerCLIConfigBuilder {
	b.releaseVersion = os.Getenv("RELEASE_VERSION")
	return b
}

func (b *ConsoleServerCLIConfigBuilder) NodesArchitecture(architectures []string) *ConsoleServerCLIConfigBuilder {
	b.nodeArchitectures = architectures
	return b
}

func (b *ConsoleServerCLIConfigBuilder) Config() Config {
	return Config{
		Kind:                     "ConsoleConfig",
		APIVersion:               "console.openshift.io/v1",
		Auth:                     b.auth(),
		ClusterInfo:              b.clusterInfo(),
		Customization:            b.customization(),
		ServingInfo:              b.servingInfo(),
		Providers:                b.providers(),
		MonitoringInfo:           b.monitoringInfo(),
		Plugins:                  b.plugins(),
		I18nNamespaces:           b.i18nNamespaces(),
		Proxy:                    b.proxy(),
		ManagedClusterConfigFile: b.managedClusterConfigFile,
		Telemetry:                b.telemetry,
	}
}

func (b *ConsoleServerCLIConfigBuilder) ConfigYAML() (consoleConfigYAML []byte, marshallError error) {
	conf := b.Config()
	yml, err := yaml.Marshal(conf)
	if err != nil {
		klog.V(4).Infof("could not create config yaml %v", err)
		return nil, err
	}
	return yml, nil
}

func (b *ConsoleServerCLIConfigBuilder) servingInfo() ServingInfo {
	conf := ServingInfo{
		BindAddress: "https://[::]:8443",
		CertFile:    certFilePath,
		KeyFile:     keyFilePath,
	}

	if b.customHostnameRedirectPort != 0 {
		conf.RedirectPort = b.customHostnameRedirectPort
	}

	return conf
}

func (b *ConsoleServerCLIConfigBuilder) clusterInfo() ClusterInfo {
	conf := ClusterInfo{
		ConsoleBasePath: "",
	}

	if len(b.apiServerURL) > 0 {
		conf.MasterPublicURL = b.apiServerURL
	}
	if len(b.host) > 0 {
		conf.ConsoleBaseAddress = util.HTTPS(b.host)
	}
	if len(b.controlPlaneToplogy) > 0 {
		conf.ControlPlaneToplogy = b.controlPlaneToplogy
	}
	if len(b.releaseVersion) > 0 {
		conf.ReleaseVersion = b.releaseVersion
	}
	if len(b.nodeArchitectures) > 0 {
		conf.NodeArchitectures = b.nodeArchitectures
	}
	return conf
}

func (b *ConsoleServerCLIConfigBuilder) monitoringInfo() MonitoringInfo {
	conf := MonitoringInfo{}
	if len(b.monitoring) == 0 {
		return conf
	}

	m, err := yaml.Marshal(b.monitoring)
	if err != nil {
		return conf
	}

	var monitoringInfo MonitoringInfo
	err = yaml.Unmarshal(m, &monitoringInfo)
	if err != nil {
		return conf
	}

	if len(monitoringInfo.AlertmanagerUserWorkloadHost) > 0 {
		conf.AlertmanagerUserWorkloadHost = monitoringInfo.AlertmanagerUserWorkloadHost
	}

	if len(monitoringInfo.AlertmanagerTenancyHost) > 0 {
		conf.AlertmanagerTenancyHost = monitoringInfo.AlertmanagerTenancyHost
	}

	return conf
}

func (b *ConsoleServerCLIConfigBuilder) auth() Auth {
	// we need this fallback due to the way our unit test are structured,
	// where the ConsoleServerCLIConfigBuilder object is being instantiated empty
	if b.CAFile == "" {
		b.CAFile = oauthEndpointCAFilePath
	}
	conf := Auth{
		ClientID:                 api.OpenShiftConsoleName,
		ClientSecretFile:         clientSecretFilePath,
		OAuthEndpointCAFile:      b.CAFile,
		InactivityTimeoutSeconds: b.inactivityTimeoutSeconds,
	}
	if len(b.logoutRedirectURL) > 0 {
		conf.LogoutRedirect = b.logoutRedirectURL
	}
	return conf
}

func (b *ConsoleServerCLIConfigBuilder) customization() Customization {
	conf := Customization{}
	if len(b.brand) > 0 {
		conf.Branding = string(b.brand)
	}
	if len(b.docURL) > 0 {
		conf.DocumentationBaseURL = b.docURL
	}
	if len(b.customProductName) > 0 {
		conf.CustomProductName = b.customProductName
	}
	if len(b.customLogoFile) > 0 {
		conf.CustomLogoFile = b.customLogoFile
	}

	if b.devCatalogCustomization.Categories != nil {
		mapMeta := func(meta operatorv1.DeveloperConsoleCatalogCategoryMeta) DeveloperConsoleCatalogCategoryMeta {
			return DeveloperConsoleCatalogCategoryMeta{
				ID:    meta.ID,
				Label: meta.Label,
				Tags:  meta.Tags,
			}
		}

		categories := make([]DeveloperConsoleCatalogCategory, len(b.devCatalogCustomization.Categories))
		for categoryIndex, category := range b.devCatalogCustomization.Categories {
			var subcategories []DeveloperConsoleCatalogCategoryMeta = nil
			if category.Subcategories != nil {
				subcategories = make([]DeveloperConsoleCatalogCategoryMeta, len(category.Subcategories))
				for subcategoryIndex, subcategory := range category.Subcategories {
					subcategories[subcategoryIndex] = mapMeta(subcategory)
				}
			}
			categories[categoryIndex] = DeveloperConsoleCatalogCategory{
				DeveloperConsoleCatalogCategoryMeta: mapMeta(category.DeveloperConsoleCatalogCategoryMeta),
				Subcategories:                       subcategories,
			}
		}

		conf.DeveloperCatalog = &DeveloperConsoleCatalogCustomization{
			Categories: &categories,
		}
	}

	if !reflect.DeepEqual(b.devCatalogCustomization.Types, operatorv1.DeveloperConsoleCatalogTypesState{}) {

		var catalogTypes DeveloperConsoleCatalogTypesState
		var disabledTypes []string = nil
		var enabledTypes []string = nil

		if b.devCatalogCustomization.Types.State == "Disabled" && b.devCatalogCustomization.Types.Disabled != nil {
			disabledTypes = *b.devCatalogCustomization.Types.Disabled
		}

		if b.devCatalogCustomization.Types.State == "Enabled" && b.devCatalogCustomization.Types.Enabled != nil {
			enabledTypes = *b.devCatalogCustomization.Types.Enabled
		}
		catalogTypes = DeveloperConsoleCatalogTypesState{
			State:    CatalogTypesState(b.devCatalogCustomization.Types.State),
			Enabled:  &enabledTypes,
			Disabled: &disabledTypes,
		}
		conf.DeveloperCatalog = &DeveloperConsoleCatalogCustomization{
			Types: catalogTypes,
		}
	}

	if len(b.projectAccess.AvailableClusterRoles) > 0 {
		conf.ProjectAccess = ProjectAccess{
			AvailableClusterRoles: b.projectAccess.AvailableClusterRoles,
		}
	}

	if len(b.quickStarts.Disabled) > 0 {
		conf.QuickStarts = QuickStarts{
			Disabled: b.quickStarts.Disabled,
		}
	}

	conf.AddPage = AddPage{
		DisabledActions: b.addPage.DisabledActions,
	}

	if b.perspectives != nil {
		accessReviewMap := func(accessReview authorizationv1.ResourceAttributes) authorizationv1.ResourceAttributes {
			return authorizationv1.ResourceAttributes{
				Resource: accessReview.Resource,
				Verb:     accessReview.Verb,
				Group:    accessReview.Group,
			}
		}

		perspectives := make([]Perspective, len(b.perspectives))
		for perspectiveIndex, perspective := range b.perspectives {
			var perspectiveVisibility PerspectiveVisibility
			var accessReview *ResourceAttributesAccessReview
			if !reflect.DeepEqual(perspective.Visibility, PerspectiveVisibility{}) {
				if perspective.Visibility.State == "AccessReview" && !reflect.DeepEqual(perspective.Visibility.AccessReview, ResourceAttributesAccessReview{}) {

					var requiredAccessReviews []authorizationv1.ResourceAttributes = nil
					var missingAccessReviews []authorizationv1.ResourceAttributes = nil
					if perspective.Visibility.AccessReview.Required != nil {
						requiredAccessReviews = make([]authorizationv1.ResourceAttributes, len(perspective.Visibility.AccessReview.Required))
						for requiredAccessReviewIndex, requiredAccessReview := range perspective.Visibility.AccessReview.Required {
							requiredAccessReviews[requiredAccessReviewIndex] = accessReviewMap(requiredAccessReview)
						}
					}

					if perspective.Visibility.AccessReview.Missing != nil {
						missingAccessReviews = make([]authorizationv1.ResourceAttributes, len(perspective.Visibility.AccessReview.Missing))
						for missingAccessReviewIndex, missingAccessReview := range perspective.Visibility.AccessReview.Missing {
							missingAccessReviews[missingAccessReviewIndex] = accessReviewMap(missingAccessReview)
						}
					}
					accessReview = &ResourceAttributesAccessReview{
						Required: requiredAccessReviews,
						Missing:  missingAccessReviews,
					}
					perspectiveVisibility = PerspectiveVisibility{
						State:        PerspectiveState(perspective.Visibility.State),
						AccessReview: accessReview,
					}
				} else {
					perspectiveVisibility = PerspectiveVisibility{
						State: PerspectiveState(perspective.Visibility.State),
					}
				}
			}
			perspectives[perspectiveIndex] = Perspective{
				ID:         perspective.ID,
				Visibility: perspectiveVisibility,
			}
		}

		conf.Perspectives = perspectives
	}

	return conf
}

func (b *ConsoleServerCLIConfigBuilder) providers() Providers {
	if len(b.statusPageID) > 0 {
		return Providers{
			StatuspageID: b.statusPageID,
		}
	}
	return Providers{}
}

func (b *ConsoleServerCLIConfigBuilder) plugins() map[string]string {
	return b.pluginsList
}

func (b *ConsoleServerCLIConfigBuilder) i18nNamespaces() []string {
	return b.i18nNamespaceList
}

func (b *ConsoleServerCLIConfigBuilder) proxy() Proxy {
	return Proxy{
		Services: b.proxyServices,
	}
}

func (b *ConsoleServerCLIConfigBuilder) Telemetry() map[string]string {
	return b.telemetry
}
