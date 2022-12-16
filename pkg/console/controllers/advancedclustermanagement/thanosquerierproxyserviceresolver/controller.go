package thanosquerierproxyserviceresolver

import (
	"context"
	"fmt"
	"time"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	operatorsv1 "github.com/openshift/api/operator/v1"
	operatorclientv1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	operatorinformersv1 "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/controllers/util"
	"github.com/openshift/console-operator/pkg/console/status"
	managedproxyserviceresolver "github.com/openshift/console-operator/pkg/console/subresource/managedserviceproxyresolver"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	clusterproxyclient "open-cluster-management.io/cluster-proxy/pkg/generated/clientset/versioned/typed/proxy/v1alpha1"
)

type ThanosQuerierProxyResolverController struct {
	operatorClient                    v1helpers.OperatorClient
	consoleOperatorConfigClient       operatorclientv1.ConsoleInterface
	crdClient                         apiextensionsclient.CustomResourceDefinitionInterface
	managedProxyServiceResolverClient clusterproxyclient.ManagedProxyServiceResolverInterface
}

const ThanosQuerierProxyResolverConditionPrefix = "ThanosQuerierProxyServiceResolverSync"

func NewThanosQuerierProxyServiceResolverController(
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

	ctrl := &ThanosQuerierProxyResolverController{
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
			"ThanosQuerierProxyServiceResolverController",
			recorder.WithComponentSuffix("thanos-querier-proxy-service-resolver-controller"),
		)
}

func (c *ThanosQuerierProxyResolverController) Sync(ctx context.Context, controllerContext factory.SyncContext) error {
	operatorConfig, err := c.consoleOperatorConfigClient.Get(ctx, api.ConfigResourceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	switch operatorConfig.Spec.ManagementState {
	case operatorsv1.Managed:
		klog.V(4).Infoln("console-operator is in a managed state: syncing ManagedServiceProxyResolver for thanos-querier")
	case operatorsv1.Unmanaged:
		klog.V(4).Infoln("console-operator is in an unmanaged state: skipping thanos-querier ManagedServiceProxyResolver sync")
		return nil
	case operatorsv1.Removed:
		klog.V(4).Infoln("console-operator is in a removed state: deleting thanos-querier ManagedServiceProxyResolver")
		return c.removeThanosQuerierServiceResolver(ctx)
	default:
		return fmt.Errorf("unknown state: %v", operatorConfig.Spec.ManagementState)
	}

	statusHandler := status.NewStatusHandler(c.operatorClient)

	// Confirm that the ManagedProxyServiceResolver API is installed before proceeding
	_, err = c.crdClient.Get(ctx, api.ManagedProxyServiceResolverCRDName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		klog.V(4).Infoln("Skipping thanos-querier ManagedProxyServiceResolver sync. API is missing.")
		return nil
	}

	if err != nil {
		klog.V(4).Infof("Error getting ManagedProxyServiceResolver CRD: %s/n", err)
		statusHandler.AddConditions(status.HandleProgressingOrDegraded(ThanosQuerierProxyResolverConditionPrefix, "GetManagedProxyServiceResolverCRDFailed", err))
		return statusHandler.FlushAndReturn(err)
	}

	reason, err := c.SyncThanosQuerierServiceResolver(ctx)
	statusHandler.AddConditions(status.HandleProgressingOrDegraded(ThanosQuerierProxyResolverConditionPrefix, reason, err))
	return statusHandler.FlushAndReturn(err)
}

func (c *ThanosQuerierProxyResolverController) SyncThanosQuerierServiceResolver(ctx context.Context) (string, error) {
	required := managedproxyserviceresolver.DefaultThanosQuerierProxyServiceResolver()
	err := managedproxyserviceresolver.ApplyManagedProxyServiceResolver(ctx, c.managedProxyServiceResolverClient, required)
	if err != nil {
		return "ApplyManagedProxyServiceResolverFailed", err
	}
	return "", nil
}

func (c *ThanosQuerierProxyResolverController) removeThanosQuerierServiceResolver(ctx context.Context) error {
	required := managedproxyserviceresolver.DefaultThanosQuerierProxyServiceResolver()
	err := c.managedProxyServiceResolverClient.Delete(ctx, required.Name, metav1.DeleteOptions{})
	if !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
