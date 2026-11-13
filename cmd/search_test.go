package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

func resetSearchFlagsForTest() {
	searchFlags = struct {
		title         string
		applicant     string
		inventor      string
		patent        string
		cpc           string
		cpcGroup      string
		status        string
		appType       string
		examiner      string
		artUnit       string
		assignee      string
		assignor      string
		reelFrame     string
		docket        string
		pubNumber     string
		filedAfter    string
		filedBefore   string
		grantedAfter  string
		grantedBefore string
		filedWithin   string
		granted       bool
		pending       bool
		limit         int
		offset        int
		all           bool
		page          int
		countOnly     bool
		sort          string
		fields        string
		filters       []string
		facets        string
	}{
		limit: 25,
	}
}

func TestValidateSearchMode_CountOnlyWithDownload(t *testing.T) {
	orig := searchFlags
	defer func() {
		searchFlags = orig
	}()
	resetSearchFlagsForTest()
	searchFlags.countOnly = true

	cmd := &cobra.Command{}
	cmd.Flags().String("download", "", "")
	if err := cmd.Flags().Set("download", "csv"); err != nil {
		t.Fatalf("Flags().Set(download): %v", err)
	}

	err := validateSearchMode(cmd)
	if err == nil {
		t.Fatal("expected error for --count-only with --download")
	}
	if !strings.Contains(err.Error(), "--count-only") {
		t.Fatalf("error = %q, want mention of --count-only", err.Error())
	}
}

func TestExecuteSearchCountOnly_GETUsesLightweightRequest(t *testing.T) {
	origFlags := searchFlags
	origDryRun := flagDryRun
	origClient := api.DefaultClient
	defer func() {
		searchFlags = origFlags
		flagDryRun = origDryRun
		api.DefaultClient = origClient
	}()

	resetSearchFlagsForTest()
	searchFlags.sort = "filingDate:desc" // should be ignored in count-only mode
	flagDryRun = false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/patent/applications/search" {
			t.Fatalf("path = %s, want /api/v1/patent/applications/search", r.URL.Path)
		}

		q := r.URL.Query()
		if got := q.Get("q"); got != "solar" {
			t.Fatalf("q = %q, want %q", got, "solar")
		}
		if got := q.Get("limit"); got != "1" {
			t.Fatalf("limit = %q, want %q", got, "1")
		}
		if got := q.Get("fields"); got != "applicationNumberText" {
			t.Fatalf("fields = %q, want %q", got, "applicationNumberText")
		}
		if got := q.Get("sort"); got != "" {
			t.Fatalf("sort = %q, want empty in count-only mode", got)
		}
		if got := q.Get("offset"); got != "" {
			t.Fatalf("offset = %q, want empty in count-only mode", got)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"count":42,"patentFileWrapperDataBag":[{"applicationNumberText":"12345678"}]}`)
	}))
	defer ts.Close()

	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	resp, err := executeSearchCountOnly(context.Background(), "solar", false)
	if err != nil {
		t.Fatalf("executeSearchCountOnly(GET) error: %v", err)
	}
	if resp.Count != 42 {
		t.Fatalf("count = %d, want 42", resp.Count)
	}
}

func TestExecuteSearchCountOnly_POSTUsesLimitOneAndMinimalFields(t *testing.T) {
	origFlags := searchFlags
	origDryRun := flagDryRun
	origClient := api.DefaultClient
	defer func() {
		searchFlags = origFlags
		flagDryRun = origDryRun
		api.DefaultClient = origClient
	}()

	resetSearchFlagsForTest()
	searchFlags.sort = "filingDate:desc" // should be ignored in count-only mode
	flagDryRun = false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/patent/applications/search" {
			t.Fatalf("path = %s, want /api/v1/patent/applications/search", r.URL.Path)
		}

		var body types.SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		if body.Pagination == nil {
			t.Fatal("pagination is nil, want limit/offset set")
		}
		if body.Pagination.Limit != 1 {
			t.Fatalf("pagination.limit = %d, want 1", body.Pagination.Limit)
		}
		if body.Pagination.Offset != 0 {
			t.Fatalf("pagination.offset = %d, want 0", body.Pagination.Offset)
		}
		if body.Q != "battery" {
			t.Fatalf("q = %q, want %q", body.Q, "battery")
		}
		if len(body.Fields) != 1 || body.Fields[0] != "applicationNumberText" {
			t.Fatalf("fields = %v, want [applicationNumberText]", body.Fields)
		}
		if len(body.Sort) != 0 {
			t.Fatalf("sort = %v, want empty in count-only mode", body.Sort)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"count":7,"patentFileWrapperDataBag":[]}`)
	}))
	defer ts.Close()

	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	resp, err := executeSearchCountOnly(context.Background(), "battery", true)
	if err != nil {
		t.Fatalf("executeSearchCountOnly(POST) error: %v", err)
	}
	if resp.Count != 7 {
		t.Fatalf("count = %d, want 7", resp.Count)
	}
}

