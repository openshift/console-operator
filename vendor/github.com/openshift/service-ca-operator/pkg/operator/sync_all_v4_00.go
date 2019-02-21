package operator

import (
	scsv1 "github.com/openshift/service-ca-operator/pkg/apis/serviceca/v1"
)

// sync_v4_00_to_latest takes care of synchronizing (not upgrading) the thing we're managing.
// most of the time the sync method will be good for a large span of minor versions
func sync_v4_00_to_latest(c serviceCertSignerOperator, operatorConfig *scsv1.ServiceCA) error {
	err := syncSigningController_v4_00_to_latest(c, operatorConfig)
	if err != nil {
		return err
	}
	err = syncAPIServiceController_v4_00_to_latest(c, operatorConfig)
	if err != nil {
		return err
	}
	err = syncConfigMapCABundleController_v4_00_to_latest(c, operatorConfig)
	if err != nil {
		return err
	}
	return nil
}
