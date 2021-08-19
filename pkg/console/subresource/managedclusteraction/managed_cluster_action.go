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

func DefaultManagedClusterAction(cr *operatorv1.Console, cn string) *unstructured.Unstructured {
	managedClusterAction := Stub(cn)
	withManagedClusterInfo(managedClusterAction, cn)
	return managedClusterAction
}

func withManagedClusterInfo(mca *unstructured.Unstructured, cn string) {
	unstructured.SetNestedField(mca.Object, cn, "metadata", "name")
	unstructured.SetNestedField(mca.Object, cn, "metadata", "namespace")
}

func Stub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("crds/console-managed-cluster-action-oauth-create.yaml"))
}
