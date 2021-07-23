package deployment

import (
	"context"
	"fmt"

	// kube
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/klog/v2"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

const (
	ConsoleOauthConfigName    = "console-oauth-config"
	DefaultConsoleReplicas    = 2
	SingleNodeConsoleReplicas = 1
)

const (
	configMapResourceVersionAnnotation                   = "console.openshift.io/console-config-version"
	proxyConfigResourceVersionAnnotation                 = "console.openshift.io/proxy-config-version"
	infrastructureConfigResourceVersionAnnotation        = "console.openshift.io/infrastructure-config-version"
	serviceCAConfigMapResourceVersionAnnotation          = "console.openshift.io/service-ca-config-version"
	defaultIngressCertConfigMapResourceVersionAnnotation = "console.openshift.io/default-ingress-cert-config-version"
	trustedCAConfigMapResourceVersionAnnotation          = "console.openshift.io/trusted-ca-config-version"
	secretResourceVersionAnnotation                      = "console.openshift.io/oauth-secret-version"
	consoleImageAnnotation                               = "console.openshift.io/image"
	workloadManagementAnnotation                         = "target.workload.openshift.io/management"
	workloadManagementAnnotationValue                    = `{"effect": "PreferredDuringScheduling"}`
)

var (
	resourceAnnotations = []string{
		configMapResourceVersionAnnotation,
		proxyConfigResourceVersionAnnotation,
		infrastructureConfigResourceVersionAnnotation,
		serviceCAConfigMapResourceVersionAnnotation,
		defaultIngressCertConfigMapResourceVersionAnnotation,
		trustedCAConfigMapResourceVersionAnnotation,
		secretResourceVersionAnnotation,
		consoleImageAnnotation,
	}
	tolerationSeconds = int64(120)
)

type volumeConfig struct {
	name     string
	readOnly bool
	path     string
	// isSecret or isConfigMap are mutually exclusive
	isSecret    bool
	isConfigMap bool
	mappedKeys  map[string]string
}

func DefaultDeployment(operatorConfig *operatorv1.Console, cm *corev1.ConfigMap, serviceCAConfigMap *corev1.ConfigMap, defaultIngressCertConfigMap *corev1.ConfigMap, trustedCAConfigMap *corev1.ConfigMap, sec *corev1.Secret, proxyConfig *configv1.Proxy, infrastructureConfig *configv1.Infrastructure, canMountCustomLogo bool) *appsv1.Deployment {
	labels := util.LabelsForConsole()
	meta := util.SharedMeta()
	meta.Labels = labels
	deploymentAnnotations := map[string]string{
		configMapResourceVersionAnnotation:                   cm.GetResourceVersion(),
		serviceCAConfigMapResourceVersionAnnotation:          serviceCAConfigMap.GetResourceVersion(),
		defaultIngressCertConfigMapResourceVersionAnnotation: defaultIngressCertConfigMap.GetResourceVersion(),
		trustedCAConfigMapResourceVersionAnnotation:          trustedCAConfigMap.GetResourceVersion(),
		proxyConfigResourceVersionAnnotation:                 proxyConfig.GetResourceVersion(),
		infrastructureConfigResourceVersionAnnotation:        infrastructureConfig.GetResourceVersion(),
		secretResourceVersionAnnotation:                      sec.GetResourceVersion(),
		consoleImageAnnotation:                               util.GetImageEnv("CONSOLE_IMAGE"),
	}

	// Set any annotations as needed so that `ApplyDeployment` rolls out a
	// new version when they change.
	meta.Annotations = deploymentAnnotations
	replicas := Replicas(infrastructureConfig)
	affinity := consolePodAffinity(infrastructureConfig)
	rollingUpdateParams := rollingUpdateParams(infrastructureConfig)
	gracePeriod := int64(40)
	volumeConfig := defaultVolumeConfig()
	caBundle, caBundleExists := trustedCAConfigMap.Data["ca-bundle.crt"]
	if caBundleExists && caBundle != "" {
		volumeConfig = append(volumeConfig, trustedCAVolume())
	}
	if canMountCustomLogo {
		volumeConfig = append(volumeConfig, customLogoVolume())
	}

	podAnnotations := map[string]string{
		workloadManagementAnnotation: workloadManagementAnnotationValue,
	}
	for k, v := range deploymentAnnotations {
		podAnnotations[k] = v
	}
	nodeSelector := map[string]string{
		// by default, we want to deploy on master nodes
		// empty string is correct
		"node-role.kubernetes.io/master": "",
	}
	// If running with an externalized control plane, remove the master node selector
	if infrastructureConfig.Status.ControlPlaneTopology == configv1.ExternalTopologyMode {
		nodeSelector = map[string]string{}
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type:          appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: rollingUpdateParams,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleName,
					Labels:      labels,
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "console",
					NodeSelector:       nodeSelector,
					Affinity:           affinity,
					// toleration is a taint override. we can and should be scheduled on a master node.
					Tolerations:                   tolerations(),
					PriorityClassName:             "system-cluster-critical",
					RestartPolicy:                 corev1.RestartPolicyAlways,
					SchedulerName:                 corev1.DefaultSchedulerName,
					TerminationGracePeriodSeconds: &gracePeriod,
					SecurityContext:               &corev1.PodSecurityContext{},
					Containers: []corev1.Container{
						consoleContainer(operatorConfig, volumeConfig, proxyConfig),
					},
					Volumes: consoleVolumes(volumeConfig),
				},
			},
		},
	}
	util.AddOwnerRef(deployment, util.OwnerRefFrom(operatorConfig))
	return deployment
}

