package deployment

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"reflect"
	"sort"
	"strings"

	// kube
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/klog"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

const (
	consolePortName   = "https"
	consolePort       = 443
	consoleTargetPort = 8443
	publicURLName     = "BRIDGE_DEVELOPER_CONSOLE_URL"
	ConsoleReplicas   = 2
)

const (
	consoleImageAnnotation   = "console.openshift.io/image"
	deploymentVersionHashKey = "console.openshift.io/rsv"
)

var (
	resourceAnnotations = []string{
		deploymentVersionHashKey,
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

func DefaultDeployment(
	// top level configs
	operatorConfig *operatorv1.Console,
	proxyConfig *configv1.Proxy,
	// configmaps
	consoleServerConfig *corev1.ConfigMap,
	serviceCAConfigMap *corev1.ConfigMap,
	trustedCAConfigMap *corev1.ConfigMap,
	// secrets
	oauthClientSecret *corev1.Secret,
	servingCert *corev1.Secret,
	// other
	consoleRoute *routev1.Route,
	canMountCustomLogo bool) *appsv1.Deployment {

	labels := util.LabelsForConsole()
	meta := util.SharedMeta()
	meta.Labels = labels

	annotations := map[string]string{
		// if the image changes, we should redeploy
		consoleImageAnnotation: util.GetImageEnv(),
		// if any of the relevant resources change, we should redeploy
		deploymentVersionHashKey: redeployRSVHash(rsvTokens(consoleServerConfig, serviceCAConfigMap, trustedCAConfigMap, oauthClientSecret, servingCert, proxyConfig)),
	}
	// Set any annotations as needed so that `ApplyDeployment` rolls out a
	// new version when they change. `ApplyDeployment` doesn't compare that
	// pod template, but it does check deployment annotations.
	meta.Annotations = annotations
	replicas := int32(ConsoleReplicas)
	gracePeriod := int64(30)
	tolerationSeconds := int64(120)
	volumeConfig := defaultVolumeConfig()
	caBundle, caBundleExists := trustedCAConfigMap.Data["ca-bundle.crt"]
	if caBundleExists && caBundle != "" {
		volumeConfig = append(volumeConfig, trustedCAVolume())
	}
	if canMountCustomLogo {
		volumeConfig = append(volumeConfig, customLogoVolume())
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        api.OpenShiftConsoleName,
					Labels:      labels,
					Annotations: annotations,
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
					},
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

func Stub() *appsv1.Deployment {
	meta := util.SharedMeta()
	dep := &appsv1.Deployment{
		ObjectMeta: meta,
	}
	return dep
}

func LogDeploymentAnnotationChanges(client appsclientv1.DeploymentsGetter, updated *appsv1.Deployment) {
	existing, err := client.Deployments(updated.Namespace).Get(updated.Name, metav1.GetOptions{})
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
	// Since the console-operator logging has different logging levels then the capnslog,
	// that we use for console server(bridge) we need to map them to each other
	switch logLevel {
	case operatorv1.Normal:
		flag = "--log-level=*=NOTICE"
	case operatorv1.Debug:
		flag = "--log-level=*=DEBUG"
	case operatorv1.Trace, operatorv1.TraceAll:
		flag = "--log-level=*=TRACE"
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
		Image:           util.GetImageEnv(),
		ImagePullPolicy: corev1.PullPolicy("IfNotPresent"),
		Name:            api.OpenShiftConsoleName,
		Command:         flags,
		// TODO: can probably remove, this is used for local dev
		//Env: []corev1.EnvVar{{
		//	Name:  publicURLName,
		//	Value: consoleURL(),
		//}},
		Env: setEnvironmentVariables(proxyConfig),
		Ports: []corev1.ContainerPort{{
			Name:          consolePortName,
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: consolePort,
		}},
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
	probe.InitialDelaySeconds = 150
	return probe
}

// for the purpose of availability, ready is when we have at least
// one ready replica
func IsReady(deployment *appsv1.Deployment) bool {
	avail := deployment.Status.ReadyReplicas >= 1
	if avail {
		klog.V(4).Infof("deployment is available, ready replicas: %v", deployment.Status.ReadyReplicas)
	} else {
		klog.V(4).Infof("deployment is not available, ready replicas: %v", deployment.Status.ReadyReplicas)
	}
	return avail
}

func IsReadyAndUpdated(deployment *appsv1.Deployment) bool {
	return deployment.Status.Replicas == deployment.Status.ReadyReplicas &&
		deployment.Status.Replicas == deployment.Status.UpdatedReplicas
}

func IsAvailableAndUpdated(deployment *appsv1.Deployment) bool {
	return deployment.Status.AvailableReplicas > 0 &&
		deployment.Status.ObservedGeneration >= deployment.Generation &&
		deployment.Status.UpdatedReplicas == deployment.Status.Replicas
}

func defaultVolumeConfig() []volumeConfig {
	return []volumeConfig{
		{
			name:     api.ServingCertSecretName,
			readOnly: true,
			path:     "/var/serving-cert",
			isSecret: true,
		},
		{
			name:     api.OAuthConfigName,
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

// redeployRSVHash is a hash of the resource versions of important
// operatorConfig resources. If any of these resources changes, the hash should
// change, causing a rollout of the deployment.
// The operator operatorConfig is omitted as it would cause redeploy loops (as the
// status is updated frequently), but user driven operator operatorConfig changes
// will already cause redeploy.
func redeployRSVHash(rsvList []string) string {
	// sort so we get a stable array
	sort.Strings(rsvList)
	rsvJoined := strings.Join(rsvList, ",")
	klog.V(4).Infof("tracked resource versions: %s", rsvJoined)
	// hash to prevent it from growing indefinitely
	rsvHash := sha512.Sum512([]byte(rsvJoined))
	rsvHashStr := base64.RawURLEncoding.EncodeToString(rsvHash[:])
	return rsvHashStr
}

func rsvToken(obj metav1.Object) string {
	// kind is only relevant if kube is using multiple etcd as backing, else GetResourceVersion is guaranteed to be unique across all objects.
	return reflect.TypeOf(obj).String() + ":" + obj.GetNamespace() + ":" + obj.GetName() + ":" + obj.GetResourceVersion()
}

// creates an array of tokens from a list of objects
func rsvTokens(objs ...metav1.Object) []string {
	list := []string{}
	for _, obj := range objs {
		list = append(list, rsvToken(obj))
	}
	return list
}
