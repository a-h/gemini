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

func BenchmarkDocumentBuilder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db := NewDocumentBuilder()
		var err error
		if err = db.AddH1Header("heading 1"); err != nil {
			b.Error(err)
		}
		if err = db.AddH2Header("heading 2"); err != nil {
			b.Error(err)
		}
		if err = db.AddLine("normal text"); err != nil {
			b.Error(err)
		}
		if err = db.AddQuote("quote"); err != nil {
			b.Error(err)
		}
		result, err := db.Build()
		if err != nil {
			b.Error(err)
		}
		if len(result) == 0 {
			b.Error("expected output, but didn't get any")
		}
	}
}
