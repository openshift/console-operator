package configmap

import (
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

func DefaultManagedClustersConfigMap(operatorConfig *operatorv1.Console, managedClusters []consoleserver.ManagedClusterConfig) (*corev1.ConfigMap, error) {
	yml, err := yaml.Marshal(managedClusters)
	if err != nil {
		klog.V(4).Infof("Error marshalling managed clusters YAML: %v", err)
		return nil, err
	}

	configMap := ManagedClustersConfigMapStub()
	configMap.Data = map[string]string{
		api.ManagedClusterConfigKey: string(yml),
	}
	util.AddOwnerRef(configMap, util.OwnerRefFrom(operatorConfig))

	return configMap, nil
}

func ManagedClustersConfigMapStub() *corev1.ConfigMap {
	meta := util.SharedMeta()
	meta.Name = api.ManagedClusterConfigMapName
	meta.Labels = map[string]string{
		"app":                   "console",
		api.ManagedClusterLabel: "",
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: meta,
	}
	return configMap
}
