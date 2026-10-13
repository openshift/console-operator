package oidcsetup

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
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
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	authnsub "github.com/openshift/console-operator/pkg/console/subresource/authentication"
	deploymentsub "github.com/openshift/console-operator/pkg/console/subresource/deployment"
	utilsub "github.com/openshift/console-operator/pkg/console/subresource/util"
)

// reachabilityError wraps errors caused by transient network issues (DNS,
// TLS handshake, timeouts, non-200 responses) as opposed to permanent
// configuration mistakes. The caller uses this to choose Degraded (retry)
// over DegradedNotAvailable (component unavailable).
type reachabilityError struct {
	err error
}

func (e *reachabilityError) Error() string { return e.err.Error() }
func (e *reachabilityError) Unwrap() error { return e.err }

// issuerValidationCache caches the result of the last OIDC issuer probe
// so that the synchronous HTTP call is not repeated on every sync when
// the issuer configuration has not changed.
type issuerValidationCache struct {
	issuerURL    string
	discoveryURL string
	caBundleHash [32]byte
	err          error
}

// oidcSetupController:
//
//	writes:
//	- authentication.config.openshift.io/cluster .status.oidcClients:
//		- componentName=console
//		- componentNamespace=openshift-console
//		- currentOIDCClients
//		- conditions:
//			- Available
//			- Progressing
//			- Degraded
//	- consoles.operator.openshift.io/cluster .status.conditions:
//		- type=OIDCClientConfigProgressing
//		- type=OIDCClientConfigDegraded
//		- type=AuthStatusHandlerProgressing
//		- type=AuthStatusHandlerDegraded
type oidcSetupController struct {
	operatorClient  v1helpers.OperatorClient
	configMapClient corev1client.ConfigMapsGetter

	authnLister               configv1listers.AuthenticationLister
	consoleOperatorLister     operatorv1listers.ConsoleLister
	configConfigMapLister     corev1listers.ConfigMapLister
	targetNSSecretsLister     corev1listers.SecretLister
	targetNSConfigMapLister   corev1listers.ConfigMapLister
	targetNSDeploymentsLister appsv1listers.DeploymentLister

	externalOIDCFeatureEnabled bool

	authStatusHandler    *status.AuthStatusHandler
	lastIssuerValidation *issuerValidationCache
}

func NewOIDCSetupController(
	operatorClient v1helpers.OperatorClient,
	configMapClient corev1client.ConfigMapsGetter,
	authnInformer configv1informers.AuthenticationInformer,
	authenticationClient configv1client.AuthenticationInterface,
	consoleOperatorInformer operatorv1informers.ConsoleInformer,
	configConfigMapInformer corev1informers.ConfigMapInformer,
	targetNSsecretsInformer corev1informers.SecretInformer,
	targetNSConfigMapInformer corev1informers.ConfigMapInformer,
	targetNSDeploymentsInformer appsv1informers.DeploymentInformer,
	externalOIDCFeatureEnabled bool,
	recorder events.Recorder,
) factory.Controller {
	c := &oidcSetupController{
		operatorClient:  operatorClient,
		configMapClient: configMapClient,

		authnLister:               authnInformer.Lister(),
		consoleOperatorLister:     consoleOperatorInformer.Lister(),
		configConfigMapLister:     configConfigMapInformer.Lister(),
		targetNSSecretsLister:     targetNSsecretsInformer.Lister(),
		targetNSDeploymentsLister: targetNSDeploymentsInformer.Lister(),
		targetNSConfigMapLister:   targetNSConfigMapInformer.Lister(),

		externalOIDCFeatureEnabled: externalOIDCFeatureEnabled,

		authStatusHandler: status.NewAuthStatusHandler(authenticationClient, api.OpenShiftConsoleName, api.TargetNamespace, api.OpenShiftConsoleOperator),
	}
	return factory.New().
		WithSync(c.sync).
		ResyncEvery(wait.Jitter(time.Minute, 1.0)).
		WithInformers(
			authnInformer.Informer(),
			configConfigMapInformer.Informer(),
			consoleOperatorInformer.Informer(),
			targetNSsecretsInformer.Informer(),
			targetNSDeploymentsInformer.Informer(),
			targetNSConfigMapInformer.Informer(),
		).
		ToController("OIDCSetupController", recorder.WithComponentSuffix("oidc-setup-controller"))
}

