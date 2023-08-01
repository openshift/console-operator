package managedclusterview

import (
	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	// openshift
	"github.com/openshift/console-operator/bindata"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	// acm - TODO conflicts adding package to go.mod with several dependencies
	// managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
)

func DefaultOAuthServerCertView(cr *operatorv1.Console, cn string) (*unstructured.Unstructured, error) {
	mcv := OAuthServerCertViewStub(cn)
	var errors []error
	errors = append(errors, unstructured.SetNestedField(mcv.Object, api.OAuthServerCertManagedClusterViewName, "metadata", "name"))
	errors = append(errors, unstructured.SetNestedField(mcv.Object, cn, "metadata", "namespace"))
	errors = append(errors, unstructured.SetNestedStringMap(mcv.Object, util.LabelsForManagedClusterResources(cn), "metadata", "labels"))
	aggregateError := utilerrors.NewAggregate(errors)
	if aggregateError != nil {
		return nil, aggregateError
	}
	return mcv, nil
}

func OAuthServerCertViewStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(bindata.MustAsset("assets/managedclusterviews/console-oauth-server-cert.yaml"))
}

func GetCertBundle(mcv *unstructured.Unstructured) (string, error) {
	cert, found, err := unstructured.NestedString(mcv.Object, "status", "result", "data", "ca-bundle.crt")
	if err != nil || !found || cert == "" {
		return "", err
	}
	return cert, nil
}
