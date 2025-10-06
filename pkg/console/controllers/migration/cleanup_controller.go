package migration

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"

	"github.com/openshift/console-operator/pkg/api"
)

const (
	ConversionWebhookDeploymentName = "console-conversion-webhook"
	ConversionWebhookServiceName    = "webhook"
	ConversionWebhookSecretName     = "webhook-serving-cert"
)

type MigrationCleanupController struct {
	kubeClient kubernetes.Interface
	recorder   events.Recorder
}

func NewMigrationCleanupController(
	kubeClient kubernetes.Interface,
	recorder events.Recorder,
) factory.Controller {
	c := &MigrationCleanupController{
		kubeClient: kubeClient,
		recorder:   recorder,
	}

	return factory.New().
		WithSync(c.Sync).
		ToController("MigrationCleanupController", recorder)
}

func (c *MigrationCleanupController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	klog.V(4).Info("Running console-conversion-webhook cleanup from 4.16 → 4.xx migration")

	// Perform cleanup
	if err := c.cleanupConversionWebhookResources(ctx); err != nil {
		return fmt.Errorf("failed to cleanup conversion webhook resources: %w", err)
	}

	return nil
}

func (c *MigrationCleanupController) cleanupConversionWebhookResources(ctx context.Context) error {
	var errs []error
	var deletedResources []string

	// Delete Deployment
	err := c.kubeClient.AppsV1().Deployments(api.TargetNamespace).Delete(
		ctx, ConversionWebhookDeploymentName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to delete deployment %s: %w", ConversionWebhookDeploymentName, err))
	} else if err == nil {
		klog.V(4).Infof("Deleted deployment: %s", ConversionWebhookDeploymentName)
		deletedResources = append(deletedResources, "deployment/"+ConversionWebhookDeploymentName)
	}

	// Delete Service
	err = c.kubeClient.CoreV1().Services(api.TargetNamespace).Delete(
		ctx, ConversionWebhookServiceName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to delete service %s: %w", ConversionWebhookServiceName, err))
	} else if err == nil {
		klog.V(4).Infof("Deleted service: %s", ConversionWebhookServiceName)
		deletedResources = append(deletedResources, "service/"+ConversionWebhookServiceName)
	}

	// Delete Secret
	err = c.kubeClient.CoreV1().Secrets(api.TargetNamespace).Delete(
		ctx, ConversionWebhookSecretName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to delete secret %s: %w", ConversionWebhookSecretName, err))
	} else if err == nil {
		klog.V(4).Infof("Deleted secret: %s", ConversionWebhookSecretName)
		deletedResources = append(deletedResources, "secret/"+ConversionWebhookSecretName)
	}

	// Log summary and emit event if any resources were actually deleted
	if len(deletedResources) > 0 {
		klog.V(4).Infof("console-conversion-webhook cleanup completed: deleted %v", deletedResources)
		c.recorder.Eventf("MigrationCleanupCompleted", "Successfully cleaned up console-conversion-webhook resources: %v", deletedResources)
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}

	return nil
}
