// Package api implements the USPTO Open Data Portal API client with rate
// limiting, automatic retry on 429, file-based cross-process rate limit
// state, and all documented endpoints.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/smcronin/uspto-cli/internal/types"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	// DefaultBaseURL is the USPTO Open Data Portal API root.
	DefaultBaseURL = "https://api.uspto.gov"

	// defaultTimeout is the HTTP timeout for metadata/search requests.
	defaultTimeout = 30 * time.Second

	// downloadTimeout is the HTTP timeout for file downloads. The API docs
	// recommend 600 seconds because large files may have a long initial
	// delay before content starts streaming.
	downloadTimeout = 600 * time.Second

	// minRequestGap is the minimum gap between consecutive requests.
	// The API burst limit is 1 (sequential only).
	minRequestGap = 100 * time.Millisecond

	// retryWait429 is how long to wait after a 429 before retrying.
	// USPTO docs say "wait at least 5 seconds."
	retryWait429 = 5 * time.Second

	// maxRetries is the number of times to retry on 429 responses.
	maxRetries = 3

	// rateLimitFile is the name of the file used for cross-process rate
	// limit state. It is stored in the OS temp directory.
	rateLimitFile = "uspto-cli-ratelimit"
)

// ---------------------------------------------------------------------------
// UsptoAPIError
// ---------------------------------------------------------------------------

// UsptoAPIError represents an error returned by the USPTO API.
type UsptoAPIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *UsptoAPIError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("API error (%d): %s -- %s", e.StatusCode, e.Message, e.Body)
	}
	return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Message)
}

// IsRetryable returns true for status codes that may succeed on retry.
func (e *UsptoAPIError) IsRetryable() bool {
	return e.StatusCode == 429 || e.StatusCode >= 500
}

// ---------------------------------------------------------------------------
// RateLimiter (in-process + file-based cross-process)
// ---------------------------------------------------------------------------

// rateLimiter enforces sequential requests (burst limit = 1) with a minimum
// gap of 100 ms between requests. On 429 it forces a 5 s wait. Timestamp
// state is persisted to a temp file so concurrent CLI invocations sharing
// the same API key do not race.
type rateLimiter struct {
	mu             sync.Mutex
	lastRequestEnd time.Time
	retryAfter     time.Time
}

// newRateLimiter creates a rate limiter and loads the last-request timestamp
// from the cross-process state file.
func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{}
	rl.loadState()
	return rl
}

// stateFilePath returns the full path to the rate limiter state file.
func stateFilePath() string {
	return filepath.Join(os.TempDir(), rateLimitFile)
}

// loadState reads the last-request timestamp from the temp file.
func (rl *rateLimiter) loadState() {
	data, err := os.ReadFile(stateFilePath())
	if err != nil {
		return
	}
	ts := strings.TrimSpace(string(data))
	nsec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return
	}
	rl.lastRequestEnd = time.Unix(0, nsec)
}

// saveState writes the last-request timestamp to the temp file.
func (rl *rateLimiter) saveState() {
	data := []byte(strconv.FormatInt(rl.lastRequestEnd.UnixNano(), 10))
	_ = os.WriteFile(stateFilePath(), data, 0644)
}

// waitForSlot blocks until a request slot is available. It respects both
// the 429 retry-after delay and the 100 ms minimum gap.
func (rl *rateLimiter) waitForSlot() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Respect 429 retry-after.
	if rl.retryAfter.After(now) {
		time.Sleep(time.Until(rl.retryAfter))
		now = time.Now()
	}

	// Enforce minimum gap.
	elapsed := now.Sub(rl.lastRequestEnd)
	if elapsed < minRequestGap {
		time.Sleep(minRequestGap - elapsed)
	}
}

// markRequestComplete records the current time as the end of the most
// recent request and persists it for cross-process coordination.
func (rl *rateLimiter) markRequestComplete() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.lastRequestEnd = time.Now()
	rl.saveState()
}

