package tmsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout    = 30 * time.Second
	maxErrorBody      = 1 << 20
	maxErrorDetail    = 1000
	maxResponseBody   = 64 << 20
	maxRequestRetries = 3
	maxRetryDelay     = 30 * time.Second
)

// HTTPError describes a non-2xx response from Trademark Search configuration
// discovery or search. This API is keyless; authentication failures here are
// not TSDR API-key failures.
type HTTPError struct {
	Operation  string
	Method     string
	URL        string
	StatusCode int
	Status     string
	Body       string
	Header     http.Header
}

type redirectPolicyError struct{ message string }

func (e *redirectPolicyError) Error() string { return e.message }

func (e *HTTPError) Error() string {
	if e == nil {
		return "<nil>"
	}
	detail := strings.TrimSpace(e.Body)
	if detail != "" {
		return fmt.Sprintf("Trademark Search %s failed: %s %s returned %s: %s", e.Operation, e.Method, e.URL, e.Status, detail)
	}
	return fmt.Sprintf("Trademark Search %s failed: %s %s returned %s", e.Operation, e.Method, e.URL, e.Status)
}

// IsRetryable reports whether a failed request may succeed if retried.
func (e *HTTPError) IsRetryable() bool {
	return e != nil && (e.StatusCode == http.StatusTooManyRequests || e.StatusCode == http.StatusRequestTimeout || e.StatusCode >= 500)
}

// RetryAfter returns the provider's requested minimum delay, if present.
func (e *HTTPError) RetryAfter() time.Duration {
	if e == nil {
		return 0
	}
	return parseRetryAfter(e.Header.Get("Retry-After"))
}

// Client accesses the keyless official Trademark Search backend. It lazily
// discovers the versioned search root from configuration.json unless a base
// URL override is supplied.
type Client struct {
	httpClient       *http.Client
	configurationURL string
	debug            bool
	debugWriter      io.Writer
	allowCustomURLs  bool

	mu      sync.RWMutex
	baseURL string
}

// Option configures a Client.
type Option func(*Client)

// WithTimeout overrides the total HTTP request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithDebug enables concise request/response logging to stderr. Search bodies
// are not logged because trademark queries may contain client-sensitive text.
func WithDebug(enabled bool) Option {
	return func(c *Client) { c.debug = enabled }
}

// WithDebugWriter directs debug output to writer. A nil writer is ignored.
func WithDebugWriter(writer io.Writer) Option {
	return func(c *Client) {
		if writer != nil {
			c.debugWriter = writer
		}
	}
}

// WithBaseURL overrides service discovery with a Trademark Search service
// root such as https://tmsearch.uspto.gov/prod-v1-0-0/. It must not be the
// unrelated TSDR host and requires no API key.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = normalizeBaseURL(baseURL)
		c.allowCustomURLs = true
	}
}

// WithConfigurationURL overrides configuration discovery, primarily for
// tests and compatible USPTO deployments.
func WithConfigurationURL(configurationURL string) Option {
	return func(c *Client) {
		c.configurationURL = strings.TrimSpace(configurationURL)
		c.allowCustomURLs = true
	}
}

// WithHTTPClient supplies an HTTP client. Its timeout and transport are used
// as-is; a nil client is ignored.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// NewClient creates a Trademark Search client. Unlike TSDR and ODP clients,
// it intentionally accepts no API key.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient:       &http.Client{Timeout: defaultTimeout},
		configurationURL: DefaultConfigurationURL,
		debugWriter:      os.Stderr,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// DefaultClient is configured by the CLI root command and may be replaced by
// tests. Trademark Search is intentionally keyless.
var DefaultClient = NewClient()

