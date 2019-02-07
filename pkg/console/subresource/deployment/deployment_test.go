package deployment

import (
	"reflect"
	"testing"

	operatorsv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/configmap"
	"github.com/openshift/console-operator/pkg/console/subresource/util"

	v1 "github.com/openshift/console-operator/pkg/apis/console/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDefaultDeployment(t *testing.T) {
	var (
		replicaCount int32 = 3
		labels             = map[string]string{"app": api.OpenShiftConsoleName, "component": "ui"}
		gracePeriod  int64 = 30
	)
	type args struct {
		cr  *v1.Console
		cm  *corev1.ConfigMap
		ca  *corev1.ConfigMap
		sec *corev1.Secret
	}
	tests := []struct {
		name string
		args args
		want *appsv1.Deployment
	}{
		{
			name: "Test Default Config Map",
			args: args{
				cr: &v1.Console{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: v1.ConsoleSpec{
						OperatorSpec: operatorsv1.OperatorSpec{},
						Count:        replicaCount,
					},
					Status: v1.ConsoleStatus{},
				},
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
						Labels:          labels,
						Annotations:     nil,
						OwnerReferences: nil,
						Initializers:    nil,
						Finalizers:      nil,
						ClusterName:     "",
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
					Labels: labels,
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
						Name:   api.OpenShiftConsoleShortName,
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
							},
							RestartPolicy:                 corev1.RestartPolicyAlways,
							SchedulerName:                 corev1.DefaultSchedulerName,
							TerminationGracePeriodSeconds: &gracePeriod,
							SecurityContext:               &corev1.PodSecurityContext{},
							Containers: []corev1.Container{
								consoleContainer(nil),
							},
							Volumes: consoleVolumes(volumeConfigList),
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
			if got := DefaultDeployment(tt.args.cr, tt.args.cm, tt.args.cm, tt.args.sec); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("\nDefaultDeployment() = %v\n, want %v", got, tt.want)
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
			if got := Stub(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Stub() = %v, want %v", got, tt.want)
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
				vc: volumeConfigList,
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
					Name: configmap.ConsoleConfigMapName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configmap.ConsoleConfigMapName,
							},
							Items:       nil,
							DefaultMode: nil,
							Optional:    nil,
						},
					},
				},
				{
					Name: configmap.ServiceCAConfigMapName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configmap.ServiceCAConfigMapName,
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
			if got := consoleVolumes(tt.args.vc); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("\nconsoleVolumes() = %v, \nwant %v", got, tt.want)
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
				vc: volumeConfigList,
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
					Name:      configmap.ConsoleConfigMapName,
					ReadOnly:  true,
					MountPath: "/var/console-config",
				},
				{
					Name:      configmap.ServiceCAConfigMapName,
					ReadOnly:  true,
					MountPath: "/var/service-ca",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := consoleVolumeMounts(tt.args.vc); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("consoleVolumeMounts() = %v, \nwant %v", got, tt.want)
			}
		})
	}
}
