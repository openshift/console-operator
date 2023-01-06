package managedclusteraction

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	// openshift

	"github.com/openshift/console-operator/pkg/console/assets"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	// acm - TODO conflicts adding package to go.mod with several dependencies
	// managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
)

func DefaultCreateHubClusterConfigMapAction(clusterName string, configmap string) (*unstructured.Unstructured, error) {
	mca := CreateHubClusterConfigMapStub(clusterName)
	var errors []error
	errors = append(errors, unstructured.SetNestedField(mca.Object, clusterName, "metadata", "namespace"))
	errors = append(errors, unstructured.SetNestedStringMap(mca.Object, util.LabelsForManagedClusterResources(clusterName), "metadata", "labels"))
	errors = append(errors, unstructured.SetNestedField(mca.Object, configmap, "spec", "kube", "template", "configmap"))
	aggregateError := utilerrors.NewAggregate(errors)
	if aggregateError != nil {
		return nil, aggregateError
	}
	return mca, nil
}

func CreateHubClusterConfigMapStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedclusteractions/console-create-hub-cluster-configmap.yaml"))
}