// Discover fetches the official web application's runtime configuration,
// validates serviceUrlSearchElastic, and caches it for subsequent searches.
func (c *Client) Discover(ctx context.Context) (Configuration, error) {
	if c == nil {
		return Configuration{}, fmt.Errorf("nil Trademark Search client")
	}
	if strings.TrimSpace(c.configurationURL) == "" {
		return Configuration{}, fmt.Errorf("Trademark Search configuration URL cannot be empty")
	}
	if err := c.validateServiceURL(c.configurationURL); err != nil {
		return Configuration{}, fmt.Errorf("invalid Trademark Search configuration URL: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.configurationURL, nil)
	if err != nil {
		return Configuration{}, fmt.Errorf("creating Trademark Search configuration request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := c.do(request, "configuration discovery")
	if err != nil {
		return Configuration{}, err
	}
	defer response.Body.Close()

	var configuration Configuration
	if err := json.NewDecoder(response.Body).Decode(&configuration); err != nil {
		return Configuration{}, fmt.Errorf("decoding Trademark Search configuration: %w", err)
	}
	configuration.ServiceURLSearchElastic = normalizeBaseURL(configuration.ServiceURLSearchElastic)
	if err := c.validateServiceURL(configuration.ServiceURLSearchElastic); err != nil {
		return Configuration{}, fmt.Errorf("invalid serviceUrlSearchElastic in Trademark Search configuration: %w", err)
	}

	c.mu.Lock()
	c.baseURL = configuration.ServiceURLSearchElastic
	c.mu.Unlock()
	return configuration, nil
}

// BaseURL returns the discovered or overridden search service root. Discovery
// occurs lazily the first time this method or a search operation is called.
func (c *Client) BaseURL(ctx context.Context) (string, error) {
	if c == nil {
		return "", fmt.Errorf("nil Trademark Search client")
	}
	c.mu.RLock()
	baseURL := c.baseURL
	c.mu.RUnlock()
	if baseURL != "" {
		if err := c.validateServiceURL(baseURL); err != nil {
			return "", fmt.Errorf("invalid Trademark Search base URL: %w", err)
		}
		return baseURL, nil
	}
	configuration, err := c.Discover(ctx)
	if err != nil {
		return "", err
	}
	return configuration.ServiceURLSearchElastic, nil
}

// Search executes a typed Elasticsearch-style request.
func (c *Client) Search(ctx context.Context, request SearchRequest) (*SearchResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encoding Trademark Search request: %w", err)
	}
	raw, err := c.SearchRaw(ctx, body)
	if err != nil {
		return nil, err
	}
	var response SearchResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("decoding Trademark Search response: %w", err)
	}
	response.Raw = append(response.Raw[:0], raw...)
	return &response, nil
}

// SearchQuery builds and executes a field-tag query in one operation.
func (c *Client) SearchQuery(ctx context.Context, query string, opts SearchOptions) (*SearchResponse, error) {
	request, err := BuildSearchRequest(query, opts)
	if err != nil {
		return nil, err
	}
	return c.Search(ctx, request)
}

// SearchRaw executes an arbitrary valid JSON search body and returns the raw
// response. It is the escape hatch for new Elasticsearch clauses not yet
// represented by SearchRequest.
func (c *Client) SearchRaw(ctx context.Context, body json.RawMessage) (json.RawMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("nil Trademark Search client")
	}
	if len(bytes.TrimSpace(body)) == 0 || !json.Valid(body) {
		return nil, fmt.Errorf("Trademark Search body must be valid JSON")
	}
	baseURL, err := c.BaseURL(ctx)
	if err != nil {
		return nil, err
	}
	searchURL, err := appendURLPath(baseURL, "tmsearch")
	if err != nil {
		return nil, fmt.Errorf("building Trademark Search URL: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating Trademark Search request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	response, err := c.do(request, "search")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBody+1))
	if err != nil {
		return nil, fmt.Errorf("reading Trademark Search response: %w", err)
	}
	if len(data) > maxResponseBody {
		return nil, fmt.Errorf("Trademark Search response exceeded %d bytes", maxResponseBody)
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("Trademark Search returned invalid JSON")
	}
	return json.RawMessage(data), nil
}

func (c *Client) do(request *http.Request, operation string) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		response, err := c.doOnce(request, operation)
		if err == nil {
			return response, nil
		}
		var redirectErr *redirectPolicyError
		if errors.As(err, &redirectErr) {
			return nil, err
		}
		if attempt >= maxRequestRetries || request.Context().Err() != nil {
			return nil, err
		}
		wait := time.Duration(attempt+1) * 500 * time.Millisecond
		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			if !httpErr.IsRetryable() {
				return nil, err
			}
			if retryAfter := parseRetryAfter(httpErr.Header.Get("Retry-After")); retryAfter > 0 {
				wait = retryAfter
			}
		}
		if wait > maxRetryDelay {
			// Retry-After is a minimum. Return the provider error rather than
			// shortening it and retrying before the service permits.
			c.debugf("Trademark Search %s: not auto-retrying because Retry-After %s exceeds the %s retry-wait cap", operation, wait, maxRetryDelay)
			return nil, err
		}
		c.debugf("Trademark Search %s: retry %d/%d after %s", operation, attempt+1, maxRequestRetries, wait)
		timer := time.NewTimer(wait)
		select {
		case <-request.Context().Done():
			timer.Stop()
			return nil, request.Context().Err()
		case <-timer.C:
		}
		clone := request.Clone(request.Context())
		if request.GetBody != nil {
			body, bodyErr := request.GetBody()
			if bodyErr != nil {
				return nil, fmt.Errorf("rebuilding Trademark Search request: %w", bodyErr)
			}
			clone.Body = body
		}
		request = clone
	}
}

