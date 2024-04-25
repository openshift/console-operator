package deployment

import (
	"k8s.io/apimachinery/pkg/api/errors"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
)

const (
	TelemeterClientDeploymentName      = "telemeter-client"
	TelemeterClientDeploymentNamespace = "openshift-monitoring"
)

func IsTelemeterClientAvailable(deploymentLister appsv1listers.DeploymentLister) (bool, error) {
	deployment, err := deploymentLister.Deployments(TelemeterClientDeploymentNamespace).Get(TelemeterClientDeploymentName)

	if errors.IsNotFound(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return IsAvailable(deployment), nil
}
