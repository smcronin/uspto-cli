package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestWarnAutoPaginationTruncated(t *testing.T) {
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	warnAutoPaginationTruncated(10001, 10000)
	_ = w.Close()
	os.Stderr = oldStderr

	out, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatalf("io.ReadAll: %v", err)
	}
	if !strings.Contains(string(out), "10000 of 10001") || !strings.Contains(string(out), "--download") {
		t.Fatalf("unexpected warning: %s", out)
	}
}

func TestDisplayDryRunPath_UsesResolutionPlaceholder(t *testing.T) {
	got := displayDryRunPath("/api/v1/patent/applications/" + dryRunResolvedApplicationPlaceholder + "/meta-data")
	if !strings.Contains(got, "<resolved-application-number>") {
		t.Fatalf("displayDryRunPath() = %q", got)
	}
}
