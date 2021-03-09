# Gemini

Applications and libraries for building applications on Gemini (see https://gemini.circumlunar.space/).

## Gemini CLI

### Run a server

```sh
gemini serve --domain=example.com --certFile=a.crt --keyFile=a.key --path=.
```

### Request content

curl for Gemini.

```sh
gemini request --insecure --verbose gemini://example.com/pass
```

## Gemini Server Docker image

### Run a server with Docker

```sh
docker run \
    -v /path_to_your_cert_files:/certs \
    -e PORT=1965 \
    -e DOMAIN=localhost \
    -v /path_to_your_content:/content \
    -p 1965:1965 \
    adrianhesketh/gemini:latest
```

## Quick start

Check out https://github.com/a-h/gemini/releases for the latest version of the `gemini` command line tool to run locally, or use Docker:

```sh
# Create a server certificate.
openssl ecparam -genkey -name secp384r1 -out server.key
openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650
# Make a Gemini file.
mkdir content
echo "# Hello, World!" > content/index.gmi
# Run the container.
docker pull adrianhesketh/gemini:latest
docker run -v `pwd`:/certs -e PORT=1965 -e DOMAIN=localhost -v `pwd`/content:/content -p 1965:1965 adrianhesketh/gemini:latest
```

## Libraries

### Serve

Use `gemini.Server` / `gemini.ListenAndServe` to build your own custom servers. 

Supports hosting multiple Gemini servers on a single IP address.

These are used to build a Gemini application that supports dynamic content.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/a-h/gemini"
	"github.com/a-h/gemini/mux"
)

func main() {
	// Create the handlers for a domain (a.gemini).
	okHandler := gemini.HandlerFunc(func(w gemini.ResponseWriter, r *gemini.Request) {
		w.Write([]byte("OK"))
	})

	helloHandler := gemini.HandlerFunc(func(w gemini.ResponseWriter, r *gemini.Request) {
		w.Write([]byte("# Hello, user!\n"))
		if r.Certificate.ID == "" {
			w.Write([]byte("You're not authenticated"))
			return
		}
		w.Write([]byte(fmt.Sprintf("Certificate: %v\n", r.Certificate.ID)))
	})

	// Create a router for gemini://a.gemini/require_cert and gemini://a.gemini/public
	routerA := mux.NewMux()
	// Let's make /require_cert require the client to be authenticated.
	routerA.AddRoute("/require_cert", gemini.RequireCertificateHandler(helloHandler, nil))
	routerA.AddRoute("/public", okHandler)

	// Create a file system handler gemini://b.gemini/{path}
	handlerB := gemini.FileSystemHandler(gemini.Dir("./content"))

	// Set up the domain handlers.
	ctx := context.Background()
	a, err := gemini.NewDomainHandler("a.gemini", "a.crt", "a.key", routerA)
	if err != nil {
		log.Fatal("error creating domain handler A:", err)
	}
	b, err := gemini.NewDomainHandler("b.gemini", "b.crt", "b.key", handlerB)
	if err != nil {
		log.Fatal("error creating domain handler B:", err)
	}

	// Start the server for two domains (a.gemini / b.gemini).
	err = gemini.ListenAndServe(ctx, ":1965", a, b)
	if err != nil {
		log.Fatal("error:", err)
	}
}
```

### Route

Use `github.com/a-h/gemini/mux` to provide routing between Gemini handlers and extract variables from URL paths.

### Built-in utility handlers

* `RequireCertificateHandler` a handler that ensures that users present certificates.
* `FileSystemHandler` to support hosting static content.


### Gemini client

```go
client := gemini.NewClient()

// Make a request to the server without accepting its certificate.
r, certificates, authenticated, ok, err := client.Request("gemini://a.gemini/require_cert")
if err != nil {
	log.Printf("Request failed: %v", err)
	return
}
```

Configure allowed server certificates for trust-on-first-use certificate support:

```
client.AddAlllowedCertificateForHost("a.gemini", "3082016c3081f3020900d4c7c9907518eb61300a06082a8648ce3d0403023020310b30090603550406130267623111300f06035504030c08612e67656d696e69301e170d3230303832303139303330335a170d3330303831383139303330335a3020310b30090603550406130267623111300f06035504030c08612e67656d696e693076301006072a8648ce3d020106052b8104002203620004ae5cabe01f708d8f9423725df49601e1a033a1b51eb73cd3a8a9853011346127cbfedb57c4bd14ad6000ccb2f748d32b2a2b817b1860781d937e7666680874876fb4a9a91c44e2cf8c9804d40f6e7122f6c92a1884b62bd9f0749cca4e12cfa8300a06082a8648ce3d0403020368003065023100ae447eb9455e9ca1f02f013390d2c4029a7f29732cf6e29787b53b6435904d622f47f3b1fbffe60a284dbd4cddd6ef580230518dcb0355d5c3d880357128972c630ca90a915f1eb417a7ea0e4518a72dfc8a76c9b50c51d56f6a6835c4dfa989b72be3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
```
