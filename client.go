package gemini

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"strings"
	"time"
)

// Response from the Gemini server.
type Response struct {
	Header *Header
	Body   io.ReadCloser
}

// NewResponse parses the server response.
func NewResponse(r io.ReadCloser) (resp *Response, err error) {
	resp = &Response{
		Body: r,
	}
	h, err := readHeader(r)
	resp.Header = &h
	return
}

type Header struct {
	Code Code
	Meta string
}

// ErrInvalidStatus is returned if the Gemini request did not match the expected format.
var ErrInvalidStatus = errors.New("gemini: server status did not match the expected format")

// ErrInvalidCode is returned if the Gemini server returns an invalid code.
var ErrInvalidCode = errors.New("gemini: invalid code")

// ErrInvalidMeta is returned if the Gemini server returns an invalid meta value.
var ErrInvalidMeta = errors.New("gemini: invalid meta")

// ErrCrLfNotFoundWithinMaxLength is returned if the Gemini server returns an invalid response.
var ErrCrLfNotFoundWithinMaxLength = errors.New("gemini: invalid header - CRLF not found within maximum length")

func readHeader(r io.Reader) (h Header, err error) {
	// Read <STATUS><SPACE><META><CR><LF>
	statusLine, ok, err := readUntilCrLf(r, 1029)
	if err != nil {
		err = fmt.Errorf("gemini: failed to read status line: %v", err)
		return
	}
	if !ok {
		err = ErrCrLfNotFoundWithinMaxLength
		return
	}
	parts := strings.SplitN(strings.TrimRight(string(statusLine), "\r\n"), " ", 2)
	if len(parts) != 1 && len(parts) != 2 {
		err = ErrInvalidStatus
		return
	}
	h.Code = Code(parts[0])
	if !isValidCode(h.Code) {
		err = ErrInvalidCode
		return
	}
	if len(parts) > 1 {
		h.Meta = parts[1]
		if !isValidMeta(h.Meta) {
			err = ErrInvalidMeta
			return
		}
	}
	return
}

func readUntilCrLf(src io.Reader, maxLength int) (output []byte, ok bool, err error) {
	var previousIsCr bool
	buffer := make([]byte, 1)
	for i := 0; i < maxLength; i++ {
		_, err = src.Read(buffer)
		if err != nil {
			return
		}
		current := buffer[0]
		if current == '\n' {
			if previousIsCr {
				ok = true
				return
			}
		}
		previousIsCr = current == '\r'
		output = append(output, buffer[0])
	}
	return
}

var validStart map[byte]bool = map[byte]bool{
	'1': true,
	'2': true,
	'3': true,
	'4': true,
	'5': true,
	'6': true,
}

func isValidCode(code Code) bool {
	if len(code) == 0 {
		return false
	}
	return validStart[code[0]]
}

func isValidMeta(m string) bool {
	return len(m) <= 1024
}

// NewClient creates a new gemini client.
func NewClient() *Client {
	return &Client{
		prefixToCertificate:            make(map[string]tls.Certificate),
		domainToAllowedCertificateHash: make(map[string]map[string]interface{}),
		WriteTimeout:                   time.Second * 5,
		ReadTimeout:                    time.Second * 5,
	}
}

// Client for Gemini requests.
type Client struct {
	// prefixToCertificate maps URL prefixes to certificates.
	// Load a keypair from disk with tls.LoadX509KeyPair("client.pem", "client.key")
	// If a certificate is not required, use &Client{}.
	prefixToCertificate map[string]tls.Certificate
	// domainToAllowedCertificateHash is used to validate the remote server.
	domainToAllowedCertificateHash map[string]map[string]interface{}
	// Insecure mode does not check the hash of remote certificates.
	Insecure     bool
	WriteTimeout time.Duration
	ReadTimeout  time.Duration
}

// AddClientCertificate adds a certificate to use when the URL prefix is encountered.
func (client *Client) AddClientCertificate(prefix string, cert tls.Certificate) {
	client.prefixToCertificate[prefix] = cert
}

// AddServerCertificate allows the client to connect to a domain based on its hash.
func (client *Client) AddServerCertificate(host, certificateHash string) {
	host = strings.ToLower(host)
	if m := client.domainToAllowedCertificateHash[host]; m == nil {
		client.domainToAllowedCertificateHash[host] = make(map[string]interface{})
	}
	client.domainToAllowedCertificateHash[host][certificateHash] = struct{}{}
}

