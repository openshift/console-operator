package operator

import (
	"bytes"
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
// Degraded = the operator (not the operand) is failing
//    example: The console operator is unable to update the console config with
//      a new logoutRedirect URL. The operator is failing to do its job, however the
//      console (operand) may or may not be functional (see Available above).
// Failing = deprecated.
//
// Status Condition Reason & Message
// Reason:  OperatorSyncLoopError
// Message: "The operator sync loop was not completed successfully"
//
//

const (
	reasonAsExpected          = "AsExpected"
	reasonWorkloadFailing     = "WorkloadFailing"
	reasonUnmanaged           = "ManagementStateUnmanaged"
	reasonRemoved             = "ManagementStateRemoved"
	reasonInvalid             = "ManagementStateInvalid"
	reasonSyncLoopProgressing = "SyncLoopProgressing"
	reasonSyncError           = "SynchronizationError"
	reasonNoPodsAvailable     = "NoPodsAvailable"
	reasonSyncLoopError       = "SyncLoopError"
)

// TODO:
// a single builder helper would be sufficiently easy to read and
// reason about. Consider migrating to this structure as part of aggregating status
// AddStatus(ConditionFail)
//			.To(ConditionTrue)
//			.Reason("FooBar")
//			.Message("This truly broke FooBar"))

// Lets transition to using this, and get the repetition out of all of the above.
func (c *consoleOperator) SyncStatus(operatorConfig *operatorsv1.Console) (*operatorsv1.Console, error) {
	c.logConditions(operatorConfig.Status.Conditions)
	updatedConfig, err := c.operatorConfigClient.UpdateStatus(operatorConfig)
	if err != nil {
		errMsg := fmt.Errorf("status update error: %v \n", err)
		logrus.Error(errMsg)
		return nil, errMsg
	}
	return updatedConfig, nil
}

// Outputs the condition as a log message based on the detail of the condition in the form of:
//   Status.Condition.<Condition>: <Bool>
//   Status.Condition.<Condition>: <Bool> (<Reason>)
//   Status.Condition.<Condition>: <Bool> (<Reason>) <Message>
//   Status.Condition.<Condition>: <Bool> <Message>
func (c *consoleOperator) logConditions(conditions []operatorsv1.OperatorCondition) {
	logrus.Println("Operator.Status.Conditions")

	for _, condition := range conditions {
		buf := bytes.Buffer{}
		buf.WriteString(fmt.Sprintf("Status.Condition.%s: %s", condition.Type, condition.Status))
		hasMessage := condition.Message != ""
		hasReason := condition.Reason != ""
		if hasMessage && hasReason {
			buf.WriteString(" |")
			if hasReason {
				buf.WriteString(fmt.Sprintf(" (%s)", condition.Reason))
			}
			if hasMessage {
				buf.WriteString(fmt.Sprintf(" %s", condition.Message))
			}
		}
		logrus.Println(buf.String())
	}
}

// setStatusCondition
// A generic helper for setting a status condition.
// To use when another more specific status function is not sufficient.
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

func (c *consoleOperator) ConditionResourceSyncSuccess(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeFailing,
		Status:             operatorsv1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
	})

	return operatorConfig
}

func (c *consoleOperator) ConditionDeploymentAvailable(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeAvailable,
		Status:             operatorsv1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	})

	return operatorConfig
}

func (c *consoleOperator) ConditionDeploymentNotAvailable(operatorConfig *operatorsv1.Console, message string) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeAvailable,
		Status:             operatorsv1.ConditionFalse,
		Reason:             reasonNoPodsAvailable,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})

	return operatorConfig
}

// ConditionResourceSyncProgressing()
// When a sync loop aborts early:
// - we dont know if the operand is available, so it is best not to set it.
// - on install, an incomplete sync will mean the operator is unavailable.
// - however, on a later sync when a change is encountered, this is not true.
// - we do know that we encountered a component of the operand in an incorrect state
// - we do know we are progressing because we are trying to change something about the operand
func (c *consoleOperator) ConditionResourceSyncProgressing(operatorConfig *operatorsv1.Console, message string) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeProgressing,
		Status:             operatorsv1.ConditionTrue,
		Reason:             reasonSyncLoopProgressing,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})

	return operatorConfig
}

func (c *consoleOperator) ConditionResourceSyncNotProgressing(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeProgressing,
		Status:             operatorsv1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
	})

	return operatorConfig
}

