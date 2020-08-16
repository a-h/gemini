package gemini

import (
	"bytes"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"
)

func TestServer(t *testing.T) {
	var tests = []struct {
		name              string
		request           string
		handler           func(w ResponseWriter, r *Request)
		expectedCode      Code
		expectedMeta      string
		expectedHeaderErr error
		expectedBody      []byte
		expectedBodyErr   error
	}{
		{
			"invalid request URLs return a CodeBadRequest",
			"tab	is	invalid\r\n",
			func(w ResponseWriter, r *Request) {
			},
			CodeBadRequest,
			"request malformed",
			nil,
			[]byte{},
			nil,
		},
		{
			"very long requests return a CodeBadRequest",
			longString("a", 2048) + "\r\n",
			func(w ResponseWriter, r *Request) {
			},
			CodeBadRequest,
			"request too long or malformed",
			nil,
			[]byte{},
			nil,
		},
		{
			"successful handlers are sent",
			"gemini://sensible\r\n",
			func(w ResponseWriter, r *Request) {
				w.SetHeader(CodeInput, "What's your name?")
			},
			CodeInput,
			"What's your name?",
			nil,
			[]byte{},
			nil,
		},
		{
			"the header can only be set once",
			"gemini://sensible\r\n",
			func(w ResponseWriter, r *Request) {
				w.SetHeader(CodeInput, "What's your name?")
				w.SetHeader(CodeClientCertificateRequired, "nope")
			},
			CodeInput,
			"What's your name?",
			nil,
			[]byte{},
			nil,
		},
		{
			"a body can be sent if the code is success",
			"gemini://sensible\r\n",
			func(w ResponseWriter, r *Request) {
				w.SetHeader(CodeSuccess, "application/json")
				w.Write([]byte(`{ "key": "value" }`))
			},
			CodeSuccess,
			"application/json",
			nil,
			[]byte(`{ "key": "value" }`),
			nil,
		},
		{
			"the default header is set if one isn't provided",
			"gemini://sensible\r\n",
			func(w ResponseWriter, r *Request) {
				w.Write([]byte("# Hello World!"))
			},
			CodeSuccess,
			"text/gemini; charset=utf-8",
			nil,
			[]byte("# Hello World!"),
			nil,
		},
		{
			"a body isn't written if the code is not success",
			"gemini://sensible\r\n",
			func(w ResponseWriter, r *Request) {
				w.SetHeader(CodeCGIError, "oops")
				w.Write([]byte("# Hello World!"))
			},
			CodeCGIError,
			"oops",
			nil,
			[]byte{},
			nil,
		},
		{
			"metadata is truncated down to the max size",
			"gemini://sensible\r\n",
			func(w ResponseWriter, r *Request) {
				w.SetHeader(CodeCGIError, longString("a", 2048))
			},
			CodeCGIError,
			longString("a", 1024),
			nil,
			[]byte{},
			nil,
		},
		{
			"handlers receive the URL",
			"gemini://sensible\r\n",
			func(w ResponseWriter, r *Request) {
				if r.URL.String() != "gemini://sensible" {
					t.Errorf("expected url, got: %v", r.URL.String())
				}
				w.Write([]byte("OK"))
			},
			CodeSuccess,
			"text/gemini; charset=utf-8",
			nil,
			[]byte("OK"),
			nil,
		},
		{
			"handlers that forget a response are given a default",
			"gemini://sensible\r\n",
			func(w ResponseWriter, r *Request) {
				// Do nothing.
			},
			CodeCGIError,
			"empty response",
			nil,
			[]byte{},
			nil,
		},
		{
			"panics in handlers return a GCI error",
			"gemini://sensible\r\n",
			func(w ResponseWriter, r *Request) {
				panic("oops")
			},
			CodeCGIError,
			"internal error",
			nil,
			[]byte{},
			nil,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			rec := NewRecorder([]byte(tt.request))
			// Skip the usual setup, because this test doesn't carry out integration work.
			s := &Server{
				Handler: HandlerFunc(tt.handler),
			}
			s.handle(rec)

			response := &Response{
				r: bytes.NewBuffer(rec.written.Bytes()),
			}

			actualCode, actualMeta, err := response.Header()
			if err != tt.expectedHeaderErr {
				t.Errorf("expected header err: %v, got: %v", tt.expectedHeaderErr, err)
			}
			if actualCode != tt.expectedCode {
				t.Errorf("expected code: %q, got: %q", tt.expectedCode, actualCode)
			}
			if actualMeta != tt.expectedMeta {
				t.Errorf("expected meta: %q, got %q", tt.expectedMeta, actualMeta)
			}

			actualBody, err := ioutil.ReadAll(response.Body())
			if err != tt.expectedBodyErr {
				t.Errorf("expected body err: %v, got: %v", tt.expectedBodyErr, err)
			}
			if reflect.DeepEqual(actualBody, tt.expectedBody) != true {
				t.Errorf("expected body: %q, got %q", string(tt.expectedBody), string(actualBody))
			}
		})
	}
}

func longString(of string, count int) string {
	var sb strings.Builder
	for i := 0; i < count; i++ {
		sb.WriteString(of)
	}
	return sb.String()
}

func NewRecorder(request []byte) *Recorder {
	return &Recorder{
		request: bytes.NewBuffer(request),
		written: new(bytes.Buffer),
	}
}

type Recorder struct {
	request *bytes.Buffer
	read    int
	written *bytes.Buffer
}

func (rec *Recorder) Write(p []byte) (n int, err error) {
	return rec.written.Write(p)
}

func (rec *Recorder) Read(p []byte) (n int, err error) {
	n, err = rec.request.Read(p)
	rec.read += n
	return n, err
}
