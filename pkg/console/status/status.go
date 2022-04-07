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
//   c.handleDegraded(operatorConfig, "RouteStatus", "FailedHost", error.New("route is not available at canonical host..."))
// generates:
//   - Type RouteStatusDegraded
//     Status: true
//     Reason: Failedhost
//     Message: error string value is used as message
// all degraded suffix conditions will be aggregated into a final "Degraded" status that will be set on the console ClusterOperator
func HandleDegraded(typePrefix string, reason string, err error) v1helpers.UpdateStatusFunc {
	conditionType := typePrefix + operatorsv1.OperatorStatusTypeDegraded
	condition := handleCondition(conditionType, reason, err)
	return v1helpers.UpdateConditionFn(condition)
}

func HandleProgressing(typePrefix string, reason string, err error) v1helpers.UpdateStatusFunc {
	conditionType := typePrefix + operatorsv1.OperatorStatusTypeProgressing
	condition := handleCondition(conditionType, reason, err)
	return v1helpers.UpdateConditionFn(condition)
}

func HandleAvailable(typePrefix string, reason string, err error) v1helpers.UpdateStatusFunc {
	conditionType := typePrefix + operatorsv1.OperatorStatusTypeAvailable
	condition := handleCondition(conditionType, reason, err)
	return v1helpers.UpdateConditionFn(condition)
}

func HandleUpgradable(typePrefix string, reason string, err error) v1helpers.UpdateStatusFunc {
	conditionType := typePrefix + operatorsv1.OperatorStatusTypeUpgradeable
	condition := handleCondition(conditionType, reason, err)
	return v1helpers.UpdateConditionFn(condition)
}

// HandleProgressingOrDegraded exists until we remove type SyncError
// If isSyncError
// - Type suffix will be set to Progressing
// if it is any other kind of error
// - Type suffix will be set to Degraded
// TODO: when we eliminate the special case SyncError, this helper can go away.
// When we do that, however, we must make sure to register deprecated conditions with NewRemoveStaleConditions()
func HandleProgressingOrDegraded(typePrefix string, reason string, err error) []v1helpers.UpdateStatusFunc {
	updateStatusFuncs := []v1helpers.UpdateStatusFunc{}
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
	client      v1helpers.OperatorClient
	statusFuncs []v1helpers.UpdateStatusFunc
}

func (c *StatusHandler) AddCondition(newStatusFunc v1helpers.UpdateStatusFunc) {
	c.statusFuncs = append(c.statusFuncs, newStatusFunc)
}

func (c *StatusHandler) AddConditions(newStatusFuncs []v1helpers.UpdateStatusFunc) {
	for _, newStatusFunc := range newStatusFuncs {
		c.statusFuncs = append(c.statusFuncs, newStatusFunc)
	}
}

func (c *StatusHandler) FlushAndReturn(returnErr error) error {
	if _, _, updateErr := v1helpers.UpdateStatus(context.TODO(), c.client, c.statusFuncs...); updateErr != nil {
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
		client: client,
	}
}

// Outputs the condition as a log message based on the detail of the condition in the form of:
//   Status.Condition.<Condition>: <Bool>
//   Status.Condition.<Condition>: <Bool> (<Reason>)
//   Status.Condition.<Condition>: <Bool> (<Reason>) <Message>
//   Status.Condition.<Condition>: <Bool> <Message>
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
