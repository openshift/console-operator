package operator

import (
	"bytes"
	"fmt"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"

	scsv1 "github.com/openshift/service-ca-operator/pkg/apis/serviceca/v1"
	"github.com/openshift/service-ca-operator/pkg/operator/operatorclient"
	"github.com/openshift/service-ca-operator/pkg/operator/v4_00_assets"
)

// syncSigningController_v4_00_to_latest takes care of synchronizing (not upgrading) the thing we're managing.
// most of the time the sync method will be good for a large span of minor versions
func syncSigningController_v4_00_to_latest(c serviceCAOperator, operatorConfig *scsv1.ServiceCA) error {
	var err error

	requiredNamespace := resourceread.ReadNamespaceV1OrDie(v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/ns.yaml"))
	_, _, err = resourceapply.ApplyNamespace(c.corev1Client, c.eventRecorder, requiredNamespace)
	if err != nil {
		return fmt.Errorf("%q: %v", "ns", err)
	}

	requiredClusterRole := resourceread.ReadClusterRoleV1OrDie(v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/clusterrole.yaml"))
	_, _, err = resourceapply.ApplyClusterRole(c.rbacv1Client, c.eventRecorder, requiredClusterRole)
	if err != nil {
		return fmt.Errorf("%q: %v", "clusterrole", err)
	}

	requiredClusterRoleBinding := resourceread.ReadClusterRoleBindingV1OrDie(v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/clusterrolebinding.yaml"))
	_, _, err = resourceapply.ApplyClusterRoleBinding(c.rbacv1Client, c.eventRecorder, requiredClusterRoleBinding)
	if err != nil {
		return fmt.Errorf("%q: %v", "clusterrolebinding", err)
	}

	requiredRole := resourceread.ReadRoleV1OrDie(v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/role.yaml"))
	_, _, err = resourceapply.ApplyRole(c.rbacv1Client, c.eventRecorder, requiredRole)
	if err != nil {
		return fmt.Errorf("%q: %v", "role", err)
	}

	requiredRoleBinding := resourceread.ReadRoleBindingV1OrDie(v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/rolebinding.yaml"))
	_, _, err = resourceapply.ApplyRoleBinding(c.rbacv1Client, c.eventRecorder, requiredRoleBinding)
	if err != nil {
		return fmt.Errorf("%q: %v", "rolebinding", err)
	}

	requiredSA := resourceread.ReadServiceAccountV1OrDie(v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/sa.yaml"))
	_, saModified, err := resourceapply.ApplyServiceAccount(c.corev1Client, c.eventRecorder, requiredSA)
	if err != nil {
		return fmt.Errorf("%q: %v", "sa", err)
	}

	// TODO create a new configmap whenever the data value changes
	_, configMapModified, err := manageSigningConfigMap_v4_00_to_latest(c.corev1Client, c.eventRecorder, operatorConfig)
	if err != nil {
		return fmt.Errorf("%q: %v", "configmap", err)
	}

	_, signingSecretModified, err := manageSigningSecret_v4_00_to_latest(c.corev1Client, c.eventRecorder)
	if err != nil {
		return fmt.Errorf("%q: %v", "signing-key", err)
	}

	var forceDeployment bool
	if saModified { // SA modification can cause new tokens
		forceDeployment = true
	}
	if signingSecretModified {
		forceDeployment = true
	}
	if configMapModified {
		forceDeployment = true
	}

	// our configmaps and secrets are in order, now it is time to create the DS
	// TODO check basic preconditions here
	_, _, err = manageSignerDeployment_v4_00_to_latest(c.appsv1Client, c.eventRecorder, operatorConfig, forceDeployment)
	return err
}

func manageSigningConfigMap_v4_00_to_latest(client coreclientv1.ConfigMapsGetter, eventRecorder events.Recorder, operatorConfig *scsv1.ServiceCA) (*corev1.ConfigMap, bool, error) {
	configMap := resourceread.ReadConfigMapV1OrDie(v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/cm.yaml"))
	defaultConfig := v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/defaultconfig.yaml")
	requiredConfigMap, _, err := resourcemerge.MergeConfigMap(configMap, "controller-config.yaml", nil, defaultConfig, operatorConfig.Spec.ServiceServingCertSignerConfig.Raw)
	if err != nil {
		return nil, false, err
	}
	return resourceapply.ApplyConfigMap(client, eventRecorder, requiredConfigMap)
}

// TODO manage rotation in addition to initial creation
func manageSigningSecret_v4_00_to_latest(client coreclientv1.SecretsGetter, eventRecorder events.Recorder) (*corev1.Secret, bool, error) {
	secret := resourceread.ReadSecretV1OrDie(v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/signing-secret.yaml"))
	existing, err := client.Secrets(secret.Namespace).Get(secret.Name, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		return existing, false, err
	}

	ca, err := crypto.MakeSelfSignedCAConfig(serviceServingCertSignerName(), 365)
	if err != nil {
		return existing, false, err
	}

	certBytes := &bytes.Buffer{}
	keyBytes := &bytes.Buffer{}
	if err := ca.WriteCertConfig(certBytes, keyBytes); err != nil {
		return existing, false, err
	}

	secret.Data["tls.crt"] = certBytes.Bytes()
	secret.Data["tls.key"] = keyBytes.Bytes()

	return resourceapply.ApplySecret(client, eventRecorder, secret)
}

func manageSignerDeployment_v4_00_to_latest(client appsclientv1.AppsV1Interface, eventRecorder events.Recorder, options *scsv1.ServiceCA, forceDeployment bool) (*appsv1.Deployment, bool, error) {
	required := resourceread.ReadDeploymentV1OrDie(v4_00_assets.MustAsset("v4.0.0/service-serving-cert-signer-controller/deployment.yaml"))
	required.Spec.Template.Spec.Containers[0].Image = os.Getenv("CONTROLLER_IMAGE")
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%s", options.Spec.LogLevel))

	return resourceapply.ApplyDeployment(client, eventRecorder, required, getGeneration(client, operatorclient.TargetNamespace, required.Name), forceDeployment)
}

func serviceServingCertSignerName() string {
	return fmt.Sprintf("%s@%d", "openshift-service-serving-signer", time.Now().Unix())
}
