package configmap

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/assets"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

func DefaultManagedClusterOAuthServerCertConfigMap(clusterName string, caBundle string, cr *operatorv1.Console) *corev1.ConfigMap {
	configMap := ManagedClusterOAuthServerCertConfigMapStub(clusterName)

	if caBundle != "" {
		configMap.Data = map[string]string{
			api.ManagedClusterOAuthServerCertKey: caBundle,
		}
	}

	util.AddOwnerRef(configMap, util.OwnerRefFrom(cr))
	return configMap
}

func ManagedClusterOAuthServerCertConfigMapStub(clusterName string) *corev1.ConfigMap {
	configMap := resourceread.ReadConfigMapV1OrDie(assets.MustAsset("configmaps/console-managed-cluster-ingress-cert-configmap.yaml"))
	configMap.Name = ManagedClusterOAuthServerCertConfigMapName(clusterName)
	configMap.Labels = util.LabelsForManagedClusterResources(clusterName)
	configMap.Labels[api.ManagedClusterOAuthServerCertName] = ""
	return configMap
}

func ManagedClusterOAuthServerCertConfigMapName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, api.ManagedClusterOAuthServerCertName)
}

func ManagedClusterOAuthServerCAFileMountPath(clusterName string) string {
	return fmt.Sprintf("%s/%s/%s", api.ManagedClusterOAuthServerCertMountDir, ManagedClusterOAuthServerCertConfigMapName(clusterName), api.ManagedClusterAPIServerCertKey)
}
