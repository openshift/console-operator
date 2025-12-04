package util

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

// Return func which returns true if obj name is in names
func IncludeNamesFilter(names ...string) factory.EventFilterFunc {
	nameSet := sets.NewString(names...)
	return func(obj interface{}) bool {
		if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			obj = tombstone.Obj
		}
		metaObj, ok := obj.(metav1.Object)
		if !ok {
			klog.Errorf("Unexpected type %T", obj)
			return false
		}
		return nameSet.Has(metaObj.GetName())
	}
}

// Inverse of IncludeNamesFilter
func ExcludeNamesFilter(names ...string) factory.EventFilterFunc {
	return func(obj interface{}) bool {
		return !IncludeNamesFilter(names...)(obj)
	}
}

// Return a func which returns true if obj matches on every label in labels
// (i.e for each key in labels map, obj.metadata.labels[key] is equal to labels[key])
func LabelFilter(labels map[string]string) factory.EventFilterFunc {
	return func(obj interface{}) bool {
		if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			obj = tombstone.Obj
		}
		metaObj, ok := obj.(metav1.Object)
		if !ok {
			klog.Errorf("Unexpected type %T", obj)
			return false
		}
		objLabels := metaObj.GetLabels()
		for k, v := range labels {
			if objLabels[k] != v {
				return false
			}
		}
		return true
	}
}

type consoleOperatorController interface {
	HandleManaged(context.Context) error
	HandleUnmanaged(context.Context) error
	HandleRemoved(context.Context) error
}

func HandleManagementState(ctx context.Context, c consoleOperatorController, operatorClient v1helpers.OperatorClient) error {
	operatorSpec, _, _, err := operatorClient.GetOperatorState()
	if err != nil {
		return fmt.Errorf("failed to retrieve operator config: %w", err)
	}

	switch managementState := operatorSpec.ManagementState; managementState {
	case operatorv1.Managed:
		return c.HandleManaged(ctx)
	case operatorv1.Unmanaged:
		return c.HandleUnmanaged(ctx)
	case operatorv1.Removed:
		return c.HandleRemoved(ctx)
	default:
		return fmt.Errorf("console is in an unknown state: %v", managementState)
	}
}

// IsExternalControlPlaneWithIngressDisabled returns true if the cluster is in external control plane topology (hypershift)
// and the ingress cluster capability is disabled.
func IsExternalControlPlaneWithIngressDisabled(infrastructureConfig *configv1.Infrastructure, clusterVersionConfig *configv1.ClusterVersion) bool {
	isIngressCapabilityEnabled := false
	for _, capability := range clusterVersionConfig.Status.Capabilities.EnabledCapabilities {
		if capability == configv1.ClusterVersionCapabilityIngress {
			isIngressCapabilityEnabled = true
			break
		}
	}

	return infrastructureConfig.Status.ControlPlaneTopology == configv1.ExternalTopologyMode && !isIngressCapabilityEnabled
}

// IsExternalControlPlaneWithNLB returns true if the cluster is in external control plane topology (hypershift)
func IsExternalControlPlaneWithNLB(infrastructureConfig *configv1.Infrastructure, ingressConfig *configv1.Ingress) bool {
	return infrastructureConfig.Status.ControlPlaneTopology == configv1.ExternalTopologyMode &&
		infrastructureConfig.Status.PlatformStatus.Type == configv1.AWSPlatformType &&
		ingressConfig.Spec.LoadBalancer.Platform.Type == configv1.AWSPlatformType &&
		ingressConfig.Spec.LoadBalancer.Platform.AWS.Type == configv1.NLB
}
