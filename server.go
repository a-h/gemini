package gemini

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
	"time"
)

// Handler of Gemini content.
type Handler interface {
	ServeGemini(w ResponseWriter, r *Request)
}

// HandlerFunc handles a Gemini request and returns a response.
type HandlerFunc func(ResponseWriter, *Request)

// ServeGeServeGemini implements the Handler interface.
func (f HandlerFunc) ServeGemini(w ResponseWriter, r *Request) {
	f(w, r)
}

// Request from the client. A Gemini request contains only the
// URL, the Certificates field is populated by the TLS certificates
// presented by the client.
type Request struct {
	Context     context.Context
	URL         *url.URL
	Certificate Certificate
}

// Certificate information provided to the server by the client.
type Certificate struct {
	// ID is the hex-encoded SHA256 hash of the key.
	ID string
	// Key is the user public key in PKIX, ASN.1 DER form.
	Key string
}

// ResponseWriter used by handlers to send a response to the client.
type ResponseWriter interface {
	io.Writer
	SetHeader(code Code, meta string) error
}

// Code returned as part of the Gemini response (see https://gemini.circumlunar.space/docs/specification.html).
type Code string

const (
	CodeInput                          Code = "10"
	CodeInputSensitive                      = "11"
	CodeSuccess                             = "20"
	CodeRedirect                            = "30"
	CodeRedirectTemporary                   = CodeRedirect
	CodeRedirectPermanent                   = "31"
	CodeTemporaryFailure                    = "40"
	CodeServerUnavailable                   = "41"
	CodeCGIError                            = "42"
	CodeProxyError                          = "43"
	CodeSlowDown                            = "44"
	CodePermanentFailure                    = "50"
	CodeNotFound                            = "51"
	CodeGone                                = "52"
	CodeProxyRequestRefused                 = "53"
	CodeBadRequest                          = "59"
	CodeClientCertificateRequired           = "60"
	CodeClientCertificateNotAuthorised      = "61"
	CodeClientCertificateNotValid           = "62"
)

// NewServer creates a new Gemini server.
// addr is in the form "<optional_ip>:<port>", e.g. ":1965". If left empty, it will default to ":1965".
// cert is the X509 keypair. This can be generated using openssl:
//   openssl ecparam -genkey -name secp384r1 -out server.key
//   openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650
// cert can be loaded using tls.LoadX509KeyPair(certFile, keyFile).
// handler is the Gemini handler used to serve content.
func NewServer(ctx context.Context, addr string, cert tls.Certificate, handler Handler) *Server {
	return &Server{
		Addr:         addr,
		Context:      ctx,
		Handler:      handler,
		KeyPair:      cert,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 30,
	}
}

// Server hosts Gemini content.
type Server struct {
	Addr         string
	Handler      Handler
	Context      context.Context
	KeyPair      tls.Certificate
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func (srv *Server) logf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// Set the server listening on the specified port.
func (srv *Server) ListenAndServe() error {
	// Don't start if the server is already closing down.
	if srv.Context.Err() != nil {
		return ErrServerClosed
	}
	addr := srv.Addr
	if addr == "" {
		addr = ":1965"
	}
	srv.logf("gemini: starting on %v", addr)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	err = srv.serveTLS(ln)
	srv.logf("gemini: stopped")
	return err
}

// ErrServerClosed is returned when a server is attempted to start up when it's already shutting down.
var ErrServerClosed = errors.New("gemini: server closed")

func (srv *Server) serveTLS(l net.Listener) (err error) {
	certificates := []tls.Certificate{srv.KeyPair}
	config := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		Certificates:       certificates,
		ClientAuth:         tls.RequestClientCert,
		InsecureSkipVerify: true,
	}
	if err != nil {
		return err
	}
	tlsListener := tls.NewListener(l, config)
	for {
		if err = srv.Context.Err(); err != nil {
			srv.logf("gemini: context caused shutdown: %v", err)
			return err
		}
		rw, err := tlsListener.Accept()
		if err != nil {
			srv.logf("gemini: tls listener error: %v", err)
			continue
		}
		go srv.handshakeAndHandle(rw)
	}
}

