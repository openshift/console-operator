package operator

import (
	"fmt"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
)

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
					Volumes: consoleVolumes(volumeConfigList),
				},
			},
		},
	}
	addOwnerRef(deployment, ownerRefFrom(cr))
	logrus.Info("Creating console deployment manifest")
	return deployment
}

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

func CreateConsoleDeployment(cr *v1alpha1.Console) {
	d := newConsoleDeployment(cr)
	if err := sdk.Create(d); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("failed to create console deployment : %v", err)
	} else {
		logrus.Info("created console deployment")
		// logYaml(d)
	}
}
