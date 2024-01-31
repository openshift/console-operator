package oauthclients

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions/config/v1"
	configv1lister "github.com/openshift/client-go/config/listers/config/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned"
	oauthv1client "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	oauthv1lister "github.com/openshift/client-go/oauth/listers/oauth/v1"
	operatorv1informers "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	operatorv1listers "github.com/openshift/client-go/operator/listers/operator/v1"
	routev1informers "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	routev1listers "github.com/openshift/client-go/route/listers/route/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
	oauthsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"
	utilsub "github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/console-operator/pkg/crypto"
)

type oauthClientsController struct {
	oauthClient    oauthv1client.OAuthClientsGetter
	operatorClient v1helpers.OperatorClient
	secretsClient  corev1client.SecretsGetter

	oauthClientLister           oauthv1lister.OAuthClientLister
	oauthClientSwitchedInformer *util.InformerWithSwitch
	authentication              configv1client.AuthenticationInterface
	authnLister                 configv1lister.AuthenticationLister
	consoleOperatorLister       operatorv1listers.ConsoleLister
	routesLister                routev1listers.RouteLister
	ingressConfigLister         configv1lister.IngressLister
	targetNSSecretsLister       corev1listers.SecretLister
	configNSSecretsLister       corev1listers.SecretLister
	targetNSDeploymentsLister   appsv1listers.DeploymentLister
	targetNSConfigLister        corev1listers.ConfigMapLister
	featureGatesLister          configv1lister.FeatureGateLister

	authStatusHandler status.AuthStatusHandler
}

func NewOAuthClientsController(
	ctx context.Context,
	operatorClient v1helpers.OperatorClient,
	oauthClient oauthclient.Interface,
	secretsClient corev1client.SecretsGetter,
	authentication configv1client.AuthenticationInterface,
	authnInformer configv1informers.AuthenticationInformer,
	consoleOperatorInformer operatorv1informers.ConsoleInformer,
	routeInformer routev1informers.RouteInformer,
	ingressConfigInformer configv1informers.IngressInformer,
	targetNSsecretsInformer corev1informers.SecretInformer,
	configNSSecretsInformer corev1informers.SecretInformer,
	targetNSConfigInformer corev1informers.ConfigMapInformer,
	targetNSDeploymentsInformer appsv1informers.DeploymentInformer,
	oauthClientSwitchedInformer *util.InformerWithSwitch,
	featureGatesInformer configv1informers.FeatureGateInformer,
	recorder events.Recorder,
) factory.Controller {
	c := oauthClientsController{
		oauthClient:    oauthClient.OauthV1(),
		operatorClient: operatorClient,
		secretsClient:  secretsClient,

		oauthClientLister:           oauthClientSwitchedInformer.Lister(),
		oauthClientSwitchedInformer: oauthClientSwitchedInformer,
		authentication:              authentication,
		authnLister:                 authnInformer.Lister(),
		consoleOperatorLister:       consoleOperatorInformer.Lister(),
		routesLister:                routeInformer.Lister(),
		ingressConfigLister:         ingressConfigInformer.Lister(),
		targetNSSecretsLister:       targetNSsecretsInformer.Lister(),
		configNSSecretsLister:       configNSSecretsInformer.Lister(),
		targetNSConfigLister:        targetNSConfigInformer.Lister(),
		targetNSDeploymentsLister:   targetNSDeploymentsInformer.Lister(),
		featureGatesLister:          featureGatesInformer.Lister(),

		authStatusHandler: status.NewAuthStatusHandler(authentication, api.OpenShiftConsoleName, api.TargetNamespace, api.OpenShiftConsoleOperator),
	}

	return factory.New().
		WithSync(c.sync).
		WithInformers(
			authnInformer.Informer(),
			consoleOperatorInformer.Informer(),
			routeInformer.Informer(),
			ingressConfigInformer.Informer(),
			targetNSsecretsInformer.Informer(),
			configNSSecretsInformer.Informer(),
			targetNSDeploymentsInformer.Informer(),
		).
		WithFilteredEventsInformers(
			factory.NamesFilter(api.OAuthClientName),
			oauthClientSwitchedInformer.Informer(),
		).
		WithSyncDegradedOnError(operatorClient).
		ResyncEvery(wait.Jitter(time.Minute, 1.0)).
		ToController("OAuthClientsController", recorder.WithComponentSuffix("oauth-clients-controller"))
}

