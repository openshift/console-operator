package operator

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"

	// kube
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	v1alpha1 "github.com/openshift/api/console/v1alpha1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/route/routeapihelpers"

	// operator
	customerrors "github.com/openshift/console-operator/pkg/console/errors"
	"github.com/openshift/console-operator/pkg/console/metrics"
	"github.com/openshift/console-operator/pkg/console/status"
	configmapsub "github.com/openshift/console-operator/pkg/console/subresource/configmap"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
	managedclusteractionsub "github.com/openshift/console-operator/pkg/console/subresource/managedclusteraction"
	oauthsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"
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

	route, consoleURL, routeReasoneErr, routeErr := co.GetActiveRouteInfo(ctx, routeName)
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

	cm, cmChanged, cmErrReason, cmErr := co.SyncConfigMap(ctx, set.Operator, set.Console, set.Infrastructure, set.OAuth, route, controllerContext.Recorder())
	toUpdate = toUpdate || cmChanged
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ConfigMapSync", cmErrReason, cmErr))
	if cmErr != nil {
		return statusHandler.FlushAndReturn(cmErr)
	}

	clusterCAConfigMaps, clusterCAConfigMapsChanged, clusterCAConfigMapsErrReason, clusterCAConfigMapsErr := co.SyncClusterCAConfigMaps(ctx, set.Operator, controllerContext.Recorder())
	toUpdate = toUpdate || clusterCAConfigMapsChanged
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ClusterCASync", clusterCAConfigMapsErrReason, clusterCAConfigMapsErr))

	_, managedClusterActionsChanged, managedClusterActionsErrReason, managedClusterActionsErr := co.SyncManagedClusterActions(ctx, set.Operator)
	toUpdate = toUpdate || managedClusterActionsChanged
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterActionSync", managedClusterActionsErrReason, managedClusterActionsErr))

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

	defaultIngressCertConfigMap, defaultIngressCertErrReason, defaultIngressCertErr := co.ValidateDefaultIngressCertConfigMap(ctx)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("DefaultIngressCertValidation", defaultIngressCertErrReason, defaultIngressCertErr))
	if defaultIngressCertErr != nil {
		return statusHandler.FlushAndReturn(defaultIngressCertErr)
	}

	sec, secChanged, secErr := co.SyncSecret(ctx, set.Operator, controllerContext.Recorder())
	toUpdate = toUpdate || secChanged
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSecretSync", "FailedApply", secErr))
	if secErr != nil {
		return statusHandler.FlushAndReturn(secErr)
	}

	oauthClient, oauthChanged, oauthErrReason, oauthErr := co.SyncOAuthClient(ctx, set.Operator, sec, consoleURL.String())
	toUpdate = toUpdate || oauthChanged
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSync", oauthErrReason, oauthErr))
	if oauthErr != nil {
		return statusHandler.FlushAndReturn(oauthErr)
	}

	actualDeployment, depChanged, depErrReason, depErr := co.SyncDeployment(ctx, set.Operator, cm, clusterCAConfigMaps, serviceCAConfigMap, defaultIngressCertConfigMap, trustedCAConfigMap, sec, set.Proxy, set.Infrastructure, customLogoCanMount, controllerContext.Recorder())
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
			return errors.New("Changes made during sync updates, additional sync expected.")
		}
		version := os.Getenv("RELEASE_VERSION")
		if !deploymentsub.IsAvailableAndUpdated(actualDeployment) {
			return errors.New(fmt.Sprintf("Working toward version %s", version))
		}
		if co.versionGetter.GetVersions()["operator"] != version {
			co.versionGetter.SetVersion("operator", version)
		}
		return nil
	}()))

	statusHandler.AddCondition(status.HandleAvailable(func() (prefix string, reason string, err error) {
		prefix = "Deployment"
		if !deploymentsub.IsReady(actualDeployment) {
			return prefix, "InsufficientReplicas", errors.New(fmt.Sprintf("%v pods available for console deployment", actualDeployment.Status.ReadyReplicas))
		}
		if !deploymentsub.IsReadyAndUpdated(actualDeployment) {
			return prefix, "FailedUpdate", errors.New(fmt.Sprintf("%v replicas ready at version %s", actualDeployment.Status.ReadyReplicas, os.Getenv("RELEASE_VERSION")))
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

	_, _, consolePublicConfigErr := co.SyncConsolePublicConfig(consoleURL.String(), controllerContext.Recorder())
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
		if secChanged {
			klog.V(4).Infof("\t secret changed: %v", sec.GetResourceVersion())
		}
		if oauthChanged {
			klog.V(4).Infof("\t oauth changed: %v", oauthClient.GetResourceVersion())
		}
		if depChanged {
			klog.V(4).Infof("\t deployment changed: %v", actualDeployment.GetResourceVersion())
		}
	}()

	return statusHandler.FlushAndReturn(nil)
}

func (co *consoleOperator) GetActiveRouteInfo(ctx context.Context, activeRouteName string) (route *routev1.Route, routeURL *url.URL, reason string, err error) {
	route, routeErr := co.routeClient.Routes(api.TargetNamespace).Get(ctx, activeRouteName, metav1.GetOptions{})
	if routeErr != nil {
		return nil, nil, "FailedGet", routeErr
	}
	uri, _, uriErr := routeapihelpers.IngressURI(route, route.Spec.Host)
	if uriErr != nil {
		return nil, nil, "FailedIngress", uriErr
	}

	return route, uri, "", nil
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

func (co *consoleOperator) SyncConsolePublicConfig(consoleURL string, recorder events.Recorder) (*corev1.ConfigMap, bool, error) {
	requiredConfigMap := configmapsub.DefaultPublicConfig(consoleURL)
	return resourceapply.ApplyConfigMap(co.configMapClient, recorder, requiredConfigMap)
}

func (co *consoleOperator) SyncDeployment(
	ctx context.Context,
	operatorConfig *operatorv1.Console,
	cm *corev1.ConfigMap,
	clusterCAConfigMaps []*corev1.ConfigMap,
	serviceCAConfigMap *corev1.ConfigMap,
	defaultIngressCertConfigMap *corev1.ConfigMap,
	trustedCAConfigMap *corev1.ConfigMap,
	sec *corev1.Secret,
	proxyConfig *configv1.Proxy,
	infrastructureConfig *configv1.Infrastructure,
	canMountCustomLogo bool,
	recorder events.Recorder) (consoleDeployment *appsv1.Deployment, changed bool, reason string, err error) {

	updatedOperatorConfig := operatorConfig.DeepCopy()
	requiredDeployment := deploymentsub.DefaultDeployment(operatorConfig, cm, clusterCAConfigMaps, serviceCAConfigMap, defaultIngressCertConfigMap, trustedCAConfigMap, sec, proxyConfig, infrastructureConfig, canMountCustomLogo)
	genChanged := operatorConfig.ObjectMeta.Generation != operatorConfig.Status.ObservedGeneration

	if genChanged {
		klog.V(4).Infof("deployment generation changed from %d to %d", operatorConfig.ObjectMeta.Generation, operatorConfig.Status.ObservedGeneration)
	}
	deploymentsub.LogDeploymentAnnotationChanges(co.deploymentClient, requiredDeployment, ctx)

	deployment, deploymentChanged, applyDepErr := resourceapply.ApplyDeployment(
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

// applies changes to the oauthclient
// should not be called until route & secret dependencies are verified
func (co *consoleOperator) SyncOAuthClient(
	ctx context.Context,
	operatorConfig *operatorv1.Console,
	sec *corev1.Secret,
	consoleURL string,
) (consoleoauthclient *oauthv1.OAuthClient, changed bool, reason string, err error) {
	oauthClient, err := co.oauthClient.OAuthClients().Get(ctx, oauthsub.Stub().Name, metav1.GetOptions{})
	if err != nil {
		// at this point we must die & wait for someone to fix the lack of an outhclient. there is nothing we can do.
		return nil, false, "FailedGet", errors.New(fmt.Sprintf("oauth client for console does not exist and cannot be created (%v)", err))
	}
	oauthsub.RegisterConsoleToOAuthClient(oauthClient, consoleURL, secretsub.GetSecretString(sec))
	oauthClient, oauthChanged, oauthErr := oauthsub.CustomApplyOAuth(co.oauthClient, oauthClient, ctx)
	if oauthErr != nil {
		return nil, false, "FailedRegister", oauthErr
	}
	return oauthClient, oauthChanged, "", nil
}

func (co *consoleOperator) SyncSecret(ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder) (*corev1.Secret, bool, error) {
	secret, err := co.secretsClient.Secrets(api.TargetNamespace).Get(ctx, secretsub.Stub().Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) || secretsub.GetSecretString(secret) == "" {
		return resourceapply.ApplySecret(co.secretsClient, recorder, secretsub.DefaultSecret(operatorConfig, crypto.Random256BitsString()))
	}
	// any error should be returned & kill the sync loop
	if err != nil {
		return nil, false, err
	}
	return secret, false, nil
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
	activeConsoleRoute *routev1.Route,
	recorder events.Recorder,
) (consoleConfigMap *corev1.ConfigMap, changed bool, reason string, err error) {

	managedConfig, mcErr := co.configMapClient.ConfigMaps(api.OpenShiftConfigManagedNamespace).Get(ctx, api.OpenShiftConsoleConfigMapName, metav1.GetOptions{})
	if mcErr != nil && !apierrors.IsNotFound(mcErr) {
		return nil, false, "FailedGetManagedConfig", mcErr
	}

	useDefaultCAFile := false
	// We are syncing the `default-ingress-cert` configmap from `openshift-config-managed` to `openshift-console`.
	// `default-ingress-cert` is only published in `openshift-config-managed` in OpenShift 4.4.0 and newer.
	// If the `default-ingress-cert` configmap in `openshift-console` exists, we should mount that to the console container,
	// otherwise default to `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt`
	_, rcaErr := co.configMapClient.ConfigMaps(api.OpenShiftConsoleNamespace).Get(ctx, api.DefaultIngressCertConfigMapName, metav1.GetOptions{})
	if rcaErr != nil && apierrors.IsNotFound(rcaErr) {
		useDefaultCAFile = true
	}

	inactivityTimeoutSeconds := 0
	oauthClient, oacErr := co.oauthClient.OAuthClients().Get(ctx, oauthsub.Stub().Name, metav1.GetOptions{})
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

	pluginsEndpointMap := co.GetPluginsEndpointMap(operatorConfig.Spec.Plugins)
	managedClusters := co.GetMangagedClusters(ctx)
	defaultConfigmap, _, err := configmapsub.DefaultConfigMap(operatorConfig, consoleConfig, managedConfig, infrastructureConfig, activeConsoleRoute, useDefaultCAFile, inactivityTimeoutSeconds, pluginsEndpointMap, managedClusters)
	if err != nil {
		return nil, false, "FailedConsoleConfigBuilder", err
	}
	cm, cmChanged, cmErr := resourceapply.ApplyConfigMap(co.configMapClient, recorder, defaultConfigmap)
	if cmErr != nil {
		return nil, false, "FailedApply", cmErr
	}
	if cmChanged {
		klog.V(4).Infoln("new console config yaml:")
		klog.V(4).Infof("%s", cm.Data)
	}
	return cm, cmChanged, "ConsoleConfigBuilder", cmErr
}

func (co *consoleOperator) SyncClusterCAConfigMaps(ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder) ([]*corev1.ConfigMap, bool, string, error) {
	managedClusters, err := co.managedClusterClient.ManagedClusters().List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("local-cluster!=true")})
	if err != nil || len(managedClusters.Items) == 0 {
		return nil, false, "", nil
	}

	configMaps := []*corev1.ConfigMap{}
	errorReason := ""
	modified := false
	for _, managedCluster := range managedClusters.Items {
		clusterName := managedCluster.GetName()
		clientConfigs := managedCluster.Spec.ManagedClusterClientConfigs
		if clientConfigs == nil || len(clientConfigs) == 0 {
			klog.V(4).Infoln(fmt.Sprintf("Skippping API server CA ConfigMap for managed cluster %v, no client config found", clusterName))
			continue
		}
		caBundle := clientConfigs[0].CABundle
		if caBundle == nil {
			klog.V(4).Infoln(fmt.Sprintf("Skippping API server CA ConfigMap for managed cluster %v, ca bundle not found", clusterName))
			continue
		}
		required := configmapsub.DefaultClusterCAConfigMap(clusterName, caBundle, operatorConfig)
		configMap, configMapModified, configMapApplyError := resourceapply.ApplyConfigMap(co.configMapClient, recorder, required)
		modified = configMapModified || modified
		if configMapApplyError != nil {
			klog.V(4).Infoln(fmt.Sprintf("Skippping API server CA ConfigMap for managed cluster %v, CA ConfigMap not applied", clusterName))
			err = configMapApplyError
			errorReason = "FailedApply"
			continue
		}
		configMaps = append(configMaps, configMap)
	}
	return configMaps, modified, errorReason, err
}

// apply service-ca configmap
func (co *consoleOperator) SyncServiceCAConfigMap(ctx context.Context, operatorConfig *operatorv1.Console) (consoleCM *corev1.ConfigMap, changed bool, reason string, err error) {
	required := configmapsub.DefaultServiceCAConfigMap(operatorConfig)
	// we can't use `resourceapply.ApplyConfigMap` since it compares data, and the service serving cert operator injects the data
	existing, err := co.configMapClient.ConfigMaps(required.Namespace).Get(ctx, required.Name, metav1.GetOptions{})
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
	existing, err := co.configMapClient.ConfigMaps(required.Namespace).Get(ctx, required.Name, metav1.GetOptions{})
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

func (co *consoleOperator) ValidateDefaultIngressCertConfigMap(ctx context.Context) (defaultIngressCert *corev1.ConfigMap, reason string, err error) {
	defaultIngressCertConfigMap, err := co.configMapClient.ConfigMaps(api.OpenShiftConsoleNamespace).Get(ctx, api.DefaultIngressCertConfigMapName, metav1.GetOptions{})
	if err != nil {
		klog.V(4).Infoln("default-ingress-cert configmap not found")
		return nil, "FailedGet", fmt.Errorf("default-ingress-cert configmap not found")
	}

	_, caBundle := defaultIngressCertConfigMap.Data["ca-bundle.crt"]
	if !caBundle {
		return nil, "MissingDefaultIngressCertBundle", fmt.Errorf("default-ingress-cert configmap is missing ca-bundle.crt data")
	}
	return defaultIngressCertConfigMap, "", nil
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
	logoConfigMap, err := co.configMapClient.ConfigMaps(api.OpenShiftConfigNamespace).Get(ctx, logoConfigMapName, metav1.GetOptions{})
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

func (co *consoleOperator) GetPluginsEndpointMap(enabledPluginsNames []string) map[string]string {
	pluginsEndpointMap := map[string]string{}
	for _, pluginName := range enabledPluginsNames {
		plugin, err := co.consolePluginLister.Get(pluginName)
		if err != nil {
			klog.Errorf("failed to get %q plugin: %v", pluginName, err)
			continue
		}
		pluginsEndpointMap[pluginName] = getServiceHostname(plugin)
	}
	return pluginsEndpointMap
}

func (co *consoleOperator) GetMangagedClusters(ctx context.Context) []consoleserver.ManagedClusterConfig {
	managedClusters, err := co.managedClusterClient.ManagedClusters().List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("local-cluster!=true")})
	if err != nil {
		klog.Errorf("Failed to list ManagedClusters: %v", err)
		return nil
	}

	if len(managedClusters.Items) == 0 {
		return nil
	}

	managedClusterConfigs := []consoleserver.ManagedClusterConfig{}
	for _, managedCluster := range managedClusters.Items {
		managedClusterName := managedCluster.GetName()
		klog.V(4).Infoln(fmt.Sprintf("Building config for  managed cluster: %v", managedClusterName))

		// Check that managed cluster CA ConfigMap has already been synced, if not skip this managed cluster
		caConfigMap, err := co.configMapClient.ConfigMaps(api.OpenShiftConsoleNamespace).Get(ctx, fmt.Sprintf("%s-%s", managedClusterName, api.ClusterCAConfigMapNameSuffix), metav1.GetOptions{})
		if apierrors.IsNotFound(err) || caConfigMap == nil {
			klog.V(4).Infoln(fmt.Sprintf("Skipping managed cluster %v, no CA file found", managedClusterName))
			continue
		}

		clientConfigs := managedCluster.Spec.ManagedClusterClientConfigs
		if clientConfigs == nil || len(clientConfigs) == 0 {
			klog.V(4).Infoln(fmt.Sprintf("Skippping  managed cluster %v, no client config found", managedClusterName))
			continue
		}

		url := clientConfigs[0].URL
		if url == "" {
			klog.V(4).Infoln(fmt.Sprintf("Skipping managed cluster %v, no API server URL defined", managedClusterName))
			continue
		}

		managedClusterConfig := consoleserver.ManagedClusterConfig{
			Name: managedClusterName,
			APIServer: consoleserver.ManagedClusterAPIServerConfig{
				URL:    url,
				CAFile: fmt.Sprintf("/var/managed-clusters/%s/ca.crt", configmapsub.ClusterCAConfigMapName(managedClusterName)),
			},
		}
		managedClusterConfigs = append(managedClusterConfigs, managedClusterConfig)
	}

	return managedClusterConfigs
}

func getServiceHostname(plugin *v1alpha1.ConsolePlugin) string {
	pluginURL := &url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s.%s.svc.cluster.local:%d", plugin.Spec.Service.Name, plugin.Spec.Service.Namespace, plugin.Spec.Service.Port),
		Path:   plugin.Spec.Service.BasePath,
	}
	return pluginURL.String()
}

func (co *consoleOperator) SyncManagedClusterActions(ctx context.Context, operatorConfig *operatorv1.Console) ([]*unstructured.Unstructured, bool, string, error) {
	managedClusters, err := co.managedClusterClient.ManagedClusters().List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("local-cluster!=true")})
	if err != nil || len(managedClusters.Items) == 0 {
		return nil, false, "", fmt.Errorf("Failed to list ManagedClusters: %v", err)
	}
	errors := []error{}
	managedClusterActions := []*unstructured.Unstructured{}
	for _, managedCluster := range managedClusters.Items {
		mca := managedclusteractionsub.DefaultManagedClusterAction(operatorConfig, managedCluster.Name)
		gv := schema.GroupVersion{Group: "action.open-cluster-management.io", Version: "v1beta1"}
		opt := metav1.CreateOptions{}
		resp, err := co.dynamicClient.Resource(gv.WithResource("managedclusteraction")).Namespace(managedCluster.Namespace).Create(ctx, mca, opt)
		if err != nil {
			errors = append(errors, fmt.Errorf("Error syncing managed cluster action for cluster %s: %v", managedCluster.Name, err))
		} else {
			managedClusterActions = append(managedClusterActions, resp)
		}
	}
	if len(errors) > 0 {
		return nil, false, "", fmt.Errorf("One or more errors syncing managed cluster actions: %v", errors)
	}
	return managedClusterActions, true, "", nil
}
