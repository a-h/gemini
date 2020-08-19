package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/a-h/gemini"
)

func main() {
	client := gemini.NewClient()

	// Enable client authentication.
	clientCert, err := tls.LoadX509KeyPair("client.pem", "client.key")
	if err != nil {
		log.Fatalf("failed to load keys: %v", err)
	}
	client.AddCertificateForURLPrefix("gemini://localhost", clientCert)

	r, certificates, authenticated, ok, err := client.Request("gemini://localhost")
	if err != nil {
		log.Printf("request failed: %v", err)
		return
	}
	if !ok {
		log.Printf("request won't be allowed unless the following certificates are accepted: %v", certificates)
	}

	// Try again with the certificate set.
	log.Println("Trying again with the certificate added manually.")
	log.Println("Allowed certificates are returned by the request.")
	client.AddAlllowedCertificateForHost("localhost", "3082016f3081f5020900bd8411af77d2f052300a06082a8648ce3d0403023021310b30090603550406130267623112301006035504030c096c6f63616c686f7374301e170d3230303831363231343133305a170d3330303831343231343133305a3021310b30090603550406130267623112301006035504030c096c6f63616c686f73743076301006072a8648ce3d020106052b81040022036200044b8f60f29a38bcf6dd505f68976965ccc78f12270459c403c65e0215c6daee224b6f68c7249d86c2273c62c0a8af8169b6750801264ad563560a5f3c921f5ec1f01eb857a635a3043ae25f26aaa5a375e657467f7f70fdf963496506abfba4fa300a06082a8648ce3d0403020369003066023100a0cd07b89b80b5d8c102b09c8af6d5a11417f83ef9d7d7f4028bc375e2ddd2dcb0f87596ac8a4c32e573f54e0cd44a47023100feacab1b6daee415f2985ddcecb8dc709524e309c87a2731fd72563fb74db559692bfc59abd7287c24e0f0cd506a9236e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")

	r, _, authenticated, ok, err = client.Request("gemini://localhost")
	if err != nil {
		log.Fatalf("It should work with the certificate added, but got error %v", err)
	}
	if !ok {
		log.Fatalf("It should have worked with the certificate added, but got ok %v", ok)
	}
	fmt.Println("Authenticated:", authenticated)
	fmt.Println("Code:", r.Header.Code)
	fmt.Println("Meta:", r.Header.Meta)
	fmt.Println("")
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatalf("failed to read body: %v", err)
	}
	fmt.Println(string(body))
}
