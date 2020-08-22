package gemini

import (
	"context"
	"io/ioutil"
	"net/url"
	"testing"
)

var geminiSuccessHeader = Header{
	Code: CodeSuccess,
	Meta: DefaultMIMEType,
}

func TestFileSystemHandler(t *testing.T) {
	var tests = []struct {
		name           string
		url            string
		expectedHeader Header
		expectedBody   string
	}{
		{
			name:           "if a directory contains index.gmi, it is used",
			url:            "/a",
			expectedHeader: geminiSuccessHeader,
			expectedBody:   "# /tests/a/index.gmi\n",
		},
		{
			name:           "files can be accessed directly",
			url:            "/a/index.gmi",
			expectedHeader: geminiSuccessHeader,
			expectedBody:   "# /tests/a/index.gmi\n",
		},
		{
			name:           "a slash prefix is added if missing",
			url:            "a/index.gmi",
			expectedHeader: geminiSuccessHeader,
			expectedBody:   "# /tests/a/index.gmi\n",
		},
		{
			name:           "if a directory does not contain an index, a listing is returned",
			url:            "/b",
			expectedHeader: geminiSuccessHeader,
			expectedBody: `# Index of /b/

=> ../
=> c/
=> not_index
`,
		},
		{
			name: "directory traversal attacks are deflected",
			url:  "../a/index.gmi",
			expectedHeader: Header{
				Code: CodeBadRequest,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			dir := Dir("tests")
			h := FileSystemHandler(dir)
			u, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("failed to parse URL %q", tt.url)
			}
			r := &Request{
				Context: context.Background(),
				URL:     u,
			}
			resp, err := Record(r, h)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.expectedHeader.Code != resp.Header.Code {
				t.Errorf("expected header code %v, got %v", tt.expectedHeader.Code, resp.Header.Code)
			}
			if tt.expectedHeader.Meta != resp.Header.Meta {
				t.Errorf("expected header meta %q, got %q", tt.expectedHeader.Meta, resp.Header.Meta)
			}
			bdy, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("unexpected error reading body: %v", err)
			}
			if tt.expectedBody != string(bdy) {
				t.Errorf("expected\n%v\nactual\n%v", tt.expectedBody, string(bdy))
			}
		})
	}
}
