package mux

import (
	"strings"
)

// Route is an array of segments.
type Route struct {
	Pattern  string
	Segments []*Segment
}

// NewRoute creates a route based on a pattern, e.g /users/{userid}.
func NewRoute(pattern string) *Route {
	var r Route
	r.Pattern = pattern

	pattern = strings.TrimSuffix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")

	for _, seg := range strings.Split(pattern, "/") {
		ps := &Segment{
			Name: seg,
		}
		if seg == "*" {
			ps.IsWildcard = true
		}
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			ps.IsVariable = true
			ps.Name = strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
		}
		r.Segments = append(r.Segments, ps)
	}

	return &r
}

// Match returns whether the route was matched, and extracts variables.
func (r Route) Match(segments []string) (vars map[string]string, ok bool) {
	vars = make(map[string]string)
	var wildcard bool
	for i := 0; i < len(r.Segments); i++ {
		routeSegment := r.Segments[len(r.Segments)-1-i]
		inputSegmentIndex := len(segments) - 1 - i
		var inputSegment string
		if inputSegmentIndex > -1 {
			inputSegment = segments[inputSegmentIndex]
		}
		name, capture, wildcardMatch, matches := routeSegment.Match(inputSegment)
		if matches {
			if wildcardMatch {
				wildcard = true
			} else {
				wildcard = false
			}
		}
		if wildcard {
			matches = true
		}
		if !matches {
			return
		}
		if capture {
			vars[name] = inputSegment
		}
	}
	ok = true
	return
}
