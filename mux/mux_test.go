package mux

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"testing"

	"github.com/a-h/gemini"
)

func TestMux(t *testing.T) {
	var tests = []struct {
		name           string
		routeHandlers  []*RouteHandler
		requestURL     string
		expectedHeader gemini.Header
		expectedBody   string
	}{
		{
			name:          "if no routes match, the NotFoundHandler is used",
			routeHandlers: []*RouteHandler{},
			requestURL:    "/not_found",
			expectedHeader: gemini.Header{
				Code: gemini.CodeNotFound,
			},
		},
		{
			name: "matching routes go to the correct handler",
			routeHandlers: []*RouteHandler{
				&RouteHandler{
					Route: NewRoute("/route/a"),
					Handler: gemini.HandlerFunc(func(w gemini.ResponseWriter, r *gemini.Request) {
						w.Write([]byte("a"))
					}),
				},
				&RouteHandler{
					Route: NewRoute("/route/b"),
					Handler: gemini.HandlerFunc(func(w gemini.ResponseWriter, r *gemini.Request) {
						w.Write([]byte("b"))
					}),
				},
			},
			requestURL: "/route/b",
			expectedHeader: gemini.Header{
				Code: gemini.CodeSuccess,
				Meta: gemini.DefaultMIMEType,
			},
			expectedBody: "b",
		},
		{
			name: "route information is available to the handler",
			routeHandlers: []*RouteHandler{
				&RouteHandler{
					Route: NewRoute("/user/{id}/{section}"),
					Handler: gemini.HandlerFunc(func(w gemini.ResponseWriter, r *gemini.Request) {
						mr, ok := GetMatchedRoute(r.Context)
						if !ok {
							t.Fatalf("failed to get matched route")
						}
						output := fmt.Sprintf("%v\n%v", mr.Pattern, mr.PathVars)
						w.Write([]byte(output))
					}),
				},
			},
			requestURL: "/user/user213/settings",
			expectedHeader: gemini.Header{
				Code: gemini.CodeSuccess,
				Meta: gemini.DefaultMIMEType,
			},
			expectedBody: "/user/{id}/{section}\nmap[id:user213 section:settings]",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			h := NewMux()
			h.RouteHandlers = tt.routeHandlers
			u, err := url.Parse(tt.requestURL)
			if err != nil {
				t.Fatalf("failed to parse URL %q: %v", tt.requestURL, err)
			}
			r := &gemini.Request{
				Context: context.Background(),
				URL:     u,
			}
			resp, err := gemini.Record(r, h)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.expectedHeader.Code != resp.Header.Code {
				t.Errorf("expected header code %v, got %v", tt.expectedHeader.Code, resp.Header.Code)
			}
			if tt.expectedHeader.Meta != resp.Header.Meta {
				t.Errorf("expected header meta %q, got %q", tt.expectedHeader.Meta, resp.Header.Meta)
			}
			bdy, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("unexpected error reading body: %v", err)
			}
			if tt.expectedBody != string(bdy) {
				t.Errorf("expected\n%v\nactual\n%v", tt.expectedBody, string(bdy))
			}
		})
	}
}

func TestAddRoute(t *testing.T) {
	m := NewMux()
	m.AddRoute("/test", gemini.HandlerFunc(func(w gemini.ResponseWriter, r *gemini.Request) {
		w.Write([]byte("Hello"))
	}))
	if len(m.RouteHandlers) != 1 {
		t.Errorf("expected 1 route handler to be added, got %d", len(m.RouteHandlers))
	}
}
