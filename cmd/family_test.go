package cmd

import "testing"

func TestRelationshipNormalization(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "con", want: "CON"},
		{in: " div ", want: "DIV"},
		{in: "cip", want: "CIP"},
		{in: "pro", want: "PRO"},
		{in: "", want: "PARENT"},
	}
	for _, tc := range tests {
		if got := parentRelationship(tc.in); got != tc.want {
			t.Fatalf("parentRelationship(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := childRelationship(""); got != "CHILD" {
		t.Fatalf("childRelationship(\"\") = %q, want CHILD", got)
	}
}
