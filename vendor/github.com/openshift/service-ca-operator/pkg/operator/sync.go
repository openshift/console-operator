package operator

import (
	"fmt"

	operatorv1 "github.com/openshift/api/operator/v1"
)

func syncControllers(c serviceCAOperator, operatorConfig *operatorv1.ServiceCA) error {
	// Sync the controller NS and the other resources. These should be mostly static.
	modified, err := manageControllerNS(c)
	if err != nil {
		return fmt.Errorf("Error syncing NS: %v", err)
	}
	modified, reason := manageSignerControllerResources(c)
	if err != nil {
		return fmt.Errorf("Error syncing signer controller resources: %v", reason)
	}
	modified, reason = manageAPIServiceControllerResources(c)
	if err != nil {
		return fmt.Errorf("Error syncing API service controller resources: %v", reason)
	}
	modified, reason = manageConfigMapCABundleControllerResources(c)
	if err != nil {
		return fmt.Errorf("Error syncing CA bundle controller resources: %v", reason)
	}

	// Sync the CA (regenerate if missing).
	_, modified, err = manageSignerCA(c.corev1Client, c.eventRecorder)
	if err != nil {
		return fmt.Errorf("Error syncing signer CA: %v", err)
	}
	// Sync the CA bundle. This will be updated if the CA has changed.
	_, modified, err = manageSignerCABundle(c.corev1Client, c.eventRecorder)
	if err != nil {
		return fmt.Errorf("Error syncing signer CA bundle: %v", err)
	}

	// Sync the signing controller.
	_, modified, err = manageSignerControllerConfig(c.corev1Client, c.eventRecorder)
	if err != nil {
		return fmt.Errorf("Error syncing signing controller config: %v", err)
	}
	_, modified, err = manageSignerControllerDeployment(c.appsv1Client, c.eventRecorder, operatorConfig, modified)
	if err != nil {
		return fmt.Errorf("Error syncing signing controller deployment: %v", err)
	}

	// Sync the API service controller.
	_, modified, err = manageAPIServiceControllerConfig(c.corev1Client, c.eventRecorder)
	if err != nil {
		return fmt.Errorf("Error syncing API service controller config: %v", err)
	}
	_, modified, err = manageAPIServiceControllerDeployment(c.appsv1Client, c.eventRecorder, operatorConfig, modified)
	if err != nil {
		return fmt.Errorf("Error syncing API service controller deployment: %v", err)
	}

	// Sync the API service controller.
	_, modified, err = manageConfigMapCABundleControllerConfig(c.corev1Client, c.eventRecorder)
	if err != nil {
		return fmt.Errorf("Error syncing CA bundle controller config: %v", err)
	}
	_, _, err = manageConfigMapCABundleControllerDeployment(c.appsv1Client, c.eventRecorder, operatorConfig, modified)
	if err != nil {
		return fmt.Errorf("Error syncing CA bundle controller deployment: %v", err)
	}

	return nil
}
