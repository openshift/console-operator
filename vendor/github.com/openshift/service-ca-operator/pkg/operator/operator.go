package operator

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacclientv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/status"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/service-ca-operator/pkg/boilerplate/operator"
	"github.com/openshift/service-ca-operator/pkg/controller/api"
	"github.com/openshift/service-ca-operator/pkg/operator/operatorclient"
)

type serviceCAOperator struct {
	operatorClient *operatorclient.OperatorClient

	appsv1Client  appsclientv1.AppsV1Interface
	corev1Client  coreclientv1.CoreV1Interface
	rbacv1Client  rbacclientv1.RbacV1Interface
	versionGetter status.VersionGetter
	eventRecorder events.Recorder
}

func NewServiceCAOperator(
	operatorClient *operatorclient.OperatorClient,

	namespacedKubeInformers informers.SharedInformerFactory,
	appsv1Client appsclientv1.AppsV1Interface,
	corev1Client coreclientv1.CoreV1Interface,
	rbacv1Client rbacclientv1.RbacV1Interface,
	versionGetter status.VersionGetter,
	eventRecorder events.Recorder,
) operator.Runner {
	c := &serviceCAOperator{
		operatorClient: operatorClient,

		appsv1Client:  appsv1Client,
		corev1Client:  corev1Client,
		rbacv1Client:  rbacv1Client,
		versionGetter: versionGetter,
		eventRecorder: eventRecorder,
	}

	configEvents := operator.FilterByNames(api.OperatorConfigInstanceName)
	configMapEvents := operator.FilterByNames(
		api.SignerControllerConfigMapName,
		api.APIServiceInjectorConfigMapName,
		api.ConfigMapInjectorConfigMapName,
		api.SigningCABundleConfigMapName,
	)
	saEvents := operator.FilterByNames(
		api.SignerControllerSAName,
		api.APIServiceInjectorSAName,
		api.ConfigMapInjectorSAName,
	)
	serviceEvents := operator.FilterByNames(api.SignerControllerServiceName)
	secretEvents := operator.FilterByNames(api.SignerControllerSecretName)
	deploymentEvents := operator.FilterByNames(
		api.SignerControllerDeploymentName,
		api.APIServiceInjectorDeploymentName,
		api.ConfigMapInjectorDeploymentName,
	)
	namespaceEvents := operator.FilterByNames(operatorclient.TargetNamespace)

	return operator.New("ServiceCAOperator", c,
		operator.WithInformer(namespacedKubeInformers.Core().V1().ConfigMaps(), configMapEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().ServiceAccounts(), saEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().Services(), serviceEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().Secrets(), secretEvents),
		operator.WithInformer(namespacedKubeInformers.Apps().V1().Deployments(), deploymentEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().Namespaces(), namespaceEvents),
		operator.WithInformer(operatorClient.Informers.Operator().V1().ServiceCAs(), configEvents),
	)
}

func (c serviceCAOperator) Key() (metav1.Object, error) {
	return c.operatorClient.Client.ServiceCAs().Get(api.OperatorConfigInstanceName, metav1.GetOptions{})
}

func (c serviceCAOperator) Sync(obj metav1.Object) error {
	operatorConfig := obj.(*operatorv1.ServiceCA)

	operatorConfigCopy := operatorConfig.DeepCopy()
	switch operatorConfigCopy.Spec.ManagementState {
	case operatorv1.Unmanaged, operatorv1.Removed, "Paused":
		// Totally disable the sync loop in these states until we bump deps and replace sscs.
		return nil
	case operatorv1.Managed:
		// This is to push out deployments but does not handle deployment generation like it used to. It may need tweaking.
		err := syncControllers(c, operatorConfigCopy)
		if err != nil {
			setFailingTrue(operatorConfigCopy, "OperatorSyncLoopError", err.Error())
		} else {
			if v1helpers.IsOperatorConditionTrue(operatorConfigCopy.Status.Conditions, operatorv1.OperatorStatusTypeFailing) {
				setFailingFalse(operatorConfigCopy, "OperatorSyncLoopComplete")
			}
			existingDeployments, err := c.appsv1Client.Deployments(operatorclient.TargetNamespace).List(metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("Error listing deployments in %s: %v", operatorclient.TargetNamespace, err)
			}
			c.syncStatus(operatorConfigCopy, existingDeployments, targetDeploymentNames)
		}
		c.updateStatus(operatorConfigCopy)
		return err
	}
	return nil
}

func getGeneration(client appsclientv1.AppsV1Interface, ns, name string) int64 {
	deployment, err := client.Deployments(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return -1
	}
	return deployment.Generation
}

func (c serviceCAOperator) updateStatus(operatorConfig *operatorv1.ServiceCA) {
	v1helpers.UpdateStatus(c.operatorClient, func(status *operatorv1.OperatorStatus) error {
		operatorConfig.Status.OperatorStatus.DeepCopyInto(status)
		return nil
	})
}
