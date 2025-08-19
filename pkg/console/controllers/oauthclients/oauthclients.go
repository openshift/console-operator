package oauthclients

import (
	"context"
	"fmt"
	"net/url"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
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
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	oauthsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"
)

// oauthClientsController:
//
//	updates:
//	- oauthclient.oauth.openshift.io/console (creates if doesn't exist)
//	writes:
//	- consoles.operator.openshift.io/cluster .status.conditions:
//		- type=OAuthClientSyncProgressing
//		- type=OAuthClientSyncDegraded
type oauthClientsController struct {
	oauthClient    oauthv1client.OAuthClientsGetter
	operatorClient v1helpers.OperatorClient

	oauthClientLister           oauthv1lister.OAuthClientLister
	oauthClientSwitchedInformer *util.InformerWithSwitch
	authnLister                 configv1lister.AuthenticationLister
	consoleOperatorLister       operatorv1listers.ConsoleLister
	routesLister                routev1listers.RouteLister
	ingressConfigLister         configv1lister.IngressLister
	targetNSSecretsLister       corev1listers.SecretLister
	infrastructureConfigLister  configv1lister.InfrastructureLister
	clusterVersionLister        configv1lister.ClusterVersionLister
}

func NewOAuthClientsController(
	operatorClient v1helpers.OperatorClient,
	oauthClient oauthclient.Interface,
	authnInformer configv1informers.AuthenticationInformer,
	consoleOperatorInformer operatorv1informers.ConsoleInformer,
	routeInformer routev1informers.RouteInformer,
	ingressConfigInformer configv1informers.IngressInformer,
	targetNSsecretsInformer corev1informers.SecretInformer,
	configInformer configinformer.SharedInformerFactory,
	oauthClientSwitchedInformer *util.InformerWithSwitch,
	recorder events.Recorder,
) factory.Controller {
	c := oauthClientsController{
		oauthClient:    oauthClient.OauthV1(),
		operatorClient: operatorClient,

		oauthClientLister:           oauthClientSwitchedInformer.Lister(),
		oauthClientSwitchedInformer: oauthClientSwitchedInformer,
		authnLister:                 authnInformer.Lister(),
		consoleOperatorLister:       consoleOperatorInformer.Lister(),
		routesLister:                routeInformer.Lister(),
		ingressConfigLister:         ingressConfigInformer.Lister(),
		infrastructureConfigLister:  configInformer.Config().V1().Infrastructures().Lister(),
		clusterVersionLister:        configInformer.Config().V1().ClusterVersions().Lister(),
		targetNSSecretsLister:       targetNSsecretsInformer.Lister(),
	}

	return factory.New().
		WithSync(c.sync).
		WithInformers(
			authnInformer.Informer(),
			consoleOperatorInformer.Informer(),
			routeInformer.Informer(),
			ingressConfigInformer.Informer(),
			targetNSsecretsInformer.Informer(),
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
	if shouldSync, err := c.handleManaged(ctx); err != nil {
		return err
	} else if !shouldSync {
		return nil
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	infrastructureConfig, err := c.infrastructureConfigLister.Get(api.ConfigResourceName)
	if err != nil {
		return statusHandler.FlushAndReturn(err)
	}

	clusterVersionConfig, err := c.clusterVersionLister.Get("version")
	if err != nil {
		return statusHandler.FlushAndReturn(err)
	}

	if util.IsExternalControlPlaneWithIngressDisabled(infrastructureConfig, clusterVersionConfig) {
		return statusHandler.FlushAndReturn(nil)
	}

	authnConfig, err := c.authnLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}

	switch authnConfig.Spec.Type {
	// We don't disable auth since the internal OAuth server is not disabled even with auth type 'None'.
	case "", configv1.AuthenticationTypeIntegratedOAuth, configv1.AuthenticationTypeNone:
	default:
		// if we're not using integrated oauth, reset all degraded conditions
		statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSync", "", nil))
		return statusHandler.FlushAndReturn(nil)
	}

	operatorConfig, err := c.consoleOperatorLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}

	ingressConfig, err := c.ingressConfigLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}

	var consoleURL *url.URL

	if len(operatorConfig.Spec.Ingress.ConsoleURL) == 0 {
		routeName := api.OpenShiftConsoleRouteName
		routeConfig := routesub.NewRouteConfig(operatorConfig, ingressConfig, routeName)
		if routeConfig.IsCustomHostnameSet() {
			routeName = api.OpenshiftConsoleCustomRouteName
		}

		_, url, _, routeErr := routesub.GetActiveRouteInfo(c.routesLister, routeName)
		if routeErr != nil {
			return routeErr
		}
		consoleURL = url
	} else {
		url, err := url.Parse(operatorConfig.Spec.Ingress.ConsoleURL)
		if err != nil {
			return fmt.Errorf("failed to parse console url: %w", err)
		}
		consoleURL = url
	}

	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if !cache.WaitForCacheSync(waitCtx.Done(), c.oauthClientSwitchedInformer.Informer().HasSynced) {
		return statusHandler.FlushAndReturn(fmt.Errorf("timed out waiting for OAuthClients cache sync"))
	}

	clientSecret, err := c.targetNSSecretsLister.Secrets(api.TargetNamespace).Get("console-oauth-config")
	if err != nil {
		return err
	}

	oauthErrReason, err := c.syncOAuthClient(ctx, clientSecret, consoleURL.String())
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSync", oauthErrReason, err))
	if err != nil {
		return statusHandler.FlushAndReturn(err)
	}

	return statusHandler.FlushAndReturn(nil)
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

// applies changes to the oauthclient
// should not be called until route & secret dependencies are verified
func (c *oauthClientsController) syncOAuthClient(
	ctx context.Context,
	sec *corev1.Secret,
	consoleURL string,
) (reason string, err error) {
	oauthClient, err := c.oauthClientLister.Get(oauthsub.Stub().Name)
	if err != nil && !errors.IsNotFound(err) {
		// at this point we must die & wait for someone to fix the lack of an outhclient. there is nothing we can do.
		return "FailedGet", fmt.Errorf("getting console oauth client (%w)", err)
	}

	if errors.IsNotFound(err) {
		oauthClient = &oauthv1.OAuthClient{
			ObjectMeta: metav1.ObjectMeta{
				Name: oauthsub.Stub().Name,
			},
			GrantMethod:           oauthv1.GrantHandlerAuto,
			RespondWithChallenges: false,
		}
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
