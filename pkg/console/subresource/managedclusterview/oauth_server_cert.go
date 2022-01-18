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

func DefaultOAuthServerCertView(cr *operatorv1.Console, cn string) (*unstructured.Unstructured, error) {
	mcv := OAuthServerCertViewStub(cn)
	err := unstructured.SetNestedField(mcv.Object, api.OAuthServerCertManagedClusterViewName, "metadata", "name")
	err = unstructured.SetNestedField(mcv.Object, cn, "metadata", "namespace")
	err = unstructured.SetNestedStringMap(mcv.Object, util.LabelsForManagedClusterResources(cn), "metadata", "labels")
	if err != nil {
		return nil, err
	}
	return mcv, nil
}

func OAuthServerCertViewStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedclusterviews/console-oauth-server-cert.yaml"))
}

func GetCertBundle(mcv *unstructured.Unstructured) (string, error) {
	cert, found, err := unstructured.NestedString(mcv.Object, "status", "result", "data", "ca-bundle.crt")
	if err != nil || !found || cert == "" {
		return "", err
	}
	return cert, nil
}