// markRateLimited sets the retry-after timestamp to now + 5 s.
func (rl *rateLimiter) markRateLimited() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.retryAfter = time.Now().Add(retryWait429)
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

// Client is the USPTO Open Data Portal API client.
type Client struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
	debug      bool
	timeout    time.Duration
	rl         *rateLimiter
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithTimeout sets the default HTTP client timeout for non-download requests.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = d
		c.httpClient.Timeout = d
	}
}

// WithDebug enables debug logging to stderr.
func WithDebug(debug bool) ClientOption {
	return func(c *Client) {
		c.debug = debug
	}
}

// WithBaseURL overrides the default API base URL.
func WithBaseURL(u string) ClientOption {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(u, "/")
	}
}

// NewClient creates a new USPTO API client.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
			// We handle redirects manually so that we can re-apply the API
			// key header after a redirect (the stdlib strips headers on
			// cross-origin redirects).
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		timeout: defaultTimeout,
		rl:      newRateLimiter(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// DefaultClient is the global API client singleton, initialised during
// PersistentPreRun.
var DefaultClient *Client

// ---------------------------------------------------------------------------
// GetBaseURL returns the base URL of the API client.
func (c *Client) GetBaseURL() string {
	return c.baseURL
}

// DryRunURL constructs the full URL that would be sent for a request,
// without actually executing it. Useful for --dry-run output.
func (c *Client) DryRunURL(method, path, query string, opts types.SearchOptions) string {
	params := make(map[string]string)
	if query != "" {
		params["q"] = query
	}
	if opts.Limit > 0 {
		params["limit"] = fmt.Sprintf("%d", opts.Limit)
	}
	if opts.Offset > 0 {
		params["offset"] = fmt.Sprintf("%d", opts.Offset)
	}
	if opts.Sort != "" {
		params["sort"] = opts.Sort
	}
	return method + " " + c.buildURL(path, params)
}

// Internal helpers
// ---------------------------------------------------------------------------

// debugf logs a formatted message to stderr when debug mode is on.
func (c *Client) debugf(format string, args ...interface{}) {
	if c.debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

// setHeaders applies common headers to a request.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("X-API-KEY", c.apiKey)
	req.Header.Set("Accept", "application/json")
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
}

// buildURL constructs a full URL from a path and optional query parameters.
func (c *Client) buildURL(path string, params map[string]string) string {
	u := c.baseURL + path
	if len(params) == 0 {
		return u
	}
	qv := url.Values{}
	for k, v := range params {
		if v != "" {
			qv.Set(k, v)
		}
	}
	qs := qv.Encode()
	if qs != "" {
		u += "?" + qs
	}
	return u
}

// followRedirects performs the request and manually follows 301/302
// redirects, re-applying the API key header on each hop (up to 10).
func (c *Client) followRedirects(req *http.Request) (*http.Response, error) {
	const maxRedirects = 10
	for i := 0; i < maxRedirects; i++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusMovedPermanently &&
			resp.StatusCode != http.StatusFound &&
			resp.StatusCode != http.StatusTemporaryRedirect &&
			resp.StatusCode != http.StatusPermanentRedirect {
			return resp, nil
		}
		loc := resp.Header.Get("Location")
		resp.Body.Close()
		if loc == "" {
			return nil, fmt.Errorf("redirect with no Location header")
		}
		c.debugf("Following redirect -> %s", loc)
		next, err := http.NewRequestWithContext(req.Context(), http.MethodGet, loc, nil)
		if err != nil {
			return nil, fmt.Errorf("building redirect request: %w", err)
		}
		c.setHeaders(next)
		req = next
	}
	return nil, fmt.Errorf("too many redirects (>%d)", maxRedirects)
}

