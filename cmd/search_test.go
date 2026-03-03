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
