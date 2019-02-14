package operator

import (
	operatorsv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func setUnmanagedConditions(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeAvailable,
		Status:             operatorsv1.ConditionUnknown,
		Reason:             "Unmanaged",
		Message:            "the controller manager is in an unmanaged state, therefore its availability is unknown.",
		LastTransitionTime: metav1.Now(),
	})
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeProgressing,
		Status:             operatorsv1.ConditionFalse,
		Reason:             "Unmanaged",
		Message:            "the controller manager is in an unmanaged state, therefore no changes are being applied.",
		LastTransitionTime: metav1.Now(),
	})
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeFailing,
		Status:             operatorsv1.ConditionFalse,
		Reason:             "Unmanaged",
		Message:            "the controller manager is in an unmanaged state, therefore no operator actions are failing.",
		LastTransitionTime: metav1.Now(),
	})
	return operatorConfig
}
