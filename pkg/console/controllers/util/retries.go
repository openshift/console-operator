package util

import (
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

type RetryConfig struct {
	Steps    int
	Duration time.Duration
	Factor   float64
	Jitter   float64
}

func defaultRetryConfig() RetryConfig {
	return RetryConfig{
		Steps:    10,
		Duration: 1 * time.Second,
		Factor:   1.0,
		Jitter:   0.1,
	}
}

// wrapper to perform action until given condition is met or until the backoff configuration limits are reached
func RetryWrapper(
	action func() error,
) error {
	cfg := defaultRetryConfig()

	backoff := wait.Backoff{
		Steps:    cfg.Steps,
		Duration: cfg.Duration,
		Factor:   cfg.Factor,
		Jitter:   cfg.Jitter,
	}
	return retry.OnError(backoff, func(err error) bool { return apierrors.IsNotFound(err) }, action)
}
