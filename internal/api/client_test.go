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
	"time"

	"github.com/smcronin/uspto-cli/internal/types"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newTestClient creates a Client whose baseURL points at the given test server.
// It uses a fresh rate limiter with no file-based state and millisecond retry
// waits so tests don't sleep.
func newTestClient(ts *httptest.Server) *Client {
	c := &Client{
		httpClient: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		apiKey:        "test-api-key",
		baseURL:       ts.URL,
		rl:            &rateLimiter{}, // fresh, no file state
		rateLimitWait: 5 * time.Millisecond,
		backoffStep:   time.Millisecond,
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
	c := &Client{apiKey: "my-secret-key", baseURL: "http://example.com"}

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

	t.Run("API key goes only to the API origin", func(t *testing.T) {
		tests := []struct {
			name    string
			baseURL string
			target  string
			wantKey bool
		}{
			{name: "same origin", baseURL: "https://api.uspto.gov", target: "https://api.uspto.gov/api/v1/download/x.pdf", wantKey: true},
			{name: "same origin, explicit default port", baseURL: "https://api.uspto.gov", target: "https://api.uspto.gov:443/x", wantKey: true},
			{name: "same origin, host case differs", baseURL: "https://api.uspto.gov", target: "https://API.USPTO.GOV/x", wantKey: true},
			{name: "signed storage host", baseURL: "https://api.uspto.gov", target: "https://bucket.s3.amazonaws.com/x.pdf?X-Amz-Signature=abc", wantKey: false},
			{name: "sibling uspto host", baseURL: "https://api.uspto.gov", target: "https://data.uspto.gov/x", wantKey: false},
			{name: "scheme downgrade", baseURL: "https://api.uspto.gov", target: "http://api.uspto.gov/x", wantKey: false},
			{name: "different port", baseURL: "https://api.uspto.gov", target: "https://api.uspto.gov:8443/x", wantKey: false},
			{name: "lookalike suffix", baseURL: "https://api.uspto.gov", target: "https://api.uspto.gov.example.com/x", wantKey: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				client := &Client{apiKey: "my-secret-key", baseURL: tt.baseURL}
				req, err := http.NewRequest(http.MethodGet, tt.target, nil)
				if err != nil {
					t.Fatal(err)
				}
				client.setHeaders(req)
				if got := req.Header.Get("X-API-KEY") != ""; got != tt.wantKey {
					t.Errorf("X-API-KEY sent = %v, want %v", got, tt.wantKey)
				}
				if got := req.Header.Get("Accept"); got != "application/json" {
					t.Errorf("Accept = %q, want application/json on every request", got)
				}
			})
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
			name: "DownloadPatentsPost",
			call: func(c *Client) {
				_, _ = c.DownloadPatentsPost(context.Background(), types.SearchRequest{
					Q: "battery",
					Pagination: &types.Pagination{
						Limit:  10,
						Offset: 0,
					},
				}, "csv")
			},
			wantPath: "/api/v1/patent/applications/search/download",
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
		{
			name: "SearchPetitionDecisionsPost",
			call: func(c *Client) {
				_, _ = c.SearchPetitionDecisionsPost(context.Background(), types.SearchRequest{
					Q: "revival",
					Pagination: &types.Pagination{
						Offset: 0,
						Limit:  5,
					},
				})
			},
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

func TestFetchPgpubXML_Success(t *testing.T) {
	xmlContent := `<?xml version="1.0"?><us-patent-application><title>Widget App</title></us-patent-application>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/patent/applications/16123456/associated-documents":
			resp := types.PatentDataResponse{
				Count: 1,
				PatentFileWrapperDataBag: []types.PatentFileWrapper{
					{
						ApplicationNumberText: "16123456",
						PgpubDocumentMetaData: &types.FileMetaData{
							FileLocationURI: "/pgpub-xml/2024/US20240123456A1.xml",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		case "/pgpub-xml/2024/US20240123456A1.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(xmlContent))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts)
	got, err := c.FetchPgpubXML(context.Background(), "16123456")
	if err != nil {
		t.Fatalf("FetchPgpubXML() unexpected error: %v", err)
	}
	if string(got) != xmlContent {
		t.Errorf("FetchPgpubXML() = %q, want %q", string(got), xmlContent)
	}
}

func TestFetchPatentXML_FallbackToPgpub(t *testing.T) {
	pgpubXML := `<us-patent-application><abstract><p>pending</p></abstract></us-patent-application>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/patent/applications/16123456/associated-documents":
			resp := types.PatentDataResponse{
				Count: 1,
				PatentFileWrapperDataBag: []types.PatentFileWrapper{
					{
						ApplicationNumberText: "16123456",
						GrantDocumentMetaData: nil,
						PgpubDocumentMetaData: &types.FileMetaData{
							FileLocationURI: "/pgpub-xml/pending.xml",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		case "/pgpub-xml/pending.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(pgpubXML))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts)
	got, source, err := c.FetchPatentXML(context.Background(), "16123456")
	if err != nil {
		t.Fatalf("FetchPatentXML() unexpected error: %v", err)
	}
	if source != "pgpub" {
		t.Fatalf("source = %q, want pgpub", source)
	}
	if string(got) != pgpubXML {
		t.Fatalf("FetchPatentXML() = %q, want %q", string(got), pgpubXML)
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

// ---------------------------------------------------------------------------
// followRedirects — follows 301/302 and re-applies headers
// ---------------------------------------------------------------------------

func TestFollowRedirects(t *testing.T) {
	var requestCount int32
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requestCount, 1)

		// Every hop here stays on the API origin, so every hop carries the key.
		// Hops to another host must not; see the cross-origin tests below.
		if got := r.Header.Get("X-API-KEY"); got != "test-api-key" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("missing API key on same-origin redirect hop"))
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

func TestDownloadPatentsPost_SendsFormatInBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/patent/applications/search/download" {
			t.Fatalf("path = %s, want /api/v1/patent/applications/search/download", r.URL.Path)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got, _ := body["q"].(string); got != "solid state battery" {
			t.Fatalf("q = %q, want %q", got, "solid state battery")
		}
		if got, _ := body["format"].(string); got != "csv" {
			t.Fatalf("format = %q, want %q", got, "csv")
		}

		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte("applicationNumberText\n16123456\n"))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.DownloadPatentsPost(context.Background(), types.SearchRequest{
		Q: "solid state battery",
		Pagination: &types.Pagination{
			Limit:  25,
			Offset: 0,
		},
	}, "csv")
	if err != nil {
		t.Fatalf("DownloadPatentsPost() error: %v", err)
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
// API key scope: redirects and absolute URLs on other hosts
// ---------------------------------------------------------------------------

// keyRecorder is a second test server standing in for signed file storage.
// It records the X-API-KEY header of every request it receives.
type keyRecorder struct {
	*httptest.Server
	keys []string
}

func newKeyRecorder(t *testing.T, body string) *keyRecorder {
	t.Helper()
	rec := &keyRecorder{}
	rec.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.keys = append(rec.keys, r.Header.Get("X-API-KEY"))
		w.Write([]byte(body))
	}))
	t.Cleanup(rec.Close)
	return rec
}

func (rec *keyRecorder) assertNoKey(t *testing.T, wantRequests int) {
	t.Helper()
	if len(rec.keys) != wantRequests {
		t.Fatalf("storage host received %d requests, want %d", len(rec.keys), wantRequests)
	}
	for i, key := range rec.keys {
		if key != "" {
			t.Errorf("storage request %d carried X-API-KEY; the key must stay on the API origin", i+1)
		}
	}
}

func TestDownloadDocument_CrossOriginRedirectDropsAPIKey(t *testing.T) {
	storage := newKeyRecorder(t, "file-bytes")

	var apiKeys []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKeys = append(apiKeys, r.Header.Get("X-API-KEY"))
		w.Header().Set("Location", storage.URL+"/signed/doc.pdf?X-Amz-Signature=SECRETSIG")
		w.WriteHeader(http.StatusFound)
	}))
	defer api.Close()

	c := newTestClient(api)
	outputPath := filepath.Join(t.TempDir(), "doc.pdf")
	if _, err := c.DownloadDocument(context.Background(), "/api/v1/download/doc.pdf", outputPath); err != nil {
		t.Fatalf("DownloadDocument() error: %v", err)
	}

	if len(apiKeys) != 1 || apiKeys[0] != "test-api-key" {
		t.Errorf("API origin keys = %q, want one request carrying the key", apiKeys)
	}
	storage.assertNoKey(t, 1)
	if data, _ := os.ReadFile(outputPath); string(data) != "file-bytes" {
		t.Errorf("downloaded content = %q, want %q", data, "file-bytes")
	}
}

func TestAbsoluteURLOnOtherHostGetsNoAPIKey(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to the API origin: %s", r.URL.Path)
	}))
	defer api.Close()

	t.Run("DownloadDocument", func(t *testing.T) {
		storage := newKeyRecorder(t, "pdf")
		c := newTestClient(api)
		if _, err := c.DownloadDocument(context.Background(), storage.URL+"/doc.pdf", filepath.Join(t.TempDir(), "doc.pdf")); err != nil {
			t.Fatalf("DownloadDocument() error: %v", err)
		}
		storage.assertNoKey(t, 1)
	})

	t.Run("FetchDocumentBytes", func(t *testing.T) {
		storage := newKeyRecorder(t, "docx")
		c := newTestClient(api)
		if _, err := c.FetchDocumentBytes(context.Background(), storage.URL+"/doc.docx"); err != nil {
			t.Fatalf("FetchDocumentBytes() error: %v", err)
		}
		storage.assertNoKey(t, 1)
	})

	t.Run("fetchXMLByURI", func(t *testing.T) {
		storage := newKeyRecorder(t, "<us-patent-grant/>")
		c := newTestClient(api)
		if _, err := c.fetchXMLByURI(context.Background(), storage.URL+"/grant.xml", "grant"); err != nil {
			t.Fatalf("fetchXMLByURI() error: %v", err)
		}
		storage.assertNoKey(t, 1)
	})
}

func TestFollowRedirects_KeyReturnsOnlyForAPIOrigin(t *testing.T) {
	// API -> storage -> API: the key is withheld from the storage hop and
	// sent again when the chain comes back to the API origin.
	var api *httptest.Server
	var storageKeys, apiKeys []string
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		storageKeys = append(storageKeys, r.Header.Get("X-API-KEY"))
		w.Header().Set("Location", api.URL+"/final")
		w.WriteHeader(http.StatusFound)
	}))
	defer storage.Close()
	api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKeys = append(apiKeys, r.Header.Get("X-API-KEY"))
		if r.URL.Path == "/final" {
			w.Write([]byte("done"))
			return
		}
		w.Header().Set("Location", storage.URL+"/hop")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer api.Close()

	c := newTestClient(api)
	req, err := http.NewRequest(http.MethodGet, api.URL+"/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	c.setHeaders(req)
	resp, err := c.followRedirects(req)
	if err != nil {
		t.Fatalf("followRedirects() error: %v", err)
	}
	resp.Body.Close()

	if len(storageKeys) != 1 || storageKeys[0] != "" {
		t.Errorf("storage hop keys = %q, want one request without the key", storageKeys)
	}
	if len(apiKeys) != 2 || apiKeys[0] != "test-api-key" || apiKeys[1] != "test-api-key" {
		t.Errorf("API hop keys = %q, want the key on both API requests", apiKeys)
	}
}

