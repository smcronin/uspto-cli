package cmd

import (
	"testing"

	"github.com/smcronin/uspto-cli/internal/types"
)

func TestSummarizeDocuments_ReportsPreferredTextFormat(t *testing.T) {
	docs := []types.Document{
		{
			ApplicationNumberText: "123",
			DocumentIdentifier:    "doc-1",
			DocumentCode:          "CTNF",
			DownloadOptionBag: []types.DownloadOption{
				{MimeTypeIdentifier: "PDF", DownloadURL: "https://example/1.pdf"},
				{MimeTypeIdentifier: "XML", DownloadURL: "https://example/1.xmlarchive"},
			},
		},
		{
			ApplicationNumberText: "123",
			DocumentIdentifier:    "doc-2",
			DocumentCode:          "RESP",
			DownloadOptionBag: []types.DownloadOption{
				{MimeTypeIdentifier: "PDF", DownloadURL: "https://example/2.pdf"},
			},
		},
	}

	got := summarizeDocuments(docs)
	if len(got) != 2 {
		t.Fatalf("summarizeDocuments length = %d, want 2", len(got))
	}
	if got[0].PreferredTextFormat != "xml" || !got[0].CanExtractText {
		t.Fatalf("first summary = %#v, want xml text-readable", got[0])
	}
	if got[1].PreferredTextFormat != "" || got[1].CanExtractText {
		t.Fatalf("second summary = %#v, want pdf-only not text-readable", got[1])
	}
}

func TestValidateTextFormatRequest(t *testing.T) {
	okCases := []string{"auto", "xml", "docx", ""}
	for _, tc := range okCases {
		if err := validateTextFormatRequest(tc); err != nil {
			t.Fatalf("validateTextFormatRequest(%q) error = %v, want nil", tc, err)
		}
	}

	err := validateTextFormatRequest("pdf")
	if err == nil {
		t.Fatal("validateTextFormatRequest(pdf) = nil error, want error")
	}
}
