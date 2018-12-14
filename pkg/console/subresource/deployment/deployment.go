package deployment

import (
	"github.com/sirupsen/logrus"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/console-operator/pkg/controller"
)

const (
	consolePortName        = "https"
	consolePort            = 443
	consoleTargetPort      = 8443
	publicURLName          = "BRIDGE_DEVELOPER_CONSOLE_URL"
	ConsoleServingCertName = "console-serving-cert"
	ConsoleOauthConfigName = "console-oauth-config"
)

const (
	configMapResourceVersionAnnotation = "console.openshift.io/consoleconfigversion"
	secretResourceVersionAnnotation    = "console.openshift.io/oauthsecretversion"
)

type volumeConfig struct {
	name     string
	readOnly bool
	path     string
	// isSecret or isConfigMap are mutually exclusive
	isSecret    bool
	isConfigMap bool
}

var volumeConfigList = []volumeConfig{
	{
		name:     ConsoleServingCertName,
		readOnly: true,
		path:     "/var/serving-cert",
		isSecret: true,
	},
	{
		name:     ConsoleOauthConfigName,
		readOnly: true,
		path:     "/var/oauth-config",
		isSecret: true,
	},
	{
		name:        "console-config",
		readOnly:    true,
		path:        "/var/console-config",
		isConfigMap: true,
	},
}

func DefaultDeployment(cr *v1alpha1.Console, cm *corev1.ConfigMap, sec *corev1.Secret) *appsv1.Deployment {
	labels := util.LabelsForConsole()
	meta := util.SharedMeta()
	meta.Labels = labels
	replicas := cr.Spec.Count
	gracePeriod := int64(30)

	deployment := &appsv1.Deployment{
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   controller.OpenShiftConsoleShortName,
					Labels: labels,
					Annotations: map[string]string{
						configMapResourceVersionAnnotation: cm.GetResourceVersion(),
						secretResourceVersionAnnotation:    sec.GetResourceVersion(),
					},
				},
				Spec: corev1.PodSpec{
					// we want to deploy on master nodes
					NodeSelector: map[string]string{
						// empty string is correct
						"node-role.kubernetes.io/master": "",
					},
					Affinity: &corev1.Affinity{
						// spread out across master nodes rather than congregate on one
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
								Weight: 100,
								PodAffinityTerm: corev1.PodAffinityTerm{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: util.SharedLabels(),
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							}},
						},
					},
					// toleration is a taint override. we can and should be scheduled on a master node.
					Tolerations: []corev1.Toleration{
						{
							Key:      "node-role.kubernetes.io/master",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					RestartPolicy:                 corev1.RestartPolicyAlways,
					SchedulerName:                 corev1.DefaultSchedulerName,
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
	util.AddOwnerRef(deployment, util.OwnerRefFrom(cr))
	return deployment
}

func Stub() *appsv1.Deployment {
	meta := util.SharedMeta()
	dep := &appsv1.Deployment{
		ObjectMeta: meta,
	}
	return dep
}

// deduplication, use the same volume config to generate Volumes, and VolumeMounts
func consoleVolumes(vc []volumeConfig) []corev1.Volume {
	vols := make([]corev1.Volume, len(vc))
	for i, item := range vc {
		if item.isSecret {
			vols[i] = corev1.Volume{
				Name: item.name,
				VolumeSource: corev1.VolumeSource{
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
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: item.name,
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
		Image:           util.GetImageEnv(),
		ImagePullPolicy: corev1.PullPolicy("IfNotPresent"),
		Name:            controller.OpenShiftConsoleShortName,
		Command: []string{
			"/opt/bridge/bin/bridge",
			"--public-dir=/opt/bridge/static",
			"--config=/var/console-config/console-config.yaml",
		},
		// TODO: can probably remove, this is used for local dev
		//Env: []corev1.EnvVar{{
		//	Name:  publicURLName,
		//	Value: consoleURL(),
		//}},
		Ports: []corev1.ContainerPort{{
			Name:          consolePortName,
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: consolePort,
		}},
		VolumeMounts:             volumeMounts,
		ReadinessProbe:           defaultProbe(),
		LivenessProbe:            livenessProbe(),
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessagePolicy("File"),
		Resources: corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				// TODO: fill these out
				//	"cpu": int64(100),
				//	"memory": int64(100)
			},
			Requests: map[corev1.ResourceName]resource.Quantity{},
		},
	}
}

func defaultProbe() *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health",
				Port:   intstr.FromInt(8443),
				Scheme: corev1.URIScheme("HTTPS"),
			},
		},
		TimeoutSeconds:   1,
		PeriodSeconds:    10,
		SuccessThreshold: 1,
		FailureThreshold: 3,
	}
}

func livenessProbe() *corev1.Probe {
	probe := defaultProbe()
	probe.InitialDelaySeconds = 30
	return probe
}

func ConfigMapResourceVersionChanged(dep *appsv1.Deployment, cm *corev1.ConfigMap) bool {
	currentVersion := dep.Spec.Template.Annotations[configMapResourceVersionAnnotation]
	logrus.Infof("pod template annotation [%v] changed? %v (%v vs %v)", configMapResourceVersionAnnotation, cm.GetResourceVersion() != currentVersion, cm.GetResourceVersion(), currentVersion)
	return cm.GetResourceVersion() != currentVersion
}

func SecretResourceVersionChanged(dep *appsv1.Deployment, sec *corev1.Secret) bool {
	currentVersion := dep.Spec.Template.Annotations[secretResourceVersionAnnotation]
	logrus.Infof("pod template annotation [%v] changed? %v (%v vs %v)", secretResourceVersionAnnotation, sec.GetResourceVersion() != currentVersion, sec.GetResourceVersion(), currentVersion)
	return sec.GetResourceVersion() != currentVersion
}

func ResourceVersionsChanged(dep *appsv1.Deployment, cm *corev1.ConfigMap, sec *corev1.Secret) bool {
	return SecretResourceVersionChanged(dep, sec) || ConfigMapResourceVersionChanged(dep, cm)
}

func UpdateResourceVersions(dep *appsv1.Deployment, cm *corev1.ConfigMap, sec *corev1.Secret) *appsv1.Deployment {
	dep.Spec.Template.Annotations[configMapResourceVersionAnnotation] = cm.GetResourceVersion()
	dep.Spec.Template.Annotations[secretResourceVersionAnnotation] = sec.GetResourceVersion()
	return dep
}
