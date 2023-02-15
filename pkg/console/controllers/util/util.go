package util

import (
	"context"

	//k8s
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	//github
	"github.com/blang/semver"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/library-go/pkg/controller/factory"

	// open-cluster-management
	clusterclientv1 "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

// Return func which returns true if obj name is in names
func IncludeNamesFilter(names ...string) factory.EventFilterFunc {
	nameSet := sets.NewString(names...)
	return func(obj interface{}) bool {
		metaObj := obj.(metav1.Object)
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
		metaObj := obj.(metav1.Object)
		objLabels := metaObj.GetLabels()
		for k, v := range labels {
			if objLabels[k] != v {
				return false
			}
		}
		return true
	}
}

func ClusterClaimsToMap(clusterClaims []clusterv1.ManagedClusterClaim) map[string]string {
	claimMap := map[string]string{}
	for _, claim := range clusterClaims {
		claimMap[claim.Name] = claim.Value
	}
	return claimMap
}

func IsValidManagedCluster(managedCluster clusterv1.ManagedCluster) bool {
	availableCondition := meta.FindStatusCondition(managedCluster.Status.Conditions, clusterv1.ManagedClusterConditionAvailable)
	if availableCondition == nil {
		klog.V(4).Infof("[%s] Unable to determine cluster availability.", managedCluster.Name)
		return false
	}

	if availableCondition.Status != metav1.ConditionTrue {
		klog.V(4).Infof("[%s] Cluster is not available: %s.", managedCluster.Name, availableCondition.Message)
		return false
	}

	clusterClaimMap := ClusterClaimsToMap(managedCluster.Status.ClusterClaims)
	product := clusterClaimMap[api.ManagedClusterProductClaim]
	if product == "" {
		klog.V(4).Infof("[%s] Unable to determine cluster product.", managedCluster.Name)
		return false
	}

	_, isProductSupported := api.SupportedClusterProducts[product]
	if !isProductSupported {
		klog.V(4).Infof("[%s] Product not supported for multicluster console: %v", managedCluster.Name, product)
		return false
	}

	version := clusterClaimMap[api.ManagedClusterVersionClaim]
	if version == "" {
		klog.V(4).Infof("[%s] Unable to determine cluster version.", managedCluster.Name)
		return false
	}

	clusterVersion, err := semver.Parse(version)
	if err != nil {
		klog.V(4).Infof("[%s] Unable to parse cluster version: %s", managedCluster.Name, version)
		return false
	}

	isSupportedVersion := clusterVersion.Compare(semver.MustParse("4.0.0")) >= 0
	if !isSupportedVersion {
		klog.V(4).Infof("[%s] Version not supported for multicluster console: %v", managedCluster.Name, version)
		return false
	}

	// Ensure client configs exists
	clientConfigs := managedCluster.Spec.ManagedClusterClientConfigs
	if len(clientConfigs) == 0 {
		klog.V(4).Infof("[%s] Missing client configs", managedCluster.Name)
		return false
	}

	// Ensure client config CA bundle exists
	if len(clientConfigs[0].CABundle) == 0 {
		klog.V(4).Infof("[%s] Missing client config CA bundle", managedCluster.Name)
		return false
	}

	// Ensure client config URL exists
	if len(clientConfigs[0].URL) == 0 {
		klog.V(4).Infof("[%s] Missing client config URL", managedCluster.Name)
		return false
	}
	return true
}

func GetValidManagedClusters(ctx context.Context, client clusterclientv1.ManagedClusterInterface) ([]clusterv1.ManagedCluster, string, error) {
	managedClusterList, err := client.List(ctx, metav1.ListOptions{LabelSelector: "local-cluster!=true"})
	validatedManagedClusters := []clusterv1.ManagedCluster{}

	// Not degraded, API is not found which means ACM isn't installed
	if apierrors.IsNotFound(err) {
		return validatedManagedClusters, "", nil
	}

	if err != nil {
		return validatedManagedClusters, "ManagedClusterListError", err
	}

	for _, managedCluster := range managedClusterList.Items {
		if IsValidManagedCluster(managedCluster) {
			validatedManagedClusters = append(validatedManagedClusters, managedCluster)
		}
	}
	return validatedManagedClusters, "", nil
}
