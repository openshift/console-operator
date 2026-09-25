package status

import (
	"context"
	"errors"
	"fmt"
	"testing"

	operatorsv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

func TestHandleProgressingOrDegraded_NilError_ClearsConditions(t *testing.T) {
	updates := HandleProgressingOrDegraded("OIDCProviderTrustedAuthorityConfigGet", "", nil)

	if len(updates) != 2 {
		t.Fatalf("expected 2 condition updates, got %d", len(updates))
	}

	foundDegraded := false
	foundProgressing := false

	for _, update := range updates {
		switch update.ConditionType {
		case "OIDCProviderTrustedAuthorityConfigGetDegraded":
			foundDegraded = true
		case "OIDCProviderTrustedAuthorityConfigGetProgressing":
			foundProgressing = true
		}
	}

	if !foundDegraded {
		t.Error("expected OIDCProviderTrustedAuthorityConfigGetDegraded condition update")
	}
	if !foundProgressing {
		t.Error("expected OIDCProviderTrustedAuthorityConfigGetProgressing condition update")
	}
}

func TestHandleProgressingOrDegraded_NilError_SetsFalse(t *testing.T) {
	updates := HandleProgressingOrDegraded("TestPrefix", "", nil)

	// Seed status with stale True conditions to verify they are replaced
	status := &operatorsv1.OperatorStatus{
		Conditions: []operatorsv1.OperatorCondition{
			{
				Type:    "TestPrefixDegraded",
				Status:  operatorsv1.ConditionTrue,
				Reason:  "StaleReason",
				Message: "stale error",
			},
			{
				Type:    "TestPrefixProgressing",
				Status:  operatorsv1.ConditionTrue,
				Reason:  "StaleReason",
				Message: "stale progressing",
			},
		},
	}

	for _, update := range updates {
		if err := update.StatusUpdateFn(status); err != nil {
			t.Fatalf("failed to apply condition update: %v", err)
		}
	}

	expectedTypes := map[string]bool{
		"TestPrefixDegraded":    false,
		"TestPrefixProgressing": false,
	}
	for _, cond := range status.Conditions {
		if _, ok := expectedTypes[cond.Type]; ok {
			expectedTypes[cond.Type] = true
			if cond.Status != operatorsv1.ConditionFalse {
				t.Errorf("expected condition %q to be False, got %s", cond.Type, cond.Status)
			}
		}
	}
	for condType, found := range expectedTypes {
		if !found {
			t.Errorf("expected condition %q to be present but was not found", condType)
		}
	}
}

func TestHandleProgressingOrDegraded_WithError_SetsDegradedTrue(t *testing.T) {
	testErr := fmt.Errorf("test error")
	updates := HandleProgressingOrDegraded("TestPrefix", "TestReason", testErr)

	status := &operatorsv1.OperatorStatus{}
	for _, update := range updates {
		if err := update.StatusUpdateFn(status); err != nil {
			t.Fatalf("failed to apply condition update: %v", err)
		}
	}

	expectedTypes := map[string]bool{
		"TestPrefixDegraded":    false,
		"TestPrefixProgressing": false,
	}
	for _, cond := range status.Conditions {
		switch cond.Type {
		case "TestPrefixDegraded":
			expectedTypes[cond.Type] = true
			if cond.Status != operatorsv1.ConditionTrue {
				t.Errorf("expected TestPrefixDegraded to be True, got %s", cond.Status)
			}
			if cond.Reason != "TestReason" {
				t.Errorf("expected reason 'TestReason', got %q", cond.Reason)
			}
			if cond.Message != "test error" {
				t.Errorf("expected message 'test error', got %q", cond.Message)
			}
		case "TestPrefixProgressing":
			expectedTypes[cond.Type] = true
			if cond.Status != operatorsv1.ConditionFalse {
				t.Errorf("expected TestPrefixProgressing to be False, got %s", cond.Status)
			}
		}
	}
	for condType, found := range expectedTypes {
		if !found {
			t.Errorf("expected condition %q to be present but was not found", condType)
		}
	}
}

func TestStatusHandlerFlushAndReturnCanceledContext(t *testing.T) {
	wrappedCanceledErr := fmt.Errorf("failed to get OLM config: %w", context.Canceled)
	tests := []struct {
		name      string
		returnErr error
		wantErr   error
	}{
		{
			name:      "returns original reconciliation error",
			returnErr: wrappedCanceledErr,
			wantErr:   wrappedCanceledErr,
		},
		{
			name:    "returns context error without reconciliation error",
			wantErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &recordingOperatorClient{
				OperatorClient: v1helpers.NewFakeOperatorClient(
					&operatorsv1.OperatorSpec{},
					&operatorsv1.OperatorStatus{},
					nil,
				),
			}
			statusHandler := NewStatusHandler(client)
			statusHandler.AddConditions(HandleProgressingOrDegraded("Test", "FailedGet", fmt.Errorf("genuine API error")))

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			gotErr := statusHandler.FlushAndReturn(ctx, tt.returnErr)
			if gotErr != tt.wantErr {
				t.Fatalf("expected error %v, got %v", tt.wantErr, gotErr)
			}
			if client.updateCalls != 0 {
				t.Fatalf("expected no status update, got %d", client.updateCalls)
			}

			_, operatorStatus, _, err := client.GetOperatorState()
			if err != nil {
				t.Fatalf("failed to get operator state: %v", err)
			}
			if len(operatorStatus.Conditions) != 0 {
				t.Fatalf("expected conditions to remain unchanged, got %#v", operatorStatus.Conditions)
			}
		})
	}
}

