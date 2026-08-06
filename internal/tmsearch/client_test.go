package tmsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientDiscoversAndSearches(t *testing.T) {
	var configurationCalls atomic.Int32
	var searchCalls atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/configuration.json":
			configurationCalls.Add(1)
			if r.Method != http.MethodGet {
				t.Errorf("configuration method = %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"serviceUrlSearchElastic":%q,"ignoredFutureValue":true}`, server.URL+"/prod-v9/")
		case "/prod-v9/tmsearch":
			call := searchCalls.Add(1)
			if r.Method != http.MethodPost {
				t.Errorf("search method = %s", r.Method)
			}
			if got := r.Header.Get("Accept"); got != "application/json" {
				t.Errorf("Accept = %q", got)
			}
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q", got)
			}
			if got := r.Header.Get("X-API-KEY"); got != "" {
				t.Errorf("keyless Trademark Search unexpectedly sent X-API-KEY = %q", got)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decoding request: %v", err)
			}
			query := body["query"].(map[string]any)["query_string"].(map[string]any)["query"]
			wantQuery := "CM:USPTO"
			if call == 2 {
				wantQuery = "WM:TEST"
			}
			if query != wantQuery {
				t.Errorf("query = %#v, want %q", query, wantQuery)
			}
			if body["track_total_hits"] != true || body["size"] != float64(3) {
				t.Errorf("request body = %#v", body)
			}
			fmt.Fprint(w, `{
				"took":2,"timedOut":false,"shardsTotal":5,"shardsSuccessful":5,"shardsSkipped":0,"shardsFailed":0,
				"hits":{"totalValue":24,"totalRelation":"eq","maxScore":10.5,"hits":[{"index":"tmsearch-index","type":"_doc","id":"76543210","score":10.5,"source":{"id":"76543210","wordmark":"USPTO"}}]},
				"aggregations":{"classes":{"doc_count_error_upper_bound":0,"sum_other_doc_count":0,"buckets":[{"key":"IC 042","doc_count":9}]}}
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var debug bytes.Buffer
	client := NewClient(
		WithConfigurationURL(server.URL+"/configuration.json"),
		WithDebug(true),
		WithDebugWriter(&debug),
	)
	response, err := client.SearchQuery(context.Background(), "CM:USPTO", SearchOptions{
		Limit:  3,
		Source: []string{"id", "wordmark"},
		Facets: []TermFacet{{Name: "classes", Field: "internationalClassExact", Size: 10}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Hits.TotalValue != 24 || len(response.Hits.Hits) != 1 || response.Hits.Hits[0].ID != "76543210" {
		t.Fatalf("response = %#v", response)
	}
	if !json.Valid(response.Raw) {
		t.Fatalf("raw response was not retained: %s", response.Raw)
	}
	if configurationCalls.Load() != 1 || searchCalls.Load() != 1 {
		t.Fatalf("calls: configuration=%d search=%d", configurationCalls.Load(), searchCalls.Load())
	}
	if !strings.Contains(debug.String(), "configuration discovery") || !strings.Contains(debug.String(), "search") || strings.Contains(debug.String(), "CM:USPTO") {
		t.Fatalf("unexpected debug log: %q", debug.String())
	}

	// A second search uses the cached discovered base URL.
	if _, err := client.SearchQuery(context.Background(), "WM:TEST", SearchOptions{Limit: 3}); err != nil {
		t.Fatal(err)
	}
	if configurationCalls.Load() != 1 || searchCalls.Load() != 2 {
		t.Fatalf("cached calls: configuration=%d search=%d", configurationCalls.Load(), searchCalls.Load())
	}
}

func TestClientBaseOverrideSkipsDiscoveryAndAcceptsEndpointURL(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/service/tmsearch" {
			t.Errorf("path = %q", r.URL.Path)
		}
		fmt.Fprint(w, `{"hits":{"totalValue":0,"hits":[]}}`)
	}))
	defer server.Close()

	client := NewClient(
		WithConfigurationURL("http://invalid.example.test/never-called"),
		WithBaseURL(server.URL+"/service/tmsearch"),
	)
	if _, err := client.SearchQuery(context.Background(), "WM:TEST", SearchOptions{}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d", calls.Load())
	}
}

func TestClientTypedSearchHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		http.Error(w, `{"message":"temporarily throttled"}`, http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.SearchQuery(context.Background(), "WM:TEST", SearchOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	var httpError *HTTPError
	if !errors.As(err, &httpError) {
		t.Fatalf("error type = %T, want *HTTPError: %v", err, err)
	}
	if httpError.Operation != "search" || httpError.StatusCode != http.StatusTooManyRequests || !httpError.IsRetryable() {
		t.Fatalf("HTTP error = %#v", httpError)
	}
	if httpError.Header.Get("Retry-After") != "3" || !strings.Contains(httpError.Body, "temporarily throttled") {
		t.Fatalf("HTTP error details = %#v", httpError)
	}
}

func TestClientTypedConfigurationHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "maintenance", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(WithConfigurationURL(server.URL))
	_, err := client.Discover(context.Background())
	var httpError *HTTPError
	if !errors.As(err, &httpError) {
		t.Fatalf("error = %T %v, want *HTTPError", err, err)
	}
	if httpError.Operation != "configuration discovery" || !httpError.IsRetryable() {
		t.Fatalf("HTTP error = %#v", httpError)
	}
}

func TestClientInvalidConfiguration(t *testing.T) {
	tests := []string{
		`{}`,
		`{"serviceUrlSearchElastic":"file:///tmp/search"}`,
		`not-json`,
	}
	for _, response := range tests {
		t.Run(response, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, response)
			}))
			defer server.Close()
			client := NewClient(WithConfigurationURL(server.URL))
			if _, err := client.Discover(context.Background()); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}

func TestSearchRawValidationAndInvalidResponse(t *testing.T) {
	client := NewClient(WithBaseURL("http://example.test"))
	for _, body := range []json.RawMessage{nil, []byte("{"), []byte("plain text")} {
		if _, err := client.SearchRaw(context.Background(), body); err == nil {
			t.Fatalf("expected invalid body error for %q", body)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not JSON")
	}))
	defer server.Close()
	client = NewClient(WithBaseURL(server.URL))
	if _, err := client.SearchRaw(context.Background(), json.RawMessage(`{"query":{"match_all":{}}}`)); err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("error = %v", err)
	}
}

func TestClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL), WithTimeout(10*time.Millisecond))
	_, err := client.SearchQuery(context.Background(), "WM:TEST", SearchOptions{})
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestHTTPErrorRetryable(t *testing.T) {
	for _, test := range []struct {
		status int
		want   bool
	}{
		{http.StatusBadRequest, false},
		{http.StatusRequestTimeout, true},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, true},
	} {
		if got := (&HTTPError{StatusCode: test.status}).IsRetryable(); got != test.want {
			t.Errorf("status %d retryable = %v, want %v", test.status, got, test.want)
		}
	}
}

func TestRetryAfterAboveCapReturnsWithoutEarlyRetry(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "60")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()
	client := NewClient(WithBaseURL(server.URL))
	started := time.Now()
	_, err := client.SearchQuery(context.Background(), "CM:TEST", SearchOptions{})
	if err == nil {
		t.Fatal("SearchQuery() expected a rate-limit error")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.RetryAfter() != time.Minute {
		t.Fatalf("SearchQuery() error = %T %v, want 60s HTTPError", err, err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("over-cap Retry-After delayed return by %v", elapsed)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("server calls = %d, want no early retry", got)
	}
}

func TestHTTPErrorPrintableBodyIsCapped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(strings.Repeat("x", maxErrorDetail*2)))
	}))
	defer server.Close()
	client := NewClient(WithBaseURL(server.URL))
	_, err := client.SearchQuery(context.Background(), "CM:TEST", SearchOptions{})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error = %T %v, want HTTPError", err, err)
	}
	if len(httpErr.Body) != maxErrorDetail+3 || !strings.HasSuffix(httpErr.Body, "...") {
		t.Fatalf("bounded body length/suffix = %d/%q", len(httpErr.Body), httpErr.Body[len(httpErr.Body)-3:])
	}
}

func TestOfficialURLValidation(t *testing.T) {
	for _, raw := range []string{
		"http://tmsearch.uspto.gov/prod/",
		"https://tmsearch.uspto.gov.evil.example/prod/",
		"https://evil.example/prod/",
		"https://user@tmsearch.uspto.gov/prod/",
	} {
		if err := validateOfficialUSPTOURL(raw); err == nil {
			t.Errorf("validateOfficialUSPTOURL(%q) accepted unsafe URL", raw)
		}
	}
	if err := validateOfficialUSPTOURL("https://tmsearch.uspto.gov/prod-v1/"); err != nil {
		t.Fatalf("official URL rejected: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestDefaultDiscoveryRejectsNonUSPTOService(t *testing.T) {
	client := NewClient()
	client.httpClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := io.NopCloser(strings.NewReader(`{"serviceUrlSearchElastic":"https://evil.example/prod/"}`))
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Content-Type": {"application/json"}}, Body: body, Request: request}, nil
	})
	if _, err := client.Discover(context.Background()); err == nil || !strings.Contains(err.Error(), "uspto.gov") {
		t.Fatalf("Discover() error = %v, want non-USPTO rejection", err)
	}
}

func TestTrademarkSearchRedirectTrust(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{}`)
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/same" {
			http.Redirect(w, r, "/final", http.StatusTemporaryRedirect)
			return
		}
		if r.URL.Path == "/cross" {
			http.Redirect(w, r, target.URL+"/stolen", http.StatusTemporaryRedirect)
			return
		}
		if r.URL.Path == "/loop" {
			http.Redirect(w, r, "/loop", http.StatusTemporaryRedirect)
			return
		}
		fmt.Fprint(w, `{}`)
	}))
	defer source.Close()

	custom := NewClient(WithBaseURL(source.URL))
	request, _ := http.NewRequest(http.MethodPost, source.URL+"/same", strings.NewReader(`{"secret":"query"}`))
	if response, err := custom.doOnce(request, "test"); err != nil {
		t.Fatalf("same-origin custom redirect rejected: %v", err)
	} else {
		_ = response.Body.Close()
	}
	request, _ = http.NewRequest(http.MethodPost, source.URL+"/cross", strings.NewReader(`{"secret":"query"}`))
	if _, err := custom.doOnce(request, "test"); err == nil || !strings.Contains(err.Error(), "cross-origin redirect") {
		t.Fatalf("custom cross-origin redirect error = %v", err)
	}
	request, _ = http.NewRequest(http.MethodPost, source.URL+"/loop", strings.NewReader(`{"secret":"query"}`))
	if _, err := custom.doOnce(request, "test"); err == nil || !strings.Contains(err.Error(), "10 Trademark Search redirects") {
		t.Fatalf("custom redirect loop error = %v", err)
	}

	officialPolicy := NewClient()
	officialPolicy.httpClient = source.Client()
	request, _ = http.NewRequest(http.MethodPost, source.URL+"/cross", strings.NewReader(`{"secret":"query"}`))
	if _, err := officialPolicy.doOnce(request, "test"); err == nil || !strings.Contains(err.Error(), "Trademark Search redirect") {
		t.Fatalf("official redirect policy error = %v", err)
	}
}
