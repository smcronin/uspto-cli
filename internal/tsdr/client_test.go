package tsdr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testTSDRAPIKey = "tsdr-test-key"

func newTSDRTestClient(serverURL string, opts ...ClientOption) *Client {
	options := []ClientOption{
		WithBaseURL(serverURL),
		WithoutRateLimit(),
		WithTimeout(2 * time.Second),
		WithDownloadTimeout(2 * time.Second),
	}
	options = append(options, opts...)
	return NewClient("  "+testTSDRAPIKey+"  ", options...)
}

func mustIdentifier(t *testing.T, raw, hint string) Identifier {
	t.Helper()
	id, err := ParseIdentifier(raw, hint)
	if err != nil {
		t.Fatalf("ParseIdentifier(%q, %q): %v", raw, hint, err)
	}
	return id
}

func TestNewClientDefaultsAndOptions(t *testing.T) {
	c := NewClient("  secret  ")
	if c.apiKey != "secret" {
		t.Errorf("apiKey = %q, want trimmed secret", c.apiKey)
	}
	if c.GetBaseURL() != DefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.GetBaseURL(), DefaultBaseURL)
	}
	if c.timeout != defaultTimeout || c.downloadTimeout != downloadTimeout {
		t.Errorf("timeouts = %v/%v, want %v/%v", c.timeout, c.downloadTimeout, defaultTimeout, downloadTimeout)
	}
	if c.debug {
		t.Error("debug should default to false")
	}
	if c.rl == nil || !c.rl.enabled {
		t.Error("rate limiting should be enabled by default")
	}

	c = NewClient("secret",
		WithBaseURL("https://example.test/"),
		WithDebug(true),
		WithTimeout(3*time.Second),
		WithDownloadTimeout(4*time.Second),
		WithoutRateLimit(),
	)
	if c.GetBaseURL() != "https://example.test" || !c.debug || c.timeout != 3*time.Second || c.downloadTimeout != 4*time.Second || c.rl.enabled {
		t.Errorf("configured client = %#v", c)
	}
}

