package managedcluster

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	oauthv1 "github.com/openshift/api/oauth/v1"
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

	// console-operator
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	"github.com/openshift/console-operator/pkg/console/subresource/consoleserver"
)

type ManagedClusterController struct {
	operatorClient       v1helpers.OperatorClient
	operatorConfigClient operatorclientv1.ConsoleInterface
	featureGateClient    configclientv1.FeatureGateInterface
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
		featureGateClient:    configClient.FeatureGates(),
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
			configV1Informers.FeatureGates().Informer(),
		).ResyncEvery(1*time.Minute).WithSync(ctrl.Sync).
		ToController("ManagedClusterController", recorder.WithComponentSuffix("managed-cluster-controller"))
}

func (c *ManagedClusterController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {

	// Get cluster FeatureGate config
	featureGateConfig, err := c.featureGateClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Error getting FeatureGate config: %v", err)
		return nil
	}

	// Check that the "TechPreviewNoUpgrade" feature set is configured, else exit the sync loop
	featureSet := string(featureGateConfig.Spec.FeatureSet)
	if featureSet == "" || !strings.Contains(featureSet, "TechPreviewNoUpgrade") {
		return nil
	}

	klog.V(4).Info("Tech preview is enabled. Running managed cluster sync")

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

	// Get the local OAuthClient. If this fails, do not proceed. We can't create OAuth clients on
	// managed clusters without the local client.
	localOAuthClient, errReason, err := c.SyncLocalOAuthClient(ctx)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", errReason, err))
	if err != nil {
		return c.removeManagedClusters(ctx)
	}

	// Get a list of validated ManagedCluster resources
	managedClusters, errReason, err := c.SyncManagedClusterList(ctx)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", errReason, err))
	if err != nil || len(managedClusters) == 0 {
		return c.removeManagedClusters(ctx)
	}

	// Create managed cluster views for oauth clients
	oAuthClientMCVs, errReason, err := c.SyncOAuthClientManagedClusterViews(ctx, operatorConfig, managedClusters)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", errReason, err))

	// Create managed cluster actions for oauth clients
	errReason, err = c.SyncOAuthClientCreationManagedClusterActions(ctx, operatorConfig, localOAuthClient, oAuthClientMCVs)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", errReason, err))

	// Create managed cluster views for ingress cert
	oAuthServerCertMCVs, errReason, err := c.SyncOAuthServerCertManagedClusterViews(ctx, operatorConfig, managedClusters)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", errReason, err))

	// Create config maps for each managed cluster ingress cert bundle
	errReason, err = c.SyncOAuthServerCertConfigMaps(oAuthServerCertMCVs, ctx, operatorConfig, controllerContext.Recorder())
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", errReason, err))

	// Create config maps for each managed cluster API server ca bundle
	errReason, err = c.SyncAPIServerCertConfigMaps(managedClusters, ctx, operatorConfig, controllerContext.Recorder())
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", errReason, err))
	if err != nil {
		return statusHandler.FlushAndReturn(err)
	}

	// Create  manged cluster config map
	errReason, err = c.SyncManagedClusterConfigMap(managedClusters, ctx, operatorConfig, controllerContext.Recorder())
	statusHandler.AddConditions(status.HandleProgressingOrDegraded("ManagedClusterSync", errReason, err))
	return statusHandler.FlushAndReturn(err)
}

func (c *ManagedClusterController) SyncLocalOAuthClient(ctx context.Context) (*oauthv1.OAuthClient, string, error) {
	oAuthClient, oAuthErr := c.oauthClient.OAuthClients().Get(ctx, oauthsub.Stub().Name, metav1.GetOptions{})
	if oAuthErr != nil {
		return nil, "LocalOAuthClientSyncError", fmt.Errorf("Failed to get oauth client: %v", oAuthErr)
	}

	return oAuthClient, "", nil
}

func (c *ManagedClusterController) SyncManagedClusterList(ctx context.Context) ([]clusterv1.ManagedCluster, string, error) {
	managedClusters, err := c.managedClusterClient.ManagedClusters().List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("local-cluster!=true")})

	// Not degraded, API is not found which means ACM isn't installed
	if apierrors.IsNotFound(err) {
		return nil, "", nil
	}

	if err != nil {
		return nil, "ErrorListingManagedClusters", err
	}

	valid := []clusterv1.ManagedCluster{}
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

		valid = append(valid, managedCluster)
	}

	return valid, "", nil
}

