package operator

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-test/deep"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/console-operator/pkg/api"
	"github.com/openshift/console-operator/pkg/console/telemetry"
)

func TestGetNodeComputeEnvironments(t *testing.T) {
	tests := []struct {
		name                     string
		nodeList                 []*v1.Node
		expectedArchitectures    []string
		expectedOperatingSystems []string
	}{
		{
			name: "Test getNodeComputeEnvironments",
			nodeList: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							api.NodeArchitectureLabel:    "foo",
							api.NodeOperatingSystemLabel: "bar",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							api.NodeArchitectureLabel:    "baz",
							api.NodeOperatingSystemLabel: "bat",
						},
					},
				},
			},
			expectedArchitectures:    []string{"baz", "foo"},
			expectedOperatingSystems: []string{"bar", "bat"},
		},
		{
			name:                     "Test getNodeComputeEnvironments empty node list",
			nodeList:                 []*v1.Node{},
			expectedArchitectures:    []string{},
			expectedOperatingSystems: []string{},
		},
		{
			name: "Test getNodeComputeEnvironments missing arch label",
			nodeList: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							api.NodeOperatingSystemLabel: "bar",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-2",
						Labels: map[string]string{
							api.NodeArchitectureLabel:    "baz",
							api.NodeOperatingSystemLabel: "bat",
						},
					},
				},
			},
			expectedArchitectures:    []string{"baz"},
			expectedOperatingSystems: []string{"bar", "bat"},
		},
		{
			name: "Test getNodeComputeEnvironments empty arch label",
			nodeList: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							api.NodeArchitectureLabel:    "",
							api.NodeOperatingSystemLabel: "bar",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-2",
						Labels: map[string]string{
							api.NodeArchitectureLabel:    "baz",
							api.NodeOperatingSystemLabel: "bat",
						},
					},
				},
			},
			expectedArchitectures:    []string{"baz"},
			expectedOperatingSystems: []string{"bar", "bat"},
		},
		{
			name: "Test getNodeComputeEnvironments duplicate arch label",
			nodeList: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							api.NodeArchitectureLabel:    "baz",
							api.NodeOperatingSystemLabel: "bar",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-2",
						Labels: map[string]string{
							api.NodeArchitectureLabel:    "baz",
							api.NodeOperatingSystemLabel: "bat",
						},
					},
				},
			},
			expectedArchitectures:    []string{"baz"},
			expectedOperatingSystems: []string{"bar", "bat"},
		},
		{
			name: "Test getNodeComputeEnvironments missing OS label",
			nodeList: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							api.NodeArchitectureLabel: "foo",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-2",
						Labels: map[string]string{
							api.NodeArchitectureLabel:    "baz",
							api.NodeOperatingSystemLabel: "bat",
						},
					},
				},
			},
			expectedArchitectures:    []string{"baz", "foo"},
			expectedOperatingSystems: []string{"bat"},
		},
		{
			name: "Test getNodeComputeEnvironments empty OS label",
			nodeList: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							api.NodeArchitectureLabel:    "foo",
							api.NodeOperatingSystemLabel: "",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-2",
						Labels: map[string]string{
							api.NodeArchitectureLabel:    "baz",
							api.NodeOperatingSystemLabel: "bat",
						},
					},
				},
			},
			expectedArchitectures:    []string{"baz", "foo"},
			expectedOperatingSystems: []string{"bat"},
		},
		{
			name: "Test getNodeComputeEnvironments duplicate OS label",
			nodeList: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							api.NodeArchitectureLabel:    "foo",
							api.NodeOperatingSystemLabel: "bat",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-2",
						Labels: map[string]string{
							api.NodeArchitectureLabel:    "baz",
							api.NodeOperatingSystemLabel: "bat",
						},
					},
				},
			},
			expectedArchitectures:    []string{"baz", "foo"},
			expectedOperatingSystems: []string{"bat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualArchitectures, actualOperatingSystems := getNodeComputeEnvironments(tt.nodeList)
			if diff := deep.Equal(tt.expectedArchitectures, actualArchitectures); diff != nil {
				t.Errorf("Architecture mismatch: %v", diff)
			}

			if diff := deep.Equal(tt.expectedOperatingSystems, actualOperatingSystems); diff != nil {
				t.Errorf("OS mismatch: %v", diff)
			}
		})
	}
}

