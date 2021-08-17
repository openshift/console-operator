package configmap

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

var ClusterCAConfigMapLabel = "managed-cluster-api-server-ca-certs"

// DefaultServiceCAConfigMap creates a config map that holds the API server CA bundle for a managed cluster.
// Console uses this bundle to proxy to managed clusters.
func DefaultClusterCAConfigMap(clusterName string, caBundle []byte, cr *operatorv1.Console) *corev1.ConfigMap {
	configMap := ClusterCAStub(clusterName, caBundle)
	util.AddOwnerRef(configMap, util.OwnerRefFrom(cr))
	return configMap
}

func ClusterCAStub(clusterName string, caBundle []byte) *corev1.ConfigMap {
	meta := util.SharedMeta()
	meta.Name = ClusterCAConfigMapName(clusterName)
	meta.Labels = map[string]string{
		ClusterCAConfigMapLabel: "",
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: meta,
	}
	if caBundle != nil {
		configMap.Data = map[string]string{
			"ca.crt": string(caBundle),
		}
	}
	return configMap
}

func ClusterCAConfigMapName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, api.ClusterCAConfigMapNameSuffix)
}
