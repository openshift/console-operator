package oauthclientsecret

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1clients "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions/config/v1"
	configv1listers "github.com/openshift/client-go/config/listers/config/v1"
	operatorv1informers "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	operatorv1listers "github.com/openshift/client-go/operator/listers/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/status"
	"github.com/openshift/console-operator/pkg/crypto"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	v1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"

	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"
	utilsub "github.com/openshift/console-operator/pkg/console/subresource/util"
)

// oauthClientSecretController behaves differently based on authentication/cluster .spec.type:
//
//   - IntegratedOAuth - self-manage the client secret string
//   - OIDC - lookup our client in the authentication/cluster .spec.oidcProviders[x].oidcClients
//     slice and use the 'clientSecret' from the secret referred to by .clientSecret.name
//   - None - do nothing
//
// The secret written is 'openshift-console/console-oauth-config' in .Data['clientSecret']
type oauthClientSecretController struct {
	operatorClient v1helpers.OperatorClient
	secretsClient  corev1clients.SecretsGetter

	authConfigLister      configv1listers.AuthenticationLister
	consoleOperatorLister operatorv1listers.ConsoleLister
	configSecretsLister   corev1listers.SecretLister
	targetNSSecretsLister corev1listers.SecretLister
}

func NewOAuthClientSecretController(
	operatorClient v1helpers.OperatorClient,
	secretsClient corev1clients.SecretsGetter,
	authnInformer configv1informers.AuthenticationInformer,
	consoleOperatorInformer operatorv1informers.ConsoleInformer,
	configSecretsInformer corev1informers.SecretInformer,
	targetNSsecretsInformer corev1informers.SecretInformer,
	recorder events.Recorder,
) factory.Controller {
	c := &oauthClientSecretController{
		operatorClient: operatorClient,
		secretsClient:  secretsClient,

		authConfigLister:      authnInformer.Lister(),
		consoleOperatorLister: consoleOperatorInformer.Lister(),
		configSecretsLister:   configSecretsInformer.Lister(),
		targetNSSecretsLister: targetNSsecretsInformer.Lister(),
	}

	return factory.New().
		WithSync(c.sync).
		WithInformers(
			authnInformer.Informer(),
			consoleOperatorInformer.Informer(),
			configSecretsInformer.Informer(),
		).
		WithFilteredEventsInformers(
			factory.NamesFilter("console-oauth-config"), targetNSsecretsInformer.Informer(),
		).
		ToController("OAuthClientSecretController", recorder.WithComponentSuffix("oauthclient-secret-controller"))
}

func (c *oauthClientSecretController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	if shouldSync, err := c.handleManaged(ctx); err != nil {
		return err
	} else if !shouldSync {
		return nil
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	clientSecret, err := c.targetNSSecretsLister.Secrets(api.TargetNamespace).Get("console-oauth-config")
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	authConfig, err := c.authConfigLister.Get("cluster")
	if err != nil {
		return fmt.Errorf("failed to retrieve authentication config: %w", err)
	}

	var secretString string
	switch authConfig.Spec.Type {
	case "", configv1.AuthenticationTypeIntegratedOAuth:
		// in OpenShift controlled world, we generate the client secret ourselves
		if clientSecret != nil {
			secretString = secretsub.GetSecretString(clientSecret)
		}
		if len(secretString) == 0 {
			secretString = crypto.Random256BitsString()
		}
	case configv1.AuthenticationTypeOIDC:
		clientConfig := utilsub.GetOIDCClientConfig(authConfig)
		if clientConfig == nil {
			// no config, flush the condition and return
			statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSecretSync", "", nil))
			return statusHandler.FlushAndReturn(nil)
		}

		if len(clientConfig.ClientSecret.Name) == 0 {
			statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSecretSync", "MissingClientSecretConfig", fmt.Errorf("missing client secret name reference in config")))
			return statusHandler.FlushAndReturn(nil)
		}

		conficClientSecret, err := c.configSecretsLister.Secrets(api.OpenShiftConfigNamespace).Get(clientConfig.ClientSecret.Name)
		if err != nil {
			statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSecretSync", "FailedClientSecretGet", err))
			return statusHandler.FlushAndReturn(err)
		}

		secretString = secretsub.GetSecretString(conficClientSecret)
		if len(secretString) == 0 {
			statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSecretSync", "ClientSecretKeyMissing", fmt.Errorf("missing the 'clientSecret' key in the client secret secret %q", clientConfig.ClientSecret.Name)))
			return statusHandler.FlushAndReturn(nil)
		}
	default:
		klog.V(2).Infof("unknown authentication type: %s", authConfig.Spec.Type)
		return nil
	}

	err = c.syncSecret(ctx, secretString, syncCtx.Recorder())
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("OAuthClientSecretSync", "FailedApply", err))
	return statusHandler.FlushAndReturn(err)
}

func (c *oauthClientSecretController) syncSecret(ctx context.Context, clientSecret string, recorder events.Recorder) error {
	operatorConfig, err := c.consoleOperatorLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}

	secret, err := c.targetNSSecretsLister.Secrets(api.TargetNamespace).Get("console-oauth-config")
	if apierrors.IsNotFound(err) || secretsub.GetSecretString(secret) == clientSecret {
		_, _, err = resourceapply.ApplySecret(ctx, c.secretsClient, recorder, secretsub.DefaultSecret(operatorConfig, clientSecret))
	}
	return err
}

// handleStatus returns whether sync should happen and any error encountering
// determining the operator's management state
// TODO: extract this logic to where it can be used for all controllers
func (c *oauthClientSecretController) handleManaged(ctx context.Context) (bool, error) {
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
		return false, nil
	default:
		return false, fmt.Errorf("console is in an unknown state: %v", managementState)
	}
}