// TestCheckDeploymentRolloutStatus tests the deployment rollout status check
// introduced in OCPBUGS-93982 (v2). The function examines the deployment
// controller's own Progressing condition rather than just the generation gap,
// following the 3CMO pattern (openshift/cluster-cloud-controller-manager-operator#488).
func TestCheckDeploymentRolloutStatus(t *testing.T) {
	tests := []struct {
		name               string
		generation         int64
		observedGeneration int64
		conditions         []appsv1.DeploymentCondition
		wantErr            bool
		wantErrContains    string
	}{
		{
			name:               "rollout complete: NewReplicaSetAvailable",
			generation:         5,
			observedGeneration: 5,
			conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentProgressing,
				Status: v1.ConditionTrue,
				Reason: "NewReplicaSetAvailable",
			}},
			wantErr: false,
		},
		{
			name:               "generation gap: not yet observed",
			generation:         6,
			observedGeneration: 5,
			wantErr:            true,
			wantErrContains:    "not yet observed",
		},
		{
			name:               "rollout in progress: ReplicaSetUpdated",
			generation:         5,
			observedGeneration: 5,
			conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentProgressing,
				Status: v1.ConditionTrue,
				Reason: "ReplicaSetUpdated",
			}},
			wantErr:         true,
			wantErrContains: "rollout in progress: ReplicaSetUpdated",
		},
		{
			name:               "rollout in progress: NewReplicaSetCreated",
			generation:         5,
			observedGeneration: 5,
			conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentProgressing,
				Status: v1.ConditionTrue,
				Reason: "NewReplicaSetCreated",
			}},
			wantErr:         true,
			wantErrContains: "rollout in progress: NewReplicaSetCreated",
		},
		{
			name:               "rollout stalled: ProgressDeadlineExceeded",
			generation:         5,
			observedGeneration: 5,
			conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentProgressing,
				Status: v1.ConditionFalse,
				Reason: "ProgressDeadlineExceeded",
			}},
			wantErr:         true,
			wantErrContains: "ProgressDeadlineExceeded",
		},
		{
			name:               "no progressing condition yet",
			generation:         5,
			observedGeneration: 5,
			conditions:         []appsv1.DeploymentCondition{},
			wantErr:            true,
			wantErrContains:    "not yet available",
		},
		{
			name:               "observed greater than generation: checks conditions",
			generation:         3,
			observedGeneration: 4,
			conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentProgressing,
				Status: v1.ConditionTrue,
				Reason: "NewReplicaSetAvailable",
			}},
			wantErr: false,
		},
		{
			name:               "initial rollout: Generation 1, ObservedGeneration 0",
			generation:         1,
			observedGeneration: 0,
			wantErr:            true,
			wantErrContains:    "not yet observed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Generation: tt.generation,
				},
				Status: appsv1.DeploymentStatus{
					ObservedGeneration: tt.observedGeneration,
					Conditions:         tt.conditions,
				},
			}

			err := checkDeploymentRolloutStatus(deployment)

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrContains)
			}
		})
	}
}

