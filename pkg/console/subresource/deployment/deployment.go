package deployment

import (
	"context"
	"fmt"

	// kube
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/klog/v2"

	// openshift
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/bindata"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
)

const (
	ConsoleOauthConfigName    = "console-oauth-config"
	DefaultConsoleReplicas    = 2
	SingleNodeConsoleReplicas = 1
)

const (
	configMapResourceVersionAnnotation             = "console.openshift.io/console-config-version"
	proxyConfigResourceVersionAnnotation           = "console.openshift.io/proxy-config-version"
	infrastructureConfigResourceVersionAnnotation  = "console.openshift.io/infrastructure-config-version"
	serviceCAConfigMapResourceVersionAnnotation    = "console.openshift.io/service-ca-config-version"
	trustedCAConfigMapResourceVersionAnnotation    = "console.openshift.io/trusted-ca-config-version"
	secretResourceVersionAnnotation                = "console.openshift.io/oauth-secret-version"
	consoleImageAnnotation                         = "console.openshift.io/image"
	authnConfigVersionAnnotation                   = "console.openshift.io/authentication-config-version"
	authnCATrustConfigMapResourceVersionAnnotation = "console.openshift.io/authn-ca-trust-config-version"
	sessionSecretRVAnnotation                      = "console.openshift.io/session-secret-version"
)

var (
	resourceAnnotations = []string{
		configMapResourceVersionAnnotation,
		proxyConfigResourceVersionAnnotation,
		infrastructureConfigResourceVersionAnnotation,
		serviceCAConfigMapResourceVersionAnnotation,
		authnCATrustConfigMapResourceVersionAnnotation,
		trustedCAConfigMapResourceVersionAnnotation,
		secretResourceVersionAnnotation,
		consoleImageAnnotation,
	}
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
	operatorConfig *operatorv1.Console,
	consoleConfigMap *corev1.ConfigMap,
	serviceCAConfigMap *corev1.ConfigMap,
	localOAuthServingCertConfigMap *corev1.ConfigMap,
	authServerCAConfigMap *corev1.ConfigMap,
	trustedCAConfigMap *corev1.ConfigMap,
	oAuthClientSecret *corev1.Secret,
	sessionSecret *corev1.Secret,
	proxyConfig *configv1.Proxy,
	infrastructureConfig *configv1.Infrastructure,
	canMountCustomLogo bool,
) *appsv1.Deployment {
	authnCATrustConfigMap := localOAuthServingCertConfigMap
	if authnCATrustConfigMap == nil {
		authnCATrustConfigMap = authServerCAConfigMap
	}

	deployment := resourceread.ReadDeploymentV1OrDie(bindata.MustAsset("assets/deployments/console-deployment.yaml"))
	withReplicas(deployment, infrastructureConfig)
	withAffinity(deployment, infrastructureConfig, "ui")
	withStrategy(deployment, infrastructureConfig)
	withConsoleAnnotations(
		deployment,
		consoleConfigMap,
		serviceCAConfigMap,
		authnCATrustConfigMap,
		trustedCAConfigMap,
		oAuthClientSecret,
		sessionSecret,
		proxyConfig,
		infrastructureConfig,
	)
	withConsoleVolumes(
		deployment,
		localOAuthServingCertConfigMap,
		authServerCAConfigMap,
		trustedCAConfigMap,
		sessionSecret,
		canMountCustomLogo,
	)
	withConsoleContainerImage(deployment, operatorConfig, proxyConfig)
	withConsoleNodeSelector(deployment, infrastructureConfig)
	util.AddOwnerRef(deployment, util.OwnerRefFrom(operatorConfig))
	return deployment
}

func DefaultDownloadsDeployment(
	operatorConfig *operatorv1.Console,
	infrastructureConfig *configv1.Infrastructure,
) *appsv1.Deployment {
	downloadsDeployment := resourceread.ReadDeploymentV1OrDie(
		bindata.MustAsset("assets/deployments/downloads-deployment.yaml"),
	)
	withReplicas(downloadsDeployment, infrastructureConfig)
	withAffinity(downloadsDeployment, infrastructureConfig, "downloads")
	withStrategy(downloadsDeployment, infrastructureConfig)
	withDownloadsContainerImage(downloadsDeployment)
	util.AddOwnerRef(downloadsDeployment, util.OwnerRefFrom(operatorConfig))
	return downloadsDeployment
}

