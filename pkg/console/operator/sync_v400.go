package operator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	// kube
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/api/console/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"

	// operator
	"github.com/openshift/console-operator/pkg/api"
	customerrors "github.com/openshift/console-operator/pkg/console/errors"
	"github.com/openshift/console-operator/pkg/console/metrics"
	"github.com/openshift/console-operator/pkg/console/status"
	configmapsub "github.com/openshift/console-operator/pkg/console/subresource/configmap"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
	oauthsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"
	utilsub "github.com/openshift/console-operator/pkg/console/subresource/util"
	telemetry "github.com/openshift/console-operator/pkg/console/telemetry"
)

// The sync loop starts from zero and works its way through the requirements for a running console.
// If at any point something is missing, it creates/updates that piece and immediately dies.
// The next loop will pick up where they previous left off and move the process forward one step.
// This ensures the logic is simpler as we do not have to handle coordination between objects within
// the loop.
func (co *consoleOperator) sync_v400(ctx context.Context, controllerContext factory.SyncContext, updatedOperatorConfig *operatorv1.Console, set configSet) error {
	klog.V(4).Infoln("running sync loop 4.0.0")
	statusHandler := status.NewStatusHandler(co.operatorClient)

	// track changes, may trigger ripples & update operator config or console config status
	toUpdate := false

	routeName := api.OpenShiftConsoleRouteName
	routeConfig := routesub.NewRouteConfig(updatedOperatorConfig, set.Ingress, routeName)
	if routeConfig.IsCustomHostnameSet() {
		routeName = api.OpenshiftConsoleCustomRouteName
	}

	route, consoleURL, routeReasoneErr, routeErr := routesub.GetActiveRouteInfo(co.routeLister, routeName)
	// TODO: this controller is no longer responsible for syncing the route.
	//   however, the route is essential for several of the components below.
	//   - the loop should exit early and wait until the RouteSyncController creates the route.
	//     there is nothing new in this flow, other than 2 controllers now look
	//     at the same resource.
	//     - RouteSyncController is responsible for updates
	//     - ConsoleOperatorController (future ConsoleDeploymentController) is responsible for reads only.
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("SyncLoopRefresh", routeReasoneErr, routeErr))
	if routeErr != nil {
		return statusHandler.FlushAndReturn(routeErr)
	}

	authnConfig, err := co.authnConfigLister.Get(api.ConfigResourceName)
	if err != nil {
		return statusHandler.FlushAndReturn(err)
	}

	var (
		authServerCAConfig *corev1.ConfigMap
		sessionSecret      *corev1.Secret
	)
	switch authnConfig.Spec.Type {
	case configv1.AuthenticationTypeOIDC:
		if len(authnConfig.Spec.OIDCProviders) > 0 {
			oidcProvider := authnConfig.Spec.OIDCProviders[0]
			authServerCAConfig, err = co.configNSConfigMapLister.ConfigMaps(api.OpenShiftConfigNamespace).Get(oidcProvider.Issuer.CertificateAuthority.Name)
			if err != nil && !apierrors.IsNotFound(err) {
				return statusHandler.FlushAndReturn(err)
			}
		}

		sessionSecret, err = co.syncSessionSecret(ctx, updatedOperatorConfig, controllerContext.Recorder())
		if err != nil {
			return statusHandler.FlushAndReturn(err)
		}
	}

	cm, cmChanged, cmErrReason, cmErr := co.SyncConfigMap(
		ctx,
		set.Operator,
		set.Console,
		set.Infrastructure,
		set.OAuth,
		authServerCAConfig,
		authnConfig,
		route,
		controllerContext.Recorder(),
	)
	toUpdate = toUpdate || cmChanged
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ConfigMapSync", cmErrReason, cmErr))
	if cmErr != nil {
		return statusHandler.FlushAndReturn(cmErr)
	}

	serviceCAConfigMap, serviceCAChanged, serviceCAErrReason, serviceCAErr := co.SyncServiceCAConfigMap(ctx, set.Operator)
	toUpdate = toUpdate || serviceCAChanged
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ServiceCASync", serviceCAErrReason, serviceCAErr))
	if serviceCAErr != nil {
		return statusHandler.FlushAndReturn(serviceCAErr)
	}

	trustedCAConfigMap, trustedCAConfigMapChanged, trustedCAErrReason, trustedCAErr := co.SyncTrustedCAConfigMap(ctx, set.Operator)
	toUpdate = toUpdate || trustedCAConfigMapChanged
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("TrustedCASync", trustedCAErrReason, trustedCAErr))
	if trustedCAErr != nil {
		return statusHandler.FlushAndReturn(trustedCAErr)
	}

	// TODO: why is this missing a toUpdate change?
	customLogoCanMount, customLogoErrReason, customLogoError := co.SyncCustomLogoConfigMap(ctx, updatedOperatorConfig)
	// If the custom logo sync fails for any reason, we are degraded, not progressing.
	// The sync loop may not settle, we are unable to honor it in current state.
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("CustomLogoSync", customLogoErrReason, customLogoError))
	if customLogoError != nil {
		return statusHandler.FlushAndReturn(customLogoError)
	}

	var oauthServingCertConfigMap *corev1.ConfigMap
	switch authnConfig.Spec.Type {
	case "", configv1.AuthenticationTypeIntegratedOAuth:
		var oauthServingCertErrReason string
		var oauthServingCertErr error

		oauthServingCertConfigMap, oauthServingCertErrReason, oauthServingCertErr = co.ValidateOAuthServingCertConfigMap(ctx)
		statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthServingCertValidation", oauthServingCertErrReason, oauthServingCertErr))
		if oauthServingCertErr != nil {
			return statusHandler.FlushAndReturn(oauthServingCertErr)
		}
	}

	clientSecret, secErr := co.secretsLister.Secrets(api.TargetNamespace).Get(secretsub.Stub().Name)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSecretGet", "FailedGet", secErr))
	if secErr != nil {
		return statusHandler.FlushAndReturn(secErr)
	}

	actualDeployment, depChanged, depErrReason, depErr := co.SyncDeployment(
		ctx,
		set.Operator,
		cm,
		serviceCAConfigMap,
		oauthServingCertConfigMap,
		authServerCAConfig,
		trustedCAConfigMap,
		clientSecret,
		sessionSecret,
		set.Proxy,
		set.Infrastructure,
		customLogoCanMount,
		controllerContext.Recorder(),
	)
	toUpdate = toUpdate || depChanged
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("DeploymentSync", depErrReason, depErr))
	if depErr != nil {
		return statusHandler.FlushAndReturn(depErr)
	}

	statusHandler.UpdateDeploymentGeneration(actualDeployment)
	statusHandler.UpdateReadyReplicas(actualDeployment.Status.ReadyReplicas)
	statusHandler.UpdateObservedGeneration(set.Operator.ObjectMeta.Generation)

	klog.V(4).Infoln("-----------------------")
	klog.V(4).Infof("sync loop 4.0.0 resources updated: %v", toUpdate)
	klog.V(4).Infoln("-----------------------")

	statusHandler.AddCondition(status.HandleProgressing("SyncLoopRefresh", "InProgress", func() error {
		if toUpdate {
			return errors.New("changes made during sync updates, additional sync expected")
		}
		version := os.Getenv("OPERATOR_IMAGE_VERSION")
		if !deploymentsub.IsAvailableAndUpdated(actualDeployment) {
			return fmt.Errorf("working toward version %s, %v replicas available", version, actualDeployment.Status.AvailableReplicas)
		}

		if co.versionGetter.GetVersions()["operator"] != version {
			co.versionGetter.SetVersion("operator", version)
		}
		return nil
	}()))

	statusHandler.AddCondition(status.HandleAvailable(func() (prefix string, reason string, err error) {
		prefix = "Deployment"
		if !deploymentsub.IsAvailable(actualDeployment) {
			return prefix, "InsufficientReplicas", fmt.Errorf("%v replicas available for console deployment", actualDeployment.Status.ReadyReplicas)
		}
		return prefix, "", nil
	}()))

	// if we survive the gauntlet, we need to update the console config with the
	// public hostname so that the world can know the console is ready to roll
	klog.V(4).Infoln("sync_v400: updating console status")

	_, consoleConfigErr := co.SyncConsoleConfig(ctx, set.Console, consoleURL.String())
	statusHandler.AddCondition(status.HandleDegraded("ConsoleConfig", "FailedUpdate", consoleConfigErr))
	if consoleConfigErr != nil {
		klog.Errorf("could not update console config status: %v", consoleConfigErr)
		return statusHandler.FlushAndReturn(consoleConfigErr)
	}

	_, _, consolePublicConfigErr := co.SyncConsolePublicConfig(ctx, consoleURL.String(), controllerContext.Recorder())
	statusHandler.AddCondition(status.HandleDegraded("ConsolePublicConfigMap", "FailedApply", consolePublicConfigErr))
	if consolePublicConfigErr != nil {
		klog.Errorf("could not update public console config status: %v", consolePublicConfigErr)
		return statusHandler.FlushAndReturn(consolePublicConfigErr)
	}

	defer func() {
		klog.V(4).Infof("sync loop 4.0.0 complete")

		if cmChanged {
			klog.V(4).Infof("\t configmap changed: %v", cm.GetResourceVersion())
		}
		if serviceCAChanged {
			klog.V(4).Infof("\t service-ca configmap changed: %v", serviceCAConfigMap.GetResourceVersion())
		}
		if depChanged {
			klog.V(4).Infof("\t deployment changed: %v", actualDeployment.GetResourceVersion())
		}
	}()

	return statusHandler.FlushAndReturn(nil)
}

