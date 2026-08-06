// Package tsdr implements the USPTO Trademark Status and Document Retrieval
// API. TSDR is separate from the Open Data Portal: it uses tsdrapi.uspto.gov,
// the USPTO-API-KEY header, and a separately issued credential.
package tsdr

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultBaseURL = "https://tsdrapi.uspto.gov"
	APIKeyHeader   = "USPTO-API-KEY"

	defaultTimeout    = 45 * time.Second
	downloadTimeout   = 10 * time.Minute
	metadataGap       = time.Second
	downloadGap       = 15 * time.Second
	maxRetries        = 3
	maxErrorBody      = 1 << 20
	maxMetadataBody   = 64 << 20
	maxBufferedBody   = 256 << 20
	maxRawRequestBody = 16 << 20
	maxRetryDelay     = 30 * time.Second
)

// APIError represents a non-2xx TSDR response.
type APIError struct {
	StatusCode int
	Message    string
	Body       string
	RetryAfter time.Duration
}

type redirectPolicyError struct{ message string }

func (e *redirectPolicyError) Error() string { return e.message }

func (e *APIError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("TSDR API error (%d): %s -- %s", e.StatusCode, e.Message, e.Body)
	}
	return fmt.Sprintf("TSDR API error (%d): %s", e.StatusCode, e.Message)
}

// Response is an unmodified successful TSDR payload and its metadata.
type Response struct {
	Body        []byte
	ContentType string
	URL         string
	StatusCode  int
}

// IsNoContent reports a successful response with no representation. TSDR
// commonly uses 204 for valid case-specific resources that do not apply.
func (r *Response) IsNoContent() bool {
	return r == nil || r.StatusCode == http.StatusNoContent || len(bytes.TrimSpace(r.Body)) == 0
}

// StreamResponse exposes a successful TSDR response body without buffering it
// in memory. Close must be called to release the connection and request timer.
type StreamResponse struct {
	Body          io.ReadCloser
	ContentType   string
	URL           string
	StatusCode    int
	ContentLength int64
	cancel        context.CancelFunc
	closeOnce     sync.Once
	closeErr      error
}

func (r *StreamResponse) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		if r.Body != nil {
			r.closeErr = r.Body.Close()
		}
		if r.cancel != nil {
			r.cancel()
		}
	})
	return r.closeErr
}

type rateLimiter struct {
	mu           sync.Mutex
	lastMetadata time.Time
	lastDownload time.Time
	metadataPath string
	downloadPath string
	metadataWait time.Duration
	downloadWait time.Duration
	enabled      bool
}

func newRateLimiter(apiKeys ...string) *rateLimiter {
	suffix := "default"
	if len(apiKeys) > 0 && strings.TrimSpace(apiKeys[0]) != "" {
		hash := sha256.Sum256([]byte(strings.TrimSpace(apiKeys[0])))
		suffix = fmt.Sprintf("%x", hash[:6])
	}
	r := &rateLimiter{
		enabled:      true,
		metadataPath: filepath.Join(os.TempDir(), "uspto-tsdr-"+suffix+"-metadata-ratelimit"),
		downloadPath: filepath.Join(os.TempDir(), "uspto-tsdr-"+suffix+"-download-ratelimit"),
		metadataWait: metadataGap,
		downloadWait: downloadGap,
	}
	r.lastMetadata = readTimestamp(r.metadataPath)
	r.lastDownload = readTimestamp(r.downloadPath)
	return r
}

func readTimestamp(path string) time.Time {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(0, n)
}

func writeTimestamp(path string, value time.Time) {
	_ = os.WriteFile(path, []byte(strconv.FormatInt(value.UnixNano(), 10)), 0o600)
}