func TestFollowRedirects_RelativeLocation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/a/start" {
			w.Header().Set("Location", "../b/final?x=1")
			w.WriteHeader(http.StatusFound)
			return
		}
		if r.URL.Path != "/b/final" || r.URL.Query().Get("x") != "1" {
			t.Errorf("redirect target = %s, want /b/final?x=1", r.URL.String())
		}
		if r.Header.Get("X-API-KEY") != "test-api-key" {
			t.Errorf("same-origin relative redirect lost the API key")
		}
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/a/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	c.setHeaders(req)
	resp, err := c.followRedirects(req)
	if err != nil {
		t.Fatalf("followRedirects() with a relative Location error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

// captureStderr runs fn with os.Stderr redirected and returns what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(reader)
		done <- string(data)
	}()
	fn()
	writer.Close()
	os.Stderr = original
	out := <-done
	reader.Close()
	return out
}

func TestDebugOutput_RedactsSignedRedirectURL(t *testing.T) {
	storage := newKeyRecorder(t, "file-bytes")
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", storage.URL+"/signed/doc.pdf?X-Amz-Credential=CREDVALUE&X-Amz-Signature=SECRETSIG")
		w.WriteHeader(http.StatusFound)
	}))
	defer api.Close()

	c := newTestClient(api)
	c.debug = true
	outputPath := filepath.Join(t.TempDir(), "doc.pdf")

	stderr := captureStderr(t, func() {
		if _, err := c.DownloadDocument(context.Background(), "/api/v1/download/doc.pdf?token=QUERYSECRET", outputPath); err != nil {
			t.Errorf("DownloadDocument() error: %v", err)
		}
	})

	for _, secret := range []string{"SECRETSIG", "CREDVALUE", "X-Amz-Signature"} {
		if strings.Contains(stderr, secret) {
			t.Errorf("debug output leaked %q:\n%s", secret, stderr)
		}
	}
	if !strings.Contains(stderr, "Following redirect -> "+storage.URL+"/signed/doc.pdf?<redacted>") {
		t.Errorf("debug output should name the redirect host and path with a redacted query:\n%s", stderr)
	}
}

