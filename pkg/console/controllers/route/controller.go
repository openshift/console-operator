package route

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"time"

	// k8s
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	// openshift
	operatorsv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	v1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	routeclientv1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	routesinformersv1 "github.com/openshift/client-go/route/informers/externalversions/route/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	// console-operator
	"github.com/openshift/console-operator/pkg/api"
	customerrors "github.com/openshift/console-operator/pkg/console/errors"
	"github.com/openshift/console-operator/pkg/console/status"
	routesub "github.com/openshift/console-operator/pkg/console/subresource/route"
)

const (
	controllerName         = "ConsoleRouteSyncController"
	controllerWorkQueueKey = "console-route-sync--work-queue-key"
)

type RouteSyncController struct {
	// clients
	operatorClient       v1helpers.OperatorClient
	operatorConfigClient operatorclientv1.ConsoleInterface
	ingressClient        configclientv1.IngressInterface
	routeClient          routeclientv1.RoutesGetter
	configMapClient      coreclientv1.ConfigMapsGetter
	secretClient         coreclientv1.SecretsGetter
	// names
	targetNamespace string
	routeName       string
	// events
	cachesToSync []cache.InformerSynced
	queue        workqueue.RateLimitingInterface
	recorder     events.Recorder
	// context
	ctx context.Context
}

func NewRouteSyncController(
	// top level config
	configClient configclientv1.ConfigV1Interface,
	// clients
	operatorClient v1helpers.OperatorClient,
	operatorConfigClient operatorclientv1.ConsoleInterface,
	routev1Client routeclientv1.RoutesGetter,
	configMapClient coreclientv1.ConfigMapsGetter,
	secretClient coreclientv1.SecretsGetter,
	// informers
	operatorConfigInformer v1.ConsoleInformer,
	routeInformer routesinformersv1.RouteInformer,
	// names
	targetNamespace string,
	routeName string,
	// events
	recorder events.Recorder,
	// context
	ctx context.Context,
) *RouteSyncController {
	ctrl := &RouteSyncController{
		operatorClient:       operatorClient,
		operatorConfigClient: operatorConfigClient,
		ingressClient:        configClient.Ingresses(),
		routeClient:          routev1Client,
		configMapClient:      configMapClient,
		secretClient:         secretClient,
		targetNamespace:      targetNamespace,
		routeName:            routeName,
		// events
		recorder:     recorder,
		cachesToSync: nil,
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
		ctx:          ctx,
	}

	operatorClient.Informer().AddEventHandler(ctrl.newEventHandler())
	operatorConfigInformer.Informer().AddEventHandler(ctrl.newEventHandler())
	routeInformer.Informer().AddEventHandler(ctrl.newEventHandler())

	ctrl.cachesToSync = append(ctrl.cachesToSync,
		operatorClient.Informer().HasSynced,
		operatorConfigInformer.Informer().HasSynced,
		routeInformer.Informer().HasSynced,
	)

	return ctrl
}

func (c *RouteSyncController) sync() error {
	operatorConfig, err := c.operatorConfigClient.Get(c.ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	updatedOperatorConfig := operatorConfig.DeepCopy()

	switch updatedOperatorConfig.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infoln("console is in a managed state: syncing route")
	case operatorsv1.Unmanaged:
		klog.V(4).Infoln("console is in an unmanaged state: skipping route sync")
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infoln("console is in a removed state: deleting route")
		if err = c.removeRoute(api.OpenshiftConsoleCustomRouteName); err != nil {
			return err
		}
		return c.removeRoute(api.OpenShiftConsoleName)
	default:
		return fmt.Errorf("unknown state: %v", updatedOperatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	defaultRoute, defaultRouteErrReason, defaultRouteErr := c.SyncDefaultRoute(updatedOperatorConfig)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("DefaultRouteSync", defaultRouteErrReason, defaultRouteErr))
	if defaultRouteErr != nil {
		return statusHandler.FlushAndReturn(defaultRouteErr)
	}

	customRoute, customRouteErrReason, customRouteErr := c.SyncCustomRoute(updatedOperatorConfig)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("CustomRouteSync", customRouteErrReason, customRouteErr))
	if customRouteErr != nil {
		return statusHandler.FlushAndReturn(customRouteErr)
	}

	activeRoute := defaultRoute
	if routesub.IsCustomRouteSet(updatedOperatorConfig) {
		activeRoute = customRoute
	}

	routeHealthCheckErrReason, routeHealthCheckErr := c.CheckRouteHealth(updatedOperatorConfig, activeRoute)
	statusHandler.AddCondition(status.HandleDegraded("RouteHealth", routeHealthCheckErrReason, routeHealthCheckErr))
	if routeHealthCheckErr != nil {
		return statusHandler.FlushAndReturn(routeHealthCheckErr)
	}

	return statusHandler.FlushAndReturn(nil)
}

