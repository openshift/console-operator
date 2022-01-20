package configmap

import (
	"fmt"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/api/console/v1alpha1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/assets"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
)

const (
	consoleConfigYamlFile = "console-config.yaml"
	defaultLogoutURL      = ""
	pluginProxyEndpoint   = "/api/proxy/plugin/"
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
	managedConfig *corev1.ConfigMap,
	infrastructureConfig *configv1.Infrastructure,
	activeConsoleRoute *routev1.Route,
	useDefaultCAFile bool,
	inactivityTimeoutSeconds int,
	availablePlugins []*v1alpha1.ConsolePlugin) (consoleConfigmap *corev1.ConfigMap, unsupportedOverridesHaveMerged bool, err error) {

	defaultBuilder := &consoleserver.ConsoleServerCLIConfigBuilder{}
	defaultConfig, err := defaultBuilder.Host(activeConsoleRoute.Spec.Host).
		LogoutURL(defaultLogoutURL).
		Brand(DEFAULT_BRAND).
		DocURL(DEFAULT_DOC_URL).
		OAuthServingCert(useDefaultCAFile).
		APIServerURL(getApiUrl(infrastructureConfig)).
		InactivityTimeout(inactivityTimeoutSeconds).
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
		OAuthServingCert(useDefaultCAFile).
		APIServerURL(getApiUrl(infrastructureConfig)).
		Plugins(GetPluginsEndpointMap(availablePlugins)).
		Proxy(GetPluginsProxyServices(availablePlugins)).
		CustomLogoFile(operatorConfig.Spec.Customization.CustomLogoFile.Key).
		CustomProductName(operatorConfig.Spec.Customization.CustomProductName).
		CustomDeveloperCatalog(operatorConfig.Spec.Customization.DeveloperCatalog).
		ProjectAccess(operatorConfig.Spec.Customization.ProjectAccess).
		QuickStarts(operatorConfig.Spec.Customization.QuickStarts).
		CustomHostnameRedirectPort(isCustomRoute(activeConsoleRoute)).
		AddPage(operatorConfig.Spec.Customization.AddPage).
		StatusPageID(statusPageId(operatorConfig)).
		InactivityTimeout(inactivityTimeoutSeconds).
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

func GetPluginsEndpointMap(availablePlugins []*v1alpha1.ConsolePlugin) map[string]string {
	pluginsEndpointMap := map[string]string{}
	for _, plugin := range availablePlugins {
		pluginsEndpointMap[plugin.Name] = getServiceURL(plugin)
	}
	return pluginsEndpointMap
}

func GetPluginsProxyServices(availablePlugins []*v1alpha1.ConsolePlugin) []consoleserver.ProxyService {
	proxyServices := []consoleserver.ProxyService{}
	for _, plugin := range availablePlugins {
		for _, proxy := range plugin.Spec.Proxy {
			if proxy.Type == v1alpha1.ProxyTypeService {
				proxyService := consoleserver.ProxyService{
					ConsoleAPIPath: getConsoleAPIPath(plugin.Name, &proxy),
					Endpoint:       getProxyServiceURL(&proxy.Service),
					CACertificate:  proxy.CACertificate,
					Authorize:      proxy.Authorize,
				}
				proxyServices = append(proxyServices, proxyService)
			}
		}
	}
	return proxyServices
}

func getConsoleAPIPath(pluginName string, service *v1alpha1.ConsolePluginProxy) string {
	return fmt.Sprintf("%s%s/%s/", pluginProxyEndpoint, pluginName, service.Alias)
}

func getProxyServiceURL(service *v1alpha1.ConsolePluginProxyServiceConfig) string {
	pluginURL := &url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s.%s.svc.cluster.local:%d", service.Name, service.Namespace, service.Port),
	}
	return pluginURL.String()
}

func getServiceURL(plugin *v1alpha1.ConsolePlugin) string {
	pluginURL := &url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s.%s.svc.cluster.local:%d", plugin.Spec.Service.Name, plugin.Spec.Service.Namespace, plugin.Spec.Service.Port),
		Path:   plugin.Spec.Service.BasePath,
	}
	return pluginURL.String()
}

func isCustomRoute(activeRoute *routev1.Route) bool {
	return activeRoute.GetName() == api.OpenshiftConsoleCustomRouteName
}

func DefaultPublicConfig(consoleURL string) *corev1.ConfigMap {
	config := resourceread.ReadConfigMapV1OrDie(assets.MustAsset("configmaps/console-public-configmap.yaml"))
	config.Data = map[string]string{
		"consoleURL": consoleURL,
	}
	return config
}

func EmptyPublicConfig() *corev1.ConfigMap {
	config := resourceread.ReadConfigMapV1OrDie(assets.MustAsset("configmaps/console-public-configmap.yaml"))
	config.Data = map[string]string{}
	return config
}

func Stub() *corev1.ConfigMap {
	meta := util.SharedMeta()
	meta.Name = api.OpenShiftConsoleConfigMapName
	configMap := &corev1.ConfigMap{
		ObjectMeta: meta,
	}
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