func (c *oauthClientsController) sync(ctx context.Context, controllerContext factory.SyncContext) error {
	statusHandler := status.NewStatusHandler(c.operatorClient)

	if shouldSync, err := c.handleManaged(ctx); err != nil {
		return err
	} else if !shouldSync {
		return nil
	}

	operatorConfig, err := c.consoleOperatorLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}

	ingressConfig, err := c.ingressConfigLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}

	authnConfig, err := c.authnLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}

	featureGates, err := c.featureGatesLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}

	routeName := api.OpenShiftConsoleRouteName
	routeConfig := routesub.NewRouteConfig(operatorConfig, ingressConfig, routeName)
	if routeConfig.IsCustomHostnameSet() {
		routeName = api.OpenshiftConsoleCustomRouteName
	}

	_, consoleURL, _, routeErr := routesub.GetActiveRouteInfo(c.routesLister, routeName)
	if routeErr != nil {
		return routeErr
	}

	var syncErr error
	switch authnConfig.Spec.Type {
	case "", configv1.AuthenticationTypeIntegratedOAuth:
		waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if !cache.WaitForCacheSync(waitCtx.Done(), c.oauthClientSwitchedInformer.Informer().HasSynced) {
			syncErr = fmt.Errorf("timed out waiting for OAuthClients cache sync")
			break
		}

		clientSecret, secErr := c.syncSecret(ctx, operatorConfig, controllerContext.Recorder())
		statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSecretSync", "FailedApply", secErr))
		if secErr != nil {
			syncErr = secErr
			break
		}

		oauthErrReason, oauthErr := c.syncOAuthClient(ctx, clientSecret, consoleURL.String())
		statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSync", oauthErrReason, oauthErr))
		if oauthErr != nil {
			syncErr = oauthErr
			break
		}

	case configv1.AuthenticationTypeOIDC:
		syncErr = c.syncAuthTypeOIDC(ctx, controllerContext, statusHandler, operatorConfig, authnConfig)
	}

	// AuthStatusHandler manages fields that are behind the CustomNoUpgrade and TechPreviewNoUpgrade featuregate sets
	// call Apply() only if they are enabled, otherwise server-side apply will complain
	if featureGates.Spec.FeatureSet == configv1.TechPreviewNoUpgrade || featureGates.Spec.FeatureSet == configv1.CustomNoUpgrade {
		if err := c.authStatusHandler.Apply(ctx, authnConfig); err != nil {
			statusHandler.AddConditions(status.HandleProgressingOrDegraded("AuthStatusHandler", "FailedApply", err))
			return statusHandler.FlushAndReturn(err)
		}
	}

	return statusHandler.FlushAndReturn(syncErr)
}

func (c *oauthClientsController) syncAuthTypeOIDC(
	ctx context.Context,
	controllerContext factory.SyncContext,
	statusHandler status.StatusHandler,
	operatorConfig *operatorv1.Console,
	authnConfig *configv1.Authentication,
) error {
	clientConfig := utilsub.GetOIDCClientConfig(authnConfig)
	if clientConfig == nil {
		c.authStatusHandler.WithCurrentOIDCClient("")
		c.authStatusHandler.Unavailable("OIDCClientConfig", "no OIDC client found")
		return nil
	}

	if len(clientConfig.ClientID) == 0 {
		err := fmt.Errorf("no ID set on OIDC client")
		statusHandler.AddConditions(status.HandleProgressingOrDegraded("OIDCClientConfig", "MissingID", err))
		return statusHandler.FlushAndReturn(err)
	}
	c.authStatusHandler.WithCurrentOIDCClient(clientConfig.ClientID)

	if len(clientConfig.ClientSecret.Name) == 0 {
		c.authStatusHandler.Degraded("OIDCClientMissingSecret", "no client secret in the OIDC client config")
		return nil
	}

	clientSecret, err := c.configNSSecretsLister.Secrets(api.OpenShiftConfigNamespace).Get(clientConfig.ClientSecret.Name)
	if err != nil {
		c.authStatusHandler.Degraded("OIDCClientSecretGet", err.Error())
		return err
	}

	secret, err := c.targetNSSecretsLister.Secrets(api.TargetNamespace).Get(secretsub.Stub().Name)
	expectedClientSecret := secretsub.GetSecretString(clientSecret)
	if apierrors.IsNotFound(err) || secretsub.GetSecretString(secret) != expectedClientSecret {
		secret, _, err = resourceapply.ApplySecret(ctx, c.secretsClient, controllerContext.Recorder(), secretsub.DefaultSecret(operatorConfig, expectedClientSecret))
		if err != nil {
			statusHandler.AddConditions(status.HandleProgressingOrDegraded("OIDCClientSecretSync", "FailedApply", err))
			return err
		}
	}

	if valid, msg, err := c.checkClientConfigStatus(authnConfig, secret); err != nil {
		c.authStatusHandler.Degraded("DeploymentOIDCConfig", err.Error())
		return err

	} else if !valid {
		c.authStatusHandler.Progressing("DeploymentOIDCConfig", msg)
		return nil
	}

	c.authStatusHandler.Available("OIDCConfigAvailable", "")
	return nil
}

