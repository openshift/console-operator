package managedclusteraction

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	// openshift
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/assets"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	// acm - TODO conflicts adding package to go.mod with several dependencies
	// managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
)

func DefaultCreateOAuthClientAction(cn string, sec string, redirects []string) *unstructured.Unstructured {
	mca := CreateOAuthClientStub(cn)
	unstructured.SetNestedField(mca.Object, api.CreateOAuthClientManagedClusterActionName, "metadata", "name")
	unstructured.SetNestedField(mca.Object, cn, "metadata", "namespace")
	unstructured.SetNestedStringMap(mca.Object, util.LabelsForManagedClusterResources(cn), "metadata", "labels")
	unstructured.SetNestedField(mca.Object, sec, "spec", "kube", "template", "secret")
	unstructured.SetNestedField(mca.Object, api.ManagedClusterOAuthClientName, "spec", "kube", "template", "metadata", "name")
	unstructured.SetNestedStringSlice(mca.Object, redirects, "spec", "kube", "template", "redirectURIs")
	return mca
}

func CreateOAuthClientStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedclusteractions/console-create-oauth-client.yaml"))
}

func GetName(mca *unstructured.Unstructured) (string, error) {
	name, found, err := unstructured.NestedString(mca.Object, "metadata", "name")
	if err != nil || !found || name == "" {
		return "", err
	}
	return name, nil
}
