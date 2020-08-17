package gemini

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

// Response from the Gemini server.
type Response struct {
	headerRead bool
	code       Code
	meta       string
	r          io.ReadCloser
}

// Body of the response.
func (r *Response) Body() io.ReadCloser {
	if !r.headerRead {
		r.Header()
	}
	return r.r
}

// ErrInvalidStatus is returned if the Gemini request did not match the expected format.
var ErrInvalidStatus = errors.New("gemini: server status did not match the expected format")

// ErrInvalidCode is returned if the Gemini server returns an invalid code.
var ErrInvalidCode = errors.New("gemini: invalid code")

// ErrInvalidMeta is returned if the Gemini server returns an invalid meta value.
var ErrInvalidMeta = errors.New("gemini: invalid meta")

// ErrCrLfNotFoundWithinMaxLength is returned if the Gemini server returns an invalid response.
var ErrCrLfNotFoundWithinMaxLength = errors.New("gemini: invalid header - CRLF not found within maximum length")

// Header of the Response.
func (r *Response) Header() (code Code, meta string, err error) {
	if r.headerRead {
		return r.code, r.meta, nil
	}
	// Read <STATUS><SPACE><META><CR><LF>
	statusLine, ok, err := readUntilCrLf(r.r, 1029)
	if err != nil {
		err = fmt.Errorf("gemini: failed to read status line: %v", err)
		return
	}
	if !ok {
		err = ErrCrLfNotFoundWithinMaxLength
		return
	}
	r.headerRead = true
	parts := strings.SplitN(strings.TrimSpace(string(statusLine)), " ", 2)
	if len(parts) != 2 {
		err = ErrInvalidStatus
		return
	}
	r.code = Code(parts[0])
	if !isValidCode(r.code) {
		err = ErrInvalidCode
		return
	}
	r.meta = parts[1]
	if !isValidMeta(r.meta) {
		err = ErrInvalidMeta
		return
	}
	return r.code, r.meta, err
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

func isValidCode(code Code) bool {
	return len(code) == 2 &&
		(code[0] >= 49 && code[0] <= 54) &&
		(code[1] >= 48 && code[1] <= 57)
}

func isValidMeta(m string) bool {
	return len(m) <= 1024
}

// NewClient creates a new gemini client, using the provided X509 keypair.
func NewClient() *Client {
	return &Client{
		prefixToCertificate:            make(map[string]tls.Certificate),
		domainToAllowedCertificateHash: make(map[string]map[string]interface{}),
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
}

// AddCertificateForURLPrefix adds the certificate when the URL prefix is encountered.
func (client *Client) AddCertificateForURLPrefix(prefix string, cert tls.Certificate) {
	client.prefixToCertificate[prefix] = cert
}

// AddAllowedCertificateForHost allows the client to connect to a domain based on its hash.
func (client *Client) AddAlllowedCertificateForHost(host, certificateHash string) {
	if m := client.domainToAllowedCertificateHash[host]; m == nil {
		client.domainToAllowedCertificateHash[host] = make(map[string]interface{})
	}
	client.domainToAllowedCertificateHash[host][certificateHash] = struct{}{}
}

// Request a response from a given Gemini URL.
func (client *Client) Request(u string) (resp *Response, certificates []string, authenticated, ok bool, err error) {
	uu, err := url.Parse(u)
	if err != nil {
		return
	}
	return client.RequestURL(uu)
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

// RequestURL requests a response from a parsed URL.
// ok returns true if a matching certificate is found.
func (client *Client) RequestURL(u *url.URL) (resp *Response, certificates []string, authenticated, ok bool, err error) {
	config := &tls.Config{
		InsecureSkipVerify: true,
	}
	if cert, ok := client.GetCertificate(u); ok {
		config.Certificates = []tls.Certificate{cert}
	}
	port := u.Port()
	if port == "" {
		port = "1965"
	}
	conn, err := tls.Dial("tcp", u.Host+":"+port, config)
	if err != nil {
		err = fmt.Errorf("gemini: error connecting: %w", err)
		return
	}
	allowedHashesForDomain := client.domainToAllowedCertificateHash[u.Host]
	ok = false
	for _, cert := range conn.ConnectionState().PeerCertificates {
		hash := hex.EncodeToString(sha256.New().Sum(cert.Raw))
		certificates = append(certificates, hash)
		if _, ok = allowedHashesForDomain[hash]; ok {
			ok = true
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
	if !ok {
		return
	}
	authenticated = conn.ConnectionState().NegotiatedProtocolIsMutual
	_, err = conn.Write([]byte(u.String() + "\r\n"))
	if err != nil {
		err = fmt.Errorf("gemini: error writing request: %w", err)
		return
	}
	resp = &Response{
		r: conn,
	}
	_, _, err = resp.Header()
	return
}
