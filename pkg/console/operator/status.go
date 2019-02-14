package operator

import (
	"fmt"

	operatorsv1 "github.com/openshift/api/operator/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition
//   Type: Available
//   Status: True
//   -
func (c *consoleOperator) operatorStatusAvailable(operatorConfig *operatorsv1.Console) (*operatorsv1.Console, error) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeAvailable,
		Status:             operatorsv1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	})
	if _, err := c.operatorConfigClient.UpdateStatus(operatorConfig); err != nil {
		return nil, fmt.Errorf("status update error: %v \n", err)
	}
	return operatorConfig, nil
}

func (c *consoleOperator) operatorStatusProgressing(operatorConfig *operatorsv1.Console) (*operatorsv1.Console, error) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:               operatorv1.OperatorStatusTypeProgressing,
		Status:             operatorv1.ConditionTrue,
		Reason:             "DesiredStateNotYetAchieved",
		LastTransitionTime: metav1.Now(),
	})
	if _, err := c.operatorConfigClient.UpdateStatus(operatorConfig); err != nil {
		return nil, fmt.Errorf("status update error: %v \n", err)
	}
	return operatorConfig, nil
}

func (c *consoleOperator) operatorStatusNotProgressing(operatorConfig *operatorsv1.Console) (*operatorsv1.Console, error) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:   operatorv1.OperatorStatusTypeProgressing,
		Status: operatorv1.ConditionFalse,
	})
	if _, err := c.operatorConfigClient.UpdateStatus(operatorConfig); err != nil {
		return nil, fmt.Errorf("status update error: %v \n", err)
	}
	return operatorConfig, nil
}

func (c *consoleOperator) operatorStatusFailing(operatorConfig *operatorsv1.Console) (*operatorsv1.Console, error) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:               workloadFailingCondition,
		Status:             operatorv1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	})
	if _, err := c.operatorConfigClient.UpdateStatus(operatorConfig); err != nil {
		return nil, fmt.Errorf("status update error: %v \n", err)
	}
	return operatorConfig, nil
}

func (c *consoleOperator) operatorStatusNotFailing(operatorConfig *operatorsv1.Console) (*operatorsv1.Console, error) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:               workloadFailingCondition,
		Status:             operatorv1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
	})
	if _, err := c.operatorConfigClient.UpdateStatus(operatorConfig); err != nil {
		return nil, fmt.Errorf("status update error: %v \n", err)
	}
	return operatorConfig, nil
}

// Condition
//   Type: Failing
//   Status: True
//   OperatorSyncLoopError
func (c *consoleOperator) operatorStatusFailingSyncLoopError(operatorConfig *operatorsv1.Console, err error) (*operatorsv1.Console, error) {
	fmt.Println("%s %s %s %s %s", operatorsv1.OperatorStatusTypeFailing, operatorsv1.ConditionTrue, "OperatorSyncLoopError", err.Error(), metav1.Now())
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorsv1.OperatorCondition{
		Type:               operatorsv1.OperatorStatusTypeFailing,
		Status:             operatorsv1.ConditionTrue,
		Reason:             "OperatorSyncLoopError",
		Message:            err.Error(),
		LastTransitionTime: metav1.Now(),
	})
	if _, err := c.operatorConfigClient.UpdateStatus(operatorConfig); err != nil {
		return nil, fmt.Errorf("status update error: %v \n", err)
	}
	return operatorConfig, nil
}

// Conditions
//  Type: Available
//  Type: Progressing
//  Type: Failing
//  Status: Unknown
//  - in an unmanaged state, we don't know the status of anything, the operator is effectively off
func (c *consoleOperator) operatorStatusUnknownUnmanaged(operatorConfig *operatorsv1.Console) (*operatorsv1.Console, error) {
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
	if _, err := c.operatorConfigClient.UpdateStatus(operatorConfig); err != nil {
		return nil, fmt.Errorf("status update error: %v \n", err)
	}
	return operatorConfig, nil
}

func (c *consoleOperator) operatorStatusResourceSyncFailure(operatorConfig *operatorv1.Console, message string) (*operatorsv1.Console, error) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:               workloadFailingCondition,
		Status:             operatorv1.ConditionTrue,
		Message:            message,
		Reason:             "SyncError",
		LastTransitionTime: metav1.Now(),
	})
	if _, err := c.operatorConfigClient.UpdateStatus(operatorConfig); err != nil {
		return nil, fmt.Errorf("status update error: %v \n", err)
	}
	return operatorConfig, nil
}

func (c *consoleOperator) operatorStatusDeploymentAvailable(operatorConfig *operatorv1.Console) (*operatorsv1.Console, error) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:               operatorv1.OperatorStatusTypeAvailable,
		Status:             operatorv1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	})
	if _, err := c.operatorConfigClient.UpdateStatus(operatorConfig); err != nil {
		return nil, fmt.Errorf("status update error: %v \n", err)
	}
	return operatorConfig, nil
}

func (c *consoleOperator) operatorStatusDeploymentUnavailable(operatorConfig *operatorv1.Console) (*operatorsv1.Console, error) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:               operatorv1.OperatorStatusTypeAvailable,
		Status:             operatorv1.ConditionFalse,
		Reason:             "NoPodsAvailable",
		Message:            "NoDeploymentPodsAvailableOnAnyNode.",
		LastTransitionTime: metav1.Now(),
	})
	if _, err := c.operatorConfigClient.UpdateStatus(operatorConfig); err != nil {
		return nil, fmt.Errorf("status update error: %v \n", err)
	}
	return operatorConfig, nil
}
