package deployment

import (
	"testing"

	"github.com/go-test/deep"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/assets"
	"github.com/openshift/console-operator/pkg/console/subresource/configmap"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
)

const (
	workloadManagementAnnotation      = "target.workload.openshift.io/management"
	workloadManagementAnnotationValue = `{"effect": "PreferredDuringScheduling"}`
)

func TestDefaultDeployment(t *testing.T) {
	var (
		defaultReplicaCount    int32 = DefaultConsoleReplicas
		singleNodeReplicaCount int32 = SingleNodeConsoleReplicas
		labels                       = map[string]string{"app": api.OpenShiftConsoleName, "component": "ui"}
		gracePeriod            int64 = 40
		tolerationSeconds      int64 = 120
	)
	type args struct {
		config             *operatorsv1.Console
		cm                 *corev1.ConfigMap
		ca                 *corev1.ConfigMap
		dica               *corev1.ConfigMap
		tca                *corev1.ConfigMap
		sec                *corev1.Secret
		proxy              *configv1.Proxy
		infrastructure     *configv1.Infrastructure
		canMountCustomLogo bool
	}

	consoleOperatorConfig := &operatorsv1.Console{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: operatorsv1.ConsoleSpec{
			OperatorSpec: operatorsv1.OperatorSpec{
				LogLevel: operatorsv1.Debug,
			},
		},
		Status: operatorsv1.ConsoleStatus{},
	}

	consoleDeploymentObjectMeta := metav1.ObjectMeta{
		Name:                       api.OpenShiftConsoleName,
		Namespace:                  api.OpenShiftConsoleNamespace,
		GenerateName:               "",
		SelfLink:                   "",
		UID:                        "",
		ResourceVersion:            "",
		Generation:                 0,
		CreationTimestamp:          metav1.Time{},
		DeletionTimestamp:          nil,
		DeletionGracePeriodSeconds: nil,
		Labels:                     labels,
		Annotations: map[string]string{
			configMapResourceVersionAnnotation:                   "",
			secretResourceVersionAnnotation:                      "",
			defaultIngressCertConfigMapResourceVersionAnnotation: "",
			serviceCAConfigMapResourceVersionAnnotation:          "",
			trustedCAConfigMapResourceVersionAnnotation:          "",
			proxyConfigResourceVersionAnnotation:                 "",
			infrastructureConfigResourceVersionAnnotation:        "",
			consoleImageAnnotation:                               "",
		},
		OwnerReferences: nil,
		Finalizers:      nil,
		ClusterName:     "",
	}

	consoleConfig := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "console-config",
			GenerateName:               "",
			Namespace:                  api.OpenShiftConsoleNamespace,
			SelfLink:                   "",
			UID:                        "",
			ResourceVersion:            "",
			Generation:                 0,
			CreationTimestamp:          metav1.Time{},
			DeletionTimestamp:          nil,
			DeletionGracePeriodSeconds: nil,
			Labels:                     labels,
			Annotations:                nil,
			OwnerReferences:            nil,
			Finalizers:                 nil,
			ClusterName:                "",
		},
		Data:       map[string]string{"console-config.yaml": ""},
		BinaryData: nil,
	}

	consoleDeploymentTolerations := []corev1.Toleration{
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

	consoleDeploymentTemplateAnnotations := map[string]string{
		configMapResourceVersionAnnotation:                   "",
		secretResourceVersionAnnotation:                      "",
		defaultIngressCertConfigMapResourceVersionAnnotation: "",
		serviceCAConfigMapResourceVersionAnnotation:          "",
		trustedCAConfigMapResourceVersionAnnotation:          "",
		proxyConfigResourceVersionAnnotation:                 "",
		infrastructureConfigResourceVersionAnnotation:        "",
		consoleImageAnnotation:                               "",
		workloadManagementAnnotation:                         workloadManagementAnnotationValue,
	}

	consoleDeploymentAffinity := &corev1.Affinity{
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
			}, {
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "component",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"ui"},
						},
					},
				},
				TopologyKey: "topology.kubernetes.io/zone",
			},
			},
		},
	}

	trustedCAConfigMapEmpty := configmap.TrustedCAStub()
	trustedCAConfigMapSet := configmap.TrustedCAStub()
	trustedCAConfigMapSet.Data[api.TrustedCABundleKey] = "testCAValue"

	proxyConfig := &configv1.Proxy{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: configv1.ProxySpec{
			HTTPSProxy: "https://testurl.openshift.com",
		},
		Status: configv1.ProxyStatus{
			HTTPSProxy: "https://testurl.openshift.com",
		},
	}

	infrastructureConfigHighlyAvailable := infrastructureConfigWithTopology(configv1.HighlyAvailableTopologyMode)
	infrastructureConfigSingleReplica := infrastructureConfigWithTopology(configv1.SingleReplicaTopologyMode)

	consoleDeploymentTemplate := resourceread.ReadDeploymentV1OrDie(assets.MustAsset("deployments/console-deployment.yaml"))
	withContainers(consoleDeploymentTemplate, consoleOperatorConfig, proxyConfig)
	withVolumes(consoleDeploymentTemplate, trustedCAConfigMapEmpty, false)
	consoleDeploymentContainer := consoleDeploymentTemplate.Spec.Template.Spec.Containers[0]
	withVolumes(consoleDeploymentTemplate, trustedCAConfigMapSet, false)
	consoleDeploymentContainerTrusted := consoleDeploymentTemplate.Spec.Template.Spec.Containers[0]
	rollingUpdateParamsForSingleReplica := rollingUpdateParams(infrastructureConfigSingleReplica)
	rollingUpdateParamsForHighAvail := rollingUpdateParams(infrastructureConfigHighlyAvailable)

	tests := []struct {
		name string
		args args
		want *appsv1.Deployment
	}{
		{
			name: "Test Default Config Map",
			args: args{
				config: consoleOperatorConfig,
				cm:     consoleConfig,
				ca:     &corev1.ConfigMap{},
				dica: &corev1.ConfigMap{
					Data: map[string]string{"ca-bundle.crt": "test"},
				},
				tca: trustedCAConfigMapEmpty,
				sec: &corev1.Secret{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Data:       nil,
					StringData: nil,
					Type:       "",
				},
				proxy:          proxyConfig,
				infrastructure: infrastructureConfigHighlyAvailable,
			},
			want: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: consoleDeploymentObjectMeta,
				Spec: appsv1.DeploymentSpec{
					Replicas: &defaultReplicaCount,

					Selector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
					Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{
						Name:        api.OpenShiftConsoleName,
						Labels:      labels,
						Annotations: consoleDeploymentTemplateAnnotations,
					},
						Spec: corev1.PodSpec{
							ServiceAccountName: "console",
							// we want to deploy on master nodes
							NodeSelector: map[string]string{
								// empty string is correct
								"node-role.kubernetes.io/master": "",
							},
							Affinity: consoleDeploymentAffinity,
							// toleration is a taint override. we can and should be scheduled on a master node.
							Tolerations:                   consoleDeploymentTolerations,
							PriorityClassName:             "system-cluster-critical",
							RestartPolicy:                 corev1.RestartPolicyAlways,
							SchedulerName:                 corev1.DefaultSchedulerName,
							TerminationGracePeriodSeconds: &gracePeriod,
							SecurityContext:               &corev1.PodSecurityContext{},
							Containers: []corev1.Container{
								consoleDeploymentContainer,
							},
							Volumes: consoleVolumes(defaultVolumeConfig()),
						},
					},
					Strategy: appsv1.DeploymentStrategy{
						Type:          appsv1.RollingUpdateDeploymentStrategyType,
						RollingUpdate: rollingUpdateParamsForHighAvail,
					},
					MinReadySeconds:         0,
					RevisionHistoryLimit:    nil,
					Paused:                  false,
					ProgressDeadlineSeconds: nil,
				},
				Status: appsv1.DeploymentStatus{},
			},
		},
		{
			name: "Test Trusted CA Config Map",
			args: args{
				config: consoleOperatorConfig,
				cm:     consoleConfig,
				ca:     &corev1.ConfigMap{},
				dica: &corev1.ConfigMap{
					Data: map[string]string{"ca-bundle.crt": "test"},
				},
				tca: trustedCAConfigMapSet,
				sec: &corev1.Secret{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Data:       nil,
					StringData: nil,
					Type:       "",
				},
				proxy:          proxyConfig,
				infrastructure: infrastructureConfigHighlyAvailable,
			},
			want: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: consoleDeploymentObjectMeta,
				Spec: appsv1.DeploymentSpec{
					Replicas: &defaultReplicaCount,
					Selector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
					Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{
						Name:        api.OpenShiftConsoleName,
						Labels:      labels,
						Annotations: consoleDeploymentTemplateAnnotations,
					},
						Spec: corev1.PodSpec{
							ServiceAccountName: "console",
							// we want to deploy on master nodes
							NodeSelector: map[string]string{
								// empty string is correct
								"node-role.kubernetes.io/master": "",
							},
							Affinity: consoleDeploymentAffinity,
							// toleration is a taint override. we can and should be scheduled on a master node.
							Tolerations:                   consoleDeploymentTolerations,
							PriorityClassName:             "system-cluster-critical",
							RestartPolicy:                 corev1.RestartPolicyAlways,
							SchedulerName:                 corev1.DefaultSchedulerName,
							TerminationGracePeriodSeconds: &gracePeriod,
							SecurityContext:               &corev1.PodSecurityContext{},
							Containers: []corev1.Container{
								consoleDeploymentContainerTrusted,
							},
							Volumes: consoleVolumes(append(defaultVolumeConfig(), trustedCAVolume())),
						},
					},
					Strategy: appsv1.DeploymentStrategy{
						Type:          appsv1.RollingUpdateDeploymentStrategyType,
						RollingUpdate: rollingUpdateParamsForHighAvail,
					},
					MinReadySeconds:         0,
					RevisionHistoryLimit:    nil,
					Paused:                  false,
					ProgressDeadlineSeconds: nil,
				},
				Status: appsv1.DeploymentStatus{},
			},
		},
		{
			name: "Test Infrastructure Config SingleReplicaTopologyMode",
			args: args{
				config: consoleOperatorConfig,
				cm:     consoleConfig,
				ca:     &corev1.ConfigMap{},
				dica: &corev1.ConfigMap{
					Data: map[string]string{"ca-bundle.crt": "test"},
				},
				tca: trustedCAConfigMapEmpty,
				sec: &corev1.Secret{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Data:       nil,
					StringData: nil,
					Type:       "",
				},
				proxy:          proxyConfig,
				infrastructure: infrastructureConfigSingleReplica,
			},
			want: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: consoleDeploymentObjectMeta,
				Spec: appsv1.DeploymentSpec{
					Replicas: &singleNodeReplicaCount,
					Selector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
					Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{
						Name:        api.OpenShiftConsoleName,
						Labels:      labels,
						Annotations: consoleDeploymentTemplateAnnotations,
					},
						Spec: corev1.PodSpec{
							ServiceAccountName: "console",
							// we want to deploy on master nodes
							NodeSelector: map[string]string{
								// empty string is correct
								"node-role.kubernetes.io/master": "",
							},
							Affinity: &corev1.Affinity{},
							// toleration is a taint override. we can and should be scheduled on a master node.
							Tolerations:                   consoleDeploymentTolerations,
							PriorityClassName:             "system-cluster-critical",
							RestartPolicy:                 corev1.RestartPolicyAlways,
							SchedulerName:                 corev1.DefaultSchedulerName,
							TerminationGracePeriodSeconds: &gracePeriod,
							SecurityContext:               &corev1.PodSecurityContext{},
							Containers: []corev1.Container{
								consoleDeploymentContainer,
							},
							Volumes: consoleVolumes(defaultVolumeConfig()),
						},
					},
					Strategy: appsv1.DeploymentStrategy{
						Type:          appsv1.RollingUpdateDeploymentStrategyType,
						RollingUpdate: rollingUpdateParamsForSingleReplica,
					},
					MinReadySeconds:         0,
					RevisionHistoryLimit:    nil,
					Paused:                  false,
					ProgressDeadlineSeconds: nil,
				},
				Status: appsv1.DeploymentStatus{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(DefaultDeployment(tt.args.config, tt.args.cm, tt.args.dica, tt.args.cm, tt.args.tca, tt.args.sec, tt.args.proxy, tt.args.infrastructure, tt.args.canMountCustomLogo), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestWithAnnotations(t *testing.T) {
	type args struct {
		deployment                  *appsv1.Deployment
		cm                          *corev1.ConfigMap
		serviceCAConfigMap          *corev1.ConfigMap
		defaultIngressCertConfigMap *corev1.ConfigMap
		trustedCAConfigMap          *corev1.ConfigMap
		sec                         *corev1.Secret
		proxyConfig                 *configv1.Proxy
		infrastructureConfig        *configv1.Infrastructure
	}

	consoleConfig := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "console-config",
			ResourceVersion: "10245",
		},
		Data:       map[string]string{"console-config.yaml": ""},
		BinaryData: nil,
	}

	infrastructureConfig := &configv1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "12345",
		},
		Status: configv1.InfrastructureStatus{
			InfrastructureTopology: configv1.SingleReplicaTopologyMode,
		},
	}

	proxyConfig := &configv1.Proxy{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "54321",
		},
	}

	serviceCAConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "34343",
		},
	}
	defaultIngressCertConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "77777",
		},
	}
	trustedCAConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "75577",
		},
	}

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "101010",
		},
	}

	tests := []struct {
		name string
		args args
		want *appsv1.Deployment
	}{
		{
			name: "Test Default Annotations",
			args: args{
				deployment: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{
									workloadManagementAnnotation: workloadManagementAnnotationValue,
								},
							},
						},
					},
				},
				cm:                          consoleConfig,
				serviceCAConfigMap:          serviceCAConfigMap,
				defaultIngressCertConfigMap: defaultIngressCertConfigMap,
				trustedCAConfigMap:          trustedCAConfigMap,
				sec:                         sec,
				proxyConfig:                 proxyConfig,
				infrastructureConfig:        infrastructureConfig,
			},
			want: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						configMapResourceVersionAnnotation:                   consoleConfig.GetResourceVersion(),
						serviceCAConfigMapResourceVersionAnnotation:          serviceCAConfigMap.GetResourceVersion(),
						defaultIngressCertConfigMapResourceVersionAnnotation: defaultIngressCertConfigMap.GetResourceVersion(),
						trustedCAConfigMapResourceVersionAnnotation:          trustedCAConfigMap.GetResourceVersion(),
						proxyConfigResourceVersionAnnotation:                 proxyConfig.GetResourceVersion(),
						infrastructureConfigResourceVersionAnnotation:        infrastructureConfig.GetResourceVersion(),
						secretResourceVersionAnnotation:                      sec.GetResourceVersion(),
						consoleImageAnnotation:                               util.GetImageEnv("CONSOLE_IMAGE"),
					},
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								workloadManagementAnnotation:                         workloadManagementAnnotationValue,
								configMapResourceVersionAnnotation:                   consoleConfig.GetResourceVersion(),
								serviceCAConfigMapResourceVersionAnnotation:          serviceCAConfigMap.GetResourceVersion(),
								defaultIngressCertConfigMapResourceVersionAnnotation: defaultIngressCertConfigMap.GetResourceVersion(),
								trustedCAConfigMapResourceVersionAnnotation:          trustedCAConfigMap.GetResourceVersion(),
								proxyConfigResourceVersionAnnotation:                 proxyConfig.GetResourceVersion(),
								infrastructureConfigResourceVersionAnnotation:        infrastructureConfig.GetResourceVersion(),
								secretResourceVersionAnnotation:                      sec.GetResourceVersion(),
								consoleImageAnnotation:                               util.GetImageEnv("CONSOLE_IMAGE"),
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withAnnotations(tt.args.deployment, tt.args.cm, tt.args.serviceCAConfigMap, tt.args.defaultIngressCertConfigMap, tt.args.trustedCAConfigMap, tt.args.sec, tt.args.proxyConfig, tt.args.infrastructureConfig)
			if diff := deep.Equal(tt.args.deployment, tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestWithReplicas(t *testing.T) {
	var (
		singleNodeReplicaCount int32 = SingleNodeConsoleReplicas
		defaultReplicaCount    int32 = DefaultConsoleReplicas
	)

	type args struct {
		deployment           *appsv1.Deployment
		infrastructureConfig *configv1.Infrastructure
	}

	infrastructureConfigHighlyAvailable := infrastructureConfigWithTopology(configv1.HighlyAvailableTopologyMode)
	infrastructureConfigSingleReplica := infrastructureConfigWithTopology(configv1.SingleReplicaTopologyMode)

	tests := []struct {
		name string
		args args
		want *appsv1.Deployment
	}{
		{
			name: "Test Single Replica",
			args: args{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{},
				},
				infrastructureConfig: infrastructureConfigSingleReplica,
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: &singleNodeReplicaCount,
				},
			},
		},
		{
			name: "Test Highly Available Replica",
			args: args{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{},
				},
				infrastructureConfig: infrastructureConfigHighlyAvailable,
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: &defaultReplicaCount,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withReplicas(tt.args.deployment, tt.args.infrastructureConfig)
			if diff := deep.Equal(tt.args.deployment, tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestWithAffinity(t *testing.T) {
	type args struct {
		deployment           *appsv1.Deployment
		infrastructureConfig *configv1.Infrastructure
	}

	infrastructureConfigHighlyAvailable := infrastructureConfigWithTopology(configv1.HighlyAvailableTopologyMode)
	infrastructureConfigSingleReplica := infrastructureConfigWithTopology(configv1.SingleReplicaTopologyMode)

	singleReplicaSpec := corev1.PodSpec{
		Affinity: consolePodAffinity(infrastructureConfigSingleReplica),
	}
	highlyAvailableSpec := corev1.PodSpec{
		Affinity: consolePodAffinity(infrastructureConfigHighlyAvailable),
	}

	tests := []struct {
		name string
		args args
		want *appsv1.Deployment
	}{
		{
			name: "Test Single Replica Affinity",
			args: args{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{},
				},
				infrastructureConfig: infrastructureConfigSingleReplica,
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: singleReplicaSpec,
					},
				},
			},
		},
		{
			name: "Test Highly Available Affinity",
			args: args{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{},
				},
				infrastructureConfig: infrastructureConfigHighlyAvailable,
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: highlyAvailableSpec,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withAffinity(tt.args.deployment, tt.args.infrastructureConfig)
			if diff := deep.Equal(tt.args.deployment, tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestWithVolumes(t *testing.T) {
	type args struct {
		deployment         *appsv1.Deployment
		trustedCAConfigMap *corev1.ConfigMap
		canMountCustomLogo bool
	}

	trustedCAConfigMap := &corev1.ConfigMap{
		Data: map[string]string{
			"ca-bundle.crt": "foobar",
		},
	}

	consoleDeployment := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "consoleContainer",
						},
					},
				},
			},
		},
	}

	volumeConfig := defaultVolumeConfig()
	trustedVolumeConfig := append(volumeConfig, trustedCAVolume())
	customLogoVolumeConfig := append(volumeConfig, customLogoVolume())
	allVolumeConfig := append(volumeConfig, trustedCAVolume(), customLogoVolume())

	tests := []struct {
		name string
		args args
		want *appsv1.Deployment
	}{
		{
			name: "Test Volumes With Only CA Bundle",
			args: args{
				deployment:         consoleDeployment,
				trustedCAConfigMap: trustedCAConfigMap,
				canMountCustomLogo: false,
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:         "consoleContainer",
									VolumeMounts: consoleVolumeMounts(trustedVolumeConfig),
								},
							},
							Volumes: consoleVolumes(trustedVolumeConfig),
						},
					},
				},
			},
		},
		{
			name: "Test Volumes Without CA Bundle And Custom Logo False",
			args: args{
				deployment:         consoleDeployment,
				trustedCAConfigMap: &corev1.ConfigMap{},
				canMountCustomLogo: false,
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:         "consoleContainer",
									VolumeMounts: consoleVolumeMounts(volumeConfig),
								},
							},
							Volumes: consoleVolumes(volumeConfig),
						},
					},
				},
			},
		},
		{
			name: "Test Volumes With Only Custom Logo True",
			args: args{
				deployment:         consoleDeployment,
				trustedCAConfigMap: &corev1.ConfigMap{},
				canMountCustomLogo: true,
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:         "consoleContainer",
									VolumeMounts: consoleVolumeMounts(customLogoVolumeConfig),
								},
							},
							Volumes: consoleVolumes(customLogoVolumeConfig),
						},
					},
				},
			},
		},
		{
			name: "Test Volumes With CA bundle And Custom Logo True",
			args: args{
				deployment:         consoleDeployment,
				trustedCAConfigMap: trustedCAConfigMap,
				canMountCustomLogo: true,
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:         "consoleContainer",
									VolumeMounts: consoleVolumeMounts(allVolumeConfig),
								},
							},
							Volumes: consoleVolumes(allVolumeConfig),
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withVolumes(tt.args.deployment, tt.args.trustedCAConfigMap, tt.args.canMountCustomLogo)
			if diff := deep.Equal(tt.args.deployment, tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestWithContainers(t *testing.T) {
	type args struct {
		deployment     *appsv1.Deployment
		operatorConfig *operatorsv1.Console
		proxyConfig    *configv1.Proxy
	}

	operatorConfig := &operatorsv1.Console{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: operatorsv1.ConsoleSpec{
			OperatorSpec: operatorsv1.OperatorSpec{
				LogLevel: operatorsv1.Debug,
			},
		},
		Status: operatorsv1.ConsoleStatus{},
	}

	proxyConfig := &configv1.Proxy{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "54321",
		},
	}

	defaultCommands := []string{
		"ls -Al",
	}
	expectedCommands := withLogLevelFlag(operatorConfig.Spec.LogLevel, defaultCommands)
	expectedCommands = withStatusPageFlag(operatorConfig.Spec.Providers, expectedCommands)

	consoleDeployment := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "consoleContainer",
							Command: defaultCommands,
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		args args
		want *appsv1.Deployment
	}{
		{
			name: "Test Default Containers",
			args: args{
				deployment:     consoleDeployment,
				operatorConfig: operatorConfig,
				proxyConfig:    proxyConfig,
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:    "consoleContainer",
									Command: expectedCommands,
									Env:     setEnvironmentVariables(proxyConfig),
									Image:   util.GetImageEnv("CONSOLE_IMAGE"),
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withContainers(tt.args.deployment, tt.args.operatorConfig, tt.args.proxyConfig)
			if diff := deep.Equal(tt.args.deployment, tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestDefaultDownloadsDeployment(t *testing.T) {

	var (
		defaultReplicaCount         int32 = DefaultConsoleReplicas
		singleNodeReplicaCount      int32 = SingleNodeConsoleReplicas
		labels                            = util.LabelsForDownloads()
		gracePeriod                 int64 = 0
		tolerationSeconds           int64 = 120
		downloadsDeploymentTemplate       = resourceread.ReadDeploymentV1OrDie(assets.MustAsset("deployments/downloads-deployment.yaml"))
	)

	type args struct {
		config         *operatorsv1.Console
		infrastructure *configv1.Infrastructure
	}

	consoleOperatorConfig := &operatorsv1.Console{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: operatorsv1.ConsoleSpec{
			OperatorSpec: operatorsv1.OperatorSpec{
				LogLevel: operatorsv1.Debug,
			},
		},
		Status: operatorsv1.ConsoleStatus{},
	}

	downloadsDeploymentObjectMeta := metav1.ObjectMeta{
		Name:                       api.OpenShiftConsoleDownloadsDeploymentName,
		Namespace:                  api.OpenShiftConsoleNamespace,
		GenerateName:               "",
		SelfLink:                   "",
		UID:                        "",
		ResourceVersion:            "",
		Generation:                 0,
		CreationTimestamp:          metav1.Time{},
		DeletionTimestamp:          nil,
		DeletionGracePeriodSeconds: nil,
		Labels:                     labels,
		Annotations:                map[string]string{},
		OwnerReferences:            nil,
		Finalizers:                 nil,
		ClusterName:                "",
	}

	infrastructureConfigHighlyAvailable := infrastructureConfigWithTopology(configv1.HighlyAvailableTopologyMode)
	infrastructureConfigSingleReplica := infrastructureConfigWithTopology(configv1.SingleReplicaTopologyMode)

	downloadsDeploymentPodSpecSingleReplica := corev1.PodSpec{
		NodeSelector: map[string]string{
			"kubernetes.io/os": "linux",
		},
		Affinity: downloadsPodAffinity(infrastructureConfigSingleReplica),
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
		SecurityContext:               &corev1.PodSecurityContext{},
		PriorityClassName:             "system-cluster-critical",
		TerminationGracePeriodSeconds: &gracePeriod,
		Containers: []corev1.Container{
			{
				Name:                     "download-server",
				TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
				Image:                    "",
				ImagePullPolicy:          corev1.PullPolicy("IfNotPresent"),
				Ports: []corev1.ContainerPort{{
					Name:          api.DownloadsPortName,
					Protocol:      corev1.ProtocolTCP,
					ContainerPort: api.DownloadsPort,
				}},
				ReadinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/",
							Port:   intstr.FromInt(api.DownloadsPort),
							Scheme: corev1.URIScheme("HTTP"),
						},
					},
					TimeoutSeconds:   1,
					PeriodSeconds:    10,
					SuccessThreshold: 1,
					FailureThreshold: 3,
				},
				LivenessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/",
							Port:   intstr.FromInt(api.DownloadsPort),
							Scheme: corev1.URIScheme("HTTP"),
						},
					},
					TimeoutSeconds:   1,
					PeriodSeconds:    10,
					SuccessThreshold: 1,
					FailureThreshold: 3,
				},
				Command: []string{"/bin/sh"},
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
				},
				Args: downloadsDeploymentTemplate.Spec.Template.Spec.Containers[0].Args,
			},
		},
	}
	downloadsDeploymentPodSpecHighAvail := downloadsDeploymentPodSpecSingleReplica
	downloadsDeploymentPodSpecHighAvail.Affinity = downloadsPodAffinity(infrastructureConfigHighlyAvailable)

	rollingUpdateParamsForSingleReplica := rollingUpdateParams(infrastructureConfigSingleReplica)
	rollingUpdateParamsForHighAvail := rollingUpdateParams(infrastructureConfigHighlyAvailable)

	tests := []struct {
		name string
		args args
		want *appsv1.Deployment
	}{
		{
			name: "Test Downloads Deployment for Single Node Cluster Infrastructure Config",
			args: args{
				config:         consoleOperatorConfig,
				infrastructure: infrastructureConfigSingleReplica,
			},
			want: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: downloadsDeploymentObjectMeta,
				Spec: appsv1.DeploymentSpec{
					Replicas: &singleNodeReplicaCount,
					Strategy: appsv1.DeploymentStrategy{
						Type:          appsv1.RollingUpdateDeploymentStrategyType,
						RollingUpdate: rollingUpdateParamsForSingleReplica,
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name:   api.OpenShiftConsoleDownloadsDeploymentName,
							Labels: labels,
							Annotations: map[string]string{
								workloadManagementAnnotation: workloadManagementAnnotationValue,
							},
						},
						Spec: downloadsDeploymentPodSpecSingleReplica,
					},
				},
				Status: appsv1.DeploymentStatus{},
			},
		},
		{
			name: "Test Downloads Deployment for Multi Node Cluster Infrastructure Config",
			args: args{
				config:         consoleOperatorConfig,
				infrastructure: infrastructureConfigHighlyAvailable,
			},
			want: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: downloadsDeploymentObjectMeta,
				Spec: appsv1.DeploymentSpec{
					Replicas: &defaultReplicaCount,
					Strategy: appsv1.DeploymentStrategy{
						Type:          appsv1.RollingUpdateDeploymentStrategyType,
						RollingUpdate: rollingUpdateParamsForHighAvail,
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name:   api.OpenShiftConsoleDownloadsDeploymentName,
							Labels: labels,
							Annotations: map[string]string{
								workloadManagementAnnotation: workloadManagementAnnotationValue,
							},
						},
						Spec: downloadsDeploymentPodSpecHighAvail,
					},
				},
				Status: appsv1.DeploymentStatus{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(DefaultDownloadsDeployment(tt.args.config, tt.args.infrastructure), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestWithDownloadsReplicas(t *testing.T) {
	var (
		singleNodeReplicaCount int32 = SingleNodeConsoleReplicas
		defaultReplicaCount    int32 = DefaultConsoleReplicas
	)

	type args struct {
		deployment           *appsv1.Deployment
		infrastructureConfig *configv1.Infrastructure
	}

	infrastructureConfigHighlyAvailable := infrastructureConfigWithTopology(configv1.HighlyAvailableTopologyMode)
	infrastructureConfigSingleReplica := infrastructureConfigWithTopology(configv1.SingleReplicaTopologyMode)

	tests := []struct {
		name string
		args args
		want *appsv1.Deployment
	}{
		{
			name: "Test Downloads Single Replica",
			args: args{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{},
				},
				infrastructureConfig: infrastructureConfigSingleReplica,
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: &singleNodeReplicaCount,
				},
			},
		},
		{
			name: "Test Downloads Highly Available Replica",
			args: args{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{},
				},
				infrastructureConfig: infrastructureConfigHighlyAvailable,
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: &defaultReplicaCount,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withDownloadsReplicas(tt.args.deployment, tt.args.infrastructureConfig)
			if diff := deep.Equal(tt.args.deployment, tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestWithDownloadsAffinity(t *testing.T) {
	type args struct {
		deployment           *appsv1.Deployment
		infrastructureConfig *configv1.Infrastructure
	}

	infrastructureConfigHighlyAvailable := infrastructureConfigWithTopology(configv1.HighlyAvailableTopologyMode)
	infrastructureConfigSingleReplica := infrastructureConfigWithTopology(configv1.SingleReplicaTopologyMode)

	singleReplicaSpec := corev1.PodSpec{
		Affinity: downloadsPodAffinity(infrastructureConfigSingleReplica),
	}
	highlyAvailableSpec := corev1.PodSpec{
		Affinity: downloadsPodAffinity(infrastructureConfigHighlyAvailable),
	}

	tests := []struct {
		name string
		args args
		want *appsv1.Deployment
	}{
		{
			name: "Test Single Replica Downloads Affinity",
			args: args{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{},
				},
				infrastructureConfig: infrastructureConfigSingleReplica,
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: singleReplicaSpec,
					},
				},
			},
		},
		{
			name: "Test Highly Available Downloads Affinity",
			args: args{
				deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{},
				},
				infrastructureConfig: infrastructureConfigHighlyAvailable,
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: highlyAvailableSpec,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withDownloadsAffinity(tt.args.deployment, tt.args.infrastructureConfig)
			if diff := deep.Equal(tt.args.deployment, tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestWithDownloadsContainers(t *testing.T) {
	type args struct {
		deployment *appsv1.Deployment
	}

	downloadsDeployment := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "downloadsContainer",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		args args
		want *appsv1.Deployment
	}{
		{
			name: "Test Default Download Containers",
			args: args{
				deployment: downloadsDeployment,
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "downloadsContainer",
									Image: util.GetImageEnv("DOWNLOADS_IMAGE"),
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withDownloadsContainers(tt.args.deployment)
			if diff := deep.Equal(tt.args.deployment, tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestReplicas(t *testing.T) {
	tests := []struct {
		name        string
		infraConfig *configv1.Infrastructure
		want        int32
	}{
		{
			name: "Test Replica Count For Single Node Cluster Infrastructure Config",
			infraConfig: &configv1.Infrastructure{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Status: configv1.InfrastructureStatus{
					InfrastructureTopology: configv1.SingleReplicaTopologyMode,
				},
			},
			want: 1,
		},
		{
			name: "Test Replica Count For Multi Node Cluster Infrastructure Config",
			infraConfig: &configv1.Infrastructure{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Status: configv1.InfrastructureStatus{
					InfrastructureTopology: configv1.HighlyAvailableTopologyMode,
				},
			},
			want: 2,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(Replicas(tt.infraConfig), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestConsolePodAffinity(t *testing.T) {
	tests := []struct {
		name        string
		infraConfig *configv1.Infrastructure
		want        *corev1.Affinity
	}{
		{
			name: "Test Affinity For Single Node Cluster Infrastructure Config",
			infraConfig: &configv1.Infrastructure{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Status: configv1.InfrastructureStatus{
					ControlPlaneTopology: configv1.SingleReplicaTopologyMode,
				},
			},
			want: &corev1.Affinity{},
		},
		{
			name: "Test Affinity For Single Node Cluster Infrastructure Config",
			infraConfig: &configv1.Infrastructure{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Status: configv1.InfrastructureStatus{
					ControlPlaneTopology: configv1.HighlyAvailableTopologyMode,
				},
			},
			want: &corev1.Affinity{
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
					}, {
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "component",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"ui"},
								},
							},
						},
						TopologyKey: "topology.kubernetes.io/zone",
					},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(consolePodAffinity(tt.infraConfig), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestDownloadsPodAffinity(t *testing.T) {
	tests := []struct {
		name        string
		infraConfig *configv1.Infrastructure
		want        *corev1.Affinity
	}{
		{
			name: "Test Affinity For Single Node Cluster Infrastructure Config",
			infraConfig: &configv1.Infrastructure{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Status: configv1.InfrastructureStatus{
					InfrastructureTopology: configv1.SingleReplicaTopologyMode,
				},
			},
			want: &corev1.Affinity{},
		},
		{
			name: "Test Affinity For Single Node Cluster Infrastructure Config",
			infraConfig: &configv1.Infrastructure{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Status: configv1.InfrastructureStatus{
					InfrastructureTopology: configv1.HighlyAvailableTopologyMode,
				},
			},
			want: &corev1.Affinity{
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
					}, {
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "component",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"downloads"},
								},
							},
						},
						TopologyKey: "topology.kubernetes.io/zone",
					},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(downloadsPodAffinity(tt.infraConfig), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestStub(t *testing.T) {
	tests := []struct {
		name string
		want *appsv1.Deployment
	}{
		{
			name: "Testing Stub function deployment",
			want: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "",
					APIVersion: "",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      api.OpenShiftConsoleName,
					Namespace: api.OpenShiftConsoleNamespace,
					Labels: map[string]string{
						"app": api.OpenShiftConsoleName,
					},
					GenerateName:               "",
					SelfLink:                   "",
					UID:                        "",
					ResourceVersion:            "",
					Generation:                 0,
					CreationTimestamp:          metav1.Time{},
					DeletionTimestamp:          nil,
					DeletionGracePeriodSeconds: nil,
					Annotations:                map[string]string{},
					OwnerReferences:            nil,
					Finalizers:                 nil,
					ClusterName:                "",
				},
				Spec:   appsv1.DeploymentSpec{},
				Status: appsv1.DeploymentStatus{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(Stub(), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func Test_consoleVolumes(t *testing.T) {
	type args struct {
		vc []volumeConfig
	}
	consoleServingCert := corev1.Volume{
		Name: api.ConsoleServingCertName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  api.ConsoleServingCertName,
				Items:       nil,
				DefaultMode: nil,
				Optional:    nil,
			},
		},
	}
	consoleOauthConfig := corev1.Volume{
		Name: ConsoleOauthConfigName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  ConsoleOauthConfigName,
				Items:       nil,
				DefaultMode: nil,
				Optional:    nil,
			},
		},
	}
	consoleConfig := corev1.Volume{
		Name: api.OpenShiftConsoleConfigMapName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: api.OpenShiftConsoleConfigMapName,
				},
				Items:       nil,
				DefaultMode: nil,
				Optional:    nil,
			},
		},
	}
	serviceCA := corev1.Volume{
		Name: api.ServiceCAConfigMapName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: api.ServiceCAConfigMapName,
				},
				Items:       nil,
				DefaultMode: nil,
				Optional:    nil,
			},
		},
	}
	defaultIngressCert := corev1.Volume{
		Name: api.DefaultIngressCertConfigMapName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: api.DefaultIngressCertConfigMapName,
				},
				Items:       nil,
				DefaultMode: nil,
				Optional:    nil,
			},
		},
	}
	tests := []struct {
		name string
		args args
		want []corev1.Volume
	}{
		{
			name: "Test console volumes creation",
			args: args{
				vc: defaultVolumeConfig(),
			},
			want: []corev1.Volume{
				consoleServingCert,
				consoleOauthConfig,
				consoleConfig,
				serviceCA,
				defaultIngressCert,
			},
		},
		{
			name: "Test console volumes creation with TrustedCA",
			args: args{
				vc: append(defaultVolumeConfig(), trustedCAVolume()),
			},
			want: []corev1.Volume{
				consoleServingCert,
				consoleOauthConfig,
				consoleConfig,
				serviceCA,
				defaultIngressCert,
				{
					Name: api.TrustedCAConfigMapName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: api.TrustedCAConfigMapName,
							},
							Items: []corev1.KeyToPath{
								{
									Key:  api.TrustedCABundleKey,
									Path: api.TrustedCABundleMountFile,
									Mode: nil,
								},
							},
							DefaultMode: nil,
							Optional:    nil,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(consoleVolumes(tt.args.vc), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func Test_consoleVolumeMounts(t *testing.T) {
	type args struct {
		vc []volumeConfig
	}
	tests := []struct {
		name string
		args args
		want []corev1.VolumeMount
	}{
		{name: "Test console volumes Mounts",
			args: args{
				vc: defaultVolumeConfig(),
			},
			want: []corev1.VolumeMount{
				{
					Name:      api.ConsoleServingCertName,
					ReadOnly:  true,
					MountPath: "/var/serving-cert",
				},
				{
					Name:      ConsoleOauthConfigName,
					ReadOnly:  true,
					MountPath: "/var/oauth-config",
				},
				{
					Name:      api.OpenShiftConsoleConfigMapName,
					ReadOnly:  true,
					MountPath: "/var/console-config",
				},
				{
					Name:      api.ServiceCAConfigMapName,
					ReadOnly:  true,
					MountPath: "/var/service-ca",
				},
				{
					Name:      api.DefaultIngressCertConfigMapName,
					ReadOnly:  true,
					MountPath: "/var/default-ingress-cert",
				},
			},
		},
		{name: "Test console volumes Mounts with TrustedCA",
			args: args{
				vc: append(defaultVolumeConfig(), trustedCAVolume()),
			},
			want: []corev1.VolumeMount{
				{
					Name:      api.ConsoleServingCertName,
					ReadOnly:  true,
					MountPath: "/var/serving-cert",
				},
				{
					Name:      ConsoleOauthConfigName,
					ReadOnly:  true,
					MountPath: "/var/oauth-config",
				},
				{
					Name:      api.OpenShiftConsoleConfigMapName,
					ReadOnly:  true,
					MountPath: "/var/console-config",
				},
				{
					Name:      api.ServiceCAConfigMapName,
					ReadOnly:  true,
					MountPath: "/var/service-ca",
				},
				{
					Name:      api.DefaultIngressCertConfigMapName,
					ReadOnly:  true,
					MountPath: "/var/default-ingress-cert",
				},
				{
					Name:      api.TrustedCAConfigMapName,
					ReadOnly:  true,
					MountPath: api.TrustedCABundleMountDir,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(consoleVolumeMounts(tt.args.vc), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestIsReady(t *testing.T) {
	type args struct {
		deployment *appsv1.Deployment
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Test IsReady(): Deployment has one ready replica",
			args: args{
				deployment: &appsv1.Deployment{
					Status: appsv1.DeploymentStatus{
						ReadyReplicas: 1,
					},
				},
			},
			want: true,
		}, {
			name: "Test IsReady(): Deployment has multiple ready replicas",
			args: args{
				deployment: &appsv1.Deployment{
					Status: appsv1.DeploymentStatus{
						ReadyReplicas: 5,
					},
				},
			},
			want: true,
		}, {
			name: "Test IsReady(): Deployment has no ready replicas",
			args: args{
				deployment: &appsv1.Deployment{
					Status: appsv1.DeploymentStatus{
						ReadyReplicas: 0,
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsReady(tt.args.deployment); got != tt.want {
				t.Errorf("IsReady() = \n%v\n want \n%v", got, tt.want)
			}
		})
	}

}

func TestIsAvailableAndUpdated(t *testing.T) {
	type args struct {
		deployment *appsv1.Deployment
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Test IsAvailableAndUpdated(): Deployment has one available replica, with matching generation and matching replica count",
			args: args{
				deployment: &appsv1.Deployment{
					Status: appsv1.DeploymentStatus{
						AvailableReplicas:  1,
						ObservedGeneration: 1,
						UpdatedReplicas:    1,
						Replicas:           1,
					},
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
				},
			},
			want: true,
		}, {
			name: "Test IsAvailableAndUpdated(): Deployment has multiple available replicas, with higher observed generation and matching replica count",
			args: args{
				deployment: &appsv1.Deployment{
					Status: appsv1.DeploymentStatus{
						AvailableReplicas:  5,
						ObservedGeneration: 2,
						UpdatedReplicas:    1,
						Replicas:           1,
					},
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
				},
			},
			want: true,
		}, {
			name: "Test IsAvailableAndUpdated(): Deployment has no available replicas",
			args: args{
				deployment: &appsv1.Deployment{
					Status: appsv1.DeploymentStatus{
						AvailableReplicas:  0,
						ObservedGeneration: 1,
						UpdatedReplicas:    1,
						Replicas:           1,
					},
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
				},
			},
			want: false,
		}, {
			name: "Test IsAvailableAndUpdated(): Deployment has one available replica, with none matching generation and matching replica count",
			args: args{
				deployment: &appsv1.Deployment{
					Status: appsv1.DeploymentStatus{
						AvailableReplicas:  1,
						ObservedGeneration: 0,
						UpdatedReplicas:    1,
						Replicas:           1,
					},
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
				},
			},
			want: false,
		}, {
			name: "Test IsAvailableAndUpdated(): Deployment has one available replica, with matching generation and none matching replica count",
			args: args{
				deployment: &appsv1.Deployment{
					Status: appsv1.DeploymentStatus{
						AvailableReplicas:  1,
						ObservedGeneration: 1,
						UpdatedReplicas:    1,
						Replicas:           0,
					},
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAvailableAndUpdated(tt.args.deployment); got != tt.want {
				t.Errorf("IsAvailableAndUpdated() = \n%v\n want \n%v", got, tt.want)
			}
		})
	}

}

func infrastructureConfigWithTopology(topologyMode configv1.TopologyMode) *configv1.Infrastructure {
	return &configv1.Infrastructure{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Status: configv1.InfrastructureStatus{
			InfrastructureTopology: topologyMode,
			ControlPlaneTopology:   topologyMode,
		},
	}
}