// Request a response from a given Gemini URL.
func (client *Client) Request(ctx context.Context, u string) (resp *Response, certificates []string, authenticated, ok bool, err error) {
	uu, err := url.Parse(u)
	if err != nil {
		return
	}
	return client.RequestURL(ctx, uu)
}

// GetCertificate returns a certificate to use for the given URL, if one exists.
func (client *Client) GetCertificate(u *url.URL) (cert tls.Certificate, ok bool) {
	for k, v := range client.prefixToCertificate {
		if strings.HasPrefix(u.String(), k) {
			cert = v
			ok = true
			return
		}
	}
	return
}

// RequestNoTLS carries out a request without TLS enabled.
func (client *Client) RequestNoTLS(ctx context.Context, u *url.URL) (resp *Response, err error) {
	dialer := net.Dialer{
		Timeout: client.ReadTimeout,
	}
	port := u.Port()
	if port == "" {
		port = "1965"
	}
	conn, err := dialer.DialContext(ctx, "tcp", u.Hostname()+":"+port)
	if err != nil {
		err = fmt.Errorf("gemini: error connecting: %w", err)
		return
	}
	return client.RequestConn(ctx, conn, u)
}

// RequestURL requests a response from a parsed URL.
// ok returns true if a matching server certificate is found (i.e. the server is OK).
func (client *Client) RequestURL(ctx context.Context, u *url.URL) (resp *Response, certificates []string, authenticated, ok bool, err error) {
	tlsDialer := tls.Dialer{
		NetDialer: &net.Dialer{
			Timeout: client.ReadTimeout,
		},
		Config: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	if cert, ok := client.GetCertificate(u); ok {
		tlsDialer.Config.Certificates = []tls.Certificate{cert}
	}
	port := u.Port()
	if port == "" {
		port = "1965"
	}
	cn, err := tlsDialer.DialContext(ctx, "tcp", u.Hostname()+":"+port)
	if err != nil {
		err = fmt.Errorf("gemini: error connecting: %w", err)
		return
	}
	conn := cn.(*tls.Conn)
	allowedHashesForDomain := client.domainToAllowedCertificateHash[strings.ToLower(u.Host)]
	ok = false
	for _, cert := range conn.ConnectionState().PeerCertificates {
		hash := base64.StdEncoding.EncodeToString(sha256.New().Sum(cert.Raw))
		certificates = append(certificates, hash)
		if _, ok = allowedHashesForDomain[hash]; ok {
			break
		}
		if time.Now().Before(cert.NotBefore) {
			err = fmt.Errorf("gemini: expired certificate")
			return
		}
		if time.Now().After(cert.NotAfter) {
			err = fmt.Errorf("gemini: certificate not yet valid")
			return
		}
	}
	if !ok && !client.Insecure {
		return
	}
	authenticated = conn.ConnectionState().NegotiatedProtocolIsMutual
	resp, err = client.RequestConn(ctx, conn, u)
	return
}

type readerCtx struct {
	ctx context.Context
	r   io.ReadCloser
}

func (r *readerCtx) Read(p []byte) (n int, err error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func (r *readerCtx) Close() (err error) {
	return r.r.Close()
}

func newReaderContext(ctx context.Context, r io.ReadCloser) io.ReadCloser {
	return &readerCtx{
		ctx: ctx,
		r:   r,
	}
}

// RequestConn uses a given connection to make the request. This allows for insecure requests to be made.
// net.Dial("tcp", "localhost:1965")
func (client *Client) RequestConn(ctx context.Context, conn net.Conn, u *url.URL) (resp *Response, err error) {
	conn.SetWriteDeadline(time.Now().Add(client.WriteTimeout))
	_, err = conn.Write([]byte(u.String() + "\r\n"))
	if err != nil {
		err = fmt.Errorf("gemini: error writing request: %w", err)
		return
	}
	conn.SetReadDeadline(time.Now().Add(client.ReadTimeout))
	resp, err = NewResponse(newReaderContext(ctx, conn))
	return
}

// Record a Gemini handler request in memory and return the response.
func Record(r *Request, handler Handler) (resp *Response, err error) {
	buf := new(bytes.Buffer)
	w := NewWriter(buf)
	handler.ServeGemini(w, r)
	return NewResponse(ioutil.NopCloser(bytes.NewBuffer(buf.Bytes())))
}
