package configmap

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"

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
	// TODO: should this be configurable?  likely so.
	documentationBaseURL = "https://docs.okd.io/4.0/"
	brandingDefault      = "okd"
	// serving info
	certFilePath = "/var/serving-cert/tls.crt"
	keyFilePath  = "/var/serving-cert/tls.key"
)

func DefaultConfigMap(cr *operatorv1.Console, rt *routev1.Route) *corev1.ConfigMap {
	host := rt.Spec.Host
	config := NewYamlConfigString(host)
	configMap := Stub()
	configMap.Data = map[string]string{
		consoleConfigYamlFile: config,
	}

	util.AddOwnerRef(configMap, util.OwnerRefFrom(cr))
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

func NewYamlConfigString(host string) string {
	return string(NewYamlConfig(host))
}

func NewYamlConfig(host string) []byte {
	conf := yaml.MapSlice{
		{
			Key: "kind", Value: "ConsoleConfig",
		}, {
			Key: "apiVersion", Value: "console.openshift.io/v1beta1",
		}, {
			Key: "auth", Value: authServerYaml(),
		}, {
			Key: "clusterInfo", Value: clusterInfo(host),
		}, {
			Key: "customization", Value: customization(),
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

func customization() yaml.MapSlice {
	return yaml.MapSlice{
		{
			// TODO: branding will need to be provided by higher level config.
			// it should not be configurable in the CR, but needs to be configured somewhere.
			Key: "branding", Value: brandingDefault,
		}, {
			Key: "documentationBaseURL", Value: documentationBaseURL,
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

func authServerYaml() yaml.MapSlice {
	return yaml.MapSlice{
		{
			Key: "clientID", Value: api.OpenShiftConsoleName,
		}, {
			Key: "clientSecretFile", Value: clientSecretFilePath,
		}, {
			Key: "logoutRedirect", Value: "",
		}, {
			Key: "oauthEndpointCAFile", Value: oauthEndpointCAFilePath,
		},
	}
}

func consoleBaseAddr(host string) string {
	return util.HTTPS(host)
}
