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

func DefaultOLMConfigView(cr *operatorv1.Console, cn string) (*unstructured.Unstructured, error) {
	mcv := OLMConfigViewStub(cn)
	var errors []error
	errors = append(errors, unstructured.SetNestedField(mcv.Object, api.OLMConfigManagedClusterViewName, "metadata", "name"))
	errors = append(errors, unstructured.SetNestedField(mcv.Object, cn, "metadata", "namespace"))
	errors = append(errors, unstructured.SetNestedStringMap(mcv.Object, util.LabelsForManagedClusterResources(cn), "metadata", "labels"))
	aggregateError := utilerrors.NewAggregate(errors)
	if aggregateError != nil {
		return nil, aggregateError
	}
	return mcv, nil
}

func OLMConfigViewStub(cn string) *unstructured.Unstructured {
	return util.ReadUnstructuredOrDie(assets.MustAsset("managedclusterviews/olm-config.yaml"))
}

func GetOLMConfigCopiedCSVDisabled(olmConfig *unstructured.Unstructured) (bool, error) {
	copiedCSVsDisabled, found, err := unstructured.NestedBool(olmConfig.Object, "status", "result", "spec", "features", "disableCopiedCSVs")
	if err != nil || !found {
		return false, err
	}
	return copiedCSVsDisabled, nil
}
