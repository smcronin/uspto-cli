package doctext

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestExtractXMLArchive_PreservesParagraphsAndListNumbers(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	xmlBody := `<?xml version="1.0" encoding="UTF-8"?>
<root xmlns:com="urn:test">
  <DocumentMetadata><DocumentCode>CTNF</DocumentCode></DocumentMetadata>
  <P>DETAILED ACTION</P>
  <LI com:liNumber="1.">First point</LI>
  <P>Second <B>paragraph</B>.</P>
</root>`
	if err := tw.WriteHeader(&tar.Header{Name: "doc.xml", Mode: 0600, Size: int64(len(xmlBody))}); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write([]byte(xmlBody)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := Extract("XML", buf.Bytes())
	if err != nil {
		t.Fatalf("Extract(XML): %v", err)
	}

	if got.Format != "xml" {
		t.Fatalf("Format = %q, want xml", got.Format)
	}
	if len(got.EntryNames) != 1 || got.EntryNames[0] != "doc.xml" {
		t.Fatalf("EntryNames = %#v, want [doc.xml]", got.EntryNames)
	}
	if !strings.Contains(got.Text, "DETAILED ACTION") {
		t.Fatalf("text missing heading: %q", got.Text)
	}
	if !strings.Contains(got.Text, "1. First point") {
		t.Fatalf("text missing list numbering: %q", got.Text)
	}
	if !strings.Contains(got.Text, "Second paragraph.") {
		t.Fatalf("text missing paragraph body: %q", got.Text)
	}
	if strings.Contains(got.Text, "CTNF") {
		t.Fatalf("text should skip metadata fields, got %q", got.Text)
	}
	if got.WordCount == 0 || got.CharacterCount == 0 {
		t.Fatalf("word/character counts should be populated, got %#v", got)
	}
}

func TestExtractDOCX_ReadsWordDocumentXML(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	docXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Hello world</w:t></w:r></w:p>
    <w:p><w:r><w:t>Second paragraph</w:t></w:r></w:p>
  </w:body>
</w:document>`
	if _, err := f.Write([]byte(docXML)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := Extract("MS_WORD", buf.Bytes())
	if err != nil {
		t.Fatalf("Extract(MS_WORD): %v", err)
	}

	if got.Format != "docx" {
		t.Fatalf("Format = %q, want docx", got.Format)
	}
	if len(got.EntryNames) != 1 || got.EntryNames[0] != "word/document.xml" {
		t.Fatalf("EntryNames = %#v, want [word/document.xml]", got.EntryNames)
	}
	if !strings.Contains(got.Text, "Hello world") {
		t.Fatalf("text missing first paragraph: %q", got.Text)
	}
	if !strings.Contains(got.Text, "Second paragraph") {
		t.Fatalf("text missing second paragraph: %q", got.Text)
	}
}
