package operator

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacclientv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"

	scsv1 "github.com/openshift/service-ca-operator/pkg/apis/serviceca/v1"
	"github.com/openshift/service-ca-operator/pkg/boilerplate/operator"
	"github.com/openshift/service-ca-operator/pkg/controller/api"
	scsclientv1 "github.com/openshift/service-ca-operator/pkg/generated/clientset/versioned/typed/serviceca/v1"
	scsinformerv1 "github.com/openshift/service-ca-operator/pkg/generated/informers/externalversions/serviceca/v1"
)

const targetNamespaceName = "openshift-service-cert-signer"

type serviceCertSignerOperator struct {
	operatorConfigClient scsclientv1.ServiceCAsGetter

	appsv1Client appsclientv1.AppsV1Interface
	corev1Client coreclientv1.CoreV1Interface
	rbacv1Client rbacclientv1.RbacV1Interface
}

func NewServiceCertSignerOperator(
	serviceCertSignerConfigInformer scsinformerv1.ServiceCAInformer,
	namespacedKubeInformers informers.SharedInformerFactory,
	operatorConfigClient scsclientv1.ServiceCAsGetter,
	appsv1Client appsclientv1.AppsV1Interface,
	corev1Client coreclientv1.CoreV1Interface,
	rbacv1Client rbacclientv1.RbacV1Interface,
) operator.Runner {
	c := &serviceCertSignerOperator{
		operatorConfigClient: operatorConfigClient,

		appsv1Client: appsv1Client,
		corev1Client: corev1Client,
		rbacv1Client: rbacv1Client,
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
	namespaceEvents := operator.FilterByNames(targetNamespaceName)

	return operator.New("ServiceCAOperator", c,
		operator.WithInformer(serviceCertSignerConfigInformer, configEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().ConfigMaps(), configMapEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().ServiceAccounts(), saEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().Services(), serviceEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().Secrets(), secretEvents),
		operator.WithInformer(namespacedKubeInformers.Apps().V1().Deployments(), deploymentEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().Namespaces(), namespaceEvents),
	)
}

func (c serviceCertSignerOperator) Key() (metav1.Object, error) {
	return c.operatorConfigClient.ServiceCAs().Get(api.OperatorConfigInstanceName, metav1.GetOptions{})
}

func (c serviceCertSignerOperator) Sync(obj metav1.Object) error {
	operatorConfig := obj.(*scsv1.ServiceCA)

	switch operatorConfig.Spec.ManagementState {
	case scsv1.Unmanaged, scsv1.Removed, "Paused":
		// Totally disable the sync loop in these states until we bump deps and replace sscs.
		return nil
	}
	// This is to push out deployments but does not handle deployment generation like it used to. It may need tweaking.
	err := sync_v4_00_to_latest(c, operatorConfig)
	return err
}

func getGeneration(client appsclientv1.AppsV1Interface, ns, name string) int64 {
	deployment, err := client.Deployments(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return -1
	}
	return deployment.Generation
}
