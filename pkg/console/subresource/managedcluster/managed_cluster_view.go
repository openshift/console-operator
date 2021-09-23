package managedcluster

import (
	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	// openshift
	"github.com/openshift/console-operator/pkg/console/assets"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	// acm - TODO conflicts adding package to go.mod with several dependencies
	// managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
)

func DefaultManagedClusterViewIngress(cr *operatorv1.Console, cn string) *unstructured.Unstructured {
	managedClusterView := ViewIngressStub(cn)
	withManagedClusterViewInfo(managedClusterView, cn)
	return managedClusterView
}

func DefaultManagedClusterViewOAuth(cr *operatorv1.Console, cn string) *unstructured.Unstructured {
	managedClusterView := ViewOAuthStub(cn)
	withManagedClusterViewInfo(managedClusterView, cn)
	return managedClusterView
}

func withManagedClusterViewInfo(mcv *unstructured.Unstructured, cn string) {
	unstructured.SetNestedField(mcv.Object, cn, "metadata", "namespace")
}

func ViewIngressStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedcluster/console-managed-cluster-view-ingress-cert.yaml"))
}

func ViewOAuthStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedcluster/console-managed-cluster-view-oauth.yaml"))
}

func GetResourceViewStatus(mcv *unstructured.Unstructured) (bool, error) {
	status, found, err := unstructured.NestedString(mcv.Object, "status", "conditions[0]", "status")
	if err != nil || found == false || status != "True" {
		return false, err
	}
	return true, nil
}

func GetResourceViewResult(mcv *unstructured.Unstructured) (map[string]string, error) {
	result, found, err := unstructured.NestedStringMap(mcv.Object, "status", "result")
	if err != nil || found == false || result == nil {
		return nil, err
	}
	return result, nil
}
