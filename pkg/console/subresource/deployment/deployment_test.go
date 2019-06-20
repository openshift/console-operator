package deployment

import (
	"testing"

	"github.com/go-test/deep"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorsv1 "github.com/openshift/api/operator/v1"
	v1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

func TestDefaultDeployment(t *testing.T) {
	var (
		replicaCount int32 = 2
		labels             = map[string]string{"app": api.OpenShiftConsoleName, "component": "ui"}
		gracePeriod  int64 = 30
	)
	type args struct {
		config             *operatorsv1.Console
		cm                 *corev1.ConfigMap
		ca                 *corev1.ConfigMap
		sec                *corev1.Secret
		rt                 *v1.Route
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
	tests := []struct {
		name string
		args args
		want *appsv1.Deployment
	}{
		{
			name: "Test Default Config Map",
			args: args{
				config: consoleOperatorConfig,
				cm: &corev1.ConfigMap{
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
						Initializers:               nil,
						Finalizers:                 nil,
						ClusterName:                "",
					},
					Data:       map[string]string{"console-config.yaml": ""},
					BinaryData: nil,
				},
				ca: &corev1.ConfigMap{},
				sec: &corev1.Secret{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Data:       nil,
					StringData: nil,
					Type:       "",
				},
				rt: &v1.Route{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
				},
			},
			want: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
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
						configMapResourceVersionAnnotation:          "",
						secretResourceVersionAnnotation:             "",
						serviceCAConfigMapResourceVersionAnnotation: "",
						consoleImageAnnotation:                      "",
					},
					OwnerReferences: nil,
					Initializers:    nil,
					Finalizers:      nil,
					ClusterName:     "",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicaCount,
					Selector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
					Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{
						Name:   api.OpenShiftConsoleName,
						Labels: labels,
						Annotations: map[string]string{
							configMapResourceVersionAnnotation:          "",
							secretResourceVersionAnnotation:             "",
							serviceCAConfigMapResourceVersionAnnotation: "",
							consoleImageAnnotation:                      "",
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
								consoleContainer(consoleOperatorConfig, defaultVolumeConfig()),
							},
							Volumes: consoleVolumes(defaultVolumeConfig()),
						},
					},
					Strategy:                appsv1.DeploymentStrategy{},
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
			if diff := deep.Equal(DefaultDeployment(tt.args.config, tt.args.cm, tt.args.cm, tt.args.sec, tt.args.rt, tt.args.canMountCustomLogo), tt.want); diff != nil {
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
					Initializers:               nil,
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
				{
					Name: ConsoleServingCertName,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  ConsoleServingCertName,
							Items:       nil,
							DefaultMode: nil,
							Optional:    nil,
						},
					},
				},
				{
					Name: ConsoleOauthConfigName,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  ConsoleOauthConfigName,
							Items:       nil,
							DefaultMode: nil,
							Optional:    nil,
						},
					},
				},
				{
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
				},
				{
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
					Name:      ConsoleServingCertName,
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
