package gemini

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDocumentBuilder(t *testing.T) {
	tests := []struct {
		name        string
		f           func() *DocumentBuilder
		expected    string
		expectedErr error
	}{
		{
			name: "an empty builder produces no output",
			f: func() *DocumentBuilder {
				return NewDocumentBuilder()
			},
			expected:    "",
			expectedErr: nil,
		},
	}
	for _, tt := range tests {
		tt := tt
		result, err := tt.f().Build()
		if err != tt.expectedErr {
			t.Errorf("expected err %q, got %q", tt.expectedErr, err)
			continue
		}
		actual := string(result)
		if diff := cmp.Diff(tt.expected, actual); diff != "" {
			t.Error(diff)
		}
	}
}