// ShouldDeployHA returns true if the console should be deployed in HA mode.
// If the control plane is externalized, the console should be deployed in HA mode based on the InfrastructureTopology,
// otherwise it should be deployed in HA mode based on the ControlPlaneTopology.
func ShouldDeployHA(infrastructureConfig *configv1.Infrastructure) bool {
	return infrastructureConfig.Status.ControlPlaneTopology == configv1.HighlyAvailableTopologyMode ||
		(infrastructureConfig.Status.ControlPlaneTopology == configv1.ExternalTopologyMode &&
			infrastructureConfig.Status.InfrastructureTopology == configv1.HighlyAvailableTopologyMode)
}

func withReplicas(deployment *appsv1.Deployment, infrastructureConfig *configv1.Infrastructure) {
	replicas := int32(SingleNodeConsoleReplicas)
	if ShouldDeployHA(infrastructureConfig) {
		replicas = int32(DefaultConsoleReplicas)
	}
	deployment.Spec.Replicas = &replicas
}

func withAffinity(
	deployment *appsv1.Deployment,
	infrastructureConfig *configv1.Infrastructure,
	component string,
) {
	affinity := &corev1.Affinity{}
	if ShouldDeployHA(infrastructureConfig) {
		affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "component",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{component},
							},
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				}},
			},
		}
	}
	deployment.Spec.Template.Spec.Affinity = affinity
}

func withStrategy(deployment *appsv1.Deployment, infrastructureConfig *configv1.Infrastructure) {
	rollingUpdateParams := &appsv1.RollingUpdateDeployment{}
	if ShouldDeployHA(infrastructureConfig) {
		rollingUpdateParams = &appsv1.RollingUpdateDeployment{
			MaxSurge: &intstr.IntOrString{
				IntVal: int32(3),
			},
			MaxUnavailable: &intstr.IntOrString{
				IntVal: int32(1),
			},
		}
	}
	deployment.Spec.Strategy.RollingUpdate = rollingUpdateParams
}

// withConsoleAnnotations adds annotations in the console deployment which are used to track
// resources that when updated, trigger a new deployment rollout; this happens when the resource
// version changes.
func withConsoleAnnotations(
	deployment *appsv1.Deployment,
	consoleConfigMap *corev1.ConfigMap,
	serviceCAConfigMap *corev1.ConfigMap,
	authServerCAConfigMap *corev1.ConfigMap,
	trustedCAConfigMap *corev1.ConfigMap,
	oAuthClientSecret *corev1.Secret,
	sessionSecret *corev1.Secret,
	proxyConfig *configv1.Proxy,
	infrastructureConfig *configv1.Infrastructure,
) {
	deployment.ObjectMeta.Annotations = map[string]string{
		configMapResourceVersionAnnotation:            consoleConfigMap.GetResourceVersion(),
		serviceCAConfigMapResourceVersionAnnotation:   serviceCAConfigMap.GetResourceVersion(),
		trustedCAConfigMapResourceVersionAnnotation:   trustedCAConfigMap.GetResourceVersion(),
		proxyConfigResourceVersionAnnotation:          proxyConfig.GetResourceVersion(),
		infrastructureConfigResourceVersionAnnotation: infrastructureConfig.GetResourceVersion(),
		secretResourceVersionAnnotation:               oAuthClientSecret.GetResourceVersion(),
		consoleImageAnnotation:                        util.GetImageEnv("CONSOLE_IMAGE"),
	}

	if authServerCAConfigMap != nil {
		deployment.ObjectMeta.Annotations[authnCATrustConfigMapResourceVersionAnnotation] = authServerCAConfigMap.GetResourceVersion()
	}

	if sessionSecret != nil {
		deployment.ObjectMeta.Annotations[sessionSecretRVAnnotation] = sessionSecret.GetResourceVersion()
	}

	podAnnotations := deployment.Spec.Template.ObjectMeta.Annotations
	for k, v := range deployment.ObjectMeta.Annotations {
		podAnnotations[k] = v
	}
	deployment.Spec.Template.ObjectMeta.Annotations = podAnnotations
}