// request performs a JSON API request with rate limiting, debug logging,
// and automatic retry on 429.
func (c *Client) request(ctx context.Context, method, path string, body interface{}, params map[string]string) ([]byte, error) {
	fullURL := c.buildURL(path, params)

	var attempt int
	for {
		// Wait for a rate limit slot.
		c.rl.waitForSlot()

		// Build the request body.
		var bodyReader io.Reader
		if body != nil {
			encoded, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("marshalling request body: %w", err)
			}
			bodyReader = bytes.NewReader(encoded)
			if c.debug {
				c.debugf("%s %s  body=%s", method, fullURL, string(encoded))
			}
		} else {
			c.debugf("%s %s", method, fullURL)
		}

		req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("building request: %w", err)
		}
		c.setHeaders(req)

		resp, err := c.followRedirects(req)
		c.rl.markRequestComplete()
		if err != nil {
			return nil, fmt.Errorf("executing request: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}

		c.debugf("Response: %d  (%d bytes)", resp.StatusCode, len(respBody))

		// Handle non-2xx.
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// On 429, retry up to maxRetries times.
			if resp.StatusCode == 429 && attempt < maxRetries {
				c.rl.markRateLimited()
				attempt++
				c.debugf("429 rate limited, retry %d/%d after %s", attempt, maxRetries, retryWait429)
				continue
			}

			// Parse the error body for a better message.
			var errResp types.ErrorResponse
			msg := http.StatusText(resp.StatusCode)
			if jsonErr := json.Unmarshal(respBody, &errResp); jsonErr == nil {
				if errResp.Error != "" {
					msg = errResp.Error
				} else if errResp.Message != "" {
					msg = errResp.Message
				}
				if errResp.ErrorDetails != "" {
					msg += " -- " + errResp.ErrorDetails
				} else if errResp.DetailedMessage != "" {
					msg += " -- " + errResp.DetailedMessage
				}
			}

			return nil, &UsptoAPIError{
				StatusCode: resp.StatusCode,
				Message:    msg,
				Body:       string(respBody),
			}
		}

		return respBody, nil
	}
}

// requestJSON is a typed wrapper around request that unmarshals the response
// into T.
func requestJSON[T any](c *Client, ctx context.Context, method, path string, body interface{}, params map[string]string) (*T, error) {
	data, err := c.request(ctx, method, path, body, params)
	if err != nil {
		return nil, err
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decoding response JSON: %w (first 500 bytes: %s)", err, truncate(string(data), 500))
	}
	return &result, nil
}

// truncate shortens s to at most n characters.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// searchParams builds the query parameter map used by most GET search
// endpoints.
func searchParams(query string, opts types.SearchOptions) map[string]string {
	p := make(map[string]string)
	if query != "" {
		p["q"] = query
	}
	if opts.Limit > 0 {
		p["limit"] = strconv.Itoa(opts.Limit)
	}
	if opts.Offset > 0 {
		p["offset"] = strconv.Itoa(opts.Offset)
	}
	if opts.Sort != "" {
		p["sort"] = opts.Sort
	}
	if opts.Fields != "" {
		p["fields"] = opts.Fields
	}
	if opts.Filters != "" {
		p["filters"] = opts.Filters
	}
	if opts.Facets != "" {
		p["facets"] = opts.Facets
	}
	return p
}

// ==========================================================================
// Patent File Wrapper Endpoints
// ==========================================================================

