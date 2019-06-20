package errors

import (
	"fmt"
	"testing"

	"github.com/go-test/deep"
)

func TestIsCustomLogoError(t *testing.T) {
	tests := []struct {
		name   string
		input  error
		output bool
	}{
		{
			name:   "IsCustomLogoError returns true if passed a CustomLogoError",
			input:  NewCustomLogoError("Yup, its a custom logo error"),
			output: true,
		}, {
			name:   "IsCustomLogoError returns false if passed a regular Error",
			input:  fmt.Errorf("A regular error"),
			output: false,
		}, {
			name:   "IsCustomLogoError returns true if passed nil",
			input:  nil,
			output: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(IsCustomLogoError(tt.input), tt.output); diff != nil {
				t.Error(diff)
			}
		})
	}

}
