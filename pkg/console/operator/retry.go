package operator

import (
	// operator
	"github.com/openshift/console-operator/pkg/console/controllers/util"
)

func retryOnTransientError(fn func() error) error {
	return util.RetryOnTransientError(fn)
}
