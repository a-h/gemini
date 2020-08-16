package main

import (
	"context"
	"fmt"

	"github.com/a-h/gemini"
)

func main() {
	handler := gemini.HandlerFunc(func(w gemini.ResponseWriter, r *gemini.Request) {
		w.Write([]byte("# Hello, world!"))
	})
	ctx := context.Background()
	err := gemini.ListenAndServe(ctx, "", "server.crt", "server.key", handler)
	if err != nil {
		fmt.Println("error:", err)
	}
}
