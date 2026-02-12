package configmap

import (
	"fmt"
	"net/url"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/api/console/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/bindata"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
	infrastructuresub "github.com/openshift/console-operator/pkg/console/subresource/infrastructure"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
)

const (
	consoleConfigYamlFile = "console-config.yaml"
	defaultLogoutURL      = ""
	pluginProxyEndpoint   = "/api/proxy/plugin/"
)

func statusPageId(operatorConfig *operatorv1.Console) string {
	if operatorConfig.Spec.Providers.Statuspage != nil {
		return operatorConfig.Spec.Providers.Statuspage.PageID
	}
	return ""
}

func DefaultConfigMap(
	operatorConfig *operatorv1.Console,
	consoleConfig *configv1.Console,
	authConfig *configv1.Authentication,
	managedConfig *corev1.ConfigMap,
	monitoringSharedConfig *corev1.ConfigMap,
	infrastructureConfig *configv1.Infrastructure,
	activeConsoleRoute *routev1.Route,
	inactivityTimeoutSeconds int,
	availablePlugins []*v1.ConsolePlugin,
	nodeArchitectures []string,
	nodeOperatingSystems []string,
	copiedCSVsDisabled bool,
	telemeterConfig map[string]string,
	consoleHost string,
	techPreviewEnabled bool,
) (consoleConfigMap *corev1.ConfigMap, unsupportedOverridesHaveMerged bool, err error) {

	apiServerURL := infrastructuresub.GetAPIServerURL(infrastructureConfig)

	defaultBuilder := &consoleserver.ConsoleServerCLIConfigBuilder{}
	defaultConfig, err := defaultBuilder.Host(consoleHost).
		LogoutURL(defaultLogoutURL).
		Brand(DEFAULT_BRAND).
		DocURL(DEFAULT_DOC_URL).
		APIServerURL(apiServerURL).
		Monitoring(monitoringSharedConfig).
		InactivityTimeout(inactivityTimeoutSeconds).
		ReleaseVersion().
		NodeArchitectures(nodeArchitectures).
		NodeOperatingSystems(nodeOperatingSystems).
		CopiedCSVsDisabled(copiedCSVsDisabled).
		TechPreviewEnabled(techPreviewEnabled).
		ConfigYAML()
	if err != nil {
		klog.Errorf("failed to generate default console-config config: %v", err)
		return nil, false, err
	}

	extractedManagedConfig := extractYAML(managedConfig)
	userDefinedBuilder := &consoleserver.ConsoleServerCLIConfigBuilder{}
	if activeConsoleRoute != nil {
		userDefinedBuilder = userDefinedBuilder.CustomHostnameRedirectPort(isCustomRoute(activeConsoleRoute))
	}
	userDefinedConfig, err := userDefinedBuilder.Host(consoleHost).
		LogoutURL(consoleConfig.Spec.Authentication.LogoutRedirect).
		Brand(operatorConfig.Spec.Customization.Brand).
		DocURL(operatorConfig.Spec.Customization.DocumentationBaseURL).
		APIServerURL(apiServerURL).
		TopologyMode(infrastructureConfig.Status.ControlPlaneTopology).
		Monitoring(monitoringSharedConfig).
		Plugins(getPluginsEndpointMap(availablePlugins)).
		PluginsOrder(availablePlugins, operatorConfig).
		I18nNamespaces(pluginsWithI18nNamespace(availablePlugins)).
		ContentSecurityPolicies(aggregateCSPDirectives(availablePlugins)).
		Proxy(getPluginsProxyServices(availablePlugins)).
		CustomLogoFile(operatorConfig.Spec.Customization.CustomLogoFile). // TODO Remove deprecated CustomLogoFile API.
		CustomLogos(operatorConfig.Spec.Customization.Logos).
		CustomProductName(operatorConfig.Spec.Customization.CustomProductName).
		CustomDeveloperCatalog(operatorConfig.Spec.Customization.DeveloperCatalog).
		ProjectAccess(operatorConfig.Spec.Customization.ProjectAccess).
		QuickStarts(operatorConfig.Spec.Customization.QuickStarts).
		AddPage(operatorConfig.Spec.Customization.AddPage).
		Perspectives(operatorConfig.Spec.Customization.Perspectives).
		StatusPageID(statusPageId(operatorConfig)).
		InactivityTimeout(inactivityTimeoutSeconds).
		TelemetryConfiguration(telemeterConfig).
		ReleaseVersion().
		NodeArchitectures(nodeArchitectures).
		NodeOperatingSystems(nodeOperatingSystems).
		AuthConfig(authConfig, apiServerURL).
		Capabilities(operatorConfig.Spec.Customization.Capabilities).
		TechPreviewEnabled(techPreviewEnabled).
		ConfigYAML()
	if err != nil {
		klog.Errorf("failed to generate user defined console-config config: %v", err)
		return nil, false, err
	}

	unsupportedConfigOverride := operatorConfig.Spec.UnsupportedConfigOverrides.Raw
	willMergeConfigOverrides := len(unsupportedConfigOverride) != 0
	if willMergeConfigOverrides {
		klog.V(4).Infoln(fmt.Sprintf("with UnsupportedConfigOverrides: %v", string(unsupportedConfigOverride)))
	}

	merger := &consoleserver.ConsoleYAMLMerger{}
	mergedConfig, err := merger.Merge(
		defaultConfig,
		extractedManagedConfig,
		userDefinedConfig,
		unsupportedConfigOverride)
	if err != nil {
		klog.Errorf("failed to generate configmap: %v", err)
		return nil, false, err
	}

	configMap := Stub()
	configMap.Data = map[string]string{}
	configMap.Data[consoleConfigYamlFile] = string(mergedConfig)
	util.AddOwnerRef(configMap, util.OwnerRefFrom(operatorConfig))

	return configMap, willMergeConfigOverrides, nil
}