func TestRedactURL(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "https://api.uspto.gov/api/v1/download/doc.pdf", want: "https://api.uspto.gov/api/v1/download/doc.pdf"},
		{in: "https://bucket.s3.amazonaws.com/k/doc.pdf?X-Amz-Signature=abc&X-Amz-Expires=60", want: "https://bucket.s3.amazonaws.com/k/doc.pdf?<redacted>"},
		{in: "https://user:pass@host.example/p?q=1#frag", want: "https://host.example/p?<redacted>"},
		{in: "/relative/path?sig=abc", want: "/relative/path?<redacted>"},
		{in: "://bad", want: "<invalid-url>"},
	}
	for _, tt := range tests {
		if got := redactRawURL(tt.in); got != tt.want {
			t.Errorf("redactRawURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Retries: 5xx, transport errors, Retry-After
// ---------------------------------------------------------------------------

func TestRequest_RetriesServerErrors(t *testing.T) {
	tests := []struct {
		name      string
		statuses  []int // one per call; the last repeats
		wantCalls int32
		wantErr   int // 0 = success
	}{
		{name: "503 twice then success", statuses: []int{503, 503, 200}, wantCalls: 3},
		{name: "500 once then success", statuses: []int{500, 200}, wantCalls: 2},
		{name: "502 exhausts retries", statuses: []int{502}, wantCalls: int32(maxRetries + 1), wantErr: 502},
		{name: "404 is not retried", statuses: []int{404}, wantCalls: 1, wantErr: 404},
		{name: "403 is not retried", statuses: []int{403}, wantCalls: 1, wantErr: 403},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int32
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := int(atomic.AddInt32(&calls, 1))
				if n > len(tt.statuses) {
					n = len(tt.statuses)
				}
				status := tt.statuses[n-1]
				if status != http.StatusOK {
					w.WriteHeader(status)
				}
				w.Write([]byte(`{}`))
			}))
			defer ts.Close()

			c := newTestClient(ts)
			_, err := c.request(context.Background(), http.MethodGet, "/test", nil, nil)

			if got := atomic.LoadInt32(&calls); got != tt.wantCalls {
				t.Errorf("request count = %d, want %d", got, tt.wantCalls)
			}
			if tt.wantErr == 0 {
				if err != nil {
					t.Fatalf("request() error: %v", err)
				}
				return
			}
			apiErr, ok := err.(*UsptoAPIError)
			if !ok || apiErr.StatusCode != tt.wantErr {
				t.Fatalf("request() error = %v, want UsptoAPIError %d", err, tt.wantErr)
			}
		})
	}
}

