package operator

import (
	"fmt"

	"github.com/golang/glog"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	"github.com/openshift/service-ca-operator/pkg/operator/operatorclient"
)

func (c *serviceCAOperator) setFailingStatus(operatorConfig *operatorv1.ServiceCA, reason, message string) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions,
		operatorv1.OperatorCondition{
			Type:    operatorv1.OperatorStatusTypeFailing,
			Status:  operatorv1.ConditionTrue,
			Reason:  reason,
			Message: message,
		})

	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:   operatorv1.OperatorStatusTypeProgressing,
		Status: operatorv1.ConditionFalse,
	})

	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions,
		operatorv1.OperatorCondition{
			Type:   operatorv1.OperatorStatusTypeAvailable,
			Status: operatorv1.ConditionFalse,
		})
}

func (c *serviceCAOperator) setProgressingStatus(operatorConfig *operatorv1.ServiceCA, message string) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions,
		operatorv1.OperatorCondition{
			Type:    operatorv1.OperatorStatusTypeProgressing,
			Status:  operatorv1.ConditionTrue,
			Message: message,
		})

	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:   operatorv1.OperatorStatusTypeFailing,
		Status: operatorv1.ConditionFalse,
	})

	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions,
		operatorv1.OperatorCondition{
			Type:   operatorv1.OperatorStatusTypeAvailable,
			Status: operatorv1.ConditionFalse,
		})
}

func (c *serviceCAOperator) setAvailableStatus(operatorConfig *operatorv1.ServiceCA) {
	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:   operatorv1.OperatorStatusTypeAvailable,
		Status: operatorv1.ConditionTrue,
	})

	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:   operatorv1.OperatorStatusTypeProgressing,
		Status: operatorv1.ConditionFalse,
	})

	v1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, operatorv1.OperatorCondition{
		Type:   operatorv1.OperatorStatusTypeFailing,
		Status: operatorv1.ConditionFalse,
	})
}

func isDeploymentStatusAvailable(deploy *appsv1.Deployment) bool {
	return deploy.Status.AvailableReplicas > 0
}

// isDeploymentStatusAvailableAndUpdated returns true when at least one
// replica instance exists and all replica instances are current,
// there are no replica instances remaining from the previous deployment.
// There may still be additional replica instances being created.
func isDeploymentStatusAvailableAndUpdated(deploy *appsv1.Deployment) bool {
	return deploy.Status.AvailableReplicas > 0 &&
		deploy.Status.ObservedGeneration >= deploy.Generation &&
		deploy.Status.UpdatedReplicas == deploy.Status.Replicas
}

func isDeploymentStatusComplete(deploy *appsv1.Deployment) bool {
	replicas := int32(1)
	if deploy.Spec.Replicas != nil {
		replicas = *(deploy.Spec.Replicas)
	}
	return deploy.Status.UpdatedReplicas == replicas &&
		deploy.Status.Replicas == replicas &&
		deploy.Status.AvailableReplicas == replicas &&
		deploy.Status.ObservedGeneration >= deploy.Generation
}

func (c *serviceCAOperator) syncStatus(operatorConfigCopy *operatorv1.ServiceCA, deployments []string) (bool, error) {
	ready := 0
	existingDeploymentsAndReplicas := 0
	for _, dep := range deployments {
		existing, err := c.appsv1Client.Deployments(operatorclient.TargetNamespace).Get(dep, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				statusMsg := fmt.Sprintf("Deployment %s does not exist", dep)
				c.setProgressingStatus(operatorConfigCopy, statusMsg)
				return false, nil
			}
			c.setFailingStatus(operatorConfigCopy, "Error getting service-ca-operator managed deployments", err.Error())
			return false, err
		}
		if existing.DeletionTimestamp != nil {
			glog.Infof("Deployment %s is being deleted", dep)
			statusMsg := fmt.Sprintf("Deployment %s is being deleted", dep)
			c.setProgressingStatus(operatorConfigCopy, statusMsg)
			return false, nil
		}
		if !isDeploymentStatusAvailable(existing) {
			glog.Infof("Deployment %s does not have available replicas", dep)
			statusMsg := fmt.Sprintf("Deployment %s does not have available replicas", dep)
			c.setProgressingStatus(operatorConfigCopy, statusMsg)
			return false, nil
		}
		existingDeploymentsAndReplicas++

		if !isDeploymentStatusComplete(existing) {
			glog.Infof("The deployment %s has not completed", dep)
			statusMsg := fmt.Sprintf("Deployment %s has not completed", dep)
			c.setProgressingStatus(operatorConfigCopy, statusMsg)
			return false, nil
		}
		if isDeploymentStatusAvailableAndUpdated(existing) {
			glog.Infof("Deployment %s is available and updated", dep)
			ready++
		}
	}
	// set Available if replica instances are created.
	// report version if all deployments are available and updated
	if ready == len(deployments) {
		glog.Infof("All deployments managed by service-ca-operator are available and updated")
		c.setAvailableStatus(operatorConfigCopy)
		return true, nil
	}
	// report Available is deployments and replicas exist, but don't report version
	// if there are replica instances remaining from previous deployment
	if existingDeploymentsAndReplicas == len(deployments) {
		c.setAvailableStatus(operatorConfigCopy)
		return false, nil
	}
	return false, nil
}
