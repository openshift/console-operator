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
	"github.com/openshift/console-operator/pkg/console/operator"
	"github.com/openshift/console-operator/pkg/console/status"
	oauthsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"
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
		).
		WithSyncDegradedOnError(operatorClient).
		ToController("OAuthClientsController", recorder.WithComponentSuffix("oauth-clients-controller"))
}

func (c *oauthClientsController) sync(ctx context.Context, controllerContext factory.SyncContext) error {
	statusHandler := status.NewStatusHandler(c.operatorClient)

	operatorConfig, err := c.consoleOperatorLister.Get("cluster")
	if err != nil {
		return err
	}

	ingressConfig, err := c.ingressConfigLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}

	authnConfig, err := c.authnLister.Get("cluster")
	if err != nil {
		return err
	}

	routeName := api.OpenShiftConsoleRouteName
	routeConfig := routesub.NewRouteConfig(operatorConfig, ingressConfig, routeName)
	if routeConfig.IsCustomHostnameSet() {
		routeName = api.OpenshiftConsoleCustomRouteName
	}

	_, consoleURL, _, routeErr := operator.GetActiveRouteInfo(ctx, c.routesLister, routeName)
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

		clientSecret, _, secErr := c.syncSecret(ctx, operatorConfig, controllerContext.Recorder())
		statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSecretSync", "FailedApply", secErr))
		if secErr != nil {
			return statusHandler.FlushAndReturn(secErr)
		}

		oauthErrReason, oauthErr := c.syncOAuthClient(ctx, clientSecret, consoleURL.String())
		statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSync", oauthErrReason, oauthErr))
		if oauthErr != nil {
			return statusHandler.FlushAndReturn(oauthErr)
		}
		return nil
	case "OIDC":
		c.oauthClientsInformerSwitch.Stop()
		return nil
	default:
		return nil
	}
}

func (c *oauthClientsController) syncSecret(ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder) (*corev1.Secret, bool, error) {
	secret, err := c.targetNSSecretsLister.Secrets(api.TargetNamespace).Get(secretsub.Stub().Name)
	if apierrors.IsNotFound(err) || secretsub.GetSecretString(secret) == "" {
		return resourceapply.ApplySecret(ctx, c.secretsClient, recorder, secretsub.DefaultSecret(operatorConfig, crypto.Random256BitsString()))
	}
	// any error should be returned & kill the sync loop
	if err != nil {
		return nil, false, err
	}
	return secret, false, nil
}

// applies changes to the oauthclient
// should not be called until route & secret dependencies are verified
func (c *oauthClientsController) syncOAuthClient(
	ctx context.Context,
	sec *corev1.Secret,
	consoleURL string,
) (reason string, err error) {
	oauthClient, err := c.oauthClient.OAuthClients().Get(ctx, oauthsub.Stub().Name, metav1.GetOptions{})
	if err != nil {
		// at this point we must die & wait for someone to fix the lack of an outhclient. there is nothing we can do.
		return "FailedGet", fmt.Errorf("oauth client for console does not exist and cannot be created (%w)", err)
	}
	oauthsub.RegisterConsoleToOAuthClient(oauthClient, consoleURL, secretsub.GetSecretString(sec))
	_, _, oauthErr := oauthsub.CustomApplyOAuth(c.oauthClient, oauthClient, ctx)
	if oauthErr != nil {
		return "FailedRegister", oauthErr
	}
	return "", nil
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