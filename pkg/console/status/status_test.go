package status

import (
	"fmt"
	"testing"

	operatorsv1 "github.com/openshift/api/operator/v1"
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