func (c *ManagedClusterController) SyncOAuthClientManagedClusterViews(ctx context.Context, operatorConfig *operatorv1.Console, managedClusters []clusterv1.ManagedCluster) ([]*unstructured.Unstructured, string, error) {
	errs := []string{}
	mcvs := []*unstructured.Unstructured{}
	for _, managedCluster := range managedClusters {
		mcv, err := c.dynamicClient.Resource(api.ManagedClusterViewGroupVersionResource).Namespace(managedCluster.Name).Get(ctx, api.OAuthClientManagedClusterViewName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			required, err := managedclusterviewsub.DefaultOAuthClientView(managedCluster.Name)
			if err != nil {
				errs = append(errs, fmt.Sprintf("Error initializing oauth client ManagedClusterView for cluster %s: %v", managedCluster.Name, err))
				continue
			}
			mcv, err = c.dynamicClient.Resource(api.ManagedClusterViewGroupVersionResource).Namespace(managedCluster.Name).Create(ctx, required, metav1.CreateOptions{})
		}

		if err != nil || mcv == nil {
			errs = append(errs, fmt.Sprintf("Error syncing managed cluster view for oauth client for cluster %s: %v", managedCluster.Name, err))
		} else {
			mcvs = append(mcvs, mcv)
		}
	}

	if len(errs) > 0 {
		return nil, "ManagedClusterViewOAuthClientSyncError", errors.New(strings.Join(errs, "\n"))
	}

	return mcvs, "", nil
}