// dropFirstConnections closes the first n connections without answering, then
// serves normally.
func dropFirstConnections(t *testing.T, n int32, body string) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) <= n {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			conn.Close()
			return
		}
		w.Write([]byte(body))
	}))
	return ts, &calls
}

func TestRequest_RetriesTransportErrors(t *testing.T) {
	t.Run("dropped connection then success", func(t *testing.T) {
		ts, calls := dropFirstConnections(t, 2, `{}`)
		defer ts.Close()

		c := newTestClient(ts)
		// POST: net/http never replays these itself, so the retry is ours.
		if _, err := c.request(context.Background(), http.MethodPost, "/test", map[string]string{"q": "x"}, nil); err != nil {
			t.Fatalf("request() error after transport retries: %v", err)
		}
		if got := atomic.LoadInt32(calls); got != 3 {
			t.Errorf("request count = %d, want 3", got)
		}
	})

	t.Run("gives up after maxRetries", func(t *testing.T) {
		ts, calls := dropFirstConnections(t, 100, `{}`)
		defer ts.Close()

		c := newTestClient(ts)
		_, err := c.request(context.Background(), http.MethodPost, "/test", map[string]string{"q": "x"}, nil)
		if err == nil || !strings.Contains(err.Error(), "executing request") {
			t.Fatalf("request() error = %v, want an executing-request error", err)
		}
		if got := atomic.LoadInt32(calls); got != int32(maxRetries+1) {
			t.Errorf("request count = %d, want %d", got, maxRetries+1)
		}
	})

	t.Run("timeout is not retried", func(t *testing.T) {
		var calls int32
		release := make(chan struct{})
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls, 1)
			<-release
		}))
		defer ts.Close()
		defer close(release)

		c := newTestClient(ts)
		c.httpClient.Timeout = 50 * time.Millisecond
		if _, err := c.request(context.Background(), http.MethodGet, "/slow", nil, nil); err == nil {
			t.Fatal("request() should fail on timeout")
		}
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Errorf("request count = %d, want 1 (a timed-out request must not be repeated)", got)
		}
	})

	t.Run("cancelled context is not retried", func(t *testing.T) {
		ts, calls := dropFirstConnections(t, 100, `{}`)
		defer ts.Close()

		c := newTestClient(ts)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := c.request(ctx, http.MethodGet, "/test", nil, nil); err == nil {
			t.Fatal("request() should fail with a cancelled context")
		}
		if got := atomic.LoadInt32(calls); got > 1 {
			t.Errorf("request count = %d, want at most 1", got)
		}
	})
}

