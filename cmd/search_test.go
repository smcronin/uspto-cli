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

		w.Header().Set("Content-Type", "text/csv")
		fmt.Fprint(w, "applicationNumberText\n16123456\n")
	}))
	defer ts.Close()

	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	cmd := &cobra.Command{}
	cmd.Flags().String("download", "", "")
	if err := cmd.Flags().Set("download", "csv"); err != nil {
		t.Fatalf("set download flag: %v", err)
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