func DefaultDownloadsDeployment(operatorConfig *operatorv1.Console, infrastructureConfig *configv1.Infrastructure) *appsv1.Deployment {
	labels := util.LabelsForDownloads()
	meta := util.SharedMeta()
	meta.Labels = labels
	meta.Name = api.OpenShiftConsoleDownloadsDeploymentName
	replicas := Replicas(infrastructureConfig)
	affinity := downloadsPodAffinity(infrastructureConfig)
	rollingUpdateParams := rollingUpdateParams(infrastructureConfig)
	gracePeriod := int64(0)
	downloadsDeployment := &appsv1.Deployment{
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type:          appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: rollingUpdateParams,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: api.OpenShiftConsoleDownloadsDeploymentName,
					Annotations: map[string]string{
						workloadManagementAnnotation: workloadManagementAnnotationValue,
					},
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux",
					},
					Affinity:                      affinity,
					Tolerations:                   tolerations(),
					PriorityClassName:             "system-cluster-critical",
					TerminationGracePeriodSeconds: &gracePeriod,
					Containers: []corev1.Container{
						{
							Name:                     "download-server",
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
							Image:                    util.GetImageEnv("DOWNLOADS_IMAGE"),
							ImagePullPolicy:          corev1.PullPolicy("IfNotPresent"),
							Ports: []corev1.ContainerPort{{
								Name:          api.DownloadsPortName,
								Protocol:      corev1.ProtocolTCP,
								ContainerPort: api.DownloadsPort,
							}},
							ReadinessProbe: downloadsReadinessProbe(),
							LivenessProbe:  defaultDownloadsProbe(),
							Command:        []string{"/bin/sh"},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
							Args: downloadsContainerArgs(),
						},
					},
				},
			},
		},
	}
	util.AddOwnerRef(downloadsDeployment, util.OwnerRefFrom(operatorConfig))
	return downloadsDeployment
}

func Stub() *appsv1.Deployment {
	meta := util.SharedMeta()
	dep := &appsv1.Deployment{
		ObjectMeta: meta,
	}
	return dep
}

func LogDeploymentAnnotationChanges(client appsclientv1.DeploymentsGetter, updated *appsv1.Deployment, ctx context.Context) {
	existing, err := client.Deployments(updated.Namespace).Get(ctx, updated.Name, metav1.GetOptions{})
	if err != nil {
		klog.V(4).Infof("%v", err)
		return
	}

	changed := false
	for _, annot := range resourceAnnotations {
		if existing.ObjectMeta.Annotations[annot] != updated.ObjectMeta.Annotations[annot] {
			changed = true
			klog.V(4).Infof("deployment annotation[%v] has changed from: %v to %v", annot, existing.ObjectMeta.Annotations[annot], updated.ObjectMeta.Annotations[annot])
		}
	}
	if changed {
		klog.V(4).Infoln("deployment resource versions have changed")
	}
}

func rollingUpdateParams(infrastructureConfig *configv1.Infrastructure) *appsv1.RollingUpdateDeployment {
	if infrastructureConfig.Status.InfrastructureTopology == configv1.SingleReplicaTopologyMode {
		return &appsv1.RollingUpdateDeployment{}
	}
	return &appsv1.RollingUpdateDeployment{
		MaxSurge: &intstr.IntOrString{
			IntVal: int32(3),
		},
		MaxUnavailable: &intstr.IntOrString{
			IntVal: int32(1),
		},
	}
}

