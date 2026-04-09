package cmd

import (
	"strings"
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

func TestNormalizeDocumentCodes(t *testing.T) {
	got := normalizeDocumentCodes("rejection,allowance,clm,Spec,office-action,CTFR")
	wantParts := []string{"CTNF", "CTFR", "NOA", "CLM", "SPEC"}
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("normalizeDocumentCodes() = %q, want to contain %q", got, part)
		}
	}
}

func TestAvailableFormatList_UsesCanonicalLabels(t *testing.T) {
	doc := &types.Document{
		DownloadOptionBag: []types.DownloadOption{
			{MimeTypeIdentifier: "MS_WORD"},
			{MimeTypeIdentifier: "XML"},
			{MimeTypeIdentifier: "PDF"},
			{MimeTypeIdentifier: "PDF"},
		},
	}

	got := availableFormatList(doc)
	want := []string{"docx", "xml", "pdf"}
	if len(got) != len(want) {
		t.Fatalf("availableFormatList length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("availableFormatList[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolveTextFormat_PrefersXMLThenDOCX(t *testing.T) {
	doc := &types.Document{
		DownloadOptionBag: []types.DownloadOption{
			{MimeTypeIdentifier: "MS_WORD", DownloadURL: "https://example/doc.docx"},
			{MimeTypeIdentifier: "XML", DownloadURL: "https://example/doc.xmlarchive"},
			{MimeTypeIdentifier: "PDF", DownloadURL: "https://example/doc.pdf"},
		},
	}

	mimeType, label, url, err := resolveTextFormat(doc, "auto")
	if err != nil {
		t.Fatalf("resolveTextFormat(auto) error: %v", err)
	}
	if mimeType != "XML" || label != "xml" || url != "https://example/doc.xmlarchive" {
		t.Fatalf("resolveTextFormat(auto) = (%q, %q, %q), want XML/xml/xmlarchive", mimeType, label, url)
	}

	mimeType, label, url, err = resolveTextFormat(doc, "docx")
	if err != nil {
		t.Fatalf("resolveTextFormat(docx) error: %v", err)
	}
	if mimeType != "MS_WORD" || label != "docx" || url != "https://example/doc.docx" {
		t.Fatalf("resolveTextFormat(docx) = (%q, %q, %q), want MS_WORD/docx/docxURL", mimeType, label, url)
	}
}

func TestResolveTextFormat_RejectsPDFOnly(t *testing.T) {
	doc := &types.Document{
		DownloadOptionBag: []types.DownloadOption{
			{MimeTypeIdentifier: "PDF", DownloadURL: "https://example/doc.pdf"},
		},
	}

	_, _, _, err := resolveTextFormat(doc, "auto")
	if err == nil {
		t.Fatal("resolveTextFormat(auto) with pdf-only doc = nil error, want error")
	}
	if !strings.Contains(err.Error(), "only pdf is available") {
		t.Fatalf("resolveTextFormat(auto) error = %q, want pdf-only hint", err.Error())
	}
}

func TestCanonicalFormatLabel_HandlesMIMETypes(t *testing.T) {
	tests := map[string]string{
		"application/pdf": "pdf",
		"application/xml": "xml",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
	}

	for raw, want := range tests {
		if got := canonicalFormatLabel(raw); got != want {
			t.Fatalf("canonicalFormatLabel(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestDownloadOutputExtension_MSWordUsesDownloadURL(t *testing.T) {
	tests := map[string]string{
		"https://example.com/Final%20Rejection.DOC":              ".doc",
		"https://example.com/final-rejection.docx":               ".docx",
		"https://example.com/download?id=12345":                  ".docx",
		"https://example.com/files/office-action.DOC?download=1": ".doc",
	}

	for rawURL, want := range tests {
		if got := downloadOutputExtension("MS_WORD", rawURL); got != want {
			t.Fatalf("downloadOutputExtension(MS_WORD, %q) = %q, want %q", rawURL, got, want)
		}
	}
}

func TestDownloadOutputExtension_NonMSWordUsesDefaultMapping(t *testing.T) {
	tests := map[string]string{
		"PDF": ".pdf",
		"XML": ".tar",
	}

	for mimeType, want := range tests {
		if got := downloadOutputExtension(mimeType, "https://example.com/ignored.bin"); got != want {
			t.Fatalf("downloadOutputExtension(%q) = %q, want %q", mimeType, got, want)
		}
	}
}