func TestClientEndpointRequests(t *testing.T) {
	serial := Identifier{Type: IdentifierSerial, Value: "78787878"}
	registration := Identifier{Type: IdentifierRegistration, Value: "3500038"}
	docQuery := DocumentQuery{
		Identifiers: []Identifier{serial, registration},
		FromDate:    "2025-01-01",
		ToDate:      "2025-12-31",
		Types:       []string{"OOA", "NOA"},
		Category:    "OUT",
		Sort:        "desc",
	}

	tests := []struct {
		name       string
		wantPath   string
		wantQuery  url.Values
		wantAccept string
		call       func(context.Context, *Client) (*Response, error)
	}{
		{
			name:       "case status XML",
			wantPath:   "/ts/cd/casestatus/sn78787878/info.xml",
			wantAccept: "application/xml",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.CaseStatusXML(ctx, serial) },
		},
		{
			name:       "case status JSON",
			wantPath:   "/ts/cd/casestatus/sn78787878/info.json",
			wantAccept: "application/json",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.CaseStatusJSON(ctx, serial) },
		},
		{
			name:       "case status legacy XML",
			wantPath:   "/ts/cd/casestatus/sn78787878/v1/info",
			wantAccept: "application/xml",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.CaseStatusLegacyXML(ctx, serial) },
		},
		{
			name:       "case HTML",
			wantPath:   "/ts/cd/casestatus/sn78787878/content.html",
			wantAccept: "text/html",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.CaseAsset(ctx, serial, "html") },
		},
		{
			name:       "case PDF",
			wantPath:   "/ts/cd/casestatus/sn78787878/download.pdf",
			wantAccept: "application/pdf",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.CaseAsset(ctx, serial, "pdf") },
		},
		{
			name:       "case status ZIP",
			wantPath:   "/ts/cd/casestatus/sn78787878/content.zip",
			wantAccept: "application/zip",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.CaseAsset(ctx, serial, "status-zip") },
		},
		{
			name:       "case image ZIP",
			wantPath:   "/ts/cd/casestatus/sn78787878/download.zip",
			wantAccept: "application/zip",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.CaseAsset(ctx, serial, "image-zip") },
		},
		{
			name:       "multi-case status",
			wantPath:   "/ts/cd/caseMultiStatus/sn",
			wantQuery:  url.Values{"ids": {"78787878,75757575"}, "display": {"true"}, "allowDupes": {"true"}},
			wantAccept: "application/json",
			call: func(ctx context.Context, c *Client) (*Response, error) {
				return c.MultiCaseStatus(ctx, IdentifierSerial, []string{"78787878", "75757575"}, true, true)
			},
		},
		{
			name:       "document metadata",
			wantPath:   "/ts/cd/casedocs/bundle.xml",
			wantQuery:  url.Values{"sn": {"78787878"}, "rn": {"3500038"}, "fromDate": {"2025-01-01"}, "toDate": {"2025-12-31"}, "type": {"OOA,NOA"}, "category": {"OUT"}, "sort": {"desc"}},
			wantAccept: "application/xml",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.DocumentsXML(ctx, docQuery) },
		},
		{
			name:       "document PDF bundle",
			wantPath:   "/ts/cd/casedocs/bundle.pdf",
			wantQuery:  url.Values{"sn": {"78787878"}, "rn": {"3500038"}, "fromDate": {"2025-01-01"}, "toDate": {"2025-12-31"}, "type": {"OOA,NOA"}, "category": {"OUT"}, "sort": {"desc"}},
			wantAccept: "application/pdf",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.DocumentsAsset(ctx, docQuery, "pdf") },
		},
		{
			name:       "document ZIP bundle",
			wantPath:   "/ts/cd/casedocs/bundle.zip",
			wantQuery:  url.Values{"sn": {"78787878"}, "rn": {"3500038"}, "fromDate": {"2025-01-01"}, "toDate": {"2025-12-31"}, "type": {"OOA,NOA"}, "category": {"OUT"}, "sort": {"desc"}},
			wantAccept: "application/zip",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.DocumentsAsset(ctx, docQuery, "zip") },
		},
		{
			name:       "single-case document list",
			wantPath:   "/ts/cd/casedocs/sn78787878/info.xml",
			wantAccept: "application/xml",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.CaseDocumentsXML(ctx, serial) },
		},
		{
			name:       "single document metadata",
			wantPath:   "/ts/cd/casedoc/sn78787878/DOC-123/info.xml",
			wantAccept: "application/xml",
			call: func(ctx context.Context, c *Client) (*Response, error) {
				return c.DocumentInfoXML(ctx, serial, "DOC-123")
			},
		},
		{
			name:       "single document PDF",
			wantPath:   "/ts/cd/casedoc/sn78787878/DOC-123/download.pdf",
			wantAccept: "application/pdf",
			call: func(ctx context.Context, c *Client) (*Response, error) {
				return c.DocumentAsset(ctx, serial, "DOC-123", "pdf")
			},
		},
		{
			name:       "single document ZIP",
			wantPath:   "/ts/cd/casedoc/sn78787878/DOC-123/content.zip",
			wantAccept: "application/zip",
			call: func(ctx context.Context, c *Client) (*Response, error) {
				return c.DocumentAsset(ctx, serial, "DOC-123", "zip")
			},
		},
		{
			name:       "single document page",
			wantPath:   "/ts/cd/casedoc/sn78787878/DOC-123/2/media",
			wantAccept: "*/*",
			call: func(ctx context.Context, c *Client) (*Response, error) {
				return c.DocumentPage(ctx, serial, "DOC-123", 2)
			},
		},
		{
			name:       "raw image",
			wantPath:   "/ts/cd/rawImage/78787878",
			wantAccept: "image/*",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.RawImage(ctx, "78/787,878") },
		},
		{
			name:       "maintenance",
			wantPath:   "/ts/cd/maintenance/sn78787878/info.json",
			wantAccept: "application/json",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.Maintenance(ctx, serial) },
		},
		{
			name:       "case map",
			wantPath:   "/ts/cd/casemap/MAP-123/info",
			wantAccept: "application/xml",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.CaseMap(ctx, " MAP-123 ") },
		},
		{
			name:       "multimedia listing",
			wantPath:   "/ts/cd/multimedia/sn78787878/info.xml",
			wantAccept: "application/xml",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.MultimediaInfo(ctx, serial, "") },
		},
		{
			name:       "multimedia item metadata",
			wantPath:   "/ts/cd/multimedia/sn78787878/0001/info.xml",
			wantAccept: "application/xml",
			call:       func(ctx context.Context, c *Client) (*Response, error) { return c.MultimediaInfo(ctx, serial, "0001") },
		},
		{
			name:       "multimedia content",
			wantPath:   "/ts/cd/multimedia/sn78787878/0001/content",
			wantAccept: "*/*",
			call: func(ctx context.Context, c *Client) (*Response, error) {
				return c.MultimediaContent(ctx, serial, "0001", false)
			},
		},
		{
			name:       "multimedia download",
			wantPath:   "/ts/cd/multimedia/sn78787878/0001/download",
			wantAccept: "*/*",
			call: func(ctx context.Context, c *Client) (*Response, error) {
				return c.MultimediaContent(ctx, serial, "0001", true)
			},
		},
		{
			name:       "last update JSON",
			wantPath:   "/ts/cd/caseupdate/info.json",
			wantQuery:  url.Values{"sn": {"78787878"}, "rn": {"3500038"}},
			wantAccept: "application/json",
			call: func(ctx context.Context, c *Client) (*Response, error) {
				return c.LastUpdate(ctx, []Identifier{serial, registration}, "json")
			},
		},
		{
			name:       "last update XML",
			wantPath:   "/ts/cd/caseupdate/info.xml",
			wantQuery:  url.Values{"sn": {"78787878"}},
			wantAccept: "application/xml",
			call: func(ctx context.Context, c *Client) (*Response, error) {
				return c.LastUpdate(ctx, []Identifier{serial}, "XML")
			},
		},
		{
			name:       "raw authenticated GET",
			wantPath:   "/future/endpoint",
			wantQuery:  url.Values{"one": {"a b"}, "many": {"x", "y"}},
			wantAccept: "application/vnd.uspto.test",
			call: func(ctx context.Context, c *Client) (*Response, error) {
				return c.RawGet(ctx, "/future/endpoint", url.Values{"one": {"a b"}, "many": {"x", "y"}}, "application/vnd.uspto.test", false)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotURL string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotURL = r.URL.String()
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", r.Method)
				}
				if r.URL.Path != tc.wantPath {
					t.Errorf("path = %q, want %q", r.URL.Path, tc.wantPath)
				}
				if got := r.URL.Query(); !equalURLValues(got, tc.wantQuery) {
					t.Errorf("query = %#v, want %#v", got, tc.wantQuery)
				}
				if got := r.Header.Get(APIKeyHeader); got != testTSDRAPIKey {
					t.Errorf("%s = %q, want %q", APIKeyHeader, got, testTSDRAPIKey)
				}
				if got := r.Header.Get("X-API-KEY"); got != "" {
					t.Errorf("ODP X-API-KEY header unexpectedly sent with value %q", got)
				}
				if got := r.Header.Get("Accept"); got != tc.wantAccept {
					t.Errorf("Accept = %q, want %q", got, tc.wantAccept)
				}
				w.Header().Set("Content-Type", "application/test")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("payload"))
			}))
			defer server.Close()

			c := newTSDRTestClient(server.URL)
			resp, err := tc.call(context.Background(), c)
			if err != nil {
				t.Fatalf("endpoint call unexpected error: %v", err)
			}
			if string(resp.Body) != "payload" || resp.ContentType != "application/test" {
				t.Errorf("response = %#v, want payload and application/test", resp)
			}
			if resp.URL != server.URL+gotURL {
				t.Errorf("Response.URL = %q, want %q", resp.URL, server.URL+gotURL)
			}
		})
	}
}

