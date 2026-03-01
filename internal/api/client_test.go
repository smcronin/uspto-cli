package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/smcronin/uspto-cli/internal/types"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newTestClient creates a Client whose baseURL points at the given test server.
// It uses a fresh rate limiter with no file-based state and a very short
// minRequestGap so tests don't sleep.
func newTestClient(ts *httptest.Server) *Client {
	c := &Client{
		httpClient: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		apiKey:  "test-api-key",
		baseURL: ts.URL,
		rl:      &rateLimiter{}, // fresh, no file state
	}
	return c
}

// ---------------------------------------------------------------------------
// 1. searchParams — builds query params from SearchOptions
// ---------------------------------------------------------------------------

func TestSearchParams(t *testing.T) {
	tests := []struct {
		name  string
		query string
		opts  types.SearchOptions
		want  map[string]string
	}{
		{
			name:  "empty query and options",
			query: "",
			opts:  types.SearchOptions{},
			want:  map[string]string{},
		},
		{
			name:  "query only",
			query: "widget",
			opts:  types.SearchOptions{},
			want:  map[string]string{"q": "widget"},
		},
		{
			name:  "all fields populated",
			query: "solar panel",
			opts: types.SearchOptions{
				Limit:   25,
				Offset:  50,
				Sort:    "filingDate desc",
				Fields:  "applicationNumberText,filingDate",
				Filters: "appStatus:patented",
				Facets:  "appStatus",
			},
			want: map[string]string{
				"q":       "solar panel",
				"limit":   "25",
				"offset":  "50",
				"sort":    "filingDate desc",
				"fields":  "applicationNumberText,filingDate",
				"filters": "appStatus:patented",
				"facets":  "appStatus",
			},
		},
		{
			name:  "zero limit and offset are omitted",
			query: "test",
			opts: types.SearchOptions{
				Limit:  0,
				Offset: 0,
				Sort:   "score desc",
			},
			want: map[string]string{
				"q":    "test",
				"sort": "score desc",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := searchParams(tc.query, tc.opts)
			if len(got) != len(tc.want) {
				t.Fatalf("searchParams() returned %d entries, want %d\n  got:  %v\n  want: %v",
					len(got), len(tc.want), got, tc.want)
			}
			for k, wantV := range tc.want {
				if gotV, ok := got[k]; !ok {
					t.Errorf("missing key %q in searchParams()", k)
				} else if gotV != wantV {
					t.Errorf("searchParams()[%q] = %q, want %q", k, gotV, wantV)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 2. setHeaders — sets API key and content type correctly
// ---------------------------------------------------------------------------

func TestSetHeaders(t *testing.T) {
	c := &Client{apiKey: "my-secret-key"}

	t.Run("GET request without body", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
		if err != nil {
			t.Fatal(err)
		}
		c.setHeaders(req)

		if got := req.Header.Get("X-API-KEY"); got != "my-secret-key" {
			t.Errorf("X-API-KEY = %q, want %q", got, "my-secret-key")
		}
		if got := req.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want %q", got, "application/json")
		}
		if got := req.Header.Get("Content-Type"); got != "" {
			t.Errorf("Content-Type should be empty for bodyless request, got %q", got)
		}
	})

	t.Run("POST request with body", func(t *testing.T) {
		body := strings.NewReader(`{"q":"test"}`)
		req, err := http.NewRequest(http.MethodPost, "http://example.com", body)
		if err != nil {
			t.Fatal(err)
		}
		c.setHeaders(req)

		if got := req.Header.Get("X-API-KEY"); got != "my-secret-key" {
			t.Errorf("X-API-KEY = %q, want %q", got, "my-secret-key")
		}
		if got := req.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want %q", got, "application/json")
		}
		if got := req.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}
	})
}

// ---------------------------------------------------------------------------
// 3. buildURL — all endpoint paths are correctly built
// ---------------------------------------------------------------------------

func TestBuildURL(t *testing.T) {
	c := &Client{baseURL: "https://api.uspto.gov"}

	tests := []struct {
		name   string
		path   string
		params map[string]string
		want   string
	}{
		{
			name:   "no params",
			path:   "/api/v1/patent/applications/search",
			params: nil,
			want:   "https://api.uspto.gov/api/v1/patent/applications/search",
		},
		{
			name:   "empty params map",
			path:   "/api/v1/patent/applications/search",
			params: map[string]string{},
			want:   "https://api.uspto.gov/api/v1/patent/applications/search",
		},
		{
			name:   "with query param",
			path:   "/api/v1/patent/applications/search",
			params: map[string]string{"q": "widget"},
			want:   "https://api.uspto.gov/api/v1/patent/applications/search?q=widget",
		},
		{
			name:   "empty value param is excluded",
			path:   "/api/v1/patent/applications/search",
			params: map[string]string{"q": "test", "sort": ""},
			want:   "https://api.uspto.gov/api/v1/patent/applications/search?q=test",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := c.buildURL(tc.path, tc.params)
			if got != tc.want {
				t.Errorf("buildURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestEndpointPaths verifies that the various endpoint methods build the
// correct URL path by inspecting the path received by the test server.
func TestEndpointPaths(t *testing.T) {
	tests := []struct {
		name     string
		call     func(c *Client)
		wantPath string
	}{
		{
			name:     "SearchPatents",
			call:     func(c *Client) { c.SearchPatents(context.Background(), "test", types.SearchOptions{}) },
			wantPath: "/api/v1/patent/applications/search",
		},
		{
			name:     "GetApplication",
			call:     func(c *Client) { c.GetApplication(context.Background(), "16123456") },
			wantPath: "/api/v1/patent/applications/16123456",
		},
		{
			name:     "GetMetadata",
			call:     func(c *Client) { c.GetMetadata(context.Background(), "16123456") },
			wantPath: "/api/v1/patent/applications/16123456/meta-data",
		},
		{
			name:     "GetAdjustment",
			call:     func(c *Client) { c.GetAdjustment(context.Background(), "16123456") },
			wantPath: "/api/v1/patent/applications/16123456/adjustment",
		},
		{
			name:     "GetAssignment",
			call:     func(c *Client) { c.GetAssignment(context.Background(), "16123456") },
			wantPath: "/api/v1/patent/applications/16123456/assignment",
		},
		{
			name:     "GetAttorney",
			call:     func(c *Client) { c.GetAttorney(context.Background(), "16123456") },
			wantPath: "/api/v1/patent/applications/16123456/attorney",
		},
		{
			name:     "GetContinuity",
			call:     func(c *Client) { c.GetContinuity(context.Background(), "16123456") },
			wantPath: "/api/v1/patent/applications/16123456/continuity",
		},
		{
			name:     "GetForeignPriority",
			call:     func(c *Client) { c.GetForeignPriority(context.Background(), "16123456") },
			wantPath: "/api/v1/patent/applications/16123456/foreign-priority",
		},
		{
			name:     "GetTransactions",
			call:     func(c *Client) { c.GetTransactions(context.Background(), "16123456") },
			wantPath: "/api/v1/patent/applications/16123456/transactions",
		},
		{
			name:     "GetAssociatedDocuments",
			call:     func(c *Client) { c.GetAssociatedDocuments(context.Background(), "16123456") },
			wantPath: "/api/v1/patent/applications/16123456/associated-documents",
		},
		{
			name:     "SearchBulkData",
			call:     func(c *Client) { c.SearchBulkData(context.Background(), "grant", types.SearchOptions{}) },
			wantPath: "/api/v1/datasets/products/search",
		},
		{
			name:     "SearchProceedings",
			call:     func(c *Client) { c.SearchProceedings(context.Background(), "IPR", types.SearchOptions{}) },
			wantPath: "/api/v1/patent/trials/proceedings/search",
		},
		{
			name:     "SearchDecisions",
			call:     func(c *Client) { c.SearchDecisions(context.Background(), "claim", types.SearchOptions{}) },
			wantPath: "/api/v1/patent/trials/decisions/search",
		},
		{
			name:     "SearchAppeals",
			call:     func(c *Client) { c.SearchAppeals(context.Background(), "obviousness", types.SearchOptions{}) },
			wantPath: "/api/v1/patent/appeals/decisions/search",
		},
		{
			name:     "SearchInterferences",
			call:     func(c *Client) { c.SearchInterferences(context.Background(), "priority", types.SearchOptions{}) },
			wantPath: "/api/v1/patent/interferences/decisions/search",
		},
		{
			name:     "SearchPetitionDecisions",
			call:     func(c *Client) { c.SearchPetitionDecisions(context.Background(), "revival", types.SearchOptions{}) },
			wantPath: "/api/v1/petition/decisions/search",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				// Return a minimal valid JSON response with the appropriate bag.
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"count":0,"patentFileWrapperDataBag":[],"bulkDataProductBag":[],"proceedingDataBag":[],"trialDocumentDataBag":[],"appealDecisionDataBag":[],"interferenceDecisionDataBag":[],"petitionDecisionDataBag":[]}`))
			}))
			defer ts.Close()

			c := newTestClient(ts)
			tc.call(c)

			if gotPath != tc.wantPath {
				t.Errorf("endpoint path = %q, want %q", gotPath, tc.wantPath)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 4. DownloadBulkFile — looks up fileDownloadURI, fails on missing file
// ---------------------------------------------------------------------------

func TestDownloadBulkFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test-download.zip")

	fileContent := "fake-zip-content-here"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/datasets/products/PROD-123":
			// Return product with file listing.
			resp := types.BulkDataResponse{
				Count: 1,
				BulkDataProductBag: []types.BulkDataProduct{
					{
						ProductIdentifier: "PROD-123",
						ProductFileBag: types.BulkDataFileBag{
							Count: 1,
							FileDataBag: []types.BulkFileData{
								{
									FileName:        "grant-2024.zip",
									FileDownloadURI: "/download/2024/grant-2024.zip",
								},
							},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case "/download/2024/grant-2024.zip":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write([]byte(fileContent))

		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts)
	result, err := c.DownloadBulkFile(context.Background(), "PROD-123", "grant-2024.zip", outputPath)
	if err != nil {
		t.Fatalf("DownloadBulkFile() unexpected error: %v", err)
	}
	if result != outputPath {
		t.Errorf("DownloadBulkFile() returned path %q, want %q", result, outputPath)
	}

	// Verify the file was written correctly.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(data) != fileContent {
		t.Errorf("downloaded file content = %q, want %q", string(data), fileContent)
	}
}

func TestDownloadBulkFile_FileNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := types.BulkDataResponse{
			Count: 1,
			BulkDataProductBag: []types.BulkDataProduct{
				{
					ProductIdentifier: "PROD-123",
					ProductFileBag: types.BulkDataFileBag{
						Count: 1,
						FileDataBag: []types.BulkFileData{
							{
								FileName:        "other-file.zip",
								FileDownloadURI: "/download/other-file.zip",
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.DownloadBulkFile(context.Background(), "PROD-123", "nonexistent.zip", "/tmp/out.zip")
	if err == nil {
		t.Fatal("DownloadBulkFile() expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error message %q should contain 'not found'", err.Error())
	}
}

func TestDownloadBulkFile_EmptyDownloadURI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := types.BulkDataResponse{
			Count: 1,
			BulkDataProductBag: []types.BulkDataProduct{
				{
					ProductIdentifier: "PROD-123",
					ProductFileBag: types.BulkDataFileBag{
						Count: 1,
						FileDataBag: []types.BulkFileData{
							{
								FileName:        "target.zip",
								FileDownloadURI: "", // empty URI
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.DownloadBulkFile(context.Background(), "PROD-123", "target.zip", "/tmp/out.zip")
	if err == nil {
		t.Fatal("DownloadBulkFile() expected error for empty download URI, got nil")
	}
	if !strings.Contains(err.Error(), "no download URI") {
		t.Errorf("error message %q should contain 'no download URI'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 5. FetchGrantXML — follows redirect chain, handles missing grant XML
// ---------------------------------------------------------------------------

func TestFetchGrantXML_Success(t *testing.T) {
	xmlContent := `<?xml version="1.0"?><patent-grant><title>Widget</title></patent-grant>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/patent/applications/16123456/associated-documents":
			resp := types.PatentDataResponse{
				Count: 1,
				PatentFileWrapperDataBag: []types.PatentFileWrapper{
					{
						ApplicationNumberText: "16123456",
						GrantDocumentMetaData: &types.FileMetaData{
							FileLocationURI: "/grant-xml/2024/US12345678.xml",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case "/grant-xml/2024/US12345678.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(xmlContent))

		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts)
	got, err := c.FetchGrantXML(context.Background(), "16123456")
	if err != nil {
		t.Fatalf("FetchGrantXML() unexpected error: %v", err)
	}
	if string(got) != xmlContent {
		t.Errorf("FetchGrantXML() = %q, want %q", string(got), xmlContent)
	}
}

func TestFetchGrantXML_FollowsRedirect(t *testing.T) {
	xmlContent := `<patent-grant><title>Redirected</title></patent-grant>`

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/patent/applications/16123456/associated-documents":
			resp := types.PatentDataResponse{
				Count: 1,
				PatentFileWrapperDataBag: []types.PatentFileWrapper{
					{
						ApplicationNumberText: "16123456",
						GrantDocumentMetaData: &types.FileMetaData{
							FileLocationURI: "/grant-xml/initial",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case "/grant-xml/initial":
			// Redirect to the final location (absolute URL required by followRedirects).
			w.Header().Set("Location", ts.URL+"/grant-xml/final")
			w.WriteHeader(http.StatusFound)

		case "/grant-xml/final":
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(xmlContent))

		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts)
	got, err := c.FetchGrantXML(context.Background(), "16123456")
	if err != nil {
		t.Fatalf("FetchGrantXML() unexpected error: %v", err)
	}
	if string(got) != xmlContent {
		t.Errorf("FetchGrantXML() after redirect = %q, want %q", string(got), xmlContent)
	}
}

func TestFetchGrantXML_NoAssociatedDocuments(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := types.PatentDataResponse{
			Count:                    0,
			PatentFileWrapperDataBag: []types.PatentFileWrapper{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.FetchGrantXML(context.Background(), "16123456")
	if err == nil {
		t.Fatal("FetchGrantXML() expected error for no associated docs, got nil")
	}
	if !strings.Contains(err.Error(), "no associated documents") {
		t.Errorf("error = %q, want it to contain 'no associated documents'", err.Error())
	}
}

func TestFetchGrantXML_NoGrantMetaData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := types.PatentDataResponse{
			Count: 1,
			PatentFileWrapperDataBag: []types.PatentFileWrapper{
				{
					ApplicationNumberText: "16123456",
					GrantDocumentMetaData: nil, // not granted yet
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.FetchGrantXML(context.Background(), "16123456")
	if err == nil {
		t.Fatal("FetchGrantXML() expected error for nil grant metadata, got nil")
	}
	if !strings.Contains(err.Error(), "no grant XML available") {
		t.Errorf("error = %q, want it to contain 'no grant XML available'", err.Error())
	}
}

func TestFetchGrantXML_EmptyFileLocationURI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := types.PatentDataResponse{
			Count: 1,
			PatentFileWrapperDataBag: []types.PatentFileWrapper{
				{
					ApplicationNumberText: "16123456",
					GrantDocumentMetaData: &types.FileMetaData{
						FileLocationURI: "", // empty
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.FetchGrantXML(context.Background(), "16123456")
	if err == nil {
		t.Fatal("FetchGrantXML() expected error for empty FileLocationURI, got nil")
	}
	if !strings.Contains(err.Error(), "no grant XML available") {
		t.Errorf("error = %q, want it to contain 'no grant XML available'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 6. Rate limit / retry logic — 429 responses trigger retry
// ---------------------------------------------------------------------------

func TestRequest_RetryOn429(t *testing.T) {
	var callCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n <= 2 {
			// First two calls return 429.
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		// Third call succeeds.
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":0,"patentFileWrapperDataBag":[]}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.request(context.Background(), http.MethodGet,
		"/api/v1/patent/applications/search", nil, map[string]string{"q": "test"})
	if err != nil {
		t.Fatalf("request() after 429 retries should succeed, got error: %v", err)
	}

	finalCount := atomic.LoadInt32(&callCount)
	if finalCount != 3 {
		t.Errorf("expected 3 total requests (2x429 + 1x200), got %d", finalCount)
	}
}

func TestRequest_429ExhaustsRetries(t *testing.T) {
	var callCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.request(context.Background(), http.MethodGet,
		"/api/v1/patent/applications/search", nil, map[string]string{"q": "test"})
	if err == nil {
		t.Fatal("request() should fail after exhausting retries on 429")
	}

	apiErr, ok := err.(*UsptoAPIError)
	if !ok {
		t.Fatalf("expected *UsptoAPIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 429 {
		t.Errorf("UsptoAPIError.StatusCode = %d, want 429", apiErr.StatusCode)
	}

	// Should have been called maxRetries+1 times (initial + 3 retries).
	finalCount := atomic.LoadInt32(&callCount)
	if finalCount != int32(maxRetries+1) {
		t.Errorf("expected %d total requests, got %d", maxRetries+1, finalCount)
	}
}

// ---------------------------------------------------------------------------
// 7. Error handling — proper UsptoAPIError creation from bad status codes
// ---------------------------------------------------------------------------

func TestRequest_UsptoAPIError_FromBadStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantMsg    string
	}{
		{
			name:       "404 with error field",
			statusCode: 404,
			body:       `{"error":"Not Found","errorDetails":"Application not in database"}`,
			wantMsg:    "Not Found -- Application not in database",
		},
		{
			name:       "500 with message field",
			statusCode: 500,
			body:       `{"message":"Internal Server Error","detailedMessage":"Database timeout"}`,
			wantMsg:    "Internal Server Error -- Database timeout",
		},
		{
			name:       "403 with non-JSON body",
			statusCode: 403,
			body:       `Access Denied`,
			wantMsg:    "Forbidden", // falls back to http.StatusText
		},
		{
			name:       "400 with empty JSON",
			statusCode: 400,
			body:       `{}`,
			wantMsg:    "Bad Request", // no error/message fields => http.StatusText
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.body))
			}))
			defer ts.Close()

			c := newTestClient(ts)
			_, err := c.request(context.Background(), http.MethodGet, "/test", nil, nil)
			if err == nil {
				t.Fatal("request() should return error for non-2xx status")
			}

			apiErr, ok := err.(*UsptoAPIError)
			if !ok {
				t.Fatalf("expected *UsptoAPIError, got %T: %v", err, err)
			}
			if apiErr.StatusCode != tc.statusCode {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tc.statusCode)
			}
			if apiErr.Message != tc.wantMsg {
				t.Errorf("Message = %q, want %q", apiErr.Message, tc.wantMsg)
			}
			if apiErr.Body != tc.body {
				t.Errorf("Body = %q, want %q", apiErr.Body, tc.body)
			}
		})
	}
}

func TestUsptoAPIError_Error(t *testing.T) {
	t.Run("with body", func(t *testing.T) {
		e := &UsptoAPIError{StatusCode: 404, Message: "Not Found", Body: `{"detail":"gone"}`}
		got := e.Error()
		if !strings.Contains(got, "404") || !strings.Contains(got, "Not Found") || !strings.Contains(got, `{"detail":"gone"}`) {
			t.Errorf("Error() = %q, want it to contain status code, message, and body", got)
		}
	})

	t.Run("without body", func(t *testing.T) {
		e := &UsptoAPIError{StatusCode: 500, Message: "Server Error", Body: ""}
		got := e.Error()
		if !strings.Contains(got, "500") || !strings.Contains(got, "Server Error") {
			t.Errorf("Error() = %q, want it to contain status code and message", got)
		}
		if strings.Contains(got, "--") {
			t.Errorf("Error() = %q, should not contain '--' separator when body is empty", got)
		}
	})
}

func TestUsptoAPIError_IsRetryable(t *testing.T) {
	tests := []struct {
		statusCode int
		want       bool
	}{
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
	}
	for _, tc := range tests {
		e := &UsptoAPIError{StatusCode: tc.statusCode}
		if got := e.IsRetryable(); got != tc.want {
			t.Errorf("IsRetryable() for %d = %v, want %v", tc.statusCode, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// followRedirects — follows 301/302 and re-applies headers
// ---------------------------------------------------------------------------

func TestFollowRedirects(t *testing.T) {
	var requestCount int32
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requestCount, 1)

		// Verify API key is present on every hop.
		if got := r.Header.Get("X-API-KEY"); got != "test-api-key" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("missing API key on redirect hop"))
			return
		}

		switch {
		case n == 1:
			w.Header().Set("Location", ts.URL+"/hop2")
			w.WriteHeader(http.StatusFound)
		case n == 2:
			w.Header().Set("Location", ts.URL+"/hop3")
			w.WriteHeader(http.StatusMovedPermanently)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"result":"final"}`))
		}
	}))
	defer ts.Close()

	c := newTestClient(ts)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	c.setHeaders(req)

	resp, err := c.followRedirects(req)
	if err != nil {
		t.Fatalf("followRedirects() error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("final status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "final") {
		t.Errorf("final body = %q, want it to contain 'final'", string(body))
	}

	if count := atomic.LoadInt32(&requestCount); count != 3 {
		t.Errorf("redirect hop count = %d, want 3", count)
	}
}

func TestFollowRedirects_TooManyRedirects(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always redirect — should eventually hit the limit.
		w.Header().Set("Location", ts.URL+"/loop")
		w.WriteHeader(http.StatusFound)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	c.setHeaders(req)

	_, err = c.followRedirects(req)
	if err == nil {
		t.Fatal("followRedirects() should fail on redirect loop")
	}
	if !strings.Contains(err.Error(), "too many redirects") {
		t.Errorf("error = %q, want it to contain 'too many redirects'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// request — verifies headers are sent and JSON body is marshalled
// ---------------------------------------------------------------------------

func TestRequest_SendsAPIKeyHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-KEY"); got != "test-api-key" {
			t.Errorf("X-API-KEY = %q, want %q", got, "test-api-key")
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want %q", got, "application/json")
		}
		w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.request(context.Background(), http.MethodGet, "/test", nil, nil)
	if err != nil {
		t.Fatalf("request() error: %v", err)
	}
}

func TestRequest_PostWithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("request body is not valid JSON: %v", err)
		}
		if q, ok := parsed["q"].(string); !ok || q != "solar" {
			t.Errorf("body q = %v, want 'solar'", parsed["q"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":0,"patentFileWrapperDataBag":[]}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	searchReq := types.SearchRequest{Q: "solar"}
	_, err := c.request(context.Background(), http.MethodPost,
		"/api/v1/patent/applications/search", searchReq, nil)
	if err != nil {
		t.Fatalf("request() error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// NewClient — option funcs
// ---------------------------------------------------------------------------

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient("key123")
	if c.apiKey != "key123" {
		t.Errorf("apiKey = %q, want %q", c.apiKey, "key123")
	}
	if c.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, DefaultBaseURL)
	}
	if c.debug {
		t.Error("debug should default to false")
	}
	if c.timeout != defaultTimeout {
		t.Errorf("timeout = %v, want %v", c.timeout, defaultTimeout)
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	c := NewClient("key456",
		WithBaseURL("https://custom.api.com/"),
		WithDebug(true),
		WithTimeout(60*1e9), // 60s as time.Duration
	)
	if c.baseURL != "https://custom.api.com" {
		t.Errorf("baseURL = %q, want trailing slash stripped", c.baseURL)
	}
	if !c.debug {
		t.Error("debug should be true")
	}
}

// ---------------------------------------------------------------------------
// truncate helper
// ---------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
	}
	for _, tc := range tests {
		got := truncate(tc.input, tc.n)
		if got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.input, tc.n, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// DownloadDocument — basic success path and error status
// ---------------------------------------------------------------------------

func TestDownloadDocument_Success(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "doc.pdf")
	fileContent := "fake-pdf-bytes"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-KEY"); got != "test-api-key" {
			t.Errorf("download request missing API key")
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Write([]byte(fileContent))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	result, err := c.DownloadDocument(context.Background(), ts.URL+"/files/doc.pdf", outputPath)
	if err != nil {
		t.Fatalf("DownloadDocument() error: %v", err)
	}
	if result != outputPath {
		t.Errorf("result = %q, want %q", result, outputPath)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if string(data) != fileContent {
		t.Errorf("file content = %q, want %q", string(data), fileContent)
	}
}

func TestDownloadDocument_RelativeURL(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "doc.pdf")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/doc.pdf" {
			t.Errorf("path = %q, want /files/doc.pdf", r.URL.Path)
		}
		w.Write([]byte("content"))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.DownloadDocument(context.Background(), "/files/doc.pdf", outputPath)
	if err != nil {
		t.Fatalf("DownloadDocument() with relative URL error: %v", err)
	}
}

func TestDownloadDocument_ErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("file not found"))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.DownloadDocument(context.Background(), ts.URL+"/missing.pdf", "/tmp/out.pdf")
	if err == nil {
		t.Fatal("DownloadDocument() expected error for 404, got nil")
	}

	apiErr, ok := err.(*UsptoAPIError)
	if !ok {
		t.Fatalf("expected *UsptoAPIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// DryRunURL
// ---------------------------------------------------------------------------

func TestDryRunURL(t *testing.T) {
	c := &Client{baseURL: "https://api.uspto.gov"}

	got := c.DryRunURL("GET", "/api/v1/patent/applications/search", "widget",
		types.SearchOptions{Limit: 10, Offset: 20, Sort: "filingDate desc"})

	if !strings.HasPrefix(got, "GET https://api.uspto.gov/api/v1/patent/applications/search?") {
		t.Errorf("DryRunURL() = %q, want it to start with 'GET https://api.uspto.gov/api/v1/patent/applications/search?'", got)
	}
	if !strings.Contains(got, "q=widget") {
		t.Errorf("DryRunURL() = %q, want it to contain 'q=widget'", got)
	}
	if !strings.Contains(got, "limit=10") {
		t.Errorf("DryRunURL() = %q, want it to contain 'limit=10'", got)
	}
	if !strings.Contains(got, "offset=20") {
		t.Errorf("DryRunURL() = %q, want it to contain 'offset=20'", got)
	}
}