func (c *oidcSetupController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	statusHandler := status.NewStatusHandler(c.operatorClient)

	if shouldSync, err := c.handleManaged(); err != nil {
		return err
	} else if !shouldSync {
		return nil
	}

	// we assume API validation won't allow authentication/cluster 'spec.type=OIDC'
	// if the OIDC feature gate is not enabled
	if !c.externalOIDCFeatureEnabled {
		// reset all conditions set by this controller
		statusHandler.AddConditions(status.HandleProgressingOrDegraded("OIDCClientConfig", "", nil))
		statusHandler.AddConditions(status.HandleProgressingOrDegraded("AuthStatusHandler", "", nil))
		return statusHandler.FlushAndReturn(nil)
	}

	authnConfig, err := c.authnLister.Get(api.ConfigResourceName)
	if err != nil {
		return err
	}

	operatorConfig, err := c.consoleOperatorLister.Get("cluster")
	if err != nil {
		return err
	}

	if authnConfig.Spec.Type != configv1.AuthenticationTypeOIDC {
		// If the authentication type is not "OIDC", set the CurrentOIDCClient
		// on the authStatusHandler to an empty string. This is necessary during a
		// scenario where the authentication type goes from OIDC to non-OIDC because
		// the CurrentOIDCClient would have been set while the authentication type was OIDC.
		// If the CurrentOIDCClient value isn't reset on this transition the authStatusHandler
		// will think OIDC is still configured and attempt to update the OIDC client in the
		// status to have an empty providerName and issuerURL, violating the validations
		// on the Authentication CRD as seen in https://issues.redhat.com/browse/OCPBUGS-44953
		c.authStatusHandler.WithCurrentOIDCClient("")

		applyErr := c.authStatusHandler.Apply(ctx, authnConfig)
		statusHandler.AddConditions(status.HandleProgressingOrDegraded("AuthStatusHandler", "FailedApply", applyErr))

		// reset the other condition set by this controller
		statusHandler.AddConditions(status.HandleProgressingOrDegraded("OIDCClientConfig", "", nil))
		return statusHandler.FlushAndReturn(applyErr)
	}

	// we need to keep track of errors during the sync so that we can requeue
	// if any occur
	var errs []error
	syncErr := c.syncAuthTypeOIDC(ctx, authnConfig, operatorConfig, syncCtx.Recorder())
	statusHandler.AddConditions(
		status.HandleProgressingOrDegraded(
			"OIDCClientConfig", "OIDCConfigSyncFailed",
			syncErr,
		),
	)
	if syncErr != nil {
		errs = append(errs, syncErr)
	}

	applyErr := c.authStatusHandler.Apply(ctx, authnConfig)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("AuthStatusHandler", "FailedApply", applyErr))
	if applyErr != nil {
		errs = append(errs, applyErr)
	}

	if len(errs) > 0 {
		return statusHandler.FlushAndReturn(factory.SyntheticRequeueError)
	}
	return statusHandler.FlushAndReturn(nil)
}