func (co *consoleOperator) SyncConsoleConfig(ctx context.Context, consoleConfig *configv1.Console, consoleURL string) (*configv1.Console, error) {
	oldURL := consoleConfig.Status.ConsoleURL
	metrics.HandleConsoleURL(oldURL, consoleURL)
	if oldURL != consoleURL {
		klog.V(4).Infof("updating console.config.openshift.io with url: %v", consoleURL)
		updated := consoleConfig.DeepCopy()
		updated.Status.ConsoleURL = consoleURL
		return co.consoleConfigClient.UpdateStatus(ctx, updated, metav1.UpdateOptions{})
	}
	return consoleConfig, nil
}

func (co *consoleOperator) SyncConsolePublicConfig(ctx context.Context, consoleURL string, recorder events.Recorder) (*corev1.ConfigMap, bool, error) {
	requiredConfigMap := configmapsub.DefaultPublicConfig(consoleURL)
	return resourceapply.ApplyConfigMap(ctx, co.configMapClient, recorder, requiredConfigMap)
}

func (co *consoleOperator) SyncDeployment(
	ctx context.Context,
	operatorConfig *operatorv1.Console,
	cm *corev1.ConfigMap,
	serviceCAConfigMap *corev1.ConfigMap,
	oauthServingCertConfigMap *corev1.ConfigMap,
	authServerCAConfigMap *corev1.ConfigMap,
	trustedCAConfigMap *corev1.ConfigMap,
	sec *corev1.Secret,
	sessionSecret *corev1.Secret,
	proxyConfig *configv1.Proxy,
	infrastructureConfig *configv1.Infrastructure,
	canMountCustomLogo bool,
	recorder events.Recorder,
) (consoleDeployment *appsv1.Deployment, changed bool, reason string, err error) {
	updatedOperatorConfig := operatorConfig.DeepCopy()
	requiredDeployment := deploymentsub.DefaultDeployment(
		operatorConfig,
		cm,
		serviceCAConfigMap,
		oauthServingCertConfigMap,
		authServerCAConfigMap,
		trustedCAConfigMap,
		sec,
		sessionSecret,
		proxyConfig,
		infrastructureConfig,
		canMountCustomLogo,
	)
	genChanged := operatorConfig.ObjectMeta.Generation != operatorConfig.Status.ObservedGeneration

	if genChanged {
		klog.V(4).Infof("deployment generation changed from %d to %d", operatorConfig.ObjectMeta.Generation, operatorConfig.Status.ObservedGeneration)
	}
	deploymentsub.LogDeploymentAnnotationChanges(co.deploymentClient, requiredDeployment, ctx)

	deployment, deploymentChanged, applyDepErr := resourceapply.ApplyDeployment(
		ctx,
		co.deploymentClient,
		recorder,
		requiredDeployment,
		resourcemerge.ExpectedDeploymentGeneration(requiredDeployment, updatedOperatorConfig.Status.Generations),
	)

	if applyDepErr != nil {
		return nil, false, "FailedApply", applyDepErr
	}
	return deployment, deploymentChanged, "", nil
}

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

	availablePlugins := co.GetAvailablePlugins(operatorConfig.Spec.Plugins)

	monitoringSharedConfig, mscErr := co.managedNSConfigMapLister.ConfigMaps(api.OpenShiftConfigManagedNamespace).Get(api.OpenShiftMonitoringConfigMapName)
	if mscErr != nil {
		if !apierrors.IsNotFound(mscErr) {
			return nil, false, "FailedGetMonitoringSharedConfig", mscErr
		}
		monitoringSharedConfig = &corev1.ConfigMap{}
	}

	telemetryConfig, tcErr := co.GetTelemetryConfiguration(ctx, operatorConfig)
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
		telemetryConfig,
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
func (co *consoleOperator) GetTelemetryConfiguration(ctx context.Context, operatorConfig *operatorv1.Console) (map[string]string, error) {
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

	organizationID := telemetry.GetOrganizationID(telemetryConfig, co.trackables.organizationID, clusterID, accessToken)
	// cache ORGANIZATION_ID
	co.trackables.organizationID = organizationID
	telemetryConfig["ORGANIZATION_ID"] = organizationID

	return telemetryConfig, nil
}

