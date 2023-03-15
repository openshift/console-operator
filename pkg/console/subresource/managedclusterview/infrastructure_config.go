package managedclusterview

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	// openshift
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/assets"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	// acm - TODO conflicts adding package to go.mod with several dependencies
	// managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
)

func DefaultInfrastructureConfigView(cr *operatorv1.Console, cn string) (*unstructured.Unstructured, error) {
	mcv := InfrastructureConfigViewStub(cn)
	var errors []error
	errors = append(errors, unstructured.SetNestedField(mcv.Object, api.InfrastructureConfigManagedClusterViewName, "metadata", "name"))
	errors = append(errors, unstructured.SetNestedField(mcv.Object, cn, "metadata", "namespace"))
	errors = append(errors, unstructured.SetNestedStringMap(mcv.Object, util.LabelsForManagedClusterResources(cn), "metadata", "labels"))
	aggregateError := utilerrors.NewAggregate(errors)
	if aggregateError != nil {
		return nil, aggregateError
	}
	return mcv, nil
}

func InfrastructureConfigViewStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedclusterviews/infrastructure-config.yaml"))
}

func GetInfrastructureConfigControlPlaneTopology(infraConfig *unstructured.Unstructured) (string, error) {
	controlPlaneTopology, found, err := unstructured.NestedString(infraConfig.Object, "status", "result", "status", "controlPlaneTopology")
	if err != nil || !found {
		return "", err
	}

	return controlPlaneTopology, nil
}
