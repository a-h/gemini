package mux

import (
	"context"
	"strings"

	"github.com/a-h/gemini"
)

// Mux routes Gemini requests to the appropriate handler.
type Mux struct {
	RouteHandlers   []*RouteHandler
	NotFoundHandler gemini.Handler
}

// NewMux creates a new Mux for routing requests.
func NewMux() *Mux {
	return &Mux{
		RouteHandlers:   make([]*RouteHandler, 0),
		NotFoundHandler: DefaultNotFoundHandler,
	}
}

// AddRoute to the mux.
func (m *Mux) AddRoute(pattern string, handler gemini.Handler) {
	rh := &RouteHandler{
		Route:   NewRoute(pattern),
		Handler: handler,
	}
	m.RouteHandlers = append(m.RouteHandlers, rh)
}

// DefaultNotFoundHandler is the default handler for requests to invalid routes.
var DefaultNotFoundHandler = gemini.HandlerFunc(func(w gemini.ResponseWriter, r *gemini.Request) {
	w.SetHeader(gemini.CodeNotFound, "")
})

// RouteHandler is the Handler to use for a given Route.
type RouteHandler struct {
	Route   *Route
	Handler gemini.Handler
}

// contextKey used to store the route handler in the request context.
type contextKey string

// matchedRouteContextKey is the key stored in the context.
const matchedRouteContextKey = contextKey("matchedRoute")

// MatchedRoute is provided in the context to Gemini handlers that use the router.
type MatchedRoute struct {
	Pattern  string
	PathVars map[string]string
}

func (m *Mux) ServeGemini(w gemini.ResponseWriter, r *gemini.Request) {
	s := r.URL.Path
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimPrefix(s, "/")
	segments := strings.Split(s, "/")

	for _, rh := range m.RouteHandlers {
		v, ok := rh.Route.Match(segments)
		if ok {
			mr := MatchedRoute{
				Pattern:  rh.Route.Pattern,
				PathVars: v,
			}
			r.Context = context.WithValue(r.Context, matchedRouteContextKey, mr)
			rh.Handler.ServeGemini(w, r)
			return
		}
	}
	m.NotFoundHandler.ServeGemini(w, r)
}

// GetMatchedRoute returns the route that was matched by the router, along with any path variables extracted from the URL.
func GetMatchedRoute(ctx context.Context) (mr MatchedRoute, ok bool) {
	mr, ok = ctx.Value(matchedRouteContextKey).(MatchedRoute)
	return mr, ok
}