func tolerations() []corev1.Toleration {
	return []corev1.Toleration{
		{
			Key:      "node-role.kubernetes.io/master",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			Key:               "node.kubernetes.io/unreachable",
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: &tolerationSeconds,
		},
		{
			Key:               "node.kubernetes.io/not-reachable",
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: &tolerationSeconds,
		},
	}
}

func consolePodAffinity(infrastructureConfig *configv1.Infrastructure) *corev1.Affinity {
	if infrastructureConfig.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode {
		return &corev1.Affinity{}
	}
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "component",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"ui"},
						},
					},
				},
				TopologyKey: "kubernetes.io/hostname",
			}},
		},
	}
}

// Since the downloads deployment runs on any Linux node we should be looking on infrastructureTopology field
func downloadsPodAffinity(infrastructureConfig *configv1.Infrastructure) *corev1.Affinity {
	if infrastructureConfig.Status.InfrastructureTopology == configv1.SingleReplicaTopologyMode {
		return &corev1.Affinity{}
	}
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "component",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"downloads"},
						},
					},
				},
				TopologyKey: "kubernetes.io/hostname",
			}},
		},
	}
}

func Replicas(infrastructureConfig *configv1.Infrastructure) int32 {
	if infrastructureConfig.Status.InfrastructureTopology == configv1.SingleReplicaTopologyMode {
		return int32(SingleNodeConsoleReplicas)
	}
	return int32(DefaultConsoleReplicas)
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
			var items []corev1.KeyToPath
			for key, val := range item.mappedKeys {
				items = append(items, corev1.KeyToPath{
					Key:  key,
					Path: val,
				})
			}
			vols[i] = corev1.Volume{
				Name: item.name,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: item.name,
						},
						Items: items,
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

func GetLogLevelFlag(logLevel operatorv1.LogLevel) string {
	flag := ""
	switch logLevel {
	case operatorv1.Normal:
		flag = "--v=2"
	case operatorv1.Debug:
		flag = "--v=4"
	case operatorv1.Trace:
		flag = "--v=6"
	case operatorv1.TraceAll:
		flag = "--v=10"
	}
	return flag
}

func withLogLevelFlag(logLevel operatorv1.LogLevel, flags []string) []string {
	if logLevelFlag := GetLogLevelFlag(logLevel); logLevelFlag != "" {
		return append(flags, logLevelFlag)
	}
	return flags
}

func withStatusPageFlag(providers operatorv1.ConsoleProviders, flags []string) []string {
	if providers.Statuspage != nil && len(providers.Statuspage.PageID) != 0 {
		return append(flags, fmt.Sprintf("--statuspage-id=%s", providers.Statuspage.PageID))
	}
	return flags
}

func consoleContainer(cr *operatorv1.Console, volConfigList []volumeConfig, proxyConfig *configv1.Proxy) corev1.Container {
	volumeMounts := consoleVolumeMounts(volConfigList)

	flags := []string{
		"/opt/bridge/bin/bridge",
		"--public-dir=/opt/bridge/static",
		"--config=/var/console-config/console-config.yaml",
		"--service-ca-file=/var/service-ca/service-ca.crt",
	}
	flags = withLogLevelFlag(cr.Spec.LogLevel, flags)
	flags = withStatusPageFlag(cr.Spec.Providers, flags)

	return corev1.Container{
		Image:           util.GetImageEnv("CONSOLE_IMAGE"),
		ImagePullPolicy: corev1.PullPolicy("IfNotPresent"),
		Name:            api.OpenShiftConsoleName,
		Command:         flags,
		Env:             setEnvironmentVariables(proxyConfig),
		Ports: []corev1.ContainerPort{{
			Name:          api.ConsoleContainerPortName,
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: api.ConsoleContainerTargetPort,
		}},
		// Delay shutdown for 25 seconds, which is the estimated time for:
		// * endpoint propagation on delete to the router: 5s
		// * router max reload wait: 5s
		// * time for the longest connection to shut down: 15s
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{"sleep", "25"},
				},
			},
		},
		VolumeMounts:             volumeMounts,
		ReadinessProbe:           defaultProbe(),
		LivenessProbe:            livenessProbe(),
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		Resources: corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
		},
	}
}

