package storageversionmigration

import (
	"context"
	"errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/klog/v2"

	"github.com/openshift/console-operator/pkg/console/status"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	storageVersionMigrationName = "console-plugin-storage-version-migration"
	consolePluginCRDName        = "consoleplugins.console.openshift.io"
	maxRetries                  = 5
	retryDelay                  = 2 * time.Second
)

var (
	storageVersionMigrationGVR = schema.GroupVersionResource{
		Group:    "migration.k8s.io",
		Version:  "v1alpha1",
		Resource: "storageversionmigrations",
	}
	crdGVR = schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
)

type StorageVersionMigrationController struct {
	dynamicClient  dynamic.Interface
	operatorClient v1helpers.OperatorClient
}

func NewStorageVersionMigrationController(
	operatorClient v1helpers.OperatorClient,
	dynamicClient dynamic.Interface,
	dynamicInformers dynamicinformer.DynamicSharedInformerFactory,
	recorder events.Recorder,
) factory.Controller {
	c := &StorageVersionMigrationController{
		dynamicClient:  dynamicClient,
		operatorClient: operatorClient,
	}

	return factory.New().
		WithInformers(
			dynamicInformers.ForResource(storageVersionMigrationGVR).Informer(),
		).
		ResyncEvery(10*time.Minute).
		WithSync(c.sync).
		ToController("StorageVersionMigrationController", recorder)
}

func (c *StorageVersionMigrationController) sync(ctx context.Context, syncContext factory.SyncContext) error {
	statusHandler := status.NewStatusHandler(c.operatorClient)

	reason, err := c.syncStorageVersionMigration(ctx)
	statusHandler.AddCondition(status.HandleDegraded("StorageVersionMigration", reason, err))
	return statusHandler.FlushAndReturn(err)
}

func (c *StorageVersionMigrationController) syncStorageVersionMigration(ctx context.Context) (string, error) {
	// Check if the ConsolePlugin CRD still has v1alpha1 in storedVersions
	hasV1Alpha1, err := c.checkCRDStoredVersions(ctx)
	if err != nil {
		klog.Errorf("Failed to check CRD storedVersions: %v", err)
		return "FailedCheckCRDStoredVersions", err
	}

	if !hasV1Alpha1 {
		klog.V(4).Infof("ConsolePlugin CRD does not have v1alpha1 in storedVersions, migration already complete")
		return "", nil
	}

	// Get the StorageVersionMigration instance
	svm, err := c.dynamicClient.Resource(storageVersionMigrationGVR).Get(ctx, storageVersionMigrationName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get StorageVersionMigration: %v", err)
		return "FailedGetSVM", err
	}

	// Check if the migration has succeeded with retry logic
	succeeded, err := c.hasSucceededConditionWithRetry(svm)
	if err != nil {
		klog.Errorf("Failed to check conditions for StorageVersionMigration after %d retries: %v", maxRetries, err)
		return "FailedCheckSVMConditions", err
	}

	if !succeeded {
		klog.V(4).Infof("StorageVersionMigration has not succeeded yet")
		// Delete the StorageVersionMigration if it has not succeeded yet
		if err := c.deleteStorageVersionMigration(ctx); err != nil {
			return "FailedDeleteSVM", err
		}
		return "", nil
	}

	klog.Infof("StorageVersionMigration has succeeded, setting ConsolePlugin CRD storedVersions to v1")

	// Set CRD storedVersions to v1
	if err := c.removeV1Alpha1FromCRD(ctx); err != nil {
		return "FailedPatchCRD", err
	}

	return "", nil
}

// checkCRDStoredVersions checks if the ConsolePlugin CRD has v1alpha1 in its storedVersions
func (c *StorageVersionMigrationController) checkCRDStoredVersions(ctx context.Context) (bool, error) {
	// Get the ConsolePlugin CRD
	crd, err := c.dynamicClient.Resource(crdGVR).Get(ctx, consolePluginCRDName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get ConsolePlugin CRD: %v", err)
		return false, err
	}

	// Get storedVersions from the CRD status
	storedVersions, found, err := unstructured.NestedStringSlice(crd.Object, "status", "storedVersions")
	if err != nil {
		klog.Errorf("Failed to get storedVersions for ConsolePlugin CRD: %v", err)
		return false, err
	}
	if !found {
		klog.V(4).Infof("No storedVersions found for ConsolePlugin CRD")
		return false, nil
	}

	// Check if v1alpha1 is present in storedVersions
	for _, version := range storedVersions {
		if version == "v1alpha1" {
			klog.V(4).Infof("Found v1alpha1 in ConsolePlugin CRD storedVersions: %v", storedVersions)
			return true, nil
		}
	}

	klog.V(4).Infof("v1alpha1 not found in ConsolePlugin CRD storedVersions: %v", storedVersions)
	return false, nil
}

