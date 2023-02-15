package managedclusteroauthclient

import (
	"context"
	"fmt"
	"time"

	// k8s
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	// openshift
	oauthv1 "github.com/openshift/api/oauth/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	oauthv1client "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	ouathinformers "github.com/openshift/client-go/oauth/informers/externalversions/oauth/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	operatorinformersv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	clusterclientv1 "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1"
	workclientv1 "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"

	//subresources
	manifestworksub "github.com/openshift/console-operator/pkg/console/subresource/manifestwork"
	oauthsub "github.com/openshift/console-operator/pkg/console/subresource/oauthclient"

	// console-operator
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
)

const ConditionPrefix string = "ManagedClusterOauthClientSync"

type Controller struct {
	operatorClient       v1helpers.OperatorClient
	operatorConfigClient operatorclientv1.ConsoleInterface
	oauthClient          oauthv1client.OAuthClientsGetter
	managedClusterClient clusterclientv1.ManagedClustersGetter
	workClient           workclientv1.ManifestWorksGetter
}

func NewManagedClusterOAuthClientController(
	// top level config
	configClient configclientv1.ConfigV1Interface,
	configInformer configinformer.SharedInformerFactory,

	// clients
	operatorClient v1helpers.OperatorClient,
	operatorConfigClient operatorclientv1.ConsoleInterface,
	managedClusterClient clusterclientv1.ManagedClustersGetter,
	oauthClient oauthv1client.OAuthClientsGetter,
	workClient workclientv1.ManifestWorksGetter,

	// informers
	operatorConfigInformer operatorinformersv1.ConsoleInformer,
	oauthClientInformer ouathinformers.OAuthClientInformer,

	// events
	recorder events.Recorder,
) factory.Controller {
	ctrl := &Controller{
		operatorClient:       operatorClient,
		operatorConfigClient: operatorConfigClient,
		managedClusterClient: managedClusterClient,
		oauthClient:          oauthClient,
		workClient:           workClient,
	}

	return factory.New().
		WithFilteredEventsInformers( // configs
			util.IncludeNamesFilter(api.ConfigResourceName),
			configInformer.Config().V1().Consoles().Informer(),
			operatorConfigInformer.Informer(),
		).
		WithFilteredEventsInformers(
			util.IncludeNamesFilter(api.OAuthClientName),
			oauthClientInformer.Informer(),
		).
		ResyncEvery(1*time.Minute).
		WithSync(ctrl.Sync).
		ToController("ManagedClusterOAuthClientController", recorder.WithComponentSuffix("managed-cluster-oauth-client-controller"))
}

func (c *Controller) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.operatorConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	switch operatorConfig.Spec.ManagementState {
	case operatorv1.Managed:
		klog.V(4).Info("console-operator is in a managed state: syncing managed cluster oauth clients")
	case operatorv1.Unmanaged:
		klog.V(4).Info("console-operator is in an unmanaged state: skipping managed cluster oauth client sync")
		return nil
	case operatorv1.Removed:
		klog.V(4).Infof("console-operator is in a removed state: deleting managed cluster oauth clients")
		return c.remove(ctx)
	default:
		return fmt.Errorf("unknown state: %v", operatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	// Get the local OAuthClient. If this fails, do not proceed. We can't create OAuth clients on
	// managed clusters without the local client.
	localOAuthClient, reason, err := c.GetLocalOAuthClient(ctx)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded(ConditionPrefix, reason, err))
	if err != nil {
		klog.V(4).Infof("Error getting local OAuthClient: %v", err)
		return c.remove(ctx)
	}

	managedClusters, reason, err := util.GetValidManagedClusters(ctx, c.managedClusterClient.ManagedClusters())
	statusHandler.AddConditions(status.HandleProgressingOrDegraded(ConditionPrefix, reason, err))
	if err != nil || len(managedClusters) == 0 {
		klog.V(4).Infof("Error listing ManagedClusters: %v", err)
		return c.remove(ctx)
	}

	// Create ManifestWorks which will create oauth clients on each managed cluster
	errs := []error{}
	for _, managedCluster := range managedClusters {
		err = c.SyncOauthClientManifestWork(
			ctx,
			operatorConfig,
			managedCluster.Name,
			localOAuthClient.Secret,
			localOAuthClient.RedirectURIs,
		)
		if err != nil {
			klog.V(4).Infof("Error syncing OAuthClient ManifestWork for managed cluster %q: %v.\n", managedCluster.Name, err)
			errs = append(errs, err)
		}
	}

	err = utilerrors.NewAggregate(errs)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded(ConditionPrefix, "ApplyManifestWorks", err))
	return statusHandler.FlushAndReturn(err)
}

func (c *Controller) GetLocalOAuthClient(ctx context.Context) (*oauthv1.OAuthClient, string, error) {
	oAuthClient, err := c.oauthClient.OAuthClients().Get(ctx, oauthsub.Stub().Name, metav1.GetOptions{})
	if err != nil {
		return nil, "GetLocalOAuthClientFailed", err
	}
	return oAuthClient, "", nil
}

func (c *Controller) SyncManagedClusters(ctx context.Context) (*clusterv1.ManagedClusterList, string, error) {
	managedClusterList, err := c.managedClusterClient.ManagedClusters().List(ctx, metav1.ListOptions{})
	if err != nil {
		return managedClusterList, "ListManagedClustersFailed", err
	}
	return managedClusterList, "", nil
}

func (c *Controller) SyncOauthClientManifestWork(
	ctx context.Context,
	operatorConfig *operatorv1.Console,
	namespace string,
	clientSecret string,
	redirectUris []string,
) error {
	required := manifestworksub.DefaultManagedClusterOAuthClientManifestWork(operatorConfig, namespace, clientSecret, redirectUris)
	_, err := manifestworksub.ApplyManifestWork(ctx, c.workClient.ManifestWorks(namespace), required)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) remove(ctx context.Context) error {
	manifestWorks, err := c.workClient.ManifestWorks("").List(ctx, metav1.ListOptions{LabelSelector: api.ManagedClusterLabel})
	if err != nil {
		return err
	}

	errors := []error{}
	for _, manifestWork := range manifestWorks.Items {
		err := c.workClient.ManifestWorks(manifestWork.Namespace).Delete(ctx, manifestWork.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			errors = append(errors, err)
		}
	}
	return utilerrors.NewAggregate(errors)
}
