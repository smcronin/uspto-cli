package tsdr

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListDocumentsUsesFastSingleCasePathAndFiltersLocally(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ts/cd/casedocs/sn12345678/info.xml" {
			t.Fatalf("path = %q, want single-case metadata path", r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Fatalf("single-case request unexpectedly forwarded filters: %q", r.URL.RawQuery)
		}
		if got := r.Header.Get(APIKeyHeader); got != "tsdr-key" {
			t.Fatalf("%s = %q", APIKeyHeader, got)
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0"?><DocumentList>
			<Document><SerialNumber>12345678</SerialNumber><MailRoomDate>2024-03-03-05:00</MailRoomDate><DocumentTypeCode>RSI</DocumentTypeCode><CategoryTypeCode>RC</CategoryTypeCode><UrlPathList><UrlPath>https://tsdrapi.uspto.gov/ts/cd/casedoc/sn12345678/RSI20240303000000/1/media</UrlPath></UrlPathList></Document>
			<Document><SerialNumber>12345678</SerialNumber><MailRoomDate>2024-01-02-05:00</MailRoomDate><DocumentTypeCode>RSI</DocumentTypeCode><CategoryTypeCode>RC</CategoryTypeCode><UrlPathList><UrlPath>https://tsdrapi.uspto.gov/ts/cd/casedoc/sn12345678/RSI20240102000000/1/media</UrlPath></UrlPathList></Document>
			<Document><SerialNumber>12345678</SerialNumber><MailRoomDate>2023-12-31-05:00</MailRoomDate><DocumentTypeCode>SPE</DocumentTypeCode><CategoryTypeCode>IN</CategoryTypeCode></Document>
		</DocumentList>`)
	}))
	defer server.Close()

	id, err := ParseIdentifier("12345678", "serial")
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient("tsdr-key", WithBaseURL(server.URL), WithoutRateLimit())
	list, err := client.ListDocuments(context.Background(), DocumentQuery{
		Identifiers: []Identifier{id},
		FromDate:    "2024-01-01",
		Types:       []string{"rsi"},
		Category:    "rc",
		Sort:        "date:A",
	})
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(list.Documents) != 2 {
		t.Fatalf("document count = %d, want 2", len(list.Documents))
	}
	if got := list.Documents[0].MailRoomDate; got != "2024-01-02-05:00" {
		t.Fatalf("first date = %q", got)
	}
	if got := list.Documents[0].DocumentID; got != "RSI20240102000000" {
		t.Fatalf("first document ID = %q", got)
	}
	if list.Documents[0].Index != 1 || list.Documents[1].Index != 2 {
		t.Fatalf("indexes = %d,%d", list.Documents[0].Index, list.Documents[1].Index)
	}
}

func TestListDocumentsUsesFastCasePathsForMultipleCases(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		wantPath := "/ts/cd/casedocs/sn12345678/info.xml"
		serial := "12345678"
		if requests == 2 {
			wantPath = "/ts/cd/casedocs/sn87654321/info.xml"
			serial = "87654321"
		}
		if r.URL.Path != wantPath {
			t.Fatalf("path = %q, want %q", r.URL.Path, wantPath)
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<DocumentList><Document><SerialNumber>%s</SerialNumber></Document></DocumentList>`, serial)
	}))
	defer server.Close()

	first, _ := ParseIdentifier("12345678", "serial")
	second, _ := ParseIdentifier("87654321", "serial")
	client := NewClient("tsdr-key", WithBaseURL(server.URL), WithoutRateLimit())
	list, err := client.ListDocuments(context.Background(), DocumentQuery{Identifiers: []Identifier{first, first, second, second}})
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if requests != 2 || len(list.Documents) != 2 || list.Documents[0].Index != 1 || list.Documents[1].Index != 1 {
		t.Fatalf("documents = %#v", list.Documents)
	}
}

func TestApplyDocumentQueryUsesReusableCaseScopedIndices(t *testing.T) {
	list, err := ParseDocumentList([]byte(`<DocumentList>
		<Document><SerialNumber>11111111</SerialNumber><MailRoomDate>2024-03-01</MailRoomDate></Document>
		<Document><SerialNumber>22222222</SerialNumber><MailRoomDate>2024-02-01</MailRoomDate></Document>
		<Document><SerialNumber>11111111</SerialNumber><MailRoomDate>2024-01-01</MailRoomDate></Document>
	</DocumentList>`))
	if err != nil {
		t.Fatal(err)
	}
	first, _ := ParseIdentifier("11111111", "serial")
	second, _ := ParseIdentifier("22222222", "serial")
	result, err := ApplyDocumentQuery(list, DocumentQuery{Identifiers: []Identifier{first, second}, Sort: "date:A"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Documents) != 3 {
		t.Fatalf("document count = %d, want 3", len(result.Documents))
	}
	got := result.Documents
	if got[0].SerialNumber != "11111111" || got[0].Index != 1 || got[0].SelectionIndex != 2 {
		t.Errorf("first sorted document = %#v, want case 11111111 local index 1 and original ordinal 2", got[0])
	}
	if got[1].SerialNumber != "22222222" || got[1].Index != 1 || got[1].SelectionIndex != 1 {
		t.Errorf("second sorted document = %#v, want case 22222222 local index/ordinal 1", got[1])
	}
	if got[2].SerialNumber != "11111111" || got[2].Index != 2 || got[2].SelectionIndex != 1 {
		t.Errorf("third sorted document = %#v, want case 11111111 local index 2 and original ordinal 1", got[2])
	}
}

func TestApplyDocumentQueryRejectsInvalidSort(t *testing.T) {
	id, _ := ParseIdentifier("12345678", "serial")
	_, err := ApplyDocumentQuery(&DocumentList{}, DocumentQuery{
		Identifiers: []Identifier{id},
		Sort:        "date:sideways",
	})
	if err == nil {
		t.Fatal("ApplyDocumentQuery() expected sort error")
	}
}

func TestBundleDocumentsFetchesRichMetadataThenFiltersLocally(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.RawQuery, "sn=97238896"; got != want {
			t.Fatalf("raw query = %q, want identifiers only %q", got, want)
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<DocumentList>
			<Document><SerialNumber>97238896</SerialNumber><MailRoomDate>2026-07-30-04:00</MailRoomDate><DocumentTypeCode>AMC</DocumentTypeCode><UrlPathList><UrlPath>https://tmng-al.uspto.gov/snapshot.xml</UrlPath></UrlPathList></Document>
			<Document><SerialNumber>97238896</SerialNumber><MailRoomDate>2026-07-30-04:00</MailRoomDate><DocumentTypeCode>RSI</DocumentTypeCode><CategoryTypeCode>I</CategoryTypeCode><UrlPathList><UrlPath>https://tsdrapi.uspto.gov/ts/cd/casedoc/sn97238896/RSI20260730201140/1/media</UrlPath></UrlPathList></Document>
		</DocumentList>`)
	}))
	defer server.Close()

	id, _ := ParseIdentifier("97238896", "serial")
	client := NewClient("key", WithBaseURL(server.URL), WithoutRateLimit())
	list, err := client.BundleDocuments(context.Background(), DocumentQuery{
		Identifiers: []Identifier{id},
		Date:        "2026-07-30",
		Types:       []string{"RSI"},
		Category:    "I",
	})
	if err != nil {
		t.Fatalf("BundleDocuments() error = %v", err)
	}
	if len(list.Documents) != 1 || list.Documents[0].DocumentID != "RSI20260730201140" || list.Documents[0].Index != 1 {
		t.Fatalf("documents = %#v", list.Documents)
	}
}
