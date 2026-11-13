package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
)

// bulkCatalogJSON mirrors live product records, including a category name the
// API cuts at 50 characters.
const bulkCatalogJSON = `{"count":5,"bulkDataProductBag":[
	{"productIdentifier":"PTGRXML","productTitleText":"Patent Grant Full-Text Data (No Images) - XML","productFrequencyText":"WEEKLY","productDataSetCategoryArrayText":["Issued patents (patent grants)"]},
	{"productIdentifier":"PTGRAPS","productTitleText":"Patent Grant Full-Text Data (No Images) - APS","productFrequencyText":"YEARLY","productDataSetCategoryArrayText":["Issued patents (patent grants)"]},
	{"productIdentifier":"APPXML","productTitleText":"Patent Application Full-Text Data (No Images)","productFrequencyText":"WEEKLY","productDataSetCategoryArrayText":["Patent Application","Published patent applications (pre-grant publicati"]},
	{"productIdentifier":"PASDL","productTitleText":"Patent Assignment XML (Ownership) Text - Daily","productFrequencyText":"DAILY","productDataSetCategoryArrayText":["Patent Assignment"]},
	{"productIdentifier":"TRTYRAP","productTitleText":"Trademark Full Text XML Data (No Images) - Annual Applications","productFrequencyText":"YEARLY","productDataSetCategoryArrayText":["Trademark applications"]}
]}`

func resetBulkSearchState(t *testing.T) {
	t.Helper()
	origFlags := bulkSearchFlags
	origClient := api.DefaultClient
	origFormat, origQuiet, origMinify, origDryRun := flagFormat, flagQuiet, flagMinify, flagDryRun
	t.Cleanup(func() {
		bulkSearchFlags = origFlags
		api.DefaultClient = origClient
		flagFormat, flagQuiet, flagMinify, flagDryRun = origFormat, origQuiet, origMinify, origDryRun
	})
	bulkSearchFlags.title = ""
	bulkSearchFlags.category = ""
	bulkSearchFlags.frequency = ""
	bulkSearchFlags.limit = 25
	bulkSearchFlags.offset = 0
	flagFormat, flagQuiet, flagMinify, flagDryRun = "json", true, true, false
}

type bulkSearchEnvelope struct {
	OK         bool `json:"ok"`
	Pagination struct {
		Offset  int  `json:"offset"`
		Limit   int  `json:"limit"`
		Total   int  `json:"total"`
		HasMore bool `json:"hasMore"`
	} `json:"pagination"`
	Results []types.BulkDataProduct `json:"results"`
}

func decodeBulkSearchEnvelope(t *testing.T, stdout string) bulkSearchEnvelope {
	t.Helper()
	var env bulkSearchEnvelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("json.Unmarshal output: %v\noutput=%s", err, stdout)
	}
	return env
}

func bulkProductIDs(products []types.BulkDataProduct) string {
	ids := make([]string, 0, len(products))
	for _, p := range products {
		ids = append(ids, p.ProductIdentifier)
	}
	return strings.Join(ids, ",")
}

// The products search endpoint answers 404 to every field:value query, so the
// filters must never reach q. They used to, and `bulk search --category
// "Patent Grant" --frequency WEEKLY` always exited 4.
func TestRunBulkSearch_FlagFiltersRunLocallyNotInQuery(t *testing.T) {
	resetBulkSearchState(t)
	bulkSearchFlags.category = "Patent Grant"
	bulkSearchFlags.frequency = "weekly"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/datasets/products/search" {
			t.Errorf("path = %s, want /api/v1/datasets/products/search", r.URL.Path)
		}
		if q := r.URL.Query().Get("q"); q != "" {
			t.Errorf("q = %q, want no query: the endpoint rejects field:value filters", q)
		}
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Errorf("limit = %q, want 100 (one catalog page)", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(bulkCatalogJSON))
	}))
	defer ts.Close()
	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	stdout, _, err := captureTrademarkOutput(t, func() error { return runBulkSearch(bulkSearchCmd, nil) })
	if err != nil {
		t.Fatalf("runBulkSearch error: %v", err)
	}
	env := decodeBulkSearchEnvelope(t, stdout)
	if got := bulkProductIDs(env.Results); got != "PTGRXML" {
		t.Fatalf("results = %s, want PTGRXML (category substring + frequency, case-insensitive)", got)
	}
	if env.Pagination.Total != 1 || env.Pagination.HasMore {
		t.Fatalf("pagination = %+v, want total=1 hasMore=false", env.Pagination)
	}
}

func TestRunBulkSearch_FreeTextQueryStillSentWithLocalFilters(t *testing.T) {
	resetBulkSearchState(t)
	bulkSearchFlags.title = "full-text"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if q := r.URL.Query().Get("q"); q != "patent grant" {
			t.Errorf("q = %q, want the free-text query %q", q, "patent grant")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(bulkCatalogJSON))
	}))
	defer ts.Close()
	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	stdout, _, err := captureTrademarkOutput(t, func() error {
		return runBulkSearch(bulkSearchCmd, []string{"patent grant"})
	})
	if err != nil {
		t.Fatalf("runBulkSearch error: %v", err)
	}
	if got := bulkProductIDs(decodeBulkSearchEnvelope(t, stdout).Results); got != "PTGRXML,PTGRAPS,APPXML" {
		t.Fatalf("results = %s, want PTGRXML,PTGRAPS,APPXML", got)
	}
}