func setEnvironmentVariables(proxyConfig *configv1.Proxy) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}
	if proxyConfig == nil {
		return envVars
	}
	if len(proxyConfig.Status.HTTPSProxy) != 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "HTTPS_PROXY",
			Value: proxyConfig.Status.HTTPSProxy,
		})
	}
	if len(proxyConfig.Status.HTTPProxy) != 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "HTTP_PROXY",
			Value: proxyConfig.Status.HTTPProxy,
		})
	}
	if len(proxyConfig.Status.NoProxy) != 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "NO_PROXY",
			Value: proxyConfig.Status.NoProxy,
		})
	}
	return envVars
}

func defaultProbe() *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health",
				Port:   intstr.FromInt(api.ConsoleContainerTargetPort),
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
	probe.InitialDelaySeconds = 150
	return probe
}

func defaultDownloadsProbe() *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/",
				Port:   intstr.FromInt(api.DownloadsPort),
				Scheme: corev1.URIScheme("HTTP"),
			},
		},
	}
}

func downloadsReadinessProbe() *corev1.Probe {
	probe := defaultDownloadsProbe()
	probe.FailureThreshold = int32(3)
	return probe
}

// for the purpose of availability, ready is when we have at least
// one ready replica
func IsReady(deployment *appsv1.Deployment) bool {
	avail := deployment.Status.ReadyReplicas >= 1
	if !avail {
		klog.V(4).Infof("deployment is not available, expected replicas: %v, ready replicas: %v", deployment.Spec.Replicas, deployment.Status.ReadyReplicas)
	}
	return avail
}

func IsReadyAndUpdated(deployment *appsv1.Deployment) bool {
	ready := deployment.Status.Replicas == deployment.Status.ReadyReplicas
	updated := deployment.Status.Replicas == deployment.Status.UpdatedReplicas
	if !ready {
		klog.V(4).Infof("deployment is not ready, expected replicas: %v, ready replicas: %v, total replicas: %v", deployment.Spec.Replicas, deployment.Status.ReadyReplicas, deployment.Status.Replicas)
	}
	if !updated {
		klog.V(4).Infof("deployment is not updated, expected replicas: %v, updated replicas: %v, total replicas: %v", deployment.Spec.Replicas, deployment.Status.UpdatedReplicas, deployment.Status.Replicas)
	}
	return ready && updated
}

func IsAvailableAndUpdated(deployment *appsv1.Deployment) bool {
	available := deployment.Status.AvailableReplicas > 0
	currentGen := deployment.Status.ObservedGeneration >= deployment.Generation
	updated := deployment.Status.UpdatedReplicas == deployment.Status.Replicas
	if !available {
		klog.V(4).Infof("deployment is not available, expected replicas: %v, available replicas: %v, total replicas: %v", deployment.Spec.Replicas, deployment.Status.AvailableReplicas, deployment.Status.Replicas)
	}
	if !currentGen {
		klog.V(4).Infof("deployment is not current, observing generation: %v, generation: %v", deployment.Status.ObservedGeneration, deployment.Generation)
	}
	if !updated {
		klog.V(4).Infof("deployment is not updated, updated replicas: %v, available replicas: %v, total replicas: %v", deployment.Spec.Replicas, deployment.Status.UpdatedReplicas, deployment.Status.Replicas)
	}

	return available && currentGen && updated
}

func defaultVolumeConfig() []volumeConfig {
	return []volumeConfig{
		{
			name:     api.ConsoleServingCertName,
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
			name:        api.OpenShiftConsoleConfigMapName,
			readOnly:    true,
			path:        "/var/console-config",
			isConfigMap: true,
		},
		{
			name:        api.ServiceCAConfigMapName,
			readOnly:    true,
			path:        "/var/service-ca",
			isConfigMap: true,
		},
		{
			name:        api.DefaultIngressCertConfigMapName,
			readOnly:    true,
			path:        "/var/default-ingress-cert",
			isConfigMap: true,
		},
	}
}

func trustedCAVolume() volumeConfig {
	return volumeConfig{
		name:        api.TrustedCAConfigMapName,
		readOnly:    true,
		path:        api.TrustedCABundleMountDir,
		isConfigMap: true,
		mappedKeys: map[string]string{
			api.TrustedCABundleKey: api.TrustedCABundleMountFile,
		},
	}
}

func customLogoVolume() volumeConfig {
	return volumeConfig{
		name:        api.OpenShiftCustomLogoConfigMapName,
		path:        "/var/logo/",
		isConfigMap: true}
}