func TestRequest_HonoursRetryAfterOn429(t *testing.T) {
	t.Run("waits at least Retry-After", func(t *testing.T) {
		var calls int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt32(&calls, 1) == 1 {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.Write([]byte(`{}`))
		}))
		defer ts.Close()

		c := newTestClient(ts) // rateLimitWait is 5 ms; Retry-After must win
		start := time.Now()
		if _, err := c.request(context.Background(), http.MethodGet, "/test", nil, nil); err != nil {
			t.Fatalf("request() error: %v", err)
		}
		if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
			t.Errorf("retried after %s, want at least the 1 s Retry-After", elapsed)
		}
		if got := atomic.LoadInt32(&calls); got != 2 {
			t.Errorf("request count = %d, want 2", got)
		}
	})

	t.Run("Retry-After beyond the cap is returned, not slept", func(t *testing.T) {
		var calls int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer ts.Close()

		c := newTestClient(ts)
		start := time.Now()
		_, err := c.request(context.Background(), http.MethodGet, "/test", nil, nil)
		apiErr, ok := err.(*UsptoAPIError)
		if !ok || apiErr.StatusCode != 429 {
			t.Fatalf("request() error = %v, want UsptoAPIError 429", err)
		}
		if apiErr.RetryAfter != 120*time.Second {
			t.Errorf("RetryAfter = %s, want 2m0s", apiErr.RetryAfter)
		}
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Errorf("request count = %d, want 1", got)
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("took %s; a 120 s Retry-After must not be slept through", elapsed)
		}
	})
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{in: "", want: 0},
		{in: "7", want: 7 * time.Second},
		{in: " 30 ", want: 30 * time.Second},
		{in: "0", want: 0},
		{in: "-5", want: 0},
		{in: "soon", want: 0},
		{in: "Mon, 02 Jan 2006 15:04:05 GMT", want: 0}, // in the past
	}
	for _, tt := range tests {
		if got := parseRetryAfter(tt.in); got != tt.want {
			t.Errorf("parseRetryAfter(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
	future := time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)
	if got := parseRetryAfter(future); got < 80*time.Second || got > 91*time.Second {
		t.Errorf("parseRetryAfter(HTTP date 90 s ahead) = %s, want about 90s", got)
	}
}

func TestDownloadDocument_RetriesServerErrorAndDroppedConnection(t *testing.T) {
	t.Run("503 then success", func(t *testing.T) {
		var calls int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt32(&calls, 1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Write([]byte("pdf"))
		}))
		defer ts.Close()

		c := newTestClient(ts)
		outputPath := filepath.Join(t.TempDir(), "doc.pdf")
		if _, err := c.DownloadDocument(context.Background(), "/doc.pdf", outputPath); err != nil {
			t.Fatalf("DownloadDocument() error: %v", err)
		}
		if data, _ := os.ReadFile(outputPath); string(data) != "pdf" {
			t.Errorf("content = %q, want %q", data, "pdf")
		}
	})

	t.Run("dropped connection then success", func(t *testing.T) {
		ts, calls := dropFirstConnections(t, 1, "docx")
		defer ts.Close()

		c := newTestClient(ts)
		// An absolute URL on another origin would not be retried by net/http
		// either; use the fetch path, which shares getFile.
		data, err := c.FetchDocumentBytes(context.Background(), "/doc.docx")
		if err != nil {
			t.Fatalf("FetchDocumentBytes() error: %v", err)
		}
		if string(data) != "docx" || atomic.LoadInt32(calls) < 2 {
			t.Errorf("content = %q after %d calls, want %q after a retry", data, atomic.LoadInt32(calls), "docx")
		}
	})
}

