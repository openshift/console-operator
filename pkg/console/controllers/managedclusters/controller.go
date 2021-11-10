package managedcluster

import (
	"context"
	"errors"
	"fmt"
	"strings"

	// k8s
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"

	// openshift
	clusterclientv1 "github.com/open-cluster-management/api/client/cluster/clientset/versioned/typed/cluster/v1"
	clusterinformersv1 "github.com/open-cluster-management/api/client/cluster/informers/externalversions/cluster/v1"
	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	oauthclientv1 "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	v1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	//subresources
	configmapsub "github.com/openshift/console-operator/pkg/console/subresource/configmap"
	managedclusteractionsub "github.com/openshift/console-operator/pkg/console/subresource/managedclusteraction"
	managedclusterviewsub "github.com/openshift/console-operator/pkg/console/subresource/managedclusterview"
	oauthsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"
	secretsub "github.com/openshift/console-operator/pkg/console/subresource/secret"

	// console-operator
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
)

type ManagedClusterController struct {
	operatorClient       v1helpers.OperatorClient
	operatorConfigClient operatorclientv1.ConsoleInterface
	configMapClient      coreclientv1.ConfigMapsGetter
	managedClusterClient clusterclientv1.ManagedClustersGetter
	dynamicClient        dynamic.Interface
	secretsClient        coreclientv1.SecretsGetter
	oauthClient          oauthclientv1.OAuthClientsGetter
}

func NewManagedClusterController(
	// top level config
	configClient configclientv1.ConfigV1Interface,
	configInformer configinformer.SharedInformerFactory,

	// clients
	operatorClient v1helpers.OperatorClient,
	operatorConfigClient operatorclientv1.ConsoleInterface,
	configMapClient coreclientv1.ConfigMapsGetter,
	managedClusterClient clusterclientv1.ClusterV1Interface,
	dynamicClient dynamic.Interface,
	secretsClient coreclientv1.SecretsGetter,
	oauthClient oauthclientv1.OAuthClientsGetter,

	// informers
	operatorConfigInformer v1.ConsoleInformer,
	managedClusterInformers clusterinformersv1.ManagedClusterInformer,
	dynamicInformers dynamicinformer.DynamicSharedInformerFactory,

	// events
	recorder events.Recorder,
) factory.Controller {
	ctrl := &ManagedClusterController{
		operatorClient:       operatorClient,
		operatorConfigClient: operatorConfigClient,
		configMapClient:      configMapClient,
		managedClusterClient: managedClusterClient,
		dynamicClient:        dynamicClient,
		secretsClient:        secretsClient,
		oauthClient:          oauthClient,
	}

	configV1Informers := configInformer.Config().V1()

	return factory.New().
		WithFilteredEventsInformers( // configs
			util.IncludeNamesFilter(api.ConfigResourceName),
			configV1Informers.Consoles().Informer(),
			operatorConfigInformer.Informer(),
		).WithFilteredEventsInformers(
		util.ExcludeNamesFilter(api.HubClusterName),
		managedClusterInformers.Informer(),
	).WithInformers(
		dynamicInformers.ForResource(managedclusteractionsub.GetGroupVersionResource()).Informer(),
	).WithInformers(
		dynamicInformers.ForResource(managedclusterviewsub.GetGroupVersionResource()).Informer(),
	).WithSync(ctrl.Sync).
		ToController("ManagedClusterController", recorder.WithComponentSuffix("managed-cluster-controller"))
}

