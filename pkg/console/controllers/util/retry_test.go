package util

import (
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "conflict is retryable",
			err:       apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, "test", fmt.Errorf("conflict")),
			retryable: true,
		},
		{
			name:      "server timeout is retryable",
			err:       apierrors.NewServerTimeout(schema.GroupResource{Resource: "configmaps"}, "get", 0),
			retryable: true,
		},
		{
			name:      "too many requests is retryable",
			err:       apierrors.NewTooManyRequests("slow down", 1),
			retryable: true,
		},
		{
			name:      "service unavailable is retryable",
			err:       apierrors.NewServiceUnavailable("unavailable"),
			retryable: true,
		},
		{
			name:      "internal error is retryable",
			err:       apierrors.NewInternalError(fmt.Errorf("oops")),
			retryable: true,
		},
		{
			name:      "not found is retryable",
			err:       apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "test"),
			retryable: true,
		},
		{
			name:      "generic error is retryable",
			err:       fmt.Errorf("connection refused"),
			retryable: true,
		},
		{
			name:      "forbidden is not retryable",
			err:       apierrors.NewForbidden(schema.GroupResource{Resource: "configmaps"}, "test", fmt.Errorf("forbidden")),
			retryable: false,
		},
		{
			name:      "invalid is not retryable",
			err:       apierrors.NewInvalid(schema.GroupKind{Kind: "ConfigMap"}, "test", nil),
			retryable: false,
		},
		{
			name:      "method not supported is not retryable",
			err:       apierrors.NewMethodNotSupported(schema.GroupResource{Resource: "configmaps"}, "patch"),
			retryable: false,
		},
		{
			name:      "already exists is not retryable",
			err:       apierrors.NewAlreadyExists(schema.GroupResource{Resource: "configmaps"}, "test"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryableError(tt.err)
			if got != tt.retryable {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tt.err, got, tt.retryable)
			}
		})
	}
}

func TestRetryOnTransientError(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		calls := 0
		err := RetryOnTransientError(func() error {
			calls++
			return nil
		})
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if calls != 1 {
			t.Errorf("expected 1 call, got %d", calls)
		}
	})

	t.Run("retries transient error then succeeds", func(t *testing.T) {
		calls := 0
		err := RetryOnTransientError(func() error {
			calls++
			if calls < 3 {
				return apierrors.NewServerTimeout(schema.GroupResource{Resource: "configmaps"}, "get", 0)
			}
			return nil
		})
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if calls != 3 {
			t.Errorf("expected 3 calls, got %d", calls)
		}
	})

	t.Run("does not retry permanent error", func(t *testing.T) {
		calls := 0
		err := RetryOnTransientError(func() error {
			calls++
			return apierrors.NewForbidden(schema.GroupResource{Resource: "configmaps"}, "test", fmt.Errorf("forbidden"))
		})
		if err == nil {
			t.Error("expected error, got nil")
		}
		if calls != 1 {
			t.Errorf("expected 1 call (no retry), got %d", calls)
		}
	})

	t.Run("exhausts retries on persistent transient error", func(t *testing.T) {
		calls := 0
		err := RetryOnTransientError(func() error {
			calls++
			return apierrors.NewServerTimeout(schema.GroupResource{Resource: "configmaps"}, "get", 0)
		})
		if err == nil {
			t.Error("expected error after exhausting retries, got nil")
		}
		if calls != TransientBackoff.Steps {
			t.Errorf("expected %d calls, got %d", TransientBackoff.Steps, calls)
		}
	})

	t.Run("retries generic network error", func(t *testing.T) {
		calls := 0
		err := RetryOnTransientError(func() error {
			calls++
			if calls < 2 {
				return fmt.Errorf("connection refused")
			}
			return nil
		})
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if calls != 2 {
			t.Errorf("expected 2 calls, got %d", calls)
		}
	})
}
