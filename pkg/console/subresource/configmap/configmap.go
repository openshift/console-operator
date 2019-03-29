package configmap

import (
	"fmt"
	"github.com/sirupsen/logrus"

	yaml2 "github.com/ghodss/yaml"
	"gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

const (
	ConsoleConfigMapName    = "console-config"
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

func getLogoutRedirect(consoleConfig *configv1.Console) string {
	if len(consoleConfig.Spec.Authentication.LogoutRedirect) > 0 {
		return consoleConfig.Spec.Authentication.LogoutRedirect
	}
	return defaultLogoutURL
}

func getBrand(operatorConfig *operatorv1.Console) operatorv1.Brand {
	if len(operatorConfig.Spec.Customization.Brand) > 0 {
		return operatorConfig.Spec.Customization.Brand
	}
	return DEFAULT_BRAND
}

func getDocURL(operatorConfig *operatorv1.Console) string {
	if len(operatorConfig.Spec.Customization.DocumentationBaseURL) > 0 {
		return operatorConfig.Spec.Customization.DocumentationBaseURL
	}
	return DEFAULT_DOC_URL
}

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
	logoutRedirect := getLogoutRedirect(consoleConfig)
	brand := getBrand(operatorConfig)
	docURL := getDocURL(operatorConfig)
	apiServerURL := getApiUrl(infrastructureConfig)

	host := rt.Spec.Host
	config := NewYamlConfig(host, logoutRedirect, brand, docURL, apiServerURL)

	configMap := Stub()
	configMap.Data = map[string]string{}
	unsupportedRaw := operatorConfig.Spec.UnsupportedConfigOverrides.Raw
	newConfig := extractYAML(managedConfig)

	// merge config overrides, if we have them
	mergedConfig, err := resourcemerge.MergeProcessConfig(nil, config, newConfig, unsupportedRaw)
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
		logrus.Println(fmt.Sprintf("With UnsupportedConfigOverrides: %v", string(unsupportedRaw)))
	}

	configMap.Data[consoleConfigYamlFile] = string(outConfigYaml)

	logrus.Println("generated console config yaml:")
	logrus.Printf("%s \n", string(outConfigYaml))
	util.AddOwnerRef(configMap, util.OwnerRefFrom(operatorConfig))

	return configMap, didMerge, nil
}

func Stub() *corev1.ConfigMap {
	meta := util.SharedMeta()
	meta.Name = ConsoleConfigMapName
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
			Key: "apiVersion", Value: "console.openshift.io/v1beta1",
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

func customization(brand operatorv1.Brand, docURL string) yaml.MapSlice {
	return yaml.MapSlice{
		{
			// TODO: branding will need to be provided by higher level config.
			// it should not be configurable in the CR, but needs to be configured somewhere.
			Key: "branding", Value: brand,
		}, {
			Key: "documentationBaseURL", Value: docURL,
		},
	}
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
