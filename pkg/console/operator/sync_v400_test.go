package operator

import (
	"testing"

	"github.com/go-test/deep"
	"github.com/openshift/console-operator/pkg/api"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetNodeComputeEnvironments(t *testing.T) {
	tests := []struct {
		name                     string
		nodeList                 *v1.NodeList
		expectedArchitectures    []string
		expectedOperatingSystems []string
	}{
		{
			name: "Test getNodeComputeEnvironments",
			nodeList: &v1.NodeList{
				Items: []v1.Node{
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
			},
			expectedArchitectures:    []string{"baz", "foo"},
			expectedOperatingSystems: []string{"bar", "bat"},
		},
		{
			name:                     "Test getNodeComputeEnvironments empty node list",
			nodeList:                 &v1.NodeList{},
			expectedArchitectures:    []string{},
			expectedOperatingSystems: []string{},
		},
		{
			name: "Test getNodeComputeEnvironments missing arch label",
			nodeList: &v1.NodeList{
				Items: []v1.Node{
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
			},
			expectedArchitectures:    []string{"baz"},
			expectedOperatingSystems: []string{"bar", "bat"},
		},
		{
			name: "Test getNodeComputeEnvironments empty arch label",
			nodeList: &v1.NodeList{
				Items: []v1.Node{
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
			},
			expectedArchitectures:    []string{"baz"},
			expectedOperatingSystems: []string{"bar", "bat"},
		},
		{
			name: "Test getNodeComputeEnvironments duplicate arch label",
			nodeList: &v1.NodeList{
				Items: []v1.Node{
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
			},
			expectedArchitectures:    []string{"baz"},
			expectedOperatingSystems: []string{"bar", "bat"},
		},
		{
			name: "Test getNodeComputeEnvironments missing OS label",
			nodeList: &v1.NodeList{
				Items: []v1.Node{
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
			},
			expectedArchitectures:    []string{"baz", "foo"},
			expectedOperatingSystems: []string{"bat"},
		},
		{
			name: "Test getNodeComputeEnvironments empty OS label",
			nodeList: &v1.NodeList{
				Items: []v1.Node{
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
			},
			expectedArchitectures:    []string{"baz", "foo"},
			expectedOperatingSystems: []string{"bat"},
		},
		{
			name: "Test getNodeComputeEnvironments duplicate OS label",
			nodeList: &v1.NodeList{
				Items: []v1.Node{
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
			},
			expectedArchitectures:    []string{"baz", "foo"},
			expectedOperatingSystems: []string{"bat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualArchitectures, actualOperatingSystems := getNodeComputeEnvironments(tt.nodeList)
			if diff := deep.Equal(tt.expectedArchitectures, actualArchitectures); diff != nil {
				t.Error(diff)
				return
			}

			if diff := deep.Equal(tt.expectedOperatingSystems, actualOperatingSystems); diff != nil {
				t.Error(diff)
				return
			}
		})
	}
}
