package managedclusterview

import (
	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	// openshift
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/assets"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	// acm - TODO conflicts adding package to go.mod with several dependencies
	// managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
)

func DefaultViewIngressCert(cr *operatorv1.Console, cn string) *unstructured.Unstructured {
	managedClusterView := ViewIngressCertStub(cn)
	withDefaultViewIngressCertInfo(managedClusterView, cn)
	return managedClusterView
}

func withDefaultViewIngressCertInfo(mcv *unstructured.Unstructured, cn string) {
	unstructured.SetNestedField(mcv.Object, cn, "metadata", "namespace")
}

func ViewIngressCertStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedclusterviews/console-managed-cluster-view-ingress-cert.yaml"))
}

func GetCertBundle(mcv *unstructured.Unstructured) (string, error) {
	cert, found, err := unstructured.NestedString(mcv.Object, "status", "result", "data", api.ManagedClusterIngressCertKey)
	if err != nil || !found || cert == "" {
		return "", err
	}
	return cert, nil
}
