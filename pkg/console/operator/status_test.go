package operator

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-test/deep"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
	customerrors "github.com/openshift/console-operator/pkg/console/errors"
)

func TestHandleDegraded(t *testing.T) {
	type args struct {
		operatorConfig *operatorv1.Console
		typePrefix     string
		errorReason    string
		err            error
	}
	tests := []struct {
		name string
		args args
		want operatorv1.OperatorCondition
	}{
		{
			name: "Set FooSyncDegraded:True on Operator if error is provided and not type SyncError",
			args: args{
				operatorConfig: &operatorv1.Console{},
				typePrefix:     "FooSync",
				errorReason:    "ItsBroken",
				err:            errors.New("Something is broken"),
			},
			want: operatorv1.OperatorCondition{
				Type:               "FooSyncDegraded",
				Status:             "True",
				Reason:             "ItsBroken",
				Message:            "Something is broken",
				LastTransitionTime: v1.Time{},
			},
		}, {
			name: "Set FooSyncDegraded:False on Operator if no error provided",
			args: args{
				operatorConfig: &operatorv1.Console{},
				typePrefix:     "FooSync",
				errorReason:    "",
				err:            nil,
			},
			want: operatorv1.OperatorCondition{
				Type:               "FooSyncDegraded",
				Status:             "False",
				LastTransitionTime: v1.Time{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operator := consoleOperator{}
			operator.HandleDegraded(tt.args.operatorConfig, tt.args.typePrefix, tt.args.errorReason, tt.args.err)

			condition := tt.args.operatorConfig.Status.Conditions[0]
			// nil the time for easier matching
			condition.LastTransitionTime = v1.Time{}
			if diff := deep.Equal(condition, tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}

}

func TestHandleProgressing(t *testing.T) {
	type args struct {
		operatorConfig *operatorv1.Console
		typePrefix     string
		errorReason    string
		err            error
	}
	tests := []struct {
		name string
		args args
		want operatorv1.OperatorCondition
	}{
		{
			name: "Set FooSyncProgressing:True on Operator if error is provided and is type SyncError",
			args: args{
				operatorConfig: &operatorv1.Console{},
				typePrefix:     "FooSync",
				errorReason:    "ItsProgressing",
				err:            customerrors.NewSyncError("This isn't broken, we are just progressing"),
			},
			want: operatorv1.OperatorCondition{
				Type:               "FooSyncProgressing",
				Status:             "True",
				Reason:             "ItsProgressing",
				Message:            "This isn't broken, we are just progressing",
				LastTransitionTime: v1.Time{},
			},
		}, {
			name: "Set FooSyncProgressing:False on Operator if no error provided",
			args: args{
				operatorConfig: &operatorv1.Console{},
				typePrefix:     "FooSync",
				errorReason:    "",
				err:            nil,
			},
			want: operatorv1.OperatorCondition{
				Type:               "FooSyncProgressing",
				Status:             "False",
				LastTransitionTime: v1.Time{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operator := consoleOperator{}
			operator.HandleProgressing(tt.args.operatorConfig, tt.args.typePrefix, tt.args.errorReason, tt.args.err)

			condition := tt.args.operatorConfig.Status.Conditions[0]
			// nil the time for easier matching
			condition.LastTransitionTime = v1.Time{}
			if diff := deep.Equal(condition, tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestHandleProgressingOrDegraded(t *testing.T) {
	type input struct {
		operatorConfig *operatorv1.Console
		typePrefix     string
		errorReason    string
		err            error
	}
	type output struct {
		degraded    operatorv1.OperatorCondition
		progressing operatorv1.OperatorCondition
	}
	tests := []struct {
		name   string
		input  input
		output output
	}{
		{
			name: "If no error provided, set FooSyncDegraded:False, FooSyncProgressing:False",
			input: input{
				operatorConfig: &operatorv1.Console{},
				typePrefix:     "FooSync",
				errorReason:    "",
				err:            nil,
			},
			output: output{
				progressing: operatorv1.OperatorCondition{
					Type:               "FooSyncProgressing",
					Status:             "False",
					LastTransitionTime: v1.Time{},
				},
				degraded: operatorv1.OperatorCondition{
					Type:               "FooSyncDegraded",
					Status:             "False",
					LastTransitionTime: v1.Time{},
				},
			},
		},
		{
			name: "If error provided that is not type SyncError, set FooSyncDegraded:True, FooSyncProgressing:False",
			input: input{
				operatorConfig: &operatorv1.Console{},
				typePrefix:     "FooSync",
				errorReason:    "ItsBroken",
				err:            errors.New("Something is broken"),
			},
			output: output{
				progressing: operatorv1.OperatorCondition{
					Type:               "FooSyncProgressing",
					Status:             "False",
					LastTransitionTime: v1.Time{},
				},
				degraded: operatorv1.OperatorCondition{
					Type:               "FooSyncDegraded",
					Status:             "True",
					Reason:             "ItsBroken",
					Message:            "Something is broken",
					LastTransitionTime: v1.Time{},
				},
			},
		}, {
			name: "If error provided that is type SyncError, set FooSyncDegraded:False, FooSyncProgressing:True",
			input: input{
				operatorConfig: &operatorv1.Console{},
				typePrefix:     "FooSync",
				errorReason:    "ItsProgressingNotBroken",
				err:            customerrors.NewSyncError("Something is not broken,it is progressing"),
			},
			output: output{
				degraded: operatorv1.OperatorCondition{
					Type:               "FooSyncDegraded",
					Status:             "False",
					LastTransitionTime: v1.Time{},
				},
				progressing: operatorv1.OperatorCondition{
					Type:               "FooSyncProgressing",
					Status:             "True",
					Reason:             "ItsProgressingNotBroken",
					Message:            "Something is not broken,it is progressing",
					LastTransitionTime: v1.Time{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operator := consoleOperator{}
			operator.HandleProgressingOrDegraded(tt.input.operatorConfig, tt.input.typePrefix, tt.input.errorReason, tt.input.err)

			progressingCondition := operatorv1.OperatorCondition{}
			degradedCondition := operatorv1.OperatorCondition{}
			for _, condition := range tt.input.operatorConfig.Status.Conditions {
				if strings.HasSuffix(condition.Type, "Degraded") {
					degradedCondition = condition
				}
				if strings.HasSuffix(condition.Type, "Progressing") {
					progressingCondition = condition
				}
			}
			// nil the timestamps for easier matching
			progressingCondition.LastTransitionTime = v1.Time{}
			degradedCondition.LastTransitionTime = v1.Time{}

			if diff := deep.Equal(degradedCondition, tt.output.degraded); diff != nil {
				t.Error(diff)
			}
			if diff := deep.Equal(progressingCondition, tt.output.progressing); diff != nil {
				t.Error(diff)
			}
		})
	}
}
