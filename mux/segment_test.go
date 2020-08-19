package mux

import "testing"

func TestSegmentString(t *testing.T) {
	s := Segment{
		IsVariable: false,
		IsWildcard: true,
		Name:       "segment name",
	}
	if s.String() != "{ Name: segment name, IsVariable: false, IsWildcard: true }" {
		t.Error("Unexpected string value.")
	}
}
