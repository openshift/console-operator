package managedclusteraction

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	// openshift
	"github.com/openshift/console-operator/bindata"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	// acm - TODO conflicts adding package to go.mod with several dependencies
	// managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
)

func DefaultCreateOAuthClientAction(cn string, sec string, redirects []string) (*unstructured.Unstructured, error) {
	mca := CreateOAuthClientStub(cn)
	var errors []error
	errors = append(errors, unstructured.SetNestedField(mca.Object, api.CreateOAuthClientManagedClusterActionName, "metadata", "name"))
	errors = append(errors, unstructured.SetNestedField(mca.Object, cn, "metadata", "namespace"))
	errors = append(errors, unstructured.SetNestedStringMap(mca.Object, util.LabelsForManagedClusterResources(cn), "metadata", "labels"))
	errors = append(errors, unstructured.SetNestedField(mca.Object, sec, "spec", "kube", "template", "secret"))
	errors = append(errors, unstructured.SetNestedField(mca.Object, api.ManagedClusterOAuthClientName, "spec", "kube", "template", "metadata", "name"))
	errors = append(errors, unstructured.SetNestedStringSlice(mca.Object, redirects, "spec", "kube", "template", "redirectURIs"))
	aggregateError := utilerrors.NewAggregate(errors)
	if aggregateError != nil {
		return nil, aggregateError
	}
	return mca, nil
}

func CreateOAuthClientStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(bindata.MustAsset("assets/managedclusteractions/console-create-oauth-client.yaml"))
}

func GetName(mca *unstructured.Unstructured) (string, error) {
	name, found, err := unstructured.NestedString(mca.Object, "metadata", "name")
	if err != nil || !found || name == "" {
		return "", err
	}
	return name, nil
}