func newIndexer(keyFunc cache.KeyFunc) cache.Indexer {
	return cache.NewIndexer(keyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
}

func newTestConsoleOperator(t *testing.T, telemeterAvailable bool, pullSecretHasCloudAuth bool) *consoleOperator {
	t.Helper()

	// telemetry-config ConfigMap in operator namespace
	telemetryConfigCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      telemetry.TelemetryConfigMapName,
			Namespace: api.OpenShiftConsoleOperatorNamespace,
		},
		Data: map[string]string{
			"SEGMENT_API_HOST":       "https://segment.example.com",
			"SEGMENT_JS_HOST":        "https://segment-js.example.com",
			"SEGMENT_PUBLIC_API_KEY": "test-key",
		},
	}
	operatorNSCMIndexer := newIndexer(cache.MetaNamespaceKeyFunc)
	if err := operatorNSCMIndexer.Add(telemetryConfigCM); err != nil {
		t.Fatalf("failed to add telemetry configmap to indexer: %v", err)
	}

	// ClusterVersion
	cv := &configv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: api.VersionResourceName,
		},
		Spec: configv1.ClusterVersionSpec{
			ClusterID: "test-cluster-id",
		},
	}
	cvIndexer := newIndexer(func(obj interface{}) (string, error) {
		meta := obj.(metav1.ObjectMetaAccessor).GetObjectMeta()
		return meta.GetName(), nil
	})
	if err := cvIndexer.Add(cv); err != nil {
		t.Fatalf("failed to add clusterversion to indexer: %v", err)
	}

	// telemeter-client deployment
	var replicas int32
	if telemeterAvailable {
		replicas = 1
	}
	telemeterDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      telemetry.TelemeterClientDeploymentName,
			Namespace: telemetry.TelemeterClientDeploymentNamespace,
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: replicas,
		},
	}
	deployIndexer := newIndexer(cache.MetaNamespaceKeyFunc)
	if err := deployIndexer.Add(telemeterDeploy); err != nil {
		t.Fatalf("failed to add telemeter deployment to indexer: %v", err)
	}

	// pull-secret
	pullSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      telemetry.PullSecretName,
			Namespace: api.OpenShiftConfigNamespace,
		},
		Data: map[string][]byte{},
	}
	if pullSecretHasCloudAuth {
		dockerConfig := telemetry.DockerConfig{
			Auths: map[string]telemetry.DockerAuthEntry{
				"cloud.openshift.com": {Auth: "test-token"},
			},
		}
		configBytes, err := json.Marshal(dockerConfig)
		if err != nil {
			t.Fatalf("failed to marshal docker config: %v", err)
		}
		pullSecret.Data[".dockerconfigjson"] = configBytes
	} else {
		dockerConfig := telemetry.DockerConfig{
			Auths: map[string]telemetry.DockerAuthEntry{
				"quay.io": {Auth: "quay-token"},
			},
		}
		configBytes, err := json.Marshal(dockerConfig)
		if err != nil {
			t.Fatalf("failed to marshal docker config: %v", err)
		}
		pullSecret.Data[".dockerconfigjson"] = configBytes
	}
	secretIndexer := newIndexer(cache.MetaNamespaceKeyFunc)
	if err := secretIndexer.Add(pullSecret); err != nil {
		t.Fatalf("failed to add pull-secret to indexer: %v", err)
	}

	return &consoleOperator{
		operatorNSConfigMapLister:  corev1listers.NewConfigMapLister(operatorNSCMIndexer),
		clusterVersionLister:       configlistersv1.NewClusterVersionLister(cvIndexer),
		monitoringDeploymentLister: appsv1listers.NewDeploymentLister(deployIndexer),
		configNSSecretLister:       corev1listers.NewSecretLister(secretIndexer),
	}
}

func TestGetTelemetryConfiguration_StableKeySet(t *testing.T) {
	tests := []struct {
		name               string
		telemeterAvailable bool
		hasCloudAuth       bool
		expectDisabledVal  string
	}{
		{
			name:               "telemeter available with cloud auth",
			telemeterAvailable: true,
			hasCloudAuth:       true,
			expectDisabledVal:  "false",
		},
		{
			name:               "telemeter unavailable with cloud auth",
			telemeterAvailable: false,
			hasCloudAuth:       true,
			expectDisabledVal:  "true",
		},
		{
			name:               "telemeter unavailable without cloud auth (disconnected)",
			telemeterAvailable: false,
			hasCloudAuth:       false,
			expectDisabledVal:  "true",
		},
		{
			name:               "telemeter available without cloud auth (disconnected)",
			telemeterAvailable: true,
			hasCloudAuth:       false,
			expectDisabledVal:  "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			co := newTestConsoleOperator(t, tt.telemeterAvailable, tt.hasCloudAuth)
			operatorConfig := &operatorv1.Console{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			}

			config, err := co.GetTelemetryConfiguration(context.Background(), operatorConfig)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if _, ok := config["CLUSTER_ID"]; !ok {
				t.Error("expected CLUSTER_ID key to be present")
			}
			if _, ok := config["ORGANIZATION_ID"]; !ok {
				t.Error("expected ORGANIZATION_ID key to always be present")
			}
			if _, ok := config["ACCOUNT_MAIL"]; !ok {
				t.Error("expected ACCOUNT_MAIL key to always be present")
			}

			disabledVal, ok := config["TELEMETER_CLIENT_DISABLED"]
			if !ok {
				t.Error("expected TELEMETER_CLIENT_DISABLED key to always be present")
			}
			if disabledVal != tt.expectDisabledVal {
				t.Errorf("expected TELEMETER_CLIENT_DISABLED=%q, got %q", tt.expectDisabledVal, disabledVal)
			}
		})
	}
}

