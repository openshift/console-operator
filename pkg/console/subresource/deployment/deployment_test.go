package deployment

import (
	"testing"

	"github.com/go-test/deep"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
	operatorsv1 "github.com/openshift/api/operator/v1"
	v1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/subresource/configmap"
	"github.com/openshift/console-operator/pkg/console/subresource/util"
)

func TestDefaultDeployment(t *testing.T) {
	var (
		replicaCount int32 = 2
		labels             = map[string]string{"app": api.OpenShiftConsoleName, "component": "ui"}
		gracePeriod  int64 = 30
	)
	type args struct {
		// top level configs
		operatorConfig *operatorsv1.Console
		proxyConfig    *configv1.Proxy
		// configmaps
		consoleServerConfigMap *corev1.ConfigMap
		servingCAConfigMap     *corev1.ConfigMap
		trustedCAConfigMap     *corev1.ConfigMap
		// secrets
		oauthClientSecret *corev1.Secret
		servingCert       *corev1.Secret
		// other
		consoleRoute       *v1.Route
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
		Name:      api.OpenShiftConsoleName,
		Namespace: api.OpenShiftConsoleNamespace,
		Labels:    labels,
		Annotations: map[string]string{
			deploymentVersionHashKey: "lYUJi33gP6JzQ_0yPUf9jexYOfhNRL5KsYT6To7s-FKMTbc_cSKJA9iTscvI5bMVKzz1etGhLAIkXtdJ1YdIVg",
			consoleImageAnnotation:   "",
		},
	}

	consoleConfig := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "console-config",
			Namespace: api.OpenShiftConsoleNamespace,
			Labels:    labels,
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

	consoleDeploymentAffinity := &corev1.Affinity{
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

	tests := []struct {
		name   string
		input  args
		output *appsv1.Deployment
	}{
		{
			name: "Create default deployment",
			input: args{
				// top level configs
				operatorConfig: consoleOperatorConfig,
				proxyConfig:    proxyConfig,
				// configmaps
				consoleServerConfigMap: consoleConfig,
				servingCAConfigMap:     &corev1.ConfigMap{},
				trustedCAConfigMap:     trustedCAConfigMapEmpty,
				// secrets
				oauthClientSecret: &corev1.Secret{},
				servingCert:       &corev1.Secret{},
				// other
				consoleRoute:       &v1.Route{},
				canMountCustomLogo: false,
			},
			output: &appsv1.Deployment{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: consoleDeploymentObjectMeta,
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicaCount,
					Selector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
					Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{
						Name:   api.OpenShiftConsoleName,
						Labels: labels,
						Annotations: map[string]string{
							deploymentVersionHashKey: "lYUJi33gP6JzQ_0yPUf9jexYOfhNRL5KsYT6To7s-FKMTbc_cSKJA9iTscvI5bMVKzz1etGhLAIkXtdJ1YdIVg",
							consoleImageAnnotation:   "",
						},
					},
						Spec: corev1.PodSpec{
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
				},
			},
		},
		{
			// TODO: improve this test by calling the DefaultDeployment() func, but isolating the check against only
			// the output involving the volume & volumeMount portions that are relevant to the trusted CA.  It will
			// make finding and fixing an error much easier.
			name: "Create deployment with trusted CA volume mount",
			input: args{
				// top level configs
				operatorConfig: consoleOperatorConfig,
				proxyConfig:    proxyConfig,
				// configmaps
				consoleServerConfigMap: consoleConfig,
				servingCAConfigMap:     &corev1.ConfigMap{},
				trustedCAConfigMap:     trustedCAConfigMapSet,
				// secrets
				oauthClientSecret: &corev1.Secret{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Data:       nil,
					StringData: nil,
					Type:       "",
				},
				servingCert: &corev1.Secret{},
				// other
				consoleRoute: &v1.Route{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
				},
				canMountCustomLogo: false,
			},
			output: &appsv1.Deployment{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: consoleDeploymentObjectMeta,
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicaCount,
					Selector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
					Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{
						Name:   api.OpenShiftConsoleName,
						Labels: labels,
						Annotations: map[string]string{
							deploymentVersionHashKey: "lYUJi33gP6JzQ_0yPUf9jexYOfhNRL5KsYT6To7s-FKMTbc_cSKJA9iTscvI5bMVKzz1etGhLAIkXtdJ1YdIVg",
							consoleImageAnnotation:   "",
						},
					},
						Spec: corev1.PodSpec{
							// we output to deploy on master nodes
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
			depInput := DefaultDeployment(
				// top level configs
				tt.input.operatorConfig,
				tt.input.proxyConfig,
				// configmaps
				tt.input.consoleServerConfigMap,
				tt.input.servingCAConfigMap,
				tt.input.trustedCAConfigMap,
				// secrets
				tt.input.oauthClientSecret,
				tt.input.servingCert,
				// other
				tt.input.consoleRoute,
				tt.input.canMountCustomLogo)
			if diff := deep.Equal(depInput, tt.output); diff != nil {
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
	consoleServingCert := corev1.Volume{
		Name: ConsoleServingCertName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  ConsoleServingCertName,
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
		{name: "Test console volumes Mounts with TrustedCA",
			args: args{
				vc: append(defaultVolumeConfig(), trustedCAVolume()),
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
				t.Errorf("IsReady() = \n%v\n output \n%v", got, tt.want)
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
				t.Errorf("IsAvailableAndUpdated() = \n%v\n output \n%v", got, tt.want)
			}
		})
	}

}

func TestRSVToken(t *testing.T) {
	tests := []struct {
		name   string
		input  metav1.Object
		output string
	}{
		{
			name: "A configmap",
			input: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "Foo",
					Namespace:       "foo",
					ResourceVersion: "12345",
				},
			},
			output: "*v1.ConfigMap:foo:Foo:12345",
		}, {
			name: "A partial configmap",
			input: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "Foo",
				},
			},
			// Note that this func is really for objects that
			// have been created on the server and assigned a
			// ResourceVersion already.
			output: "*v1.ConfigMap::Foo:",
		}, {
			name: "A secret",
			input: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "Bar",
					Namespace:       "bar",
					ResourceVersion: "12345",
				},
			},
			output: "*v1.Secret:bar:Bar:12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if output := rsvToken(tt.input); output != tt.output {
				t.Errorf("output: %v \n", output)
			}
		})
	}
}

func TestRSVTokens(t *testing.T) {
	tests := []struct {
		name   string
		input  []metav1.Object
		output []string
	}{
		{
			name: "A configmap",
			input: []metav1.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "Foo",
						Namespace:       "foo",
						ResourceVersion: "12345",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "Bar",
						Namespace:       "bar",
						ResourceVersion: "12345",
					},
				},
			},
			output: []string{
				"*v1.ConfigMap:foo:Foo:12345",
				"*v1.Secret:bar:Bar:12345",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(rsvTokens(tt.input...), tt.output); diff != nil {
				t.Error(diff)
			}
		})
	}
}
func TestRedeployRSVHash(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		output string
	}{
		{
			name: "A configmap",
			input: rsvTokens(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "Foo",
					Namespace:       "foo",
					ResourceVersion: "12345",
				},
			},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "Bar",
						Namespace:       "bar",
						ResourceVersion: "12345",
					},
				}),
			output: "GfmhjN6n9D4a_LVBTXKkBbzdNb8F33EBGasRXgiLugNOv5tjC5_5DQtRU7ukjwFpU4_XuRB7xd-ThvfXF6eadA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if output := redeployRSVHash(tt.input); output != tt.output {
				t.Errorf("\ninput:%v \noutput %v \n", tt.input, output)
			}
		})
	}
}
