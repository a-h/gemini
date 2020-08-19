package main

import (
	"context"
	"fmt"

	"github.com/a-h/gemini"
)

func main() {
	handler := gemini.HandlerFunc(func(w gemini.ResponseWriter, r *gemini.Request) {
		w.Write([]byte("# Hello, world!\n"))
		if r.Certificate.ID == "" {
			w.Write([]byte("You're not authenticated"))
			return
		}
		w.Write([]byte(fmt.Sprintf("Certificate: %v\n", r.Certificate.ID)))
	})
	ctx := context.Background()
	err := gemini.ListenAndServe(ctx, "", "server.crt", "server.key", handler)
	if err != nil {
		fmt.Println("error:", err)
	}
}
