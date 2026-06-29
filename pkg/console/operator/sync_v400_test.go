package operator

import (
	"testing"

	"github.com/go-test/deep"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/console-operator/pkg/api"
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
