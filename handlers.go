package gemini

import (
	"net/url"
	"strings"
)

// NotFound responds with a 51 status.
func NotFound(w ResponseWriter, r *Request) {
	w.SetHeader(CodeNotFound, "")
}

// NotFoundHandler creates a handler that returns not found.
func NotFoundHandler() Handler {
	return HandlerFunc(NotFound)
}

// RedirectTemporaryHandler returns a temporary redirection.
func RedirectTemporaryHandler(to string) Handler {
	return HandlerFunc(func(w ResponseWriter, r *Request) {
		w.SetHeader(CodeRedirect, to)
	})
}

// RedirectPermanentHandler returns a handler which returns a permanent redirect.
func RedirectPermanentHandler(to string) Handler {
	return HandlerFunc(func(w ResponseWriter, r *Request) {
		w.SetHeader(CodeRedirectPermanent, to)
	})
}

// StripPrefixHandler strips a prefix from the incoming URL and passes the strippe URL to h.
func StripPrefixHandler(prefix string, h Handler) Handler {
	if prefix == "" {
		return h
	}
	return HandlerFunc(func(w ResponseWriter, r *Request) {
		if p := strings.TrimPrefix(r.URL.Path, prefix); len(p) < len(r.URL.Path) {
			r2 := new(Request)
			*r2 = *r
			r2.URL = new(url.URL)
			*r2.URL = *r.URL
			r2.URL.Path = p
			h.ServeGemini(w, r2)
			return
		}
		NotFound(w, r)
	})
}
