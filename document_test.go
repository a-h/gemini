package gemini

import (
	"bytes"
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
	// Version 1 = 223 ns per operation, with 5 allocations.
	b.ReportAllocs()
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

func BenchmarkDocumentWriter(b *testing.B) {
	b.ReportAllocs()
	// By moving the creation of the io.Writer out of the type, the
	// buffer can be reused, resulting in lower execution speeds.
	// This comes in at 167.0 ns per operation, with zero allocations.
	// In practical terms, the io.Writer is most likely a file, or the
	// Gemini output stream.
	w := new(bytes.Buffer)
	for i := 0; i < b.N; i++ {
		db := NewDocumentWriter(w)
		var err error
		if err = db.Header1("heading 1"); err != nil {
			b.Error(err)
		}
		if err = db.Header2("heading 2"); err != nil {
			b.Error(err)
		}
		if err = db.Line("normal text"); err != nil {
			b.Error(err)
		}
		if err = db.Quote("quote"); err != nil {
			b.Error(err)
		}
		if len(w.Bytes()) == 0 {
			b.Error("expected output, but didn't get any")
		}
		w.Reset()
	}
}
