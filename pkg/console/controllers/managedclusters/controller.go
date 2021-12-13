package managedcluster

import (
	"context"
	"fmt"

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
	managedclustersub "github.com/openshift/console-operator/pkg/console/subresource/managedcluster"
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
		dynamicInformers.ForResource(managedclustersub.GetActionGroupVersionResource()).Informer(),
	).WithInformers(
		dynamicInformers.ForResource(managedclustersub.GetViewGroupVersionResource()).Informer(),
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
	managedClusterClientConfigs, managedClusterClientConfigValidationErrReason, managedClusterClientConfigValidationErr := c.ValidateManagedClusterClientConfigs(ctx, operatorConfig, controllerContext.Recorder(), managedClusters)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", managedClusterClientConfigValidationErrReason, managedClusterClientConfigValidationErr))
	if managedClusterClientConfigValidationErr != nil {
		return statusHandler.FlushAndReturn(managedClusterClientConfigValidationErr)
	}

	// No managed clusters exist, quit sync
	if len(managedClusterClientConfigs) == 0 {
		return statusHandler.FlushAndReturn(nil)
	}

	// Create config maps for each managed cluster API server ca bundle
	apiServerCASyncErrReason, apiServerCASyncErr := c.SyncAPIServerCAConfigMaps(managedClusterClientConfigs, ctx, operatorConfig, controllerContext.Recorder())
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", apiServerCASyncErrReason, apiServerCASyncErr))
	if apiServerCASyncErr != nil {
		return statusHandler.FlushAndReturn(apiServerCASyncErr)
	}

	// Create managed cluster views for oauth clients
	managedClusterViewOAuthClients, managedClusterViewOAuthClientErrReason, managedClusterViewOAuthClientErr := c.SyncManagedClusterViewOAuthClient(ctx, operatorConfig, managedClusters)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", managedClusterViewOAuthClientErrReason, managedClusterViewOAuthClientErr))
	if managedClusterViewOAuthClientErr != nil {
		return statusHandler.FlushAndReturn(managedClusterViewOAuthClientErr)
	}

	// Create managed cluster actions for oauth clients
	_, managedClusterActionCreateOAuthClientErrReason, managedClusterActionCreateOAuthClientErr := c.SyncManagedClusterActionCreateOAuthClient(ctx, operatorConfig, managedClusterViewOAuthClients)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", managedClusterActionCreateOAuthClientErrReason, managedClusterActionCreateOAuthClientErr))

	// Create managed cluster views for ingress cert
	managedClusterViewsIngressCert, managedClusterViewIngressCertErrReason, managedClusterViewIngressCertErr := c.SyncManagedClusterViewIngressCert(ctx, operatorConfig, managedClusters)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", managedClusterViewIngressCertErrReason, managedClusterViewIngressCertErr))

	// Create config maps for each managed cluster ingress cert bundle
	managedClusterIngressCertSyncErrReason, managedClusterIngressCertSyncErr := c.SyncManagedClusterIngressCertConfigMap(managedClusterViewsIngressCert, ctx, operatorConfig, controllerContext.Recorder())
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", managedClusterIngressCertSyncErrReason, managedClusterIngressCertSyncErr))

	// Create  manged cluster config map
	configSyncErrReason, configSyncErr := c.SyncManagedClusterConfigMap(managedClusterClientConfigs, ctx, operatorConfig, controllerContext.Recorder())
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", configSyncErrReason, configSyncErr))
	return statusHandler.FlushAndReturn(configSyncErr)
}

// Return slice of clusterv1.ClientConfigs that have been validated or error and its reason, if unable to list ManagedClusters
func (c *ManagedClusterController) ValidateManagedClusterClientConfigs(ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder, managedClusters *clusterv1.ManagedClusterList) (map[string]*clusterv1.ClientConfig, string, error) {
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

	return validatedClientConfigs, "", nil
}

// Using ManagedCluster.spec.ManagedClusterClientConfigs, sync ConfigMaps containing the API server CA bundle for each managed cluster
// If a managed cluster doesn't have complete client config yet, the information is logged, but no error is returned
// If applying any ConfigMap fails, an error and reason are returned
func (c *ManagedClusterController) SyncAPIServerCAConfigMaps(clientConfigs map[string]*clusterv1.ClientConfig, ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder) (string, error) {
	errs := []error{}
	for clusterName, clientConfig := range clientConfigs {
		// Apply the config map. If this fails for any managed cluster, operator is degraded
		required := configmapsub.DefaultAPIServerCAConfigMap(clusterName, clientConfig.CABundle, operatorConfig)
		_, _, configMapApplyError := resourceapply.ApplyConfigMap(c.configMapClient, recorder, required)
		if configMapApplyError != nil {
			klog.V(4).Infoln(fmt.Sprintf("error applying %s ConfigMap: %v. Skipping API server CA ConfigMap sync for managed cluster %s", required.GetName(), configMapApplyError, clusterName))
			errs = append(errs, configMapApplyError)
			continue
		}
	}

	// Return any apply errors that occurred
	if len(errs) > 0 {
		return "APIServerCAConfigMapSyncError", fmt.Errorf("one or more errors during API server CA ConfigMap sync: %v", errs)
	}

	// Success
	return "", nil
}

