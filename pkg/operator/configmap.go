package operator

import (
	"fmt"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	routev1 "github.com/openshift/api/route/v1"
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

func consoleBaseAddr(host string) string {
	if host != "" {
		logrus.Infof("console configmap base addr set to https://%v", host)
		return fmt.Sprintf("https://%s", host)
	}
	return ""
}

func authServerYaml() yaml.MapSlice {
	return yaml.MapSlice{
		{
			Key: "clientID", Value: OpenShiftConsoleName,
			// Key: "clientID", Value: OAuthClientName,
		}, {
			Key: "clientSecretFile", Value: clientSecretFilePath,
		}, {
			Key: "logoutRedirect", Value: "",
		}, {
			Key: "oauthEndpointCAFile", Value: oauthEndpointCAFilePath,
		},
	}
}

// TODO: this can take args as we update locations after we generate a router
func clusterInfo(rt *routev1.Route) yaml.MapSlice {
	host := rt.Spec.Host
	return yaml.MapSlice{
		{
			Key: "consoleBaseAddress", Value: consoleBaseAddr(host),
		}, {
			Key: "consoleBasePath", Value: "",
		}, // {
		// Key: "masterPublicURL", Value: nil,
		// },
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
// There is a better way to do this I am sure
func newConsoleConfigYaml(rt *routev1.Route) string {
	// https://godoc.org/gopkg.in/yaml.v2#MapSlice
	conf := yaml.MapSlice{
		{
			Key: "kind", Value: "ConsoleConfig",
		}, {
			Key: "apiVersion", Value: "console.openshift.io/v1beta1",
		}, {
			Key: "auth", Value: authServerYaml(),
		}, {
			Key: "clusterInfo", Value: clusterInfo(rt),
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

func newConsoleConfigMap(cr *v1alpha1.Console, rt *routev1.Route) *corev1.ConfigMap {
	meta := sharedMeta()
	// expects a non-standard name
	meta.Name = ConsoleConfigMapName
	config := newConsoleConfigYaml(rt)
	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: meta,
		Data: map[string]string{
			// haha, duplicate!  fix me
			consoleConfigYamlFile: config,
		},
	}
	addOwnerRef(configMap, ownerRefFrom(cr))
	return configMap
}

func CreateConsoleConfigMap(cr *v1alpha1.Console, rt *routev1.Route) (*corev1.ConfigMap, error) {
	configMap := newConsoleConfigMap(cr, rt)
	if err := sdk.Create(configMap); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console configmap : %v", err)
		return nil, err
	} else {
		logrus.Infof("created console configmap '%s'", configMap.ObjectMeta.Name)
		return configMap, nil
	}
}

func UpdateConsoleConfigMap(cr *v1alpha1.Console, rt *routev1.Route) (*corev1.ConfigMap, error) {
	configMap := newConsoleConfigMap(cr, rt)
	err := sdk.Update(configMap)
	return configMap, err
}

func ApplyConfigMap(cr *v1alpha1.Console, rt *routev1.Route) (*corev1.ConfigMap, error) {
	configMap := newConsoleConfigMap(cr, rt)
	err := sdk.Get(configMap)

	if err != nil {
		return CreateConsoleConfigMap(cr, rt)
	}
	return configMap, nil
}