func (r *rateLimiter) wait(ctx context.Context, download bool) error {
	if !r.enabled {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	metadataPath := r.metadataPath
	if metadataPath == "" {
		metadataPath = filepath.Join(os.TempDir(), "uspto-tsdr-default-metadata-ratelimit")
	}
	metadataInterval := r.metadataWait
	if metadataInterval <= 0 {
		metadataInterval = metadataGap
	}
	if download {
		downloadPath := r.downloadPath
		if downloadPath == "" {
			downloadPath = filepath.Join(os.TempDir(), "uspto-tsdr-default-download-ratelimit")
		}
		downloadInterval := r.downloadWait
		if downloadInterval <= 0 {
			downloadInterval = downloadGap
		}
		return r.waitNestedLanes(ctx, downloadPath, downloadInterval, metadataPath, metadataInterval)
	}
	return r.waitLane(ctx, &r.lastMetadata, metadataPath, metadataInterval)
}

// waitNestedLanes holds the artifact reservation lock until it can timestamp
// both the artifact and all-request lanes at the final dispatch boundary. This
// prevents another process from overtaking a binary request while it waits for
// the metadata lane and collapsing the 15-second artifact spacing.
func (r *rateLimiter) waitNestedLanes(ctx context.Context, downloadPath string, downloadInterval time.Duration, metadataPath string, metadataInterval time.Duration) error {
	releaseDownload, err := acquireTimestampLock(ctx, downloadPath+".lock")
	if err != nil {
		return err
	}
	defer releaseDownload()
	if err := waitForTimestampGap(ctx, &r.lastDownload, downloadPath, downloadInterval); err != nil {
		return err
	}

	releaseMetadata, err := acquireTimestampLock(ctx, metadataPath+".lock")
	if err != nil {
		return err
	}
	defer releaseMetadata()
	if err := waitForTimestampGap(ctx, &r.lastMetadata, metadataPath, metadataInterval); err != nil {
		return err
	}

	now := time.Now()
	r.lastDownload = now
	r.lastMetadata = now
	writeTimestamp(downloadPath, now)
	writeTimestamp(metadataPath, now)
	return nil
}

func (r *rateLimiter) waitLane(ctx context.Context, last *time.Time, path string, gap time.Duration) error {
	release, err := acquireTimestampLock(ctx, path+".lock")
	if err != nil {
		return err
	}
	defer release()
	if err := waitForTimestampGap(ctx, last, path, gap); err != nil {
		return err
	}
	now := time.Now()
	*last = now
	writeTimestamp(path, now)
	return nil
}

func waitForTimestampGap(ctx context.Context, last *time.Time, path string, gap time.Duration) error {
	// Reload on every request so concurrently running CLI processes observe
	// each other's latest conservative claim.
	if shared := readTimestamp(path); shared.After(*last) {
		*last = shared
	}
	if remaining := gap - time.Since(*last); remaining > 0 {
		timer := time.NewTimer(remaining)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

func acquireTimestampLock(ctx context.Context, path string) (func(), error) {
	for {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = file.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("acquiring TSDR rate-limit lock: %w", err)
		}
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > 2*time.Minute {
			_ = os.Remove(path)
			continue
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

// Client is an authenticated TSDR client.
type Client struct {
	httpClient      *http.Client
	apiKey          string
	baseURL         string
	debug           bool
	timeout         time.Duration
	downloadTimeout time.Duration
	rl              *rateLimiter
}

// ClientOption configures a Client.
type ClientOption func(*Client)

func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) { c.baseURL = strings.TrimRight(baseURL, "/") }
}

func WithDebug(debug bool) ClientOption {
	return func(c *Client) { c.debug = debug }
}

func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) { c.timeout = timeout }
}

func WithDownloadTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) { c.downloadTimeout = timeout }
}

// WithoutRateLimit is intended for tests and private mock servers.
func WithoutRateLimit() ClientOption {
	return func(c *Client) { c.rl.enabled = false }
}

func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		apiKey:          strings.TrimSpace(apiKey),
		baseURL:         DefaultBaseURL,
		timeout:         defaultTimeout,
		downloadTimeout: downloadTimeout,
		rl:              newRateLimiter(apiKey),
	}
	c.httpClient = &http.Client{
		// Never leak the separate TSDR credential if an asset redirects to a
		// different USPTO host. Same-host redirects retain it.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) == 0 {
				return nil
			}
			if len(via) >= 10 {
				return &redirectPolicyError{message: "stopped after 10 TSDR redirects"}
			}
			if !sameOrigin(req.URL, via[0].URL) {
				req.Header.Del(APIKeyHeader)
			}
			return nil
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

// DefaultClient is initialized by the root command.
var DefaultClient *Client

func (c *Client) GetBaseURL() string { return c.baseURL }

func (c *Client) debugf(format string, args ...interface{}) {
	if c.debug {
		fmt.Fprintf(os.Stderr, "[TSDR DEBUG] "+format+"\n", args...)
	}
}

func (c *Client) buildURL(path string, params url.Values) (string, error) {
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return "", fmt.Errorf("TSDR path must be a single-host absolute path beginning with /")
	}
	parsed, err := url.Parse(path)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return "", fmt.Errorf("invalid TSDR path %q", path)
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", fmt.Errorf("TSDR path must not contain a query or fragment; use query parameters")
	}
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	return u, nil
}

