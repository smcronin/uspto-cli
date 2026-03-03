package cmd

import (
	"testing"

	"github.com/smcronin/uspto-cli/internal/types"
)

func TestSortDocumentsByDateExpr(t *testing.T) {
	docs := []types.Document{
		{OfficialDate: "2024-02-01", DocumentCode: "B"},
		{OfficialDate: "2024-01-01", DocumentCode: "A"},
	}
	asc, err := sortDocumentsByDateExpr(docs, "date:asc")
	if err != nil {
		t.Fatalf("sortDocumentsByDateExpr asc error: %v", err)
	}
	if asc[0].DocumentCode != "A" {
		t.Fatalf("asc first code = %q, want A", asc[0].DocumentCode)
	}

	desc, err := sortDocumentsByDateExpr(docs, "date:desc")
	if err != nil {
		t.Fatalf("sortDocumentsByDateExpr desc error: %v", err)
	}
	if desc[0].DocumentCode != "B" {
		t.Fatalf("desc first code = %q, want B", desc[0].DocumentCode)
	}
}

func TestResolveDocumentSelection_ByIndexAndIdentifier(t *testing.T) {
	docs := []types.Document{
		{DocumentIdentifier: "doc-a"},
		{DocumentIdentifier: "doc-b"},
	}

	idx, doc, err := resolveDocumentSelection(docs, "2")
	if err != nil {
		t.Fatalf("resolve index error: %v", err)
	}
	if idx != 2 || doc.DocumentIdentifier != "doc-b" {
		t.Fatalf("index resolution mismatch: idx=%d doc=%s", idx, doc.DocumentIdentifier)
	}

	idx, doc, err = resolveDocumentSelection(docs, "doc-a")
	if err != nil {
		t.Fatalf("resolve identifier error: %v", err)
	}
	if idx != 1 || doc.DocumentIdentifier != "doc-a" {
		t.Fatalf("identifier resolution mismatch: idx=%d doc=%s", idx, doc.DocumentIdentifier)
	}
}

func TestUniqueOutputPath_AppendsSuffixOnCollision(t *testing.T) {
	seen := map[string]int{}
	p1, c1 := uniqueOutputPath("x.pdf", seen)
	p2, c2 := uniqueOutputPath("x.pdf", seen)
	p3, c3 := uniqueOutputPath("x.pdf", seen)

	if p1 != "x.pdf" || c1 {
		t.Fatalf("first path = %q collided=%v, want x.pdf false", p1, c1)
	}
	if p2 != "x_1.pdf" || !c2 {
		t.Fatalf("second path = %q collided=%v, want x_1.pdf true", p2, c2)
	}
	if p3 != "x_2.pdf" || !c3 {
		t.Fatalf("third path = %q collided=%v, want x_2.pdf true", p3, c3)
	}
}

func TestSelectPrimaryAttorney(t *testing.T) {
	pfw := &types.PatentFileWrapper{
		RecordAttorney: &types.RecordAttorney{
			AttorneyBag: []types.AttorneyEntry{
				{FirstName: "Jane", LastName: "Doe", RegistrationNumber: "12345"},
			},
		},
	}
	got := selectPrimaryAttorney(pfw)
	if got == nil {
		t.Fatal("selectPrimaryAttorney returned nil, want record")
	}
	if got["name"] != "Jane Doe" {
		t.Fatalf("primary name = %q, want Jane Doe", got["name"])
	}
}
