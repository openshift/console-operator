package operator

import (
	"fmt"

	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorsv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

// Operator status is set on the Operator Config, which is:
//   group: console.operator.openshift.io
//   kind:  Console
//   name:  console
// And is replicated out onto the the clusteroperator by a separate sync loop:
//   group: config.openshift.io
//   kind:  ClusterOperator
//   name:  console
//
// Status Condition Types
// Status conditions should not be set to a "default" state when the operator starts up.
// Instead, set a status explicitly only when the state is known.  This is because
// the various status conditions may have been set on previous runs of the sync loop, or
// possibly even by a previous operator container.
//
// Available = the operand (not the operator) is available
//    example: the console resources are stabilized.
//    example: the console is not yet in a functional state, one or
//      more resources may be missing.
// Progressing = the operator is trying to transition operand state
//    example: during the initial deployment, the operator needs to create a number
//      of resources.  the console (operand) is likely in flux.
//    example: the assigned route .spec.host changes for the console.  the oauthclient
//      must be updated, as must the configmap, etc.  numerous resources are in flux,
//      the operator reports progressing until the resources stabilize
// Failing = the operator (not the operand) is failing
//    example: The console operator is unable to update the console config with
//      a new logoutRedirect URL. The operator is failing to do its job, however the
//      console (operand) may or may not be functional (see Available above).
//
//
// Status Condition Reason & Message
// Reason:  OperatorSyncLoopError
// Message: "The operator sync loop was not completed successfully"
//
//

// Lets transition to using this, and get the repetition out of all of the above.
func (c *consoleOperator) SyncStatus(operatorConfig *operatorsv1.Console) (*operatorsv1.Console, error) {
	updatedConfig, err := c.operatorConfigClient.UpdateStatus(operatorConfig)
	if err != nil {
		return nil, fmt.Errorf("status update error: %v \n", err)
	}
	return updatedConfig, nil
}

// setStatusCondition
// A generic helper for setting a status condition
// examples:
//  setStatusCondition(operatorConfig, Failing, True, "SyncLoopError", "Sync loop failed to complete successfully")
func (c *consoleOperator) SetStatusCondition(operatorConfig *operatorsv1.Console, conditionType string, conditionStatus operatorsv1.ConditionStatus, conditionReason string, conditionMessage string) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               conditionType,
		Status:             conditionStatus,
		Reason:             conditionReason,
		Message:            conditionMessage,
		LastTransitionTime: metav1.Now(),
	})
	return operatorConfig
}

// examples:
//   conditionFailing(operatorConfig, "SyncLoopError", "Sync loop failed to complete successfully")
func (c *consoleOperator) ConditionFailing(operatorConfig *operatorsv1.Console, conditionReason string, conditionMessage string) *operatorsv1.Console {
	fmt.Printf(
		"Status: %s %s %s %s %s",
		operatorsv1.OperatorStatusTypeFailing,
		operatorsv1.ConditionTrue,
		conditionReason,
		conditionMessage,
		metav1.Now())
	// conditionReason string, conditionMessage string
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeFailing,
		Status:             operatorsv1.ConditionTrue,
		Reason:             conditionReason,
		Message:            conditionMessage,
		LastTransitionTime: metav1.Now(),
	})
	return operatorConfig
}

func (c *consoleOperator) ConditionNotFailing(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeFailing,
		Status:             operatorsv1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
	})
	return operatorConfig
}

func (c *consoleOperator) ConditionProgressing() {
	// TODO: if progressing, we are saying something is moving and we need to report why.
}
func (c *consoleOperator) ConditionNotProgressing(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeProgressing,
		Status:             operatorsv1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
	})
	return operatorConfig
}

func (c *consoleOperator) ConditionAvailable(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeAvailable,
		Status:             operatorsv1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	})
	return operatorConfig
}

func (c *consoleOperator) ConditionNotAvailable(operatorConfig *operatorsv1.Console) {
	// TODO: if not available, something with the operand is wrong asd we need to report why
}

// A sync failure has happened.
// We dont necessarily know the condition of the operand (console),
// but we do know that the operator is failing to update the operand.
func (c *consoleOperator) ConditionResourceSyncFailure(operatorConfig *operatorsv1.Console, message string) *operatorsv1.Console {
	logrus.Printf("Status: Workload sync failure: %v \n", message)
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeFailing,
		Status:             operatorsv1.ConditionTrue,
		Message:            message,
		Reason:             "SyncError",
		LastTransitionTime: metav1.Now(),
	})
	return operatorConfig
}

func (c *consoleOperator) ConditionResourceSyncSuccess(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	logrus.Printf("Status: Workload sync success: \n")
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeFailing,
		Status:             operatorsv1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
	})
	return operatorConfig
}

func (c *consoleOperator) ConditionDeploymentAvailable(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	logrus.Printf("Status: Available: console pods available \n")
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeAvailable,
		Status:             operatorsv1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	})
	return operatorConfig
}

func (c *consoleOperator) ConditionDeploymentNotAvailable(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	reason := "NoPodsAvailable"
	message := "No pods available for Console deployment."
	logrus.Printf("Status: Not Available: %v \n", message)
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeAvailable,
		Status:             operatorsv1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
	return operatorConfig
}

func (c *consoleOperator) ConditionResourceSyncProgressing(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	reason := "SyncLoopProgressing"
	message := "Changes made during sync updates, additional sync expected."
	logrus.Printf("Status: Progressing: %v \n", message)
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeProgressing,
		Status:             operatorsv1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
	return operatorConfig
}

func (c *consoleOperator) ConditionResourceSyncNotProgressing(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	logrus.Printf("Status: Not Progressing: Sync success \n")
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeProgressing,
		Status:             operatorsv1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
	})
	return operatorConfig
}

// sets multiple conditions
func (c *consoleOperator) ConditionsManagementStateUnmanaged(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	logrus.Printf("Status: ManagementState: Removed. Conditions unknown.")
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeAvailable,
		Status:             operatorsv1.ConditionUnknown,
		Reason:             "ManagementStateUnmanaged",
		Message:            "the operator is in an unmanaged state, therefore its availability is unknown.",
		LastTransitionTime: metav1.Now(),
	})
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeProgressing,
		Status:             operatorsv1.ConditionFalse,
		Reason:             "ManagementStateUnmanaged",
		Message:            "the operator is in an unmanaged state, therefore no changes are being applied.",
		LastTransitionTime: metav1.Now(),
	})
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeFailing,
		Status:             operatorsv1.ConditionFalse,
		Reason:             "ManagementStateUnmanaged",
		Message:            "the operator is in an unmanaged state, therefore no operator actions are failing.",
		LastTransitionTime: metav1.Now(),
	})
	return operatorConfig
}

// sets multiple conditions
func (c *consoleOperator) ConditionsManagementStateRemoved(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	logrus.Printf("Status: ManagementState: Removed")
	reason := "ManagementStateRemoved"
	message := "The console has been removed."
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeAvailable,
		Status:             operatorsv1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeProgressing,
		Status:             operatorsv1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeFailing,
		Status:             operatorsv1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
	return operatorConfig
}
