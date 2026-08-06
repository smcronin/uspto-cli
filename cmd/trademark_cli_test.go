package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/smcronin/uspto-cli/internal/config"
	"github.com/smcronin/uspto-cli/internal/tmsearch"
	"github.com/smcronin/uspto-cli/internal/tsdr"
	"github.com/spf13/cobra"
)

func resetTrademarkSearchFlagsForTest() {
	trademarkSearchFlags.query = ""
	trademarkSearchFlags.wordmark = ""
	trademarkSearchFlags.owner = ""
	trademarkSearchFlags.goods = ""
	trademarkSearchFlags.serial = ""
	trademarkSearchFlags.registration = ""
	trademarkSearchFlags.class = ""
	trademarkSearchFlags.usClass = ""
	trademarkSearchFlags.designCode = ""
	trademarkSearchFlags.attorney = ""
	trademarkSearchFlags.status = ""
	trademarkSearchFlags.filedFrom = ""
	trademarkSearchFlags.filedTo = ""
	trademarkSearchFlags.registeredFrom = ""
	trademarkSearchFlags.registeredTo = ""
	trademarkSearchFlags.limit = 100
	trademarkSearchFlags.offset = 0
	trademarkSearchFlags.all = false
	trademarkSearchFlags.maxResults = 10000
	trademarkSearchFlags.countOnly = false
	trademarkSearchFlags.fields = strings.Join(defaultTrademarkSearchFields, ",")
	trademarkSearchFlags.sort = ""
	trademarkSearchFlags.facets = nil
	trademarkSearchFlags.rawBody = ""
	trademarkSearchFlags.rawResponse = false
}

func captureTrademarkOutput(t *testing.T, fn func() error) (stdout, stderr string, err error) {
	t.Helper()
	originalStdout := os.Stdout
	originalStderr := os.Stderr
	stdoutReader, stdoutWriter, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatal(pipeErr)
	}
	stderrReader, stderrWriter, pipeErr := os.Pipe()
	if pipeErr != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		t.Fatal(pipeErr)
	}
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	type readResult struct {
		value string
		err   error
	}
	stdoutResult := make(chan readResult, 1)
	stderrResult := make(chan readResult, 1)
	go func() {
		data, readErr := io.ReadAll(stdoutReader)
		stdoutResult <- readResult{value: string(data), err: readErr}
	}()
	go func() {
		data, readErr := io.ReadAll(stderrReader)
		stderrResult <- readResult{value: string(data), err: readErr}
	}()

	err = fn()
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	os.Stdout = originalStdout
	os.Stderr = originalStderr
	stdoutRead := <-stdoutResult
	stderrRead := <-stderrResult
	_ = stdoutReader.Close()
	_ = stderrReader.Close()
	if stdoutRead.err != nil {
		t.Fatalf("reading captured stdout: %v", stdoutRead.err)
	}
	if stderrRead.err != nil {
		t.Fatalf("reading captured stderr: %v", stderrRead.err)
	}
	return stdoutRead.value, stderrRead.value, err
}

func TestTrademarkCommandTreeAndProviders(t *testing.T) {
	tests := []struct {
		path     []string
		provider apiProvider
	}{
		{[]string{"trademark"}, providerNone},
		{[]string{"trademark", "search"}, providerTMSearch},
		{[]string{"tm", "search"}, providerTMSearch},
		{[]string{"trademark", "case", "status"}, providerTSDR},
		{[]string{"trademark", "case", "get"}, providerTSDR},
		{[]string{"trademark", "docs", "list"}, providerTSDR},
		{[]string{"trademark", "request"}, providerTSDR},
		{[]string{"search"}, providerODP},
		{[]string{"config", "show"}, providerNone},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.path, "_"), func(t *testing.T) {
			command, remaining, err := rootCmd.Find(test.path)
			if err != nil {
				t.Fatalf("Find(%v): %v", test.path, err)
			}
			if len(remaining) != 0 {
				t.Fatalf("Find(%v) left arguments %v", test.path, remaining)
			}
			if got := providerForCommand(command); got != test.provider {
				t.Fatalf("providerForCommand(%q) = %q, want %q", command.CommandPath(), got, test.provider)
			}
		})
	}

	for _, path := range [][]string{
		{"trademark", "batch"},
		{"trademark", "image"},
		{"trademark", "last-update"},
		{"trademark", "map"},
		{"trademark", "api-spec"},
		{"trademark", "multimedia", "download"},
		{"trademark", "case", "events"},
		{"trademark", "case", "maintenance"},
		{"trademark", "case", "export"},
		{"trademark", "case", "bundle"},
		{"trademark", "docs", "info"},
		{"trademark", "docs", "download"},
		{"trademark", "docs", "page"},
		{"trademark", "docs", "bundle"},
		{"trademark", "docs", "selected"},
		{"trademark", "docs", "download-all"},
	} {
		command, remaining, err := rootCmd.Find(path)
		if err != nil || len(remaining) != 0 {
			t.Errorf("command %q missing: command=%v remaining=%v err=%v", strings.Join(path, " "), command, remaining, err)
		}
	}
}

