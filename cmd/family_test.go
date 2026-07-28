package cmd

import (
	"testing"

	"github.com/smcronin/uspto-cli/internal/types"
)

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

func TestCollectFamilyRelatedApps_PreservesDirection(t *testing.T) {
	fw := &types.PatentFileWrapper{
		ParentContinuityBag: []types.ParentContinuity{{ParentApplicationNumberText: "11111111", ClaimParentageTypeCode: "CON"}},
		ChildContinuityBag:  []types.ChildContinuity{{ChildApplicationNumberText: "22222222", ClaimParentageTypeCode: "DIV"}},
	}
	parents, children := collectFamilyRelatedApps(fw, map[string]familyVisit{})
	if len(parents) != 1 || parents[0].ApplicationNumber != "11111111" || parents[0].Direction != "parent" {
		t.Fatalf("parents = %#v", parents)
	}
	if len(children) != 1 || children[0].ApplicationNumber != "22222222" || children[0].Direction != "child" {
		t.Fatalf("children = %#v", children)
	}
}
