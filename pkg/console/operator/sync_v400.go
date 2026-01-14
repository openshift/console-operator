package operator

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"slices"
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

// RANDOM CHANGE... move along

// The sync loop starts from zero and works its way through the requirements for a running console.
// If at any point something is missing, it creates/updates that piece and immediately dies.
// The next loop will pick up where they previous left off and move the process forward one step.
// This ensures the logic is simpler as we do not have to handle coordination between objects within
// the loop.
func (co *consoleOperator) sync_v400(ctx context.Context, controllerContext factory.SyncContext, updatedOperatorConfig *operatorv1.Console, set configSet) error {
	klog.V(4).Infoln("running sync loop 4.0.0")

	var (
		statusHandler = status.NewStatusHandler(co.operatorClient)
		consoleRoute  *routev1.Route
		consoleURL    *url.URL
	)

	if len(set.Operator.Spec.Ingress.ConsoleURL) == 0 {
		routeName := api.OpenShiftConsoleRouteName
		routeConfig := routesub.NewRouteConfig(updatedOperatorConfig, set.Ingress, routeName)
		if routeConfig.IsCustomHostnameSet() {
			routeName = api.OpenshiftConsoleCustomRouteName
		}

		route, url, routeReasonErr, routeErr := routesub.GetActiveRouteInfo(co.routeLister, routeName)
		// TODO: this controller is no longer responsible for syncing the route.
		//   however, the route is essential for several of the components below.
		//   - the loop should exit early and wait until the RouteSyncController creates the route.
		//     there is nothing new in this flow, other than 2 controllers now look
		//     at the same resource.
		//     - RouteSyncController is responsible for updates
		//     - ConsoleOperatorController (future ConsoleDeploymentController) is responsible for reads only.
		statusHandler.AddConditions(status.HandleProgressingOrDegraded("SyncLoopRefresh", routeReasonErr, routeErr))
		if routeErr != nil {
			return statusHandler.FlushAndReturn(routeErr)
		}
		consoleRoute = route
		consoleURL = url
	} else {
		url, err := url.Parse(set.Operator.Spec.Ingress.ConsoleURL)
		if err != nil {
			return statusHandler.FlushAndReturn(fmt.Errorf("failed to get console url: %w", err))
		}
		consoleURL = url
	}

	authnConfig, err := co.authnConfigLister.Get(api.ConfigResourceName)
	if err != nil {
		return statusHandler.FlushAndReturn(err)
	}

	var (
		targetNamespaceAuthServerCA *corev1.ConfigMap
		sessionSecret               *corev1.Secret
	)
	switch authnConfig.Spec.Type {
	case configv1.AuthenticationTypeOIDC:
		if len(authnConfig.Spec.OIDCProviders) > 0 {
			oidcProvider := authnConfig.Spec.OIDCProviders[0]
			certAuthorityName := oidcProvider.Issuer.CertificateAuthority.Name
			if certAuthorityName != "" {
				targetNamespaceAuthServerCA, err = co.targetNSConfigMapLister.ConfigMaps(api.OpenShiftConsoleNamespace).Get(certAuthorityName)
				statusHandler.AddConditions(status.HandleProgressingOrDegraded("OIDCProviderTrustedAuthorityConfigGet", "FailedGet", err))
				if err != nil {
					return statusHandler.FlushAndReturn(err)
				}
			}
		}

		sessionSecret, err = co.syncSessionSecret(ctx, updatedOperatorConfig, controllerContext.Recorder())
		if err != nil {
			return statusHandler.FlushAndReturn(err)
		}
	}

	customLogosErr, customLogosErrReason := co.SyncCustomLogos(updatedOperatorConfig)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("CustomLogoSync", customLogosErrReason, customLogosErr))
	if customLogosErr != nil {
		return statusHandler.FlushAndReturn(customLogosErr)
	}

	techPreviewEnabled, techPreviewErrReason, techPreviewErr := co.SyncTechPreview()
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("TechPreviewSync", techPreviewErrReason, techPreviewErr))
	if techPreviewErr != nil {
		return statusHandler.FlushAndReturn(techPreviewErr)
	}

	cm, cmErrReason, cmErr := co.SyncConfigMap(
		ctx,
		set.Operator,
		set.Console,
		set.Infrastructure,
		set.OAuth,
		authnConfig,
		consoleRoute,
		controllerContext.Recorder(),
		consoleURL.Hostname(),
		techPreviewEnabled,
	)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ConfigMapSync", cmErrReason, cmErr))
	if cmErr != nil {
		return statusHandler.FlushAndReturn(cmErr)
	}

	serviceCAConfigMap, serviceCAErrReason, serviceCAErr := co.SyncServiceCAConfigMap(ctx, set.Operator)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ServiceCASync", serviceCAErrReason, serviceCAErr))
	if serviceCAErr != nil {
		return statusHandler.FlushAndReturn(serviceCAErr)
	}

	trustedCAConfigMap, trustedCAErrReason, trustedCAErr := co.SyncTrustedCAConfigMap(ctx, set.Operator)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("TrustedCASync", trustedCAErrReason, trustedCAErr))
	if trustedCAErr != nil {
		return statusHandler.FlushAndReturn(trustedCAErr)
	}

	var oauthServingCertConfigMap *corev1.ConfigMap
	switch authnConfig.Spec.Type {
	// We don't disable auth since the internal OAuth server is not disabled even with auth type 'None'.
	case "", configv1.AuthenticationTypeIntegratedOAuth, configv1.AuthenticationTypeNone:
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

	actualDeployment, depErrReason, depErr := co.SyncDeployment(
		ctx,
		set.Operator,
		cm,
		serviceCAConfigMap,
		oauthServingCertConfigMap,
		targetNamespaceAuthServerCA,
		trustedCAConfigMap,
		clientSecret,
		sessionSecret,
		set.Proxy,
		set.Infrastructure,
		controllerContext.Recorder(),
	)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("DeploymentSync", depErrReason, depErr))
	if depErr != nil {
		return statusHandler.FlushAndReturn(depErr)
	}

	statusHandler.UpdateDeploymentGeneration(actualDeployment)
	statusHandler.UpdateReadyReplicas(actualDeployment.Status.ReadyReplicas)
	statusHandler.UpdateObservedGeneration(set.Operator.ObjectMeta.Generation)

	statusHandler.AddCondition(status.HandleProgressing("SyncLoopRefresh", "InProgress", func() error {
		version := os.Getenv("OPERATOR_IMAGE_VERSION")
		// Only report Progressing=True when the deployment is actually rolling out
		// or the operator version is changing. Do NOT report Progressing just because
		// resources were updated during reconciliation, as per the API guidelines:
		// "Operators should not report Progressing when they are reconciling (without action)
		// a previously known state."
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

	klog.V(4).Infof("sync loop 4.0.0 complete")
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
	recorder events.Recorder,
) (consoleDeployment *appsv1.Deployment, reason string, err error) {
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
	)
	genChanged := operatorConfig.ObjectMeta.Generation != operatorConfig.Status.ObservedGeneration

	if genChanged {
		klog.V(4).Infof("deployment generation changed from %d to %d", operatorConfig.ObjectMeta.Generation, operatorConfig.Status.ObservedGeneration)
	}
	deploymentsub.LogDeploymentAnnotationChanges(co.deploymentClient, requiredDeployment, ctx)

	deployment, _, applyDepErr := resourceapply.ApplyDeployment(
		ctx,
		co.deploymentClient,
		recorder,
		requiredDeployment,
		resourcemerge.ExpectedDeploymentGeneration(requiredDeployment, updatedOperatorConfig.Status.Generations),
	)

	if applyDepErr != nil {
		return nil, "FailedApply", applyDepErr
	}
	return deployment, "", nil
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
	authConfig *configv1.Authentication,
	activeConsoleRoute *routev1.Route,
	recorder events.Recorder,
	consoleHost string,
	techPreviewEnabled bool,
) (consoleConfigMap *corev1.ConfigMap, reason string, err error) {

	managedConfig, mcErr := co.managedNSConfigMapLister.ConfigMaps(api.OpenShiftConfigManagedNamespace).Get(api.OpenShiftConsoleConfigMapName)
	if mcErr != nil {
		if !apierrors.IsNotFound(mcErr) {
			return nil, "FailedGetManagedConfig", mcErr
		}
		managedConfig = &corev1.ConfigMap{}
	}
	nodeList, nodeListErr := co.nodeLister.List(labels.Everything())
	if nodeListErr != nil {
		return nil, "FailedListNodes", nodeListErr
	}
	nodeArchitectures, nodeOperatingSystems := getNodeComputeEnvironments(nodeList)

	// TODO: currently there's no way to get this for authentication type OIDC
	inactivityTimeoutSeconds := 0
	switch authConfig.Spec.Type {
	case "", configv1.AuthenticationTypeIntegratedOAuth:
		oauthClient, oacErr := co.oauthClientLister.Get(oauthsub.Stub().Name)
		if oacErr != nil {
			return nil, "FailedGetOAuthClient", oacErr
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
			return nil, "FailedGetMonitoringSharedConfig", mscErr
		}
		monitoringSharedConfig = &corev1.ConfigMap{}
	}

	telemetryConfig, tcErr := co.GetTelemetryConfiguration(ctx, operatorConfig)
	if tcErr != nil {
		return nil, "FailedGetTelemetryConfig", tcErr
	}

	var (
		copiedCSVsDisabled bool
		ccdErr             error
	)
	if !co.trackables.isOLMDisabled {
		copiedCSVsDisabled, ccdErr = co.isCopiedCSVsDisabled(ctx)
		if ccdErr != nil {
			return nil, "FailedGetOLMConfig", ccdErr
		}
	}

	defaultConfigmap, _, err := configmapsub.DefaultConfigMap(
		operatorConfig,
		consoleConfig,
		authConfig,
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
		techPreviewEnabled,
	)
	if err != nil {
		return nil, "FailedConsoleConfigBuilder", err
	}
	cm, cmChanged, cmErr := resourceapply.ApplyConfigMap(ctx, co.configMapClient, recorder, defaultConfigmap)
	if cmErr != nil {
		return nil, "FailedApply", cmErr
	}
	if cmChanged {
		klog.V(4).Infoln("new console config yaml:")
		klog.V(4).Infof("%s", cm.Data)
	}
	return cm, "ConsoleConfigBuilder", cmErr
}

// Build telemetry configuration in following order:
//  1. check if the telemetry client is available and set the "TELEMETER_CLIENT_DISABLED" annotation accordingly
//  2. get telemetry annotation from console-operator config
//  3. get default telemetry value from telemetry-config configmap
//  4. get CLUSTER_ID from the cluster-version config
//  5. get ORGANIZATION_ID and ACCOUNT_MAIL from OCM, if they are not already set
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
	organizationID, accountMail, refreshCache := telemetry.GetOrganizationMeta(telemetryConfig, co.trackables.organizationID, co.trackables.accountMail, clusterID, accessToken)
	// cache fetched ORGANIZATION_ID and ACCOUNT_MAIL
	if refreshCache {
		co.trackables.organizationID = organizationID
		co.trackables.accountMail = accountMail
	}
	telemetryConfig["ORGANIZATION_ID"] = organizationID
	telemetryConfig["ACCOUNT_MAIL"] = accountMail

	return telemetryConfig, nil
}

