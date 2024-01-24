package status

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/klog/v2"

	operatorsv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/console/errors"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
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

// handleDegraded(), handleProgressing(), handleAvailable() each take a typePrefix string representing "category"
// and a reason string, representing the actual problem.
// the provided err will be used as the detailed message body if it is not nil
// note that available status is desired to be true, where degraded & progressing are desired to be false
// example:
//
//	c.handleDegraded(operatorConfig, "RouteStatus", "FailedHost", error.New("route is not available at canonical host..."))
//
// generates:
//   - Type RouteStatusDegraded
//     Status: true
//     Reason: Failedhost
//     Message: error string value is used as message
//
// all degraded suffix conditions will be aggregated into a final "Degraded" status that will be set on the console ClusterOperator
func HandleDegraded(typePrefix string, reason string, err error) ConditionUpdate {
	conditionType := typePrefix + operatorsv1.OperatorStatusTypeDegraded
	condition := handleCondition(conditionType, reason, err)
	return ConditionUpdate{
		ConditionType:  conditionType,
		StatusUpdateFn: v1helpers.UpdateConditionFn(condition),
	}
}

func HandleProgressing(typePrefix string, reason string, err error) ConditionUpdate {
	conditionType := typePrefix + operatorsv1.OperatorStatusTypeProgressing
	condition := handleCondition(conditionType, reason, err)
	return ConditionUpdate{
		ConditionType:  conditionType,
		StatusUpdateFn: v1helpers.UpdateConditionFn(condition),
	}
}

func HandleAvailable(typePrefix string, reason string, err error) ConditionUpdate {
	conditionType := typePrefix + operatorsv1.OperatorStatusTypeAvailable
	condition := handleCondition(conditionType, reason, err)
	return ConditionUpdate{
		ConditionType:  conditionType,
		StatusUpdateFn: v1helpers.UpdateConditionFn(condition),
	}
}

func HandleUpgradable(typePrefix string, reason string, err error) ConditionUpdate {
	conditionType := typePrefix + operatorsv1.OperatorStatusTypeUpgradeable
	condition := handleCondition(conditionType, reason, err)
	return ConditionUpdate{
		ConditionType:  conditionType,
		StatusUpdateFn: v1helpers.UpdateConditionFn(condition),
	}
}

func (c *StatusHandler) ResetConditions(conditions []operatorsv1.OperatorCondition) []ConditionUpdate {
	updateStatusFuncs := []ConditionUpdate{}
	for _, condition := range conditions {
		klog.V(2).Info("\nresetting condition: ", condition.Type)
		if strings.HasSuffix(condition.Type, operatorsv1.OperatorStatusTypeDegraded) {
			conditionPrefix := strings.TrimSuffix(condition.Type, operatorsv1.OperatorStatusTypeDegraded)
			updateStatusFuncs = append(updateStatusFuncs, HandleDegraded(conditionPrefix, "", nil))
			continue
		}
		if strings.HasSuffix(condition.Type, operatorsv1.OperatorStatusTypeAvailable) {
			conditionPrefix := strings.TrimSuffix(condition.Type, operatorsv1.OperatorStatusTypeAvailable)
			updateStatusFuncs = append(updateStatusFuncs, HandleAvailable(conditionPrefix, "", nil))
			continue
		}
		if strings.HasSuffix(condition.Type, operatorsv1.OperatorStatusTypeProgressing) {
			conditionPrefix := strings.TrimSuffix(condition.Type, operatorsv1.OperatorStatusTypeProgressing)
			updateStatusFuncs = append(updateStatusFuncs, HandleProgressing(conditionPrefix, "", nil))
			continue
		}
		if strings.HasSuffix(condition.Type, operatorsv1.OperatorStatusTypeUpgradeable) {
			conditionPrefix := strings.TrimSuffix(condition.Type, operatorsv1.OperatorStatusTypeUpgradeable)
			updateStatusFuncs = append(updateStatusFuncs, HandleUpgradable(conditionPrefix, "", nil))
			continue
		}
		klog.V(2).Info("unable to reset condition: ", condition.Type)
	}

	return updateStatusFuncs
}

// HandleProgressingOrDegraded exists until we remove type SyncError
// If isSyncError
// - Type suffix will be set to Progressing
// if it is any other kind of error
// - Type suffix will be set to Degraded
// TODO: when we eliminate the special case SyncError, this helper can go away.
// When we do that, however, we must make sure to register deprecated conditions with NewRemoveStaleConditions()
func HandleProgressingOrDegraded(typePrefix string, reason string, err error) []ConditionUpdate {
	updateStatusFuncs := []ConditionUpdate{}
	if errors.IsSyncError(err) {
		updateStatusFuncs = append(updateStatusFuncs, HandleDegraded(typePrefix, reason, nil))
		updateStatusFuncs = append(updateStatusFuncs, HandleProgressing(typePrefix, reason, err))
	} else {
		updateStatusFuncs = append(updateStatusFuncs, HandleDegraded(typePrefix, reason, err))
		updateStatusFuncs = append(updateStatusFuncs, HandleProgressing(typePrefix, reason, nil))
	}
	return updateStatusFuncs
}

