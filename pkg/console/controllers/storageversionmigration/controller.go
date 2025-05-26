package storageversionmigration

import (
	"context"
	"slices"

	storagemigrationv1alpha1 "k8s.io/api/storagemigration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	resourceName                = "storageversionmigrations"
)

var (
	storageVersionMigrationGVR = storagemigrationv1alpha1.SchemeGroupVersion.WithResource(resourceName)
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
		WithSync(c.sync).
		ToController("StorageVersionMigrationController", recorder)
}

func (c *StorageVersionMigrationController) sync(ctx context.Context, syncContext factory.SyncContext) error {
	statusHandler := status.NewStatusHandler(c.operatorClient)

	// Get the StorageVersionMigration instance
	svm, err := c.dynamicClient.Resource(storageVersionMigrationGVR).Get(ctx, storageVersionMigrationName, metav1.GetOptions{})
	if err != nil {
		// Error reading the object - requeue the request.
		klog.Errorf("Failed to get StorageVersionMigration: %v", err)
		return statusHandler.FlushAndReturn(err)
	}

	// Check if status.storedVersions contains v1alpha1
	storedVersions, found, err := unstructured.NestedStringSlice(svm.Object, "status", "storedVersions")
	if err != nil {
		klog.Errorf("Failed to get storedVersions for the StorageVersionMigration: %v", err)
		return statusHandler.FlushAndReturn(err)
	}
	if !found {
		klog.Errorf("Failed to get storedVersions for the StorageVersionMigration")
		return statusHandler.FlushAndReturn(nil)
	}

	if slices.Contains(storedVersions, "v1alpha1") {
		klog.Infof("Found v1alpha1 in storedVersions, deleting StorageVersionMigration")
		err = c.dynamicClient.Resource(storageVersionMigrationGVR).Delete(ctx, storageVersionMigrationName, metav1.DeleteOptions{})
		statusHandler.AddCondition(status.HandleDegraded("StorageVersionMigration", "FailedDelete", err))
		if err != nil {
			klog.Errorf("Failed to delete StorageVersionMigration: %v", err)
			return statusHandler.FlushAndReturn(err)
		}
	}

	return statusHandler.FlushAndReturn(nil)
}
