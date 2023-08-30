package upgradenotification

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	v1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	consoleclientv1 "github.com/openshift/client-go/console/clientset/versioned/typed/console/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

// ctrl just needs the clients so it can make requests
// the informers will automatically notify it of changes
// and kick the sync loop
type UpgradeNotificationController struct {
	operatorClient       v1helpers.OperatorClient
	operatorConfigClient operatorclientv1.ConsoleInterface

	consoleNotificationClient consoleclientv1.ConsoleNotificationInterface

	// lister
	clusterVersionLister configlistersv1.ClusterVersionLister
}

// factory func needs clients and informers
// informers to start them up, clients to pass
func NewUpgradeNotificationController(
	// top level config
	configClient configclientv1.ConfigV1Interface,
	configInformer configinformer.SharedInformerFactory,
	// clients
	operatorClient v1helpers.OperatorClient,
	operatorConfigClient operatorclientv1.ConsoleInterface,
	consoleNotificationClient consoleclientv1.ConsoleNotificationInterface,

	recorder events.Recorder,
) factory.Controller {

	ctrl := &UpgradeNotificationController{
		operatorClient:            operatorClient,
		operatorConfigClient:      operatorConfigClient,
		consoleNotificationClient: consoleNotificationClient,
		clusterVersionLister:      configInformer.Config().V1().ClusterVersions().Lister(),
	}

	configV1Informers := configInformer.Config().V1()

	return factory.New().
		WithFilteredEventsInformers( // configs
			util.IncludeNamesFilter(api.VersionResourceName),
			configV1Informers.ClusterVersions().Informer(),
		).ResyncEvery(time.Minute).WithSync(ctrl.Sync).
		ToController("ClusterUpgradeNotificationController", recorder.WithComponentSuffix("cluster-upgrade-notification-controller"))
}

func (c *UpgradeNotificationController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.operatorConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	updatedOperatorConfig := operatorConfig.DeepCopy()

	switch updatedOperatorConfig.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Info("console-operator is in a managed state: syncing upgrade notification")
	case operatorsv1.Unmanaged:
		klog.V(4).Info("console-operator is in an unmanaged state: skipping upgrade notification sync")
		return nil
	case operatorsv1.Removed:
		klog.V(4).Info("console-operator is in a removed state: deleting upgrade notification")
		return c.removeUpgradeNotification(ctx)
	default:
		return fmt.Errorf("unknown state: %v", updatedOperatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	reason, err := c.syncClusterUpgradeNotification(ctx)
	if err != nil {
		klog.V(4).Infof("error syncing %s consolenotification custom resource: %s", api.UpgradeConsoleNotification, err)
	}
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ConsoleNotificationSync", reason, err))
	return statusHandler.FlushAndReturn(err)
}

func (c *UpgradeNotificationController) syncClusterUpgradeNotification(ctx context.Context) (string, error) {
	clusterVersionConfig, err := c.clusterVersionLister.Get(api.VersionResourceName)
	if err != nil {
		return "FailedGetClusterVersion", err
	}

	isUpdateProgressing := getClusterVersionCondition(*clusterVersionConfig, v1.ConditionTrue, v1.OperatorProgressing)

	if isUpdateProgressing {
		var currentVersion string
		for _, version := range clusterVersionConfig.Status.History {
			if version.State == v1.CompletedUpdate {
				currentVersion = version.Version
				break
			}
		}
		desiredVersion := clusterVersionConfig.Status.Desired.Version

		if currentVersion == "" || desiredVersion == "" || currentVersion == desiredVersion {
			return "", nil
		}

		notification := &consolev1.ConsoleNotification{
			ObjectMeta: metav1.ObjectMeta{
				Name: api.UpgradeConsoleNotification,
			},
			Spec: consolev1.ConsoleNotificationSpec{
				Text:            fmt.Sprintf("This cluster is updating from %s to %s", currentVersion, desiredVersion),
				Location:        "BannerTop",
				Color:           "#000000",
				BackgroundColor: "#F0AB00",
			},
		}
		_, err = c.consoleNotificationClient.Create(ctx, notification, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return "FailedCreate", err
		}
	} else {
		err = c.removeUpgradeNotification(ctx)
		if err != nil {
			return "FailedDelete", err
		}
	}

	return "", nil
}

func (c *UpgradeNotificationController) removeUpgradeNotification(ctx context.Context) error {
	err := c.consoleNotificationClient.Delete(ctx, api.UpgradeConsoleNotification, metav1.DeleteOptions{})
	if !apierrors.IsNotFound(err) {
		klog.V(4).Infof("error deleting %s consolenotification custom resource: %s", api.UpgradeConsoleNotification, err)
		return err
	}
	return nil
}

func getClusterVersionCondition(cvo v1.ClusterVersion, conditionStatus v1.ConditionStatus, conditionType v1.ClusterStatusConditionType) bool {
	isConditionFulfilled := false
	for _, condition := range cvo.Status.Conditions {
		if condition.Status == conditionStatus && condition.Type == conditionType {
			isConditionFulfilled = true
			break
		}
	}

	return isConditionFulfilled
}