func equalURLValues(got, want url.Values) bool {
	if len(got) != len(want) {
		return false
	}
	for key, wantValues := range want {
		gotValues, ok := got[key]
		if !ok || strings.Join(gotValues, "\x00") != strings.Join(wantValues, "\x00") {
			return false
		}
	}
	return true
}

func TestClientRejectsUnsupportedFormatsBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	c := newTSDRTestClient(server.URL)
	id := mustIdentifier(t, "78787878", "")
	query := DocumentQuery{Identifiers: []Identifier{id}}

	tests := []struct {
		name string
		call func() error
		want string
	}{
		{name: "case asset", call: func() error { _, err := c.CaseAsset(context.Background(), id, "docx"); return err }, want: "unsupported case asset format"},
		{name: "document asset", call: func() error { _, err := c.DocumentsAsset(context.Background(), query, "tar"); return err }, want: "expected pdf or zip"},
		{name: "last update", call: func() error { _, err := c.LastUpdate(context.Background(), []Identifier{id}, "yaml"); return err }, want: "expected json or xml"},
		{name: "raw image serial validation", call: func() error { _, err := c.RawImage(context.Background(), "1234"); return err }, want: "expected 8 digits"},
		{name: "multi-case empty", call: func() error {
			_, err := c.MultiCaseStatus(context.Background(), IdentifierSerial, nil, false, false)
			return err
		}, want: "requires 1-25"},
		{name: "multi-case too many", call: func() error {
			_, err := c.MultiCaseStatus(context.Background(), IdentifierSerial, make([]string, 26), false, false)
			return err
		}, want: "requires 1-25"},
		{name: "multi-case auto type", call: func() error {
			_, err := c.MultiCaseStatus(context.Background(), IdentifierAuto, []string{"78787878"}, false, false)
			return err
		}, want: "supports serial"},
		{name: "multi-case proceeding type", call: func() error {
			_, err := c.MultiCaseStatus(context.Background(), IdentifierProceeding, []string{"1234567890"}, false, false)
			return err
		}, want: "supports serial"},
		{name: "multi-case unknown type", call: func() error {
			_, err := c.MultiCaseStatus(context.Background(), IdentifierType("bogus"), []string{"78787878"}, false, false)
			return err
		}, want: "supports serial"},
		{name: "empty document ID", call: func() error { _, err := c.DocumentInfoXML(context.Background(), id, " "); return err }, want: "unsafe document ID"},
		{name: "unsafe document ID metadata", call: func() error { _, err := c.DocumentInfoXML(context.Background(), id, "foo/bar"); return err }, want: "unsafe document ID"},
		{name: "unsafe document ID asset", call: func() error { _, err := c.DocumentAsset(context.Background(), id, "foo?bar", "pdf"); return err }, want: "unsafe document ID"},
		{name: "single document asset format", call: func() error { _, err := c.DocumentAsset(context.Background(), id, "DOC-123", "tar"); return err }, want: "expected pdf or zip"},
		{name: "document page zero", call: func() error { _, err := c.DocumentPage(context.Background(), id, "DOC-123", 0); return err }, want: "page must be >= 1"},
		{name: "document page unsafe ID", call: func() error { _, err := c.DocumentPage(context.Background(), id, "foo&bar", 1); return err }, want: "unsafe document ID"},
		{name: "empty case-map token", call: func() error { _, err := c.CaseMap(context.Background(), " "); return err }, want: "unsafe case-map token"},
		{name: "unsafe case-map token", call: func() error { _, err := c.CaseMap(context.Background(), "map/path"); return err }, want: "unsafe case-map token"},
		{name: "unsafe multimedia info sequence", call: func() error { _, err := c.MultimediaInfo(context.Background(), id, "one/two"); return err }, want: "invalid multimedia sequence"},
		{name: "empty multimedia content sequence", call: func() error { _, err := c.MultimediaContent(context.Background(), id, " ", false); return err }, want: "safe multimedia sequence"},
		{name: "unsafe multimedia content sequence", call: func() error { _, err := c.MultimediaContent(context.Background(), id, "one#two", false); return err }, want: "safe multimedia sequence"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("invalid inputs made %d HTTP requests, want 0", got)
	}
}