func TestRunSearchCountOnly_JSONOutput(t *testing.T) {
	origFlags := searchFlags
	origDryRun := flagDryRun
	origClient := api.DefaultClient
	origFormat := flagFormat
	origQuiet := flagQuiet
	origMinify := flagMinify
	defer func() {
		searchFlags = origFlags
		flagDryRun = origDryRun
		api.DefaultClient = origClient
		flagFormat = origFormat
		flagQuiet = origQuiet
		flagMinify = origMinify
	}()

	resetSearchFlagsForTest()
	flagDryRun = false
	flagFormat = "json"
	flagQuiet = true
	flagMinify = true

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"count":9,"patentFileWrapperDataBag":[{"applicationNumberText":"12345678"}]}`)
	}))
	defer ts.Close()

	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := runSearchCountOnly(context.Background(), searchCmd, "sensor", false)
	w.Close()
	os.Stdout = oldStdout
	if runErr != nil {
		t.Fatalf("runSearchCountOnly() error: %v", runErr)
	}

	outBytes, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatalf("io.ReadAll(stdout): %v", err)
	}

	var env struct {
		OK         bool                   `json:"ok"`
		Command    string                 `json:"command"`
		Pagination types.PaginationMeta   `json:"pagination"`
		Results    map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(outBytes, &env); err != nil {
		t.Fatalf("json.Unmarshal(output): %v\noutput=%s", err, string(outBytes))
	}
	if !env.OK {
		t.Fatal("ok = false, want true")
	}
	if env.Command != "search" {
		t.Fatalf("command = %q, want %q", env.Command, "search")
	}
	if env.Pagination.Total != 9 {
		t.Fatalf("pagination.total = %d, want 9", env.Pagination.Total)
	}
	if got := env.Results["count"]; got != float64(9) {
		t.Fatalf("results.count = %v, want 9", got)
	}
}

func TestSearchTypeHelpText_UsesDESCode(t *testing.T) {
	usage := searchCmd.Flags().Lookup("type").Usage
	if !strings.Contains(usage, "DES") {
		t.Fatalf("type usage = %q, want to contain DES", usage)
	}
	if strings.Contains(usage, "DSN") {
		t.Fatalf("type usage = %q, should not contain DSN", usage)
	}
}

func TestRunSearch_DownloadWithFiltersUsesPostEndpoint(t *testing.T) {
	origFlags := searchFlags
	origDryRun := flagDryRun
	origClient := api.DefaultClient
	defer func() {
		searchFlags = origFlags
		flagDryRun = origDryRun
		api.DefaultClient = origClient
	}()

	resetSearchFlagsForTest()
	flagDryRun = false

	searchFlags.filters = []string{"applicationTypeLabelName=Utility"}
	searchFlags.filedAfter = "2024-01-01"
	searchFlags.granted = true
	searchFlags.sort = "filingDate:desc"
	searchFlags.limit = 10

	var sawPost bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/patent/applications/search/download" {
			t.Fatalf("path = %s, want /api/v1/patent/applications/search/download", r.URL.Path)
		}
		sawPost = true

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got, _ := body["format"].(string); got != "csv" {
			t.Fatalf("format = %q, want %q", got, "csv")
		}
		filters, ok := body["filters"].([]interface{})
		if !ok || len(filters) == 0 {
			t.Fatalf("filters missing from POST body: %#v", body["filters"])
		}
		ranges, ok := body["rangeFilters"].([]interface{})
		if !ok || len(ranges) == 0 {
			t.Fatalf("rangeFilters missing from POST body: %#v", body["rangeFilters"])
		}
		pagination, _ := body["pagination"].(map[string]interface{})
		if got, _ := pagination["limit"].(float64); got != 10 {
			t.Fatalf("pagination.limit = %v, want the explicit --limit 10", pagination["limit"])
		}

		w.Header().Set("Content-Type", "text/csv")
		fmt.Fprint(w, "applicationNumberText\n16123456\n")
	}))
	defer ts.Close()

	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	// An explicit --limit is sent as given, with no count request first (the
	// handler above fails the test on any path but /search/download).
	cmd := &cobra.Command{}
	cmd.Flags().String("download", "", "")
	cmd.Flags().Int("limit", 25, "")
	if err := cmd.Flags().Set("download", "csv"); err != nil {
		t.Fatalf("set download flag: %v", err)
	}
	if err := cmd.Flags().Set("limit", "10"); err != nil {
		t.Fatalf("set limit flag: %v", err)
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := runSearch(cmd, []string{"battery"})

	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.ReadAll(r)
	_ = r.Close()

	if runErr != nil {
		t.Fatalf("runSearch() error: %v", runErr)
	}
	if !sawPost {
		t.Fatal("expected POST download request, but server was not hit")
	}
}

func TestAnnotateSearch404_AddsHintsForFiledAfterGrantedAndFiledWithin(t *testing.T) {
	orig := searchFlags
	defer func() {
		searchFlags = orig
	}()
	resetSearchFlagsForTest()
	searchFlags.granted = true
	searchFlags.filedAfter = "2024-01-01"
	searchFlags.filedWithin = "6m"

	err := annotateSearch404(&api.UsptoAPIError{
		StatusCode: 404,
		Message:    "Not Found",
		Body:       "{}",
	})
	apiErr, ok := err.(*api.UsptoAPIError)
	if !ok {
		t.Fatalf("annotateSearch404 returned %T, want *api.UsptoAPIError", err)
	}
	if !strings.Contains(apiErr.Message, "--granted-after") {
		t.Fatalf("message = %q, want granted-after hint", apiErr.Message)
	}
	if !strings.Contains(apiErr.Message, "--filed-within") {
		t.Fatalf("message = %q, want filed-within hint", apiErr.Message)
	}
}

func TestCPCGroupTerm(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "H01M", want: "H01M*"},
		{in: " h01m ", want: "H01M*"},
		{in: "H01M*", want: "H01M*"},
	}
	for _, tc := range tests {
		if got := cpcGroupTerm(tc.in); got != tc.want {
			t.Fatalf("cpcGroupTerm(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildGetQuery_IncludesCPCGroupWildcard(t *testing.T) {
	orig := searchFlags
	defer func() {
		searchFlags = orig
	}()
	resetSearchFlagsForTest()
	searchFlags.cpcGroup = "h01m"

	q := buildGetQuery("")
	if !strings.Contains(q, "applicationMetaData.cpcClassificationBag:\"H01M*\"") {
		t.Fatalf("query = %q, want CPC group wildcard clause", q)
	}
}

func TestSearchPublicationNumberAliasFlagExists(t *testing.T) {
	if searchCmd.Flags().Lookup("publication-number") == nil {
		t.Fatal("expected --publication-number flag to be registered")
	}
}

func TestRunSearchAllPagesCSV_ClientSideConcat(t *testing.T) {
	origFlags := searchFlags
	origDryRun := flagDryRun
	origClient := api.DefaultClient
	origFormat := flagFormat
	origQuiet := flagQuiet
	defer func() {
		searchFlags = origFlags
		flagDryRun = origDryRun
		api.DefaultClient = origClient
		flagFormat = origFormat
		flagQuiet = origQuiet
	}()

	resetSearchFlagsForTest()
	searchFlags.all = true
	flagDryRun = false
	flagFormat = "csv"
	flagQuiet = true

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset := r.URL.Query().Get("offset")
		resp := types.PatentDataResponse{Count: 101}
		switch offset {
		case "":
			for i := 1; i <= 100; i++ {
				resp.PatentFileWrapperDataBag = append(resp.PatentFileWrapperDataBag, types.PatentFileWrapper{
					ApplicationNumberText: fmt.Sprintf("A%03d", i),
				})
			}
		case "100":
			resp.PatentFileWrapperDataBag = append(resp.PatentFileWrapperDataBag, types.PatentFileWrapper{
				ApplicationNumberText: "A101",
			})
		default:
			t.Fatalf("unexpected offset query: %q", offset)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer ts.Close()

	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	oldStdout := os.Stdout
	tmpFile, err := os.CreateTemp("", "search-all-csv-*.csv")
	if err != nil {
		t.Fatalf("os.CreateTemp: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	os.Stdout = tmpFile

	runErr := runSearch(searchCmd, []string{"sensor"})
	_ = tmpFile.Close()
	os.Stdout = oldStdout
	if runErr != nil {
		t.Fatalf("runSearch() error: %v", runErr)
	}

	out, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("os.ReadFile(stdout temp): %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "applicationNumberText") {
		t.Fatalf("csv output missing header, got:\n%s", s)
	}
	if !strings.Contains(s, "A101") {
		t.Fatalf("csv output missing concatenated second-page row A101, got:\n%s", s)
	}
}

func TestValidateSearchInputs_FilterNeedsFieldAndValue(t *testing.T) {
	origFlags := searchFlags
	defer func() { searchFlags = origFlags }()

	tests := []struct {
		name    string
		filter  string
		wantErr string
	}{
		{name: "field and value", filter: "applicationTypeLabelName=Utility"},
		{name: "value containing equals", filter: "docketNumber=A=B"},
		{name: "no equals sign", filter: "applicationTypeLabelName", wantErr: "expected field=value"},
		{name: "empty value", filter: "applicationTypeLabelName=", wantErr: "missing value"},
		{name: "blank value", filter: "applicationTypeLabelName=  ", wantErr: "missing value"},
		{name: "empty field", filter: "=Utility", wantErr: "missing field name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetSearchFlagsForTest()
			searchFlags.filters = []string{tt.filter}
			err := validateSearchInputs()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateSearchInputs() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateSearchInputs() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// --pending means "still alive": not granted, not abandoned, not an expired
// provisional. It used to filter on the PGPub publication category, which
// most granted patents carry too.
func TestBuildPostBody_PendingExcludesGrantedAndAbandoned(t *testing.T) {
	orig := searchFlags
	defer func() { searchFlags = orig }()

	const exclusions = `NOT applicationMetaData.publicationCategoryBag:"Granted/Issued"` +
		` AND NOT applicationMetaData.applicationStatusDescriptionText:Abandon*` +
		` AND NOT applicationMetaData.applicationStatusDescriptionText:Expressly*` +
		` AND NOT applicationMetaData.applicationStatusDescriptionText:"Provisional Application Expired"`

	tests := []struct {
		name  string
		query string
		setup func()
		wantQ string
	}{
		{name: "free text", query: "battery", wantQ: "(battery) AND " + exclusions},
		{name: "no other terms", query: "", wantQ: exclusions},
		{
			name:  "OR in the caller's query stays grouped",
			query: "battery OR capacitor",
			wantQ: "(battery OR capacitor) AND " + exclusions,
		},
		{
			name:  "shorthand flag",
			setup: func() { searchFlags.assignee = "Tesla" },
			wantQ: "(assignmentBag.assigneeBag.assigneeNameText:Tesla) AND " + exclusions,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetSearchFlagsForTest()
			searchFlags.pending = true
			if tt.setup != nil {
				tt.setup()
			}
			if !needsPostEndpoint() {
				t.Fatal("--pending must use the POST endpoint")
			}

			body := buildPostBody(tt.query, 25, 0)

			if body.Q != tt.wantQ {
				t.Errorf("q =\n  %s\nwant\n  %s", body.Q, tt.wantQ)
			}
			for _, f := range body.Filters {
				if f.Name == "applicationMetaData.publicationCategoryBag" {
					t.Errorf("--pending still sends a publicationCategoryBag filter: %+v", f)
				}
			}
			// The GET builder must agree, should the routing ever change.
			if got := buildGetQueryWithDates(tt.query); got != tt.wantQ {
				t.Errorf("GET query =\n  %s\nwant\n  %s", got, tt.wantQ)
			}
		})
	}
}

func TestBuildPostBody_GrantedIsUnchangedAndNotPending(t *testing.T) {
	orig := searchFlags
	defer func() { searchFlags = orig }()
	resetSearchFlagsForTest()
	searchFlags.granted = true

	body := buildPostBody("battery", 25, 0)

	if body.Q != "battery" {
		t.Errorf("q = %q, want the caller's query untouched", body.Q)
	}
	if len(body.Filters) != 1 || body.Filters[0].Name != "applicationMetaData.publicationCategoryBag" ||
		len(body.Filters[0].Value) != 1 || body.Filters[0].Value[0] != "Granted/Issued" {
		t.Errorf("filters = %+v, want the Granted/Issued filter", body.Filters)
	}
}

// truncatedSearchServer reports total matches and serves full pages of
// pageSize records for whatever offset is asked for.
func truncatedSearchServer(t *testing.T, total, pageSize int) (*httptest.Server, *[]string) {
	t.Helper()
	var offsets []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offsets = append(offsets, r.URL.Query().Get("offset"))
		resp := types.PatentDataResponse{Count: total}
		for i := 0; i < pageSize; i++ {
			resp.PatentFileWrapperDataBag = append(resp.PatentFileWrapperDataBag, types.PatentFileWrapper{
				ApplicationNumberText: fmt.Sprintf("A%s-%03d", r.URL.Query().Get("offset"), i),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(ts.Close)
	return ts, &offsets
}

// allPagesHarness runs --all against ts with a small cap (300) and page
// size (100) so the 10,000-result ceiling can be reached in three requests.
func allPagesHarness(t *testing.T, ts *httptest.Server, format string) {
	t.Helper()
	origFlags, origDryRun, origClient := searchFlags, flagDryRun, api.DefaultClient
	origFormat, origQuiet, origMinify := flagFormat, flagQuiet, flagMinify
	origLimit, origSize := autoPageLimit, autoPageSize
	t.Cleanup(func() {
		searchFlags, flagDryRun, api.DefaultClient = origFlags, origDryRun, origClient
		flagFormat, flagQuiet, flagMinify = origFormat, origQuiet, origMinify
		autoPageLimit, autoPageSize = origLimit, origSize
	})
	resetSearchFlagsForTest()
	searchFlags.all = true
	flagDryRun, flagFormat, flagQuiet, flagMinify = false, format, true, true
	autoPageLimit, autoPageSize = 300, 100
	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))
}

// --all stops at USPTO's 10,000-result ceiling. That used to be visible only
// in a progress line -q hides; it is now reported in every format.
func TestRunSearchAllPages_TruncationIsReportedInEveryFormatWhenQuiet(t *testing.T) {
	const wantLine = "Warning: returned 300 of 25000 results; USPTO search stops at 300."

	for _, format := range []string{"json", "ndjson", "csv", "table"} {
		t.Run(format, func(t *testing.T) {
			ts, offsets := truncatedSearchServer(t, 25000, 100)
			allPagesHarness(t, ts, format)

			stdout, stderr, err := captureTrademarkOutput(t, func() error {
				return runSearch(searchCmd, []string{"battery"})
			})
			if err != nil {
				t.Fatalf("runSearch() error: %v (truncation must not change the exit code)", err)
			}
			if got := strings.Join(*offsets, ","); got != ",100,200" {
				t.Errorf("requested offsets %q, want three pages", got)
			}
			if !strings.Contains(stderr, wantLine) {
				t.Errorf("stderr lacks the truncation line under -q:\n%s", stderr)
			}
			if strings.Count(stderr, "Warning:") != 1 {
				t.Errorf("want exactly one warning line on stderr:\n%s", stderr)
			}

			switch format {
			case "json":
				var env struct {
					OK         bool     `json:"ok"`
					Warnings   []string `json:"warnings"`
					Pagination struct {
						Total     int  `json:"total"`
						HasMore   bool `json:"hasMore"`
						Truncated bool `json:"truncated"`
					} `json:"pagination"`
					Results []json.RawMessage `json:"results"`
				}
				if err := json.Unmarshal([]byte(stdout), &env); err != nil {
					t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout)
				}
				if !env.OK || !env.Pagination.Truncated || !env.Pagination.HasMore || env.Pagination.Total != 25000 {
					t.Errorf("envelope = ok %v pagination %+v, want ok with truncated and hasMore", env.OK, env.Pagination)
				}
				if len(env.Results) != 300 {
					t.Errorf("results = %d, want 300", len(env.Results))
				}
				if len(env.Warnings) != 1 || !strings.Contains(env.Warnings[0], "returned 300 of 25000 results") {
					t.Errorf("warnings = %q, want the truncation warning", env.Warnings)
				}
			case "ndjson":
				if got := strings.Count(strings.TrimSpace(stdout), "\n") + 1; got != 300 {
					t.Errorf("ndjson lines = %d, want 300", got)
				}
			case "csv":
				if got := strings.Count(strings.TrimSpace(stdout), "\n"); got != 300 {
					t.Errorf("csv data rows = %d, want 300", got)
				}
			}
		})
	}
}

func TestRunSearchAllPages_CompleteRunHasNoTruncationMarker(t *testing.T) {
	for _, format := range []string{"json", "csv"} {
		t.Run(format, func(t *testing.T) {
			ts, _ := truncatedSearchServer(t, 200, 100)
			allPagesHarness(t, ts, format)

			stdout, stderr, err := captureTrademarkOutput(t, func() error {
				return runSearch(searchCmd, []string{"battery"})
			})
			if err != nil {
				t.Fatalf("runSearch() error: %v", err)
			}
			if stderr != "" {
				t.Errorf("a complete --all run under -q should leave stderr empty, got %q", stderr)
			}
			if strings.Contains(stdout, "truncated") || strings.Contains(stdout, `"warnings"`) {
				t.Errorf("a complete run should carry no truncation marker:\n%.300s", stdout)
			}
		})
	}
}
