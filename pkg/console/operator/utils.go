package operator

import (
	// kube
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	// openshift
	operatorv1 "github.com/openshift/api/operator/v1"

	// operator
	"github.com/openshift/console-operator/pkg/api"
	configmapsub "github.com/openshift/console-operator/pkg/console/subresource/configmap"
)

func getCustomLogoConfigMapRefs(operatorConfig *operatorv1.Console) []configmapsub.CustomLogoRef {
	var normalizedLogos = []configmapsub.CustomLogoRef{}
	if len(operatorConfig.Spec.Customization.Logos) > 0 {
		for _, logo := range operatorConfig.Spec.Customization.Logos {
			for _, theme := range logo.Themes {
				if theme.Source.From == "ConfigMap" {
					normalizedLogos = append(
						normalizedLogos,
						configmapsub.CustomLogoRef{
							File: *theme.Source.ConfigMap,
							Mode: theme.Mode,
							Type: logo.Type,
						},
					)
				}
			}
		}
		return normalizedLogos
	}

	if operatorConfig.Spec.Customization.CustomLogoFile.Key != "" || operatorConfig.Spec.Customization.CustomLogoFile.Name != "" {
		normalizedLogos = append(
			normalizedLogos,
			configmapsub.CustomLogoRef{
				File: operatorv1.ConfigMapFileReference(operatorConfig.Spec.Customization.CustomLogoFile),
				Type: operatorv1.LogoTypeMasthead,
			},
		)
	}

	return normalizedLogos
}

func getNodeComputeEnvironments(nodes []*corev1.Node) ([]string, []string) {
	nodeArchitecturesSet := sets.NewString()
	nodeOperatingSystemSet := sets.NewString()
	for _, node := range nodes {
		nodeArch := node.Labels[api.NodeArchitectureLabel]
		if nodeArch == "" {
			klog.Warningf("Missing architecture label %q on node %q.", api.NodeArchitectureLabel, node.GetName())
		} else {
			nodeArchitecturesSet.Insert(nodeArch)
		}

		nodeOperatingSystem := node.Labels[api.NodeOperatingSystemLabel]
		if nodeOperatingSystem == "" {
			klog.Warningf("Missing operating system label %q on node %q", api.NodeOperatingSystemLabel, node.GetName())
		} else {
			nodeOperatingSystemSet.Insert(nodeOperatingSystem)
		}
	}
	return nodeArchitecturesSet.List(), nodeOperatingSystemSet.List()
}