func (c *ManagedClusterController) SyncOAuthClientCreationManagedClusterActions(ctx context.Context, operatorConfig *operatorv1.Console, localOAuthClient *oauthv1.OAuthClient, oAuthClientMCVs []*unstructured.Unstructured) (string, error) {
	managedClusterList := []string{}
	managedClusterListErrors := []string{}
	for _, managedClusterOAuthView := range oAuthClientMCVs {
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

	errs := []string{}
	secretString := oauthsub.GetSecretString(localOAuthClient)
	redirects := oauthsub.GetRedirectURIs(localOAuthClient)
	for _, managedClusterName := range managedClusterList {
		_, err := c.dynamicClient.Resource(api.ManagedClusterActionGroupVersionResource).Namespace(managedClusterName).Get(ctx, api.CreateOAuthClientManagedClusterActionName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			required, err := managedclusteractionsub.DefaultCreateOAuthClientAction(managedClusterName, secretString, redirects)
			if err != nil {
				errs = append(errs, fmt.Sprintf("Error initializing oauth client ManagedClusterAction for cluster %s: %v", managedClusterName, err))
				continue
			}
			_, err = c.dynamicClient.Resource(api.ManagedClusterActionGroupVersionResource).Namespace(managedClusterName).Create(ctx, required, metav1.CreateOptions{})
		}

		if err != nil {
			errs = append(errs, fmt.Sprintf("Error syncing managed cluster action for oauth client for cluster %s: %v", managedClusterName, err))
		}
	}

	if len(errs) > 0 {
		return "ManagedClusterActionCreateOAuthClientSyncError", errors.New(strings.Join(errs, "\n"))
	}

	if len(managedClusterListErrors) > 0 {
		return "ManagedClusterActionCreateOAuthClientSyncError", errors.New(strings.Join(managedClusterListErrors, "\n"))
	}

	return "", nil
}

func (c *ManagedClusterController) SyncOAuthServerCertManagedClusterViews(ctx context.Context, operatorConfig *operatorv1.Console, managedClusters []clusterv1.ManagedCluster) ([]*unstructured.Unstructured, string, error) {
	errs := []string{}
	mcvs := []*unstructured.Unstructured{}
	for _, managedCluster := range managedClusters {
		mcv, err := c.dynamicClient.Resource(api.ManagedClusterViewGroupVersionResource).Namespace(managedCluster.Name).Get(ctx, api.OAuthServerCertManagedClusterViewName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			required, err := managedclusterviewsub.DefaultOAuthServerCertView(operatorConfig, managedCluster.Name)
			if err != nil {
				errs = append(errs, fmt.Sprintf("Error initializing oauth server cert ManagedClusterView for cluster %s: %v", managedCluster.Name, err))
				continue
			}
			mcv, err = c.dynamicClient.Resource(api.ManagedClusterViewGroupVersionResource).Namespace(managedCluster.Name).Create(ctx, required, metav1.CreateOptions{})
		}

		if err != nil || mcv == nil {
			errs = append(errs, fmt.Sprintf("Error syncing oauth server cert ManagedClusterView for cluster %s: %v", managedCluster.Name, err))
		} else {
			mcvs = append(mcvs, mcv)
		}
	}

	if len(errs) > 0 {
		return nil, "OAuthServerCertManagedClusterViewSyncError", errors.New(strings.Join(errs, "\n"))
	}

	return mcvs, "", nil
}

func (c *ManagedClusterController) SyncOAuthServerCertConfigMaps(oAuthServerCertMCVs []*unstructured.Unstructured, ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder) (string, error) {
	errs := []string{}
	for _, oAuthServerCertMCV := range oAuthServerCertMCVs {
		clusterName, _ := managedclusterviewsub.GetNamespace(oAuthServerCertMCV)
		certBundle, _ := managedclusterviewsub.GetCertBundle(oAuthServerCertMCV)
		if certBundle == "" {
			klog.V(4).Infoln(fmt.Sprintf("Skipping OAuth server certificate ConfigMap sync for managed cluster %v, cert bundle is empty", clusterName))
			continue
		}

		required := configmapsub.DefaultManagedClusterOAuthServerCertConfigMap(clusterName, certBundle, operatorConfig)
		_, _, configMapApplyError := resourceapply.ApplyConfigMap(ctx, c.configMapClient, recorder, required)
		if configMapApplyError != nil {
			klog.V(4).Infoln(fmt.Sprintf("Skipping OAuth server certificate ConfigMap sync for managed cluster %v, Error applying ConfigMap", clusterName))
			errs = append(errs, configMapApplyError.Error())
			continue
		}
	}

	if len(errs) > 0 {
		return "ManagedClusterIngressCertConfigMapSyncError", errors.New(strings.Join(errs, "\n"))
	}

	return "", nil
}

// Using ManagedCluster.spec.ManagedClusterClientConfigs, sync ConfigMaps containing the API server CA bundle for each managed cluster
// If a managed cluster doesn't have complete client config yet, the information is logged, but no error is returned
// If applying any ConfigMap fails, an error and reason are returned
func (c *ManagedClusterController) SyncAPIServerCertConfigMaps(managedClusters []clusterv1.ManagedCluster, ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder) (string, error) {
	errs := []string{}
	for _, managedCluster := range managedClusters {
		// Apply the config map. If this fails for any managed cluster, operator is degraded
		clusterName := managedCluster.GetName()
		caBundle := managedCluster.Spec.ManagedClusterClientConfigs[0].CABundle
		required := configmapsub.DefaultAPIServerCAConfigMap(managedCluster.GetName(), caBundle, operatorConfig)
		_, _, configMapApplyError := resourceapply.ApplyConfigMap(ctx, c.configMapClient, recorder, required)
		if configMapApplyError != nil {
			klog.V(4).Infoln(fmt.Sprintf("Skipping API server CA ConfigMap sync for managed cluster %v, Error applying ConfigMap", clusterName))
			errs = append(errs, configMapApplyError.Error())
			continue
		}
	}

	// Return any apply errors that occurred
	if len(errs) > 0 {
		return "APIServerCAConfigMapSyncError", errors.New(strings.Join(errs, "\n"))
	}

	// Success
	return "", nil
}

// Using ManagedClusters.Spec.ManagedClusterClientConfigs and previously synced CA bundles, sync a ConfigMap containing serverconfig.ManagedClusterConfig YAML for each managed cluster
// If a managed cluster doesn't have an API server CA bundle ConfigMap yet or the client config is incomplete, this is logged, but no error is returned
// If applying the ConfigMap fails, an error and reason are returned
func (c *ManagedClusterController) SyncManagedClusterConfigMap(managedClusters []clusterv1.ManagedCluster, ctx context.Context, operatorConfig *operatorv1.Console, recorder events.Recorder) (string, error) {
	managedClusterConfigs := []consoleserver.ManagedClusterConfig{}
	for _, managedCluster := range managedClusters {
		clusterName := managedCluster.GetName()
		klog.V(4).Infoln(fmt.Sprintf("Building config for managed cluster: %v", clusterName))

		// Check that managed cluster API server CA ConfigMap has already been synced, skip if not found
		_, err := c.configMapClient.ConfigMaps(api.OpenShiftConsoleNamespace).Get(ctx, configmapsub.APIServerCAConfigMapName(clusterName), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("API server CA file not found for managed cluster %v", clusterName)
			continue
		}

		// Skip if unable to get managed cluster API server config map for any other reason
		if err != nil {
			klog.V(4).Infof("Error getting API server CA file for managed cluster %v", clusterName)
			continue
		}

		// Check that managed cluster OAuth server CA ConfigMap has already been synced, skip if not found
		_, err = c.configMapClient.ConfigMaps(api.OpenShiftConsoleNamespace).Get(ctx, configmapsub.ManagedClusterOAuthServerCertConfigMapName(clusterName), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("OAuth server CA file not found for managed cluster %v", clusterName)
			continue
		}

		// Skip if unable to get managed cluster OAuth server config map for any other reason
		if err != nil {
			klog.V(4).Infof("Error getting OAuth server CA file for managed cluster %v", clusterName)
			continue
		}

		// Check that managed cluster OAuth client MCV has already been synced, skip if not found
		oAuthClientMCV, err := c.dynamicClient.Resource(api.ManagedClusterViewGroupVersionResource).Namespace(managedCluster.Name).Get(ctx, api.OAuthClientManagedClusterViewName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("OAuth client ManagedClusterView not found for managed cluster %v", clusterName)
			continue
		}

		// Skip if unable to get managed cluster OAuth client MCV for any other reason
		if err != nil {
			klog.V(4).Infof("Error getting OAuth client ManagedClusterView for managed cluster %v", clusterName)
			continue
		}

		oAuthClientSecret, err := managedclusterviewsub.GetOAuthClientSecret(oAuthClientMCV)
		if err != nil || oAuthClientSecret == "" {
			klog.V(4).Infof("Error getting OAuth client secret for managed cluster %v", clusterName)
			continue
		}

		managedClusterConfigs = append(managedClusterConfigs, consoleserver.ManagedClusterConfig{
			Name: clusterName,
			APIServer: consoleserver.ManagedClusterAPIServerConfig{
				URL:    managedCluster.Spec.ManagedClusterClientConfigs[0].URL,
				CAFile: configmapsub.APIServerCAFileMountPath(clusterName),
			},
			Oauth: consoleserver.ManagedClusterOAuthConfig{
				CAFile:       configmapsub.ManagedClusterOAuthServerCAFileMountPath(clusterName),
				ClientID:     api.ManagedClusterOAuthClientName,
				ClientSecret: oAuthClientSecret,
			},
		})
	}

	if len(managedClusterConfigs) > 0 {
		required, err := configmapsub.DefaultManagedClustersConfigMap(operatorConfig, managedClusterConfigs)
		if err != nil {
			return "FailedMarshallingYAML", err
		}

		if _, _, applyErr := resourceapply.ApplyConfigMap(ctx, c.configMapClient, recorder, required); applyErr != nil {
			return "FailedApply", applyErr
		}
	}

	return "", nil
}

func (c *ManagedClusterController) removeManagedClusters(ctx context.Context) error {
	klog.V(4).Info("Removing managed cluster resources.")
	errs := []string{}
	err := c.removeManagedClusterConfigMaps(ctx)
	if err != nil {
		errs = append(errs, err.Error())
	}

	err = c.removeManagedClusterActions(ctx)
	if err != nil {
		errs = append(errs, err.Error())
	}

	err = c.removeManagedClusterViews(ctx)
	if err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		klog.Errorf("Errors were encountered while removing managed cluster resources: %v", errs)
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

func (c *ManagedClusterController) removeManagedClusterActions(ctx context.Context) error {
	errs := []string{}
	mcas, err := c.dynamicClient.Resource(api.ManagedClusterViewGroupVersionResource).List(ctx, metav1.ListOptions{LabelSelector: api.ManagedClusterLabel})

	if apierrors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return err
	}

	if len(mcas.Items) == 0 {
		return nil
	}

	for _, mca := range mcas.Items {
		deletionErr := c.dynamicClient.Resource(api.ManagedClusterViewGroupVersionResource).Namespace(mca.GetNamespace()).Delete(ctx, mca.GetName(), metav1.DeleteOptions{})
		if deletionErr != nil && !apierrors.IsNotFound(deletionErr) {
			errs = append(errs, deletionErr.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

func (c *ManagedClusterController) removeManagedClusterViews(ctx context.Context) error {
	errs := []string{}
	mcvs, err := c.dynamicClient.Resource(api.ManagedClusterViewGroupVersionResource).List(ctx, metav1.ListOptions{LabelSelector: api.ManagedClusterLabel})

	if apierrors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return err
	}

	if len(mcvs.Items) == 0 {
		return nil
	}

	for _, mcv := range mcvs.Items {
		deletionErr := c.dynamicClient.Resource(api.ManagedClusterViewGroupVersionResource).Namespace(mcv.GetNamespace()).Delete(ctx, mcv.GetName(), metav1.DeleteOptions{})
		if deletionErr != nil && !apierrors.IsNotFound(deletionErr) {
			errs = append(errs, deletionErr.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

func (c *ManagedClusterController) removeManagedClusterConfigMaps(ctx context.Context) error {
	errs := []string{}
	configMaps, err := c.configMapClient.ConfigMaps(api.OpenShiftConsoleNamespace).List(ctx, metav1.ListOptions{LabelSelector: api.ManagedClusterLabel})

	if err != nil {
		return err
	}

	if len(configMaps.Items) == 0 {
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
