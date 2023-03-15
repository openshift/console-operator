package managedproxyserviceresolver

import (
	"context"
	"fmt"
	"time"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"

	operatorsv1 "github.com/openshift/api/operator/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	operatorinformersv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	managedproxyserviceresolversub "github.com/openshift/console-operator/pkg/console/subresource/managedproxyserviceresolver"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	clusterproxyclient "open-cluster-management.io/cluster-proxy/pkg/generated/clientset/versioned/typed/proxy/v1alpha1"
)

type ManagedProxyServiceResolverController struct {
	operatorClient                    v1helpers.OperatorClient
	consoleOperatorConfigClient       operatorclientv1.ConsoleInterface
	crdClient                         apiextensionsclient.CustomResourceDefinitionInterface
	managedProxyServiceResolverClient clusterproxyclient.ManagedProxyServiceResolverInterface
}

const ManagedProxyServiceResolverConditionPrefix = "ManagedProxyServiceResolverSync"

func NewManagedProxyServiceResolverController(
	// clients
	operatorClient v1helpers.OperatorClient,
	consoleOperatorConfigClient operatorclientv1.ConsoleInterface,
	crdClient apiextensionsclient.CustomResourceDefinitionInterface,
	managedProxyServiceResolverClient clusterproxyclient.ManagedProxyServiceResolverInterface,

	// informers
	consoleOperatorConfigInformer operatorinformersv1.ConsoleInformer,
	crdInformer apiextensionsinformers.CustomResourceDefinitionInformer,

	// events
	recorder events.Recorder,
) factory.Controller {

	ctrl := &ManagedProxyServiceResolverController{
		operatorClient:                    operatorClient,
		consoleOperatorConfigClient:       consoleOperatorConfigClient,
		managedProxyServiceResolverClient: managedProxyServiceResolverClient,
		crdClient:                         crdClient,
	}

	return factory.New().
		WithFilteredEventsInformers( // configs
			util.IncludeNamesFilter(api.ConfigResourceName),
			consoleOperatorConfigInformer.Informer(),
		).
		WithFilteredEventsInformers(
			util.IncludeNamesFilter(api.ManagedProxyServiceResolverCRDName),
			crdInformer.Informer(),
		).
		ResyncEvery(time.Minute).
		WithSync(ctrl.Sync).
		ToController(
			"ManagedProxyServiceResolverController",
			recorder.WithComponentSuffix("managed-proxy-service-resolver-controller"),
		)
}

func (c *ManagedProxyServiceResolverController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.consoleOperatorConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	switch operatorConfig.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infoln("console-operator is in a managed state: syncing ManagedProxyServiceResolvers")
	case operatorsv1.Unmanaged:
		klog.V(4).Infoln("console-operator is in an unmanaged state: skipping ManagedProxyServiceResolvers sync")
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infoln("console-operator is in a removed state: deleting ManagedProxyServiceResolvers")
		return c.remove(ctx)
	default:
		return fmt.Errorf("unknown state: %v", operatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	// Confirm that the ManagedProxyServiceResolver API is installed before proceeding
	_, err = c.crdClient.Get(ctx, api.ManagedProxyServiceResolverCRDName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		klog.V(4).Infoln("Skipping ManagedProxyServiceResolver sync. API is not installed.")
		return nil
	}

	if err != nil {
		klog.V(4).Infof("Error getting CustomResourceDefinition for ManagedProxyServiceResolver: %v", err)
		statusHandler.AddConditions(status.HandleProgressingOrDegraded(ManagedProxyServiceResolverConditionPrefix, "GetCustomResourceDefinitionFailed", err))
		return statusHandler.FlushAndReturn(err)
	}

	reason, err := c.SyncThanosQuerier(ctx)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded(ManagedProxyServiceResolverConditionPrefix, reason, err))
	if err != nil {
		klog.Errorf("Error syncing thanos-quierier ManagedProxyServiceResolver: %v", err)
	}

	reason, err = c.SyncAlertManagerMain(ctx)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded(ManagedProxyServiceResolverConditionPrefix, reason, err))
	if err != nil {
		klog.Errorf("Error syncing alertmanager-main ManagedProxyServiceResolver: %v", err)
	}

	return statusHandler.FlushAndReturn(err)
}

func (c *ManagedProxyServiceResolverController) SyncThanosQuerier(ctx context.Context) (string, error) {
	required := managedproxyserviceresolversub.DefaultThanosQuerierManagedProxyServiceResolver()
	err := managedproxyserviceresolversub.ApplyManagedProxyServiceResolver(ctx, c.managedProxyServiceResolverClient, required)
	if err != nil {
		return "ApplyThanosQuerierFailed", err
	}
	return "", nil
}

func (c *ManagedProxyServiceResolverController) SyncAlertManagerMain(ctx context.Context) (string, error) {
	required := managedproxyserviceresolversub.DefaultAlertManagerMainManagedProxyServiceResolver()
	err := managedproxyserviceresolversub.ApplyManagedProxyServiceResolver(ctx, c.managedProxyServiceResolverClient, required)
	if err != nil {
		return "ApplyAlertManagerMainFailed", err
	}
	return "", nil
}

func (c *ManagedProxyServiceResolverController) remove(ctx context.Context) error {
	errs := []error{}

	managedProxyServiceResolverList, err := c.managedProxyServiceResolverClient.List(ctx, metav1.ListOptions{LabelSelector: api.ManagedClusterLabel})
	if err != nil {
		return err
	}

	for _, managedProxyServiceResolver := range managedProxyServiceResolverList.Items {
		err = c.managedProxyServiceResolverClient.Delete(ctx, managedProxyServiceResolver.Name, metav1.DeleteOptions{})
		if err != nil {
			errs = append(errs, err)
		}
	}

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		klog.Errorf("Error removing ManagedProxyServiceResolvers: %v", err)
		return err
	}
	return nil
}
