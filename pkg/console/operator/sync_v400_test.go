package operator

import (
	"fmt"
	"testing"

	"github.com/go-test/deep"
	"github.com/openshift/console-operator/pkg/api"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		{
			name: "3 AMD64 nodes - should return single architecture",
			nodeList: func() []*v1.Node {
				nodes := make([]*v1.Node, 3)
				for i := 0; i < 3; i++ {
					nodes[i] = &v1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("node-%d", i),
							Labels: map[string]string{
								api.NodeArchitectureLabel:    "amd64",
								api.NodeOperatingSystemLabel: "linux",
							},
						},
					}
				}
				return nodes
			}(),
			expectedArchitectures:    []string{"amd64"}, // Single architecture despite 3 nodes
			expectedOperatingSystems: []string{"linux"},
		},
		{
			name: "2 AMD64 + 1 Power node - should return both architectures",
			nodeList: func() []*v1.Node {
				nodes := make([]*v1.Node, 3)
				// 2 AMD64 nodes
				for i := 0; i < 2; i++ {
					nodes[i] = &v1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: fmt.Sprintf("amd64-node-%d", i),
							Labels: map[string]string{
								api.NodeArchitectureLabel:    "amd64",
								api.NodeOperatingSystemLabel: "linux",
							},
						},
					}
				}
				// 1 Power node
				nodes[2] = &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "power-node",
						Labels: map[string]string{
							api.NodeArchitectureLabel:    "ppc64le",
							api.NodeOperatingSystemLabel: "linux",
						},
					},
				}
				return nodes
			}(),
			expectedArchitectures:    []string{"amd64", "ppc64le"}, // Both architectures
			expectedOperatingSystems: []string{"linux"},
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
