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
	routerA.AddRoute("/require_cert", gemini.RequireCertificateHandler(helloHandler, nil))
	routerA.AddRoute("/public", okHandler)

	// Create a file system handler gemini://b.gemini/{path}
	handlerB := gemini.FileSystemHandler(gemini.Dir("./content"))

	// Set up the domain handlers.
	ctx := context.Background()
	a, err := gemini.NewDomainHandlerFromFiles("a.gemini", "a.crt", "a.key", routerA)
	if err != nil {
		log.Fatal("error creating domain handler A:", err)
	}
	b, err := gemini.NewDomainHandlerFromFiles("b.gemini", "b.crt", "b.key", handlerB)
	if err != nil {
		log.Fatal("error creating domain handler B:", err)
	}

	// Start the server for two domains (a.gemini / b.gemini).
	err = gemini.ListenAndServe(ctx, ":1965", a, b)
	if err != nil {
		log.Fatal("error:", err)
	}
}
