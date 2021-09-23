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

func DefaultManagedClusterIngressCertConfigMap(clusterName string, caBundle string, cr *operatorv1.Console) *corev1.ConfigMap {
	configMap := ManagedClusterIngressCertConfigMapStub(clusterName)

	if caBundle != "" {
		configMap.Data = map[string]string{
			"ca.crt": caBundle,
		}
	}

	util.AddOwnerRef(configMap, util.OwnerRefFrom(cr))
	return configMap
}

func ManagedClusterIngressCertConfigMapStub(clusterName string) *corev1.ConfigMap {
	configMap := resourceread.ReadConfigMapV1OrDie(assets.MustAsset("configmaps/console-managed-cluster-ingress-cert-configmap.yaml"))
	configMap.Name = ManagedClusterIngressCertConfigMapName(clusterName)
	return configMap
}

func ManagedClusterIngressCertConfigMapName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, api.ManagedClusterIngressCertName)
}