// Using ManagedClusters.Spec.ManagedClusterClientConfigs and previously synced CA bundles, sync a ConfigMap containing serverconfig.ManagedClusterConfig YAML for each managed cluster
// If a managed cluster doesn't have an API server CA bundle ConfigMap yet or the client config is incomplete, this is logged, but no error is returned
// If applying the ConfigMap fails, an error and reason are returned
func (c *ManagedClusterController) SyncManagedClusterConfigMap(clientConfigs map[string]*clusterv1.ClientConfig, ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder) (string, error) {
	managedClusterConfigs := []consoleserver.ManagedClusterConfig{}
	for clusterName, clientConfig := range clientConfigs {
		klog.V(4).Infoln(fmt.Sprintf("Building config for managed cluster: %s", clusterName))

		// Check that managed cluster CA ConfigMap has already been synced, skip if not found
		_, err := c.configMapClient.ConfigMaps(api.OpenShiftConsoleNamespace).Get(ctx, configmapsub.APIServerCAConfigMapName(clusterName), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("CA file not found for managed cluster %s", clusterName)
			continue
		}

		// Skip if unable to get managed cluster API server config map for any other reason
		if err != nil {
			klog.V(4).Infof("Error getting CA file for managed cluster %s", clusterName)
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
			return "FailedMarshallingYAML", err
		}

		if _, _, applyErr := resourceapply.ApplyConfigMap(c.configMapClient, recorder, required); applyErr != nil {
			return "FailedApply", applyErr
		}
	}

	return "", nil
}

