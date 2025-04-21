package gemini

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/a-h/gemini/log"
)

// Handler of Gemini content.
type Handler interface {
	ServeGemini(w ResponseWriter, r *Request)
}

// HandlerFunc handles a Gemini request and returns a response.
type HandlerFunc func(ResponseWriter, *Request)

// ServeGemini implements the Handler interface.
func (f HandlerFunc) ServeGemini(w ResponseWriter, r *Request) {
	f(w, r)
}

// DefaultMIMEType for Gemini responses.
const DefaultMIMEType = "text/gemini; charset=utf-8"

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
	// ID is the base64-encoded SHA256 hash of the key.
	ID string
	// Key is the user public key in PKIX, ASN.1 DER form.
	Key string
	// Error is an error message related to any failures in handling the client certificate.
	Error string
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

// IsErrorCode returns true if the code is invalid, or starts with 4, 5 or 6.
func IsErrorCode(code Code) bool {
	if !isValidCode(code) || len(code) != 2 {
		return false
	}
	return code[0] == '4' || code[0] == '5' || code[0] == '6'
}

// NewServer creates a new Gemini server.
// addr is in the form "<optional_ip>:<port>", e.g. ":1965". If left empty, it will default to ":1965".
// domainToHandler is a map of the server name (domain) to the certificate key pair and the Gemini handler used to serve content.
func NewServer(ctx context.Context, addr string, domainToHandler map[string]*DomainHandler) *Server {
	for k, v := range domainToHandler {
		domainToHandler[strings.ToLower(k)] = v
	}
	return &Server{
		Context:         ctx,
		Addr:            addr,
		DomainToHandler: domainToHandler,
		ReadTimeout:     time.Second * 5,
		WriteTimeout:    time.Second * 10,
		HandlerTimeout:  time.Second * 30,
	}
}

// Server hosts Gemini content.
type Server struct {
	Context         context.Context
	Addr            string
	Insecure        bool
	DomainToHandler map[string]*DomainHandler
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	HandlerTimeout  time.Duration
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
	log.Info("gemini: starting", log.String("addr", addr))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	if srv.Insecure {
		err = srv.serveInsecure(ln)
		if err != nil {
			log.Error("gemini: serveInsecure failure", err, log.String("addr", addr))
		}
	} else {
		err = srv.serveTLS(ln)
		if err != nil {
			log.Error("gemini: serveTLS failure", err, log.String("addr", addr))
		}
	}
	log.Info("gemini: stopped")
	return err
}

// ErrServerClosed is returned when a server is attempted to start up when it's already shutting down.
var ErrServerClosed = errors.New("gemini: server closed")

func (srv *Server) serveInsecure(l net.Listener) (err error) {
	if len(srv.DomainToHandler) > 1 {
		return fmt.Errorf("gemini: cannot start insecure mode for more than one domain")
	}
	var handler *DomainHandler
	for _, handler = range srv.DomainToHandler {
		break
	}
	for {
		if err = srv.Context.Err(); err != nil {
			log.Error("gemini: context caused shutdown", err)
			return err
		}
		rw, err := l.Accept()
		if err != nil {
			log.Error("gemini: insecure listener error", err)
			continue
		}
		go func() {
			defer rw.Close()
			srv.handle(handler, Certificate{}, rw)
		}()
	}
}

func (srv *Server) serveTLS(l net.Listener) (err error) {
	config := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ClientAuth:         tls.RequestClientCert,
		InsecureSkipVerify: true,
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			dh, ok := srv.DomainToHandler[strings.ToLower(hello.ServerName)]
			if !ok {
				return nil, fmt.Errorf("gemini: certificate not found for %q", hello.ServerName)
			}
			return &dh.KeyPair, nil
		},
	}
	if err != nil {
		return err
	}
	tlsListener := tls.NewListener(l, config)
	for {
		if err = srv.Context.Err(); err != nil {
			log.Error("gemini: context caused shutdown", err)
			return err
		}
		conn, err := tlsListener.Accept()
		if err != nil {
			log.Error("gemini: tls listener error", err)
			continue
		}
		tlsConn, ok := conn.(*tls.Conn)
		if !ok {
			panic("gemini: tls.Listener did not return TLS connection")
		}
		go srv.handleTLS(tlsConn)
	}
}

func (srv *Server) handleTLS(conn *tls.Conn) {
	defer conn.Close()
	if err := conn.Handshake(); err != nil {
		log.Info("gemini: failed TLS handshake", log.String("remote", conn.RemoteAddr().String()), log.String("reason", err.Error()))
		return
	}
	var certificate Certificate
	peerCerts := conn.ConnectionState().PeerCertificates
	if len(peerCerts) > 0 {
		now := time.Now()
		cert := peerCerts[0]
		certHash := sha256.Sum256(cert.Raw)
		certificate.ID = base64.StdEncoding.EncodeToString(certHash[:])
		certificate.Key = string(cert.Raw)
		if now.Before(cert.NotBefore) {
			certificate.Error = "certificate not yet valid"
		}
		if now.After(cert.NotAfter) {
			certificate.Error = "certificate expired"
		}
	}
	serverName := conn.ConnectionState().ServerName
	dh, ok := srv.DomainToHandler[strings.ToLower(serverName)]
	if !ok {
		log.Warn("gemini: failed to find domain handler", log.String("serverName", serverName))
	}
	srv.handle(dh, certificate, conn)
}

