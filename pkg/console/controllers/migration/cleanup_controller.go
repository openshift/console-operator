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
	kubeClient       kubernetes.Interface
	recorder         events.Recorder
	cleanupCompleted bool
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
		WithPostStartHooks(c.runCleanupOnce).
		ToController("MigrationCleanupController", recorder)
}

// runCleanupOnce is a post-start hook that runs the cleanup once when the controller starts
func (c *MigrationCleanupController) runCleanupOnce(ctx context.Context, syncContext factory.SyncContext) error {
	klog.V(2).Infof("MigrationCleanupController: checking for console-conversion-webhook resources in namespace %s from 4.16 → 4.xx migration", api.OpenShiftConsoleOperatorNamespace)

	// Perform cleanup
	if err := c.cleanupConversionWebhookResources(ctx); err != nil {
		klog.Errorf("MigrationCleanupController: failed to cleanup conversion webhook resources: %v", err)
		return fmt.Errorf("failed to cleanup conversion webhook resources: %w", err)
	}

	// Mark cleanup as completed
	c.cleanupCompleted = true
	klog.V(2).Info("MigrationCleanupController: cleanup completed successfully")

	return nil
}

func (c *MigrationCleanupController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	// This Sync function is kept minimal since cleanup runs via post-start hook
	// It will only be called if something manually adds to the queue
	if c.cleanupCompleted {
		klog.V(4).Info("MigrationCleanupController: cleanup already completed, skipping")
		return nil
	}

	// If for some reason the post-start hook didn't run, run cleanup here
	klog.V(2).Info("MigrationCleanupController: running cleanup via Sync")
	return c.runCleanupOnce(ctx, controllerContext)
}

func (c *MigrationCleanupController) cleanupConversionWebhookResources(ctx context.Context) error {
	var errs []error
	var deletedResources []string

	// Delete Deployment
	err := c.kubeClient.AppsV1().Deployments(api.OpenShiftConsoleOperatorNamespace).Delete(
		ctx, ConversionWebhookDeploymentName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to delete deployment %s: %w", ConversionWebhookDeploymentName, err))
	} else if err == nil {
		klog.V(4).Infof("Deleted deployment: %s/%s", api.OpenShiftConsoleOperatorNamespace, ConversionWebhookDeploymentName)
		deletedResources = append(deletedResources, "deployment/"+ConversionWebhookDeploymentName)
	}

	// Delete Service
	err = c.kubeClient.CoreV1().Services(api.OpenShiftConsoleOperatorNamespace).Delete(
		ctx, ConversionWebhookServiceName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to delete service %s: %w", ConversionWebhookServiceName, err))
	} else if err == nil {
		klog.V(4).Infof("Deleted service: %s/%s", api.OpenShiftConsoleOperatorNamespace, ConversionWebhookServiceName)
		deletedResources = append(deletedResources, "service/"+ConversionWebhookServiceName)
	}

	// Delete Secret
	err = c.kubeClient.CoreV1().Secrets(api.OpenShiftConsoleOperatorNamespace).Delete(
		ctx, ConversionWebhookSecretName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to delete secret %s: %w", ConversionWebhookSecretName, err))
	} else if err == nil {
		klog.V(4).Infof("Deleted secret: %s/%s", api.OpenShiftConsoleOperatorNamespace, ConversionWebhookSecretName)
		deletedResources = append(deletedResources, "secret/"+ConversionWebhookSecretName)
	}

	// Log summary and emit event if any resources were actually deleted
	if len(deletedResources) > 0 {
		klog.Infof("MigrationCleanupController: successfully deleted console-conversion-webhook resources from namespace %s: %v", api.OpenShiftConsoleOperatorNamespace, deletedResources)
		c.recorder.Eventf("MigrationCleanupCompleted", "Successfully cleaned up console-conversion-webhook resources from namespace %s: %v", api.OpenShiftConsoleOperatorNamespace, deletedResources)
	} else {
		klog.V(4).Infof("MigrationCleanupController: no console-conversion-webhook resources found to clean up in namespace %s", api.OpenShiftConsoleOperatorNamespace)
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}

	return nil
}