func TestRequestURLBuildsEncodedSameHostURL(t *testing.T) {
	c := NewClient("key", WithBaseURL("https://tsdr.example.test/"))
	got, err := c.RequestURL("/ts/cd/casedocs/bundle.xml", url.Values{
		"sn":   {"78787878,75757575"},
		"type": {"Office Action & Response"},
	})
	if err != nil {
		t.Fatalf("RequestURL() unexpected error: %v", err)
	}
	want := "https://tsdr.example.test/ts/cd/casedocs/bundle.xml?sn=78787878%2C75757575&type=Office+Action+%26+Response"
	if got != want {
		t.Fatalf("RequestURL() = %q, want %q", got, want)
	}
}

func TestRequestURLRejectsHostEscapes(t *testing.T) {
	c := NewClient("key", WithBaseURL("https://tsdrapi.uspto.gov"))
	for _, path := range []string{
		"",
		"relative/path",
		"//evil.example/steal",
		"https://evil.example/steal",
		"http://evil.example/steal",
		"/ts/swagger.json?x=1",
		"/ts/swagger.json#fragment",
	} {
		t.Run(fmt.Sprintf("path_%q", path), func(t *testing.T) {
			if got, err := c.RequestURL(path, nil); err == nil {
				t.Fatalf("RequestURL(%q) = %q, want containment error", path, got)
			}
		})
	}
}

