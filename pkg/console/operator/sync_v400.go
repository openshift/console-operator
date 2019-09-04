package operator

import (
	"errors"
	"fmt"
	"os"

	// kube
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"

	// operator
	customerrors "github.com/openshift/console-operator/pkg/console/errors"
	configmapsub "github.com/openshift/console-operator/pkg/console/subresource/configmap"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
	oauthsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"
	servicesub "github.com/openshift/console-operator/pkg/console/subresource/service"
)

// The sync loop starts from zero and works its way through the requirements for a running console.
// If at any point something is missing, it creates/updates that piece and immediately dies.
// The next loop will pick up where they previous left off and move the process forward one step.
// This ensures the logic is simpler as we do not have to handle coordination between objects within
// the loop.
func (co *consoleOperator) sync_v400(updatedOperatorConfig *operatorv1.Console, set configSet) error {
	klog.V(4).Infoln("running sync loop 4.0.0")

	// track changes, may trigger ripples & update operator config or console config status
	toUpdate := false

	rt, rtChanged, rtErrReason, rtErr := co.SyncRoute(set.Operator)
	toUpdate = toUpdate || rtChanged
	co.HandleProgressingOrDegraded(updatedOperatorConfig, "RouteSync", rtErrReason, rtErr)
	if rtErr != nil {
		return rtErr
	}

	svc, svcChanged, svcErrReason, svcErr := co.SyncService(set.Operator)
	toUpdate = toUpdate || svcChanged
	co.HandleProgressingOrDegraded(updatedOperatorConfig, "ServiceSync", svcErrReason, svcErr)
	if svcErr != nil {
		return svcErr
	}

	cm, cmChanged, cmErrReason, cmErr := co.SyncConfigMap(set.Operator, set.Console, set.Infrastructure, rt)
	toUpdate = toUpdate || cmChanged
	co.HandleProgressingOrDegraded(updatedOperatorConfig, "ConfigMapSync", cmErrReason, cmErr)
	if cmErr != nil {
		return cmErr
	}

	serviceCAConfigMap, serviceCAChanged, serviceCAErrReason, serviceCAErr := co.SyncServiceCAConfigMap(set.Operator)
	toUpdate = toUpdate || serviceCAChanged
	co.HandleProgressingOrDegraded(updatedOperatorConfig, "ServiceCASync", serviceCAErrReason, serviceCAErr)
	if serviceCAErr != nil {
		return serviceCAErr
	}

	trustedCAConfigMap, trustedCAConfigMapChanged, trustedCAErrReason, trustedCAErr := co.SyncTrustedCAConfigMap(set.Operator)
	toUpdate = toUpdate || trustedCAConfigMapChanged
	co.HandleProgressingOrDegraded(updatedOperatorConfig, "TrustedCASync", trustedCAErrReason, trustedCAErr)
	if trustedCAErr != nil {
		return trustedCAErr
	}

	// TODO: why is this missing a toUpdate change?
	customLogoCanMount, customLogoErrReason, customLogoError := co.SyncCustomLogoConfigMap(updatedOperatorConfig)
	// If the custom logo sync fails for any reason, we are degraded, not progressing.
	// The sync loop may not settle, we are unable to honor it in current state.
	co.HandleProgressingOrDegraded(updatedOperatorConfig, "CustomLogoSync", customLogoErrReason, customLogoError)

	sec, secChanged, secErr := co.SyncSecret(set.Operator)
	toUpdate = toUpdate || secChanged
	co.HandleProgressingOrDegraded(updatedOperatorConfig, "OAuthClientSecretSync", "FailedApply", secErr)
	if secErr != nil {
		return secErr
	}

	oauthClient, oauthChanged, oauthErrReason, oauthErr := co.SyncOAuthClient(set.Operator, sec, rt)
	toUpdate = toUpdate || oauthChanged
	co.HandleProgressingOrDegraded(updatedOperatorConfig, "OAuthClientSync", oauthErrReason, oauthErr)
	if oauthErr != nil {
		return oauthErr
	}

	actualDeployment, depChanged, depErrReason, depErr := co.SyncDeployment(set.Operator, cm, serviceCAConfigMap, trustedCAConfigMap, sec, rt, set.Proxy, customLogoCanMount)
	toUpdate = toUpdate || depChanged
	co.HandleProgressingOrDegraded(updatedOperatorConfig, "DeploymentSync", depErrReason, depErr)
	if depErr != nil {
		return depErr
	}

	resourcemerge.SetDeploymentGeneration(&updatedOperatorConfig.Status.Generations, actualDeployment)
	updatedOperatorConfig.Status.ObservedGeneration = set.Operator.ObjectMeta.Generation

	klog.V(4).Infoln("-----------------------")
	klog.V(4).Infof("sync loop 4.0.0 resources updated: %v", toUpdate)
	klog.V(4).Infoln("-----------------------")

	co.HandleProgressing(updatedOperatorConfig, "SyncLoopRefresh", "InProgress", func() error {
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
	}())

	co.HandleAvailable(func() (conf *operatorv1.Console, prefix string, reason string, err error) {
		prefix = "Deployment"
		if !deploymentsub.IsReady(actualDeployment) {
			return updatedOperatorConfig, prefix, "InsufficientReplicas", errors.New(fmt.Sprintf("%v pods available for console deployment", actualDeployment.Status.ReadyReplicas))
		}
		if !deploymentsub.IsReadyAndUpdated(actualDeployment) {
			return updatedOperatorConfig, prefix, "FailedUpdate", errors.New(fmt.Sprintf("%v replicas ready at version %s", actualDeployment.Status.ReadyReplicas, os.Getenv("RELEASE_VERSION")))
		}
		return updatedOperatorConfig, prefix, "", nil
	}())

	co.HandleAvailable(updatedOperatorConfig, "Route", "FailedAdmittedIngress", func() error {
		if !routesub.IsAdmitted(rt) {
			return errors.New("console route is not admitted")
		}
		return nil
	}())

	// if we survive the gauntlet, we need to update the console config with the
	// public hostname so that the world can know the console is ready to roll
	klog.V(4).Infoln("sync_v400: updating console status")
	consoleURL := getConsoleURL(rt)
	if consoleURL == "" {
		err := customerrors.NewSyncError("waiting on route host")
		klog.Errorf("%q: %v", "route", err)
		return err
	}

	if _, err := co.SyncConsoleConfig(set.Console, consoleURL); err != nil {
		klog.Errorf("could not update console config status: %v", err)
		return err
	}

	if _, _, err := co.SyncConsolePublicConfig(consoleURL); err != nil {
		klog.Errorf("could not update public console config status: %v", err)
		return err
	}

	defer func() {
		klog.V(4).Infof("sync loop 4.0.0 complete")
		if svcChanged {
			klog.V(4).Infof("\t service changed: %v", svc.GetResourceVersion())
		}
		if rtChanged {
			klog.V(4).Infof("\t route changed: %v", rt.GetResourceVersion())
		}
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

	return nil
}

func (co *consoleOperator) SyncConsoleConfig(consoleConfig *configv1.Console, consoleURL string) (*configv1.Console, error) {
	updated := consoleConfig.DeepCopy()
	if updated.Status.ConsoleURL != consoleURL {
		klog.V(4).Infof("updating console.config.openshift.io with url: %v", consoleURL)
		updated.Status.ConsoleURL = consoleURL
	}
	return co.consoleConfigClient.UpdateStatus(updated)
}

func (co *consoleOperator) SyncConsolePublicConfig(consoleURL string) (*corev1.ConfigMap, bool, error) {
	requiredConfigMap := configmapsub.DefaultPublicConfig(consoleURL)
	return resourceapply.ApplyConfigMap(co.configMapClient, co.recorder, requiredConfigMap)
}

func (co *consoleOperator) SyncDeployment(
	operatorConfig *operatorv1.Console,
	cm *corev1.ConfigMap,
	serviceCAConfigMap *corev1.ConfigMap,
	trustedCAConfigMap *corev1.ConfigMap,
	sec *corev1.Secret,
	rt *routev1.Route,
	proxyConfig *configv1.Proxy,
	canMountCustomLogo bool) (consoleDeployment *appsv1.Deployment, changed bool, reason string, err error) {

	requiredDeployment := deploymentsub.DefaultDeployment(operatorConfig, cm, serviceCAConfigMap, trustedCAConfigMap, sec, rt, proxyConfig, canMountCustomLogo)
	expectedGeneration := getDeploymentGeneration(co)
	genChanged := operatorConfig.ObjectMeta.Generation != operatorConfig.Status.ObservedGeneration

	if genChanged {
		klog.V(4).Infof("deployment generation changed from %d to %d", operatorConfig.ObjectMeta.Generation, operatorConfig.Status.ObservedGeneration)
	}
	deploymentsub.LogDeploymentAnnotationChanges(co.deploymentClient, requiredDeployment)

	deployment, deploymentChanged, applyDepErr := resourceapply.ApplyDeployment(
		co.deploymentClient,
		co.recorder,
		requiredDeployment,
		expectedGeneration,
		// redeploy on operatorConfig.spec changes
		genChanged,
	)

	if applyDepErr != nil {
		return nil, false, "FailedApply", applyDepErr
	}
	return deployment, deploymentChanged, "", nil
}

// applies changes to the oauthclient
// should not be called until route & secret dependencies are verified
func (co *consoleOperator) SyncOAuthClient(operatorConfig *operatorv1.Console, sec *corev1.Secret, rt *routev1.Route) (consoleoauthclient *oauthv1.OAuthClient, changed bool, reason string, err error) {
	host := routesub.GetCanonicalHost(rt)
	if host == "" {
		return nil, false, "FailedHost", customerrors.NewSyncError("waiting on route host")
	}
	oauthClient, err := co.oauthClient.OAuthClients().Get(oauthsub.Stub().Name, metav1.GetOptions{})
	if err != nil {
		// at this point we must die & wait for someone to fix the lack of an outhclient. there is nothing we can do.
		return nil, false, "FailedGet", errors.New(fmt.Sprintf("oauth client for console does not exist and cannot be created (%v)", err))
	}
	oauthsub.RegisterConsoleToOAuthClient(oauthClient, host, secretsub.GetSecretString(sec))
	oauthClient, oauthChanged, oauthErr := oauthsub.CustomApplyOAuth(co.oauthClient, oauthClient)
	if oauthErr != nil {
		return nil, false, "FailedRegister", oauthErr
	}
	return oauthClient, oauthChanged, "", nil
}

func (co *consoleOperator) SyncSecret(operatorConfig *operatorv1.Console) (*corev1.Secret, bool, error) {
	secret, err := co.secretsClient.Secrets(api.TargetNamespace).Get(secretsub.Stub().Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) || secretsub.GetSecretString(secret) == "" {
		return resourceapply.ApplySecret(co.secretsClient, co.recorder, secretsub.DefaultSecret(operatorConfig, crypto.Random256BitsString()))
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
	operatorConfig *operatorv1.Console,
	consoleConfig *configv1.Console,
	infrastructureConfig *configv1.Infrastructure,
	rt *routev1.Route) (consoleConfigMap *corev1.ConfigMap, changed bool, reason string, err error) {

	managedConfig, mcErr := co.configMapClient.ConfigMaps(api.OpenShiftConfigManagedNamespace).Get(api.OpenShiftConsoleConfigMapName, metav1.GetOptions{})
	if mcErr != nil && !apierrors.IsNotFound(mcErr) {
		return nil, false, "FailedManagedConfig", mcErr
	}

	defaultConfigmap, _, err := configmapsub.DefaultConfigMap(operatorConfig, consoleConfig, managedConfig, infrastructureConfig, rt)
	if err != nil {
		return nil, false, "FailedConsoleConfigBuilder", err
	}
	cm, cmChanged, cmErr := resourceapply.ApplyConfigMap(co.configMapClient, co.recorder, defaultConfigmap)
	if cmErr != nil {
		return nil, false, "FailedApply", cmErr
	}
	if cmChanged {
		klog.V(4).Infoln("new console config yaml:")
		klog.V(4).Infof("%s", cm.Data)
	}
	return cm, cmChanged, "ConsoleConfigBuilder", cmErr
}

// apply service-ca configmap
func (co *consoleOperator) SyncServiceCAConfigMap(operatorConfig *operatorv1.Console) (consoleCM *corev1.ConfigMap, changed bool, reason string, err error) {
	required := configmapsub.DefaultServiceCAConfigMap(operatorConfig)
	// we can't use `resourceapply.ApplyConfigMap` since it compares data, and the service serving cert operator injects the data
	existing, err := co.configMapClient.ConfigMaps(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := co.configMapClient.ConfigMaps(required.Namespace).Create(required)
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

	actual, err := co.configMapClient.ConfigMaps(required.Namespace).Update(existing)
	if err == nil {
		klog.V(4).Infoln("service-ca configmap updated")
		return actual, true, "", err
	} else {
		return actual, true, "FailedUpdate", err
	}
}

func (co *consoleOperator) SyncTrustedCAConfigMap(operatorConfig *operatorv1.Console) (trustedCA *corev1.ConfigMap, changed bool, reason string, err error) {
	required := configmapsub.DefaultTrustedCAConfigMap(operatorConfig)
	existing, err := co.configMapClient.ConfigMaps(required.Namespace).Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := co.configMapClient.ConfigMaps(required.Namespace).Create(required)
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

	actual, err := co.configMapClient.ConfigMaps(required.Namespace).Update(existing)
	if err != nil {
		return actual, true, "FailedUpdate", err
	}
	klog.V(4).Infoln("trusted-ca-bundle configmap updated")
	return actual, true, "", err
}

// apply service
// there is nothing special about our service, so no additional error handling is needed here.
func (co *consoleOperator) SyncService(operatorConfig *operatorv1.Console) (consoleService *corev1.Service, changed bool, reason string, err error) {
	svc, svcChanged, svcErr := resourceapply.ApplyService(co.serviceClient, co.recorder, servicesub.DefaultService(operatorConfig))
	if svcErr != nil {
		return nil, false, "FailedApply", svcErr
	}
	return svc, svcChanged, "", svcErr
}

// apply route
// - be sure to test that we don't trigger an infinite loop by stomping on the
//   default host name set by the server, or any other values. The ApplyRoute()
//   logic will have to be sound.
// - update to ApplyRoute() once the logic is settled
func (co *consoleOperator) SyncRoute(operatorConfig *operatorv1.Console) (consoleRoute *routev1.Route, isNew bool, reason string, err error) {
	// ensure we have a route. any error returned is a non-404 error
	rt, rtIsNew, rtErr := routesub.GetOrCreate(co.routeClient, routesub.DefaultRoute(operatorConfig))
	if rtErr != nil {
		return nil, false, "FailedCreate", rtErr
	}

	// we will not proceed until the route is valid. this eliminates complexity with the
	// configmap, secret & oauth client as they can be certain they have a host if we pass this point.
	host := routesub.GetCanonicalHost(rt)
	if len(host) == 0 {
		return nil, false, "FailedHost", customerrors.NewSyncError(fmt.Sprintf("route is not available at canonical host %s", rt.Status.Ingress))
	}

	if validatedRoute, changed := routesub.Validate(rt); changed {
		// if validation changed the route, issue an update
		if _, err := co.routeClient.Routes(api.TargetNamespace).Update(validatedRoute); err != nil {
			// error is unexpected, this is a real error
			return nil, false, "InvalidRouteCorrection", err
		}
		// abort sync, route changed, let it settle & retry
		return nil, true, "InvalidRoute", customerrors.NewSyncError("route is invalid, correcting route state")
	}
	// only return the route if it is valid with a host
	return rt, rtIsNew, "", rtErr
}

func (c *consoleOperator) SyncCustomLogoConfigMap(operatorConfig *operatorsv1.Console) (okToMount bool, reason string, err error) {
	// validate first, to avoid a broken volume mount & a crashlooping console
	okToMount, reason, err = c.ValidateCustomLogo(operatorConfig)

	if okToMount || configmapsub.IsRemoved(operatorConfig) {
		if err := c.UpdateCustomLogoSyncSource(operatorConfig); err != nil {
			return false, "FailedSyncSource", customerrors.NewCustomLogoError("custom logo sync source update error")
		}
	}
	return okToMount, reason, err
}

// on each pass of the operator sync loop, we need to check the
// operator config for a custom logo.  If this has been set, then
// we notify the resourceSyncer that it needs to start watching this
// configmap in its own sync loop.  Note that the resourceSyncer's actual
// sync loop will run later.  Our operator is waiting to receive
// the copied configmap into the console namespace for a future
// sync loop to mount into the console deployment.
func (c *consoleOperator) UpdateCustomLogoSyncSource(operatorConfig *operatorsv1.Console) error {
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

func (c *consoleOperator) ValidateCustomLogo(operatorConfig *operatorsv1.Console) (okToMount bool, reason string, err error) {
	logoConfigMapName := operatorConfig.Spec.Customization.CustomLogoFile.Name
	logoImageKey := operatorConfig.Spec.Customization.CustomLogoFile.Key

	// inconsistency, error, no mount
	if configmapsub.FileNameOrKeyInconsistentlySet(operatorConfig) {
		klog.Infoln(">>>>> FileNameOrKeyInconsistentlySet >>>>>")
		klog.V(4).Infoln("custom logo filename or key have not been set")
		return false, "KeyOrFilenameInvalid", customerrors.NewCustomLogoError("either custom logo filename or key have not been set")
	}
	// if not set, feature is not used. no error, no mount
	if configmapsub.FileNameNotSet(operatorConfig) {
		klog.Infoln(">>>>> not configured >>>>>")
		klog.V(4).Infoln("no custom logo configured")
		return false, "", nil
	}

	// original user-defined logo in openshift-config
	_, err = c.configMapClient.ConfigMaps(api.OpenShiftConfigNamespace).Get(logoConfigMapName, metav1.GetOptions{})
	// 404, wait for user to create. error, no mount
	if apierrors.IsNotFound(err) {
		klog.Infoln(">>>>> source not found >>>>>")
		msg := fmt.Sprintf("source custom logo file %v in openshift-config not found", logoConfigMapName)
		klog.V(4).Infof(msg)
		return false, "FailedGetSource", customerrors.NewCustomLogoError(msg)
	}
	// unknown error, no mount
	if err != nil {
		klog.Infoln(">>>>> source other error >>>>>")
		return false, "SourceError", customerrors.NewCustomLogoError(fmt.Sprintf("custom logo: %v\n", err))
	}

	// sync'd logo configmap into openshift-console namespace
	logoConfigMap, err := c.configMapClient.ConfigMaps(api.OpenShiftConsoleNamespace).Get(api.OpenShiftCustomLogoConfigMapName, metav1.GetOptions{})
	// 404, no mount, error
	if apierrors.IsNotFound(err) {
		klog.Infoln(">>>>> destination not found >>>>>")
		msg := fmt.Sprintf("destination custom logo file %v in openshift-console not found", logoConfigMapName)
		klog.V(4).Infof(msg)
		return false, "FailedGetDestination", customerrors.NewCustomLogoError(msg)
	}
	// unknown error, no mount
	if err != nil {
		klog.Infoln(">>>>> destination other error >>>>>")
		return false, "DestinationError", customerrors.NewCustomLogoError(fmt.Sprintf("custom logo: %v\n", err))
	}
	// we need to test bytes in the destination, not the source
	imageBytes := logoConfigMap.BinaryData[logoImageKey]
	if configmapsub.LogoImageIsEmpty(imageBytes) {
		klog.Infoln(">>>>> logo image empty >>>>>")
		klog.V(4).Infoln("custom logo file exists but no image provided")
		return false, "NoImageProvided", customerrors.NewCustomLogoError("custom logo file exists but no image provided")
	}
	// we will mount it anyway, but should notify the user if doesn't look right
	if !configmapsub.IsLikelyCommonImageFormat(imageBytes) {
		klog.Infoln(">>>>> not a common format >>>>>")
		klog.V(4).Infoln("custom logo does not appear to be a common image format")
		return true, "UncommonImageFormat", customerrors.NewCustomLogoError("custom logo does not appear to be a common image format")
	}

	klog.Infoln(">>>>> yay made it look at that! >>>>>")
	klog.V(4).Infoln("custom logo ok to mount")
	return true, "", nil
}

func getDeploymentGeneration(co *consoleOperator) int64 {
	deployment, err := co.deploymentClient.Deployments(api.TargetNamespace).Get(deploymentsub.Stub().Name, metav1.GetOptions{})
	if err != nil {
		return -1
	}
	return deployment.Generation
}

func getConsoleURL(route *routev1.Route) string {
	host := routesub.GetCanonicalHost(route)
	if host == "" {
		return ""
	}
	return util.HTTPS(host)
}
