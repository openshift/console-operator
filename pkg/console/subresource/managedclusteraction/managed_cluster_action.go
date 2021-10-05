package managedclusteraction

import (
	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	// openshift
	"github.com/openshift/console-operator/pkg/console/assets"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	// acm - TODO conflicts adding package to go.mod with several dependencies
	// managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
)

func DefaultCreateOAuthClient(cr *operatorv1.Console, cn string, sec string, redirects []string) *unstructured.Unstructured {
	managedClusterAction := CreateOAuthClientStub(cn)
	withInfo(managedClusterAction, cn, sec, redirects)
	return managedClusterAction
}

func withInfo(mca *unstructured.Unstructured, cn string, sec string, redirects []string) {
	unstructured.SetNestedField(mca.Object, cn, "metadata", "namespace")
	unstructured.SetNestedField(mca.Object, sec, "spec", "kube", "template", "secret")
	unstructured.SetNestedStringSlice(mca.Object, redirects, "spec", "kube", "template", "redirectURIs")
}

func CreateOAuthClientStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedclusteractions/console-managed-cluster-action-create-oauth-client.yaml"))
}
