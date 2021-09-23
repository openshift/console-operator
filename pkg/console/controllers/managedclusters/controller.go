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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
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

	// Get a list of ManagedCluster resources, degraded if error is returned
	managedClusterClientConfigs, managedClusterClientConfigValidationErr, managedClusterClientConfigValidationErrReason := c.ValidateManagedClusterClientConfigs(ctx, operatorConfig, controllerContext.Recorder())
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

	// Create managed cluster oauth views
	managedClusterOAuthViews, _, managedClusterOAuthViewsErrReason, managedClusterOAuthViewsErr := c.SyncManagedClusterOAuthViews(ctx, operatorConfig)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterViewOAuthSync", managedClusterOAuthViewsErrReason, managedClusterOAuthViewsErr))

	shouldCreateOAuthClients := false
	for _, managedClusterOAuthView := range managedClusterOAuthViews {
		status, err := managedclustersub.GetResourceViewStatus(managedClusterOAuthView)
		if !status || err != nil {
			shouldCreateOAuthClients = true
		}
	}

	// Create managed cluster actions IFF they have not already been created (ACM cleans these up)
	if shouldCreateOAuthClients {
		_, _, managedClusterActionsErrReason, managedClusterActionsErr := c.SyncManagedClusterActions(ctx, operatorConfig)
		statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterActionsSync", managedClusterActionsErrReason, managedClusterActionsErr))
	}

	// Create managed cluster ingress views
	_, _, managedClusterIngressViewsErrReason, managedClusterIngressViewsErr := c.SyncManagedClusterIngressViews(ctx, operatorConfig)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterViewIngressSync", managedClusterIngressViewsErrReason, managedClusterIngressViewsErr))

	// Create  manged cluster config map
	configSyncErr, configSyncErrReason := c.SyncManagedClusterConfigMap(managedClusterClientConfigs, ctx, operatorConfig, controllerContext.Recorder())
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterConfigSync", configSyncErrReason, configSyncErr))
	return statusHandler.FlushAndReturn(configSyncErr)
}

