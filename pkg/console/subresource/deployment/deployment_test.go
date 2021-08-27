package deployment

import (
	"testing"

	"github.com/go-test/deep"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/configmap"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

func TestDefaultDeployment(t *testing.T) {
	var (
		defaultReplicaCount    int32 = DefaultConsoleReplicas
		singleNodeReplicaCount int32 = SingleNodeConsoleReplicas
		labels                       = map[string]string{"app": api.OpenShiftConsoleName, "component": "ui"}
		gracePeriod            int64 = 40
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
			configMapResourceVersionAnnotation:                 "",
			secretResourceVersionAnnotation:                    "",
			oauthServingCertConfigMapResourceVersionAnnotation: "",
			serviceCAConfigMapResourceVersionAnnotation:        "",
			trustedCAConfigMapResourceVersionAnnotation:        "",
			proxyConfigResourceVersionAnnotation:               "",
			infrastructureConfigResourceVersionAnnotation:      "",
			consoleImageAnnotation:                             "",
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
		configMapResourceVersionAnnotation:                 "",
		secretResourceVersionAnnotation:                    "",
		oauthServingCertConfigMapResourceVersionAnnotation: "",
		serviceCAConfigMapResourceVersionAnnotation:        "",
		trustedCAConfigMapResourceVersionAnnotation:        "",
		proxyConfigResourceVersionAnnotation:               "",
		infrastructureConfigResourceVersionAnnotation:      "",
		consoleImageAnnotation:                             "",
		workloadManagementAnnotation:                       workloadManagementAnnotationValue,
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
			}},
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
	infrastructureConfigExternalTopologyMode := infrastructureConfigWithTopology(configv1.ExternalTopologyMode)

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
				TypeMeta:   metav1.TypeMeta{},
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
								consoleContainer(consoleOperatorConfig, defaultVolumeConfig(), proxyConfig),
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
				TypeMeta:   metav1.TypeMeta{},
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
								consoleContainer(consoleOperatorConfig, append(defaultVolumeConfig(), trustedCAVolume()), proxyConfig),
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
				TypeMeta:   metav1.TypeMeta{},
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
								consoleContainer(consoleOperatorConfig, defaultVolumeConfig(), proxyConfig),
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
		{
			name: "Test Infrastructure Config ExternalTopologyMode",
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
				infrastructure: infrastructureConfigExternalTopologyMode,
			},
			want: &appsv1.Deployment{
				TypeMeta:   metav1.TypeMeta{},
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
							// we do not want to deploy on master nodes
							NodeSelector: map[string]string{},
							Affinity:     consoleDeploymentAffinity,
							// toleration is a taint override. we can and should be scheduled on a master node.
							Tolerations:                   consoleDeploymentTolerations,
							PriorityClassName:             "system-cluster-critical",
							RestartPolicy:                 corev1.RestartPolicyAlways,
							SchedulerName:                 corev1.DefaultSchedulerName,
							TerminationGracePeriodSeconds: &gracePeriod,
							SecurityContext:               &corev1.PodSecurityContext{},
							Containers: []corev1.Container{
								consoleContainer(consoleOperatorConfig, defaultVolumeConfig(), proxyConfig),
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(DefaultDeployment(tt.args.config, tt.args.cm, tt.args.dica, tt.args.cm, tt.args.tca, tt.args.sec, tt.args.proxy, tt.args.infrastructure, tt.args.canMountCustomLogo), tt.want); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestDefaultDownloadsDeployment(t *testing.T) {

	var (
		defaultReplicaCount    int32 = DefaultConsoleReplicas
		singleNodeReplicaCount int32 = SingleNodeConsoleReplicas
		labels                       = util.LabelsForDownloads()
		gracePeriod            int64 = 0
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
		Affinity:                      downloadsPodAffinity(infrastructureConfigSingleReplica),
		Tolerations:                   tolerations(),
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
				TypeMeta:   metav1.TypeMeta{},
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
				TypeMeta:   metav1.TypeMeta{},
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
					}},
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
					}},
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
	oauthServingCert := corev1.Volume{
		Name: api.OAuthServingCertConfigMapName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: api.OAuthServingCertConfigMapName,
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
				oauthServingCert,
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
				oauthServingCert,
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
					Name:      api.OAuthServingCertConfigMapName,
					ReadOnly:  true,
					MountPath: "/var/oauth-serving-cert",
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
					Name:      api.OAuthServingCertConfigMapName,
					ReadOnly:  true,
					MountPath: "/var/oauth-serving-cert",
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
