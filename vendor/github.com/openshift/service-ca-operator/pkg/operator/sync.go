package operator

import (
	"github.com/golang/glog"
	operatorv1 "github.com/openshift/api/operator/v1"
)

func syncControllers(c serviceCAOperator, operatorConfig *operatorv1.ServiceCA) error {
	// Any modification of resource we want to trickle down to force deploy all of the controllers.
	// Sync the controller NS and the other resources. These should be mostly static.
	needsDeploy, err := manageControllerNS(c)
	if err != nil {
		return err
	}

	err = manageSignerControllerResources(c, &needsDeploy)
	if err != nil {
		return err
	}

	err = manageAPIServiceControllerResources(c, &needsDeploy)
	if err != nil {
		return err
	}

	err = manageConfigMapCABundleControllerResources(c, &needsDeploy)
	if err != nil {
		return err
	}

	// Sync the CA (regenerate if missing).
	caModified, err := manageSignerCA(c.corev1Client, c.eventRecorder)
	if err != nil {
		return err
	}
	// Sync the CA bundle. This will be updated if the CA has changed.
	_, err = manageSignerCABundle(c.corev1Client, c.eventRecorder, caModified)
	if err != nil {
		return err
	}

	// Sync the signing controller.
	configModified, err := manageSignerControllerConfig(c.corev1Client, c.eventRecorder)
	if err != nil {
		return err
	}
	_, err = manageSignerControllerDeployment(c.appsv1Client, c.eventRecorder, operatorConfig, needsDeploy || caModified || configModified)
	if err != nil {
		return err
	}

	// Sync the API service controller.
	configModified, err = manageAPIServiceControllerConfig(c.corev1Client, c.eventRecorder)
	if err != nil {
		return err
	}
	_, err = manageAPIServiceControllerDeployment(c.appsv1Client, c.eventRecorder, operatorConfig, needsDeploy || caModified || configModified)
	if err != nil {
		return err
	}

	// Sync the API service controller.
	configModified, err = manageConfigMapCABundleControllerConfig(c.corev1Client, c.eventRecorder)
	if err != nil {
		return err
	}
	_, err = manageConfigMapCABundleControllerDeployment(c.appsv1Client, c.eventRecorder, operatorConfig, needsDeploy || caModified || configModified)
	if err != nil {
		return err
	}

	glog.V(4).Infof("synced all controller resources")
	return nil
}