// while this function could be inlined, exposing it makes it easier to test in isolation.
func (srv *Server) handle(dh *DomainHandler, certificate Certificate, conn net.Conn) {
	start := time.Now()
	conn.SetReadDeadline(time.Now().Add(srv.ReadTimeout))
	r, ok, err := srv.parseRequest(conn)
	if err != nil {
		log.Info("gemini: failed to parse request", log.String("reason", err.Error()))
		return
	}
	if !ok {
		return
	}
	r.Certificate = certificate
	ctx, cancel := context.WithTimeout(srv.Context, srv.HandlerTimeout)
	defer cancel()
	r.Context = ctx
	conn.SetWriteDeadline(time.Now().Add(srv.WriteTimeout))
	w := NewWriter(conn)
	defer func() {
		if p := recover(); p != nil {
			log.Error("gemini: server error", nil, log.String("url", r.URL.String()), log.Interface("recover", p))
			w.SetHeader(CodeCGIError, "internal error")
		}
	}()
	if certificate.Error != "" {
		w.SetHeader(CodeClientCertificateNotValid, certificate.Error)
		return
	}
	dh.Handler.ServeGemini(w, r)
	if w.Code == "" {
		log.Error("gemini: handler resulted in empty response", nil, log.String("url", r.URL.String()), log.String("handlerType", reflect.TypeOf(dh.Handler).PkgPath()))
		w.SetHeader(CodeCGIError, "empty response")
	}
	duration := time.Now().Sub(start)
	log.Info("gemini: response",
		log.String("url", r.URL.String()),
		log.String("path", r.URL.Path),
		log.String("code", w.Code),
		log.String("handlerType", reflect.TypeOf(dh.Handler).PkgPath()),
		log.Int64("ms", duration.Milliseconds()),
		log.Int64("us", int64(duration.Microseconds())),
		log.Int64("lenBody", w.WrittenBody),
		log.Int("lenHeader", w.WrittenHeader),
		log.Int64("len", int64(w.WrittenHeader)+w.WrittenBody),
	)
}

func (srv *Server) parseRequest(rw io.ReadWriter) (r *Request, ok bool, err error) {
	request, ok, err := readUntilCrLf(rw, 1026)
	if err != nil && err != io.EOF {
		writeHeaderToWriter(CodeBadRequest, fmt.Sprintf("error reading request: %v", err), rw)
		return
	}
	if !ok {
		log.Info("gemini: request too long or malformed", log.String("request", string(request)))
		writeHeaderToWriter(CodeBadRequest, "request too long or malformed", rw)
		return
	}
	ok = false
	url, err := url.Parse(strings.TrimSpace(string(request)))
	if err != nil {
		log.Info("gemini: malformed request", log.String("request", string(request)))
		writeHeaderToWriter(CodeBadRequest, "request malformed", rw)
		return
	}
	log.Info("gemini: received request", log.String("request", url.String()))
	r = &Request{
		URL: url,
	}
	return r, true, err
}

// Writer passed to Gemini handlers.
type Writer struct {
	Code          string
	Writer        io.Writer
	WrittenHeader int
	WrittenBody   int64
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
		gw.SetHeader(CodeSuccess, DefaultMIMEType)
		gw.Code = CodeSuccess
	}
	if !isSuccessCode(Code(gw.Code)) {
		err = ErrCannotWriteBodyWithoutSuccessCode
		return
	}
	n, err = gw.Writer.Write(p)
	gw.WrittenBody += int64(n)
	return
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
	var n int
	n, err = writeHeaderToWriter(code, meta, gw.Writer)
	gw.WrittenHeader += n
	return
}

func writeHeaderToWriter(code Code, meta string, w io.Writer) (n int, err error) {
	// <STATUS><SPACE><META><CR><LF>
	// Set default meta if required.
	if meta == "" && isSuccessCode(code) {
		meta = DefaultMIMEType
	}
	if len(meta) > 1024 {
		meta = meta[:1024]
	}
	return w.Write([]byte(string(code) + " " + meta + "\r\n"))
}

// DomainHandler handles incoming requests for the ServerName using the provided KeyPair certificate
// and Handler to process the request.
type DomainHandler struct {
	ServerName string
	KeyPair    tls.Certificate
	Handler    Handler
}

// NewDomainHandler creates a new handler to listen for Gemini requests using TLS.
// The cert can be generated by the github.com/a-h/gemini/cert.Generate package,
// or can generated using openssl:
// keyFile:
//
//	openssl ecparam -genkey -name secp384r1 -out server.key
//
// certFile:
//
//	openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650
func NewDomainHandler(serverName string, cert tls.Certificate, handler Handler) *DomainHandler {
	return &DomainHandler{
		ServerName: serverName,
		KeyPair:    cert,
		Handler:    handler,
	}
}

// NewDomainHandlerFromFiles creates a new handler to listen for Gemini requests using TLS.
// certFile / keyFile are links to the X509 keypair. This can be generated using openssl:
// keyFile:
//
//	openssl ecparam -genkey -name secp384r1 -out server.key
//
// certFile:
//
//	openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650
func NewDomainHandlerFromFiles(serverName, certFile, keyFile string, handler Handler) (*DomainHandler, error) {
	keyPair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return NewDomainHandler(serverName, keyPair, handler), nil
}

// ListenAndServe starts up a new server to handle multiple domains with a specific certFile, keyFile and handler.
func ListenAndServe(ctx context.Context, addr string, domains ...*DomainHandler) (err error) {
	if len(domains) == 0 {
		return fmt.Errorf("gemini: no default handler provided")
	}
	domainToHandler := make(map[string]*DomainHandler, len(domains))
	for i := 0; i < len(domains); i++ {
		domainToHandler[domains[i].ServerName] = domains[i]
	}
	server := NewServer(ctx, addr, domainToHandler)
	return server.ListenAndServe()
}
