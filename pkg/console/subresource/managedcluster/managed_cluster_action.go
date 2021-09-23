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

func DefaultManagedClusterActionOAuthCreate(cr *operatorv1.Console, cn string, sec string, redirects []string) *unstructured.Unstructured {
	managedClusterAction := ActionOAuthCreateStub(cn)
	withManagedClusterActionInfo(managedClusterAction, cn, sec, redirects)
	return managedClusterAction
}

func withManagedClusterActionInfo(mca *unstructured.Unstructured, cn string, sec string, redirects []string) {
	unstructured.SetNestedField(mca.Object, cn, "metadata", "namespace")
	unstructured.SetNestedField(mca.Object, sec, "spec", "kube", "template", "secret")
	unstructured.SetNestedStringSlice(mca.Object, redirects, "spec", "kube", "template", "redirectURIs")
}

func ActionOAuthCreateStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedcluster/console-managed-cluster-action-oauth-create.yaml"))
}
