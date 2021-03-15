package gemini

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
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
			name:           "directories without a trailing slash are redirected",
			url:            "/a",
			expectedHeader: Header{Code: CodeRedirectPermanent, Meta: "/a/"},
			expectedBody:   "",
		},
		{
			name:           "if a directory contains index.gmi, it is used",
			url:            "/a/",
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
			name: "non-existent files return a 51 status code",
			url:  "/a/non-existent.gmi",
			expectedHeader: Header{
				Code: CodeNotFound,
				Meta: "",
			},
			expectedBody: "",
		},
		{
			name:           "a slash prefix is added if missing",
			url:            "a/index.gmi",
			expectedHeader: geminiSuccessHeader,
			expectedBody:   "# /tests/a/index.gmi\n",
		},
		{
			name:           "if a directory does not contain an index, a listing is returned",
			url:            "/b/",
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

func TestFileSystemBinaryHandling(t *testing.T) {
	var tests = []struct {
		name           string
		url            string
		expectedHeader Header
		expectedHash   string
	}{
		{
			name: "mp3 files are served as expected",
			url:  "cordova.mp3",
			expectedHeader: Header{
				Code: CodeSuccess,
				Meta: "audio/mpeg",
			},
			expectedHash: "9f3bd09333627c218b3b0af371d9c7f4c6ac6a8b1c60b9e9aba0ca68c602e511",
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
			hash := sha256.New()
			_, err = io.Copy(hash, resp.Body)
			if err != nil {
				t.Fatalf("unexpected error reading body: %v", err)
			}
			actualHash := hex.EncodeToString(hash.Sum(nil))
			if tt.expectedHash != actualHash {
				t.Errorf("expected\n%v\nactual\n%v", tt.expectedHash, string(actualHash))
			}
		})
	}
}
