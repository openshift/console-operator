package operator

import (
	"fmt"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned"
	operatorv1informers "github.com/openshift/client-go/operator/informers/externalversions"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/status"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/openshift/service-ca-operator/pkg/controller/api"
	"github.com/openshift/service-ca-operator/pkg/operator/operatorclient"
	"github.com/openshift/service-ca-operator/pkg/operator/v4_00_assets"
)

const (
	resyncDuration      = 10 * time.Minute
	clusterOperatorName = "service-ca"
)

var deploymentNames []string = []string{
	api.SignerControllerDeploymentName,
	api.APIServiceInjectorDeploymentName,
	api.ConfigMapInjectorDeploymentName,
}

func RunOperator(ctx *controllercmd.ControllerContext) error {

	// This kube client uses protobuf, do not use it for CRs
	kubeClient, err := kubernetes.NewForConfig(ctx.ProtoKubeConfig)
	if err != nil {
		return err
	}
	operatorConfigClient, err := operatorv1client.NewForConfig(ctx.KubeConfig)
	if err != nil {
		return err
	}
	dynamicClient, err := dynamic.NewForConfig(ctx.KubeConfig)
	if err != nil {
		return err
	}
	configClient, err := configv1client.NewForConfig(ctx.KubeConfig)
	if err != nil {
		return err
	}
	configInformers := configv1informers.NewSharedInformerFactory(configClient, 10*time.Minute)
	operatorConfigInformers := operatorv1informers.NewSharedInformerFactory(operatorConfigClient, resyncDuration)
	kubeInformersNamespaced := informers.NewFilteredSharedInformerFactory(kubeClient, resyncDuration, operatorclient.TargetNamespace, nil)
	kubeInformersForNamespaces := v1helpers.NewKubeInformersForNamespaces(kubeClient,
		"",
		operatorclient.GlobalUserSpecifiedConfigNamespace,
		operatorclient.GlobalMachineSpecifiedConfigNamespace,
		operatorclient.OperatorNamespace,
		operatorclient.TargetNamespace,
	)
	v1helpers.EnsureOperatorConfigExists(
		dynamicClient,
		v4_00_assets.MustAsset("v4.0.0/service-ca-operator/operator-config.yaml"),
		operatorv1.GroupVersion.WithResource("servicecas"))

	operatorClient := &operatorclient.OperatorClient{
		Informers: operatorConfigInformers,
		Client:    operatorConfigClient.OperatorV1(),
	}

	versionGetter := status.NewVersionGetter()

	clusterOperatorStatus := status.NewClusterOperatorStatusController(
		clusterOperatorName,
		[]configv1.ObjectReference{
			{Group: operatorv1.GroupName, Resource: "servicecas", Name: api.OperatorConfigInstanceName},
			{Resource: "namespaces", Name: operatorclient.GlobalUserSpecifiedConfigNamespace},
			{Resource: "namespaces", Name: operatorclient.GlobalMachineSpecifiedConfigNamespace},
			{Resource: "namespaces", Name: operatorclient.OperatorNamespace},
			{Resource: "namespaces", Name: operatorclient.TargetNamespace},
		},
		configClient.ConfigV1(),
		configInformers.Config().V1().ClusterOperators(),
		operatorClient,
		versionGetter,
		ctx.EventRecorder,
	)

	resourceSyncController := resourcesynccontroller.NewResourceSyncController(
		operatorClient,
		kubeInformersForNamespaces,
		v1helpers.CachedSecretGetter(kubeClient.CoreV1(), kubeInformersForNamespaces),
		v1helpers.CachedConfigMapGetter(kubeClient.CoreV1(), kubeInformersForNamespaces),
		ctx.EventRecorder,
	)
	if err := resourceSyncController.SyncConfigMap(
		resourcesynccontroller.ResourceLocation{Namespace: operatorclient.GlobalMachineSpecifiedConfigNamespace, Name: clusterOperatorName},
		resourcesynccontroller.ResourceLocation{Namespace: operatorclient.TargetNamespace, Name: api.SigningCABundleConfigMapName},
	); err != nil {
		return err
	}

	operator := NewServiceCAOperator(
		operatorConfigInformers.Operator().V1().ServiceCAs(),
		kubeInformersNamespaced,
		operatorClient.Client,
		kubeClient.AppsV1(),
		kubeClient.CoreV1(),
		kubeClient.RbacV1(),
		versionGetter,
		ctx.EventRecorder,
	)

	operatorConfigInformers.Start(ctx.Done())
	configInformers.Start(ctx.Done())
	kubeInformersNamespaced.Start(ctx.Done())
	kubeInformersForNamespaces.Start(ctx.Done())

	go operator.Run(ctx.Done())
	go clusterOperatorStatus.Run(1, ctx.Done())
	go resourceSyncController.Run(1, ctx.Done())

	<-ctx.Done()
	return fmt.Errorf("stopped")
}
