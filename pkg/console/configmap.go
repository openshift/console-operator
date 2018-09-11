package console

import (
	"fmt"
	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"gopkg.in/yaml.v2"
)

const (
	consoleConfigYamlFile = "console-config.yaml"
	clientSecretFilePath = "/var/oauth-config/clientSecret"
	oauthEndpointCAFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	documentationBaseURL = "https://docs.okd.io/3.11/"
	brandingDefault = "okd"
	// serving info
	certFilePath = "/var/serving-cert/tls.crt"
	keyFilePath = "/var/serving-cert/tls.key"
)

func authServerYaml() yaml.MapSlice {
	return yaml.MapSlice{
		{
			Key: "clientID", Value: openshiftConsoleName,
		}, {
			Key: "clientSecretFile", Value: clientSecretFilePath,
		}, {
			Key: "logoutRedirect", Value: nil,
		}, {
			Key: "oauthEndpointCAFile", Value: oauthEndpointCAFilePath,
		},
	}
}

// TODO: this can take args as we update locations after we generate a router
func clusterInfo() yaml.MapSlice {
	return yaml.MapSlice{
		{
			Key: "consoleBaseAddress", Value: nil,
		}, {
			Key: "consoleBasePath", Value: nil,
		}, {
			Key: "developerConsolePublicURL", Value: nil,
		}, {
			Key: "masterPublicURL", Value: nil,
		},
	}
}

// TODO: take args as we update branding based on cluster config?
func customization() yaml.MapSlice {
	return yaml.MapSlice{
		{
			Key: "branding", Value: brandingDefault,
		}, {
			Key: "documentationBaseURL", Value: documentationBaseURL,
		},
	}
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

// Generates our embedded yaml file
// There may be a better way to do this, lets improve if we can.
func newConsoleConfigYaml() string {
	// https://godoc.org/gopkg.in/yaml.v2#MapSlice
	conf := yaml.MapSlice{
		{
			Key: "kind", Value: "ConsoleConfig",
		}, {
			Key: "apiVersion", Value: "console.openshift.io/v1beta1",
		}, {
			Key: "auth", Value: authServerYaml(),
		}, {
			Key: "clusterInfo", Value: clusterInfo(),
		}, {
			Key: "customization", Value: customization(),
		}, {
			Key: "servingInfo", Value: servingInfo(),
		},
	}

	yml, err := yaml.Marshal(conf)
	if err != nil {
		fmt.Printf("Could not create config yaml %v", err)
		return ""
	}
	return string(yml)
}

func newConsoleConfigMap(cr *v1alpha1.Console) *corev1.ConfigMap {
	meta := sharedMeta()
	meta.Name = "console-config" // expects a different name
	config := newConsoleConfigYaml()

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind: "ConfigMap",
		},
		ObjectMeta: meta,
		Data: map[string]string{
			// haha, duplicate!  fix me
			consoleConfigYamlFile: config,
		},
	}
}