func TestResolveTSDRAPIKeyPrecedence(t *testing.T) {
	originalFlag := flagTSDRAPIKey
	defer func() { flagTSDRAPIKey = originalFlag }()
	t.Setenv(config.ConfigDirOverrideEnvVar, t.TempDir())
	if _, err := config.SaveTSDRAPIKey("global-key"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		flag      string
		canonical string
		legacy    string
		want      string
	}{
		{name: "flag", flag: " flag-key ", canonical: "canonical-key", legacy: "legacy-key", want: "flag-key"},
		{name: "canonical environment", canonical: " canonical-key ", legacy: "legacy-key", want: "canonical-key"},
		{name: "legacy environment", legacy: " legacy-key ", want: "legacy-key"},
		{name: "global config", want: "global-key"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flagTSDRAPIKey = test.flag
			t.Setenv(config.TSDRAPIKeyEnvVar, test.canonical)
			t.Setenv(config.LegacyTSDRAPIKeyEnvVar, test.legacy)
			got, err := resolveTSDRAPIKey()
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("resolveTSDRAPIKey() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestInitConfigAllowsKeylessTrademarkSearchAndFailsFastForTSDR(t *testing.T) {
	originalAPIKey := flagAPIKey
	originalTSDRKey := flagTSDRAPIKey
	originalDryRun := flagDryRun
	originalTimeout := flagTimeout
	defer func() {
		flagAPIKey = originalAPIKey
		flagTSDRAPIKey = originalTSDRKey
		flagDryRun = originalDryRun
		flagTimeout = originalTimeout
	}()
	t.Setenv(config.ConfigDirOverrideEnvVar, t.TempDir())
	t.Setenv(config.APIKeyEnvVar, "")
	t.Setenv(config.TSDRAPIKeyEnvVar, "")
	t.Setenv(config.LegacyTSDRAPIKeyEnvVar, "")
	flagAPIKey = ""
	flagTSDRAPIKey = ""
	flagDryRun = false
	flagTimeout = 1

	search, _, err := rootCmd.Find([]string{"trademark", "search"})
	if err != nil {
		t.Fatal(err)
	}
	if err := initConfig(search); err != nil {
		t.Fatalf("keyless Trademark Search should initialize: %v", err)
	}

	status, _, err := rootCmd.Find([]string{"trademark", "case", "status"})
	if err != nil {
		t.Fatal(err)
	}
	err = initConfig(status)
	credential, ok := err.(*credentialError)
	if !ok {
		t.Fatalf("TSDR without key error = %T %v, want *credentialError", err, err)
	}
	if credential.provider != "TSDR" || !strings.Contains(credential.hint, "patent ODP key will not work") {
		t.Fatalf("credential error = %#v", credential)
	}

	flagDryRun = true
	if err := initConfig(status); err != nil {
		t.Fatalf("TSDR dry-run should not require a key: %v", err)
	}
}

func TestBuildTrademarkSearchQueryFriendlyAndRawSyntax(t *testing.T) {
	original := trademarkSearchFlags
	defer func() { trademarkSearchFlags = original }()

	t.Run("friendly filters", func(t *testing.T) {
		resetTrademarkSearchFlagsForTest()
		trademarkSearchFlags.owner = "OpenAI OpCo, LLC"
		trademarkSearchFlags.class = "042"
		trademarkSearchFlags.status = "live"
		trademarkSearchFlags.filedFrom = "2024-01-02"
		trademarkSearchFlags.filedTo = "20241231"
		query, err := buildTrademarkSearchQuery([]string{"CHAT GPT"})
		if err != nil {
			t.Fatal(err)
		}
		want := `CM:"CHAT GPT" AND ON:"OpenAI OpCo, LLC" AND IC:"042" AND LD:true AND FD:[20240102 TO 20241231]`
		if query != want {
			t.Fatalf("query = %q, want %q", query, want)
		}
	})

	t.Run("official field syntax is preserved", func(t *testing.T) {
		resetTrademarkSearchFlagsForTest()
		query := `CM:(APPLE OR PEAR) AND NOT ON:"Example LLC"`
		trademarkSearchFlags.query = query
		got, err := buildTrademarkSearchQuery(nil)
		if err != nil {
			t.Fatal(err)
		}
		if got != query {
			t.Fatalf("query was rewritten: %q", got)
		}
	})

	for _, test := range []struct {
		name   string
		status string
		from   string
		want   string
	}{
		{name: "bad status", status: "pending", want: "invalid --status"},
		{name: "bad date", from: "01/02/2024", want: "date must be YYYYMMDD"},
	} {
		t.Run(test.name, func(t *testing.T) {
			resetTrademarkSearchFlagsForTest()
			trademarkSearchFlags.wordmark = "TEST"
			trademarkSearchFlags.status = test.status
			trademarkSearchFlags.filedFrom = test.from
			_, err := buildTrademarkSearchQuery(nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestParseTrademarkSortsAndFacets(t *testing.T) {
	sorts, err := parseTrademarkSorts("wordmarkExact:ASC, id:desc")
	if err != nil {
		t.Fatal(err)
	}
	wantSorts := []tmsearch.SortSpec{
		{Field: tmsearch.SortWordmarkExact, Direction: tmsearch.SortAscending},
		{Field: tmsearch.SortID, Direction: tmsearch.SortDescending},
	}
	if !reflect.DeepEqual(sorts, wantSorts) {
		t.Fatalf("sorts = %#v, want %#v", sorts, wantSorts)
	}

	facets, err := parseTrademarkFacets([]string{"classes=internationalClassExact:25,status=alive", "owners=ownerNameExact:3"})
	if err != nil {
		t.Fatal(err)
	}
	wantFacets := []tmsearch.TermFacet{
		{Name: "classes", Field: "internationalClassExact", Size: 25},
		{Name: "status", Field: "alive", Size: 20},
		{Name: "owners", Field: "ownerNameExact", Size: 3},
	}
	if !reflect.DeepEqual(facets, wantFacets) {
		t.Fatalf("facets = %#v, want %#v", facets, wantFacets)
	}

	for _, test := range []struct {
		name string
		fn   func() error
	}{
		{name: "sort without direction", fn: func() error { _, err := parseTrademarkSorts("id"); return err }},
		{name: "bad sort direction", fn: func() error { _, err := parseTrademarkSorts("id:up"); return err }},
		{name: "facet without name", fn: func() error { _, err := parseTrademarkFacets([]string{"=alive"}); return err }},
		{name: "zero facet size", fn: func() error { _, err := parseTrademarkFacets([]string{"status=alive:0"}); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.fn(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestRunTrademarkSearchMockServerJSONEnvelope(t *testing.T) {
	originalFlags := trademarkSearchFlags
	originalClient := tmsearch.DefaultClient
	originalFormat := flagFormat
	originalQuiet := flagQuiet
	originalMinify := flagMinify
	originalDryRun := flagDryRun
	defer func() {
		trademarkSearchFlags = originalFlags
		tmsearch.DefaultClient = originalClient
		flagFormat = originalFormat
		flagQuiet = originalQuiet
		flagMinify = originalMinify
		flagDryRun = originalDryRun
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/service/tmsearch" {
			t.Errorf("request = %s %s", request.Method, request.URL.Path)
		}
		if got := request.Header.Get(tsdr.APIKeyHeader); got != "" {
			t.Errorf("keyless search sent TSDR key %q", got)
		}
		if got := request.Header.Get("X-API-KEY"); got != "" {
			t.Errorf("keyless search sent ODP key %q", got)
		}
		var body tmsearch.SearchRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if got := body.Query.QueryString.Query; got != `CM:"USPTO" AND LD:true` {
			t.Errorf("query = %q", got)
		}
		if body.Size != 2 || body.From != 5 || !body.TrackTotalHits {
			t.Errorf("paging = from %d size %d track %v", body.From, body.Size, body.TrackTotalHits)
		}
		if len(body.Sort) != 2 || body.Sort[0].Field != tmsearch.SortWordmarkExact || body.Sort[1].Field != tmsearch.SortID {
			t.Errorf("sort = %#v", body.Sort)
		}
		classes := body.Aggregations["classes"].Terms
		if classes["field"] != "internationalClassExact" || classes["size"] != float64(5) && classes["size"] != 5 {
			t.Errorf("classes facet = %#v", classes)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"took":1,
			"hits":{"totalValue":9,"totalRelation":"eq","hits":[
				{"id":"97054561","score":2.5,"source":{"id":"97054561","wordmark":"USPTO","alive":true}}
			]},
			"aggregations":{"classes":{"buckets":[{"key":"042","doc_count":7}]}}
		}`)
	}))
	defer server.Close()

	resetTrademarkSearchFlagsForTest()
	trademarkSearchFlags.limit = 2
	trademarkSearchFlags.offset = 5
	trademarkSearchFlags.status = "live"
	trademarkSearchFlags.fields = "id,wordmark,alive"
	trademarkSearchFlags.sort = "wordmarkExact:asc,id:desc"
	trademarkSearchFlags.facets = []string{"classes=internationalClassExact:5"}
	flagFormat = "json"
	flagQuiet = true
	flagMinify = true
	flagDryRun = false
	tmsearch.DefaultClient = tmsearch.NewClient(tmsearch.WithBaseURL(server.URL + "/service"))

	stdout, stderr, err := captureTrademarkOutput(t, func() error {
		return runTrademarkSearch(trademarkSearchCmd, []string{"USPTO"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("quiet search stderr = %q", stderr)
	}
	var envelope struct {
		OK         bool                        `json:"ok"`
		Command    string                      `json:"command"`
		Results    []map[string]any            `json:"results"`
		Pagination map[string]any              `json:"pagination"`
		Facets     map[string][]map[string]any `json:"facets"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	if !envelope.OK || envelope.Command != "search" || len(envelope.Results) != 1 {
		t.Fatalf("envelope = %#v", envelope)
	}
	if envelope.Results[0]["wordmark"] != "USPTO" || envelope.Results[0]["searchId"] != "97054561" {
		t.Fatalf("result = %#v", envelope.Results[0])
	}
	if envelope.Pagination["offset"] != float64(5) || envelope.Pagination["total"] != float64(9) || envelope.Pagination["hasMore"] != true {
		t.Fatalf("pagination = %#v", envelope.Pagination)
	}
	if len(envelope.Facets["classes"]) != 1 || envelope.Facets["classes"][0]["value"] != "042" {
		t.Fatalf("facets = %#v", envelope.Facets)
	}
}

func TestTrademarkSearchAllEnforcesMaxResultsOnInitialPage(t *testing.T) {
	originalFlags := trademarkSearchFlags
	originalClient := tmsearch.DefaultClient
	originalFormat, originalQuiet, originalMinify, originalDryRun := flagFormat, flagQuiet, flagMinify, flagDryRun
	defer func() {
		trademarkSearchFlags = originalFlags
		tmsearch.DefaultClient = originalClient
		flagFormat, flagQuiet, flagMinify, flagDryRun = originalFormat, originalQuiet, originalMinify, originalDryRun
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var body tmsearch.SearchRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Size != 1 {
			t.Errorf("initial request size = %d, want max-results cap 1", body.Size)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"hits":{"totalValue":3,"totalRelation":"eq","hits":[
			{"id":"1","source":{"id":"1","wordmark":"ONE"}},
			{"id":"2","source":{"id":"2","wordmark":"TWO"}},
			{"id":"3","source":{"id":"3","wordmark":"THREE"}}
		]}}`)
	}))
	defer server.Close()

	resetTrademarkSearchFlagsForTest()
	trademarkSearchFlags.all = true
	trademarkSearchFlags.limit = 100
	trademarkSearchFlags.maxResults = 1
	flagFormat, flagQuiet, flagMinify, flagDryRun = "json", true, true, false
	tmsearch.DefaultClient = tmsearch.NewClient(tmsearch.WithBaseURL(server.URL))

	stdout, _, err := captureTrademarkOutput(t, func() error {
		return runTrademarkSearch(trademarkSearchCmd, []string{"ONE"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Results    []map[string]any `json:"results"`
		Pagination struct {
			Limit   int  `json:"limit"`
			HasMore bool `json:"hasMore"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Results) != 1 || envelope.Pagination.Limit != 1 || !envelope.Pagination.HasMore {
		t.Fatalf("max-results envelope = %#v", envelope)
	}
}

func TestTrademarkSearchAllClampsToProviderResultWindow(t *testing.T) {
	originalFlags := trademarkSearchFlags
	originalClient := tmsearch.DefaultClient
	originalFormat, originalQuiet, originalMinify, originalDryRun := flagFormat, flagQuiet, flagMinify, flagDryRun
	defer func() {
		trademarkSearchFlags = originalFlags
		tmsearch.DefaultClient = originalClient
		flagFormat, flagQuiet, flagMinify, flagDryRun = originalFormat, originalQuiet, originalMinify, originalDryRun
	}()
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		calls++
		var body tmsearch.SearchRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.From != 9990 || body.Size != 10 {
			t.Errorf("window request from/size = %d/%d", body.From, body.Size)
		}
		hits := make([]string, 10)
		for i := range hits {
			hits[i] = fmt.Sprintf(`{"id":"%d","source":{"id":"%d","wordmark":"MARK"}}`, 9990+i, 9990+i)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"hits":{"totalValue":10020,"totalRelation":"eq","hits":[%s]}}`, strings.Join(hits, ","))
	}))
	defer server.Close()

	resetTrademarkSearchFlagsForTest()
	trademarkSearchFlags.offset = 9990
	trademarkSearchFlags.limit = 10
	trademarkSearchFlags.all = true
	trademarkSearchFlags.maxResults = 20
	flagFormat, flagQuiet, flagMinify, flagDryRun = "json", true, true, false
	tmsearch.DefaultClient = tmsearch.NewClient(tmsearch.WithBaseURL(server.URL))
	stdout, _, err := captureTrademarkOutput(t, func() error { return runTrademarkSearch(trademarkSearchCmd, []string{"MARK"}) })
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("search calls = %d, want one request inside result window", calls)
	}
	var envelope struct {
		Results    []any `json:"results"`
		Pagination struct {
			HasMore bool `json:"hasMore"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Results) != 10 || !envelope.Pagination.HasMore {
		t.Fatalf("window envelope = %#v", envelope)
	}
}

func TestTrademarkPlainTextPunctuationAndExplicitQuery(t *testing.T) {
	original := trademarkSearchFlags
	defer func() { trademarkSearchFlags = original }()
	resetTrademarkSearchFlagsForTest()
	got, err := buildTrademarkSearchQuery([]string{"LOVE: YOU"})
	if err != nil {
		t.Fatal(err)
	}
	if got != `CM:"LOVE: YOU"` {
		t.Fatalf("plain punctuation query = %q", got)
	}
	trademarkSearchFlags.query = `CM:APPLE AND IC:009`
	got, err = buildTrademarkSearchQuery(nil)
	if err != nil || got != trademarkSearchFlags.query {
		t.Fatalf("explicit query = %q, %v", got, err)
	}
}

func TestTrademarkCountOnlyDryRunRequestsNoHits(t *testing.T) {
	originalFlags := trademarkSearchFlags
	originalDryRun := flagDryRun
	defer func() {
		trademarkSearchFlags = originalFlags
		flagDryRun = originalDryRun
	}()
	resetTrademarkSearchFlagsForTest()
	trademarkSearchFlags.owner = "NIKE"
	trademarkSearchFlags.class = "025"
	trademarkSearchFlags.countOnly = true
	flagDryRun = true
	_, stderr, err := captureTrademarkOutput(t, func() error {
		return runTrademarkSearch(trademarkSearchCmd, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, `"size": 0`) || strings.Contains(stderr, `"_source"`) || strings.Contains(stderr, `"sort"`) || strings.Contains(stderr, `"aggs"`) {
		t.Fatalf("count-only dry-run requested unnecessary records or shaping:\n%s", stderr)
	}
}

func TestTrademarkFriendlyQueriesQuoteLuceneReservedCharacters(t *testing.T) {
	for _, test := range []struct {
		value string
		want  string
	}{
		{value: "APPLE", want: `CM:"APPLE"`},
		{value: "AC/DC", want: `CM:"AC/DC"`},
		{value: `A+B-C*D?E`, want: `CM:"A+B-C*D?E"`},
		{value: `A && B || C`, want: `CM:"A && B || C"`},
		{value: `say "yes" \\ now`, want: `CM:"say \"yes\" \\\\ now"`},
	} {
		t.Run(test.value, func(t *testing.T) {
			if got := taggedTrademarkQuery("CM", test.value); got != test.want {
				t.Fatalf("taggedTrademarkQuery() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTrademarkRawResponseOmitsDefaultSourceProjection(t *testing.T) {
	originalFlags := trademarkSearchFlags
	originalClient := tmsearch.DefaultClient
	originalFormat, originalQuiet, originalMinify := flagFormat, flagQuiet, flagMinify
	defer func() {
		trademarkSearchFlags = originalFlags
		tmsearch.DefaultClient = originalClient
		flagFormat, flagQuiet, flagMinify = originalFormat, originalQuiet, originalMinify
	}()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if _, projected := body["_source"]; projected {
			t.Errorf("raw response unexpectedly projected _source: %#v", body["_source"])
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"hits":{"totalValue":1,"hits":[{"id":"1","source":{"id":"1","futureField":"kept"}}]}}`)
	}))
	defer server.Close()
	resetTrademarkSearchFlagsForTest()
	trademarkSearchFlags.rawResponse = true
	flagFormat, flagQuiet, flagMinify = "json", true, true
	tmsearch.DefaultClient = tmsearch.NewClient(tmsearch.WithBaseURL(server.URL))
	if _, _, err := captureTrademarkOutput(t, func() error { return runTrademarkSearch(trademarkSearchCmd, []string{"MARK"}) }); err != nil {
		t.Fatal(err)
	}
}

func TestTrademarkSearchDryRunDoesNotUseClient(t *testing.T) {
	originalFlags := trademarkSearchFlags
	originalClient := tmsearch.DefaultClient
	originalDryRun := flagDryRun
	defer func() {
		trademarkSearchFlags = originalFlags
		tmsearch.DefaultClient = originalClient
		flagDryRun = originalDryRun
	}()
	resetTrademarkSearchFlagsForTest()
	trademarkSearchFlags.wordmark = "OPEN AI"
	trademarkSearchFlags.limit = 10
	trademarkSearchFlags.sort = "id:asc"
	flagDryRun = true
	tmsearch.DefaultClient = nil

	stdout, stderr, err := captureTrademarkOutput(t, func() error {
		return runTrademarkSearch(trademarkSearchCmd, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" {
		t.Fatalf("dry-run stdout = %q", stdout)
	}
	for _, want := range []string{"POST https://tmsearch.uspto.gov/<discovered-version>/tmsearch", `CM:\"OPEN AI\"`, `"id": "asc"`} {
		if !strings.Contains(stderr, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, stderr)
		}
	}
}

func TestCollectValuesAndDocumentQueryFromFile(t *testing.T) {
	original := trademarkDocsListFlags
	defer func() { trademarkDocsListFlags = original }()
	path := filepath.Join(t.TempDir(), "identifiers.txt")
	if err := os.WriteFile(path, []byte("rn:1234567, ir:9999999\nref:Z1231384; sn:97054561\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	trademarkDocsListFlags.idType = "auto"
	trademarkDocsListFlags.idsFile = path
	trademarkDocsListFlags.fromDate = "2024-01-01"
	trademarkDocsListFlags.toDate = "2024-12-31"
	trademarkDocsListFlags.types = "SPE,OOA"
	trademarkDocsListFlags.category = "RC"
	trademarkDocsListFlags.sort = "date:A"
	query, err := documentQuery([]string{"sn:87654321"})
	if err != nil {
		t.Fatal(err)
	}
	if len(query.Identifiers) != 5 || query.Identifiers[0].Type != tsdr.IdentifierSerial || query.Identifiers[0].Value != "87654321" {
		t.Fatalf("identifiers = %#v", query.Identifiers)
	}
	values, err := query.Values()
	if err != nil {
		t.Fatal(err)
	}
	if got := values.Get("sn"); got != "87654321,97054561" {
		t.Fatalf("sn = %q", got)
	}
	if values.Get("rn") != "1234567" || values.Get("ir") != "9999999" || values.Get("ref") != "Z1231384" {
		t.Fatalf("identifier params = %v", values)
	}
	if values.Get("fromDate") != "2024-01-01" || values.Get("toDate") != "2024-12-31" || values.Get("type") != "SPE,OOA" || values.Get("category") != "RC" || values.Get("sort") != "date:A" {
		t.Fatalf("filter params = %v", values)
	}
}

func TestDocumentQueryDeduplicatesNormalizedIdentifiers(t *testing.T) {
	original := trademarkDocsListFlags
	defer func() { trademarkDocsListFlags = original }()
	trademarkDocsListFlags = struct {
		idType   string
		idsFile  string
		date     string
		fromDate string
		toDate   string
		types    string
		category string
		sort     string
	}{idType: "serial"}
	query, err := documentQuery([]string{"72131351", "sn:72131351", "72-131351"})
	if err != nil {
		t.Fatal(err)
	}
	if len(query.Identifiers) != 1 || query.Identifiers[0].Value != "72131351" {
		t.Fatalf("normalized document identifiers = %#v, want one serial", query.Identifiers)
	}
	encoded, err := query.EncodedQuery()
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "sn=72131351" {
		t.Fatalf("encoded query = %q, want one identifier", encoded)
	}
}

func TestTrademarkBatchFileDryRunChunksWithoutNetwork(t *testing.T) {
	originalFlags := trademarkBatchFlags
	originalDryRun := flagDryRun
	originalClient := tsdr.DefaultClient
	defer func() {
		trademarkBatchFlags = originalFlags
		flagDryRun = originalDryRun
		tsdr.DefaultClient = originalClient
	}()
	path := filepath.Join(t.TempDir(), "serials.txt")
	if err := os.WriteFile(path, []byte("97054561\n97054562,97054563\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	trademarkBatchFlags.idsFile = path
	trademarkBatchFlags.display = true
	trademarkBatchFlags.allowDupes = true
	trademarkBatchFlags.chunkSize = 2
	flagDryRun = true
	tsdr.DefaultClient = nil

	stdout, stderr, err := captureTrademarkOutput(t, func() error {
		return runTrademarkBatch(trademarkBatchCmd, []string{"serial"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" {
		t.Fatalf("dry-run stdout = %q", stdout)
	}
	if strings.Count(stderr, "GET /ts/cd/caseMultiStatus/sn") != 2 {
		t.Fatalf("expected two chunks:\n%s", stderr)
	}
	for _, want := range []string{"ids=97054561,97054562", "ids=97054563", "allowDupes=true", "display=true"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, stderr)
		}
	}
}

func TestTrademarkBatchDeduplicatesNormalizedIdentifiers(t *testing.T) {
	originalFlags := trademarkBatchFlags
	originalDryRun := flagDryRun
	defer func() {
		trademarkBatchFlags = originalFlags
		flagDryRun = originalDryRun
	}()
	trademarkBatchFlags.idsFile = ""
	trademarkBatchFlags.allowDupes = false
	trademarkBatchFlags.chunkSize = 25
	flagDryRun = true
	_, stderr, err := captureTrademarkOutput(t, func() error {
		return runTrademarkBatch(trademarkBatchCmd, []string{"serial", "97054561", "sn:97054561", "97-054561"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "ids=97054561") || strings.Contains(stderr, "97054561,97054561") {
		t.Fatalf("normalized equivalents were not deduplicated:\n%s", stderr)
	}
}

func TestTrademarkBatchPreservesBooleanOversizedShape(t *testing.T) {
	originalFlags := trademarkBatchFlags
	originalDryRun, originalFormat, originalMinify, originalQuiet := flagDryRun, flagFormat, flagMinify, flagQuiet
	originalClient := tsdr.DefaultClient
	defer func() {
		trademarkBatchFlags = originalFlags
		flagDryRun, flagFormat, flagMinify, flagQuiet = originalDryRun, originalFormat, originalMinify, originalQuiet
		tsdr.DefaultClient = originalClient
	}()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"transactionList":[{"serialNumber":"97054561"}],"missedElements":[],"oversized":false}`)
	}))
	defer server.Close()
	trademarkBatchFlags = struct {
		idsFile    string
		display    bool
		allowDupes bool
		chunkSize  int
		from       string
		to         string
	}{chunkSize: 25}
	flagDryRun, flagFormat, flagMinify, flagQuiet = false, "json", true, true
	tsdr.DefaultClient = tsdr.NewClient("key", tsdr.WithBaseURL(server.URL), tsdr.WithoutRateLimit())
	testCommand := &cobra.Command{Use: "batch"}
	testCommand.SetContext(context.Background())
	stdout, _, err := captureTrademarkOutput(t, func() error {
		return runTrademarkBatch(testCommand, []string{"serial", "97054561"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Results struct {
			Oversized any `json:"oversized"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if value, ok := envelope.Results.Oversized.(bool); !ok || value {
		t.Fatalf("oversized = %#v, want boolean false", envelope.Results.Oversized)
	}
}

func TestRunTrademarkCaseGetJSONEnvelopeAndSeparateHeader(t *testing.T) {
	originalClient := tsdr.DefaultClient
	originalIDType := trademarkCaseIDType
	originalGetFlags := trademarkCaseGetFlags
	originalFormat := flagFormat
	originalMinify := flagMinify
	defer func() {
		tsdr.DefaultClient = originalClient
		trademarkCaseIDType = originalIDType
		trademarkCaseGetFlags = originalGetFlags
		flagFormat = originalFormat
		flagMinify = originalMinify
	}()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/ts/cd/casestatus/sn97054561/info.json" {
			t.Errorf("request = %s %s", request.Method, request.URL.Path)
		}
		if got := request.Header.Get(tsdr.APIKeyHeader); got != "unit-tsdr-key" {
			t.Errorf("%s = %q", tsdr.APIKeyHeader, got)
		}
		if got := request.Header.Get("X-API-KEY"); got != "" {
			t.Errorf("unexpected ODP key header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"transaction":{"serialNumber":"97054561","mark":"EXAMPLE"}}`)
	}))
	defer server.Close()
	tsdr.DefaultClient = tsdr.NewClient("unit-tsdr-key", tsdr.WithBaseURL(server.URL), tsdr.WithoutRateLimit())
	trademarkCaseIDType = "auto"
	trademarkCaseGetFlags.representation = "json"
	flagFormat = "json"
	flagMinify = true

	command := &cobra.Command{Use: "get"}
	command.SetContext(context.Background())
	stdout, _, err := captureTrademarkOutput(t, func() error {
		return runTrademarkCaseGet(command, []string{"97054561"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	if envelope["ok"] != true || envelope["command"] != "get" {
		t.Fatalf("envelope = %#v", envelope)
	}
	results := envelope["results"].(map[string]any)
	transaction := results["transaction"].(map[string]any)
	if transaction["serialNumber"] != "97054561" || transaction["mark"] != "EXAMPLE" {
		t.Fatalf("transaction = %#v", transaction)
	}
}

func TestTrademarkRawPOSTDryRunPreservesRepeatedAndEmptyParams(t *testing.T) {
	originalFlags := trademarkRequestFlags
	originalDryRun := flagDryRun
	originalClient := tsdr.DefaultClient
	defer func() {
		trademarkRequestFlags = originalFlags
		flagDryRun = originalDryRun
		tsdr.DefaultClient = originalClient
	}()
	trademarkRequestFlags = struct {
		params      []string
		method      string
		body        string
		form        []string
		contentType string
		accept      string
		output      string
		download    bool
		expected    string
		overwrite   bool
	}{
		params: []string{"sn=72131351,76515878", "x=1", "x=2", "assignments=", "prosecutionHistory="},
		method: http.MethodPost,
	}
	flagDryRun = true
	tsdr.DefaultClient = tsdr.NewClient("test", tsdr.WithBaseURL("https://tsdrapi.uspto.gov"), tsdr.WithoutRateLimit())
	_, stderr, err := captureTrademarkOutput(t, func() error {
		return runTrademarkRequest(trademarkRequestCmd, []string{"/ts/cd/example"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "POST /ts/cd/example?sn=72131351,76515878&x=1&x=2&assignments=&prosecutionHistory=") {
		t.Fatalf("raw POST dry-run lost repeated/empty values:\n%s", stderr)
	}
}

func TestTrademarkRawPathRejectsEmbeddedQueryAndFragment(t *testing.T) {
	originalFlags := trademarkRequestFlags
	originalClient := tsdr.DefaultClient
	defer func() {
		trademarkRequestFlags = originalFlags
		tsdr.DefaultClient = originalClient
	}()
	trademarkRequestFlags = struct {
		params      []string
		method      string
		body        string
		form        []string
		contentType string
		accept      string
		output      string
		download    bool
		expected    string
		overwrite   bool
	}{params: []string{"y=2"}, method: http.MethodGet}
	tsdr.DefaultClient = tsdr.NewClient("key", tsdr.WithBaseURL("https://tsdrapi.uspto.gov"), tsdr.WithoutRateLimit())
	for _, path := range []string{"/ts/swagger.json?x=1", "/ts/swagger.json#fragment"} {
		if err := runTrademarkRequest(trademarkRequestCmd, []string{path}); err == nil || !strings.Contains(err.Error(), "must not contain") {
			t.Fatalf("path %q error = %v, want local query/fragment rejection", path, err)
		}
	}
}

func TestTrademarkRawPOSTStreamsValidatedOutput(t *testing.T) {
	originalFlags := trademarkRequestFlags
	originalDryRun, originalFormat, originalQuiet, originalMinify := flagDryRun, flagFormat, flagQuiet, flagMinify
	originalClient := tsdr.DefaultClient
	defer func() {
		trademarkRequestFlags = originalFlags
		flagDryRun, flagFormat, flagQuiet, flagMinify = originalDryRun, originalFormat, originalQuiet, originalMinify
		tsdr.DefaultClient = originalClient
	}()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Query().Get("docs") != "4" {
			t.Errorf("request = %s %s", request.Method, request.URL.String())
		}
		if _, present := request.URL.Query()["assignments"]; !present {
			t.Errorf("explicit empty assignments parameter was dropped")
		}
		if got := request.Header.Get(tsdr.APIKeyHeader); got != "post-key" {
			t.Errorf("key header = %q", got)
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.7\nstreamed post payload"))
	}))
	defer server.Close()
	output := filepath.Join(t.TempDir(), "selected.pdf")
	trademarkRequestFlags.params = []string{"case=false", "docs=4", "assignments=", "prosecutionHistory="}
	trademarkRequestFlags.method = http.MethodPost
	trademarkRequestFlags.body = ""
	trademarkRequestFlags.form = nil
	trademarkRequestFlags.contentType = ""
	trademarkRequestFlags.accept = "application/pdf"
	trademarkRequestFlags.output = output
	trademarkRequestFlags.download = false
	trademarkRequestFlags.expected = "PDF"
	trademarkRequestFlags.overwrite = false
	flagDryRun, flagFormat, flagQuiet, flagMinify = false, "json", true, true
	tsdr.DefaultClient = tsdr.NewClient("post-key", tsdr.WithBaseURL(server.URL), tsdr.WithoutRateLimit())
	if _, _, err := captureTrademarkOutput(t, func() error {
		return runTrademarkRequest(trademarkRequestCmd, []string{"/selected"})
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("streamed output = %q", data)
	}
}

func TestTrademarkRawOutputPreflightAvoidsNetwork(t *testing.T) {
	originalFlags := trademarkRequestFlags
	originalDryRun := flagDryRun
	originalClient := tsdr.DefaultClient
	defer func() {
		trademarkRequestFlags = originalFlags
		flagDryRun = originalDryRun
		tsdr.DefaultClient = originalClient
	}()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.7\nshould not be requested"))
	}))
	defer server.Close()

	output := filepath.Join(t.TempDir(), "existing.pdf")
	if err := os.WriteFile(output, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	trademarkRequestFlags = struct {
		params      []string
		method      string
		body        string
		form        []string
		contentType string
		accept      string
		output      string
		download    bool
		expected    string
		overwrite   bool
	}{method: http.MethodGet, output: output, expected: "pdf"}
	flagDryRun = false
	tsdr.DefaultClient = tsdr.NewClient("key", tsdr.WithBaseURL(server.URL), tsdr.WithoutRateLimit())
	err := runTrademarkRequest(trademarkRequestCmd, []string{"/bundle.pdf"})
	if err == nil || !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("preflight error = %v, want existing-file refusal", err)
	}
	if requests != 0 {
		t.Fatalf("existing output triggered %d network calls, want 0", requests)
	}
}

func TestTrademarkRawBinaryModeRequiresOutputBeforeNetwork(t *testing.T) {
	originalFlags := trademarkRequestFlags
	originalClient := tsdr.DefaultClient
	defer func() {
		trademarkRequestFlags = originalFlags
		tsdr.DefaultClient = originalClient
	}()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte("%PDF-1.7\nshould not be requested"))
	}))
	defer server.Close()
	tsdr.DefaultClient = tsdr.NewClient("key", tsdr.WithBaseURL(server.URL), tsdr.WithoutRateLimit())

	for _, setup := range []struct {
		name     string
		download bool
		expected string
	}{
		{name: "download lane", download: true},
		{name: "expected PDF", expected: "pdf"},
		{name: "expected ZIP", expected: "zip"},
		{name: "expected PNG", expected: "png"},
	} {
		t.Run(setup.name, func(t *testing.T) {
			trademarkRequestFlags = struct {
				params      []string
				method      string
				body        string
				form        []string
				contentType string
				accept      string
				output      string
				download    bool
				expected    string
				overwrite   bool
			}{method: http.MethodGet, download: setup.download, expected: setup.expected}
			err := runTrademarkRequest(trademarkRequestCmd, []string{"/binary"})
			var invalid *invalidArgsError
			if err == nil || !errors.As(err, &invalid) || !strings.Contains(err.Error(), "require --output") {
				t.Fatalf("error = %v, want local invalid-arguments output requirement", err)
			}
		})
	}
	if requests != 0 {
		t.Fatalf("invalid binary invocations triggered %d network calls, want 0", requests)
	}
}

func TestTrademarkRawExpectedTypeValidatesBufferedResponse(t *testing.T) {
	originalFlags := trademarkRequestFlags
	originalClient := tsdr.DefaultClient
	defer func() {
		trademarkRequestFlags = originalFlags
		tsdr.DefaultClient = originalClient
	}()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()
	tsdr.DefaultClient = tsdr.NewClient("key", tsdr.WithBaseURL(server.URL), tsdr.WithoutRateLimit())
	trademarkRequestFlags = struct {
		params      []string
		method      string
		body        string
		form        []string
		contentType string
		accept      string
		output      string
		download    bool
		expected    string
		overwrite   bool
	}{method: http.MethodGet, expected: "xml"}
	testCommand := &cobra.Command{Use: "request"}
	testCommand.SetContext(context.Background())
	err := runTrademarkRequest(testCommand, []string{"/maintenance.json"})
	if err == nil || !strings.Contains(err.Error(), "expected xml") {
		t.Fatalf("mismatched --expected error = %v", err)
	}
}

func TestResolveSelectedDocumentIndexToOriginalServerOrdinal(t *testing.T) {
	originalClient := tsdr.DefaultClient
	originalFlags := trademarkDocsListFlags
	defer func() {
		tsdr.DefaultClient = originalClient
		trademarkDocsListFlags = originalFlags
	}()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<DocumentList>
<Document><SerialNumber>72131351</SerialNumber><MailRoomDate>2024-01-03</MailRoomDate><DocumentTypeCode>OOA</DocumentTypeCode></Document>
<Document><SerialNumber>72131351</SerialNumber><MailRoomDate>2024-01-02</MailRoomDate><DocumentTypeCode>SPE</DocumentTypeCode></Document>
<Document><SerialNumber>72131351</SerialNumber><MailRoomDate>2024-01-01</MailRoomDate><DocumentTypeCode>SPE</DocumentTypeCode></Document>
</DocumentList>`)
	}))
	defer server.Close()
	tsdr.DefaultClient = tsdr.NewClient("key", tsdr.WithBaseURL(server.URL), tsdr.WithoutRateLimit())
	trademarkDocsListFlags = struct {
		idType   string
		idsFile  string
		date     string
		fromDate string
		toDate   string
		types    string
		category string
		sort     string
	}{idType: "auto", types: "SPE", sort: "date:A"}
	id, _ := tsdr.ParseIdentifier("72131351", "serial")
	got, err := resolveSelectedDocumentValues(context.Background(), id, "1,2")
	if err != nil {
		t.Fatal(err)
	}
	if got != "3,2" {
		t.Fatalf("resolved selections = %q, want original server ordinals 3,2", got)
	}
}

func TestTrademarkDocumentIndexCommandsValidateBeforePlanningOrNetwork(t *testing.T) {
	originalListFlags := trademarkDocsListFlags
	originalSelectedFlags := trademarkDocsSelectedFlags
	originalFetchFlags := trademarkDocsFetchFlags
	originalDryRun := flagDryRun
	originalClient := tsdr.DefaultClient
	defer func() {
		trademarkDocsListFlags = originalListFlags
		trademarkDocsSelectedFlags = originalSelectedFlags
		trademarkDocsFetchFlags = originalFetchFlags
		flagDryRun = originalDryRun
		tsdr.DefaultClient = originalClient
	}()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	tsdr.DefaultClient = tsdr.NewClient("key", tsdr.WithBaseURL(server.URL), tsdr.WithoutRateLimit())
	trademarkDocsListFlags.idType = "auto"
	trademarkDocsListFlags.sort = "nonsense"
	trademarkDocsSelectedFlags.asset = "pdf"
	trademarkDocsSelectedFlags.docs = "1"
	flagDryRun = true
	if err := trademarkDocsSelectedCmd.RunE(trademarkDocsSelectedCmd, []string{"sn:72131351"}); err == nil || !strings.Contains(err.Error(), "sort") {
		t.Fatalf("selected invalid-sort error = %v", err)
	}

	trademarkDocsListFlags.sort = ""
	trademarkDocsFetchFlags.page = -1
	if err := trademarkDocsFetchCmd.RunE(trademarkDocsFetchCmd, []string{"sn:97238896", "1"}); err == nil || !strings.Contains(err.Error(), "--page cannot be negative") {
		t.Fatalf("fetch negative-page error = %v", err)
	}
	if requests != 0 {
		t.Fatalf("invalid index commands triggered %d network calls, want 0", requests)
	}
}

func TestWriteDownloadValidatesBeforeOverwriteAndCleansTemporaryFiles(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "nested", "record.pdf")
	invalid := &tsdr.Response{Body: []byte(`{"error":"not a PDF"}`), ContentType: "application/json", URL: "https://example.test/file"}
	if _, err := writeDownload(destination, invalid, "pdf", false); err == nil {
		t.Fatal("expected invalid PDF error")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("invalid payload created destination: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("preserve-me"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := writeDownload(destination, invalid, "pdf", true); err == nil {
		t.Fatal("expected invalid overwrite error")
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "preserve-me" {
		t.Fatalf("validation failure changed existing file to %q", data)
	}

	valid := &tsdr.Response{Body: []byte("%PDF-1.7\nunit test"), ContentType: "application/pdf", URL: "https://example.test/file"}
	if _, err := writeDownload(destination, valid, "pdf", false); err == nil || !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("non-overwrite error = %v", err)
	}
	result, err := writeDownload(destination, valid, "pdf", true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Bytes != int64(len(valid.Body)) || !filepath.IsAbs(result.Path) {
		t.Fatalf("result = %#v", result)
	}
	wantHash := fmt.Sprintf("%x", sha256.Sum256(valid.Body))
	if result.SHA256 != wantHash {
		t.Fatalf("SHA256 = %q, want %q", result.SHA256, wantHash)
	}
	data, err = os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, valid.Body) {
		t.Fatalf("written bytes = %q", data)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(destination), ".uspto-download-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary downloads left behind: %v", matches)
	}
}

func TestReplaceFileNoClobberIsAtomic(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.tmp")
	destination := filepath.Join(dir, "destination.bin")
	if err := os.WriteFile(source, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(source, destination, false); err == nil {
		t.Fatal("replaceFile no-clobber unexpectedly replaced an existing destination")
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing" {
		t.Fatalf("destination changed to %q", data)
	}
}

func TestTrademarkCredentialErrorJSONEnvelope(t *testing.T) {
	originalFormat := flagFormat
	originalMinify := flagMinify
	originalCommand, originalPath, originalProvider := activeCommand, activePath, activeProvider
	defer func() {
		flagFormat = originalFormat
		flagMinify = originalMinify
		activeCommand, activePath, activeProvider = originalCommand, originalPath, originalProvider
	}()
	flagFormat = "json"
	flagMinify = true
	activeCommand = "status"
	activePath = "uspto trademark case status"
	activeProvider = providerTSDR
	stdout, stderr, _ := captureTrademarkOutput(t, func() error {
		code := handleError(&credentialError{
			provider: "TSDR",
			message:  "no trademark TSDR API key configured",
			hint:     "Get a separate key; the patent ODP key will not work",
		})
		if code == 0 {
			t.Fatal("authentication error returned success exit code")
		}
		return nil
	})
	if stderr != "" {
		t.Fatalf("JSON error wrote stderr = %q", stderr)
	}
	var envelope struct {
		OK          bool   `json:"ok"`
		Command     string `json:"command"`
		CommandPath string `json:"commandPath"`
		Provider    string `json:"provider"`
		Error       struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Hint    string `json:"hint"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	if envelope.OK || envelope.Command != "status" || envelope.CommandPath != "uspto trademark case status" || envelope.Provider != "tsdr" || envelope.Error.Type != "AUTH_FAILURE" || !strings.Contains(envelope.Error.Hint, "patent ODP key will not work") {
		t.Fatalf("error envelope = %#v", envelope)
	}
}

func TestTrademarkProviderRequestErrorsUseActionableExitTypes(t *testing.T) {
	originalFormat := flagFormat
	originalMinify := flagMinify
	originalCommand, originalPath, originalProvider := activeCommand, activePath, activeProvider
	defer func() {
		flagFormat = originalFormat
		flagMinify = originalMinify
		activeCommand, activePath, activeProvider = originalCommand, originalPath, originalProvider
	}()
	flagFormat, flagMinify = "json", true
	activeCommand, activePath = "request", "uspto trademark request"

	tests := []struct {
		name     string
		provider apiProvider
		err      error
		wantType string
	}{
		{name: "TSDR bad request", provider: providerTSDR, err: &tsdr.APIError{StatusCode: 400, Message: "Bad Request", Body: "bad selector"}, wantType: "INVALID_ARGUMENTS"},
		{name: "TSDR not acceptable", provider: providerTSDR, err: &tsdr.APIError{StatusCode: 406, Message: "Not Acceptable"}, wantType: "NOT_ACCEPTABLE"},
		{name: "Trademark Search bad request", provider: providerTMSearch, err: &tmsearch.HTTPError{StatusCode: 400, Status: "400 Bad Request", Body: "bad query"}, wantType: "INVALID_ARGUMENTS"},
		{name: "Trademark Search not acceptable", provider: providerTMSearch, err: &tmsearch.HTTPError{StatusCode: 406, Status: "406 Not Acceptable"}, wantType: "NOT_ACCEPTABLE"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			activeProvider = test.provider
			stdout, stderr, err := captureTrademarkOutput(t, func() error {
				if code := handleError(test.err); code != 2 {
					t.Fatalf("handleError() code = %d, want 2", code)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if stderr != "" {
				t.Fatalf("structured error wrote stderr = %q", stderr)
			}
			var envelope struct {
				Error struct {
					Type string `json:"type"`
					Hint string `json:"hint"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
				t.Fatalf("decode output %q: %v", stdout, err)
			}
			if envelope.Error.Type != test.wantType || envelope.Error.Hint == "" {
				t.Fatalf("error envelope = %#v, want type %q with hint", envelope, test.wantType)
			}
		})
	}
}

func TestTrademarkForbiddenTextShowsRouteHint(t *testing.T) {
	originalFormat := flagFormat
	defer func() { flagFormat = originalFormat }()
	flagFormat = "table"
	_, stderr, err := captureTrademarkOutput(t, func() error {
		if code := handleError(&tsdr.APIError{StatusCode: 403, Message: "Forbidden"}); code != 3 {
			t.Fatalf("handleError() code = %d, want 3", code)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "api-spec") || !strings.Contains(stderr, "GET alias") {
		t.Fatalf("forbidden stderr lacks route-vs-key recovery hint:\n%s", stderr)
	}
}

func TestTrademarkNDJSONErrorIsExactlyOneJSONLine(t *testing.T) {
	originalFormat, originalMinify := flagFormat, flagMinify
	defer func() { flagFormat, flagMinify = originalFormat, originalMinify }()
	flagFormat, flagMinify = "ndjson", false
	stdout, stderr, err := captureTrademarkOutput(t, func() error {
		if code := handleError(&invalidArgsError{message: "bad input"}); code != 2 {
			t.Fatalf("handleError() code = %d, want 2", code)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("NDJSON error wrote stderr = %q", stderr)
	}
	line := strings.TrimSuffix(stdout, "\n")
	if strings.Contains(line, "\n") || !json.Valid([]byte(line)) {
		t.Fatalf("NDJSON error is not exactly one valid JSON line: %q", stdout)
	}
}

func TestTrademarkRateLimitErrorsExposeRetryAfterSeconds(t *testing.T) {
	originalFormat, originalMinify := flagFormat, flagMinify
	defer func() { flagFormat, flagMinify = originalFormat, originalMinify }()
	flagFormat, flagMinify = "json", true
	tests := []struct {
		name string
		err  error
	}{
		{name: "TSDR", err: &tsdr.APIError{StatusCode: http.StatusTooManyRequests, Message: "Too Many Requests", RetryAfter: 75 * time.Second}},
		{name: "Trademark Search", err: &tmsearch.HTTPError{StatusCode: http.StatusTooManyRequests, Status: "429 Too Many Requests", Header: http.Header{"Retry-After": {"90"}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, _, err := captureTrademarkOutput(t, func() error {
				if code := handleError(test.err); code != 5 {
					t.Fatalf("handleError() code = %d, want 5", code)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Error struct {
					RetryAfterSeconds int64 `json:"retryAfterSeconds"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
				t.Fatal(err)
			}
			want := int64(75)
			if test.name == "Trademark Search" {
				want = 90
			}
			if envelope.Error.RetryAfterSeconds != want {
				t.Fatalf("retryAfterSeconds = %d, want %d", envelope.Error.RetryAfterSeconds, want)
			}
		})
	}
}

func TestTrademarkCLIValidationFailuresUseInvalidArgumentsJSON(t *testing.T) {
	originalSearchFlags := trademarkSearchFlags
	originalCaseIDType, originalCaseIDsFile := trademarkCaseIDType, trademarkCaseIDsFile
	originalFormat, originalMinify, originalDryRun := flagFormat, flagMinify, flagDryRun
	originalCommand, originalPath, originalProvider := activeCommand, activePath, activeProvider
	defer func() {
		trademarkSearchFlags = originalSearchFlags
		trademarkCaseIDType, trademarkCaseIDsFile = originalCaseIDType, originalCaseIDsFile
		flagFormat, flagMinify, flagDryRun = originalFormat, originalMinify, originalDryRun
		activeCommand, activePath, activeProvider = originalCommand, originalPath, originalProvider
	}()

	tests := []struct {
		name        string
		command     *cobra.Command
		prepare     func()
		run         func() error
		wantMessage string
	}{
		{
			name:    "trademark search OPENAI --limit 0 -f json",
			command: trademarkSearchCmd,
			prepare: func() {
				resetTrademarkSearchFlagsForTest()
				trademarkSearchFlags.limit = 0
				flagDryRun = false
			},
			run:         func() error { return runTrademarkSearch(trademarkSearchCmd, []string{"OPENAI"}) },
			wantMessage: "--limit must be between 1 and 1000",
		},
		{
			name:    "trademark case status abc --dry-run -f json",
			command: trademarkCaseStatusCmd,
			prepare: func() {
				trademarkCaseIDType = "auto"
				trademarkCaseIDsFile = ""
				flagDryRun = true
			},
			run:         func() error { return trademarkCaseStatusCmd.RunE(trademarkCaseStatusCmd, []string{"abc"}) },
			wantMessage: "cannot infer identifier type",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.prepare()
			flagFormat, flagMinify = "json", true
			setActiveCommand(test.command)
			err := test.run()
			if err == nil {
				t.Fatal("expected validation error")
			}
			stdout, stderr, captureErr := captureTrademarkOutput(t, func() error {
				if code := handleError(err); code != 2 {
					t.Fatalf("handleError() code = %d, want 2", code)
				}
				return nil
			})
			if captureErr != nil {
				t.Fatal(captureErr)
			}
			if stderr != "" {
				t.Fatalf("JSON validation error wrote stderr = %q", stderr)
			}
			var envelope struct {
				OK    bool `json:"ok"`
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
				t.Fatalf("decode output %q: %v", stdout, err)
			}
			if envelope.OK || envelope.Error.Type != "INVALID_ARGUMENTS" || !strings.Contains(envelope.Error.Message, test.wantMessage) {
				t.Fatalf("error envelope = %#v", envelope)
			}
		})
	}
}

func TestTrademarkPackageOptionErrorsAreMarkedInvalidArguments(t *testing.T) {
	originalSearchFlags, originalDocsFlags := trademarkSearchFlags, trademarkDocsListFlags
	defer func() {
		trademarkSearchFlags, trademarkDocsListFlags = originalSearchFlags, originalDocsFlags
	}()

	resetTrademarkSearchFlagsForTest()
	trademarkSearchFlags.sort = ":asc"
	searchErr := runTrademarkSearch(trademarkSearchCmd, []string{"OPENAI"})
	var searchInvalid *invalidArgsError
	if searchErr == nil || !errors.As(searchErr, &searchInvalid) {
		t.Fatalf("tmsearch option error = %v, want *invalidArgsError", searchErr)
	}

	trademarkDocsListFlags = struct {
		idType   string
		idsFile  string
		date     string
		fromDate string
		toDate   string
		types    string
		category string
		sort     string
	}{idType: "serial", date: "2024/01/01"}
	_, documentErr := documentQuery([]string{"97054561"})
	var documentInvalid *invalidArgsError
	if documentErr == nil || !errors.As(documentErr, &documentInvalid) {
		t.Fatalf("tsdr option error = %v, want *invalidArgsError", documentErr)
	}
}

func TestTrademarkSearchResponseFailureRemainsGeneralError(t *testing.T) {
	originalFlags, originalClient := trademarkSearchFlags, tmsearch.DefaultClient
	originalFormat, originalMinify := flagFormat, flagMinify
	originalCommand, originalPath, originalProvider := activeCommand, activePath, activeProvider
	defer func() {
		trademarkSearchFlags, tmsearch.DefaultClient = originalFlags, originalClient
		flagFormat, flagMinify = originalFormat, originalMinify
		activeCommand, activePath, activeProvider = originalCommand, originalPath, originalProvider
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"incomplete":`)
	}))
	defer server.Close()

	resetTrademarkSearchFlagsForTest()
	tmsearch.DefaultClient = tmsearch.NewClient(tmsearch.WithBaseURL(server.URL))
	err := runTrademarkSearch(trademarkSearchCmd, []string{"OPENAI"})
	if err == nil {
		t.Fatal("expected Trademark Search response error")
	}
	var invalid *invalidArgsError
	if errors.As(err, &invalid) {
		t.Fatalf("response error was misclassified as invalid arguments: %v", err)
	}

	flagFormat, flagMinify = "json", true
	setActiveCommand(trademarkSearchCmd)
	stdout, stderr, captureErr := captureTrademarkOutput(t, func() error {
		if code := handleError(err); code != 1 {
			t.Fatalf("handleError() code = %d, want 1", code)
		}
		return nil
	})
	if captureErr != nil {
		t.Fatal(captureErr)
	}
	if stderr != "" {
		t.Fatalf("JSON response error wrote stderr = %q", stderr)
	}
	var envelope struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	if envelope.Error.Type != "GENERAL_ERROR" {
		t.Fatalf("error envelope = %#v, want GENERAL_ERROR", envelope)
	}
}

func TestTrademarkRawSearchRejectsIgnoredTypedShapingFlags(t *testing.T) {
	originalFlags := trademarkSearchFlags
	defer func() { trademarkSearchFlags = originalFlags }()
	resetTrademarkSearchFlagsForTest()
	trademarkSearchFlags.rawBody = "unused.json"

	for _, name := range []string{"limit", "offset", "max-results", "fields", "sort", "facets"} {
		t.Run(name, func(t *testing.T) {
			flag := trademarkSearchCmd.Flags().Lookup(name)
			if flag == nil {
				t.Fatalf("flag --%s is missing", name)
			}
			originalChanged := flag.Changed
			flag.Changed = true
			defer func() { flag.Changed = originalChanged }()
			err := runTrademarkSearch(trademarkSearchCmd, nil)
			var invalid *invalidArgsError
			if err == nil || !errors.As(err, &invalid) || !strings.Contains(err.Error(), "--"+name) {
				t.Fatalf("error = %v, want explicit --%s conflict", err, name)
			}
		})
	}
}

func TestTrademarkGroupRejectsMissingNestingWithHint(t *testing.T) {
	previousCommand, previousPath, previousProvider := activeCommand, activePath, activeProvider
	defer func() {
		activeCommand, activePath, activeProvider = previousCommand, previousPath, previousProvider
	}()
	err := validateTrademarkGroupArgs(trademarkCmd, []string{"status", "rn:916522"})
	if err == nil {
		t.Fatal("validateTrademarkGroupArgs() expected an error")
	}
	if _, ok := err.(*invalidArgsError); !ok {
		t.Fatalf("error type = %T, want *invalidArgsError", err)
	}
	if !strings.Contains(err.Error(), "uspto trademark case status") {
		t.Fatalf("error = %q, want case nesting hint", err)
	}
	if code := handleError(err); code != 2 {
		t.Fatalf("handleError() code = %d, want 2", code)
	}
}

func TestNativeAssetExtensionPrefersActualContentType(t *testing.T) {
	if got := nativeAssetExtension("https://tmng-al.uspto.gov/page.jpg", "image/png"); got != ".png" {
		t.Fatalf("nativeAssetExtension() = %q, want .png", got)
	}
	if got := nativeAssetExtension("https://tmng-al.uspto.gov/page.jpeg", "application/octet-stream"); got != ".jpg" {
		t.Fatalf("nativeAssetExtension() fallback = %q, want .jpg", got)
	}
}

func TestValidateDownloadRejectsUnknownExpectedType(t *testing.T) {
	if err := validateDownload([]byte("payload"), "application/octet-stream", "invented"); err == nil || !strings.Contains(err.Error(), "unsupported expected payload type") {
		t.Fatalf("validateDownload() error = %v", err)
	}
}

func TestCompactMaintenanceAddsIdentityAndFormLabel(t *testing.T) {
	id, err := tsdr.ParseIdentifier("5711111", "registration")
	if err != nil {
		t.Fatal(err)
	}
	result := compactMaintenance(id, map[string]any{
		"earliestDate":   "2028-03-26",
		"nextFormToFile": float64(89),
	})
	if result["identifier"] != "rn5711111" || result["nextForm"] != "Combined Sections 8 and 9" {
		t.Fatalf("compact maintenance = %#v", result)
	}
}
