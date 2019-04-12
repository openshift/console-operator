package operator

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/service-ca-operator/pkg/operator/v4_00_assets"
)

func manageControllerNS(c serviceCAOperator) (bool, error) {
	_, modified, err := resourceapply.ApplyNamespace(c.corev1Client, c.eventRecorder, resourceread.ReadNamespaceV1OrDie(v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/ns.yaml")))
	return modified, err
}

func manageSignerControllerResources(c serviceCAOperator, modified *bool) error {
	return manageControllerResources(c, "v4.0.0/service-serving-cert-signer-controller/", modified)
}

func manageAPIServiceControllerResources(c serviceCAOperator, modified *bool) error {
	return manageControllerResources(c, "v4.0.0/apiservice-cabundle-controller/", modified)
}

func manageConfigMapCABundleControllerResources(c serviceCAOperator, modified *bool) error {
	return manageControllerResources(c, "v4.0.0/configmap-cabundle-controller/", modified)
}

func manageControllerResources(c serviceCAOperator, resourcePath string, modified *bool) error {
	var err error
	requiredClusterRole := resourceread.ReadClusterRoleV1OrDie(v4_00_assets.MustAsset(resourcePath + "clusterrole.yaml"))
	_, mod, err := resourceapply.ApplyClusterRole(c.rbacv1Client, c.eventRecorder, requiredClusterRole)
	if err != nil {
		return err
	}
	*modified = *modified || mod

	requiredClusterRoleBinding := resourceread.ReadClusterRoleBindingV1OrDie(v4_00_assets.MustAsset(resourcePath + "clusterrolebinding.yaml"))
	_, mod, err = resourceapply.ApplyClusterRoleBinding(c.rbacv1Client, c.eventRecorder, requiredClusterRoleBinding)
	if err != nil {
		return err
	}
	*modified = *modified || mod

	requiredRole := resourceread.ReadRoleV1OrDie(v4_00_assets.MustAsset(resourcePath + "role.yaml"))
	_, mod, err = resourceapply.ApplyRole(c.rbacv1Client, c.eventRecorder, requiredRole)
	if err != nil {
		return err
	}
	*modified = *modified || mod

	requiredRoleBinding := resourceread.ReadRoleBindingV1OrDie(v4_00_assets.MustAsset(resourcePath + "rolebinding.yaml"))
	_, mod, err = resourceapply.ApplyRoleBinding(c.rbacv1Client, c.eventRecorder, requiredRoleBinding)
	if err != nil {
		return err
	}
	*modified = *modified || mod

	requiredSA := resourceread.ReadServiceAccountV1OrDie(v4_00_assets.MustAsset(resourcePath + "sa.yaml"))
	_, mod, err = resourceapply.ApplyServiceAccount(c.corev1Client, c.eventRecorder, requiredSA)
	if err != nil {
		return err
	}
	*modified = *modified || mod

	return nil
}

// TODO manage rotation in addition to initial creation
func manageSignerCA(client coreclientv1.SecretsGetter, eventRecorder events.Recorder) (bool, error) {
	secret := resourceread.ReadSecretV1OrDie(v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/signing-secret.yaml"))
	_, err := client.Secrets(secret.Namespace).Get(secret.Name, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		return false, err
	}
	name := serviceServingCertSignerName()
	glog.V(4).Infof("generating signing CA: %s", name)

	ca, err := crypto.MakeSelfSignedCAConfig(name, 365)
	if err != nil {
		return false, err
	}

	certBytes := &bytes.Buffer{}
	keyBytes := &bytes.Buffer{}
	if err := ca.WriteCertConfig(certBytes, keyBytes); err != nil {
		return false, err
	}

	secret.Data["tls.crt"] = certBytes.Bytes()
	secret.Data["tls.key"] = keyBytes.Bytes()
	_, mod, err := resourceapply.ApplySecret(client, eventRecorder, secret)
	return mod, err
}

func manageSignerCABundle(client coreclientv1.CoreV1Interface, eventRecorder events.Recorder, forceUpdate bool) (bool, error) {
	configMap := resourceread.ReadConfigMapV1OrDie(v4_00_assets.MustAsset("v4.0.0/apiservice-cabundle-controller/signing-cabundle.yaml"))
	if !forceUpdate {
		// We don't need to force an update; return if the configmap already exists (or error getting).
		_, err := client.ConfigMaps(configMap.Namespace).Get(configMap.Name, metav1.GetOptions{})
		if !apierrors.IsNotFound(err) {
			return false, err
		}
	}

	glog.V(4).Infof("updating CA bundle configmap")
	secret := resourceread.ReadSecretV1OrDie(v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/signing-secret.yaml"))
	currentSigningKeySecret, err := client.Secrets(secret.Namespace).Get(secret.Name, metav1.GetOptions{})
	// Return err or if the signing secret has no data (should not normally happen).
	if err != nil || len(currentSigningKeySecret.Data["tls.crt"]) == 0 {
		return false, err
	}

	configMap.Data["ca-bundle.crt"] = string(currentSigningKeySecret.Data["tls.crt"])

	_, mod, err := resourceapply.ApplyConfigMap(client, eventRecorder, configMap)
	return mod, err
}

func manageSignerControllerConfig(client coreclientv1.ConfigMapsGetter, eventRecorder events.Recorder) (bool, error) {
	configMap := resourceread.ReadConfigMapV1OrDie(v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/cm.yaml"))
	defaultConfig := v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/defaultconfig.yaml")
	requiredConfigMap, _, err := resourcemerge.MergeConfigMap(configMap, "controller-config.yaml", nil, defaultConfig)
	if err != nil {
		return false, err
	}
	_, mod, err := resourceapply.ApplyConfigMap(client, eventRecorder, requiredConfigMap)
	return mod, err
}

func manageAPIServiceControllerConfig(client coreclientv1.ConfigMapsGetter, eventRecorder events.Recorder) (bool, error) {
	configMap := resourceread.ReadConfigMapV1OrDie(v4_00_assets.MustAsset("v4.0.0/apiservice-cabundle-controller/cm.yaml"))
	defaultConfig := v4_00_assets.MustAsset("v4.0.0/apiservice-cabundle-controller/defaultconfig.yaml")
	requiredConfigMap, _, err := resourcemerge.MergeConfigMap(configMap, "controller-config.yaml", nil, defaultConfig)
	if err != nil {
		return false, err
	}
	_, mod, err := resourceapply.ApplyConfigMap(client, eventRecorder, requiredConfigMap)
	return mod, err
}

func manageConfigMapCABundleControllerConfig(client coreclientv1.ConfigMapsGetter, eventRecorder events.Recorder) (bool, error) {
	configMap := resourceread.ReadConfigMapV1OrDie(v4_00_assets.MustAsset("v4.0.0/configmap-cabundle-controller/cm.yaml"))
	defaultConfig := v4_00_assets.MustAsset("v4.0.0/configmap-cabundle-controller/defaultconfig.yaml")
	requiredConfigMap, _, err := resourcemerge.MergeConfigMap(configMap, "controller-config.yaml", nil, defaultConfig)
	if err != nil {
		return false, err
	}
	_, mod, err := resourceapply.ApplyConfigMap(client, eventRecorder, requiredConfigMap)
	return mod, err
}

func manageSignerControllerDeployment(client appsclientv1.AppsV1Interface, eventRecorder events.Recorder, options *operatorv1.ServiceCA, forceDeployment bool) (bool, error) {
	return manageDeployment(client, eventRecorder, options, "v4.0.0/service-serving-cert-signer-controller/", forceDeployment)
}

func manageAPIServiceControllerDeployment(client appsclientv1.AppsV1Interface, eventRecorder events.Recorder, options *operatorv1.ServiceCA, forceDeployment bool) (bool, error) {
	return manageDeployment(client, eventRecorder, options, "v4.0.0/apiservice-cabundle-controller/", forceDeployment)
}

func manageConfigMapCABundleControllerDeployment(client appsclientv1.AppsV1Interface, eventRecorder events.Recorder, options *operatorv1.ServiceCA, forceDeployment bool) (bool, error) {
	return manageDeployment(client, eventRecorder, options, "v4.0.0/configmap-cabundle-controller/", forceDeployment)
}

func manageDeployment(client appsclientv1.AppsV1Interface, eventRecorder events.Recorder, options *operatorv1.ServiceCA, resourcePath string, forceDeployment bool) (bool, error) {
	required := resourceread.ReadDeploymentV1OrDie(v4_00_assets.MustAsset(resourcePath + "deployment.yaml"))
	required.Spec.Template.Spec.Containers[0].Image = os.Getenv("CONTROLLER_IMAGE")
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%s", options.Spec.LogLevel))
	deployment, mod, err := resourceapply.ApplyDeployment(client, eventRecorder, required, resourcemerge.ExpectedDeploymentGeneration(required, options.Status.Generations), forceDeployment)
	if err != nil {
		return mod, err
	}
	glog.V(4).Infof("current deployment of %s: %#v", resourcePath, deployment)
	resourcemerge.SetDeploymentGeneration(&options.Status.Generations, deployment)

	return mod, nil
}

func serviceServingCertSignerName() string {
	return fmt.Sprintf("%s@%d", "openshift-service-serving-signer", time.Now().Unix())
}
