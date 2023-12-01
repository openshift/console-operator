package oauthclients

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"

	operatorv1 "github.com/openshift/api/operator/v1"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions/config/v1"
	configv1lister "github.com/openshift/client-go/config/listers/config/v1"
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
	"github.com/openshift/console-operator/pkg/crypto"
)

type oauthClientsController struct {
	oauthClient    oauthv1client.OAuthClientsGetter
	operatorClient v1helpers.OperatorClient
	secretsClient  corev1client.SecretsGetter

	oauthClientLister     oauthv1lister.OAuthClientLister
	consoleOperatorLister operatorv1listers.ConsoleLister
	routesLister          routev1listers.RouteLister
	ingressConfigLister   configv1lister.IngressLister
	targetNSSecretsLister corev1listers.SecretLister

	statusHandler status.StatusHandler
}

func NewOAuthClientsController(
	operatorClient v1helpers.OperatorClient,
	oauthClient oauthv1client.OAuthClientsGetter,
	secretsClient corev1client.SecretsGetter,
	oauthClientInformer oauthv1informers.OAuthClientInformer,
	consoleOperatorInformer operatorv1informers.ConsoleInformer,
	routeInformer routev1informers.RouteInformer,
	ingressConfigInformer configv1informers.IngressInformer,
	targetNSsecretsInformer corev1informers.SecretInformer,
	recorder events.Recorder,
) factory.Controller {
	c := oauthClientsController{
		oauthClient:    oauthClient,
		operatorClient: operatorClient,
		secretsClient:  secretsClient,

		oauthClientLister:     oauthClientInformer.Lister(),
		consoleOperatorLister: consoleOperatorInformer.Lister(),
		routesLister:          routeInformer.Lister(),
		ingressConfigLister:   ingressConfigInformer.Lister(),
		targetNSSecretsLister: targetNSsecretsInformer.Lister(),

		statusHandler: status.NewStatusHandler(operatorClient),
	}

	return factory.New().
		WithSync(c.sync).
		WithInformers(
			consoleOperatorInformer.Informer(),
			routeInformer.Informer(),
			ingressConfigInformer.Informer(),
			targetNSsecretsInformer.Informer(),
		).
		WithFilteredEventsInformers(
			factory.NamesFilter(api.OAuthClientName),
			oauthClientInformer.Informer(),
		).
		WithSyncDegradedOnError(operatorClient).
		ToController("OAuthClientsController", recorder.WithComponentSuffix("oauth-clients-controller"))
}

func (c *oauthClientsController) sync(ctx context.Context, controllerContext factory.SyncContext) error {
	if shouldSync, err := c.handleStatus(ctx); err != nil {
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

	routeName := api.OpenShiftConsoleRouteName
	routeConfig := routesub.NewRouteConfig(operatorConfig, ingressConfig, routeName)
	if routeConfig.IsCustomHostnameSet() {
		routeName = api.OpenshiftConsoleCustomRouteName
	}

	_, consoleURL, _, routeErr := routesub.GetActiveRouteInfo(c.routesLister, routeName)
	if routeErr != nil {
		return routeErr
	}

	clientSecret, secErr := c.syncSecret(ctx, operatorConfig, controllerContext.Recorder())
	c.statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSecretSync", "FailedApply", secErr))
	if secErr != nil {
		return c.statusHandler.FlushAndReturn(secErr)
	}

	oauthErrReason, oauthErr := c.syncOAuthClient(ctx, clientSecret, consoleURL.String())
	c.statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSync", oauthErrReason, oauthErr))

	return c.statusHandler.FlushAndReturn(oauthErr)
}

// handleStatus returns whether sync should happen and any error encountering
// determining the operator's management state
// TODO: extract this logic to where it can be used for all controllers
func (c *oauthClientsController) handleStatus(ctx context.Context) (bool, error) {
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
