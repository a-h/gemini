package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/a-h/gemini"
)

func main() {
	// Run the server example and ensure that the following lines are in your host file (e.g. /etc/hosts) to allow
	// the server to listen locally for the two domains.
	// 127.0.0.1	a.gemini
	// 127.0.0.1	b.gemini
	ctx := context.Background()
	client := gemini.NewClient()

	// Make a request to the server without accepting its certificate.
	r, certificates, authenticated, ok, err := client.Request(ctx, "gemini://a.gemini/require_cert")
	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}
	if !ok {
		log.Printf("Request won't be allowed unless the following certificates are accepted: %v", certificates)
	}

	// Try again with the certificate set.
	log.Println("Trying again with the certificate added manually.")
	client.AddServerCertificate("a.gemini", "3082016c3081f3020900d4c7c9907518eb61300a06082a8648ce3d0403023020310b30090603550406130267623111300f06035504030c08612e67656d696e69301e170d3230303832303139303330335a170d3330303831383139303330335a3020310b30090603550406130267623111300f06035504030c08612e67656d696e693076301006072a8648ce3d020106052b8104002203620004ae5cabe01f708d8f9423725df49601e1a033a1b51eb73cd3a8a9853011346127cbfedb57c4bd14ad6000ccb2f748d32b2a2b817b1860781d937e7666680874876fb4a9a91c44e2cf8c9804d40f6e7122f6c92a1884b62bd9f0749cca4e12cfa8300a06082a8648ce3d0403020368003065023100ae447eb9455e9ca1f02f013390d2c4029a7f29732cf6e29787b53b6435904d622f47f3b1fbffe60a284dbd4cddd6ef580230518dcb0355d5c3d880357128972c630ca90a915f1eb417a7ea0e4518a72dfc8a76c9b50c51d56f6a6835c4dfa989b72be3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")

	// Try to access the authenticated area without a client certificate.
	r, _, authenticated, ok, err = client.Request(ctx, "gemini://a.gemini/require_cert")
	if err != nil {
		log.Fatalf("It should work with the certificate added, but got error %v", err)
	}
	if !ok {
		log.Fatalf("It should have worked with the certificate added, but got ok %v", ok)
	}
	if r.Header.Code != gemini.CodeClientCertificateRequired {
		log.Printf("Expected code %v, but got %v", gemini.CodeClientCertificateRequired, r.Header.Code)
	} else {
		log.Println("The request was rejected because a client certificate is required. Let's try again...")
	}

	// Enable client authentication.
	clientCert, err := tls.LoadX509KeyPair("client.pem", "client.key")
	if err != nil {
		log.Fatalf("Failed to load keys: %v", err)
	}
	client.AddClientCertificate("gemini://a.gemini", clientCert)

	r, certificates, authenticated, ok, err = client.Request(ctx, "gemini://a.gemini/require_cert")
	fmt.Println("Authenticated:", authenticated)
	fmt.Println("Code:", r.Header.Code)
	fmt.Println("Meta:", r.Header.Meta)
	fmt.Println("")
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatalf("failed to read body: %v", err)
	}
	fmt.Println(string(body))

	fmt.Println("Now attempting to access b.gemini...")
	client.AddServerCertificate("b.gemini", "3082016c3081f3020900bb2a5435c3beaec4300a06082a8648ce3d0403023020310b30090603550406130267623111300f06035504030c08622e67656d696e69301e170d3230303832303139303032375a170d3330303831383139303032375a3020310b30090603550406130267623111300f06035504030c08622e67656d696e693076301006072a8648ce3d020106052b8104002203620004e3fc081f8c7de5e558c9b054d4b8ea1005786aef3d38b2de2cd7b5950ed98926fa4402360441e2b61587581d0f4d7f8156b770b06c2dfa15e27104d38f39587cc1bd3c3984518d5c35c1e4f789317c01ce54c9fd1402417a9cb2e25048ee48da300a06082a8648ce3d0403020368003065023100b4eca072abaa84eafe983ffeddd5e0ac0fc7dbd5b22d02bd80438f8d1ee1348d222a43af951e986c19492aaf7426728d02305de8eca89504be7b6f12430f9be8d4d8192c919d9b5bc6846d8c399f7155c7bbaae7592681be06c9b6553a2fffd662d0e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	r, certificates, authenticated, ok, err = client.Request(ctx, "gemini://b.gemini")
	if err != nil {
		log.Fatalf("Failed to access b.gemini: %v", err)
	}
	if !ok {
		log.Fatalf("No known certificate for b.gemini, the server provided certificates: %v", certificates)
	}
	fmt.Printf("Received header request: %v\n", r.Header)
	bdy, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatalf("Failed to read body: %v", err)
	}
	fmt.Println(string(bdy))
}