// apply service-ca configmap
func (co *consoleOperator) SyncServiceCAConfigMap(ctx context.Context, operatorConfig *operatorv1.Console) (consoleCM *corev1.ConfigMap, reason string, err error) {
	required := configmapsub.DefaultServiceCAConfigMap(operatorConfig)
	// we can't use `resourceapply.ApplyConfigMap` since it compares data, and the service serving cert operator injects the data
	existing, err := co.targetNSConfigMapLister.ConfigMaps(required.Namespace).Get(required.Name)
	if apierrors.IsNotFound(err) {
		actual, err := co.configMapClient.ConfigMaps(required.Namespace).Create(ctx, required, metav1.CreateOptions{})
		if err == nil {
			klog.V(4).Infoln("service-ca configmap created")
			return actual, "", err
		} else {
			return actual, "FailedCreate", err
		}
	}
	if err != nil {
		return nil, "FailedGet", err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	if !*modified {
		klog.V(4).Infoln("service-ca configmap exists and is in the correct state")
		return existing, "", nil
	}

	actual, err := co.configMapClient.ConfigMaps(required.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
	if err == nil {
		klog.V(4).Infoln("service-ca configmap updated")
		return actual, "", err
	} else {
		return actual, "FailedUpdate", err
	}
}

func (co *consoleOperator) SyncTrustedCAConfigMap(ctx context.Context, operatorConfig *operatorv1.Console) (trustedCA *corev1.ConfigMap, reason string, err error) {
	required := configmapsub.DefaultTrustedCAConfigMap(operatorConfig)
	existing, err := co.targetNSConfigMapLister.ConfigMaps(required.Namespace).Get(required.Name)
	if apierrors.IsNotFound(err) {
		actual, err := co.configMapClient.ConfigMaps(required.Namespace).Create(ctx, required, metav1.CreateOptions{})
		if err != nil {
			return actual, "FailedCreate", err
		}
		klog.V(4).Infoln("trusted-ca-bundle configmap created")
		return actual, "", err
	}
	if err != nil {
		return nil, "FailedGet", err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	if !*modified {
		klog.V(4).Infoln("trusted-ca-bundle configmap exists and is in the correct state")
		return existing, "", nil
	}

	actual, err := co.configMapClient.ConfigMaps(required.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return actual, "FailedUpdate", err
	}
	klog.V(4).Infoln("trusted-ca-bundle configmap updated")
	return actual, "", err
}

// SyncTechPreview determines if tech preview features should be enabled based on cluster FeatureSet
func (co *consoleOperator) SyncTechPreview() (techPreviewEnabled bool, reason string, err error) {
	featureGate, err := co.featureGateLister.Get(api.ConfigResourceName)
	if err != nil {
		klog.V(4).Infof("failed to get FeatureGate resource: %v.", err)
		return false, "FailedGet", err
	}

	techPreviewEnabled = featureGate.Spec.FeatureSet == configv1.TechPreviewNoUpgrade

	if techPreviewEnabled {
		klog.V(4).Infoln("Console Technology Preview features enabled based on cluster FeatureSet TechPreviewNoUpgrade.")
	}
	return techPreviewEnabled, "", nil
}

func (co *consoleOperator) SyncCustomLogos(operatorConfig *operatorv1.Console) (error, string) {
	if operatorConfig.Spec.Customization.CustomLogoFile.Name != "" || operatorConfig.Spec.Customization.CustomLogoFile.Key != "" {
		return co.SyncCustomLogoConfigMap(operatorConfig)
	}

	var (
		aggregatedError error
		err             error
		reason          string
		newSyncedLogos  []string
	)
	for _, logo := range operatorConfig.Spec.Customization.Logos {
		for _, theme := range logo.Themes {
			logoToSync := theme.Source.ConfigMap
			if err, reason = co.ValidateCustomLogo(logoToSync); err != nil {
				if aggregatedError == nil {
					aggregatedError = fmt.Errorf("error syncing custom logos:  - Invalid config: %v, %s", logoToSync, err.Error())
				} else {
					aggregatedError = fmt.Errorf("%s  - %v, %s", aggregatedError.Error(), logoToSync, err.Error())
				}
			} else {
				newSyncedLogos = append(newSyncedLogos, logoToSync.Name)
			}
		}
	}
	if aggregatedError != nil {
		return aggregatedError, reason
	}
	slices.Sort(newSyncedLogos)
	return co.UpdateCustomLogoSyncSources(newSyncedLogos)
}

// TODO remove deprecated CustomLogoFile API
func (co *consoleOperator) SyncCustomLogoConfigMap(operatorConfig *operatorv1.Console) (error, string) {
	var customLogoRef = operatorv1.ConfigMapFileReference(operatorConfig.Spec.Customization.CustomLogoFile)
	klog.V(4).Infof("[SyncCustomLogoConfigMap] syncing customLogoFile, Name: %s, Key: %s", customLogoRef.Name, customLogoRef.Key)
	err, reason := co.ValidateCustomLogo(&customLogoRef)
	if err != nil {
		klog.V(4).Infof("[SyncCustomLogoConfigMap] failed to sync customLogoFile, %v", err)
		return err, reason
	}
	return co.UpdateCustomLogoSyncSources([]string{customLogoRef.Name})
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
// operator config for custom logos.  If this has been set, then
// we notify the resourceSyncer that it needs to start watching the associated
// configmaps in its own sync loop.  Note that the resourceSyncer's actual
// sync loop will run later.  Our operator is waiting to receive
// the copied configmaps into the console namespace for a future
// sync loop to mount into the console deployment.
func (co *consoleOperator) UpdateCustomLogoSyncSources(configMapNames []string) (error, string) {
	klog.V(4).Info("[UpdateCustomLogoSyncSources] syncing custom logo configmap resources")
	klog.V(4).Infof("%#v", configMapNames)

	errors := []string{}
	if len(co.trackables.customLogoConfigMaps) > 0 {
		klog.V(4).Info("[UpdateCustomLogoSyncSources] unsyncing custom logo configmap resources from previous sync loop...")
		for _, configMapName := range co.trackables.customLogoConfigMaps {
			err := co.UpdateCustomLogoSyncSource(configMapName, true)
			if err != nil {
				errors = append(errors, err.Error())
			}
		}

		if len(errors) > 0 {
			msg := fmt.Sprintf("error syncing custom logo configmap resources:\n%v", errors)
			klog.V(4).Infof("[UpdateCustomLogoSyncSources] %s", msg)
			return fmt.Errorf("%s", msg), "FailedResourceSync"
		}
	}

	if len(configMapNames) > 0 {
		// If the new list of synced configmaps is different than the last sync, we need to update the
		// resource syncer with the new list, and re
		klog.V(4).Infof("[UpdateCustomLogoSyncSources] syncing new custom logo configmap resources...")
		for _, configMapName := range configMapNames {
			err := co.UpdateCustomLogoSyncSource(configMapName, false)
			if err != nil {
				errors = append(errors, err.Error())
			}
		}

		if len(errors) > 0 {
			msg := fmt.Sprintf("error syncing custom logo configmap resources:\n%v", errors)
			klog.V(4).Infof("[UpdateCustomLogoSyncSources] %s", msg)
			return fmt.Errorf("%s", msg), "FailedResourceSync"
		}
	}

	co.trackables.customLogoConfigMaps = configMapNames

	klog.V(4).Info("[UpdateCustomLogoSyncSources] done")
	return nil, ""
}

func (co *consoleOperator) ValidateCustomLogo(logoFileRef *operatorv1.ConfigMapFileReference) (err error, reason string) {
	logoConfigMapName := logoFileRef.Name
	logoImageKey := logoFileRef.Key

	if (len(logoConfigMapName) == 0) != (len(logoImageKey) == 0) {
		msg := "custom logo filename or key have not been set"
		klog.V(4).Infof("[ValidateCustomLogo] %s", msg)
		return customerrors.NewCustomLogoError(msg), "KeyOrFilenameInvalid"
	}
	// fine if nothing set, but don't mount it
	if len(logoConfigMapName) == 0 {
		klog.V(4).Infoln("[ValidateCustomLogo] no custom logo configured")
		return nil, ""
	}
	logoConfigMap, err := co.configNSConfigMapLister.ConfigMaps(api.OpenShiftConfigNamespace).Get(logoConfigMapName)
	// If we 404, the logo file may not have been created yet.
	if err != nil {
		msg := fmt.Sprintf("failed to get ConfigMap %v, %v", logoConfigMapName, err)
		klog.V(4).Infof("[ValidateCustomLogo] %s", msg)
		return customerrors.NewCustomLogoError(msg), "FailedGet"
	}

	_, imageDataFound := logoConfigMap.BinaryData[logoImageKey]
	if !imageDataFound {
		_, imageDataFound = logoConfigMap.Data[logoImageKey]
	}
	if !imageDataFound {
		msg := "custom logo file exists but no image provided"
		klog.V(4).Infof("[ValidateCustomLogo] %s", msg)
		return customerrors.NewCustomLogoError(msg), "NoImageProvided"
	}

	klog.V(4).Infof("[ValidateCustomLogo] custom logo %s ok to mount", logoConfigMapName)
	return nil, ""
}

func (co *consoleOperator) UpdateCustomLogoSyncSource(targetName string, unsync bool) error {
	source := resourcesynccontroller.ResourceLocation{}
	if !unsync {
		source.Name = targetName
		source.Namespace = api.OpenShiftConfigNamespace
	}

	target := resourcesynccontroller.ResourceLocation{
		Namespace: api.OpenShiftConsoleNamespace,
		Name:      targetName,
	}

	if unsync {
		klog.V(4).Infof("[UpdateCustomLogoSyncSource] unsyncing %s", targetName)
	} else {
		klog.V(4).Infof("[UpdateCustomLogoSyncSource] syncing %s", targetName)
	}
	return co.resourceSyncer.SyncConfigMap(target, source)
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