func TestStatusHandlerFlushAndReturnCancellationDuringNoopStatusFlush(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &recordingOperatorClient{
		OperatorClient: v1helpers.NewFakeOperatorClient(
			&operatorsv1.OperatorSpec{},
			&operatorsv1.OperatorStatus{},
			nil,
		),
		beforeGet: cancel,
	}
	statusHandler := NewStatusHandler(client)

	gotErr := statusHandler.FlushAndReturn(ctx, nil)
	if gotErr != context.Canceled {
		t.Fatalf("expected context cancellation, got %v", gotErr)
	}
	if client.updateCalls != 0 {
		t.Fatalf("expected no status update for a no-op flush, got %d", client.updateCalls)
	}
}

func TestStatusHandlerFlushAndReturnCancellationDuringStatusUpdate(t *testing.T) {
	reconciliationErr := errors.New("reconciliation error")
	tests := []struct {
		name      string
		returnErr error
		wantErr   error
	}{
		{
			name:      "returns original reconciliation error",
			returnErr: reconciliationErr,
			wantErr:   reconciliationErr,
		},
		{
			name:    "returns context error without reconciliation error",
			wantErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			client := &recordingOperatorClient{
				OperatorClient: v1helpers.NewFakeOperatorClient(
					&operatorsv1.OperatorSpec{},
					&operatorsv1.OperatorStatus{},
					nil,
				),
				beforeUpdate: cancel,
			}
			statusHandler := NewStatusHandler(client)
			statusHandler.AddCondition(HandleDegraded("Test", "FailedGet", errors.New("genuine API error")))

			gotErr := statusHandler.FlushAndReturn(ctx, tt.returnErr)
			if gotErr != tt.wantErr {
				t.Fatalf("expected error %v, got %v", tt.wantErr, gotErr)
			}
			if client.updateCalls != 1 {
				t.Fatalf("expected one attempted status update, got %d", client.updateCalls)
			}

			_, operatorStatus, _, err := client.GetOperatorState()
			if err != nil {
				t.Fatalf("failed to get operator state: %v", err)
			}
			if len(operatorStatus.Conditions) != 0 {
				t.Fatalf("expected conditions to remain unchanged, got %#v", operatorStatus.Conditions)
			}
		})
	}
}

func TestStatusHandlerFlushAndReturnActiveContextReturnsStatusUpdateError(t *testing.T) {
	reconciliationErr := errors.New("reconciliation error")
	statusUpdateErr := errors.New("status update error")
	client := &recordingOperatorClient{
		OperatorClient: v1helpers.NewFakeOperatorClient(
			&operatorsv1.OperatorSpec{},
			&operatorsv1.OperatorStatus{},
			nil,
		),
		updateErr: statusUpdateErr,
	}
	statusHandler := NewStatusHandler(client)
	statusHandler.AddCondition(HandleDegraded("Test", "FailedGet", reconciliationErr))

	gotErr := statusHandler.FlushAndReturn(context.Background(), reconciliationErr)
	if gotErr != statusUpdateErr {
		t.Fatalf("expected status update error %v, got %v", statusUpdateErr, gotErr)
	}
	if client.updateCalls != 1 {
		t.Fatalf("expected one attempted status update, got %d", client.updateCalls)
	}
}

func TestStatusHandlerFlushAndReturnActiveContextPersistsDegraded(t *testing.T) {
	client := &recordingOperatorClient{
		OperatorClient: v1helpers.NewFakeOperatorClient(
			&operatorsv1.OperatorSpec{},
			&operatorsv1.OperatorStatus{},
			nil,
		),
	}
	statusHandler := NewStatusHandler(client)
	genuineErr := errors.New("genuine API error")
	statusHandler.AddConditions(HandleProgressingOrDegraded("Test", "FailedGet", genuineErr))

	gotErr := statusHandler.FlushAndReturn(context.Background(), genuineErr)
	if gotErr != genuineErr {
		t.Fatalf("expected original reconciliation error %v, got %v", genuineErr, gotErr)
	}
	if client.updateCalls != 1 {
		t.Fatalf("expected one status update, got %d", client.updateCalls)
	}

	_, operatorStatus, _, err := client.GetOperatorState()
	if err != nil {
		t.Fatalf("failed to get operator state: %v", err)
	}
	condition := v1helpers.FindOperatorCondition(operatorStatus.Conditions, "TestDegraded")
	if condition == nil {
		t.Fatal("expected TestDegraded condition")
	}
	if condition.Status != operatorsv1.ConditionTrue {
		t.Fatalf("expected TestDegraded=True, got %s", condition.Status)
	}
	if condition.Reason != "FailedGet" {
		t.Fatalf("expected reason FailedGet, got %q", condition.Reason)
	}
	if condition.Message != genuineErr.Error() {
		t.Fatalf("expected message %q, got %q", genuineErr, condition.Message)
	}
}

type recordingOperatorClient struct {
	v1helpers.OperatorClient
	beforeGet    func()
	beforeUpdate func()
	updateErr    error
	updateCalls  int
}

func (c *recordingOperatorClient) GetOperatorState() (*operatorsv1.OperatorSpec, *operatorsv1.OperatorStatus, string, error) {
	if c.beforeGet != nil {
		c.beforeGet()
	}
	return c.OperatorClient.GetOperatorState()
}

func (c *recordingOperatorClient) UpdateOperatorStatus(ctx context.Context, resourceVersion string, status *operatorsv1.OperatorStatus) (*operatorsv1.OperatorStatus, error) {
	c.updateCalls++
	if c.beforeUpdate != nil {
		c.beforeUpdate()
	}
	if c.updateErr != nil {
		return nil, c.updateErr
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	return c.OperatorClient.UpdateOperatorStatus(ctx, resourceVersion, status)
}