// checkClientConfigStatus checks whether the current client configuration is being currently in use,
// by looking at the deployment status. It checks whether the deployment is available and updated,
// and also whether the resource versions for the oauth secret and server CA trust configmap match
// the deployment.
func (c *oauthClientsController) checkClientConfigStatus(authnConfig *configv1.Authentication, clientSecret *corev1.Secret) (bool, string, error) {
	depl, err := c.targetNSDeploymentsLister.Deployments(api.OpenShiftConsoleNamespace).Get(api.OpenShiftConsoleDeploymentName)
	if err != nil {
		return false, "", err
	}

	deplAvailableUpdated := deploymentsub.IsAvailableAndUpdated(depl)
	if !deplAvailableUpdated {
		return false, "deployment unavailable or outdated", nil
	}

	if clientSecret.GetResourceVersion() != depl.ObjectMeta.Annotations["console.openshift.io/oauth-secret-version"] {
		return false, "client secret version not up to date in current deployment", nil
	}

	if len(authnConfig.Spec.OIDCProviders) > 0 {
		serverCAConfigName := authnConfig.Spec.OIDCProviders[0].Issuer.CertificateAuthority.Name
		if len(serverCAConfigName) == 0 {
			return deplAvailableUpdated, "", nil
		}

		serverCAConfig, err := c.targetNSConfigLister.ConfigMaps(api.OpenShiftConsoleNamespace).Get(serverCAConfigName)
		if err != nil {
			return false, "", err
		}

		if serverCAConfig.GetResourceVersion() != depl.ObjectMeta.Annotations["console.openshift.io/authn-ca-trust-config-version"] {
			return false, "OIDC provider CA version not up to date in current deployment", nil
		}
	}

	return deplAvailableUpdated, "", nil
}

// handleStatus returns whether sync should happen and any error encountering
// determining the operator's management state
// TODO: extract this logic to where it can be used for all controllers
func (c *oauthClientsController) handleManaged(ctx context.Context) (bool, error) {
	operatorSpec, _, _, err := c.operatorClient.GetOperatorState()
	if err != nil {
		return false, fmt.Errorf("failed to retrieve operator config: %w", err)
	}

	switch managementState := operatorSpec.ManagementState; managementState {
	case operatorv1.Managed:
		klog.V(4).Infoln("console is in a managed state.")
		return true, nil
	case operatorv1.Unmanaged:
		klog.V(4).Infoln("console is in an unmanaged state.")
		return false, nil
	case operatorv1.Removed:
		klog.V(4).Infoln("console has been removed.")
		return false, c.deregisterClient(ctx)
	default:
		return false, fmt.Errorf("console is in an unknown state: %v", managementState)
	}
}

func (c *oauthClientsController) syncSecret(ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder) (*corev1.Secret, error) {
	secret, err := c.targetNSSecretsLister.Secrets(api.TargetNamespace).Get(secretsub.Stub().Name)
	if apierrors.IsNotFound(err) || secretsub.GetSecretString(secret) == "" {
		secret, _, err = resourceapply.ApplySecret(ctx, c.secretsClient, recorder, secretsub.DefaultSecret(operatorConfig, crypto.Random256BitsString()))
	}
	// any error should be returned & kill the sync loop
	return secret, err
}

// applies changes to the oauthclient
// should not be called until route & secret dependencies are verified
func (c *oauthClientsController) syncOAuthClient(
	ctx context.Context,
	sec *corev1.Secret,
	consoleURL string,
) (reason string, err error) {
	oauthClient, err := c.oauthClientLister.Get(oauthsub.Stub().Name)
	if err != nil {
		// at this point we must die & wait for someone to fix the lack of an outhclient. there is nothing we can do.
		return "FailedGet", fmt.Errorf("oauth client for console does not exist and cannot be created (%w)", err)
	}
	clientCopy := oauthClient.DeepCopy()
	oauthsub.RegisterConsoleToOAuthClient(clientCopy, consoleURL, secretsub.GetSecretString(sec))
	_, _, oauthErr := oauthsub.CustomApplyOAuth(c.oauthClient, clientCopy, ctx)
	if oauthErr != nil {
		return "FailedRegister", oauthErr
	}
	return "", nil
}

func (c *oauthClientsController) deregisterClient(ctx context.Context) error {
	// existingOAuthClient is not a delete, it is a deregister/neutralize
	existingOAuthClient, err := c.oauthClientLister.Get(oauthsub.Stub().Name)
	if err != nil {
		return err
	}

	if len(existingOAuthClient.RedirectURIs) == 0 {
		return nil
	}

	updated := oauthsub.DeRegisterConsoleFromOAuthClient(existingOAuthClient.DeepCopy())
	_, err = c.oauthClient.OAuthClients().Update(ctx, updated, metav1.UpdateOptions{})
	return err

}
