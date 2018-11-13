package operator

import (
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	statusv1 "github.com/openshift/cluster-version-operator/pkg/apis/operatorstatus.openshift.io/v1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
)

func defaultOperatorStatus() *statusv1.ClusterOperator {

	status := &statusv1.ClusterOperator{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "config.openshift.io/v1",
			Kind:       "ClusterOperator",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      OpenShiftConsoleName,
			Namespace: OpenShiftConsoleNamespace,
		},
		Spec: statusv1.ClusterOperatorSpec{},
		Status: statusv1.ClusterOperatorStatus{
			Conditions: nil,
			Version:    "",
		},
	}

	return status
}

// TODO: convert my CR.Status into the approprate Status on this:
// https://github.com/openshift/cluster-version-operator/blob/master/pkg/apis/operatorstatus.openshift.io/v1/types.go#L42
// Example:
// - all three of these conditions SHOULD exist at once
//   but toggle the status True|False
//   and leave an arbitrary Reason and message
//   Reason is CamelCase for machines
//   message is sensible case for peoples
//status:
//conditions:
//	- lastTransitionTime: null
//message: replicas ready
//status: 'True'
//	 type: Available
//	- lastTransitionTime: '2018-10-17T19:11:23Z'
//	 message: no errors found
//status: 'False'
//	 type: Failing
//	- lastTransitionTime: null
//message: available and not waiting for a change
//status: 'False'
//	 type: Progressing
func ApplyClusterOperatorStatus(console *v1alpha1.Console) error {
	status := defaultOperatorStatus()
	// ensure it exists
	if err := sdk.Get(status); errors.IsNotFound(err) {
		if sdk.Create(status); err != nil {
			return err
		}
	}
	status.Status.Conditions = []statusv1.ClusterOperatorStatusCondition{
		{
			Type:               statusv1.OperatorAvailable,
			Status:             statusv1.ConditionUnknown,
			LastTransitionTime: metav1.Now(),
		},
		{
			Type:               statusv1.OperatorProgressing,
			Status:             statusv1.ConditionUnknown,
			LastTransitionTime: metav1.Now(),
		},
		{
			Type:               statusv1.OperatorFailing,
			Status:             statusv1.ConditionUnknown,
			LastTransitionTime: metav1.Now(),
		},
	}
	return sdk.Update(status)
}
