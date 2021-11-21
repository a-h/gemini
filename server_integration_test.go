package gemini

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"testing"
	"time"
)

func TestServerIntegration(t *testing.T) {
	if testing.Short() {
		return
	}
	// Start local server to listen on a-h.gemini
	h := HandlerFunc(func(w ResponseWriter, r *Request) {
		io.WriteString(w, "# Hello")
	})

	cf := "./example/server/a.crt"
	kf := "./example/server/a.key"
	cert, err := tls.LoadX509KeyPair(cf, kf)
	if err != nil {
		t.Fatalf("failed to load test certs: %v", err)
	}

	domain := "a-h.gEmInI"
	dh := NewDomainHandler(domain, cert, h)
	ctx := context.Background()
	domainToHandler := map[string]*DomainHandler{
		domain: dh,
	}
	server := NewServer(ctx, "localhost:1965", domainToHandler)
	go func() {
		err = server.ListenAndServe()
		if err != nil {
			fmt.Printf("error starting server: %v\n", err)
			return
		}
	}()

	// Wait for the server to start up.
	time.Sleep(time.Second)

	// Use client.
	// Check that variations of case are handled.
	c := NewClient()
	c.AddServerCertificate("a-h.gEmInI", "MIIBbDCB8wIJANTHyZB1GOthMAoGCCqGSM49BAMCMCAxCzAJBgNVBAYTAmdiMREwDwYDVQQDDAhhLmdlbWluaTAeFw0yMDA4MjAxOTAzMDNaFw0zMDA4MTgxOTAzMDNaMCAxCzAJBgNVBAYTAmdiMREwDwYDVQQDDAhhLmdlbWluaTB2MBAGByqGSM49AgEGBSuBBAAiA2IABK5cq+AfcI2PlCNyXfSWAeGgM6G1Hrc806iphTARNGEny/7bV8S9FK1gAMyy90jTKyorgXsYYHgdk352ZmgIdIdvtKmpHETiz4yYBNQPbnEi9skqGIS2K9nwdJzKThLPqDAKBggqhkjOPQQDAgNoADBlAjEArkR+uUVenKHwLwEzkNLEApp/KXMs9uKXh7U7ZDWQTWIvR/Ox+//mCihNvUzd1u9YAjBRjcsDVdXD2IA1cSiXLGMMqQqRXx60F6fqDkUYpy38inbJtQxR1W9qaDXE36mJtyvjsMRCmPwcFJr79MiZb7kkJ65B5GSbk0yklZkbeFK4VQ==")
	testURIs := []string{
		"gemini://a-h.gemini",
		"gemini://a-h.gEmInI",
		"gemini://a-h.GEMINI",
	}
	for _, uri := range testURIs {
		t.Run(fmt.Sprintf("%v", uri), func(t *testing.T) {
			resp, certs, _, ok, err := c.Request(context.Background(), uri)
			if err != nil {
				t.Fatalf("request failed: %v", err)
				return
			}
			if !ok {
				t.Errorf("response not OK: %v, certs: %v", resp, certs)
			}
		})
	}
}
