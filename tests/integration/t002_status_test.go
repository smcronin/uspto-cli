package integration

import (
	"strings"
	"testing"
)

func TestT002a_StatusByCode(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("status", "150")
	assertExitCode(t, r, 0)
	assertContains(t, r.Stdout, "Patented Case")
}

func TestT002b_StatusByText(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("status", "abandoned")
	assertExitCode(t, r, 0)
	// Should find multiple status codes containing "abandoned".
	lines := nonEmptyLines(r.Stdout)
	if len(lines) < 3 {
		t.Errorf("expected multiple results for 'abandoned', got %d lines", len(lines))
	}
}

func TestT002c_StatusJSON(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("status", "150", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Error("expected ok=true")
	}
	results := parseResultsArray(t, env.Results)
	if len(results) == 0 {
		t.Error("expected at least 1 result")
	}
}

func TestT002d_StatusCSV(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("status", "patented", "-f", "csv", "-q")
	assertExitCode(t, r, 0)
	lines := nonEmptyLines(r.Stdout)
	// Header + at least 2 data rows.
	if len(lines) < 3 {
		t.Fatalf("expected header + 2+ rows, got %d lines", len(lines))
	}
	// First line should be CSV header with commas.
	if !strings.Contains(lines[0], ",") {
		t.Errorf("first line doesn't look like CSV header: %s", lines[0])
	}
}
