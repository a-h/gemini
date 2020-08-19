package main

import (
	"context"
	"fmt"

	"github.com/a-h/gemini"
	"github.com/a-h/gemini/mux"
)

func main() {
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

	// Create a router.
	router := mux.NewMux()
	router.AddRoute("/require_cert", gemini.RequireCertificateHandler(helloHandler, nil))
	router.AddRoute("/public", okHandler)

	ctx := context.Background()
	err := gemini.ListenAndServe(ctx, "", "server.crt", "server.key", router)
	if err != nil {
		fmt.Println("error:", err)
	}
}
