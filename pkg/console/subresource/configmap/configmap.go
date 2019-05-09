package configmap

import (
	"fmt"

	"github.com/sirupsen/logrus"

	yaml2 "github.com/ghodss/yaml"
	yaml "gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

const (
	consoleConfigYamlFile   = "console-config.yaml"
	clientSecretFilePath    = "/var/oauth-config/clientSecret"
	oauthEndpointCAFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	// serving info
	certFilePath = "/var/serving-cert/tls.crt"
	keyFilePath  = "/var/serving-cert/tls.key"
)

// overridden by console config
const (
	defaultLogoutURL = ""
)

func getApiUrl(infrastructureConfig *configv1.Infrastructure) string {
	if infrastructureConfig != nil {
		return infrastructureConfig.Status.APIServerURL
	}
	return ""
}

// DefaultConfigMap returns
// - a new configmap,
// - a bool indicating if config was merged (unsupportedConfigOverrides)
// - an error
func DefaultConfigMap(operatorConfig *operatorv1.Console, consoleConfig *configv1.Console, managedConfig *corev1.ConfigMap, infrastructureConfig *configv1.Infrastructure, rt *routev1.Route) (*corev1.ConfigMap, bool, error) {

	// Build a default config that can be overwritten from other sources
	host := rt.Spec.Host
	apiServerURL := getApiUrl(infrastructureConfig)
	defaultConfig := NewYamlConfig(host, defaultLogoutURL, DEFAULT_BRAND, DEFAULT_DOC_URL, apiServerURL)
	// Get console config from the openshift-config-managed namespace
	extractedManagedConfig := extractYAML(managedConfig)

	// Derive a user defined config using operator config
	logoutRedirect := consoleConfig.Spec.Authentication.LogoutRedirect
	brand := operatorConfig.Spec.Customization.Brand
	docURL := operatorConfig.Spec.Customization.DocumentationBaseURL
	userDefinedConfig := NewYamlConfig(host, logoutRedirect, brand, docURL, apiServerURL)

	unsupportedConfigOverride := operatorConfig.Spec.UnsupportedConfigOverrides.Raw

	// merge configs with overrides, if we have them
	mergedConfig, err := resourcemerge.MergeProcessConfig(nil, defaultConfig, extractedManagedConfig, userDefinedConfig, unsupportedConfigOverride)
	if err != nil {
		logrus.Errorf("failed to merge configmap: %v \n", err)
		return nil, false, err
	}

	outConfigYaml, err := yaml2.JSONToYAML(mergedConfig)
	if err != nil {
		logrus.Errorf("failed to generate configmap: %v \n", err)
		return nil, false, err
	}

	// if we actually merged config overrides, log this information
	didMerge := len(operatorConfig.Spec.UnsupportedConfigOverrides.Raw) != 0
	if didMerge {
		logrus.Println(fmt.Sprintf("with UnsupportedConfigOverrides: %v", string(unsupportedConfigOverride)))
	}

	configMap := Stub()
	configMap.Data = map[string]string{}
	configMap.Data[consoleConfigYamlFile] = string(outConfigYaml)
	util.AddOwnerRef(configMap, util.OwnerRefFrom(operatorConfig))

	return configMap, didMerge, nil
}

func DefaultPublicConfig(consoleURL string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.OpenShiftConsolePublicConfigMapName,
			Namespace: api.OpenShiftConfigManagedNamespace,
		},
		Data: map[string]string{
			"consoleURL": consoleURL,
		},
	}
}

func EmptyPublicConfig() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.OpenShiftConsolePublicConfigMapName,
			Namespace: api.OpenShiftConfigManagedNamespace,
		},
		Data: map[string]string{},
	}
}

func Stub() *corev1.ConfigMap {
	meta := util.SharedMeta()
	meta.Name = api.OpenShiftConsoleConfigMapName
	configMap := &corev1.ConfigMap{
		ObjectMeta: meta,
	}
	return configMap
}

func NewYamlConfig(host string, logoutRedirect string, brand operatorv1.Brand, docURL string, apiServerURL string) []byte {
	conf := yaml.MapSlice{
		{
			Key: "kind", Value: "ConsoleConfig",
		}, {
			Key: "apiVersion", Value: "console.openshift.io/v1",
		}, {
			Key: "auth", Value: authServerYaml(logoutRedirect),
		}, {
			Key: "clusterInfo", Value: clusterInfo(host, apiServerURL),
		}, {
			Key: "customization", Value: customization(brand, docURL),
		}, {
			Key: "servingInfo", Value: servingInfo(),
		},
	}
	yml, err := yaml.Marshal(conf)
	if err != nil {
		fmt.Printf("Could not create config yaml %v", err)
		return nil
	}
	return yml
}

func servingInfo() yaml.MapSlice {
	return yaml.MapSlice{
		{
			Key: "bindAddress", Value: "https://0.0.0.0:8443",
		}, {
			Key: "certFile", Value: certFilePath,
		}, {
			Key: "keyFile", Value: keyFilePath,
		},
	}
}

func customization(brand operatorv1.Brand, docURL string) map[string]string {
	tmpMap := make(map[string]string)
	if brand != "" {
		tmpMap["branding"] = string(brand)
	}
	if docURL != "" {
		tmpMap["documentationBaseURL"] = docURL
	}
	return tmpMap
}

func clusterInfo(host string, apiServerURL string) yaml.MapSlice {
	return yaml.MapSlice{
		{
			Key: "consoleBaseAddress", Value: consoleBaseAddr(host),
		}, {
			Key: "consoleBasePath", Value: "",
		}, {
			Key: "masterPublicURL", Value: apiServerURL,
		},
	}

}

func authServerYaml(logoutRedirect string) yaml.MapSlice {
	return yaml.MapSlice{
		{
			Key: "clientID", Value: api.OpenShiftConsoleName,
		}, {
			Key: "clientSecretFile", Value: clientSecretFilePath,
		}, {
			Key: "logoutRedirect", Value: logoutRedirect,
		}, {
			Key: "oauthEndpointCAFile", Value: oauthEndpointCAFilePath,
		},
	}
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
