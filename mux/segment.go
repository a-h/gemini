package mux

import (
	"fmt"
	"strings"
)

// Segment is a path segment, e.g. in /users/{userid}/ there are two segments,
// "users" and "{userid}". "{userid}" is a variable and will be captured.
type Segment struct {
	Name       string
	IsVariable bool
	IsWildcard bool
}

// String pretty prints the segment, for debugging.
func (ps *Segment) String() string {
	return fmt.Sprintf("{ Name: %v, IsVariable: %v, IsWildcard: %v }",
		ps.Name, ps.IsVariable, ps.IsWildcard)
}

// Match on the string path segment.
func (ps *Segment) Match(s string) (name string, capture bool, wildcard bool, matches bool) {
	if ps.IsWildcard {
		wildcard = true
		matches = true
		return
	}
	if ps.IsVariable {
		name = ps.Name
		capture = true
		matches = true
		return
	}
	if strings.EqualFold(s, ps.Name) {
		matches = true
		return
	}
	return
}
