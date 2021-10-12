package managedclusterview

import (
	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	// openshift
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/assets"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	// acm - TODO conflicts adding package to go.mod with several dependencies
	// managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
)

func DefaultViewIngressCert(cr *operatorv1.Console, cn string) *unstructured.Unstructured {
	managedClusterView := ViewIngressCertStub(cn)
	withInfo(managedClusterView, cn)
	return managedClusterView
}

func DefaultViewOAuthClient(cr *operatorv1.Console, cn string) *unstructured.Unstructured {
	managedClusterView := ViewOAuthClientStub(cn)
	withInfo(managedClusterView, cn)
	return managedClusterView
}

func withInfo(mcv *unstructured.Unstructured, cn string) {
	unstructured.SetNestedField(mcv.Object, cn, "metadata", "namespace")
}

func ViewIngressCertStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedclusterviews/console-managed-cluster-view-ingress-cert.yaml"))
}

func ViewOAuthClientStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedclusterviews/console-managed-cluster-view-oauth-client.yaml"))
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

func GetGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "view.open-cluster-management.io",
		Version:  "v1beta1",
		Resource: "managedclusterviews",
	}
}

func GetCertBundle(mcv *unstructured.Unstructured) (string, error) {
	cert, found, err := unstructured.NestedString(mcv.Object, "status", "result", "data", api.ManagedClusterIngressCertKey)
	if err != nil || !found || cert == "" {
		return "", err
	}
	return cert, nil
}