func withConsoleVolumes(
	deployment *appsv1.Deployment,
	oauthServingCert *corev1.ConfigMap,
	authServerCAConfigMap *corev1.ConfigMap,
	trustedCAConfigMap *corev1.ConfigMap,
	sessionSecret *corev1.Secret,
	canMountCustomLogo bool) {
	volumeConfig := defaultVolumeConfig()

	caBundle, caBundleExists := trustedCAConfigMap.Data["ca-bundle.crt"]
	if caBundleExists && caBundle != "" {
		volumeConfig = append(volumeConfig, trustedCAVolume())
	}
	if canMountCustomLogo {
		volumeConfig = append(volumeConfig, customLogoVolume())
	}

	if oauthServingCert != nil {
		volumeConfig = append(volumeConfig, oauthServingCertVolumeConfig())
	}

	if authServerCAConfigMap != nil {
		volumeConfig = append(volumeConfig, authServerCAVolumeConfig(authServerCAConfigMap.Name))
	}

	if sessionSecret != nil {
		volumeConfig = append(volumeConfig, sessionSecretVolumeConfig())
	}

	volMountList := make([]corev1.VolumeMount, len(volumeConfig))
	for i, item := range volumeConfig {
		volMountList[i] = corev1.VolumeMount{
			Name:      item.name,
			ReadOnly:  item.readOnly,
			MountPath: item.path,
		}
	}
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = volMountList

	vols := make([]corev1.Volume, len(volumeConfig))
	for i, item := range volumeConfig {
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
	deployment.Spec.Template.Spec.Volumes = vols
}

func withConsoleContainerImage(
	deployment *appsv1.Deployment,
	operatorConfig *operatorv1.Console,
	proxyConfig *configv1.Proxy,
) {
	commands := deployment.Spec.Template.Spec.Containers[0].Command
	commands = withLogLevelFlag(operatorConfig.Spec.LogLevel, commands)
	commands = withStatusPageFlag(operatorConfig.Spec.Providers, commands)
	deployment.Spec.Template.Spec.Containers[0].Command = commands
	deployment.Spec.Template.Spec.Containers[0].Env = setEnvironmentVariables(proxyConfig)
	// console distinguishes cookie sessions by pod names in OIDC envs
	deployment.Spec.Template.Spec.Containers[0].Env = append(
		deployment.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.name",
				},
			},
		},
	)
	deployment.Spec.Template.Spec.Containers[0].Image = util.GetImageEnv("CONSOLE_IMAGE")
}

func withConsoleNodeSelector(
	deployment *appsv1.Deployment,
	infrastructureConfig *configv1.Infrastructure,
) {
	nodeSelector := deployment.Spec.Template.Spec.NodeSelector

	// If running with an externalized control plane, remove the master node selector
	if infrastructureConfig.Status.ControlPlaneTopology == configv1.ExternalTopologyMode {
		nodeSelector = map[string]string{}
	}

	deployment.Spec.Template.Spec.NodeSelector = nodeSelector
}

func withDownloadsContainerImage(downloadsDeployment *appsv1.Deployment) {
	downloadsDeployment.Spec.Template.Spec.Containers[0].Image = util.GetImageEnv("DOWNLOADS_IMAGE")
}

func Stub() *appsv1.Deployment {
	meta := util.SharedMeta()
	dep := &appsv1.Deployment{
		ObjectMeta: meta,
	}
	return dep
}

func LogDeploymentAnnotationChanges(
	client appsclientv1.DeploymentsGetter,
	updated *appsv1.Deployment,
	ctx context.Context,
) {
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

func IsAvailable(deployment *appsv1.Deployment) bool {
	avail := deployment.Status.AvailableReplicas > 0
	if !avail {
		klog.V(4).Infof("deployment is not available, expected replicas: %v, available replicas: %v, total replicas: %v", deployment.Spec.Replicas, deployment.Status.AvailableReplicas, deployment.Status.Replicas)
	}
	return avail
}

func IsAvailableAndUpdated(deployment *appsv1.Deployment) bool {
	available := IsAvailable(deployment)
	currentGen := deployment.Status.ObservedGeneration >= deployment.Generation
	updated := deployment.Status.UpdatedReplicas == deployment.Status.Replicas
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

func oauthServingCertVolumeConfig() volumeConfig {
	return volumeConfig{
		name:        api.OAuthServingCertConfigMapName,
		readOnly:    true,
		path:        "/var/oauth-serving-cert",
		isConfigMap: true,
	}
}

func authServerCAVolumeConfig(cmName string) volumeConfig {
	return volumeConfig{
		name:        cmName,
		path:        api.AuthServerCAMountDir,
		readOnly:    true,
		isConfigMap: true,
	}
}

func sessionSecretVolumeConfig() volumeConfig {
	return volumeConfig{
		name:     api.SessionSecretName,
		path:     "/var/session-secret",
		readOnly: true,
		isSecret: true,
	}
}
