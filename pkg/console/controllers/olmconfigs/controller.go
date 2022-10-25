package olmconfigs

import (
	"context"
	"fmt"
	"time"

	// k8s

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// policyv1 "k8s.io/client-go/informers/policy/v1"
	// policyv1client "k8s.io/client-go/kubernetes/typed/policy/v1"
	"k8s.io/klog/v2"

	// // informers
	// olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"

	// openshift
	operatorsv1 "github.com/openshift/api/operator/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	"github.com/openshift/console-operator/pkg/console/status"
	configmapsub "github.com/openshift/console-operator/pkg/console/subresource/configmap"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	// console-operator
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
)

type OLMConfigController struct {
	configName           string
	operatorClient       v1helpers.OperatorClient
	operatorConfigClient operatorclientv1.ConsoleInterface
	olmConfig            olmclient.OLMConfig
}

func NewOLMConfigController(
	// name of the olm instance
	configName string,
	// clients
	operatorClient v1helpers.OperatorClient,
	operatorConfigClient operatorclientv1.ConsoleInterface,
	// informer
	olmConfig olmclient.OLMConfig,
	//events
	recorder events.Recorder,
) factory.Controller {

	ctrl := &OLMConfigController{
		configName:           configName,
		operatorClient:       operatorClient,
		operatorConfigClient: operatorConfigClient,
		olmConfig:            olmConfig,
	}

	return factory.New().
		WithFilteredEventsInformers(
			util.IncludeNamesFilter(configName),
			olmConfig.Informer(),
		).ResyncEvery(time.Minute).WithSync(ctrl.Sync).
		ToController("OLMConfigController", recorder.WithComponentSuffix(fmt.Sprintf("%s-olm-controller", configName)))
}

func (c *OLMConfigController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.operatorConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	updatedOperatorConfig := operatorConfig.DeepCopy()

	switch updatedOperatorConfig.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infof("console-operator is in a managed state: syncing %q olm", c.configName)
	case operatorsv1.Unmanaged:
		klog.V(4).Infof("console-operator is in an unmanaged state: skipping olm %q sync", c.configName)
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infof("console-operator is in a removed state: skipping %q olm", c.configName)
		return nil
	default:
		return fmt.Errorf("unknown state: %v", updatedOperatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	olmConfig, olmErr := c.olmConfig.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("OlmSync", "FailedOLMConfigGet", olmErr))
	if olmErr != nil {
		return statusHandler.FlushAndReturn(olmErr)
	}

	if !olmConfig.CopiedCSVsAreEnabled() {
		_, updateConfigErr := c.AddDisableCopiedCSVsToConsoleConfig(ctx, updatedOperatorConfig)
		if updateConfigErr != nil {
			return statusHandler.FlushAndReturn(updateConfigErr)
		}
	}
	return statusHandler.FlushAndReturn(olmErr)
}

func (c *OLMConfigController) AddDisableCopiedCSVsToConsoleConfig(ctx context.Context, operatorConfig *operatorsv1.Console) (string, error) {
	//Update ConsoleConfig
	clusterInfo := ClusterInfo{
		CopiedCSVsDisabled: "true",
	}

	defaultConfigmap, _, err := configmapsub.DefaultConfigMap(
		operatorConfig,
		clusterInfo,
	)
	if err != nil {
		return nil, false, "FailedConsoleConfigBuilder", err
	}
	cm, cmChanged, cmErr := resourceapply.ApplyConfigMap(ctx, co.configMapClient, recorder, defaultConfigmap)
	if cmErr != nil {
		return nil, false, "FailedApply", cmErr
	}
	if cmChanged {
		klog.V(4).Infoln("new console config yaml:")
		klog.V(4).Infof("%s", cm.Data)
	}
	return cm, cmChanged, "ConsoleConfigBuilder", cmErr
	return "", nil
}
