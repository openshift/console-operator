package olmconfigs

import (
	"testing"

	"github.com/go-test/deep"

	v1 "github.com/open-cluster-management/api/cluster/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func generateOLMConfig(t *testing.T) operatorsv1.OLMConfig {
	var olmConfig operatorsv1.OLMConfig
	olmConfig.Spec = operatorsv1.OLMConfigSpec{
		Features: &operatorsv1.Features{
			DisableCopiedCSVs: getPointer(false),
		},
	}
}

func generateClusters(t *testing.T) []v1.ManagedCluster {

	clusters := []v1.ManagedCluster{}

	cluster1 := v1.ManagedCluster{}
	cluster1.ObjectMeta = metav1.ObjectMeta{Name: "ProductAKSVer412"}
	cluster1.Status = v1.ManagedClusterStatus{
		ClusterClaims: []v1.ManagedClusterClaim{
			{Name: "product.open-cluster-management.io", Value: "AKS"},
			{Name: "version.openshift.io", Value: "4.12.0-ec.1"},
		},
	}

	clusters = append(clusters, cluster1)

	cluster2 := v1.ManagedCluster{}
	cluster2.ObjectMeta = metav1.ObjectMeta{Name: "ProductOpenshiftVer49"}
	cluster2.Status = v1.ManagedClusterStatus{
		ClusterClaims: []v1.ManagedClusterClaim{
			{Name: "product.open-cluster-management.io", Value: "OpenShift"},
			{Name: "version.openshift.io", Value: "4.9.0-ec.1"},
		},
	}
	clusters = append(clusters, cluster2)

	cluster3 := v1.ManagedCluster{}
	cluster3.ObjectMeta = metav1.ObjectMeta{Name: "ProductOpenshiftVer39"}
	cluster3.Status = v1.ManagedClusterStatus{
		ClusterClaims: []v1.ManagedClusterClaim{
			{Name: "product.open-cluster-management.io", Value: "OpenShift"},
			{Name: "version.openshift.io", Value: "3.9"},
		},
	}
	clusters = append(clusters, cluster3)

	cluster4 := v1.ManagedCluster{}
	cluster4.ObjectMeta = metav1.ObjectMeta{Name: "ProductEKSVer311"}
	cluster4.Status = v1.ManagedClusterStatus{
		ClusterClaims: []v1.ManagedClusterClaim{
			{Name: "product.open-cluster-management.io", Value: "EKS"},
			{Name: "version.openshift.io", Value: "3.11.rcbeata"},
		},
	}
	clusters = append(clusters, cluster4)

	return clusters
}

func TestCheckingDisabledCSVs(t *testing.T) {
	expectedResult := true
	olmConfig := generateOLMConfig(t)

	if diff := deep.Equal(olmConfig.CopiedCSVsAreEnabled, expectedResult); diff != nil {
		t.Error(diff)
	}
}

func TestProductAndVersionCheckForClusterList(t *testing.T) {
	expectedCluster := []string{"ProductOpenshiftVer49"}
	clusters := generateClusters(t)
	validClusters := []string{}
	if len(clusters) < 1 {
		t.Fatalf("No clusters are defined")
	}
	for _, managedCluster := range clusters {
		clusterName := managedCluster.GetName()
		validProduct, validVersion := isSupportedCluster(managedCluster.Status.ClusterClaims)
		if !validProduct {
			t.Logf("Skipping managed cluster %q, product is unsupported", clusterName)
			continue
		}
		if !validVersion {
			t.Logf("Skipping managed cluster %q, version is unsupported", clusterName)
			continue
		}
		validClusters = append(validClusters, clusterName)
	}

	if diff := deep.Equal(validClusters, expectedCluster); diff != nil {
		t.Error(diff)
	}
}