func handleCondition(conditionTypeWithSuffix string, reason string, err error) operatorsv1.OperatorCondition {
	if err != nil {
		klog.Errorln(conditionTypeWithSuffix, reason, err.Error())
		return operatorsv1.OperatorCondition{
			Type:    conditionTypeWithSuffix,
			Status:  setConditionValue(conditionTypeWithSuffix, err),
			Reason:  reason,
			Message: err.Error(),
		}
	}
	return operatorsv1.OperatorCondition{
		Type:   conditionTypeWithSuffix,
		Status: setConditionValue(conditionTypeWithSuffix, err),
	}
}

// Available and Upgradable are an inversions of the Degraded and Progressing conditions
func setConditionValue(conditionType string, err error) operatorsv1.ConditionStatus {
	if strings.HasSuffix(conditionType, operatorsv1.OperatorStatusTypeAvailable) || strings.HasSuffix(conditionType, operatorsv1.OperatorStatusTypeUpgradeable) {
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

type StatusHandler struct {
	client v1helpers.OperatorClient
	// conditionUpdates are keyed by condition type so that we always choose the latest as authoritative
	conditionUpdates map[string]v1helpers.UpdateStatusFunc

	statusFuncs []v1helpers.UpdateStatusFunc
}

type ConditionUpdate struct {
	ConditionType  string
	StatusUpdateFn v1helpers.UpdateStatusFunc
}

func (c *StatusHandler) AddCondition(conditionUpdate ConditionUpdate) {
	c.conditionUpdates[conditionUpdate.ConditionType] = conditionUpdate.StatusUpdateFn
}

func (c *StatusHandler) AddConditions(conditionUpdates []ConditionUpdate) {
	for i := range conditionUpdates {
		conditionUpdate := conditionUpdates[i]
		c.conditionUpdates[conditionUpdate.ConditionType] = conditionUpdate.StatusUpdateFn
	}
}

func (c *StatusHandler) FlushAndReturn(returnErr error) error {
	allStatusFns := []v1helpers.UpdateStatusFunc{}
	for i := range c.statusFuncs {
		allStatusFns = append(allStatusFns, c.statusFuncs[i])
	}
	for k := range c.conditionUpdates {
		allStatusFns = append(allStatusFns, c.conditionUpdates[k])
	}

	if _, _, updateErr := v1helpers.UpdateStatus(context.TODO(), c.client, allStatusFns...); updateErr != nil {
		return updateErr
	}
	return returnErr
}

func (c *StatusHandler) UpdateObservedGeneration(newObservedGeneration int64) {
	generationFunc := func(oldStatus *operatorsv1.OperatorStatus) error {
		oldStatus.ObservedGeneration = newObservedGeneration
		return nil
	}
	c.statusFuncs = append(c.statusFuncs, generationFunc)
}

func (c *StatusHandler) UpdateReadyReplicas(newReadyReplicas int32) {
	generationFunc := func(oldStatus *operatorsv1.OperatorStatus) error {
		oldStatus.ReadyReplicas = newReadyReplicas
		return nil
	}
	c.statusFuncs = append(c.statusFuncs, generationFunc)
}

func (c *StatusHandler) UpdateDeploymentGeneration(actualDeployment *appsv1.Deployment) {
	generationFunc := func(oldStatus *operatorsv1.OperatorStatus) error {
		resourcemerge.SetDeploymentGeneration(&oldStatus.Generations, actualDeployment)
		return nil
	}
	c.statusFuncs = append(c.statusFuncs, generationFunc)
}

func NewStatusHandler(client v1helpers.OperatorClient) StatusHandler {
	return StatusHandler{
		client:           client,
		conditionUpdates: map[string]v1helpers.UpdateStatusFunc{},
	}
}

// Outputs the condition as a log message based on the detail of the condition in the form of:
//
//	Status.Condition.<Condition>: <Bool>
//	Status.Condition.<Condition>: <Bool> (<Reason>)
//	Status.Condition.<Condition>: <Bool> (<Reason>) <Message>
//	Status.Condition.<Condition>: <Bool> <Message>
func logConditions(conditions []operatorsv1.OperatorCondition) {
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