// Return slice of clusterv1.ClientConfigs that have been validated or error and reaons if unable to list ManagedClusters
func (c *ManagedClusterController) ValidateManagedClusterClientConfigs(ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder) (map[string]*clusterv1.ClientConfig, error, string) {
	managedClusters, err := c.listManagedClusters(ctx)

	// Not found means API is not on the cluster
	if apierrors.IsNotFound(err) {
		return nil, nil, ""
	}

	// Any other list request failure means operator is degraded
	if err != nil {
		return nil, err, "FailedList"
	}

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

func (c *ManagedClusterController) SyncManagedClusterIngressViews(ctx context.Context, operatorConfig *operatorv1.Console) ([]*unstructured.Unstructured, bool, string, error) {
	managedClusters, listErr := c.listManagedClusters(ctx)
	if listErr != nil || len(managedClusters.Items) == 0 {
		return nil, false, "", fmt.Errorf("Failed to list ManagedClusters: %v", listErr)
	}

	errors := []error{}
	managedClusterIngressViews := []*unstructured.Unstructured{}
	for _, managedCluster := range managedClusters.Items {
		mcvIngress := managedclustersub.DefaultManagedClusterViewIngress(operatorConfig, managedCluster.Name)
		gvr := schema.GroupVersionResource{
			Group:    "view.open-cluster-management.io",
			Version:  "v1beta1",
			Resource: "managedclusterviews",
		}
		opt := metav1.CreateOptions{}
		// TODO create throws an error if this already exists. Need to use patch/put
		ingressResp, ingressErr := c.dynamicClient.Resource(gvr).Namespace(managedCluster.Name).Create(ctx, mcvIngress, opt)
		if ingressErr != nil {
			errors = append(errors, fmt.Errorf("Error syncing managed cluster view ingress for cluster %s: %v", managedCluster.Name, ingressErr))
		} else {
			managedClusterIngressViews = append(managedClusterIngressViews, ingressResp)
		}
	}

	if len(errors) > 0 {
		return nil, false, "", fmt.Errorf("One or more errors syncing managed cluster views: %v", errors)
	}

	return managedClusterIngressViews, true, "", nil
}

func (c *ManagedClusterController) SyncManagedClusterOAuthViews(ctx context.Context, operatorConfig *operatorv1.Console) ([]*unstructured.Unstructured, bool, string, error) {
	managedClusters, listErr := c.listManagedClusters(ctx)
	if listErr != nil || len(managedClusters.Items) == 0 {
		return nil, false, "", fmt.Errorf("Failed to list ManagedClusters: %v", listErr)
	}

	errors := []error{}
	managedClusterOAuthViews := []*unstructured.Unstructured{}
	for _, managedCluster := range managedClusters.Items {
		mcvOAuth := managedclustersub.DefaultManagedClusterViewOAuth(operatorConfig, managedCluster.Name)
		gvr := schema.GroupVersionResource{
			Group:    "view.open-cluster-management.io",
			Version:  "v1beta1",
			Resource: "managedclusterviews",
		}
		opt := metav1.CreateOptions{}
		oAuthResp, oAuthErr := c.dynamicClient.Resource(gvr).Namespace(managedCluster.Name).Create(ctx, mcvOAuth, opt)
		if oAuthErr != nil {
			errors = append(errors, fmt.Errorf("Error syncing managed cluster view oauth for cluster %s: %v", managedCluster.Name, oAuthErr))
		} else {
			managedClusterOAuthViews = append(managedClusterOAuthViews, oAuthResp)
		}
	}

	if len(errors) > 0 {
		return nil, false, "", fmt.Errorf("One or more errors syncing managed cluster views: %v", errors)
	}

	return managedClusterOAuthViews, true, "", nil
}

func (c *ManagedClusterController) SyncManagedClusterActions(ctx context.Context, operatorConfig *operatorv1.Console) ([]*unstructured.Unstructured, bool, string, error) {
	managedClusters, listErr := c.listManagedClusters(ctx)
	if listErr != nil || len(managedClusters.Items) == 0 {
		return nil, false, "", fmt.Errorf("Failed to list ManagedClusters: %v", listErr)
	}

	secret, secErr := c.secretsClient.Secrets(api.TargetNamespace).Get(ctx, secretsub.Stub().Name, metav1.GetOptions{})
	if secErr != nil {
		return nil, false, "", fmt.Errorf("Failed to get secret: %v", secErr)
	}

	oauthClient, oAuthErr := c.oauthClient.OAuthClients().Get(ctx, oauthsub.Stub().Name, metav1.GetOptions{})
	if oAuthErr != nil {
		return nil, false, "", fmt.Errorf("Failed to get oauthclient: %v", oAuthErr)
	}

	errors := []error{}
	managedClusterActions := []*unstructured.Unstructured{}
	secretString := secretsub.GetSecretString(secret)
	redirects := oauthsub.GetRedirectURIs(oauthClient)
	for _, managedCluster := range managedClusters.Items {
		mca := managedclustersub.DefaultManagedClusterActionOAuthCreate(operatorConfig, managedCluster.Name, secretString, redirects)
		gvr := schema.GroupVersionResource{
			Group:    "action.open-cluster-management.io",
			Version:  "v1beta1",
			Resource: "managedclusteractions",
		}
		opt := metav1.CreateOptions{}
		resp, err := c.dynamicClient.Resource(gvr).Namespace(managedCluster.Name).Create(ctx, mca, opt)
		if err != nil {
			errors = append(errors, fmt.Errorf("Error syncing managed cluster action for cluster %s: %v", managedCluster.Name, err))
		} else {
			managedClusterActions = append(managedClusterActions, resp)
		}
	}

	if len(errors) > 0 {
		return nil, false, "", fmt.Errorf("One or more errors syncing managed cluster actions: %v", errors)
	}

	return managedClusterActions, true, "", nil
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
