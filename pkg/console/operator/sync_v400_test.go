package operator

import (
	"testing"

	"github.com/go-test/deep"
	"github.com/openshift/console-operator/pkg/api"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetNodeArchitectures(t *testing.T) {
	tests := []struct {
		name     string
		nodeList *v1.NodeList
		expected []string
	}{
		{
			name: "Test getNodeArchitectures",
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
							Name: "node-1",
							Labels: map[string]string{
								api.NodeArchitectureLabel: "baz",
							},
						},
					},
				},
			},
			expected: []string{"baz", "foo"},
		},
		{
			name:     "Test getNodeArchitectures empty node list",
			nodeList: &v1.NodeList{},
			expected: []string{},
		},
		{
			name: "Test getNodeArchitectures empty labels",
			nodeList: &v1.NodeList{
				Items: []v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-1",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-2",
							Labels: map[string]string{
								api.NodeArchitectureLabel: "baz",
							},
						},
					},
				},
			},
			expected: []string{"baz"},
		},
		{
			name: "Test getNodeArchitectures missing arch label",
			nodeList: &v1.NodeList{
				Items: []v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "node-1",
							Labels: map[string]string{},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-2",
							Labels: map[string]string{
								api.NodeArchitectureLabel: "baz",
							},
						},
					},
				},
			},
			expected: []string{"baz"},
		},
		{
			name: "Test getNodeArchitectures empty arch label",
			nodeList: &v1.NodeList{
				Items: []v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-1",
							Labels: map[string]string{
								api.NodeArchitectureLabel: "",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-2",
							Labels: map[string]string{
								api.NodeArchitectureLabel: "baz",
							},
						},
					},
				},
			},
			expected: []string{"baz"},
		},
		{
			name: "Test getNodeArchitectures duplicate arch label",
			nodeList: &v1.NodeList{
				Items: []v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-1",
							Labels: map[string]string{
								api.NodeArchitectureLabel: "baz",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node-2",
							Labels: map[string]string{
								api.NodeArchitectureLabel: "baz",
							},
						},
					},
				},
			},
			expected: []string{"baz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getNodeArchitectures(tt.nodeList)
			if diff := deep.Equal(tt.expected, actual); diff != nil {
				t.Error(diff)
			}
		})
	}
}
