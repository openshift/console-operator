package operator

import (
	"context"
	"strings"

	// kube
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"

	// operator
	"github.com/openshift/console-operator/pkg/api"
	configmapsub "github.com/openshift/console-operator/pkg/console/subresource/configmap"
	oauthsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	utilsub "github.com/openshift/console-operator/pkg/console/subresource/util"
	telemetry "github.com/openshift/console-operator/pkg/console/telemetry"
)

// apply configmap (needs route)
// by the time we get to the configmap, we can assume the route exits & is configured properly
// therefore no additional error handling is needed here.
func (co *consoleOperator) SyncConfigMap(
	ctx context.Context,
	operatorConfig *operatorv1.Console,
	consoleConfig *configv1.Console,
	infrastructureConfig *configv1.Infrastructure,
	oauthConfig *configv1.OAuth,
	authServerCAConfig *corev1.ConfigMap,
	authConfig *configv1.Authentication,
	activeConsoleRoute *routev1.Route,
	recorder events.Recorder,
	consoleHost string,
) (consoleConfigMap *corev1.ConfigMap, changed bool, reason string, err error) {

	managedConfig, mcErr := co.managedNSConfigMapLister.ConfigMaps(api.OpenShiftConfigManagedNamespace).Get(api.OpenShiftConsoleConfigMapName)
	if mcErr != nil {
		if !apierrors.IsNotFound(mcErr) {
			return nil, false, "FailedGetManagedConfig", mcErr
		}
		managedConfig = &corev1.ConfigMap{}
	}
	nodeList, nodeListErr := co.nodeLister.List(labels.Everything())
	if nodeListErr != nil {
		return nil, false, "FailedListNodes", nodeListErr
	}
	nodeArchitectures, nodeOperatingSystems := getNodeComputeEnvironments(nodeList)

	// TODO: currently there's no way to get this for authentication type OIDC
	inactivityTimeoutSeconds := 0
	switch authConfig.Spec.Type {
	case "", configv1.AuthenticationTypeIntegratedOAuth:
		oauthClient, oacErr := co.oauthClientLister.Get(oauthsub.Stub().Name)
		if oacErr != nil {
			return nil, false, "FailedGetOAuthClient", oacErr
		}
		if oauthClient.AccessTokenInactivityTimeoutSeconds != nil {
			inactivityTimeoutSeconds = int(*oauthClient.AccessTokenInactivityTimeoutSeconds)
		} else {
			if oauthConfig.Spec.TokenConfig.AccessTokenInactivityTimeout != nil {
				inactivityTimeoutSeconds = int(oauthConfig.Spec.TokenConfig.AccessTokenInactivityTimeout.Seconds())
			}
		}
	}

	availablePlugins := co.getAvailablePlugins(operatorConfig.Spec.Plugins)

	monitoringSharedConfig, mscErr := co.managedNSConfigMapLister.ConfigMaps(api.OpenShiftConfigManagedNamespace).Get(api.OpenShiftMonitoringConfigMapName)
	if mscErr != nil {
		if !apierrors.IsNotFound(mscErr) {
			return nil, false, "FailedGetMonitoringSharedConfig", mscErr
		}
		monitoringSharedConfig = &corev1.ConfigMap{}
	}

	telemetryConfig, tcErr := co.getTelemetryConfiguration(operatorConfig)
	if tcErr != nil {
		return nil, false, "FailedGetTelemetryConfig", tcErr
	}

	var (
		copiedCSVsDisabled bool
		ccdErr             error
	)
	if !co.trackables.isOLMDisabled {
		copiedCSVsDisabled, ccdErr = co.isCopiedCSVsDisabled(ctx)
		if ccdErr != nil {
			return nil, false, "FailedGetOLMConfig", ccdErr
		}
	}

	defaultConfigmap, _, err := configmapsub.DefaultConfigMap(
		operatorConfig,
		consoleConfig,
		authConfig,
		authServerCAConfig,
		managedConfig,
		monitoringSharedConfig,
		infrastructureConfig,
		activeConsoleRoute,
		inactivityTimeoutSeconds,
		availablePlugins,
		nodeArchitectures,
		nodeOperatingSystems,
		copiedCSVsDisabled,
		co.contentSecurityPolicyEnabled,
		telemetryConfig,
		consoleHost,
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
}

// Build telemetry configuration in following order:
//  1. check if the telemetry client is available and set the "TELEMETER_CLIENT_DISABLED" annotation accordingly
//  2. get telemetry annotation from console-operator config
//  3. get default telemetry value from telemetry-config configmap
//  4. get CLUSTER_ID from the cluster-version config
//  5. get ORGANIZATION_ID from OCM, if ORGANIZATION_ID is not already set
func (co *consoleOperator) getTelemetryConfiguration(operatorConfig *operatorv1.Console) (map[string]string, error) {
	telemetryConfig := make(map[string]string)

	if len(operatorConfig.Annotations) > 0 {
		for k, v := range operatorConfig.Annotations {
			if strings.HasPrefix(k, telemetry.TelemetryAnnotationPrefix) && len(k) > len(telemetry.TelemetryAnnotationPrefix) {
				telemetryConfig[k[len(telemetry.TelemetryAnnotationPrefix):]] = v
			}
		}
	}

	telemetryConfigMap, err := co.operatorNSConfigMapLister.ConfigMaps(api.OpenShiftConsoleOperatorNamespace).Get(telemetry.TelemetryConfigMapName)
	if err != nil {
		return telemetryConfig, err
	}

	if len(telemetryConfigMap.Data) > 0 {
		for k, v := range telemetryConfigMap.Data {
			telemetryConfig[k] = v
		}
	}

	clusterID, err := telemetry.GetClusterID(co.clusterVersionLister)
	if err != nil {
		return nil, err
	}
	telemetryConfig["CLUSTER_ID"] = clusterID

	telemeterClientIsAvailable, err := telemetry.IsTelemeterClientAvailable(co.monitoringDeploymentLister)
	if err != nil {
		return telemetryConfig, err
	}
	if !telemeterClientIsAvailable {
		telemetryConfig["TELEMETER_CLIENT_DISABLED"] = "true"
		return telemetryConfig, nil
	}

	accessToken, err := telemetry.GetAccessToken(co.configNSSecretLister)
	if err != nil {
		return nil, err
	}
	organizationID, refreshCache := telemetry.GetOrganizationID(telemetryConfig, co.trackables.organizationID, clusterID, accessToken)
	// cache fetched ORGANIZATION_ID
	if refreshCache {
		co.trackables.organizationID = organizationID
	}
	telemetryConfig["ORGANIZATION_ID"] = organizationID

	return telemetryConfig, nil
}

func (co *consoleOperator) getAvailablePlugins(enabledPluginsNames []string) []*consolev1.ConsolePlugin {
	var availablePlugins []*consolev1.ConsolePlugin
	for _, pluginName := range utilsub.RemoveDuplicateStr(enabledPluginsNames) {
		plugin, err := co.consolePluginLister.Get(pluginName)
		if err != nil {
			klog.Errorf("failed to get %q plugin: %v", pluginName, err)
			continue
		}
		availablePlugins = append(availablePlugins, plugin)
	}
	return availablePlugins
}

func (co *consoleOperator) isCopiedCSVsDisabled(ctx context.Context) (bool, error) {
	olmConfig, err := co.dynamicClient.Resource(schema.GroupVersionResource{Group: api.OLMConfigGroup, Version: api.OLMConfigVersion, Resource: api.OLMConfigResource}).Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	copiedCSVsDisabled, found, err := unstructured.NestedBool(olmConfig.Object, "spec", "features", "disableCopiedCSVs")
	if err != nil || !found {
		return false, err
	}

	return copiedCSVsDisabled, nil
}