func (c *oidcSetupController) syncAuthTypeOIDC(ctx context.Context, authnConfig *configv1.Authentication, operatorConfig *operatorv1.Console, recorder events.Recorder) error {
	oidcProvider, clientConfig := authnsub.GetOIDCClientConfig(authnConfig, api.TargetNamespace, api.OpenShiftConsoleName)
	if clientConfig == nil {
		c.authStatusHandler.WithCurrentOIDCClient("")
		c.authStatusHandler.Unavailable("OIDCClientConfig", "no OIDC client found")
		return nil
	}

	if len(clientConfig.ClientID) == 0 {
		return fmt.Errorf("no ID set on console's OIDC client")
	}
	c.authStatusHandler.WithCurrentOIDCClient(clientConfig.ClientID)

	if len(clientConfig.ClientSecret.Name) == 0 {
		c.authStatusHandler.Degraded("OIDCClientMissingSecret", "no client secret in the OIDC client config")
		return nil
	}

	clientSecret, err := c.targetNSSecretsLister.Secrets(api.TargetNamespace).Get("console-oauth-config")
	if err != nil {
		c.authStatusHandler.Degraded("OIDCClientSecretGet", err.Error())
		return err
	}

	var caBundle []byte
	if caCMName := oidcProvider.Issuer.CertificateAuthority.Name; len(caCMName) > 0 {
		caCM, err := c.configConfigMapLister.ConfigMaps(api.OpenShiftConfigNamespace).Get(caCMName)
		if err != nil {
			return fmt.Errorf("failed to get the CA configMap %q configured for the OIDC provider %q: %w", caCMName, oidcProvider.Name, err)
		}

		data, ok := caCM.Data["ca-bundle.crt"]
		if !ok {
			return fmt.Errorf("CA configMap %q for OIDC provider %q does not contain the expected %q key", caCMName, oidcProvider.Name, "ca-bundle.crt")
		}
		caBundle = []byte(data)

		_, _, err = resourceapply.SyncPartialConfigMap(ctx,
			c.configMapClient,
			recorder,
			caCM.Namespace, caCM.Name,
			api.TargetNamespace, caCM.Name,
			sets.New[string]("ca-bundle.crt"),
			[]metav1.OwnerReference{*utilsub.OwnerRefFrom(operatorConfig)})
		if err != nil {
			return fmt.Errorf("failed to sync the provider's CA configMap: %w", err)
		}
	}

	if err := c.validateOIDCIssuerCached(ctx, oidcProvider.Issuer.URL, oidcProvider.Issuer.DiscoveryURL, caBundle); err != nil {
		var reachErr *reachabilityError
		if errors.As(err, &reachErr) {
			// Transient reachability failures use Degraded so the operator
			// retries on the next sync; Available is left unchanged because
			// operator-pod reachability does not reflect end-user reachability.
			c.authStatusHandler.Degraded("OIDCIssuerNotReachable", err.Error())
		} else {
			c.authStatusHandler.DegradedNotAvailable("OIDCIssuerURLInvalid", err.Error())
		}
		return err
	}

	if valid, msg, err := c.checkClientConfigStatus(authnConfig, clientSecret); err != nil {
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
func (c *oidcSetupController) checkClientConfigStatus(authnConfig *configv1.Authentication, clientSecret *corev1.Secret) (bool, string, error) {
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

		serverCAConfig, err := c.targetNSConfigMapLister.ConfigMaps(api.OpenShiftConsoleNamespace).Get(serverCAConfigName)
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
func (c *oidcSetupController) handleManaged() (bool, error) {
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

// validateOIDCIssuerCached wraps validateOIDCIssuer with a cache keyed on the
// issuer configuration (URL, discovery URL, CA bundle). The probe is only
// executed when the configuration changes, avoiding a synchronous HTTP request
// on every sync triggered by unrelated informer events.
func (c *oidcSetupController) validateOIDCIssuerCached(ctx context.Context, issuerURL, discoveryURLOverride string, caBundle []byte) error {
	bundleHash := sha256.Sum256(caBundle)
	if c.lastIssuerValidation != nil &&
		c.lastIssuerValidation.issuerURL == issuerURL &&
		c.lastIssuerValidation.discoveryURL == discoveryURLOverride &&
		c.lastIssuerValidation.caBundleHash == bundleHash {
		return c.lastIssuerValidation.err
	}

	err := validateOIDCIssuer(ctx, issuerURL, discoveryURLOverride, caBundle)
	c.lastIssuerValidation = &issuerValidationCache{
		issuerURL:    issuerURL,
		discoveryURL: discoveryURLOverride,
		caBundleHash: bundleHash,
		err:          err,
	}
	return err
}

// validateOIDCIssuer probes the OIDC discovery endpoint and validates the
// response. It returns a *reachabilityError for transient network/HTTP
// failures and a plain error for permanent configuration problems.
//
// Structural URL checks (scheme, host, query, fragment) are omitted because
// the API server's XValidation rules on issuerURL already enforce them before
// the config is persisted.
func validateOIDCIssuer(ctx context.Context, issuerURL string, discoveryURLOverride string, caBundle []byte) error {
	if len(issuerURL) == 0 {
		return fmt.Errorf("issuer URL is empty")
	}

	// Probe the OIDC discovery endpoint. When the API specifies an explicit
	// discoveryURL (ExternalOIDCWithUpstreamParity feature gate), use it
	// instead of deriving the standard path from the issuer URL.
	discoveryURL := discoveryURLOverride
	if len(discoveryURL) == 0 {
		discoveryURL = strings.TrimRight(issuerURL, "/") + "/.well-known/openid-configuration"
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if len(caBundle) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caBundle) {
			return fmt.Errorf("failed to parse CA bundle for OIDC issuer")
		}
		tlsConfig.RootCAs = pool
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create OIDC discovery request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return &reachabilityError{err: fmt.Errorf("OIDC issuer URL %q is not reachable: %w", issuerURL, err)}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			klog.V(4).Infof("failed to close OIDC discovery response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return &reachabilityError{err: fmt.Errorf("OIDC discovery endpoint returned HTTP %d for issuer %q", resp.StatusCode, issuerURL)}
	}

	// Per OpenID Connect Discovery 1.0 §4.2, the response MUST use
	// application/json. Require the header so we fail fast on
	// misconfigured providers instead of falling through to JSON parsing.
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		return fmt.Errorf("OIDC discovery endpoint returned no Content-Type header for issuer %q", issuerURL)
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		return fmt.Errorf("OIDC discovery endpoint returned non-JSON content type %q for issuer %q", contentType, issuerURL)
	}

	// Decode and validate the discovery document
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		return &reachabilityError{err: fmt.Errorf("failed to read OIDC discovery response: %w", err)}
	}

	var discovery struct {
		Issuer string `json:"issuer"`
	}
	if err := json.Unmarshal(body, &discovery); err != nil {
		return fmt.Errorf("OIDC discovery response is not valid JSON: %w", err)
	}

	// Per OpenID Connect Discovery 1.0, the issuer value in the discovery
	// document must match the configured issuer URL. Trailing slashes are
	// normalized because the CRD itself normalizes them on write.
	normalizedIssuer := strings.TrimRight(issuerURL, "/")
	normalizedDiscovery := strings.TrimRight(discovery.Issuer, "/")
	if normalizedDiscovery != normalizedIssuer {
		return fmt.Errorf("OIDC discovery issuer %q does not match configured issuer %q", discovery.Issuer, issuerURL)
	}

	return nil
}