// ---------------------------------------------------------------------------
// DownloadDocument: temp file + rename
// ---------------------------------------------------------------------------

func TestDownloadDocument_FailedTransferLeavesNoPartialFile(t *testing.T) {
	// The server promises 1000 bytes and sends 7, so the body copy fails.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.Write([]byte("partial"))
	}))
	defer ts.Close()

	t.Run("no file at the final path", func(t *testing.T) {
		dir := t.TempDir()
		outputPath := filepath.Join(dir, "doc.pdf")

		c := newTestClient(ts)
		if _, err := c.DownloadDocument(context.Background(), "/doc.pdf", outputPath); err == nil {
			t.Fatal("DownloadDocument() should fail when the body is cut short")
		}
		if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
			t.Errorf("a file exists at the final path after a failed transfer (stat err = %v)", err)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("output directory should be empty after a failed transfer, has %d entries (%s)", len(entries), entries[0].Name())
		}
	})

	t.Run("an existing file is left intact", func(t *testing.T) {
		dir := t.TempDir()
		outputPath := filepath.Join(dir, "doc.pdf")
		if err := os.WriteFile(outputPath, []byte("previous complete download"), 0644); err != nil {
			t.Fatal(err)
		}

		c := newTestClient(ts)
		if _, err := c.DownloadDocument(context.Background(), "/doc.pdf", outputPath); err == nil {
			t.Fatal("DownloadDocument() should fail when the body is cut short")
		}
		if data, _ := os.ReadFile(outputPath); string(data) != "previous complete download" {
			t.Errorf("existing file content = %q, want it untouched", data)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 1 {
			t.Errorf("output directory has %d entries, want only the existing file", len(entries))
		}
	})
}

func TestDownloadDocument_SuccessReplacesExistingAndLeavesNoTempFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("new content"))
	}))
	defer ts.Close()

	dir := filepath.Join(t.TempDir(), "nested", "out")
	outputPath := filepath.Join(dir, "doc.pdf")
	c := newTestClient(ts)

	// First download creates the directory and the file.
	if _, err := c.DownloadDocument(context.Background(), "/doc.pdf", outputPath); err != nil {
		t.Fatalf("DownloadDocument() error: %v", err)
	}
	// Overwrite behaviour is unchanged: a second download replaces the file.
	if err := os.WriteFile(outputPath, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DownloadDocument(context.Background(), "/doc.pdf", outputPath); err != nil {
		t.Fatalf("second DownloadDocument() error: %v", err)
	}

	if data, _ := os.ReadFile(outputPath); string(data) != "new content" {
		t.Errorf("content = %q, want %q", data, "new content")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "doc.pdf" {
		t.Errorf("output directory should hold only doc.pdf, has %d entries", len(entries))
	}
}

// ---------------------------------------------------------------------------
// Download timeout
// ---------------------------------------------------------------------------

// trickleServer sends its headers at once and then the body in two halves
// with a pause in between, like a large file on a slow link.
func trickleServer(pause time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		w.Write([]byte("first"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		select {
		case <-time.After(pause):
		case <-r.Context().Done():
			return
		}
		w.Write([]byte("-rest"))
	}))
}

