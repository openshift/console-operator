package util

import (
	"time"

	// kube
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

// TransientBackoff defines the retry parameters for transient API errors.
// 3 steps with 500ms base and 2.0 factor gives: 500ms, 1s, 2s = ~3.5s max per call.
var TransientBackoff = wait.Backoff{
	Steps:    3,
	Duration: 500 * time.Millisecond,
	Factor:   2.0,
	Jitter:   0.1,
}

// IsRetryableError returns true for errors worth retrying — everything
// except known permanent errors. This naturally handles both API status
// errors (apierrors.StatusError) and network-level errors (connection
// refused, EOF, TLS failures) without explicit net.Error detection.
func IsRetryableError(err error) bool {
	if apierrors.IsForbidden(err) ||
		apierrors.IsInvalid(err) ||
		apierrors.IsMethodNotSupported(err) ||
		apierrors.IsNotAcceptable(err) ||
		apierrors.IsAlreadyExists(err) {
		return false
	}
	return true
}

// RetryOnTransientError wraps a function call with retry logic to absorb
// transient API server errors (conflicts, timeouts, connection refused)
// that occur during upgrades. Only API write operations (Apply, Create,
// Update, Delete) should be wrapped — not lister/cache reads.
func RetryOnTransientError(fn func() error) error {
	attempt := 0
	return retry.OnError(TransientBackoff, IsRetryableError, func() error {
		err := fn()
		if err != nil {
			attempt++
			klog.V(4).Infof("transient error (attempt %d/%d): %v", attempt, TransientBackoff.Steps, err)
		}
		return err
	})
}