func downloadsContainerArgs() []string {
	return []string{"-c", `cat <<EOF >>/tmp/serve.py
import errno, http.server, os, re, signal, socket, sys, tarfile, tempfile, threading, time, zipfile

signal.signal(signal.SIGTERM, lambda signum, frame: sys.exit(0))

def write_index(path, message):
	with open(path, 'wb') as f:
		f.write('\n'.join([
			'<!doctype html>',
			'<html lang="en">',
			'<head>',
			'  <meta charset="utf-8">',
			'</head>',
			'<body>',
			'  {}'.format(message),
			'</body>',
			'</html>',
			'',
		]).encode('utf-8'))

# Launch multiple listeners as threads
class Thread(threading.Thread):
	def __init__(self, i, socket):
		threading.Thread.__init__(self)
		self.i = i
		self.socket = socket
		self.daemon = True
		self.start()

	def run(self):
		httpd = http.server.HTTPServer(addr, http.server.SimpleHTTPRequestHandler, False)

		# Prevent the HTTP server from re-binding every handler.
		# https://stackoverflow.com/questions/46210672/
		httpd.socket = self.socket
		httpd.server_bind = self.server_close = lambda self: None

		httpd.serve_forever()

temp_dir = tempfile.mkdtemp()
print('serving from {}'.format(temp_dir))
os.chdir(temp_dir)
for arch in ['amd64']:
	os.mkdir(arch)
	for operating_system in ['linux', 'mac', 'windows']:
		os.mkdir(os.path.join(arch, operating_system))
for arch in ['arm64', 'ppc64le', 's390x']:
	os.mkdir(arch)
	for operating_system in ['linux']:
		os.mkdir(os.path.join(arch, operating_system))
content = ['<a href="oc-license">license</a>']
os.symlink('/usr/share/openshift/LICENSE', 'oc-license')

for arch, operating_system, path in [
		('amd64', 'linux', '/usr/share/openshift/linux_amd64/oc'),
		('amd64', 'mac', '/usr/share/openshift/mac/oc'),
		('amd64', 'windows', '/usr/share/openshift/windows/oc.exe'),
		('arm64', 'linux', '/usr/share/openshift/linux_arm64/oc'),
		('ppc64le', 'linux', '/usr/share/openshift/linux_ppc64le/oc'),
		('s390x', 'linux', '/usr/share/openshift/linux_s390x/oc'),
		]:
	basename = os.path.basename(path)
	target_path = os.path.join(arch, operating_system, basename)
	os.symlink(path, target_path)
	base_root, _ = os.path.splitext(basename)
	archive_path_root = os.path.join(arch, operating_system, base_root)
	with tarfile.open('{}.tar'.format(archive_path_root), 'w') as tar:
		tar.add(path, basename)
	with zipfile.ZipFile('{}.zip'.format(archive_path_root), 'w') as zip:
		zip.write(path, basename)
	content.append('<a href="{0}">oc ({1} {2})</a> (<a href="{0}.tar">tar</a> <a href="{0}.zip">zip</a>)'.format(target_path, arch, operating_system))

for root, directories, filenames in os.walk(temp_dir):
	root_link = os.path.relpath(temp_dir, os.path.join(root, 'child')).replace(os.path.sep, '/')
	for directory in directories:
		write_index(
			path=os.path.join(root, directory, 'index.html'),
			message='<p>Directory listings are disabled.  See <a href="{}">here</a> for available content.</p>'.format(root_link),
		)

write_index(
	path=os.path.join(temp_dir, 'index.html'),
	message='\n'.join(
		['<ul>'] +
		['  <li>{}</li>'.format(entry) for entry in content] +
		['</ul>']
	),
)

# Create socket
# IPv6 should handle IPv4 passively so long as it is not bound to a
# specific address or set to IPv6_ONLY
# https://stackoverflow.com/questions/25817848/python-3-does-http-server-support-ipv6
try:
	addr = ('::', 8080)
	sock = socket.socket(socket.AF_INET6, socket.SOCK_STREAM)
except socket.error as err:
	# errno.EAFNOSUPPORT is "socket.error: [Errno 97] Address family not supported by protocol"
	# When IPv6 is disabled, socket will bind using IPv4.
	if err.errno == errno.EAFNOSUPPORT:
		addr = ('', 8080)
		sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
	else:
		raise    
sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
sock.bind(addr)
sock.listen(5)

[Thread(i, socket=sock) for i in range(100)]
time.sleep(9e9)
EOF
exec python3 /tmp/serve.py`}
}
