package managedcluster

import (
	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	// openshift
	"github.com/openshift/console-operator/pkg/console/assets"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	// acm - TODO conflicts adding package to go.mod with several dependencies
	// managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
)

func DefaultCreateOAuthClient(cr *operatorv1.Console, cn string, sec string, redirects []string) *unstructured.Unstructured {
	managedClusterAction := CreateOAuthClientStub(cn)
	withDefaultCreateOAuthClientInfo(managedClusterAction, cn, sec, redirects)
	return managedClusterAction
}

func withDefaultCreateOAuthClientInfo(mca *unstructured.Unstructured, cn string, sec string, redirects []string) {
	unstructured.SetNestedField(mca.Object, cn, "metadata", "namespace")
	unstructured.SetNestedField(mca.Object, sec, "spec", "kube", "template", "secret")
	unstructured.SetNestedStringSlice(mca.Object, redirects, "spec", "kube", "template", "redirectURIs")
}

func CreateOAuthClientStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedclusteractions/console-managed-cluster-action-create-oauth-client.yaml"))
}

func DefaultViewOAuthClient(cr *operatorv1.Console, cn string) *unstructured.Unstructured {
	managedClusterView := ViewOAuthClientStub(cn)
	withDefaultViewOAuthClientInfo(managedClusterView, cn)
	return managedClusterView
}

func withDefaultViewOAuthClientInfo(mcv *unstructured.Unstructured, cn string) {
	unstructured.SetNestedField(mcv.Object, cn, "metadata", "namespace")
}

func ViewOAuthClientStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedclusterviews/console-managed-cluster-view-oauth-client.yaml"))
}

func GetActionGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "view.open-cluster-management.io",
		Version:  "v1beta1",
		Resource: "managedclusterviews",
	}
}

func GetViewGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "action.open-cluster-management.io",
		Version:  "v1beta1",
		Resource: "managedclusteractions",
	}
}

func GetStatus(mcv *unstructured.Unstructured) (bool, error) {
	conditions, found, err := unstructured.NestedSlice(mcv.Object, "status", "conditions")
	if err != nil || !found || len(conditions) == 0 {
		return false, err
	}

	condition := conditions[0].(map[string]interface{})
	status := condition["status"].(string)
	if status != "True" {
		return false, err
	}

	return true, nil
}

func GetName(mcv *unstructured.Unstructured) (string, error) {
	name, found, err := unstructured.NestedString(mcv.Object, "metadata", "name")
	if err != nil || !found || name == "" {
		return "", err
	}
	return name, nil
}

func GetNamespace(mcv *unstructured.Unstructured) (string, error) {
	namespace, found, err := unstructured.NestedString(mcv.Object, "metadata", "namespace")
	if err != nil || !found || namespace == "" {
		return "", err
	}
	return namespace, nil
}
