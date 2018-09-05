package console

import (
	"fmt"
	// "encoding/json" think ill stick with yaml
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/runtime/schema"
	// "github.com/ose/Godeps/_workspace/src/k8s.io/kubernetes/pkg/api/v1"
)

const (
	consolePortName = "HTTP"
	consolePort     = 8443
	publicURLName   = "BRIDGE_DEVELOPER_CONSOLE_URL"
)

// This should return the public url provided for us by the ROUTE or Ingress...
func consoleURL() string {
	// This will need to do some work to generate the real path
	logrus.Infof("Console container BRIDGE_DEVELOPER_CONSOLE_URL is HARD-CODED value for now")
	return fmt.Sprintf("https://%s/console/", "api.ui-preserve.origin-gce.dev.openshift.com")
}

func sharedLabels() map[string]string {
	return map[string]string{
		"app": "openshift-console",
	}
}

// similar to how I did this with the helm chart
func labelsForConsole() map[string]string {
	baseLabels := sharedLabels()

	extraLabels := map[string]string{
		"component": "ui",
	}
	// we want to deduplicate, so doing these two loops.
	allLabels := map[string]string{}

	for key, value := range baseLabels {
		allLabels[key] = value
	}
	for key, value := range extraLabels {
		allLabels[key] = value
	}
	return allLabels
}

func sharedMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      "openshift-console",               // ATM no configuration, stable name
		Namespace: "openshift-console-operator-test", // ATM no configuration, "openshift-"
		// these can be overridden/mutated
		Labels: sharedLabels(),
	}
}

// might do something with this, if it is meaningful.
// perhaps reconcile() will need to fiddle? maybe not,
// cuz pods should be handled by their deployments...
//func newConsolePod() *corev1.Pod {
//	labels := sharedLabels() // Didn't add them all here...
//	meta := sharedMeta()
//	meta.Labels = labels
//
//	pod := &corev1.Pod{
//		TypeMeta: metav1.TypeMeta{
//			APIVersion: "apps/v1",
//			Kind:       "Pod",
//		},
//		ObjectMeta: meta,
//		Spec: corev1.PodSpec{
//			Containers: []corev1.Container{
//				{
//					//Name:    "busybox",
//					//Image:   "busybox",
//					//Command: []string{"sleep", "3600"},
//				},
//			},
//		}
//	}
//	return pod
//}

// deduplication, use the same volume config to generate
// Volumes, and VolumeMounts
func consoleVolumes(vc []volumeConfig) []corev1.Volume {
	vols := make([]corev1.Volume, len(vc))
	for i, item := range vc {
		if item.isSecret {
			vols[i] = corev1.Volume{
				Name: item.name,
				VolumeSource: corev1.VolumeSource{
					// NOTE: error if this is not a pointer.
					Secret: &corev1.SecretVolumeSource{
						SecretName: item.name,
					},
				},
			}
		}
		if item.isConfigMap {
			vols[i] = corev1.Volume{
				Name: item.name,
				VolumeSource: corev1.VolumeSource{
					// NOTE: error if this is not a pointer.
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: item.name,
							// cant set this here. will check if
							// its a generated value
							// DefaultMode: 288,
						},
					},
				},
			}
		}
	}
	return vols
}

func consoleVolumeMounts(vc []volumeConfig) []corev1.VolumeMount {
	volMountList := make([]corev1.VolumeMount, len(vc))
	for i, item := range vc {
		volMountList[i] = corev1.VolumeMount{
			Name:      item.name,
			ReadOnly:  item.readOnly,
			MountPath: item.path,
		}
	}
	return volMountList
}

func consoleContainer(cr *v1alpha1.Console) corev1.Container {
	volumeMounts := consoleVolumeMounts(volumeConfigList)
	return corev1.Container{
		Image: fmt.Sprintf("%s:%s", cr.Spec.BaseImage, cr.Spec.Version),
		Name:  "openshift-console",
		Command: []string{
			"/opt/bridge/bin/bridge",
			"--public-dir=/opt/bridge/static",
			"--config=/var/console-config/console-config.yaml",
		},
		// Resources: limits{cpu:100m, memory:100Mi},requests{cpu:100m, memory:100Mi}
		// ReadinessProbe
		// LivenessProbe
		// terminationMessagePath
		Env: []corev1.EnvVar{{
			Name:  publicURLName,
			Value: consoleURL(),
		}},
		Ports: []corev1.ContainerPort{{
			Name:          consolePortName,
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: consolePort,
		}},
		// terminationMessagePolicy
		VolumeMounts: volumeMounts,
	}
}

func newConsoleDeployment(cr *v1alpha1.Console) *appsv1.Deployment {
	labels := labelsForConsole()
	meta := sharedMeta()
	replicas := cr.Spec.Count
	// tack on the deployment specific labels
	meta.Labels = labels
	gracePeriod := int64(30)

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas, // requires pointer
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: "Always",
					SchedulerName: "default-scheduler",
					//the values here may be openshift specific.
					//Affinity: corev1.Affinity{
					//
					//},
					TerminationGracePeriodSeconds: &gracePeriod,
					SecurityContext:               &corev1.PodSecurityContext{},
					Containers: []corev1.Container{
						consoleContainer(cr),
					},
					// TODO: loop this & deduplicate the work done in
					// consoleContainer().  Its silly to maintain these
					// separately.
					Volumes: consoleVolumes(volumeConfigList),
				},
			},
		},
	}

	return deployment
}

// Deploy Vault Example:
// https://github.com/operator-framework/operator-sdk-samples/blob/master/vault-operator/pkg/vault/deploy_vault.go#L39
func deployConsole(cr *v1alpha1.Console) error {
	// What to do to actually deploy a console?
	d := newConsoleDeployment(cr)
	// deployJSON, err := json.Marshal(d)
	deployYAML, err := yaml.Marshal(d)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Deploying:", string(deployYAML))
	// https://github.com/operator-framework/operator-sdk-samples/blob/master/vault-operator/pkg/vault/deploy_vault.go#L98
	err = sdk.Create(d) // reuse err, so no :=
	if err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console deployment : %v", err)
		return err
	}
	return nil
}