func TestRedirectRetainsCredentialOnSameOrigin(t *testing.T) {
	var firstHeader, finalHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ts/cd/casestatus/sn78787878/info.xml":
			firstHeader = r.Header.Get(APIKeyHeader)
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			finalHeader = r.Header.Get(APIKeyHeader)
			_, _ = w.Write([]byte("ok"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := newTSDRTestClient(server.URL)
	_, err := c.CaseStatusXML(context.Background(), Identifier{Type: IdentifierSerial, Value: "78787878"})
	if err != nil {
		t.Fatalf("CaseStatusXML() unexpected error: %v", err)
	}
	if firstHeader != testTSDRAPIKey || finalHeader != testTSDRAPIKey {
		t.Fatalf("same-origin redirect headers = %q -> %q, want credential retained", firstHeader, finalHeader)
	}
}

func TestRedirectStripsCredentialOnCrossOrigin(t *testing.T) {
	var targetHeader string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHeader = r.Header.Get(APIKeyHeader)
		_, _ = w.Write([]byte("redirect target"))
	}))
	defer target.Close()

	var sourceHeader string
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceHeader = r.Header.Get(APIKeyHeader)
		http.Redirect(w, r, target.URL+"/asset", http.StatusFound)
	}))
	defer source.Close()

	c := newTSDRTestClient(source.URL)
	_, err := c.CaseStatusXML(context.Background(), Identifier{Type: IdentifierSerial, Value: "78787878"})
	if err != nil {
		t.Fatalf("CaseStatusXML() unexpected error: %v", err)
	}
	if sourceHeader != testTSDRAPIKey {
		t.Fatalf("source request %s = %q, want credential", APIKeyHeader, sourceHeader)
	}
	if targetHeader != "" {
		t.Fatalf("cross-origin redirect leaked %s = %q", APIKeyHeader, targetHeader)
	}
}

func TestClientReturnsStructuredAPIErrorAndBoundsBody(t *testing.T) {
	longBody := "  " + strings.Repeat("x", 1100) + "  "
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(longBody))
	}))
	defer server.Close()

	c := newTSDRTestClient(server.URL)
	_, err := c.RawGet(context.Background(), "/denied", nil, "", false)
	if err == nil {
		t.Fatal("RawGet() expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusForbidden || apiErr.Message != "Forbidden" {
		t.Errorf("APIError = %#v, want 403 Forbidden", apiErr)
	}
	if len(apiErr.Body) != 1003 || !strings.HasSuffix(apiErr.Body, "...") {
		t.Errorf("bounded APIError.Body length/suffix = %d/%q", len(apiErr.Body), apiErr.Body[len(apiErr.Body)-3:])
	}
	if got := apiErr.Error(); !strings.Contains(got, "TSDR API error (403): Forbidden -- ") {
		t.Errorf("APIError.Error() = %q", got)
	}

	withoutBody := (&APIError{StatusCode: 500, Message: "Internal Server Error"}).Error()
	if strings.Contains(withoutBody, "--") || !strings.Contains(withoutBody, "500") {
		t.Errorf("bodyless APIError.Error() = %q", withoutBody)
	}
}

func TestRetryAfterParsing(t *testing.T) {
	if got := retryAfter(""); got != 0 {
		t.Errorf("retryAfter(empty) = %v, want 0", got)
	}
	if got := retryAfter("17"); got != 17*time.Second {
		t.Errorf("retryAfter(seconds) = %v, want 17s", got)
	}
	if got := retryAfter("not-a-date"); got != 0 {
		t.Errorf("retryAfter(invalid) = %v, want 0", got)
	}

	future := time.Now().Add(2 * time.Minute).UTC().Truncate(time.Second)
	got := retryAfter(future.Format(http.TimeFormat))
	if got < 118*time.Second || got > 121*time.Second {
		t.Errorf("retryAfter(http-date) = %v, want approximately 2m", got)
	}
}

func TestRetryAfterAboveCapReturnsWithoutEarlyRetry(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limited"))
	}))
	defer server.Close()
	client := newTSDRTestClient(server.URL)
	started := time.Now()
	_, err := client.RawGet(context.Background(), "/limited", nil, "", false)
	if err == nil {
		t.Fatal("RawGet() expected a rate-limit error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("RawGet() error = %T %v, want 429 APIError", err, err)
	}
	if apiErr.RetryAfter != time.Minute {
		t.Fatalf("RetryAfter = %v, want 1m", apiErr.RetryAfter)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("over-cap Retry-After delayed return by %v", elapsed)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("server calls = %d, want no early retry", got)
	}
}

func TestRateLimitRetryWaitHonorsContextCancellation(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limited"))
	}))
	defer server.Close()
	c := newTSDRTestClient(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := c.RawGet(ctx, "/limited", nil, "", false)
	elapsed := time.Since(started)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("RawGet() error = %v, want context deadline exceeded", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("retry wait took %v after context cancellation, want under 500ms", elapsed)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("server calls = %d, want one request before cancellation", got)
	}
}

