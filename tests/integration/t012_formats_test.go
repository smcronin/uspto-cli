package integration

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestT012a_NDJSON(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("status", "patented", "-f", "ndjson", "-q")
	assertExitCode(t, r, 0)
	lines := nonEmptyLines(r.Stdout)
	if len(lines) < 2 {
		t.Errorf("expected at least 2 NDJSON lines, got %d", len(lines))
	}
	// Each line should be valid JSON.
	for i, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Errorf("line %d is not valid JSON: %s", i+1, line)
		}
	}
}

func TestT012b_CSV(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("status", "patented", "-f", "csv", "-q")
	assertExitCode(t, r, 0)
	lines := nonEmptyLines(r.Stdout)
	// Header + at least 2 data rows.
	if len(lines) < 3 {
		t.Errorf("expected header + 2+ rows, got %d lines", len(lines))
	}
}

func TestT012c_JSONMinified(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("status", "150", "-f", "json", "--minify", "-q")
	assertExitCode(t, r, 0)
	trimmed := strings.TrimSpace(r.Stdout)
	// Minified JSON should be a single line.
	if strings.Contains(trimmed, "\n") {
		t.Error("minified JSON should be a single line")
	}
	if !json.Valid([]byte(trimmed)) {
		t.Error("output is not valid JSON")
	}
}

func TestT012d_QuietMode(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "docs", testApp, "-q")
	assertExitCode(t, r, 0)
	// Quiet mode: stderr should not contain progress messages.
	assertNotContains(t, r.Stderr, "Found")
	assertNotContains(t, r.Stderr, "Fetching")
}