// examples:
//   conditionFailing(operatorConfig, "SyncLoopError", "Sync loop failed to complete successfully")
func (c *consoleOperator) ConditionDegraded(operatorConfig *operatorsv1.Console, conditionReason string, conditionMessage string) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeDegraded,
		Status:             operatorsv1.ConditionTrue,
		Reason:             conditionReason,
		Message:            conditionMessage,
		LastTransitionTime: metav1.Now(),
	})

	return operatorConfig
}

func (c *consoleOperator) ConditionNotDegraded(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeDegraded,
		Status:             operatorsv1.ConditionFalse,
		Reason:             reasonAsExpected,
		LastTransitionTime: metav1.Now(),
	})

	return operatorConfig
}

// When a sync error happens,
// - we don't know if the operand is Available, best to leave Available alone
// - we do know we are Progressing because we are trying to change an operand resource
// - we do know that we are likely Degraded as some resources may be mismatched until a
//  successful loop completes.
func (c *consoleOperator) ConditionResourceSyncDegraded(operatorConfig *operatorsv1.Console, message string) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeProgressing,
		Status:             operatorsv1.ConditionTrue,
		Reason:             reasonSyncError,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeDegraded,
		Status:             operatorsv1.ConditionTrue,
		Message:            message,
		Reason:             reasonSyncError,
		LastTransitionTime: metav1.Now(),
	})

	return operatorConfig
}

func (c *consoleOperator) ConditionsManagementStateUnmanaged(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type: operatorsv1.OperatorStatusTypeAvailable,
		// See https://github.com/openshift/api/pull/206
		// While the ConditionUnknown state seems to be the correct fit, the current understanding is that
		// If the operator is fulfilling the user's desired state, set Available:true
		// Status:             operatorsv1.ConditionUnknown,
		Status:             operatorsv1.ConditionTrue,
		Reason:             reasonUnmanaged,
		Message:            "The operator is in an unmanaged state, therefore its availability is unknown.",
		LastTransitionTime: metav1.Now(),
	})
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeProgressing,
		Status:             operatorsv1.ConditionFalse,
		Reason:             reasonUnmanaged,
		Message:            "The operator is in an unmanaged state, therefore no changes are being applied.",
		LastTransitionTime: metav1.Now(),
	})
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeDegraded,
		Status:             operatorsv1.ConditionFalse,
		Reason:             reasonUnmanaged,
		Message:            "The operator is in an unmanaged state, therefore no operator actions are degraded.",
		LastTransitionTime: metav1.Now(),
	})

	return operatorConfig
}

func (c *consoleOperator) ConditionsManagementStateRemoved(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type: operatorsv1.OperatorStatusTypeAvailable,
		// See https://github.com/openshift/api/pull/206
		// At present, Available is the gate for upgrades.  The removal of an operand should NOT cause
		// an upgrade to fail. Therefore, ManagementState:Removed should delete the operand (console),
		// BUT should still report Available:True. Hopefully this will change.
		Status:             operatorsv1.ConditionTrue,
		Reason:             reasonRemoved,
		Message:            "The operator is in a removed state, the console has been removed.",
		LastTransitionTime: metav1.Now(),
	})
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeProgressing,
		Status:             operatorsv1.ConditionFalse,
		Reason:             reasonRemoved,
		Message:            "The operator is in a removed state, therefore no changes are being applied.",
		LastTransitionTime: metav1.Now(),
	})
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeDegraded,
		Status:             operatorsv1.ConditionFalse,
		Reason:             reasonRemoved,
		Message:            "The operator is in a removed state, therefore no operator actions are degraded.",
		LastTransitionTime: metav1.Now(),
	})

	return operatorConfig
}

// If ManagementState is invalid,
// We don't know if the operand is available bc we exit the sync loop w/o assessing resources
// We aren't progressing because we exit the sync loop
// We don't know if we are degraded because we exit the sync loop
func (c *consoleOperator) ConditionsManagementStateInvalid(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeAvailable,
		Status:             operatorsv1.ConditionUnknown,
		Reason:             reasonInvalid,
		Message:            "The operator management state is invalid",
		LastTransitionTime: metav1.Now(),
	})
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeProgressing,
		Status:             operatorsv1.ConditionFalse,
		Reason:             reasonInvalid,
		Message:            "The operator management state is invalid",
		LastTransitionTime: metav1.Now(),
	})
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeDegraded,
		Status:             operatorsv1.ConditionUnknown,
		Reason:             reasonInvalid,
		Message:            "The operator management state is invalid",
		LastTransitionTime: metav1.Now(),
	})

	return operatorConfig
}
