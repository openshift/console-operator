package configmap

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
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

func DefaultConfigMap(operatorConfig *operatorv1.Console, consoleConfig *configv1.Console, rt *routev1.Route) *corev1.ConfigMap {

	logoutRedirect := getLogoutRedirect(consoleConfig)
	brand := getBrand(operatorConfig)
	docURL := getDocURL(operatorConfig)

	host := rt.Spec.Host
	config := string(NewYamlConfig(host, logoutRedirect, brand, docURL))
	configMap := Stub()
	configMap.Data = map[string]string{
		consoleConfigYamlFile: config,
	}

	util.AddOwnerRef(configMap, util.OwnerRefFrom(operatorConfig))
	return configMap
}

func Stub() *corev1.ConfigMap {
	meta := util.SharedMeta()
	meta.Name = ConsoleConfigMapName
	configMap := &corev1.ConfigMap{
		ObjectMeta: meta,
	}
	return configMap
}

func NewYamlConfig(host string, logoutRedirect string, brand operatorv1.Brand, docURL string) []byte {
	conf := yaml.MapSlice{
		{
			Key: "kind", Value: "ConsoleConfig",
		}, {
			Key: "apiVersion", Value: "console.openshift.io/v1beta1",
		}, {
			Key: "auth", Value: authServerYaml(logoutRedirect),
		}, {
			Key: "clusterInfo", Value: clusterInfo(host),
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

func clusterInfo(host string) yaml.MapSlice {
	return yaml.MapSlice{
		{
			Key: "consoleBaseAddress", Value: consoleBaseAddr(host),
		}, {
			Key: "consoleBasePath", Value: "",
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
