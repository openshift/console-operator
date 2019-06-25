package framework

import (
	"testing"

	v1 "github.com/openshift/api/operator/v1"
	"k8s.io/client-go/util/retry"
)

// func that ensures a clean slate before a test runs.
// setup is more aggressive than cleanup as the request for
// a clean slate on setup is assertive, not courtesy
func StandardSetup(t *testing.T) (*ClientSet, *v1.Console) {
	t.Helper()
	client := MustNewClientset(t, nil)
	operatorConfig := &v1.Console{}

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		conf, err := Pristine(t, client)
		operatorConfig = conf // fix shadowing
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	WaitForSettledState(t, client)

	return client, operatorConfig
}

// courtesy func to return state to something reasonable before
// the next test runs.
func StandardCleanup(t *testing.T, client *ClientSet) {
	t.Helper()
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := Pristine(t, client)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	WaitForSettledState(t, client)
}