func TestNewClient_DownloadTimeoutDefaultAndOption(t *testing.T) {
	if got := NewClient("k").DownloadTimeout(); got != 600*time.Second {
		t.Errorf("default download timeout = %s, want 10m0s", got)
	}
	if got := NewClient("k", WithDownloadTimeout(45*time.Minute)).DownloadTimeout(); got != 45*time.Minute {
		t.Errorf("WithDownloadTimeout(45m) = %s", got)
	}
	if got := NewClient("k", WithDownloadTimeout(0)).DownloadTimeout(); got != 0 {
		t.Errorf("WithDownloadTimeout(0) = %s, want 0 (no limit)", got)
	}
	if got := NewClient("k", WithDownloadTimeout(-time.Second)).DownloadTimeout(); got != 0 {
		t.Errorf("a negative download timeout = %s, want 0 (no limit)", got)
	}
	// --timeout is the metadata limit and must not leak into downloads.
	if got := NewClient("k", WithTimeout(5*time.Second)).DownloadTimeout(); got != 600*time.Second {
		t.Errorf("WithTimeout changed the download timeout to %s", got)
	}
}

// The download timeout covers the body. A transfer that outlasts it fails,
// leaves nothing behind, and says which flag raises the limit.
func TestDownloadDocument_DownloadTimeoutCutsOffSlowBody(t *testing.T) {
	ts := trickleServer(2 * time.Second)
	defer ts.Close()

	dir := t.TempDir()
	outputPath := filepath.Join(dir, "big.zip")
	c := newTestClient(ts)
	c.downloadTimeout = 80 * time.Millisecond

	start := time.Now()
	_, err := c.DownloadDocument(context.Background(), "/big.zip", outputPath)
	if err == nil {
		t.Fatal("DownloadDocument() should fail when the body outlasts the download timeout")
	}
	if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
		t.Errorf("gave up after %s, want about the 80ms timeout (and no retry)", elapsed)
	}
	if !strings.Contains(err.Error(), "--download-timeout") || !strings.Contains(err.Error(), "80ms") {
		t.Errorf("error should name the limit and the flag, got: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("output directory should be empty after a timed-out transfer, has %d entries", len(entries))
	}
}

func TestDownloadDocument_GenerousOrNoDownloadTimeoutCompletes(t *testing.T) {
	for name, timeout := range map[string]time.Duration{"generous": 30 * time.Second, "no limit": 0} {
		t.Run(name, func(t *testing.T) {
			ts := trickleServer(150 * time.Millisecond)
			defer ts.Close()

			outputPath := filepath.Join(t.TempDir(), "big.zip")
			c := newTestClient(ts)
			c.downloadTimeout = timeout
			// The metadata timeout is far shorter than the transfer. It must
			// not apply to file downloads.
			c.httpClient.Timeout = 20 * time.Millisecond

			if _, err := c.DownloadDocument(context.Background(), "/big.zip", outputPath); err != nil {
				t.Fatalf("DownloadDocument() error: %v", err)
			}
			if data, _ := os.ReadFile(outputPath); string(data) != "first-rest" {
				t.Errorf("file content = %q, want first-rest", data)
			}
		})
	}
}

func TestFetchDocumentBytes_DownloadTimeoutNamesTheFlag(t *testing.T) {
	ts := trickleServer(2 * time.Second)
	defer ts.Close()

	c := newTestClient(ts)
	c.downloadTimeout = 80 * time.Millisecond

	_, err := c.FetchDocumentBytes(context.Background(), "/doc.tar")
	if err == nil {
		t.Fatal("FetchDocumentBytes() should fail when the body outlasts the download timeout")
	}
	if !strings.Contains(err.Error(), "--download-timeout") {
		t.Errorf("error should name the flag, got: %v", err)
	}
}
