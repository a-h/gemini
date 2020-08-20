package gemini

import (
	"net/url"
	"strings"
)

// BadRequest responds with a 59 status.
func BadRequest(w ResponseWriter, r *Request) {
	w.SetHeader(CodeBadRequest, "")
}

// BadRequestHandler creates a handler that returns a bad request code (59).
func BadRequestHandler() Handler {
	return HandlerFunc(BadRequest)
}

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

// RequireCertificateHandler returns a handler that enforces authentication on h.
// authoriser can be set to limit which users can access h. If authoriser
// is nil, authoriser is set to AuthoriserAllowAll which allows any authenticated
// user to access the handler.
func RequireCertificateHandler(h Handler, authoriser func(certID, certKey string) bool) Handler {
	if authoriser == nil {
		authoriser = AuthoriserAllowAll
	}
	return HandlerFunc(func(w ResponseWriter, r *Request) {
		if r.Certificate.ID == "" {
			w.SetHeader(CodeClientCertificateRequired, "")
			return
		}
		if !authoriser(r.Certificate.ID, r.Certificate.Key) {
			w.SetHeader(CodeClientCertificateNotAuthorised, "")
			return
		}
		h.ServeGemini(w, r)
	})
}

// AuthoriserAllowAll allows any authenticated user to access the handler.
func AuthoriserAllowAll(id, key string) bool {
	return true
}