func (c *RouteSyncController) removeRoute(routeName string) error {
	err := c.routeClient.Routes(c.targetNamespace).Delete(c.ctx, routeName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *RouteSyncController) SyncDefaultRoute(operatorConfig *operatorsv1.Console) (*routev1.Route, string, error) {
	requiredDefaultRoute := routesub.DefaultRoute(operatorConfig)

	defaultRoute, _, defaultRouteError := routesub.ApplyRoute(c.routeClient, c.recorder, requiredDefaultRoute)
	if defaultRouteError != nil {
		return nil, "FailedDefaultRouteApply", defaultRouteError
	}

	if len(routesub.GetCanonicalHost(defaultRoute)) == 0 {
		return nil, "FailedDefaultRouteHost", customerrors.NewSyncError(fmt.Sprintf("default route is not available at canonical host %s", defaultRoute.Status.Ingress))
	}

	return defaultRoute, "", defaultRouteError
}

// Custom route sync needs to:
// 1. validate if the reference for secret with TLS certificate and key is defined in operator config(in case a non-openshift cluster domain is used)
// 2. if secret is defined, verify the TLS certificate and key
// 4. create the custom console route, if custom TLS certificate and key are defined use them
// 5. apply the custom route
func (c *RouteSyncController) SyncCustomRoute(operatorConfig *operatorsv1.Console) (*routev1.Route, string, error) {
	if !routesub.IsCustomRouteSet(operatorConfig) {
		if err := c.removeRoute(api.OpenshiftConsoleCustomRouteName); err != nil {
			return nil, "FailedDeleteCustomRoutes", err
		}
		return nil, "", nil
	}

	customSecret, configErr := c.ValidateCustomRouteConfig(operatorConfig)
	if configErr != nil {
		return nil, "InvalidCustomRouteConfig", configErr
	}

	customTLSCert, secretValidationErr := ValidateCustomCertSecret(customSecret)
	if secretValidationErr != nil {
		return nil, "InvalidCustomTLSSecret", secretValidationErr
	}

	requiredCustomRoute := routesub.CustomRoute(operatorConfig, customTLSCert)
	customRoute, _, customRouteError := routesub.ApplyRoute(c.routeClient, c.recorder, requiredCustomRoute)
	if customRouteError != nil {
		return nil, "FailedCustomRouteApply", customRouteError
	}

	if len(routesub.GetCanonicalHost(customRoute)) == 0 {
		return nil, "FailedCustomRouteHost", customerrors.NewSyncError(fmt.Sprintf("custom route is not available at canonical host %s", customRoute.Status.Ingress))
	}

	return customRoute, "", customRouteError
}

func (c *RouteSyncController) ValidateCustomRouteConfig(operatorConfig *operatorsv1.Console) (*corev1.Secret, error) {
	// get ingress
	ingress, err := c.ingressClient.Get(c.ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	// Check if the custom hostname has cluster domain suffix, which indicates
	// if a secret that contains TLS certificate and key needs to exist in the
	// `openshift-config` namespace and referenced in  the operator config.
	// If the suffix matches the cluster domain, then the secret is optional.
	// If the suffix doesn't matches the cluster domain, then the secret is mandatory.
	if !routesub.IsCustomRouteSecretSet(operatorConfig) {
		if !strings.HasSuffix(operatorConfig.Spec.Route.Hostname, ingress.Spec.Domain) {
			return nil, fmt.Errorf("secret reference for custom route TLS secret is not defined")
		}
		return nil, nil
	}

	secret, secretErr := c.secretClient.Secrets(api.OpenShiftConfigNamespace).Get(c.ctx, operatorConfig.Spec.Route.Secret.Name, metav1.GetOptions{})
	if secretErr != nil {
		return nil, fmt.Errorf("failed to GET custom route TLS secret: %s", secretErr)
	}
	return secret, nil
}

// Validate secret that holds custom TLS certificate and key.
// Secret has to contain `tls.crt` and `tls.key` data keys
// where the certificate and key are stored and both need
// to be in valid format.
// Return the custom TLS certificate and key
func ValidateCustomCertSecret(customCertSecret *corev1.Secret) (*routesub.CustomTLSCert, error) {
	if customCertSecret == nil {
		return nil, nil
	}
	if customCertSecret.Type != corev1.SecretTypeTLS {
		return nil, fmt.Errorf("custom cert secret is not in %q type, instead uses %q type", corev1.SecretTypeTLS, customCertSecret.Type)
	}

	customTLS := &routesub.CustomTLSCert{}
	cert, certExist := customCertSecret.Data["tls.crt"]
	if !certExist {
		return nil, fmt.Errorf("custom cert secret data doesn't contain 'tls.crt' entry")
	}

	certificateVerifyErr := certificateVerifier(cert)
	if certificateVerifyErr != nil {
		return nil, fmt.Errorf("failed to verify custom certificate PEM: " + certificateVerifyErr.Error())
	}
	customTLS.Certificate = string(cert)

	key, keyExist := customCertSecret.Data["tls.key"]
	if !keyExist {
		return nil, fmt.Errorf("custom cert secret data doesn't contain 'tls.key' entry")
	}

	privateKeyVerifyErr := privateKeyVerifier(key)
	if privateKeyVerifyErr != nil {
		return nil, fmt.Errorf("failed to verify custom key PEM: " + privateKeyVerifyErr.Error())
	}
	customTLS.Key = string(key)

	return customTLS, nil
}

func certificateVerifier(customCert []byte) error {
	block, _ := pem.Decode([]byte(customCert))
	if block == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	now := time.Now()
	if now.After(certificate.NotAfter) {
		return fmt.Errorf("custom TLS certificate is expired")
	}
	if now.Before(certificate.NotBefore) {
		return fmt.Errorf("custom TLS certificate is not valid yet")
	}
	return nil
}

func privateKeyVerifier(customKey []byte) error {
	block, _ := pem.Decode([]byte(customKey))
	if block == nil {
		return fmt.Errorf("failed to decode key PEM")
	}
	if _, err := x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
		if _, err = x509.ParsePKCS1PrivateKey(block.Bytes); err != nil {
			if _, err = x509.ParseECPrivateKey(block.Bytes); err != nil {
				return fmt.Errorf("block %s is not valid key PEM", block.Type)
			}
		}
	}
	return nil
}

func (c *RouteSyncController) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()
	klog.Infof("starting %v", controllerName)
	defer klog.Infof("shutting down %v", controllerName)
	if !cache.WaitForCacheSync(stopCh, c.cachesToSync...) {
		klog.Infoln("caches did not sync")
		runtime.HandleError(fmt.Errorf("caches did not sync"))
		return
	}
	// only start one worker
	go wait.Until(c.runWorker, time.Second, stopCh)
	<-stopCh
}

