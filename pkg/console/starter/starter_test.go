package starter

import (
	"reflect"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestDeduplicateObjectReferences(t *testing.T) {
	tests := []struct {
		name     string
		input    []configv1.ObjectReference
		expected []configv1.ObjectReference
	}{
		{
			name:     "no duplicates",
			input:    []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1"}, {Group: "g2", Resource: "r2", Name: "n2"}},
			expected: []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1"}, {Group: "g2", Resource: "r2", Name: "n2"}},
		},
		{
			name:     "with duplicates",
			input:    []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1"}, {Group: "g1", Resource: "r1", Name: "n1"}},
			expected: []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1"}},
		},
		{
			name:     "different namespace not duplicate",
			input:    []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1", Namespace: "ns1"}, {Group: "g1", Resource: "r1", Name: "n1", Namespace: "ns2"}},
			expected: []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1", Namespace: "ns1"}, {Group: "g1", Resource: "r1", Name: "n1", Namespace: "ns2"}},
		},
		{
			name:     "all fields equal",
			input:    []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1", Namespace: "ns1"}, {Group: "g1", Resource: "r1", Name: "n1", Namespace: "ns1"}},
			expected: []configv1.ObjectReference{{Group: "g1", Resource: "r1", Name: "n1", Namespace: "ns1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateObjectReferences(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("deduplicateObjectReferences() = %v, want %v", got, tt.expected)
			}
		})
	}
}