// apply service-ca configmap
func (co *consoleOperator) SyncServiceCAConfigMap(ctx context.Context, operatorConfig *operatorv1.Console) (consoleCM *corev1.ConfigMap, changed bool, reason string, err error) {
	required := configmapsub.DefaultServiceCAConfigMap(operatorConfig)
	// we can't use `resourceapply.ApplyConfigMap` since it compares data, and the service serving cert operator injects the data
	existing, err := co.targetNSConfigMapLister.ConfigMaps(required.Namespace).Get(required.Name)
	if apierrors.IsNotFound(err) {
		actual, err := co.configMapClient.ConfigMaps(required.Namespace).Create(ctx, required, metav1.CreateOptions{})
		if err == nil {
			klog.V(4).Infoln("service-ca configmap created")
			return actual, true, "", err
		} else {
			return actual, true, "FailedCreate", err
		}
	}
	if err != nil {
		return nil, false, "FailedGet", err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	if !*modified {
		klog.V(4).Infoln("service-ca configmap exists and is in the correct state")
		return existing, false, "", nil
	}

	actual, err := co.configMapClient.ConfigMaps(required.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
	if err == nil {
		klog.V(4).Infoln("service-ca configmap updated")
		return actual, true, "", err
	} else {
		return actual, true, "FailedUpdate", err
	}
}

func (co *consoleOperator) SyncTrustedCAConfigMap(ctx context.Context, operatorConfig *operatorv1.Console) (trustedCA *corev1.ConfigMap, changed bool, reason string, err error) {
	required := configmapsub.DefaultTrustedCAConfigMap(operatorConfig)
	existing, err := co.targetNSConfigMapLister.ConfigMaps(required.Namespace).Get(required.Name)
	if apierrors.IsNotFound(err) {
		actual, err := co.configMapClient.ConfigMaps(required.Namespace).Create(ctx, required, metav1.CreateOptions{})
		if err != nil {
			return actual, true, "FailedCreate", err
		}
		klog.V(4).Infoln("trusted-ca-bundle configmap created")
		return actual, true, "", err
	}
	if err != nil {
		return nil, false, "FailedGet", err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	if !*modified {
		klog.V(4).Infoln("trusted-ca-bundle configmap exists and is in the correct state")
		return existing, false, "", nil
	}

	actual, err := co.configMapClient.ConfigMaps(required.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return actual, true, "FailedUpdate", err
	}
	klog.V(4).Infoln("trusted-ca-bundle configmap updated")
	return actual, true, "", err
}

func (co *consoleOperator) SyncCustomLogoConfigMap(ctx context.Context, operatorConfig *operatorv1.Console) (okToMount bool, reason string, err error) {
	// validate first, to avoid a broken volume mount & a crashlooping console
	okToMount, reason, err = co.ValidateCustomLogo(ctx, operatorConfig)

	if okToMount || configmapsub.IsRemoved(operatorConfig) {
		if err := co.UpdateCustomLogoSyncSource(operatorConfig); err != nil {
			return false, "FailedSyncSource", customerrors.NewCustomLogoError("custom logo sync source update error")
		}
	}
	return okToMount, reason, err
}

func (co *consoleOperator) ValidateOAuthServingCertConfigMap(ctx context.Context) (oauthServingCert *corev1.ConfigMap, reason string, err error) {
	oauthServingCertConfigMap, err := co.targetNSConfigMapLister.ConfigMaps(api.OpenShiftConsoleNamespace).Get(api.OAuthServingCertConfigMapName)
	if err != nil {
		klog.V(4).Infoln("oauth-serving-cert configmap not found")
		return nil, "FailedGet", fmt.Errorf("oauth-serving-cert configmap not found")
	}

	_, caBundle := oauthServingCertConfigMap.Data["ca-bundle.crt"]
	if !caBundle {
		return nil, "MissingOAuthServingCertBundle", fmt.Errorf("oauth-serving-cert configmap is missing ca-bundle.crt data")
	}
	return oauthServingCertConfigMap, "", nil
}

// on each pass of the operator sync loop, we need to check the
// operator config for a custom logo.  If this has been set, then
// we notify the resourceSyncer that it needs to start watching this
// configmap in its own sync loop.  Note that the resourceSyncer's actual
// sync loop will run later.  Our operator is waiting to receive
// the copied configmap into the console namespace for a future
// sync loop to mount into the console deployment.
func (c *consoleOperator) UpdateCustomLogoSyncSource(operatorConfig *operatorv1.Console) error {
	source := resourcesynccontroller.ResourceLocation{}
	logoConfigMapName := operatorConfig.Spec.Customization.CustomLogoFile.Name

	if logoConfigMapName != "" {
		source.Name = logoConfigMapName
		source.Namespace = api.OpenShiftConfigNamespace
	}
	// if no custom logo provided, sync an empty source to delete
	return c.resourceSyncer.SyncConfigMap(
		resourcesynccontroller.ResourceLocation{Namespace: api.OpenShiftConsoleNamespace, Name: api.OpenShiftCustomLogoConfigMapName},
		source,
	)
}

func (co *consoleOperator) ValidateCustomLogo(ctx context.Context, operatorConfig *operatorv1.Console) (okToMount bool, reason string, err error) {
	logoConfigMapName := operatorConfig.Spec.Customization.CustomLogoFile.Name
	logoImageKey := operatorConfig.Spec.Customization.CustomLogoFile.Key

	if configmapsub.FileNameOrKeyInconsistentlySet(operatorConfig) {
		klog.V(4).Infoln("custom logo filename or key have not been set")
		return false, "KeyOrFilenameInvalid", customerrors.NewCustomLogoError("either custom logo filename or key have not been set")
	}
	// fine if nothing set, but don't mount it
	if configmapsub.FileNameNotSet(operatorConfig) {
		klog.V(4).Infoln("no custom logo configured")
		return false, "", nil
	}
	logoConfigMap, err := co.configNSConfigMapLister.ConfigMaps(api.OpenShiftConfigNamespace).Get(logoConfigMapName)
	// If we 404, the logo file may not have been created yet.
	if err != nil {
		klog.V(4).Infof("custom logo file %v not found", logoConfigMapName)
		return false, "FailedGet", customerrors.NewCustomLogoError(fmt.Sprintf("custom logo file %v not found", logoConfigMapName))
	}

	_, imageDataFound := logoConfigMap.BinaryData[logoImageKey]
	if !imageDataFound {
		_, imageDataFound = logoConfigMap.Data[logoImageKey]
	}
	if !imageDataFound {
		klog.V(4).Infoln("custom logo file exists but no image provided")
		return false, "NoImageProvided", customerrors.NewCustomLogoError("custom logo file exists but no image provided")
	}

	klog.V(4).Infoln("custom logo ok to mount")
	return true, "", nil
}

func (co *consoleOperator) GetAvailablePlugins(enabledPluginsNames []string) []*v1.ConsolePlugin {
	var availablePlugins []*v1.ConsolePlugin
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

func getNodeComputeEnvironments(nodes []*corev1.Node) ([]string, []string) {
	nodeArchitecturesSet := sets.NewString()
	nodeOperatingSystemSet := sets.NewString()
	for _, node := range nodes {
		nodeArch := node.Labels[api.NodeArchitectureLabel]
		if nodeArch == "" {
			klog.Warningf("Missing architecture label %q on node %q.", api.NodeArchitectureLabel, node.GetName())
		} else {
			nodeArchitecturesSet.Insert(nodeArch)
		}

		nodeOperatingSystem := node.Labels[api.NodeOperatingSystemLabel]
		if nodeOperatingSystem == "" {
			klog.Warningf("Missing operating system label %q on node %q", api.NodeOperatingSystemLabel, node.GetName())
		} else {
			nodeOperatingSystemSet.Insert(nodeOperatingSystem)
		}
	}
	return nodeArchitecturesSet.List(), nodeOperatingSystemSet.List()
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

func (co *consoleOperator) syncSessionSecret(
	ctx context.Context,
	operatorConfig *operatorv1.Console,
	recorder events.Recorder,
) (*corev1.Secret, error) {

	sessionSecret, err := co.secretsLister.Secrets(api.TargetNamespace).Get(api.SessionSecretName)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	var required *corev1.Secret
	if sessionSecret == nil {
		required = secretsub.DefaultSessionSecret(operatorConfig)
	} else {
		required = sessionSecret.DeepCopy()
		changed := secretsub.ResetSessionSecretKeysIfNeeded(required)
		if !changed {
			return required, nil
		}
	}

	secret, _, err := resourceapply.ApplySecret(ctx, co.secretsClient, recorder, required)
	return secret, err
}