// RequestURL builds a safe dry-run URL without executing it.
func (c *Client) RequestURL(path string, params url.Values) (string, error) {
	return c.buildURL(path, params)
}

func (c *Client) get(ctx context.Context, path string, params url.Values, accept string, download bool) (*Response, error) {
	fullURL, err := c.buildURL(path, params)
	if err != nil {
		return nil, err
	}
	return c.getURL(ctx, fullURL, accept, download)
}

func (c *Client) getEncoded(ctx context.Context, path, encodedQuery, accept string, download bool) (*Response, error) {
	fullURL, err := c.buildURL(path, nil)
	if err != nil {
		return nil, err
	}
	if encodedQuery != "" {
		fullURL += "?" + encodedQuery
	}
	return c.getURL(ctx, fullURL, accept, download)
}

func (c *Client) openEncoded(ctx context.Context, path, encodedQuery, accept string, download bool) (*StreamResponse, error) {
	fullURL, err := c.buildURL(path, nil)
	if err != nil {
		return nil, err
	}
	if encodedQuery != "" {
		fullURL += "?" + encodedQuery
	}
	return c.openURL(ctx, fullURL, accept, download)
}

func (c *Client) getURL(ctx context.Context, fullURL, accept string, download bool) (*Response, error) {
	stream, err := c.openURL(ctx, fullURL, accept, download)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	limit := int64(maxMetadataBody)
	if download {
		limit = int64(maxBufferedBody)
	}
	body, err := readBoundedBody(stream.Body, limit)
	if err != nil {
		return nil, fmt.Errorf("reading TSDR response: %w", err)
	}
	c.debugf("Response: %d (%d bytes, %s)", stream.StatusCode, len(body), stream.ContentType)
	return &Response{Body: body, ContentType: stream.ContentType, URL: stream.URL, StatusCode: stream.StatusCode}, nil
}

func (c *Client) openURL(ctx context.Context, fullURL, accept string, download bool) (*StreamResponse, error) {
	return c.openRequestURL(ctx, http.MethodGet, fullURL, accept, "", nil, download)
}

func (c *Client) openRequestURL(ctx context.Context, method, fullURL, accept, contentType string, body []byte, download bool) (*StreamResponse, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method != http.MethodGet && method != http.MethodPost {
		return nil, fmt.Errorf("unsupported TSDR request method %q: expected GET or POST", method)
	}
	if len(body) > maxRawRequestBody {
		return nil, fmt.Errorf("TSDR request body exceeded %d bytes", maxRawRequestBody)
	}
	if method == http.MethodGet && len(body) > 0 {
		return nil, fmt.Errorf("GET requests cannot include a body")
	}
	timeout := c.timeout
	if download {
		timeout = c.downloadTimeout
	}

	for attempt := 0; ; attempt++ {
		if err := c.rl.wait(ctx, download); err != nil {
			return nil, err
		}
		reqCtx, cancel := context.WithTimeout(ctx, timeout)
		var requestBody io.Reader
		if len(body) > 0 {
			requestBody = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(reqCtx, method, fullURL, requestBody)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("building TSDR request: %w", err)
		}
		req.Header.Set(APIKeyHeader, c.apiKey)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		c.debugf("%s %s", method, redactTSDRURL(fullURL))
		resp, err := c.httpClient.Do(req)
		if err != nil {
			cancel()
			var redirectErr *redirectPolicyError
			if errors.As(err, &redirectErr) {
				return nil, fmt.Errorf("executing TSDR request: %w", redirectErr)
			}
			if attempt < maxRetries {
				wait := time.Duration(attempt+1) * 250 * time.Millisecond
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(wait):
					continue
				}
			}
			return nil, fmt.Errorf("executing TSDR request: %w", err)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			c.debugf("Response: %d (%s, content-length %d)", resp.StatusCode, resp.Header.Get("Content-Type"), resp.ContentLength)
			return &StreamResponse{
				Body: resp.Body, ContentType: resp.Header.Get("Content-Type"), URL: resp.Request.URL.String(),
				StatusCode: resp.StatusCode, ContentLength: resp.ContentLength, cancel: cancel,
			}, nil
		}
		errorBody, readErr := readTruncatedBody(resp.Body, maxErrorBody)
		_ = resp.Body.Close()
		cancel()
		if readErr != nil {
			return nil, fmt.Errorf("reading TSDR error response: %w", readErr)
		}
		retryDelay := retryAfter(resp.Header.Get("Retry-After"))
		if resp.StatusCode == http.StatusTooManyRequests && retryDelay <= 0 {
			retryDelay = time.Minute
		}
		if retryableStatus(resp.StatusCode) && attempt < maxRetries {
			wait := retryDelay
			if wait <= 0 {
				wait = time.Duration(attempt+1) * time.Second
			}
			if wait > maxRetryDelay {
				// Retry-After is a server minimum. Return the provider error to
				// preserve a bounded CLI call instead of retrying too early.
				c.debugf("HTTP %d; not auto-retrying because Retry-After %s exceeds the %s retry-wait cap", resp.StatusCode, wait, maxRetryDelay)
			} else {
				c.debugf("HTTP %d; retry %d/%d after %s", resp.StatusCode, attempt+1, maxRetries, wait)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(wait):
					continue
				}
			}
		}
		message := http.StatusText(resp.StatusCode)
		bodyText := strings.TrimSpace(string(errorBody))
		if len(bodyText) > 1000 {
			bodyText = bodyText[:1000] + "..."
		}
		return nil, &APIError{StatusCode: resp.StatusCode, Message: message, Body: bodyText, RetryAfter: retryDelay}
	}
}

func redactTSDRURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "<invalid-url>"
	}
	segments := strings.Split(parsed.Path, "/")
	for i, segment := range segments {
		lower := strings.ToLower(segment)
		if hasIdentifierPrefix(lower) {
			segments[i] = lower[:identifierPrefixLength(lower)] + "{id}"
			continue
		}
		if i > 0 {
			switch strings.ToLower(segments[i-1]) {
			case "rawimage", "casemap":
				segments[i] = "{id}"
			}
		}
	}
	parsed.Path = strings.Join(segments, "/")
	parsed.RawPath = ""
	keys := make([]string, 0, len(parsed.Query()))
	for key := range parsed.Query() {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	query := url.Values{}
	for _, key := range keys {
		query.Set(key, "{redacted}")
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func hasIdentifierPrefix(value string) bool {
	return identifierPrefixLength(value) > 0
}

func identifierPrefixLength(value string) int {
	for _, prefix := range []string{"ref", "sn", "rn", "ir", "pn"} {
		if strings.HasPrefix(value, prefix) && len(value) > len(prefix) {
			return len(prefix)
		}
	}
	return 0
}

func readTruncatedBody(reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func readBoundedBody(reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("response exceeded %d bytes", limit)
	}
	return body, nil
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusInternalServerError || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func retryAfter(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(raw); err == nil {
		return time.Until(at)
	}
	return 0
}

// DocumentQuery selects one or more TSDR cases and optional document filters.
type DocumentQuery struct {
	Identifiers []Identifier
	Date        string
	FromDate    string
	ToDate      string
	Types       []string
	Category    string
	Sort        string
}

func (q DocumentQuery) Values() (url.Values, error) {
	if len(q.Identifiers) == 0 {
		return nil, fmt.Errorf("at least one trademark identifier is required")
	}
	groups := map[string][]string{}
	seenIdentifiers := make(map[string]struct{}, len(q.Identifiers))
	for _, id := range q.Identifiers {
		if id.Prefix() == "" || id.Value == "" {
			return nil, fmt.Errorf("invalid empty trademark identifier")
		}
		key := id.PathToken()
		if _, exists := seenIdentifiers[key]; exists {
			continue
		}
		seenIdentifiers[key] = struct{}{}
		groups[id.Prefix()] = append(groups[id.Prefix()], id.Value)
	}
	values := url.Values{}
	for _, item := range []struct{ label, value string }{{"date", q.Date}, {"fromDate", q.FromDate}, {"toDate", q.ToDate}} {
		label, value := item.label, item.value
		if value == "" {
			continue
		}
		if parsed, err := time.Parse("2006-01-02", value); err != nil || parsed.Format("2006-01-02") != value {
			return nil, fmt.Errorf("%s must be a valid YYYY-MM-DD date", label)
		}
	}
	if q.FromDate != "" && q.ToDate != "" && q.FromDate > q.ToDate {
		return nil, fmt.Errorf("fromDate must not be after toDate")
	}
	for key, items := range groups {
		values.Set(key, strings.Join(items, ","))
	}
	if q.Date != "" {
		values.Set("date", q.Date)
	}
	if q.FromDate != "" {
		values.Set("fromDate", q.FromDate)
	}
	if q.ToDate != "" {
		values.Set("toDate", q.ToDate)
	}
	if len(q.Types) > 0 {
		values.Set("type", strings.Join(q.Types, ","))
	}
	if q.Category != "" {
		values.Set("category", q.Category)
	}
	if q.Sort != "" {
		values.Set("sort", q.Sort)
	}
	return values, nil
}

// EncodedQuery preserves the query order required by TSDR's bundle routes.
// Although URL query parameters are normally order-insensitive, the live TSDR
// service rejects date/category parameters when they sort before an identifier.
func (q DocumentQuery) EncodedQuery() (string, error) {
	values, err := q.Values()
	if err != nil {
		return "", err
	}
	pairs := make([]string, 0, len(values))
	for _, key := range []string{"sn", "rn", "ir", "ref", "pn", "date", "fromDate", "toDate", "type", "category", "sort"} {
		for _, value := range values[key] {
			encodedValue := url.QueryEscape(value)
			// TSDR treats comma-delimited identifier values as syntax, not as an
			// opaque query value. Its live bundle routes reject an otherwise
			// equivalent %2C-encoded separator between same-namespace IDs.
			if key == "sn" || key == "rn" || key == "ir" || key == "ref" || key == "pn" {
				parts := strings.Split(value, ",")
				for i := range parts {
					parts[i] = url.QueryEscape(parts[i])
				}
				encodedValue = strings.Join(parts, ",")
			}
			pairs = append(pairs, url.QueryEscape(key)+"="+encodedValue)
		}
	}
	return strings.Join(pairs, "&"), nil
}

func (c *Client) CaseStatusXML(ctx context.Context, id Identifier) (*Response, error) {
	return c.get(ctx, "/ts/cd/casestatus/"+url.PathEscape(id.PathToken())+"/info.xml", nil, "application/xml", false)
}

func (c *Client) CaseStatusJSON(ctx context.Context, id Identifier) (*Response, error) {
	return c.get(ctx, "/ts/cd/casestatus/"+url.PathEscape(id.PathToken())+"/info.json", nil, "application/json", false)
}

func (c *Client) CaseStatusLegacyXML(ctx context.Context, id Identifier) (*Response, error) {
	return c.get(ctx, "/ts/cd/casestatus/"+url.PathEscape(id.PathToken())+"/v1/info", nil, "application/xml", false)
}

// CaseAsset retrieves documented alternate status representations.
// Supported formats: html, pdf, status-zip, and image-zip.
func (c *Client) CaseAsset(ctx context.Context, id Identifier, format string) (*Response, error) {
	var suffix, accept string
	switch strings.ToLower(format) {
	case "html":
		suffix, accept = "/content.html", "text/html"
	case "pdf":
		suffix, accept = "/download.pdf", "application/pdf"
	case "status-zip", "content-zip":
		suffix, accept = "/content.zip", "application/zip"
	case "image-zip", "download-zip":
		suffix, accept = "/download.zip", "application/zip"
	default:
		return nil, fmt.Errorf("unsupported case asset format %q", format)
	}
	return c.get(ctx, "/ts/cd/casestatus/"+url.PathEscape(id.PathToken())+suffix, nil, accept, format != "html")
}

// MultiCaseStatus retrieves up to 25 cases of one identifier namespace.
func (c *Client) MultiCaseStatus(ctx context.Context, idType IdentifierType, ids []string, display, allowDupes bool) (*Response, error) {
	return c.MultiCaseStatusRange(ctx, idType, ids, "", "", display, allowDupes)
}

// MultiCaseStatusRange exposes Swagger's optional opaque from/to batch
// selectors. Their semantics are provider-defined, so values are preserved.
func (c *Client) MultiCaseStatusRange(ctx context.Context, idType IdentifierType, ids []string, from, to string, display, allowDupes bool) (*Response, error) {
	if idType != IdentifierSerial && idType != IdentifierRegistration && idType != IdentifierInternational && idType != IdentifierReference {
		return nil, fmt.Errorf("batch status supports serial, registration, international, or reference identifiers")
	}
	if len(ids) == 0 || len(ids) > 25 {
		return nil, fmt.Errorf("batch status requires 1-25 identifiers, got %d", len(ids))
	}
	values := url.Values{}
	values.Set("ids", strings.Join(ids, ","))
	if strings.TrimSpace(from) != "" {
		values.Set("from", strings.TrimSpace(from))
	}
	if strings.TrimSpace(to) != "" {
		values.Set("to", strings.TrimSpace(to))
	}
	if display {
		values.Set("display", "true")
	}
	if allowDupes {
		values.Set("allowDupes", "true")
	}
	prefix := (Identifier{Type: idType}).Prefix()
	return c.get(ctx, "/ts/cd/caseMultiStatus/"+prefix, values, "application/json", false)
}

func (c *Client) DocumentsXML(ctx context.Context, query DocumentQuery) (*Response, error) {
	encoded, err := query.EncodedQuery()
	if err != nil {
		return nil, err
	}
	return c.getEncoded(ctx, "/ts/cd/casedocs/bundle.xml", encoded, "application/xml", false)
}

// DocumentsXMLStream opens multi-case document metadata without buffering it.
func (c *Client) DocumentsXMLStream(ctx context.Context, query DocumentQuery) (*StreamResponse, error) {
	encoded, err := query.EncodedQuery()
	if err != nil {
		return nil, err
	}
	return c.openEncoded(ctx, "/ts/cd/casedocs/bundle.xml", encoded, "application/xml", false)
}

// CaseDocumentsXML retrieves the complete file-wrapper document list for one
// case. The explicit XML suffix avoids unreliable content negotiation.
func (c *Client) CaseDocumentsXML(ctx context.Context, id Identifier) (*Response, error) {
	return c.get(ctx, "/ts/cd/casedocs/"+url.PathEscape(id.PathToken())+"/info.xml", nil, "application/xml", false)
}

func (c *Client) DocumentInfoXML(ctx context.Context, id Identifier, documentID string) (*Response, error) {
	if strings.TrimSpace(documentID) == "" || strings.ContainsAny(documentID, "/?#&") {
		return nil, fmt.Errorf("invalid empty or unsafe document ID")
	}
	path := "/ts/cd/casedoc/" + url.PathEscape(id.PathToken()) + "/" + url.PathEscape(documentID) + "/info.xml"
	return c.get(ctx, path, nil, "application/xml", false)
}

// DocumentAsset returns one rendered PDF or a ZIP containing native pages.
func (c *Client) DocumentAsset(ctx context.Context, id Identifier, documentID, format string) (*Response, error) {
	if strings.TrimSpace(documentID) == "" || strings.ContainsAny(documentID, "/?#&") {
		return nil, fmt.Errorf("invalid empty or unsafe document ID")
	}
	var suffix, accept string
	switch strings.ToLower(format) {
	case "pdf":
		suffix, accept = "/download.pdf", "application/pdf"
	case "zip":
		suffix, accept = "/content.zip", "application/zip"
	default:
		return nil, fmt.Errorf("unsupported document format %q: expected pdf or zip", format)
	}
	path := "/ts/cd/casedoc/" + url.PathEscape(id.PathToken()) + "/" + url.PathEscape(documentID) + suffix
	return c.get(ctx, path, nil, accept, true)
}

func (c *Client) DocumentPage(ctx context.Context, id Identifier, documentID string, page int) (*Response, error) {
	if page < 1 {
		return nil, fmt.Errorf("page must be >= 1")
	}
	if strings.TrimSpace(documentID) == "" || strings.ContainsAny(documentID, "/?#&") {
		return nil, fmt.Errorf("invalid empty or unsafe document ID")
	}
	path := fmt.Sprintf("/ts/cd/casedoc/%s/%s/%d/media", url.PathEscape(id.PathToken()), url.PathEscape(documentID), page)
	return c.get(ctx, path, nil, "*/*", true)
}

func (c *Client) DocumentsAsset(ctx context.Context, query DocumentQuery, format string) (*Response, error) {
	encoded, err := query.EncodedQuery()
	if err != nil {
		return nil, err
	}
	format = strings.ToLower(format)
	if format != "pdf" && format != "zip" {
		return nil, fmt.Errorf("unsupported document bundle format %q: expected pdf or zip", format)
	}
	accept := "application/" + format
	return c.getEncoded(ctx, "/ts/cd/casedocs/bundle."+format, encoded, accept, true)
}

// DocumentsAssetStream opens a merged PDF/ZIP without buffering it in memory.
func (c *Client) DocumentsAssetStream(ctx context.Context, query DocumentQuery, format string) (*StreamResponse, error) {
	encoded, err := query.EncodedQuery()
	if err != nil {
		return nil, err
	}
	format = strings.ToLower(format)
	if format != "pdf" && format != "zip" {
		return nil, fmt.Errorf("unsupported document bundle format %q: expected pdf or zip", format)
	}
	return c.openEncoded(ctx, "/ts/cd/casedocs/bundle."+format, encoded, "application/"+format, true)
}

func (c *Client) RawImage(ctx context.Context, serial string) (*Response, error) {
	id, err := ParseIdentifier(serial, "serial")
	if err != nil {
		return nil, err
	}
	return c.get(ctx, "/ts/cd/rawImage/"+url.PathEscape(id.Value), nil, "image/*", false)
}

func (c *Client) Maintenance(ctx context.Context, id Identifier) (*Response, error) {
	return c.get(ctx, "/ts/cd/maintenance/"+url.PathEscape(id.PathToken())+"/info.json", nil, "application/json", false)
}

func (c *Client) CaseMap(ctx context.Context, token string) (*Response, error) {
	token = strings.TrimSpace(token)
	if token == "" || strings.ContainsAny(token, "/?#&") {
		return nil, fmt.Errorf("invalid empty or unsafe case-map token")
	}
	return c.get(ctx, "/ts/cd/casemap/"+url.PathEscape(token)+"/info", nil, "application/xml", false)
}

func (c *Client) MultimediaInfo(ctx context.Context, id Identifier, sequence string) (*Response, error) {
	path := "/ts/cd/multimedia/" + url.PathEscape(id.PathToken())
	if strings.TrimSpace(sequence) != "" {
		if strings.ContainsAny(sequence, "/?#&") {
			return nil, fmt.Errorf("invalid multimedia sequence")
		}
		path += "/" + url.PathEscape(sequence)
	}
	return c.get(ctx, path+"/info.xml", nil, "application/xml", false)
}

func (c *Client) MultimediaContent(ctx context.Context, id Identifier, sequence string, download bool) (*Response, error) {
	sequence = strings.TrimSpace(sequence)
	if sequence == "" || strings.ContainsAny(sequence, "/?#&") {
		return nil, fmt.Errorf("a safe multimedia sequence is required")
	}
	suffix := "/content"
	if download {
		suffix = "/download"
	}
	path := "/ts/cd/multimedia/" + url.PathEscape(id.PathToken()) + "/" + url.PathEscape(sequence) + suffix
	return c.get(ctx, path, nil, "*/*", download)
}

func (c *Client) LastUpdate(ctx context.Context, ids []Identifier, format string) (*Response, error) {
	values, err := (DocumentQuery{Identifiers: ids}).Values()
	if err != nil {
		return nil, err
	}
	format = strings.ToLower(format)
	if format != "json" && format != "xml" {
		return nil, fmt.Errorf("unsupported last-update format %q: expected json or xml", format)
	}
	return c.get(ctx, "/ts/cd/caseupdate/info."+format, values, "application/"+format, false)
}

// RawGet is a future-proof authenticated GET escape hatch for documented TSDR
// paths not yet wrapped by a first-class command. Absolute URLs are rejected.
func (c *Client) RawGet(ctx context.Context, path string, params url.Values, accept string, download bool) (*Response, error) {
	return c.RawRequest(ctx, http.MethodGet, path, params, accept, "", nil, download)
}

// RawRequest reaches documented retrieval aliases with a replayable GET or
// POST body while enforcing the same single-origin and rate-limit policy.
func (c *Client) RawRequest(ctx context.Context, method, path string, params url.Values, accept, contentType string, body []byte, download bool) (*Response, error) {
	stream, err := c.RawRequestStream(ctx, method, path, params, accept, contentType, body, download)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	limit := int64(maxMetadataBody)
	if download {
		limit = int64(maxBufferedBody)
	}
	responseBody, err := readBoundedBody(stream.Body, limit)
	if err != nil {
		return nil, fmt.Errorf("reading TSDR response: %w", err)
	}
	return &Response{Body: responseBody, ContentType: stream.ContentType, URL: stream.URL, StatusCode: stream.StatusCode}, nil
}

// RawEncodedRequest preserves a caller-supplied, already percent-encoded query
// order for TSDR routes whose gateway incorrectly treats order as significant.
func (c *Client) RawEncodedRequest(ctx context.Context, method, path, encodedQuery, accept, contentType string, body []byte, download bool) (*Response, error) {
	stream, err := c.RawEncodedRequestStream(ctx, method, path, encodedQuery, accept, contentType, body, download)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	limit := int64(maxMetadataBody)
	if download {
		limit = int64(maxBufferedBody)
	}
	responseBody, err := readBoundedBody(stream.Body, limit)
	if err != nil {
		return nil, fmt.Errorf("reading TSDR response: %w", err)
	}
	return &Response{Body: responseBody, ContentType: stream.ContentType, URL: stream.URL, StatusCode: stream.StatusCode}, nil
}

// RawStream is the streaming counterpart to RawGet for large or opaque
// read-only responses. The same single-origin path checks and auth apply.
func (c *Client) RawStream(ctx context.Context, path string, params url.Values, accept string, download bool) (*StreamResponse, error) {
	return c.RawRequestStream(ctx, http.MethodGet, path, params, accept, "", nil, download)
}

// RawRequestStream is the streaming GET/POST escape hatch. POST bodies are
// copied by the caller and replayed exactly for bounded transient retries.
func (c *Client) RawRequestStream(ctx context.Context, method, path string, params url.Values, accept, contentType string, body []byte, download bool) (*StreamResponse, error) {
	fullURL, err := c.buildURL(path, params)
	if err != nil {
		return nil, err
	}
	return c.openRequestURL(ctx, method, fullURL, accept, contentType, body, download)
}

// RawEncodedRequestStream is the ordered-query streaming escape hatch. The
// encoded query must not include a leading ?, fragment, or control characters.
func (c *Client) RawEncodedRequestStream(ctx context.Context, method, path, encodedQuery, accept, contentType string, body []byte, download bool) (*StreamResponse, error) {
	if strings.HasPrefix(encodedQuery, "?") || strings.ContainsAny(encodedQuery, "#\r\n") {
		return nil, fmt.Errorf("invalid encoded TSDR query")
	}
	fullURL, err := c.buildURL(path, nil)
	if err != nil {
		return nil, err
	}
	if encodedQuery != "" {
		fullURL += "?" + encodedQuery
	}
	return c.openRequestURL(ctx, method, fullURL, accept, contentType, body, download)
}

// PublicAsset retrieves a URL returned in TSDR document metadata without
// forwarding the TSDR credential. Only HTTPS USPTO hosts are accepted.
func (c *Client) PublicAsset(ctx context.Context, rawURL string) (*Response, error) {
	stream, err := c.PublicAssetStream(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	body, err := readBoundedBody(stream.Body, maxBufferedBody)
	if err != nil {
		return nil, fmt.Errorf("reading public USPTO asset: %w", err)
	}
	return &Response{Body: body, ContentType: stream.ContentType, URL: stream.URL, StatusCode: stream.StatusCode}, nil
}

// PublicAssetStream opens a metadata-provided USPTO URL without forwarding
// the TSDR key. Every redirect is revalidated as HTTPS on a USPTO host.
func (c *Client) PublicAssetStream(ctx context.Context, rawURL string) (*StreamResponse, error) {
	parsed, err := parsePublicAssetURL(rawURL)
	if err != nil {
		return nil, err
	}
	if err := c.rl.wait(ctx, true); err != nil {
		return nil, err
	}
	reqCtx, cancel := context.WithTimeout(ctx, c.downloadTimeout)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("building public USPTO asset request: %w", err)
	}
	req.Header.Set("Accept", "*/*")
	publicClient := *c.httpClient
	publicClient.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 public USPTO asset redirects")
		}
		next.Header.Del(APIKeyHeader)
		if _, err := parsePublicAssetURL(next.URL.String()); err != nil {
			return fmt.Errorf("refusing public asset redirect: %w", err)
		}
		return nil
	}
	resp, err := publicClient.Do(req)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("retrieving public USPTO asset: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := readBoundedBody(resp.Body, maxErrorBody)
		_ = resp.Body.Close()
		cancel()
		if readErr != nil {
			return nil, fmt.Errorf("reading public USPTO asset error: %w", readErr)
		}
		text := strings.TrimSpace(string(body))
		if len(text) > 1000 {
			text = text[:1000] + "..."
		}
		retryDelay := retryAfter(resp.Header.Get("Retry-After"))
		if resp.StatusCode == http.StatusTooManyRequests && retryDelay <= 0 {
			retryDelay = time.Minute
		}
		return nil, &APIError{StatusCode: resp.StatusCode, Message: http.StatusText(resp.StatusCode), Body: text, RetryAfter: retryDelay}
	}
	return &StreamResponse{
		Body: resp.Body, ContentType: resp.Header.Get("Content-Type"), URL: resp.Request.URL.String(),
		StatusCode: resp.StatusCode, ContentLength: resp.ContentLength, cancel: cancel,
	}, nil
}

func parsePublicAssetURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("invalid public USPTO asset URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "uspto.gov" && !strings.HasSuffix(host, ".uspto.gov") {
		return nil, fmt.Errorf("refusing non-USPTO asset host %q", parsed.Hostname())
	}
	return parsed, nil
}
