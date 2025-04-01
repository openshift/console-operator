package operator

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"

	// kube

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/library-go/pkg/controller/factory"

	// operator
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/status"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"
)

// The sync loop starts from zero and works its way through the requirements for a running console.
// If at any point something is missing, it creates/updates that piece and immediately dies.
// The next loop will pick up where they previous left off and move the process forward one step.
// This ensures the logic is simpler as we do not have to handle coordination between objects within
// the loop.
func (co *consoleOperator) sync(ctx context.Context, controllerContext factory.SyncContext, updatedOperatorConfig *operatorv1.Console, set configSet) error {
	klog.V(4).Infoln("running sync loop 4.0.0")

	var (
		statusHandler = status.NewStatusHandler(co.operatorClient)
		// track changes, may trigger ripples & update operator config or console config status
		toUpdate     = false
		consoleRoute *routev1.Route
		consoleURL   *url.URL
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
		consoleRoute,
		controllerContext.Recorder(),
		consoleURL.Hostname(),
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