func aggregateCSPDirectives(plugins []*v1.ConsolePlugin) map[v1.DirectiveType][]string {
	aggregated := make(map[v1.DirectiveType]map[string]struct{}) // Use a map to ensure uniqueness

	for _, plugin := range plugins {
		for _, csp := range plugin.Spec.ContentSecurityPolicy {
			if aggregated[csp.Directive] == nil {
				aggregated[csp.Directive] = make(map[string]struct{}) // Initialize if not already done
			}
			for _, v := range csp.Values {
				stringValue := string(v)
				aggregated[csp.Directive][stringValue] = struct{}{} // Use empty struct for uniqueness
			}
		}
	}
	// Check if the aggregated map is empty
	if len(aggregated) == 0 {
		return nil
	}

	// Convert back to the desired format
	result := make(map[v1.DirectiveType][]string)
	for directive, valuesMap := range aggregated {
		result[directive] = make([]string, 0, len(valuesMap))
		for value := range valuesMap {
			result[directive] = append(result[directive], value)
		}
		// Sort the slice of strings for each directive
		sort.Strings(result[directive])
	}

	return result
}

func pluginsWithI18nNamespace(availablePlugins []*v1.ConsolePlugin) []string {
	i18nNamespaces := []string{}
	for _, plugin := range availablePlugins {
		if plugin.Spec.I18n.LoadType == v1.Preload {
			i18nNamespaces = append(i18nNamespaces, fmt.Sprintf("plugin__%s", plugin.Name))
		}
	}
	return i18nNamespaces
}

func getPluginsEndpointMap(availablePlugins []*v1.ConsolePlugin) map[string]string {
	pluginsEndpointMap := map[string]string{}
	for _, plugin := range availablePlugins {
		switch plugin.Spec.Backend.Type {
		case v1.Service:
			pluginsEndpointMap[plugin.Name] = getServiceURL(&plugin.Spec.Backend)
		default:
			klog.Errorf("unknown backend type for %q plugin: %q. Currently only %q backend type is supported.", plugin.Name, plugin.Spec.Backend.Type, v1.Service)
		}
	}
	return pluginsEndpointMap
}

func getPluginsProxyServices(availablePlugins []*v1.ConsolePlugin) []consoleserver.ProxyService {
	proxyServices := []consoleserver.ProxyService{}
	for _, plugin := range availablePlugins {
		for _, proxy := range plugin.Spec.Proxy {
			// currently we only supprot 'Service' backend type for proxy
			switch proxy.Endpoint.Type {
			case v1.ProxyTypeService:
				proxyService := consoleserver.ProxyService{
					ConsoleAPIPath: getConsoleAPIPath(plugin.Name, &proxy),
					Endpoint:       getProxyServiceURL(proxy.Endpoint.Service),
					CACertificate:  proxy.CACertificate,
					Authorize:      getProxyAuthorization(proxy.Authorization),
				}
				proxyServices = append(proxyServices, proxyService)
			default:
				klog.Errorf("unknown proxy service type for %q plugin: %q. Currently only %q proxy endpoint type is supported.", plugin.Name, proxy.Endpoint.Type, v1.ProxyTypeService)
			}
		}
	}
	return proxyServices
}

func getConsoleAPIPath(pluginName string, service *v1.ConsolePluginProxy) string {
	return fmt.Sprintf("%s%s/%s/", pluginProxyEndpoint, pluginName, service.Alias)
}

func getProxyAuthorization(authorizationType v1.AuthorizationType) bool {
	if authorizationType == v1.UserToken {
		return true
	}
	return false
}

func getProxyServiceURL(service *v1.ConsolePluginProxyServiceConfig) string {
	pluginURL := &url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s.%s.svc.cluster.local:%d", service.Name, service.Namespace, service.Port),
	}
	return pluginURL.String()
}

func getServiceURL(pluginBackend *v1.ConsolePluginBackend) string {
	pluginURL := &url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s.%s.svc.cluster.local:%d", pluginBackend.Service.Name, pluginBackend.Service.Namespace, pluginBackend.Service.Port),
		Path:   pluginBackend.Service.BasePath,
	}
	return pluginURL.String()
}

func isCustomRoute(activeRoute *routev1.Route) bool {
	return activeRoute.GetName() == api.OpenshiftConsoleCustomRouteName
}

func DefaultPublicConfig(consoleURL string) *corev1.ConfigMap {
	config := resourceread.ReadConfigMapV1OrDie(bindata.MustAsset("assets/configmaps/console-public-configmap.yaml"))
	config.Data = map[string]string{
		"consoleURL": consoleURL,
	}
	return config
}

func EmptyPublicConfig() *corev1.ConfigMap {
	config := resourceread.ReadConfigMapV1OrDie(bindata.MustAsset("assets/configmaps/console-public-configmap.yaml"))
	config.Data = map[string]string{}
	return config
}

func ConsoleConfigMapStub() *corev1.ConfigMap {
	return resourceread.ReadConfigMapV1OrDie(bindata.MustAsset("assets/configmaps/console-configmap.yaml"))
}

func Stub() *corev1.ConfigMap {
	configMap := ConsoleConfigMapStub()
	configMap.Name = api.OpenShiftConsoleConfigMapName
	return configMap
}

func consoleBaseAddr(host string) string {
	return util.HTTPS(host)
}

// Helper function that pulls the yaml struct out of the data section of a configmap yaml
func extractYAML(managedConfig *corev1.ConfigMap) []byte {
	data := managedConfig.Data
	for _, v := range data {
		return []byte(v)
	}

	return []byte{}
}