// hasSucceededConditionWithRetry checks if the StorageVersionMigration has succeeded with retry logic
func (c *StorageVersionMigrationController) hasSucceededConditionWithRetry(svm *unstructured.Unstructured) (bool, error) {
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		succeeded, err := c.hasSucceededCondition(svm)
		if err == nil {
			return succeeded, nil
		}

		lastErr = err
		klog.Warningf("Attempt %d/%d failed to check StorageVersionMigration conditions: %v", attempt, maxRetries, err)

		if attempt < maxRetries {
			klog.V(4).Infof("Retrying in %v...", retryDelay)
			time.Sleep(retryDelay)
		}
	}

	return false, lastErr
}

// hasSucceededCondition checks if the StorageVersionMigration has a 'Succeeded' condition with status 'True'
func (c *StorageVersionMigrationController) hasSucceededCondition(svm *unstructured.Unstructured) (bool, error) {
	conditions, found, err := unstructured.NestedSlice(svm.Object, "status", "conditions")
	if err != nil {
		return false, err
	}
	if !found {
		return false, errors.New("conditions not found")
	}

	for _, condition := range conditions {
		conditionMap, ok := condition.(map[string]interface{})
		if !ok {
			continue
		}

		conditionType, typeFound := conditionMap["type"].(string)
		conditionStatus, statusFound := conditionMap["status"].(string)

		if typeFound && statusFound && conditionType == "Succeeded" && conditionStatus == "True" {
			return true, nil
		}
	}

	return false, nil
}

// removeV1Alpha1FromCRD removes 'v1alpha1' from the ConsolePlugin CRD's storedVersions
func (c *StorageVersionMigrationController) removeV1Alpha1FromCRD(ctx context.Context) error {
	// Get the ConsolePlugin CRD
	crd, err := c.dynamicClient.Resource(crdGVR).Get(ctx, consolePluginCRDName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get ConsolePlugin CRD: %v", err)
		return err
	}

	// Verify CRD exists and has status
	_, found, err := unstructured.NestedMap(crd.Object, "status")
	if err != nil {
		klog.Errorf("Failed to get status for ConsolePlugin CRD: %v", err)
		return err
	}
	if !found {
		klog.Info("No status found for ConsolePlugin CRD")
		return nil
	}

	// Set storedVersions to only contain v1
	newStoredVersions := []string{"v1"}

	// Create and apply patch
	return c.patchCRDStoredVersions(ctx, newStoredVersions)
}

// patchCRDStoredVersions applies a patch to update the CRD's storedVersions
func (c *StorageVersionMigrationController) patchCRDStoredVersions(ctx context.Context, newStoredVersions []string) error {
	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"storedVersions": newStoredVersions,
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		klog.Errorf("Failed to marshal patch for ConsolePlugin CRD: %v", err)
		return err
	}

	// Apply the patch to the status subresource
	_, err = c.dynamicClient.Resource(crdGVR).Patch(ctx, consolePluginCRDName, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		klog.Errorf("Failed to patch ConsolePlugin CRD status: %v", err)
		return err
	}

	klog.Infof("Successfully set ConsolePlugin CRD storedVersions to v1")
	return nil
}

// deleteStorageVersionMigration deletes the StorageVersionMigration resource
func (c *StorageVersionMigrationController) deleteStorageVersionMigration(ctx context.Context) error {
	klog.Infof("Deleting StorageVersionMigration")
	err := c.dynamicClient.Resource(storageVersionMigrationGVR).Delete(ctx, storageVersionMigrationName, metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("Failed to delete StorageVersionMigration: %v", err)
		return err
	}
	return nil
}