// SearchPatents performs a GET-based patent application search using
// simplified query syntax.
func (c *Client) SearchPatents(ctx context.Context, query string, opts types.SearchOptions) (*types.PatentDataResponse, error) {
	return requestJSON[types.PatentDataResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/applications/search", nil, searchParams(query, opts))
}

// SearchPatentsPost performs a POST-based patent application search using
// the advanced JSON body syntax (structured filters, range filters, facets,
// field projection, sort).
func (c *Client) SearchPatentsPost(ctx context.Context, body types.SearchRequest) (*types.PatentDataResponse, error) {
	return requestJSON[types.PatentDataResponse](c, ctx, http.MethodPost,
		"/api/v1/patent/applications/search", body, nil)
}

// DownloadPatents hits the search download endpoint and returns the result.
// The format parameter should be "json" or "csv".
func (c *Client) DownloadPatents(ctx context.Context, query string, format string, opts types.SearchOptions) ([]byte, error) {
	params := searchParams(query, opts)
	params["format"] = format
	return c.request(ctx, http.MethodGet, "/api/v1/patent/applications/search/download", nil, params)
}

// GetApplication returns the full patent file wrapper for a single
// application.
func (c *Client) GetApplication(ctx context.Context, appNumber string) (*types.PatentDataResponse, error) {
	return requestJSON[types.PatentDataResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/applications/"+url.PathEscape(appNumber), nil, nil)
}

// GetMetadata returns only the metadata section for an application.
func (c *Client) GetMetadata(ctx context.Context, appNumber string) (*types.PatentDataResponse, error) {
	return requestJSON[types.PatentDataResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/applications/"+url.PathEscape(appNumber)+"/meta-data", nil, nil)
}

// GetAdjustment returns patent term adjustment data.
func (c *Client) GetAdjustment(ctx context.Context, appNumber string) (*types.PatentDataResponse, error) {
	return requestJSON[types.PatentDataResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/applications/"+url.PathEscape(appNumber)+"/adjustment", nil, nil)
}

// GetExtension returns patent term extension data.
func (c *Client) GetExtension(ctx context.Context, appNumber string) (*types.PatentDataResponse, error) {
	return requestJSON[types.PatentDataResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/applications/"+url.PathEscape(appNumber)+"/extension", nil, nil)
}

// GetAssignment returns assignment data for an application.
func (c *Client) GetAssignment(ctx context.Context, appNumber string) (*types.PatentDataResponse, error) {
	return requestJSON[types.PatentDataResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/applications/"+url.PathEscape(appNumber)+"/assignment", nil, nil)
}

// GetAttorney returns attorney/agent information for an application.
func (c *Client) GetAttorney(ctx context.Context, appNumber string) (*types.PatentDataResponse, error) {
	return requestJSON[types.PatentDataResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/applications/"+url.PathEscape(appNumber)+"/attorney", nil, nil)
}

// GetContinuity returns continuity (parent/child) data for an application.
func (c *Client) GetContinuity(ctx context.Context, appNumber string) (*types.PatentDataResponse, error) {
	return requestJSON[types.PatentDataResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/applications/"+url.PathEscape(appNumber)+"/continuity", nil, nil)
}

// GetForeignPriority returns foreign priority data for an application.
func (c *Client) GetForeignPriority(ctx context.Context, appNumber string) (*types.PatentDataResponse, error) {
	return requestJSON[types.PatentDataResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/applications/"+url.PathEscape(appNumber)+"/foreign-priority", nil, nil)
}

// GetTransactions returns the prosecution event/transaction history.
func (c *Client) GetTransactions(ctx context.Context, appNumber string) (*types.PatentDataResponse, error) {
	return requestJSON[types.PatentDataResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/applications/"+url.PathEscape(appNumber)+"/transactions", nil, nil)
}

// GetDocuments returns the document list for an application, optionally
// filtered by document codes and date range.
func (c *Client) GetDocuments(ctx context.Context, appNumber string, opts types.DocumentOptions) (*types.DocumentBagResponse, error) {
	params := make(map[string]string)
	if opts.DocumentCodes != "" {
		params["documentCodes"] = opts.DocumentCodes
	}
	if opts.OfficialDateFrom != "" {
		params["officialDateFrom"] = opts.OfficialDateFrom
	}
	if opts.OfficialDateTo != "" {
		params["officialDateTo"] = opts.OfficialDateTo
	}
	return requestJSON[types.DocumentBagResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/applications/"+url.PathEscape(appNumber)+"/documents", nil, params)
}

// GetAssociatedDocuments returns associated (grant/pgpub) XML document
// metadata for an application.
func (c *Client) GetAssociatedDocuments(ctx context.Context, appNumber string) (*types.PatentDataResponse, error) {
	return requestJSON[types.PatentDataResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/applications/"+url.PathEscape(appNumber)+"/associated-documents", nil, nil)
}

// FetchGrantXML fetches the patent grant XML for an application. It first
// calls the associated-documents endpoint to get the XML file URL, then
// downloads and returns the raw XML bytes. Returns nil if no grant XML exists
// (e.g. application is not yet granted).
func (c *Client) FetchGrantXML(ctx context.Context, appNumber string) ([]byte, error) {
	resp, err := c.GetAssociatedDocuments(ctx, appNumber)
	if err != nil {
		return nil, fmt.Errorf("fetching associated documents: %w", err)
	}

	if len(resp.PatentFileWrapperDataBag) == 0 {
		return nil, fmt.Errorf("no associated documents found for %s", appNumber)
	}

	fw := resp.PatentFileWrapperDataBag[0]
	grantXML := fw.GrantDocumentMetaData
	if grantXML == nil || grantXML.FileLocationURI == "" {
		return nil, fmt.Errorf("no grant XML available for %s (application may not be granted)", appNumber)
	}

	// The file location URI points to the bulk data file endpoint which
	// returns a redirect to a signed S3 URL. Use the standard request
	// flow to follow redirects.
	c.debugf("Fetching grant XML from %s", grantXML.FileLocationURI)

	xmlURL := grantXML.FileLocationURI
	if !strings.HasPrefix(xmlURL, "http") {
		xmlURL = c.baseURL + "/" + strings.TrimLeft(xmlURL, "/")
	}

	// Use the download client with longer timeout since the file can be large.
	dlClient := &http.Client{
		Timeout: downloadTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	c.rl.waitForSlot()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, xmlURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building XML request: %w", err)
	}
	c.setHeaders(req)

	xmlResp, err := c.followRedirectsWithClient(dlClient, req)
	c.rl.markRequestComplete()
	if err != nil {
		return nil, fmt.Errorf("fetching grant XML: %w", err)
	}
	defer xmlResp.Body.Close()

	if xmlResp.StatusCode < 200 || xmlResp.StatusCode >= 300 {
		body, _ := io.ReadAll(xmlResp.Body)
		return nil, &UsptoAPIError{
			StatusCode: xmlResp.StatusCode,
			Message:    fmt.Sprintf("grant XML download failed: HTTP %d", xmlResp.StatusCode),
			Body:       string(body),
		}
	}

	return io.ReadAll(xmlResp.Body)
}

// SearchStatusCodes searches patent application status codes.
func (c *Client) SearchStatusCodes(ctx context.Context, query string, opts types.SearchOptions) (*types.StatusCodeResponse, error) {
	return requestJSON[types.StatusCodeResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/status-codes", nil, searchParams(query, opts))
}

// ==========================================================================
// Bulk Data Endpoints
// ==========================================================================

// SearchBulkData searches bulk data products.
func (c *Client) SearchBulkData(ctx context.Context, query string, opts types.SearchOptions) (*types.BulkDataResponse, error) {
	return requestJSON[types.BulkDataResponse](c, ctx, http.MethodGet,
		"/api/v1/datasets/products/search", nil, searchParams(query, opts))
}

// GetBulkDataProduct retrieves a single bulk data product by ID.
// The API wraps the response in the same format as the search endpoint
// (with bulkDataProductBag array), so we use BulkDataResponse and extract
// the first product.
func (c *Client) GetBulkDataProduct(ctx context.Context, productID string, opts types.BulkDataProductOptions) (*types.BulkDataProduct, error) {
	params := make(map[string]string)
	if opts.IncludeFiles {
		params["includeFiles"] = "true"
	}
	if opts.Latest {
		params["latest"] = "true"
	}
	resp, err := requestJSON[types.BulkDataResponse](c, ctx, http.MethodGet,
		"/api/v1/datasets/products/"+url.PathEscape(productID), nil, params)
	if err != nil {
		return nil, err
	}
	if len(resp.BulkDataProductBag) == 0 {
		return nil, fmt.Errorf("no product found with ID %q", productID)
	}
	return &resp.BulkDataProductBag[0], nil
}

// DownloadBulkFile downloads a bulk data file to outputPath.
func (c *Client) DownloadBulkFile(ctx context.Context, productID, fileName, outputPath string) (string, error) {
	dlURL := c.baseURL + "/api/v1/datasets/products/" + url.PathEscape(productID) + "/files/" + url.PathEscape(fileName)
	return c.DownloadDocument(ctx, dlURL, outputPath)
}

// ==========================================================================
// PTAB Trials -- Proceedings
// ==========================================================================

// SearchProceedings searches PTAB trial proceedings.
func (c *Client) SearchProceedings(ctx context.Context, query string, opts types.SearchOptions) (*types.ProceedingDataResponse, error) {
	return requestJSON[types.ProceedingDataResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/trials/proceedings/search", nil, searchParams(query, opts))
}

// DownloadProceedingsSearch hits the proceedings search download endpoint.
func (c *Client) DownloadProceedingsSearch(ctx context.Context, query string, opts types.SearchOptions) ([]byte, error) {
	return c.request(ctx, http.MethodGet,
		"/api/v1/patent/trials/proceedings/search/download", nil, searchParams(query, opts))
}

// GetProceeding retrieves a single proceeding by trial number.
func (c *Client) GetProceeding(ctx context.Context, trialNumber string) (*types.ProceedingDataResponse, error) {
	return requestJSON[types.ProceedingDataResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/trials/proceedings/"+url.PathEscape(trialNumber), nil, nil)
}

// ==========================================================================
// PTAB Trials -- Decisions
// ==========================================================================

// SearchDecisions searches PTAB trial decisions.
func (c *Client) SearchDecisions(ctx context.Context, query string, opts types.SearchOptions) (*types.TrialDocumentResponse, error) {
	return requestJSON[types.TrialDocumentResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/trials/decisions/search", nil, searchParams(query, opts))
}

// DownloadDecisionsSearch hits the decisions search download endpoint.
func (c *Client) DownloadDecisionsSearch(ctx context.Context, query string, opts types.SearchOptions) ([]byte, error) {
	return c.request(ctx, http.MethodGet,
		"/api/v1/patent/trials/decisions/search/download", nil, searchParams(query, opts))
}

// GetTrialDecision retrieves a single trial decision by document ID.
func (c *Client) GetTrialDecision(ctx context.Context, documentID string) (*types.TrialDocumentResponse, error) {
	return requestJSON[types.TrialDocumentResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/trials/decisions/"+url.PathEscape(documentID), nil, nil)
}

// GetTrialDecisionsByTrial retrieves all decisions for a trial number.
func (c *Client) GetTrialDecisionsByTrial(ctx context.Context, trialNumber string) (*types.TrialDocumentResponse, error) {
	return requestJSON[types.TrialDocumentResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/trials/"+url.PathEscape(trialNumber)+"/decisions", nil, nil)
}

// ==========================================================================
// PTAB Trials -- Documents
// ==========================================================================

// SearchTrialDocuments searches PTAB trial documents.
func (c *Client) SearchTrialDocuments(ctx context.Context, query string, opts types.SearchOptions) (*types.TrialDocumentResponse, error) {
	return requestJSON[types.TrialDocumentResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/trials/documents/search", nil, searchParams(query, opts))
}

// DownloadTrialDocumentsSearch hits the trial documents search download endpoint.
func (c *Client) DownloadTrialDocumentsSearch(ctx context.Context, query string, opts types.SearchOptions) ([]byte, error) {
	return c.request(ctx, http.MethodGet,
		"/api/v1/patent/trials/documents/search/download", nil, searchParams(query, opts))
}

// GetTrialDocument retrieves a single trial document by document ID.
func (c *Client) GetTrialDocument(ctx context.Context, documentID string) (*types.TrialDocumentResponse, error) {
	return requestJSON[types.TrialDocumentResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/trials/documents/"+url.PathEscape(documentID), nil, nil)
}

// GetTrialDocumentsByTrial retrieves all documents for a trial number.
func (c *Client) GetTrialDocumentsByTrial(ctx context.Context, trialNumber string) (*types.TrialDocumentResponse, error) {
	return requestJSON[types.TrialDocumentResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/trials/"+url.PathEscape(trialNumber)+"/documents", nil, nil)
}

// ==========================================================================
// PTAB Appeals
// ==========================================================================

// SearchAppeals searches PTAB appeal decisions.
func (c *Client) SearchAppeals(ctx context.Context, query string, opts types.SearchOptions) (*types.AppealDecisionResponse, error) {
	return requestJSON[types.AppealDecisionResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/appeals/decisions/search", nil, searchParams(query, opts))
}

// DownloadAppealsSearch hits the appeals search download endpoint.
func (c *Client) DownloadAppealsSearch(ctx context.Context, query string, opts types.SearchOptions) ([]byte, error) {
	return c.request(ctx, http.MethodGet,
		"/api/v1/patent/appeals/decisions/search/download", nil, searchParams(query, opts))
}

// GetAppealDecision retrieves a single appeal decision by document ID.
func (c *Client) GetAppealDecision(ctx context.Context, documentID string) (*types.AppealDecisionResponse, error) {
	return requestJSON[types.AppealDecisionResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/appeals/decisions/"+url.PathEscape(documentID), nil, nil)
}

// GetAppealDecisionsByAppeal retrieves all decisions for an appeal number.
func (c *Client) GetAppealDecisionsByAppeal(ctx context.Context, appealNumber string) (*types.AppealDecisionResponse, error) {
	return requestJSON[types.AppealDecisionResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/appeals/"+url.PathEscape(appealNumber)+"/decisions", nil, nil)
}

// ==========================================================================
// PTAB Interferences
// ==========================================================================

// SearchInterferences searches PTAB interference decisions.
func (c *Client) SearchInterferences(ctx context.Context, query string, opts types.SearchOptions) (*types.InterferenceDecisionResponse, error) {
	return requestJSON[types.InterferenceDecisionResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/interferences/decisions/search", nil, searchParams(query, opts))
}

// DownloadInterferencesSearch hits the interferences search download endpoint.
func (c *Client) DownloadInterferencesSearch(ctx context.Context, query string, opts types.SearchOptions) ([]byte, error) {
	return c.request(ctx, http.MethodGet,
		"/api/v1/patent/interferences/decisions/search/download", nil, searchParams(query, opts))
}

// GetInterferenceDecision retrieves a single interference decision by
// document ID.
func (c *Client) GetInterferenceDecision(ctx context.Context, documentID string) (*types.InterferenceDecisionResponse, error) {
	return requestJSON[types.InterferenceDecisionResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/interferences/decisions/"+url.PathEscape(documentID), nil, nil)
}

// GetInterferenceDecisionsByNumber retrieves all decisions for an
// interference number.
func (c *Client) GetInterferenceDecisionsByNumber(ctx context.Context, interferenceNumber string) (*types.InterferenceDecisionResponse, error) {
	return requestJSON[types.InterferenceDecisionResponse](c, ctx, http.MethodGet,
		"/api/v1/patent/interferences/"+url.PathEscape(interferenceNumber)+"/decisions", nil, nil)
}

// ==========================================================================
// Petition Decisions
// ==========================================================================

// SearchPetitionDecisions searches petition decisions.
func (c *Client) SearchPetitionDecisions(ctx context.Context, query string, opts types.SearchOptions) (*types.PetitionDecisionResponse, error) {
	return requestJSON[types.PetitionDecisionResponse](c, ctx, http.MethodGet,
		"/api/v1/petition/decisions/search", nil, searchParams(query, opts))
}

// GetPetitionDecision retrieves a single petition decision by record ID.
func (c *Client) GetPetitionDecision(ctx context.Context, recordID string, includeDocuments bool) (*types.PetitionDecisionResponse, error) {
	params := make(map[string]string)
	if includeDocuments {
		params["includeDocuments"] = "true"
	}
	return requestJSON[types.PetitionDecisionResponse](c, ctx, http.MethodGet,
		"/api/v1/petition/decisions/"+url.PathEscape(recordID), nil, params)
}

// ==========================================================================
// Document Download
// ==========================================================================

// DownloadDocument downloads a file from the given URL (which may be
// absolute or relative to baseURL) and writes it to outputPath. It uses
// the download timeout (600 s), follows redirects manually to re-apply
// the API key header, and retries on 429.
func (c *Client) DownloadDocument(ctx context.Context, rawURL, outputPath string) (string, error) {
	// Resolve relative URLs.
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = c.baseURL + "/" + strings.TrimLeft(rawURL, "/")
	}

	// Use a separate HTTP client with the download timeout.
	dlClient := &http.Client{
		Timeout: downloadTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	var attempt int
	for {
		c.rl.waitForSlot()

		c.debugf("DOWNLOAD %s -> %s", rawURL, outputPath)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return "", fmt.Errorf("building download request: %w", err)
		}
		c.setHeaders(req)

		// Follow redirects manually.
		resp, err := c.followRedirectsWithClient(dlClient, req)
		c.rl.markRequestComplete()
		if err != nil {
			return "", fmt.Errorf("executing download request: %w", err)
		}

		if resp.StatusCode == 429 && attempt < maxRetries {
			resp.Body.Close()
			c.rl.markRateLimited()
			attempt++
			c.debugf("429 rate limited during download, retry %d/%d", attempt, maxRetries)
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return "", &UsptoAPIError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("download failed: HTTP %d", resp.StatusCode),
				Body:       string(body),
			}
		}

		// Ensure parent directory exists.
		if dir := filepath.Dir(outputPath); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				resp.Body.Close()
				return "", fmt.Errorf("creating output directory: %w", err)
			}
		}

		out, err := os.Create(outputPath)
		if err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("creating output file: %w", err)
		}

		_, copyErr := io.Copy(out, resp.Body)
		resp.Body.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return "", fmt.Errorf("writing download to disk: %w", copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("closing output file: %w", closeErr)
		}

		c.debugf("Download complete: %s", outputPath)
		return outputPath, nil
	}
}

// followRedirectsWithClient is like followRedirects but uses a custom
// http.Client (e.g. one with a longer timeout for downloads).
func (c *Client) followRedirectsWithClient(hc *http.Client, req *http.Request) (*http.Response, error) {
	const maxRedirects = 10
	for i := 0; i < maxRedirects; i++ {
		resp, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusMovedPermanently &&
			resp.StatusCode != http.StatusFound &&
			resp.StatusCode != http.StatusTemporaryRedirect &&
			resp.StatusCode != http.StatusPermanentRedirect {
			return resp, nil
		}
		loc := resp.Header.Get("Location")
		resp.Body.Close()
		if loc == "" {
			return nil, fmt.Errorf("redirect with no Location header")
		}
		c.debugf("Following download redirect -> %s", loc)
		next, err := http.NewRequestWithContext(req.Context(), http.MethodGet, loc, nil)
		if err != nil {
			return nil, fmt.Errorf("building redirect request: %w", err)
		}
		c.setHeaders(next)
		req = next
	}
	return nil, fmt.Errorf("too many redirects (>%d)", maxRedirects)
}