func TestGetTelemetryConfiguration_KeySetStableAcrossAvailabilityChange(t *testing.T) {
	operatorConfig := &operatorv1.Console{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	coUnavailable := newTestConsoleOperator(t, false, true)
	configUnavailable, err := coUnavailable.GetTelemetryConfiguration(context.Background(), operatorConfig)
	if err != nil {
		t.Fatalf("unexpected error (unavailable): %v", err)
	}

	coAvailable := newTestConsoleOperator(t, true, true)
	configAvailable, err := coAvailable.GetTelemetryConfiguration(context.Background(), operatorConfig)
	if err != nil {
		t.Fatalf("unexpected error (available): %v", err)
	}

	keysUnavailable := sortedKeys(configUnavailable)
	keysAvailable := sortedKeys(configAvailable)

	// All keys must be present in both states — no key-set difference allowed.
	sharedKeys := []string{"CLUSTER_ID", "ORGANIZATION_ID", "ACCOUNT_MAIL", "TELEMETER_CLIENT_DISABLED", "SEGMENT_API_HOST", "SEGMENT_JS_HOST", "SEGMENT_PUBLIC_API_KEY"}
	for _, key := range sharedKeys {
		if _, ok := configUnavailable[key]; !ok {
			t.Errorf("key %q missing from unavailable config, keys present: %v", key, keysUnavailable)
		}
		if _, ok := configAvailable[key]; !ok {
			t.Errorf("key %q missing from available config, keys present: %v", key, keysAvailable)
		}
	}

	if len(keysUnavailable) != len(keysAvailable) {
		t.Fatalf("key set sizes differ: unavailable=%v, available=%v", keysUnavailable, keysAvailable)
	}
	for i := range keysUnavailable {
		if keysUnavailable[i] != keysAvailable[i] {
			t.Errorf("key set mismatch at index %d: unavailable has %q, available has %q", i, keysUnavailable[i], keysAvailable[i])
		}
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Verify that GetAccessToken failure does not cause GetTelemetryConfiguration to error
func TestGetTelemetryConfiguration_DisconnectedClusterNoError(t *testing.T) {
	co := newTestConsoleOperator(t, false, false)
	operatorConfig := &operatorv1.Console{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	config, err := co.GetTelemetryConfiguration(context.Background(), operatorConfig)
	if err != nil {
		t.Fatalf("expected no error on disconnected cluster, got: %v", err)
	}

	if config["TELEMETER_CLIENT_DISABLED"] != "true" {
		t.Error("expected TELEMETER_CLIENT_DISABLED=true on disconnected cluster")
	}
	if _, ok := config["ORGANIZATION_ID"]; !ok {
		t.Error("expected ORGANIZATION_ID key to be present even on disconnected cluster")
	}
	if _, ok := config["ACCOUNT_MAIL"]; !ok {
		t.Error("expected ACCOUNT_MAIL key to be present even on disconnected cluster")
	}
}

// TestEvaluateDeploymentAvailability tests the grace period logic for
// OCPBUGS-67134: the operator should suppress brief Available=False blips
// when all replicas are temporarily offline during disruptive operations.
func TestEvaluateDeploymentAvailability(t *testing.T) {
	makeDeployment := func(availableReplicas int32) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "console",
				Namespace: "openshift-console",
			},
			Status: appsv1.DeploymentStatus{
				AvailableReplicas: availableReplicas,
				ReadyReplicas:     availableReplicas,
			},
		}
	}

	t.Run("available deployment reports Available=True and updates timestamp", func(t *testing.T) {
		co := &consoleOperator{}
		deployment := makeDeployment(2)

		prefix, reason, err := co.evaluateDeploymentAvailability(deployment)

		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if reason != "" {
			t.Errorf("expected empty reason, got: %q", reason)
		}
		if prefix != "Deployment" {
			t.Errorf("expected prefix 'Deployment', got: %q", prefix)
		}
		if co.lastDeploymentAvailableTime.IsZero() {
			t.Error("expected lastDeploymentAvailableTime to be set")
		}
	})

	t.Run("unavailable with no prior availability reports Available=False immediately", func(t *testing.T) {
		co := &consoleOperator{}
		deployment := makeDeployment(0)

		_, reason, err := co.evaluateDeploymentAvailability(deployment)

		if err == nil {
			t.Error("expected error, got nil")
		}
		if reason != "InsufficientReplicas" {
			t.Errorf("expected reason 'InsufficientReplicas', got: %q", reason)
		}
	})

	t.Run("unavailable within grace period reports Available=True (suppressed)", func(t *testing.T) {
		co := &consoleOperator{
			lastDeploymentAvailableTime: time.Now().Add(-10 * time.Second),
		}
		deployment := makeDeployment(0)

		_, reason, err := co.evaluateDeploymentAvailability(deployment)

		if err != nil {
			t.Errorf("expected no error within grace period, got: %v", err)
		}
		if reason != "" {
			t.Errorf("expected empty reason within grace period, got: %q", reason)
		}
	})

	t.Run("unavailable beyond grace period reports Available=False", func(t *testing.T) {
		co := &consoleOperator{
			lastDeploymentAvailableTime: time.Now().Add(-3 * time.Minute),
		}
		deployment := makeDeployment(0)

		_, reason, err := co.evaluateDeploymentAvailability(deployment)

		if err == nil {
			t.Error("expected error beyond grace period, got nil")
		}
		if reason != "InsufficientReplicas" {
			t.Errorf("expected reason 'InsufficientReplicas', got: %q", reason)
		}
	})

	t.Run("recovery after blip resets timestamp", func(t *testing.T) {
		co := &consoleOperator{
			lastDeploymentAvailableTime: time.Now().Add(-5 * time.Second),
		}

		// Simulate: was available, went to 0, then recovered
		deployment := makeDeployment(0)
		_, _, err := co.evaluateDeploymentAvailability(deployment)
		if err != nil {
			t.Error("expected suppression within grace period")
		}

		// Recovery
		deployment = makeDeployment(2)
		before := co.lastDeploymentAvailableTime
		_, _, err = co.evaluateDeploymentAvailability(deployment)
		if err != nil {
			t.Errorf("expected no error on recovery, got: %v", err)
		}
		if !co.lastDeploymentAvailableTime.After(before) {
			t.Error("expected lastDeploymentAvailableTime to be updated on recovery")
		}
	})

	t.Run("unavailable just inside grace period boundary reports Available=True", func(t *testing.T) {
		co := &consoleOperator{
			lastDeploymentAvailableTime: time.Now().Add(-deploymentAvailableGracePeriod + time.Second),
		}
		deployment := makeDeployment(0)

		_, reason, err := co.evaluateDeploymentAvailability(deployment)

		if err != nil {
			t.Errorf("expected no error just inside grace period, got: %v", err)
		}
		if reason != "" {
			t.Errorf("expected empty reason just inside grace period, got: %q", reason)
		}
	})

	t.Run("error message includes ready replica count", func(t *testing.T) {
		co := &consoleOperator{}
		deployment := makeDeployment(0)

		_, _, err := co.evaluateDeploymentAvailability(deployment)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		expected := "0 replicas available for console deployment"
		if err.Error() != expected {
			t.Errorf("expected error message %q, got %q", expected, err.Error())
		}
	})
}
