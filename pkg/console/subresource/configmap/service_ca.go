package configmap

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

const (
	// ServiceCAConfigMapName is the name of the config map that contains the service CA bundle.
	ServiceCAConfigMapName = "service-ca"
	// See https://github.com/openshift/service-serving-cert-signer
	injectCABundleAnnotation = "service.alpha.openshift.io/inject-cabundle"
)

// DefaultServiceCAConfigMap creates a config map that holds the service CA bundle.
// ConsoleOperatorConfig uses this bundle to proxy to Prometheus. The value is injected into
// key "service-ca.crt" by the service serving cert operator.
func DefaultServiceCAConfigMap(cr *v1alpha1.ConsoleOperatorConfig) *corev1.ConfigMap {
	configMap := ServiceCAStub()
	util.AddOwnerRef(configMap, util.OwnerRefFrom(cr))
	return configMap
}

func ServiceCAStub() *corev1.ConfigMap {
	meta := util.SharedMeta()
	meta.Name = ServiceCAConfigMapName
	meta.Annotations = map[string]string{
		injectCABundleAnnotation: "true",
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: meta,
	}
	return configMap
}