func (srv *Server) handshakeAndHandle(conn net.Conn) {
	defer conn.Close()
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		panic("gemini: serving unencrypted traffic is not permitted")
	}
	tlsConn.SetReadDeadline(time.Now().Add(srv.ReadTimeout))
	tlsConn.SetWriteDeadline(time.Now().Add(srv.WriteTimeout))
	if err := tlsConn.Handshake(); err != nil {
		srv.logf("gemini: failed TLS handshake from %s: %v", conn.RemoteAddr(), err)
		return
	}
	var certificate Certificate
	peerCerts := tlsConn.ConnectionState().PeerCertificates
	if len(peerCerts) > 0 {
		pk, err := x509.MarshalPKIXPublicKey(peerCerts[0])
		if err != nil {
			srv.logf("gemini: failed to marshal public key: %v", err)
			return
		}
		certificate.ID = hex.EncodeToString(sha256.New().Sum(pk))
		certificate.Key = string(pk)

	}
	srv.handle(certificate, tlsConn)
}

// while this function could be inlined, exposing it makes it easier to test in isolation.
func (srv *Server) handle(certificate Certificate, rw io.ReadWriter) {
	start := time.Now()
	r, ok, err := srv.parseRequest(rw)
	if err != nil {
		srv.logf("gemini: failed to parse request: %v", err)
		return
	}
	if !ok {
		srv.logf("gemini: could not parse request")
		return
	}
	r.Context = srv.Context
	r.Certificate = certificate
	w := NewWriter(rw)
	defer func() {
		if p := recover(); p != nil {
			srv.logf("gemini: server error: %v: %v", r.URL.Path, p)
			w.SetHeader(CodeCGIError, "internal error")
		}
	}()
	srv.Handler.ServeGemini(w, r)
	if w.Code == "" {
		srv.logf("gemini: handler for %q resulted in empty response, sending empty document", r.URL.Path)
		w.SetHeader(CodeCGIError, "empty response")
	}
	srv.logf("gemini: %v response: %v time: %v", r.URL.Path, w.Code, time.Now().Sub(start))
}

func (srv *Server) parseRequest(rw io.ReadWriter) (r *Request, ok bool, err error) {
	request, ok, err := readUntilCrLf(rw, 1026)
	if err != nil && err != io.EOF {
		writeHeaderToWriter(CodeBadRequest, fmt.Sprintf("error reading request: %v", err), rw)
		return
	}
	if !ok {
		writeHeaderToWriter(CodeBadRequest, "request too long or malformed", rw)
		return
	}
	ok = false
	url, err := url.Parse(strings.TrimSpace(string(request)))
	if err != nil {
		srv.logf("gemini: malformed request: %q", string(request))
		writeHeaderToWriter(CodeBadRequest, "request malformed", rw)
		return
	}
	srv.logf("gemini: received request: %s", url)
	r = &Request{
		URL: url,
	}
	return r, true, err
}

// Writer passed to Gemini handlers.
type Writer struct {
	Code   string
	Writer io.Writer
}

// NewWriter creates a new Gemini writer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		Writer: w,
	}
}

var ErrCannotWriteBodyWithoutSuccessCode = errors.New("gemini: cannot write body without success code")

func (gw *Writer) Write(p []byte) (n int, err error) {
	if gw.Code == "" {
		// Section 3.3
		gw.SetHeader(CodeSuccess, "")
		gw.Code = CodeSuccess
	}
	if !isSuccessCode(Code(gw.Code)) {
		err = ErrCannotWriteBodyWithoutSuccessCode
		return
	}
	return gw.Writer.Write(p)
}

func isSuccessCode(code Code) bool {
	return len(code) == 2 && code[0] == '2'
}

// ErrHeaderAlreadyWritten is returned by SetHeader when the Gemini header has already been written to the response.
var ErrHeaderAlreadyWritten = errors.New("gemini: header already written")

func (gw *Writer) SetHeader(code Code, meta string) (err error) {
	if gw.Code != "" {
		return ErrHeaderAlreadyWritten
	}
	gw.Code = string(code)
	return writeHeaderToWriter(code, meta, gw.Writer)
}

func writeHeaderToWriter(code Code, meta string, w io.Writer) error {
	// <STATUS><SPACE><META><CR><LF>
	// Set default meta if required.
	if meta == "" && isSuccessCode(code) {
		meta = "text/gemini; charset=utf-8"
	}
	if len(meta) > 1024 {
		meta = meta[:1024]
	}
	_, err := w.Write([]byte(string(code) + " " + meta + "\r\n"))
	return err
}

// ListenAndServe starts up a new server using the provided certFile, keyFile and handler.
func ListenAndServe(ctx context.Context, addr string, certFile, keyFile string, handler Handler) (err error) {
	keyPair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return
	}
	server := NewServer(ctx, addr, keyPair, handler)
	return server.ListenAndServe()
}