func (c *Client) doOnce(request *http.Request, operation string) (*http.Response, error) {
	started := time.Now()
	c.debugf("Trademark Search %s: %s %s", operation, request.Method, request.URL.Redacted())
	client := *c.httpClient
	configuredRedirect := client.CheckRedirect
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) == 0 {
			return nil
		}
		if len(via) >= 10 {
			return &redirectPolicyError{message: "stopped after 10 Trademark Search redirects"}
		}
		if c.allowCustomURLs {
			if !sameOrigin(next.URL, via[0].URL) {
				return &redirectPolicyError{message: fmt.Sprintf("refusing Trademark Search cross-origin redirect from %s to %s", via[0].URL.Redacted(), next.URL.Redacted())}
			}
		} else if err := validateOfficialUSPTOURL(next.URL.String()); err != nil {
			return &redirectPolicyError{message: fmt.Sprintf("refusing Trademark Search redirect: %v", err)}
		}
		if configuredRedirect != nil {
			return configuredRedirect(next, via)
		}
		return nil
	}
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("Trademark Search %s: %w", operation, err)
		}
		return nil, fmt.Errorf("Trademark Search %s request failed: %w", operation, err)
	}
	c.debugf("Trademark Search %s: %s in %s", operation, response.Status, time.Since(started).Round(time.Millisecond))
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return response, nil
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
	bodyText := strings.TrimSpace(string(body))
	if len(bodyText) > maxErrorDetail {
		bodyText = bodyText[:maxErrorDetail] + "..."
	}
	return nil, &HTTPError{
		Operation:  operation,
		Method:     request.Method,
		URL:        request.URL.Redacted(),
		StatusCode: response.StatusCode,
		Status:     response.Status,
		Body:       bodyText,
		Header:     response.Header.Clone(),
	}
}

func parseRetryAfter(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := time.ParseDuration(raw + "s"); err == nil && seconds > 0 {
		return seconds
	}
	if at, err := http.ParseTime(raw); err == nil {
		if wait := time.Until(at); wait > 0 {
			return wait
		}
	}
	return 0
}

func (c *Client) debugf(format string, args ...any) {
	if c.debug && c.debugWriter != nil {
		_, _ = fmt.Fprintf(c.debugWriter, format+"\n", args...)
	}
}

func normalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return strings.TrimRight(raw, "/") + "/"
}

func validateHTTPURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("URL must include a host")
	}
	return nil
}

func validateOfficialUSPTOURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("official Trademark Search URL must use https")
	}
	if parsed.User != nil || parsed.Hostname() == "" {
		return fmt.Errorf("official Trademark Search URL must include a safe host")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "uspto.gov" && !strings.HasSuffix(host, ".uspto.gov") {
		return fmt.Errorf("official Trademark Search URL must use a uspto.gov host")
	}
	return nil
}

func (c *Client) validateServiceURL(raw string) error {
	if c.allowCustomURLs {
		return validateHTTPURL(raw)
	}
	return validateOfficialUSPTOURL(raw)
}

func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func appendURLPath(baseURL, element string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if err := validateHTTPURL(baseURL); err != nil {
		return "", err
	}
	if strings.TrimRight(parsed.Path, "/") == "/"+element || strings.HasSuffix(strings.TrimRight(parsed.Path, "/"), "/"+element) {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
		return parsed.String(), nil
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + element
	return parsed.String(), nil
}
