package tsdr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A trademark download that runs out of time names the limit and the flag
// that raises it, as an ODP download does. It used to fail with a bare
// "context deadline exceeded".
func TestDownloadTimeoutErrorNamesTheFlag(t *testing.T) {
	serial := Identifier{Type: IdentifierSerial, Value: "78787878"}
	const hint = "download did not finish within the 50ms download timeout; raise it with --download-timeout, 0 = no limit"

	t.Run("before the response headers, without retrying", func(t *testing.T) {
		var hits int32
		release := make(chan struct{})
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hits, 1)
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}))
		defer ts.Close()
		defer close(release)
		c := newTSDRTestClient(ts.URL, WithDownloadTimeout(50*time.Millisecond))

		_, err := c.DocumentAsset(context.Background(), serial, "DOC-123", "pdf")
		if err == nil || !strings.Contains(err.Error(), hint) {
			t.Fatalf("error = %v, want the --download-timeout hint", err)
		}
		if got := atomic.LoadInt32(&hits); got != 1 {
			t.Errorf("server saw %d requests, want 1 (a spent download budget is not retried)", got)
		}
	})

	t.Run("in the middle of the body", func(t *testing.T) {
		release := make(chan struct{})
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.7 partial"))
			w.(http.Flusher).Flush()
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}))
		defer ts.Close()
		defer close(release)
		c := newTSDRTestClient(ts.URL, WithDownloadTimeout(50*time.Millisecond))

		_, err := c.DocumentAsset(context.Background(), serial, "DOC-123", "pdf")
		if err == nil || !strings.Contains(err.Error(), hint) {
			t.Fatalf("error = %v, want the --download-timeout hint", err)
		}
	})

	t.Run("not on a metadata request", func(t *testing.T) {
		release := make(chan struct{})
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}))
		defer ts.Close()
		defer close(release)
		c := newTSDRTestClient(ts.URL, WithTimeout(50*time.Millisecond), WithDownloadTimeout(50*time.Millisecond))

		_, err := c.CaseStatusXML(context.Background(), serial)
		if err == nil {
			t.Fatal("want a timeout error")
		}
		if strings.Contains(err.Error(), "--download-timeout") {
			t.Errorf("a metadata timeout must not point at --download-timeout: %v", err)
		}
	})
}
