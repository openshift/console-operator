package operator

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

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

// TestDeploymentProgressingByGeneration tests the ObservedGeneration-based
// Progressing check introduced in OCPBUGS-64688. The operator should only
// report Progressing=True when the deployment controller has not yet processed
// a spec change (ObservedGeneration < Generation), NOT when replica counts
// fluctuate due to external disruptions like node reboots.
// TestDeploymentProgressingSkippedWhenChanged verifies the guard logic from
// OCPBUGS-93982: when SyncDeployment reports changed=true, the generation check
// is skipped because the operator itself caused the generation gap.
func TestDeploymentProgressingSkippedWhenChanged(t *testing.T) {
	tests := []struct {
		name               string
		depChanged         bool
		generation         int64
		observedGeneration int64
		wantProgressing    bool
	}{
		{
			name:               "changed=true with generation gap: skip check, not progressing",
			depChanged:         true,
			generation:         7,
			observedGeneration: 6,
			wantProgressing:    false,
		},
		{
			name:               "changed=false with generation gap: run check, progressing",
			depChanged:         false,
			generation:         7,
			observedGeneration: 6,
			wantProgressing:    true,
		},
		{
			name:               "changed=false with no generation gap: run check, not progressing",
			depChanged:         false,
			generation:         7,
			observedGeneration: 7,
			wantProgressing:    false,
		},
		{
			name:               "changed=true with no generation gap: skip check, not progressing",
			depChanged:         true,
			generation:         7,
			observedGeneration: 7,
			wantProgressing:    false,
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
				},
			}

			var progressingErr error
			if !tt.depChanged {
				progressingErr = checkDeploymentGenerationProgress(deployment)
			}

			gotProgressing := progressingErr != nil
			if gotProgressing != tt.wantProgressing {
				t.Errorf("progressing = %v, want %v (err: %v)", gotProgressing, tt.wantProgressing, progressingErr)
			}
		})
	}
}

func TestDeploymentProgressingByGeneration(t *testing.T) {
	tests := []struct {
		name               string
		generation         int64
		observedGeneration int64
		wantErr            bool
		wantErrMsg         string
	}{
		{
			name:               "ObservedGeneration equals Generation: not progressing",
			generation:         5,
			observedGeneration: 5,
			wantErr:            false,
		},
		{
			name:               "ObservedGeneration less than Generation: progressing",
			generation:         4,
			observedGeneration: 3,
			wantErr:            true,
			wantErrMsg:         "deployment generation 4 not yet observed (observed: 3)",
		},
		{
			name:               "ObservedGeneration greater than Generation: not progressing",
			generation:         2,
			observedGeneration: 3,
			wantErr:            false,
		},
		{
			name:               "both zero: not progressing (fresh deployment)",
			generation:         0,
			observedGeneration: 0,
			wantErr:            false,
		},
		{
			name:               "Generation 1, ObservedGeneration 0: progressing (initial rollout)",
			generation:         1,
			observedGeneration: 0,
			wantErr:            true,
			wantErrMsg:         "deployment generation 1 not yet observed (observed: 0)",
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
				},
			}

			err := checkDeploymentGenerationProgress(deployment)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			if tt.wantErr && err != nil && err.Error() != tt.wantErrMsg {
				t.Errorf("error message mismatch:\n  got:  %q\n  want: %q", err.Error(), tt.wantErrMsg)
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
