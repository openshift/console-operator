package oauthclients

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions/config/v1"
	configv1lister "github.com/openshift/client-go/config/listers/config/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned"
	oauthv1client "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	oauthv1informers "github.com/openshift/client-go/oauth/informers/externalversions/oauth/v1"
	oauthv1lister "github.com/openshift/client-go/oauth/listers/oauth/v1"
	operatorv1informers "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	operatorv1listers "github.com/openshift/client-go/operator/listers/operator/v1"
	routev1informers "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	routev1listers "github.com/openshift/client-go/route/listers/route/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/status"
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

	oauthClientLister     oauthv1lister.OAuthClientLister
	authnLister           configv1lister.AuthenticationLister
	consoleOperatorLister operatorv1listers.ConsoleLister
	routesLister          routev1listers.RouteLister
	ingressConfigLister   configv1lister.IngressLister
	targetNSSecretsLister corev1listers.SecretLister
	configNSSecretsLister corev1listers.SecretLister

	statusHandler              status.StatusHandler
	oauthClientsInformerSwitch *informerWithSwitch
}

func NewOAuthClientsController(
	ctx context.Context,
	operatorClient v1helpers.OperatorClient,
	oauthClient oauthclient.Interface,
	secretsClient corev1client.SecretsGetter,
	authnInformer configv1informers.AuthenticationInformer,
	consoleOperatorInformer operatorv1informers.ConsoleInformer,
	routeInformer routev1informers.RouteInformer,
	ingressConfigInformer configv1informers.IngressInformer,
	targetNSsecretsInformer corev1informers.SecretInformer,
	configNSSecretsInformer corev1informers.SecretInformer,
	recorder events.Recorder,
) factory.Controller {
	oauthClientInformer := oauthv1informers.NewOAuthClientInformer(oauthClient, 10*time.Minute, nil)

	c := oauthClientsController{
		oauthClient:    oauthClient.OauthV1(),
		operatorClient: operatorClient,
		secretsClient:  secretsClient,

		oauthClientLister:     oauthv1lister.NewOAuthClientLister(oauthClientInformer.GetIndexer()),
		authnLister:           authnInformer.Lister(),
		consoleOperatorLister: consoleOperatorInformer.Lister(),
		routesLister:          routeInformer.Lister(),
		ingressConfigLister:   ingressConfigInformer.Lister(),
		targetNSSecretsLister: targetNSsecretsInformer.Lister(),
		configNSSecretsLister: configNSSecretsInformer.Lister(),

		statusHandler:              status.NewStatusHandler(operatorClient),
		oauthClientsInformerSwitch: NewOAuthClientsInformerSwitch(ctx, oauthClientInformer),
	}
	defer c.oauthClientsInformerSwitch.EnsureRunning()

	return factory.New().
		WithSync(c.sync).
		WithInformers(
			authnInformer.Informer(),
			consoleOperatorInformer.Informer(),
			routeInformer.Informer(),
			ingressConfigInformer.Informer(),
			targetNSsecretsInformer.Informer(),
			configNSSecretsInformer.Informer(),
		).
		WithFilteredEventsInformers(
			factory.NamesFilter(api.OAuthClientName),
			oauthClientInformer,
		).
		WithSyncDegradedOnError(operatorClient).
		ToController("OAuthClientsController", recorder.WithComponentSuffix("oauth-clients-controller"))
}

func (c *oauthClientsController) sync(ctx context.Context, controllerContext factory.SyncContext) error {
	if shouldSync, err := c.handleManaged(ctx); err != nil {
		return err
	} else if !shouldSync {
		return nil
	}

	operatorConfig, err := c.consoleOperatorLister.Get("cluster")
	if err != nil {
		return err
	}

	ingressConfig, err := c.ingressConfigLister.Get("cluster")
	if err != nil {
		return err
	}

	authnConfig, err := c.authnLister.Get(api.ConfigResourceName)
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

	switch authnConfig.Spec.Type {
	case "", configv1.AuthenticationTypeIntegratedOAuth:
		c.oauthClientsInformerSwitch.EnsureRunning()

		waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if !cache.WaitForCacheSync(waitCtx.Done(), c.oauthClientsInformerSwitch.informer.HasSynced) {
			return fmt.Errorf("timed out waiting for OAuthClients cache sync")
		}

		clientSecret, secErr := c.syncSecret(ctx, operatorConfig, controllerContext.Recorder())
		c.statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSecretSync", "FailedApply", secErr))
		if secErr != nil {
			return c.statusHandler.FlushAndReturn(secErr)
		}

		oauthErrReason, oauthErr := c.syncOAuthClient(ctx, clientSecret, consoleURL.String())
		c.statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSync", oauthErrReason, oauthErr))
		if oauthErr != nil {
			return c.statusHandler.FlushAndReturn(oauthErr)
		}
		return nil

	case configv1.AuthenticationTypeOIDC, configv1.AuthenticationTypeNone:
		c.oauthClientsInformerSwitch.Stop()
		return c.syncAuthTypeOIDC(ctx, controllerContext, operatorConfig, authnConfig)

	default:
		return nil
	}
}

func (c *oauthClientsController) syncAuthTypeOIDC(ctx context.Context, controllerContext factory.SyncContext, operatorConfig *operatorv1.Console, authnConfig *configv1.Authentication) error {
	oidcConfig := utilsub.GetOIDCClientConfig(authnConfig)
	if oidcConfig == nil {
		klog.Error("oidc client configuration not found")
		return nil
	}

	clientSecret, err := c.configNSSecretsLister.Secrets(api.OpenShiftConfigNamespace).Get(oidcConfig.ClientSecret.Name)
	if apierrors.IsNotFound(err) {
		klog.Error("oauth client config secret not found")
		return nil
	} else if err != nil {
		return err
	}

	secret, err := c.targetNSSecretsLister.Secrets(api.TargetNamespace).Get(secretsub.Stub().Name)
	expectedClientSecret := secretsub.GetSecretString(clientSecret)
	if apierrors.IsNotFound(err) || secretsub.GetSecretString(secret) != expectedClientSecret {
		_, _, err := resourceapply.ApplySecret(ctx, c.secretsClient, controllerContext.Recorder(), secretsub.DefaultSecret(operatorConfig, expectedClientSecret))
		return err
	}
	return nil
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

type informerWithSwitch struct {
	informer  cache.SharedIndexInformer
	parentCtx context.Context
	runCtx    context.Context
	stopFunc  func()
}

func NewOAuthClientsInformerSwitch(ctx context.Context, informer cache.SharedIndexInformer) *informerWithSwitch {
	return &informerWithSwitch{
		informer:  informer,
		parentCtx: ctx,
	}
}

func (s *informerWithSwitch) EnsureRunning() {
	if s.runCtx != nil {
		return
	}

	s.runCtx, s.stopFunc = context.WithCancel(s.parentCtx)
	go s.informer.Run(s.runCtx.Done())
}

func (s *informerWithSwitch) Stop() {
	if s.runCtx == nil {
		return
	}

	s.stopFunc()
	s.runCtx = nil
	s.stopFunc = nil
}
