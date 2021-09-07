package configmap

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

// DefaultServiceCAConfigMap creates a config map that holds the API server CA bundle for a managed cluster.
// Console uses this bundle to proxy to managed clusters.
func DefaultAPIServerCAConfigMap(clusterName string, caBundle []byte, cr *operatorv1.Console) *corev1.ConfigMap {
	configMap := APIServerCAConfigMapStub(clusterName)

	if caBundle != nil {
		configMap.Data = map[string]string{
			"ca.crt": string(caBundle),
		}
	}

	util.AddOwnerRef(configMap, util.OwnerRefFrom(cr))
	return configMap
}

func APIServerCAConfigMapStub(clusterName string) *corev1.ConfigMap {
	meta := util.SharedMeta()
	meta.Name = APIServerCAConfigMapName(clusterName)
	meta.Labels = map[string]string{
		api.ManagedClusterAPIServerCAName: "",
		api.ManagedClusterLabel:           clusterName,
		"app":                             "console",
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: meta,
	}
	return configMap
}

func APIServerCAConfigMapName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, api.ManagedClusterAPIServerCAName)
}
