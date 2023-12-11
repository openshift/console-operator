package configmap

import (
	"fmt"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/api/console/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/bindata"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
)

const (
	consoleConfigYamlFile     = "console-config.yaml"
	defaultLogoutURL          = ""
	pluginProxyEndpoint       = "/api/proxy/plugin/"
	telemetryAnnotationPrefix = "telemetry.console.openshift.io/"
)

func getApiUrl(infrastructureConfig *configv1.Infrastructure) string {
	if infrastructureConfig != nil {
		return infrastructureConfig.Status.APIServerURL
	}
	return ""
}

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
	authServerCAConfig *corev1.ConfigMap,
	managedConfig *corev1.ConfigMap,
	monitoringSharedConfig *corev1.ConfigMap,
	infrastructureConfig *configv1.Infrastructure,
	activeConsoleRoute *routev1.Route,
	inactivityTimeoutSeconds int,
	availablePlugins []*v1.ConsolePlugin,
	nodeArchitectures []string,
	nodeOperatingSystems []string,
	copiedCSVsDisabled bool,
) (consoleConfigMap *corev1.ConfigMap, unsupportedOverridesHaveMerged bool, err error) {

	defaultBuilder := &consoleserver.ConsoleServerCLIConfigBuilder{}
	defaultConfig, err := defaultBuilder.Host(activeConsoleRoute.Spec.Host).
		LogoutURL(defaultLogoutURL).
		Brand(DEFAULT_BRAND).
		DocURL(DEFAULT_DOC_URL).
		APIServerURL(getApiUrl(infrastructureConfig)).
		Monitoring(monitoringSharedConfig).
		InactivityTimeout(inactivityTimeoutSeconds).
		ReleaseVersion().
		NodeArchitectures(nodeArchitectures).
		NodeOperatingSystems(nodeOperatingSystems).
		CopiedCSVsDisabled(copiedCSVsDisabled).
		ConfigYAML()
	if err != nil {
		klog.Errorf("failed to generate default console-config config: %v", err)
		return nil, false, err
	}

	extractedManagedConfig := extractYAML(managedConfig)
	userDefinedBuilder := &consoleserver.ConsoleServerCLIConfigBuilder{}
	userDefinedConfig, err := userDefinedBuilder.Host(activeConsoleRoute.Spec.Host).
		LogoutURL(consoleConfig.Spec.Authentication.LogoutRedirect).
		Brand(operatorConfig.Spec.Customization.Brand).
		DocURL(operatorConfig.Spec.Customization.DocumentationBaseURL).
		APIServerURL(getApiUrl(infrastructureConfig)).
		TopologyMode(infrastructureConfig.Status.ControlPlaneTopology).
		Monitoring(monitoringSharedConfig).
		Plugins(getPluginsEndpointMap(availablePlugins)).
		I18nNamespaces(pluginsWithI18nNamespace(availablePlugins)).
		Proxy(getPluginsProxyServices(availablePlugins)).
		CustomLogoFile(operatorConfig.Spec.Customization.CustomLogoFile.Key).
		CustomProductName(operatorConfig.Spec.Customization.CustomProductName).
		CustomDeveloperCatalog(operatorConfig.Spec.Customization.DeveloperCatalog).
		ProjectAccess(operatorConfig.Spec.Customization.ProjectAccess).
		QuickStarts(operatorConfig.Spec.Customization.QuickStarts).
		CustomHostnameRedirectPort(isCustomRoute(activeConsoleRoute)).
		AddPage(operatorConfig.Spec.Customization.AddPage).
		Perspectives(operatorConfig.Spec.Customization.Perspectives).
		StatusPageID(statusPageId(operatorConfig)).
		InactivityTimeout(inactivityTimeoutSeconds).
		TelemetryConfiguration(GetTelemetryConfiguration(operatorConfig)).
		ReleaseVersion().
		NodeArchitectures(nodeArchitectures).
		NodeOperatingSystems(nodeOperatingSystems).
		AuthConfig(authConfig, authServerCAConfig).
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

func GetTelemetryConfiguration(operatorConfig *operatorv1.Console) map[string]string {
	telemetry := make(map[string]string)
	if len(operatorConfig.Annotations) > 0 {
		for k, v := range operatorConfig.Annotations {
			if strings.HasPrefix(k, telemetryAnnotationPrefix) && len(k) > len(telemetryAnnotationPrefix) {
				telemetry[k[len(telemetryAnnotationPrefix):]] = v
			}
		}
	}
	return telemetry
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