func TestRunBulkSearch_LocalFiltersPageThroughCatalogAndApplyOffsetLimit(t *testing.T) {
	resetBulkSearchState(t)
	bulkSearchFlags.frequency = "WEEKLY"
	bulkSearchFlags.limit = 1
	bulkSearchFlags.offset = 1

	var requests int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "application/json")
		// The server returns fewer rows than the page size asked for, so the
		// client has to follow the offset until it holds count products.
		switch r.URL.Query().Get("offset") {
		case "":
			_, _ = w.Write([]byte(`{"count":3,"bulkDataProductBag":[
				{"productIdentifier":"A","productFrequencyText":"WEEKLY"},
				{"productIdentifier":"B","productFrequencyText":"DAILY"}]}`))
		case "2":
			_, _ = w.Write([]byte(`{"count":3,"bulkDataProductBag":[
				{"productIdentifier":"C","productFrequencyText":"WEEKLY"}]}`))
		default:
			t.Errorf("unexpected offset %q", r.URL.Query().Get("offset"))
			_, _ = w.Write([]byte(`{"count":3,"bulkDataProductBag":[]}`))
		}
	}))
	defer ts.Close()
	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	stdout, _, err := captureTrademarkOutput(t, func() error { return runBulkSearch(bulkSearchCmd, nil) })
	if err != nil {
		t.Fatalf("runBulkSearch error: %v", err)
	}
	if n := atomic.LoadInt32(&requests); n != 2 {
		t.Fatalf("requests = %d, want 2", n)
	}
	env := decodeBulkSearchEnvelope(t, stdout)
	if got := bulkProductIDs(env.Results); got != "C" {
		t.Fatalf("results = %s, want C (second WEEKLY match)", got)
	}
	if env.Pagination.Total != 2 || env.Pagination.Offset != 1 || env.Pagination.Limit != 1 || env.Pagination.HasMore {
		t.Fatalf("pagination = %+v, want offset=1 limit=1 total=2 hasMore=false", env.Pagination)
	}
}

func TestRunBulkSearch_NoLocalMatchIsNotFound(t *testing.T) {
	resetBulkSearchState(t)
	bulkSearchFlags.category = "no such category"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(bulkCatalogJSON))
	}))
	defer ts.Close()
	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	_, _, err := captureTrademarkOutput(t, func() error { return runBulkSearch(bulkSearchCmd, nil) })
	wantExit(t, err, types.ExitNotFound)
	if !strings.Contains(err.Error(), `category contains "no such category"`) {
		t.Fatalf("error = %q, want it to name the filter", err)
	}
}

func TestRunBulkSearch_DryRunShowsLocalFiltersAndSendsNothing(t *testing.T) {
	resetBulkSearchState(t)
	flagDryRun = true
	bulkSearchFlags.category = "Patent Grant"
	bulkSearchFlags.frequency = "WEEKLY"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("dry run sent %s %s", r.Method, r.URL)
	}))
	defer ts.Close()
	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	_, stderr, err := captureTrademarkOutput(t, func() error { return runBulkSearch(bulkSearchCmd, nil) })
	if err != nil {
		t.Fatalf("runBulkSearch error: %v", err)
	}
	if strings.Contains(stderr, "productFrequencyText") || strings.Contains(stderr, "productDataSetCategoryArrayText") {
		t.Fatalf("dry run shows a field query:\n%s", stderr)
	}
	for _, want := range []string{"GET /api/v1/datasets/products/search", "limit=100", `then filtered locally: category contains "Patent Grant", frequency = WEEKLY`} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("dry run output missing %q:\n%s", want, stderr)
		}
	}
}

func TestRunBulkSearch_RejectsNonPositiveLimit(t *testing.T) {
	resetBulkSearchState(t)
	bulkSearchFlags.limit = 0
	err := runBulkSearch(bulkSearchCmd, nil)
	if code, _ := usageEnvelope(t, err); code != types.ExitInvalidArgs {
		t.Fatalf("exit code = %d, want %d for --limit 0", code, types.ExitInvalidArgs)
	}
}

func TestFilterBulkProducts(t *testing.T) {
	var catalog types.BulkDataResponse
	if err := json.Unmarshal([]byte(bulkCatalogJSON), &catalog); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name                       string
		title, category, frequency string
		want                       string
	}{
		{"no filters keeps all", "", "", "", "PTGRXML,PTGRAPS,APPXML,PASDL,TRTYRAP"},
		{"frequency ignores case", "", "", "daily", "PASDL"},
		{"ANNUAL means YEARLY", "", "", "ANNUAL", "PTGRAPS,TRTYRAP"},
		{"category matches a cut-off name", "", "pre-grant", "", "APPXML"},
		{"category substring ignores case", "", "TRADEMARK", "", "TRTYRAP"},
		{"title substring", "assignment", "", "", "PASDL"},
		{"filters combine with AND", "patent", "patent grants", "yearly", "PTGRAPS"},
		{"frequency is exact, not a substring", "", "", "EEKLY", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bulkProductIDs(filterBulkProducts(catalog.BulkDataProductBag, tt.title, tt.category, tt.frequency))
			if got != tt.want {
				t.Fatalf("filterBulkProducts(%q, %q, %q) = %s, want %s", tt.title, tt.category, tt.frequency, got, tt.want)
			}
		})
	}
}