func (c *RouteSyncController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *RouteSyncController) processNextWorkItem() bool {
	processKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(processKey)
	err := c.sync()
	if err == nil {
		c.queue.Forget(processKey)
		return true
	}
	runtime.HandleError(fmt.Errorf("%v failed with : %v", processKey, err))
	c.queue.AddRateLimited(processKey)
	return true
}

func (c *RouteSyncController) newEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(controllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
	}
}

func (c *RouteSyncController) CheckRouteHealth(operatorConfig *operatorsv1.Console, route *routev1.Route) (string, error) {
	if !routesub.IsAdmitted(route) {
		return "RouteNotAdmitted", fmt.Errorf("console route is not admitted")
	}

	caPool, err := c.getCA(route.Spec.TLS)
	if err != nil {
		return "FailedLoadCA", fmt.Errorf("failed to read CA to check route health: %v", err)
	}
	client := clientWithCA(caPool)

	if len(route.Spec.Host) == 0 {
		return "RouteHostError", fmt.Errorf("route does not have host specified")
	}
	url := "https://" + route.Spec.Host + "/health"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "FailedRequest", fmt.Errorf("failed to build request to route (%s): %v", url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "FailedGet", fmt.Errorf("failed to GET route (%s): %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "StatusError", fmt.Errorf("route not yet available, %s returns '%s'", url, resp.Status)
	}

	return "", nil
}

func (c *RouteSyncController) getCA(tls *routev1.TLSConfig) (*x509.CertPool, error) {
	caCertPool := x509.NewCertPool()

	if tls != nil && len(tls.Certificate) != 0 {
		if ok := caCertPool.AppendCertsFromPEM([]byte(tls.Certificate)); !ok {
			klog.V(4).Infof("failed to parse custom tls.crt")
		}
	}

	for _, cmName := range []string{api.TrustedCAConfigMapName, api.DefaultIngressCertConfigMapName} {
		cm, err := c.configMapClient.ConfigMaps(api.OpenShiftConsoleNamespace).Get(c.ctx, cmName, metav1.GetOptions{})
		if err != nil {
			klog.V(4).Infof("failed to GET configmap %s / %s ", api.OpenShiftConsoleNamespace, cmName)
			return nil, err
		}
		if ok := caCertPool.AppendCertsFromPEM([]byte(cm.Data["ca-bundle.crt"])); !ok {
			klog.V(4).Infof("failed to parse %s ca-bundle.crt", cmName)
		}
	}

	return caCertPool, nil
}

func clientWithCA(caPool *x509.CertPool) *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caPool,
			},
		},
	}
}
