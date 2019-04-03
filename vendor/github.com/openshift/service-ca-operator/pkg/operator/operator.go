package operator

import (
	"os"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacclientv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/status"

	operatorv1 "github.com/openshift/api/operator/v1"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	operatorv1informers "github.com/openshift/client-go/operator/informers/externalversions/operator/v1"
	"github.com/openshift/service-ca-operator/pkg/boilerplate/operator"
	"github.com/openshift/service-ca-operator/pkg/controller/api"
	"github.com/openshift/service-ca-operator/pkg/operator/operatorclient"
)

type serviceCAOperator struct {
	operatorConfigClient   operatorv1client.ServiceCAsGetter
	operatorConfigInformer operatorv1informers.ServiceCAInformer

	appsv1Client appsclientv1.AppsV1Interface
	corev1Client coreclientv1.CoreV1Interface
	rbacv1Client rbacclientv1.RbacV1Interface

	versionGetter status.VersionGetter
	eventRecorder events.Recorder
}

func NewServiceCAOperator(
	operatorConfigInformer operatorv1informers.ServiceCAInformer,
	namespacedKubeInformers informers.SharedInformerFactory,
	operatorConfigClient operatorv1client.ServiceCAsGetter,
	appsv1Client appsclientv1.AppsV1Interface,
	corev1Client coreclientv1.CoreV1Interface,
	rbacv1Client rbacclientv1.RbacV1Interface,
	versionGetter status.VersionGetter,
	eventRecorder events.Recorder,
) operator.Runner {
	c := &serviceCAOperator{
		operatorConfigClient: operatorConfigClient,

		appsv1Client: appsv1Client,
		corev1Client: corev1Client,
		rbacv1Client: rbacv1Client,

		eventRecorder: eventRecorder,
		versionGetter: versionGetter,
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
		operator.WithInformer(operatorConfigInformer, configEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().ConfigMaps(), configMapEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().ServiceAccounts(), saEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().Services(), serviceEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().Secrets(), secretEvents),
		operator.WithInformer(namespacedKubeInformers.Apps().V1().Deployments(), deploymentEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().Namespaces(), namespaceEvents),
	)
}

func (c serviceCAOperator) Key() (metav1.Object, error) {
	return c.operatorConfigClient.ServiceCAs().Get(api.OperatorConfigInstanceName, metav1.GetOptions{})
}

func (c serviceCAOperator) Sync(obj metav1.Object) error {
	operatorConfig := obj.(*operatorv1.ServiceCA)
	setOperatorVersion := false

	operatorConfigCopy := operatorConfig.DeepCopy()
	switch operatorConfigCopy.Spec.ManagementState {
	case operatorv1.Unmanaged, operatorv1.Removed, "Paused":
		// Totally disable the sync loop in these states until we bump deps and replace sscs.
		return nil
	case operatorv1.Managed:
		// This is to push out deployments but does not handle deployment generation like it used to. It may need tweaking.
		err := syncControllers(c, operatorConfigCopy)
		if err != nil {
			c.setFailingStatus(operatorConfigCopy, "OperatorSyncLoopError", err.Error())
		} else {
			setOperatorVersion, err = c.syncStatus(operatorConfigCopy, deploymentNames)
			if err != nil {
				return err
			}
		}
		if setOperatorVersion {
			version := os.Getenv("OPERATOR_IMAGE_VERSION")
			if c.versionGetter.GetVersions()["operator"] != version {
				glog.Infof("Updating clusteroperator %s version: %v", clusterOperatorName, version)
				// Set current version
				c.versionGetter.SetVersion("operator", version)
			}
		}
	}
	// update status to be available, progressing or failing
	if !equality.Semantic.DeepEqual(operatorConfig, operatorConfigCopy) {
		if _, err := c.operatorConfigClient.ServiceCAs().UpdateStatus(operatorConfigCopy); err != nil {
			return err
		}
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
