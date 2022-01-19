package managedclusterview

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	// openshift

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/assets"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	// acm - TODO conflicts adding package to go.mod with several dependencies
	// managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
)

func DefaultOAuthClientView(cn string) (*unstructured.Unstructured, error) {
	mcv := OAuthClientViewStub()
	var errors []error
	errors = append(errors, unstructured.SetNestedField(mcv.Object, api.OAuthClientManagedClusterViewName, "metadata", "name"))
	errors = append(errors, unstructured.SetNestedField(mcv.Object, cn, "metadata", "namespace"))
	errors = append(errors, unstructured.SetNestedStringMap(mcv.Object, util.LabelsForManagedClusterResources(cn), "metadata", "labels"))
	aggregateErrors := utilerrors.NewAggregate(errors)
	if aggregateErrors != nil {
		return nil, aggregateErrors
	}
	return mcv, nil
}

func OAuthClientViewStub() *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedclusterviews/console-oauth-client.yaml"))
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
	name, _, err := unstructured.NestedString(mcv.Object, "metadata", "name")
	return name, err
}

func GetNamespace(mcv *unstructured.Unstructured) (string, error) {
	namespace, _, err := unstructured.NestedString(mcv.Object, "metadata", "namespace")
	return namespace, err
}

func GetOAuthClientSecret(mcv *unstructured.Unstructured) (string, error) {
	secret, _, err := unstructured.NestedString(mcv.Object, "status", "result", "secret")
	return secret, err
}