func (c *ManagedClusterController) SyncManagedClusterViewOAuthClient(ctx context.Context, operatorConfig *operatorv1.Console, managedClusters *clusterv1.ManagedClusterList) ([]*unstructured.Unstructured, string, error) {
	errs := []error{}
	managedClusterOAuthClientViews := []*unstructured.Unstructured{}
	for _, managedCluster := range managedClusters.Items {
		mcvOAuth := managedclustersub.DefaultViewOAuthClient(operatorConfig, managedCluster.Name)

		oAuthResp, oAuthErr := c.dynamicClient.Resource(managedclustersub.GetViewGroupVersionResource()).Namespace(managedCluster.Name).Create(ctx, mcvOAuth, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(oAuthErr) {
			mcvOAuthName, _ := managedclustersub.GetName(mcvOAuth)
			oAuthResp, oAuthErr = c.dynamicClient.Resource(managedclustersub.GetViewGroupVersionResource()).Namespace(managedCluster.Name).Get(ctx, mcvOAuthName, metav1.GetOptions{})
		}

		if oAuthErr != nil || oAuthResp == nil {
			errs = append(errs, fmt.Errorf("error syncing ManagedClusterView for oauth client for cluster %s: %v", managedCluster.Name, oAuthErr))
		} else {
			managedClusterOAuthClientViews = append(managedClusterOAuthClientViews, oAuthResp)
		}
	}

	if len(errs) > 0 {
		return nil, "ManagedClusterViewOAuthClientSyncError", fmt.Errorf("one or more errors during ManagedClusterView oauth client sync: %v", errs)
	}

	return managedClusterOAuthClientViews, "", nil
}

func (c *ManagedClusterController) SyncManagedClusterActionCreateOAuthClient(ctx context.Context, operatorConfig *operatorv1.Console, managedClusterOAuthClientViews []*unstructured.Unstructured) ([]*unstructured.Unstructured, string, error) {
	managedClusterList := []string{}
	managedClusterListErrors := []error{}
	for _, managedClusterOAuthView := range managedClusterOAuthClientViews {
		status, _ := managedclustersub.GetStatus(managedClusterOAuthView)
		namespace, namespaceErr := managedclustersub.GetNamespace(managedClusterOAuthView)
		if status {
			klog.V(4).Infof("Skipping creating oauth client action for managed cluster %s: already exists", namespace)
			continue
		}
		if namespaceErr != nil || namespace == "" {
			klog.V(4).Infof("Error retrieving namespace of managed cluster from ManagedClusterView: %v", namespaceErr)
			managedClusterListErrors = append(managedClusterListErrors, fmt.Errorf("unable to create oauth client for cluster %s: ManagedClusterView status unknown", namespace))
			continue
		}
		managedClusterList = append(managedClusterList, namespace)
	}

	secret, secErr := c.secretsClient.Secrets(api.TargetNamespace).Get(ctx, secretsub.Stub().Name, metav1.GetOptions{})
	if secErr != nil {
		return nil, "ManagedClusterActionCreateOAuthClientSyncError", fmt.Errorf("failed to get secret: %v", secErr)
	}

	oauthClient, oAuthErr := c.oauthClient.OAuthClients().Get(ctx, oauthsub.Stub().Name, metav1.GetOptions{})
	if oAuthErr != nil {
		return nil, "ManagedClusterActionCreateOAuthClientSyncError", fmt.Errorf("failed to get oauth client: %v", oAuthErr)
	}

	errs := []error{}
	managedClusterActionCreateOAuthClients := []*unstructured.Unstructured{}
	secretString := secretsub.GetSecretString(secret)
	redirects := oauthsub.GetRedirectURIs(oauthClient)
	for _, managedClusterName := range managedClusterList {
		mca := managedclustersub.DefaultCreateOAuthClient(operatorConfig, managedClusterName, secretString, redirects)
		gvr := managedclustersub.GetActionGroupVersionResource()
		oAuthCreateResp, oAuthCreateErr := c.dynamicClient.Resource(gvr).Namespace(managedClusterName).Create(ctx, mca, metav1.CreateOptions{})
		if oAuthCreateErr != nil && apierrors.IsAlreadyExists(oAuthCreateErr) {
			mcaOAuthName, _ := managedclustersub.GetName(mca)
			oAuthCreateResp, oAuthCreateErr = c.dynamicClient.Resource(gvr).Namespace(managedClusterName).Get(ctx, mcaOAuthName, metav1.GetOptions{})
		}

		if oAuthCreateErr != nil {
			errs = append(errs, fmt.Errorf("error syncing ManagedClusterAction for oauth client for cluster %s: %v", managedClusterName, oAuthCreateErr))
		} else {
			managedClusterActionCreateOAuthClients = append(managedClusterActionCreateOAuthClients, oAuthCreateResp)
		}
	}

	if len(errs) > 0 {
		return nil, "ManagedClusterActionCreateOAuthClientSyncError", fmt.Errorf("one or more errors during ManagedClusterAction create oauth client sync: %v", errs)
	}

	if len(managedClusterListErrors) > 0 {
		return nil, "ManagedClusterActionCreateOAuthClientSyncError", fmt.Errorf("one or more errors listing managed clusters from ManagedClusterViews: %v", managedClusterListErrors)
	}

	return managedClusterActionCreateOAuthClients, "", nil
}

func (c *ManagedClusterController) SyncManagedClusterViewIngressCert(ctx context.Context, operatorConfig *operatorv1.Console, managedClusters *clusterv1.ManagedClusterList) ([]*unstructured.Unstructured, string, error) {
	errs := []error{}
	managedClusterIngressCertViews := []*unstructured.Unstructured{}
	for _, managedCluster := range managedClusters.Items {
		mcvIngress := managedclustersub.DefaultViewIngressCert(operatorConfig, managedCluster.Name)

		ingressResp, ingressErr := c.dynamicClient.Resource(managedclustersub.GetViewGroupVersionResource()).Namespace(managedCluster.Name).Create(ctx, mcvIngress, metav1.CreateOptions{})
		if ingressErr != nil && apierrors.IsAlreadyExists(ingressErr) {
			mcvIngressName, _ := managedclustersub.GetName(mcvIngress)
			ingressResp, ingressErr = c.dynamicClient.Resource(managedclustersub.GetViewGroupVersionResource()).Namespace(managedCluster.Name).Get(ctx, mcvIngressName, metav1.GetOptions{})
		}

		if ingressErr != nil {
			errs = append(errs, fmt.Errorf("error syncing ManagedClusterView ingress cert for cluster %s: %v", managedCluster.Name, ingressErr))
			continue
		}
		managedClusterIngressCertViews = append(managedClusterIngressCertViews, ingressResp)
	}

	if len(errs) > 0 {
		return nil, "ManagedClusterViewIngressCertSyncError", fmt.Errorf("one or more errors during ManagedClusterView ingress cert sync: %v", errs)
	}

	return managedClusterIngressCertViews, "", nil
}

func (c *ManagedClusterController) SyncManagedClusterIngressCertConfigMap(managedClusterIngressCertViews []*unstructured.Unstructured, ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder) (string, error) {
	errs := []error{}
	for _, mcvIngress := range managedClusterIngressCertViews {
		clusterName, _ := managedclustersub.GetNamespace(mcvIngress)
		certBundle, _ := managedclustersub.GetCertBundle(mcvIngress)
		required := configmapsub.DefaultManagedClusterIngressCertConfigMap(clusterName, certBundle, operatorConfig)
		_, _, configMapApplyError := resourceapply.ApplyConfigMap(c.configMapClient, recorder, required)
		if configMapApplyError != nil {
			klog.V(4).Infoln(fmt.Sprintf("Skipping Ingress certificate ConfigMap sync for managed cluster %v, Error applying ConfigMap", clusterName))
			errs = append(errs, configMapApplyError)
			continue
		}
	}

	if len(errs) > 0 {
		return "ManagedClusterIngressCertConfigMapSyncError", fmt.Errorf("one or more errors during managed cluster ingress cert configmap sync: %v", errs)
	}

	return "", nil
}

// TODO use an object in the controller struct or a label to track resources to be removed
func (c *ManagedClusterController) removeManagedClusters(ctx context.Context) error {
	errs := []error{}
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
			errs = append(errs, deletionErr)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("one or more errors during removal of managed clusters: %v", errs)
	}
	return nil
}

func (c *ManagedClusterController) listManagedClusters(ctx context.Context) (*clusterv1.ManagedClusterList, error) {
	return c.managedClusterClient.ManagedClusters().List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("local-cluster!=true")})
}