func (c *ManagedClusterController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.operatorConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	switch operatorConfig.Spec.ManagementState {
	case operatorv1.Managed:
		klog.V(4).Info("console-operator is in a managed state: syncing managed clusters")
	case operatorv1.Unmanaged:
		klog.V(4).Info("console-operator is in an unmanaged state: skipping managed cluster sync")
		return nil
	case operatorv1.Removed:
		klog.V(4).Infof("console-operator is in a removed state: deleting managed clusters")
		return c.removeManagedClusters(ctx)
	default:
		return fmt.Errorf("unknown state: %v", operatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	managedClusters, err := c.listManagedClusters(ctx)
	// Not found means API is not on the cluster
	if apierrors.IsNotFound(err) || err != nil {
		return err
	}

	// Get a list of ManagedCluster resources, degraded if error is returned
	managedClusterClientConfigs, managedClusterClientConfigValidationErr, managedClusterClientConfigValidationErrReason := c.ValidateManagedClusterClientConfigs(ctx, operatorConfig, controllerContext.Recorder(), managedClusters)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterValidation", managedClusterClientConfigValidationErrReason, managedClusterClientConfigValidationErr))
	if managedClusterClientConfigValidationErr != nil {
		return statusHandler.FlushAndReturn(managedClusterClientConfigValidationErr)
	}

	// No managed clusters exist, quit sync
	if len(managedClusterClientConfigs) == 0 {
		return statusHandler.FlushAndReturn(nil)
	}

	// Create config maps for each managed cluster API server ca bundle
	apiServerCASyncErr, apiServerCASyncErrReason := c.SyncAPIServerCAConfigMaps(managedClusterClientConfigs, ctx, operatorConfig, controllerContext.Recorder())
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterAPIServerCASync", apiServerCASyncErrReason, apiServerCASyncErr))
	if apiServerCASyncErr != nil {
		return statusHandler.FlushAndReturn(apiServerCASyncErr)
	}

	// Create managed cluster views for oauth clients
	managedClusterViewOAuthClients, managedClusterViewOAuthClientErr, managedClusterViewOAuthClientErrReason := c.SyncManagedClusterViewOAuthClient(ctx, operatorConfig, managedClusters)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterViewOAuthClientSync", managedClusterViewOAuthClientErrReason, managedClusterViewOAuthClientErr))

	// Create managed cluster actions for oauth clients
	_, managedClusterActionCreateOAuthClientErr, managedClusterActionCreateOAuthClientErrReason := c.SyncManagedClusterActionCreateOAuthClient(ctx, operatorConfig, managedClusterViewOAuthClients)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterActionCreateOAuthClientSync", managedClusterActionCreateOAuthClientErrReason, managedClusterActionCreateOAuthClientErr))

	// Create managed cluster views for ingress cert
	managedClusterViewsIngressCert, managedClusterViewIngressCertErr, managedClusterViewIngressCertErrReason := c.SyncManagedClusterViewIngressCert(ctx, operatorConfig, managedClusters)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterViewIngressCertSync", managedClusterViewIngressCertErrReason, managedClusterViewIngressCertErr))

	// Create config maps for each managed cluster ingress cert bundle
	managedClusterIngressCertSyncErr, managedClusterIngressCertSyncErrReason := c.SyncManagedClusterIngressCertConfigMap(managedClusterViewsIngressCert, ctx, operatorConfig, controllerContext.Recorder())
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterIngressCertConfigMapSync", managedClusterIngressCertSyncErrReason, managedClusterIngressCertSyncErr))

	// Create  manged cluster config map
	configSyncErr, configSyncErrReason := c.SyncManagedClusterConfigMap(managedClusterClientConfigs, ctx, operatorConfig, controllerContext.Recorder())
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterConfigSync", configSyncErrReason, configSyncErr))
	return statusHandler.FlushAndReturn(configSyncErr)
}

// Return slice of clusterv1.ClientConfigs that have been validated or error and reaons if unable to list ManagedClusters
func (c *ManagedClusterController) ValidateManagedClusterClientConfigs(ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder, managedClusters *clusterv1.ManagedClusterList) (map[string]*clusterv1.ClientConfig, error, string) {
	validatedClientConfigs := map[string]*clusterv1.ClientConfig{}
	for _, managedCluster := range managedClusters.Items {
		clusterName := managedCluster.GetName()

		// Ensure client configs exists
		clientConfigs := managedCluster.Spec.ManagedClusterClientConfigs
		if len(clientConfigs) == 0 {
			klog.V(4).Infoln(fmt.Sprintf("Skipping managed cluster %v, no client config found", clusterName))
			continue
		}

		// Ensure client config CA bundle exists
		if clientConfigs[0].CABundle == nil {
			klog.V(4).Infoln(fmt.Sprintf("Skipping managed cluster %v, client config CA bundle not found", clusterName))
			continue
		}

		// Ensure client config URL exists
		if clientConfigs[0].URL == "" {
			klog.V(4).Infof("Skipping managed cluster %v, client config URL not found", clusterName)
			continue
		}

		validatedClientConfigs[clusterName] = &clientConfigs[0]
	}

	return validatedClientConfigs, nil, ""
}

// Using ManagedCluster.spec.ManagedClusterClientConfigs, sync ConfigMaps containing the API server CA bundle for each managed cluster
// If a managed cluster doesn't have complete client config yet, the information is logged, but no error is returned
// If applying any ConfigMap fails, an error and reason are returned
func (c *ManagedClusterController) SyncAPIServerCAConfigMaps(clientConfigs map[string]*clusterv1.ClientConfig, ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder) (error, string) {
	errs := []string{}
	for clusterName, clientConfig := range clientConfigs {
		// Apply the config map. If this fails for any managed cluster, operator is degraded
		required := configmapsub.DefaultAPIServerCAConfigMap(clusterName, clientConfig.CABundle, operatorConfig)
		_, _, configMapApplyError := resourceapply.ApplyConfigMap(c.configMapClient, recorder, required)
		if configMapApplyError != nil {
			klog.V(4).Infoln(fmt.Sprintf("Skipping API server CA ConfigMap sync for managed cluster %v, Error applying ConfigMap", clusterName))
			errs = append(errs, configMapApplyError.Error())
			continue
		}
	}

	// Return any apply errors that occurred
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n")), "APIServerCAConfigMapSyncError"
	}

	// Success
	return nil, ""
}

// Using ManagedClusters.Spec.ManagedClusterClientConfigs and previously synced CA bundles, sync a ConfigMap containing serverconfig.ManagedClusterConfig YAML for each managed cluster
// If a managed cluster doesn't have an API server CA bundle ConfigMap yet or the client config is incomplete, this is logged, but no error is returned
// If applying the ConfigMap fails, an error and reason are returned
func (c *ManagedClusterController) SyncManagedClusterConfigMap(clientConfigs map[string]*clusterv1.ClientConfig, ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder) (error, string) {
	managedClusterConfigs := []consoleserver.ManagedClusterConfig{}
	for clusterName, clientConfig := range clientConfigs {
		klog.V(4).Infoln(fmt.Sprintf("Building config for managed cluster: %v", clusterName))

		// Check that managed cluster CA ConfigMap has already been synced, skip if not found
		_, err := c.configMapClient.ConfigMaps(api.OpenShiftConsoleNamespace).Get(ctx, configmapsub.APIServerCAConfigMapName(clusterName), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("CA file not found for managed cluster %v", clusterName)
			continue
		}

		// Skip if unable to get managed cluster API server config map for any other reason
		if err != nil {
			klog.V(4).Infof("Error getting CA file for managed cluster %v", clusterName)
			continue
		}

		managedClusterConfigs = append(managedClusterConfigs, consoleserver.ManagedClusterConfig{
			Name: clusterName,
			APIServer: consoleserver.ManagedClusterAPIServerConfig{
				URL:    clientConfig.URL,
				CAFile: fmt.Sprintf("%s/%s/%s", api.ManagedClusterAPIServerCAMountDir, configmapsub.APIServerCAConfigMapName(clusterName), api.ManagedClusterAPIServerCAKey),
			},
		})
	}

	if len(managedClusterConfigs) > 0 {
		required, err := configmapsub.DefaultManagedClustersConfigMap(operatorConfig, managedClusterConfigs)
		if err != nil {
			return err, "FailedMarshallingYAML"
		}

		if _, _, applyErr := resourceapply.ApplyConfigMap(c.configMapClient, recorder, required); applyErr != nil {
			return applyErr, "FailedApply"
		}
	}

	return nil, ""
}

func (c *ManagedClusterController) SyncManagedClusterViewOAuthClient(ctx context.Context, operatorConfig *operatorv1.Console, managedClusters *clusterv1.ManagedClusterList) ([]*unstructured.Unstructured, error, string) {
	errs := []string{}
	managedClusterOAuthClientViews := []*unstructured.Unstructured{}
	for _, managedCluster := range managedClusters.Items {
		mcvOAuth := managedclusterviewsub.DefaultViewOAuthClient(operatorConfig, managedCluster.Name)
		gvr := managedclusterviewsub.GetGroupVersionResource()

		oAuthResp, oAuthErr := c.dynamicClient.Resource(gvr).Namespace(managedCluster.Name).Create(ctx, mcvOAuth, metav1.CreateOptions{})
		if oAuthErr != nil && apierrors.IsAlreadyExists(oAuthErr) {
			mcvOAuthName, _ := managedclusterviewsub.GetName(mcvOAuth)
			oAuthResp, oAuthErr = c.dynamicClient.Resource(gvr).Namespace(managedCluster.Name).Get(ctx, mcvOAuthName, metav1.GetOptions{})
		}

		if oAuthErr != nil || oAuthResp == nil {
			errs = append(errs, fmt.Sprintf("Error syncing managed cluster view for oauth client for cluster %s: %v", managedCluster.Name, oAuthErr))
		} else {
			managedClusterOAuthClientViews = append(managedClusterOAuthClientViews, oAuthResp)
		}
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n")), "ManagedClusterViewOAuthClientSyncError"
	}

	return managedClusterOAuthClientViews, nil, ""
}

func (c *ManagedClusterController) SyncManagedClusterActionCreateOAuthClient(ctx context.Context, operatorConfig *operatorv1.Console, managedClusterOAuthClientViews []*unstructured.Unstructured) ([]*unstructured.Unstructured, error, string) {
	managedClusterList := []string{}
	managedClusterListErrors := []string{}
	for _, managedClusterOAuthView := range managedClusterOAuthClientViews {
		status, statusErr := managedclusterviewsub.GetStatus(managedClusterOAuthView)
		if !status || statusErr != nil {
			namespace, namespaceErr := managedclusterviewsub.GetNamespace(managedClusterOAuthView)
			if namespaceErr != nil || namespace == "" {
				managedClusterListErrors = append(managedClusterListErrors, fmt.Sprintf("Unable to create oauth client for cluster %v: managed cluster view status unknown", namespace))
			} else {
				managedClusterList = append(managedClusterList, namespace)
			}
		}
	}

	secret, secErr := c.secretsClient.Secrets(api.TargetNamespace).Get(ctx, secretsub.Stub().Name, metav1.GetOptions{})
	if secErr != nil {
		return nil, fmt.Errorf("Failed to get secret: %v", secErr), "ManagedClusterActionCreateOAuthClientSyncError"
	}

	oauthClient, oAuthErr := c.oauthClient.OAuthClients().Get(ctx, oauthsub.Stub().Name, metav1.GetOptions{})
	if oAuthErr != nil {
		return nil, fmt.Errorf("Failed to get oauth client: %v", oAuthErr), "ManagedClusterActionCreateOAuthClientSyncError"
	}

	errs := []string{}
	managedClusterActionCreateOAuthClients := []*unstructured.Unstructured{}
	secretString := secretsub.GetSecretString(secret)
	redirects := oauthsub.GetRedirectURIs(oauthClient)
	for _, managedClusterName := range managedClusterList {
		mca := managedclusteractionsub.DefaultCreateOAuthClient(operatorConfig, managedClusterName, secretString, redirects)
		gvr := managedclusteractionsub.GetGroupVersionResource()
		opt := metav1.CreateOptions{}
		oAuthCreateResp, oAuthCreateErr := c.dynamicClient.Resource(gvr).Namespace(managedClusterName).Create(ctx, mca, opt)
		if oAuthCreateErr != nil && apierrors.IsAlreadyExists(oAuthCreateErr) {
			mcaOAuthName, _ := managedclusteractionsub.GetName(mca)
			oAuthCreateResp, oAuthCreateErr = c.dynamicClient.Resource(gvr).Namespace(managedClusterName).Get(ctx, mcaOAuthName, metav1.GetOptions{})
		}

		if oAuthCreateErr != nil {
			errs = append(errs, fmt.Sprintf("Error syncing managed cluster action for oauth client for cluster %s: %v", managedClusterName, oAuthCreateErr))
		} else {
			managedClusterActionCreateOAuthClients = append(managedClusterActionCreateOAuthClients, oAuthCreateResp)
		}
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n")), "ManagedClusterActionCreateOAuthClientSyncError"
	}

	if len(managedClusterListErrors) > 0 {
		return nil, errors.New(strings.Join(managedClusterListErrors, "\n")), "ManagedClusterActionCreateOAuthClientSyncError"
	}

	return managedClusterActionCreateOAuthClients, nil, ""
}

func (c *ManagedClusterController) SyncManagedClusterViewIngressCert(ctx context.Context, operatorConfig *operatorv1.Console, managedClusters *clusterv1.ManagedClusterList) ([]*unstructured.Unstructured, error, string) {
	errs := []string{}
	managedClusterIngressCertViews := []*unstructured.Unstructured{}
	for _, managedCluster := range managedClusters.Items {
		mcvIngress := managedclusterviewsub.DefaultViewIngressCert(operatorConfig, managedCluster.Name)
		gvr := managedclusterviewsub.GetGroupVersionResource()

		ingressResp, ingressErr := c.dynamicClient.Resource(gvr).Namespace(managedCluster.Name).Create(ctx, mcvIngress, metav1.CreateOptions{})
		if ingressErr != nil && apierrors.IsAlreadyExists(ingressErr) {
			mcvIngressName, _ := managedclusterviewsub.GetName(mcvIngress)
			ingressResp, ingressErr = c.dynamicClient.Resource(gvr).Namespace(managedCluster.Name).Get(ctx, mcvIngressName, metav1.GetOptions{})
		}

		if ingressErr != nil || ingressResp == nil {
			errs = append(errs, fmt.Sprintf("Error syncing managed cluster view ingress cert for cluster %s: %v", managedCluster.Name, ingressErr))
		} else {
			managedClusterIngressCertViews = append(managedClusterIngressCertViews, ingressResp)
		}
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n")), "ManagedClusterViewIngressCertSyncError"
	}

	return managedClusterIngressCertViews, nil, ""
}

func (c *ManagedClusterController) SyncManagedClusterIngressCertConfigMap(managedClusterIngressCertViews []*unstructured.Unstructured, ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder) (error, string) {
	errs := []string{}
	for _, mcvIngress := range managedClusterIngressCertViews {
		clusterName, _ := managedclusterviewsub.GetNamespace(mcvIngress)
		certBundle, _ := managedclusterviewsub.GetCertBundle(mcvIngress)
		required := configmapsub.DefaultManagedClusterIngressCertConfigMap(clusterName, certBundle, operatorConfig)
		_, _, configMapApplyError := resourceapply.ApplyConfigMap(c.configMapClient, recorder, required)
		if configMapApplyError != nil {
			klog.V(4).Infoln(fmt.Sprintf("Skipping Ingress certificate ConfigMap sync for managed cluster %v, Error applying ConfigMap", clusterName))
			errs = append(errs, configMapApplyError.Error())
			continue
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n")), "ManagedClusterIngressCertConfigMapSyncError"
	}

	return nil, ""
}

func (c *ManagedClusterController) removeManagedClusters(ctx context.Context) error {
	errs := []string{}
	configMaps, err := c.configMapClient.ConfigMaps(api.OpenShiftConsoleNamespace).List(ctx, metav1.ListOptions{LabelSelector: api.ManagedClusterLabel})
	if err != nil {
		klog.Errorf("Error listing managed cluster resources to remove: %v", err)
		return err
	}

	if len(configMaps.Items) == 0 {
		klog.Info("No managed cluster resources to remove.")
		return nil
	}

	for _, configMap := range configMaps.Items {
		deletionErr := c.configMapClient.ConfigMaps(api.OpenShiftConsoleNamespace).Delete(ctx, configMap.GetName(), metav1.DeleteOptions{})
		if deletionErr != nil && !apierrors.IsNotFound(deletionErr) {
			errs = append(errs, deletionErr.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

func (c *ManagedClusterController) listManagedClusters(ctx context.Context) (*clusterv1.ManagedClusterList, error) {
	return c.managedClusterClient.ManagedClusters().List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("local-cluster!=true")})
}