func TestTimestampHelpers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rate-limit-state")
	if got := readTimestamp(path); !got.IsZero() {
		t.Fatalf("readTimestamp(missing) = %v, want zero time", got)
	}

	want := time.Unix(123, 456789)
	writeTimestamp(path, want)
	if got := readTimestamp(path); !got.Equal(want) {
		t.Fatalf("timestamp round trip = %v, want %v", got, want)
	}

	if err := os.WriteFile(path, []byte("not-a-timestamp"), 0o600); err != nil {
		t.Fatalf("writing invalid timestamp fixture: %v", err)
	}
	if got := readTimestamp(path); !got.IsZero() {
		t.Fatalf("readTimestamp(invalid) = %v, want zero time", got)
	}
}

func TestSameOrigin(t *testing.T) {
	mustURL := func(raw string) *url.URL {
		t.Helper()
		got, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("url.Parse(%q): %v", raw, err)
		}
		return got
	}
	tests := []struct {
		name string
		a    *url.URL
		b    *url.URL
		want bool
	}{
		{name: "identical", a: mustURL("https://tsdrapi.uspto.gov/a"), b: mustURL("https://tsdrapi.uspto.gov/b"), want: true},
		{name: "case insensitive scheme and host", a: mustURL("HTTPS://TSDRAPI.USPTO.GOV/a"), b: mustURL("https://tsdrapi.uspto.gov/b"), want: true},
		{name: "different host", a: mustURL("https://tsdrapi.uspto.gov/a"), b: mustURL("https://tmng-al.uspto.gov/b")},
		{name: "different scheme", a: mustURL("http://tsdrapi.uspto.gov/a"), b: mustURL("https://tsdrapi.uspto.gov/b")},
		{name: "different port", a: mustURL("https://tsdrapi.uspto.gov:443/a"), b: mustURL("https://tsdrapi.uspto.gov:8443/b")},
		{name: "nil first", a: nil, b: mustURL("https://tsdrapi.uspto.gov/b")},
		{name: "nil second", a: mustURL("https://tsdrapi.uspto.gov/a"), b: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameOrigin(tc.a, tc.b); got != tc.want {
				t.Fatalf("sameOrigin(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestRetryableStatus(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   bool
	}{
		{status: http.StatusTooManyRequests, want: true},
		{status: http.StatusBadGateway, want: true},
		{status: http.StatusServiceUnavailable, want: true},
		{status: http.StatusGatewayTimeout, want: true},
		{status: http.StatusBadRequest, want: false},
		{status: http.StatusUnauthorized, want: false},
		{status: http.StatusForbidden, want: false},
		{status: http.StatusInternalServerError, want: true},
	} {
		if got := retryableStatus(tc.status); got != tc.want {
			t.Errorf("retryableStatus(%d) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestRateLimiterWaitCancellationAndDisable(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	disabled := &rateLimiter{enabled: false, lastMetadata: time.Now()}
	if err := disabled.wait(cancelled, false); err != nil {
		t.Fatalf("disabled rate limiter returned error: %v", err)
	}

	enabled := &rateLimiter{enabled: true, lastMetadata: time.Now()}
	if err := enabled.wait(cancelled, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("enabled rate limiter error = %v, want context canceled", err)
	}
}

func TestDocumentQueryEncodedQueryKeepsIdentifierFirst(t *testing.T) {
	id, err := ParseIdentifier("97238896", "serial")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := (DocumentQuery{
		Identifiers: []Identifier{id},
		Date:        "2026-07-30",
		FromDate:    "2026-01-01",
		ToDate:      "2026-12-31",
		Types:       []string{"RSI"},
		Category:    "I",
		Sort:        "date:D",
	}).EncodedQuery()
	if err != nil {
		t.Fatalf("EncodedQuery() error = %v", err)
	}
	want := "sn=97238896&date=2026-07-30&fromDate=2026-01-01&toDate=2026-12-31&type=RSI&category=I&sort=date%3AD"
	if encoded != want {
		t.Fatalf("EncodedQuery() = %q, want %q", encoded, want)
	}
}

func TestDocumentQueryEncodedQueryPreservesLiteralMultiIdentifierSeparators(t *testing.T) {
	first := mustIdentifier(t, "72131351", "serial")
	second := mustIdentifier(t, "76515878", "serial")
	encoded, err := (DocumentQuery{
		Identifiers: []Identifier{first, second},
		Types:       []string{"SPE", "OOA"},
	}).EncodedQuery()
	if err != nil {
		t.Fatalf("EncodedQuery() error = %v", err)
	}
	want := "sn=72131351,76515878&type=SPE%2COOA"
	if encoded != want {
		t.Fatalf("EncodedQuery() = %q, want %q", encoded, want)
	}
}

func TestDocumentQueryEncodedQueryDeduplicatesIdentifiers(t *testing.T) {
	id := mustIdentifier(t, "72131351", "serial")
	encoded, err := (DocumentQuery{Identifiers: []Identifier{id, id, id}}).EncodedQuery()
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "sn=72131351" {
		t.Fatalf("EncodedQuery() = %q, want one normalized identifier", encoded)
	}
}

func TestDocumentBundleRoutesUseLiteralMultiIdentifierSeparators(t *testing.T) {
	first := mustIdentifier(t, "72131351", "serial")
	second := mustIdentifier(t, "76515878", "serial")
	query := DocumentQuery{Identifiers: []Identifier{first, second}}

	tests := []struct {
		name string
		path string
		call func(context.Context, *Client) error
	}{
		{name: "list XML", path: "/ts/cd/casedocs/bundle.xml", call: func(ctx context.Context, client *Client) error {
			_, err := client.DocumentsXML(ctx, query)
			return err
		}},
		{name: "PDF", path: "/ts/cd/casedocs/bundle.pdf", call: func(ctx context.Context, client *Client) error {
			_, err := client.DocumentsAsset(ctx, query, "pdf")
			return err
		}},
		{name: "ZIP", path: "/ts/cd/casedocs/bundle.zip", call: func(ctx context.Context, client *Client) error {
			_, err := client.DocumentsAsset(ctx, query, "zip")
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.URL.Path != test.path {
					t.Errorf("path = %q, want %q", request.URL.Path, test.path)
				}
				if request.URL.RawQuery != "sn=72131351,76515878" {
					t.Errorf("raw query = %q, want literal comma separator", request.URL.RawQuery)
				}
				_, _ = w.Write([]byte("ok"))
			}))
			defer server.Close()
			if err := test.call(context.Background(), newTSDRTestClient(server.URL)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDocumentsXMLSendsIdentifierBeforeDateFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.RawQuery, "sn=97238896&date=2026-07-30"; got != want {
			t.Fatalf("raw query = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<DocumentList/>`)
	}))
	defer server.Close()

	id, _ := ParseIdentifier("97238896", "serial")
	client := NewClient("key", WithBaseURL(server.URL), WithoutRateLimit())
	if _, err := client.DocumentsXML(context.Background(), DocumentQuery{
		Identifiers: []Identifier{id},
		Date:        "2026-07-30",
	}); err != nil {
		t.Fatalf("DocumentsXML() error = %v", err)
	}
}

func TestRawRequestPOSTReplaysBodyAndPreservesHeaders(t *testing.T) {
	var calls atomic.Int32
	body := []byte(`{"case":true,"docs":"DOC-1"}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Query().Get("mode") != "selected" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		if r.Header.Get(APIKeyHeader) != testTSDRAPIKey {
			t.Errorf("missing TSDR key header")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		gotBody, _ := io.ReadAll(r.Body)
		if !bytes.Equal(gotBody, body) {
			t.Errorf("body on call %d = %q, want %q", call, gotBody, body)
		}
		if call == 1 {
			http.Error(w, "retry", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	client := newTSDRTestClient(server.URL)
	response, err := client.RawRequest(context.Background(), http.MethodPost, "/selected", url.Values{"mode": {"selected"}}, "application/json", "application/json", body, false)
	if err != nil {
		t.Fatalf("RawRequest() error = %v", err)
	}
	if calls.Load() != 2 || string(response.Body) != `{"ok":true}` {
		t.Fatalf("calls/body = %d/%s", calls.Load(), response.Body)
	}
}

func TestRawRequestValidationAndStreaming(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("streamed"))
	}))
	defer server.Close()
	client := newTSDRTestClient(server.URL)

	if _, err := client.RawRequest(context.Background(), http.MethodGet, "/bad", nil, "", "", []byte("body"), false); err == nil || !strings.Contains(err.Error(), "cannot include a body") {
		t.Fatalf("GET body error = %v", err)
	}
	if _, err := client.RawRequest(context.Background(), http.MethodPut, "/bad", nil, "", "", nil, false); err == nil || !strings.Contains(err.Error(), "GET or POST") {
		t.Fatalf("method error = %v", err)
	}
	if _, err := client.RawRequest(context.Background(), http.MethodPost, "/bad", nil, "", "", make([]byte, maxRawRequestBody+1), false); err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("oversize error = %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid requests reached server: %d", requests.Load())
	}

	stream, err := client.RawRequestStream(context.Background(), http.MethodPost, "/stream", nil, "*/*", "application/json", []byte(`{}`), false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(stream.Body)
	if err != nil {
		t.Fatal(err)
	}
	if stream.StatusCode != http.StatusPartialContent || string(data) != "streamed" {
		t.Fatalf("stream status/body = %d/%q", stream.StatusCode, data)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadLimiterReservesArtifactThenFreshAllRequestLane(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata")
	downloadPath := filepath.Join(dir, "download")
	now := time.Now()
	writeTimestamp(downloadPath, now)
	// Simulate a different process claiming the all-request lane shortly before
	// this process's artifact wait ends. A fixed timestamp avoids scheduler races
	// in a goroutine-based test while exercising the same shared-file behavior.
	externalClaim := now.Add(40 * time.Millisecond)
	writeTimestamp(metadataPath, externalClaim)
	limiter := &rateLimiter{
		enabled: true, metadataPath: metadataPath, downloadPath: downloadPath,
		lastDownload: now, metadataWait: 40 * time.Millisecond, downloadWait: 60 * time.Millisecond,
	}
	started := time.Now()
	if err := limiter.wait(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < 75*time.Millisecond {
		t.Fatalf("nested limiter elapsed %s, want artifact wait plus fresh metadata gap", elapsed)
	}
	if final := readTimestamp(metadataPath); final.Sub(externalClaim) < 35*time.Millisecond {
		t.Fatalf("all-request claim was not refreshed after artifact wait: external=%s final=%s", externalClaim, final)
	}
}

func TestConcurrentDownloadLimitersKeepArtifactSpacing(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata")
	downloadPath := filepath.Join(dir, "download")
	newLimiter := func() *rateLimiter {
		return &rateLimiter{
			enabled: true, metadataPath: metadataPath, downloadPath: downloadPath,
			metadataWait: 5 * time.Millisecond, downloadWait: 60 * time.Millisecond,
		}
	}
	limiters := []*rateLimiter{newLimiter(), newLimiter()}
	started := make(chan struct{})
	results := make(chan time.Time, 2)
	var group sync.WaitGroup
	for _, limiter := range limiters {
		group.Add(1)
		go func(limiter *rateLimiter) {
			defer group.Done()
			<-started
			if err := limiter.wait(context.Background(), true); err != nil {
				t.Errorf("wait() error: %v", err)
				return
			}
			results <- time.Now()
		}(limiter)
	}
	close(started)
	group.Wait()
	close(results)
	times := make([]time.Time, 0, 2)
	for value := range results {
		times = append(times, value)
	}
	if len(times) != 2 {
		t.Fatalf("completed downloads = %d", len(times))
	}
	if times[1].Before(times[0]) {
		times[0], times[1] = times[1], times[0]
	}
	if spacing := times[1].Sub(times[0]); spacing < 55*time.Millisecond {
		t.Fatalf("concurrent artifact spacing = %s, want at least 55ms", spacing)
	}
}

func TestPublicAssetURLTrustBoundary(t *testing.T) {
	for _, raw := range []string{
		"http://tmng-al.uspto.gov/file.pdf",
		"https://uspto.gov.evil.example/file.pdf",
		"https://evil.example/file.pdf",
		"https://user@tmng-al.uspto.gov/file.pdf",
	} {
		if _, err := parsePublicAssetURL(raw); err == nil {
			t.Errorf("parsePublicAssetURL(%q) accepted unsafe URL", raw)
		}
	}
	if _, err := parsePublicAssetURL("https://tmng-al.uspto.gov/file.pdf"); err != nil {
		t.Fatalf("official asset rejected: %v", err)
	}
}

func TestRedactTSDRURL(t *testing.T) {
	got := redactTSDRURL("https://tsdrapi.uspto.gov/ts/cd/casedocs/sn97054561/info.xml?docs=SECRET-DOC&case=true")
	if strings.Contains(got, "97054561") || strings.Contains(got, "SECRET-DOC") {
		t.Fatalf("redacted URL leaked values: %s", got)
	}
	if !strings.Contains(got, "sn%7Bid%7D") || !strings.Contains(got, "docs=%7Bredacted%7D") {
		t.Fatalf("redacted URL lost safe route/query shape: %s", got)
	}
}

func TestTSDRRedirectLoopIsBounded(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer server.Close()
	client := newTSDRTestClient(server.URL)
	if _, err := client.RawGet(context.Background(), "/loop", nil, "", false); err == nil || !strings.Contains(err.Error(), "10 TSDR redirects") {
		t.Fatalf("redirect loop error = %v", err)
	}
	if got := calls.Load(); got != 10 {
		t.Fatalf("redirect requests = %d, want 10", got)
	}
}
