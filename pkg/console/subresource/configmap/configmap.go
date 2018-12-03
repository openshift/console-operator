package configmap

import (
	"fmt"
	"github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/console-operator/pkg/controller"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
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

func DefaultConfigMap(cr *v1alpha1.Console, rt *v1.Route) *corev1.ConfigMap {
	// NOTE: this should probably just take the route.Spec.Host string.
	// without the host, the CR should not be created, it is essential for
	// the deployment to be created correctly.
	if rt == nil {
		return nil
	}
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

func Ref() *corev1.ObjectReference {
	stub := Stub()
	return &corev1.ObjectReference{
		Kind: "ConfigMap",
		Namespace: stub.ObjectMeta.Namespace,
		Name: stub.ObjectMeta.Name,
	}
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
			Key: "clientID", Value: controller.OpenShiftConsoleName,
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
