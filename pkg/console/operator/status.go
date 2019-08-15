package operator

import (
	"bytes"
	"fmt"
	"strings"

	"k8s.io/klog"

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
	reasonSyncLoopProgressing = "SyncLoopProgressing"
	reasonSyncError           = "SynchronizationError"
	reasonNoPodsAvailable     = "NoPodsAvailable"
	reasonSyncLoopError       = "SyncLoopError"
	reasonCustomLogoInvalid   = "CustomLogoInvalid"
)

// handleDegraded can be used to set a number of Degraded statuses
// example:
//   c.handleDegraded(operatorConfig, "RouteStatus", err)
//   c.handleDegraded(operatorConfig, "CustomLogoInvalid", err)
// creates condition:
//   RouteStatusDegraded
//   CustomLogoInvalidDegraded
// and uses the error as its message
func (c *consoleOperator) HandleDegraded(operatorConfig *operatorsv1.Console, prefix string, err error) {
	conditionType := prefix + operatorsv1.OperatorStatusTypeDegraded
	reason := prefix + "Error"
	handleCondition(operatorConfig, conditionType, reason, err)
}

func (c *consoleOperator) HandleProgressing(operatorConfig *operatorsv1.Console, prefix string, err error) {
	conditionType := prefix + operatorsv1.OperatorStatusTypeProgressing
	handleCondition(operatorConfig, conditionType, prefix, err)
}

func (c *consoleOperator) HandleAvailable(operatorConfig *operatorsv1.Console, prefix string, err error) {
	conditionType := prefix + operatorsv1.OperatorStatusTypeAvailable
	handleCondition(operatorConfig, conditionType, prefix, err)
}

// internal func for handling conditions
func handleCondition(operatorConfig *operatorsv1.Console, conditionType string, reason string, err error) {
	if err != nil {
		v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions,
			operatorsv1.OperatorCondition{
				Type:    conditionType,
				Status:  setConditionValue(conditionType, err),
				Reason:  reason,
				Message: err.Error(),
			})
		return
	}
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions,
		operatorsv1.OperatorCondition{
			Type:   conditionType,
			Status: setConditionValue(conditionType, err),
		})
}

// Available is an inversion of the other conditions
func setConditionValue(conditionType string, err error) operatorsv1.ConditionStatus {
	if strings.HasSuffix(conditionType, operatorsv1.OperatorStatusTypeAvailable) {
		if err != nil {
			return operatorsv1.ConditionFalse
		}
		return operatorsv1.ConditionTrue
	}
	if err != nil {
		return operatorsv1.ConditionTrue
	}
	return operatorsv1.ConditionFalse
}

func IsDegraded(operatorConfig *operatorsv1.Authentication) bool {
	for _, condition := range operatorConfig.Status.Conditions {
		if strings.HasSuffix(condition.Type, operatorsv1.OperatorStatusTypeDegraded) &&
			condition.Status == operatorsv1.ConditionTrue {
			return true
		}
	}
	return false
}

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
		errMsg := fmt.Errorf("status update error: %v", err)
		klog.Error(errMsg)
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
	klog.V(4).Infoln("Operator.Status.Conditions")

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
		klog.V(4).Infoln(buf.String())
	}
}

// TODO: eliminate the defaulting mechanism
// This no longer has any meaning.
func (c *consoleOperator) ConditionsDefault(operatorConfig *operatorsv1.Console) *operatorsv1.Console {
	c.HandleAvailable(operatorConfig, "Default", nil)
	c.HandleProgressing(operatorConfig, "Default", nil)
	c.HandleDegraded(operatorConfig, "Default", nil)
	return operatorConfig
}
