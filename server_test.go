package gemini

import (
	"bytes"
	"context"
	"io/ioutil"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	var tests = []struct {
		name              string
		request           string
		cert              Certificate
		handler           func(w ResponseWriter, r *Request)
		expectedCode      Code
		expectedMeta      string
		expectedHeaderErr error
		expectedBody      []byte
		expectedBodyErr   error
	}{
		{
			name: "invalid request URLs return a CodeBadRequest",
			request: "tab	is	invalid\r\n",
			cert: Certificate{},
			handler: func(w ResponseWriter, r *Request) {
			},
			expectedCode:      CodeBadRequest,
			expectedMeta:      "request malformed",
			expectedHeaderErr: nil,
			expectedBody:      []byte{},
			expectedBodyErr:   nil,
		},
		{
			name:    "very long requests return a CodeBadRequest",
			request: longString("a", 2048) + "\r\n",
			handler: func(w ResponseWriter, r *Request) {
			},
			expectedCode:      CodeBadRequest,
			expectedMeta:      "request too long or malformed",
			expectedHeaderErr: nil,
			expectedBody:      []byte{},
			expectedBodyErr:   nil,
		},
		{
			name:    "successful handlers are sent",
			request: "gemini://sensible\r\n",
			handler: func(w ResponseWriter, r *Request) {
				w.SetHeader(CodeInput, "What's your name?")
			},
			expectedCode:      CodeInput,
			expectedMeta:      "What's your name?",
			expectedHeaderErr: nil,
			expectedBody:      []byte{},
			expectedBodyErr:   nil,
		},
		{
			name:    "the header can only be set once",
			request: "gemini://sensible\r\n",
			handler: func(w ResponseWriter, r *Request) {
				w.SetHeader(CodeInput, "What's your name?")
				w.SetHeader(CodeClientCertificateRequired, "nope")
			},
			expectedCode:      CodeInput,
			expectedMeta:      "What's your name?",
			expectedHeaderErr: nil,
			expectedBody:      []byte{},
			expectedBodyErr:   nil,
		},
		{
			name:    "a body can be sent if the code is success",
			request: "gemini://sensible\r\n",
			handler: func(w ResponseWriter, r *Request) {
				w.SetHeader(CodeSuccess, "application/json")
				w.Write([]byte(`{ "key": "value" }`))
			},
			expectedCode:      CodeSuccess,
			expectedMeta:      "application/json",
			expectedHeaderErr: nil,
			expectedBody:      []byte(`{ "key": "value" }`),
			expectedBodyErr:   nil,
		},
		{
			name:    "the default header is set if one isn't provided",
			request: "gemini://sensible\r\n",
			handler: func(w ResponseWriter, r *Request) {
				w.Write([]byte("# Hello World!"))
			},
			expectedCode:      CodeSuccess,
			expectedMeta:      DefaultMIMEType,
			expectedHeaderErr: nil,
			expectedBody:      []byte("# Hello World!"),
			expectedBodyErr:   nil,
		},
		{
			name:    "a body isn't written if the code is not success",
			request: "gemini://sensible\r\n",
			handler: func(w ResponseWriter, r *Request) {
				w.SetHeader(CodeCGIError, "oops")
				w.Write([]byte("# Hello World!"))
			},
			expectedCode:      CodeCGIError,
			expectedMeta:      "oops",
			expectedHeaderErr: nil,
			expectedBody:      []byte{},
			expectedBodyErr:   nil,
		},
		{
			name:    "metadata is truncated down to the max size",
			request: "gemini://sensible\r\n",
			handler: func(w ResponseWriter, r *Request) {
				w.SetHeader(CodeCGIError, longString("a", 2048))
			},
			expectedCode:      CodeCGIError,
			expectedMeta:      longString("a", 1024),
			expectedHeaderErr: nil,
			expectedBody:      []byte{},
			expectedBodyErr:   nil,
		},
		{
			name:    "handlers receive the URL",
			request: "gemini://sensible\r\n",
			handler: func(w ResponseWriter, r *Request) {
				if r.URL.String() != "gemini://sensible" {
					t.Errorf("expected url, got: %v", r.URL.String())
				}
				w.Write([]byte("OK"))
			},
			expectedCode:      CodeSuccess,
			expectedMeta:      DefaultMIMEType,
			expectedHeaderErr: nil,
			expectedBody:      []byte("OK"),
			expectedBodyErr:   nil,
		},
		{
			name:    "handlers that forget a response are given a default",
			request: "gemini://sensible\r\n",
			handler: func(w ResponseWriter, r *Request) {
				// Do nothing.
			},
			expectedCode:      CodeCGIError,
			expectedMeta:      "empty response",
			expectedHeaderErr: nil,
			expectedBody:      []byte{},
			expectedBodyErr:   nil,
		},
		{
			name:    "panics in handlers return a GCI error",
			request: "gemini://sensible\r\n",
			handler: func(w ResponseWriter, r *Request) {
				panic("oops")
			},
			expectedCode:      CodeCGIError,
			expectedMeta:      "internal error",
			expectedHeaderErr: nil,
			expectedBody:      []byte{},
			expectedBodyErr:   nil,
		},
		{
			name:    "invalid certificates result in a code 62 (certificate not valid response)",
			request: "gemini://sensible\r\n",
			cert: Certificate{
				Error: "certificate failure",
			},
			handler: func(w ResponseWriter, r *Request) {
				w.Write([]byte("Hello"))
			},
			expectedCode:      CodeClientCertificateNotValid,
			expectedMeta:      "certificate failure",
			expectedHeaderErr: nil,
			expectedBody:      []byte{},
			expectedBodyErr:   nil,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			rec := NewRecorder([]byte(tt.request))
			// Skip the usual setup, because this test doesn't carry out integration work.
			dh := &DomainHandler{
				ServerName: "",
				Handler:    HandlerFunc(tt.handler),
			}
			s := &Server{
				DomainToHandler: map[string]*DomainHandler{
					"": dh,
				},
				Context: context.Background(),
			}
			s.handle(dh, tt.cert, rec)

			response, err := NewResponse(ioutil.NopCloser(bytes.NewBuffer(rec.written.Bytes())))
			if err != tt.expectedHeaderErr {
				t.Errorf("expected header err: %v, got: %v", tt.expectedHeaderErr, err)
			}
			if response.Header.Code != tt.expectedCode {
				t.Errorf("expected code: %q, got: %q", tt.expectedCode, response.Header.Code)
			}
			if response.Header.Meta != tt.expectedMeta {
				t.Errorf("expected meta: %q, got %q", tt.expectedMeta, response.Header.Meta)
			}

			actualBody, err := ioutil.ReadAll(response.Body)
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

func (rec *Recorder) Close() error {
	return nil
}

func (rec *Recorder) LocalAddr() net.Addr {
	return &net.IPAddr{}
}

func (rec *Recorder) RemoteAddr() net.Addr {
	return &net.IPAddr{}
}

func (rec *Recorder) SetDeadline(t time.Time) error {
	return nil
}

func (rec *Recorder) SetReadDeadline(t time.Time) error {
	return nil
}

func (rec *Recorder) SetWriteDeadline(t time.Time) error {
	return nil
}
