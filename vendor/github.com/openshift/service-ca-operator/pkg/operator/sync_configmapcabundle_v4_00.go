package operator

import (
	"fmt"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"

	scsv1 "github.com/openshift/service-ca-operator/pkg/apis/serviceca/v1"
	"github.com/openshift/service-ca-operator/pkg/operator/operatorclient"
	"github.com/openshift/service-ca-operator/pkg/operator/v4_00_assets"
)

// syncConfigMapCABundleController_v4_00_to_latest takes care of synchronizing (not upgrading) the thing we're managing.
// most of the time the sync method will be good for a large span of minor versions
func syncConfigMapCABundleController_v4_00_to_latest(c serviceCAOperator, operatorConfig *scsv1.ServiceCA) error {
	var err error

	requiredNamespace := resourceread.ReadNamespaceV1OrDie(v4_00_assets.MustAsset("v4.0.0/configmap-cabundle-controller/ns.yaml"))
	if _, _, err = resourceapply.ApplyNamespace(c.corev1Client, c.eventRecorder, requiredNamespace); err != nil {
		return fmt.Errorf("%q: %v", "ns", err)
	}

	requiredClusterRole := resourceread.ReadClusterRoleV1OrDie(v4_00_assets.MustAsset("v4.0.0/configmap-cabundle-controller/clusterrole.yaml"))
	if _, _, err = resourceapply.ApplyClusterRole(c.rbacv1Client, c.eventRecorder, requiredClusterRole); err != nil {
		return fmt.Errorf("%q: %v", "clusterrole", err)
	}

	requiredClusterRoleBinding := resourceread.ReadClusterRoleBindingV1OrDie(v4_00_assets.MustAsset("v4.0.0/configmap-cabundle-controller/clusterrolebinding.yaml"))
	if _, _, err = resourceapply.ApplyClusterRoleBinding(c.rbacv1Client, c.eventRecorder, requiredClusterRoleBinding); err != nil {
		return fmt.Errorf("%q: %v", "clusterrolebinding", err)
	}

	requiredRole := resourceread.ReadRoleV1OrDie(v4_00_assets.MustAsset("v4.0.0/configmap-cabundle-controller/role.yaml"))
	if _, _, err = resourceapply.ApplyRole(c.rbacv1Client, c.eventRecorder, requiredRole); err != nil {
		return fmt.Errorf("%q: %v", "role", err)
	}

	requiredRoleBinding := resourceread.ReadRoleBindingV1OrDie(v4_00_assets.MustAsset("v4.0.0/configmap-cabundle-controller/rolebinding.yaml"))
	if _, _, err = resourceapply.ApplyRoleBinding(c.rbacv1Client, c.eventRecorder, requiredRoleBinding); err != nil {
		return fmt.Errorf("%q: %v", "rolebinding", err)
	}

	requiredSA := resourceread.ReadServiceAccountV1OrDie(v4_00_assets.MustAsset("v4.0.0/configmap-cabundle-controller/sa.yaml"))
	_, saModified, err := resourceapply.ApplyServiceAccount(c.corev1Client, c.eventRecorder, requiredSA)
	if err != nil {
		return fmt.Errorf("%q: %v", "sa", err)
	}

	// TODO create a new configmap whenever the data value changes
	_, configMapModified, err := manageConfigMapCABundleConfigMap_v4_00_to_latest(c.corev1Client, c.eventRecorder, operatorConfig)
	if err != nil {
		return fmt.Errorf("%q: %v", "configmap", err)
	}

	_, signingCABundleModified, err := manageConfigMapCABundle(c.corev1Client, c.eventRecorder)
	if err != nil {
		return fmt.Errorf("%q: %v", "cabundle", err)
	}

	var forceDeployment bool
	if saModified { // SA modification can cause new tokens
		forceDeployment = true
	}
	if signingCABundleModified {
		forceDeployment = true
	}
	if configMapModified {
		forceDeployment = true
	}

	// we have attempted to update our configmaps and secrets, now it is time to create the DS
	// TODO check basic preconditions here
	_, _, err = manageConfigMapCABundleDeployment_v4_00_to_latest(c.appsv1Client, c.eventRecorder, operatorConfig, forceDeployment)
	return err
}

func manageConfigMapCABundleConfigMap_v4_00_to_latest(client coreclientv1.ConfigMapsGetter, eventRecorder events.Recorder, operatorConfig *scsv1.ServiceCA) (*corev1.ConfigMap, bool, error) {
	configMap := resourceread.ReadConfigMapV1OrDie(v4_00_assets.MustAsset("v4.0.0/configmap-cabundle-controller/cm.yaml"))
	defaultConfig := v4_00_assets.MustAsset("v4.0.0/configmap-cabundle-controller/defaultconfig.yaml")
	requiredConfigMap, _, err := resourcemerge.MergeConfigMap(configMap, "controller-config.yaml", nil, defaultConfig, operatorConfig.Spec.ConfigMapCABundleInjectorConfig.Raw)
	if err != nil {
		return nil, false, err
	}
	return resourceapply.ApplyConfigMap(client, eventRecorder, requiredConfigMap)
}

func manageConfigMapCABundleDeployment_v4_00_to_latest(client appsclientv1.AppsV1Interface, eventRecorder events.Recorder, options *scsv1.ServiceCA, forceDeployment bool) (*appsv1.Deployment, bool, error) {
	required := resourceread.ReadDeploymentV1OrDie(v4_00_assets.MustAsset("v4.0.0/configmap-cabundle-controller/deployment.yaml"))
	required.Spec.Template.Spec.Containers[0].Image = os.Getenv("CONTROLLER_IMAGE")
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%s", options.Spec.LogLevel))

	return resourceapply.ApplyDeployment(client, eventRecorder, required, getGeneration(client, operatorclient.TargetNamespace, required.Name), forceDeployment)
}

// TODO manage rotation in addition to initial creation
func manageConfigMapCABundle(client coreclientv1.CoreV1Interface, eventRecorder events.Recorder) (*corev1.ConfigMap, bool, error) {
	configMap := resourceread.ReadConfigMapV1OrDie(v4_00_assets.MustAsset("v4.0.0/configmap-cabundle-controller/signing-cabundle.yaml"))
	existing, err := client.ConfigMaps(configMap.Namespace).Get(configMap.Name, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		return existing, false, err
	}

	secret := resourceread.ReadSecretV1OrDie(v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/signing-secret.yaml"))
	currentSigningKeySecret, err := client.Secrets(secret.Namespace).Get(secret.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return existing, false, err
	}
	if err != nil {
		return existing, false, err
	}
	if len(currentSigningKeySecret.Data["tls.crt"]) == 0 {
		return existing, false, err
	}

	configMap.Data["ca-bundle.crt"] = string(currentSigningKeySecret.Data["tls.crt"])

	return resourceapply.ApplyConfigMap(client, eventRecorder, configMap)
}
