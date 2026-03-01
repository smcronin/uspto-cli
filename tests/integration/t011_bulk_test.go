package integration

import "testing"

func TestT011a_BulkSearch(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("bulk", "search", "--limit", "3", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	results := parseResultsArray(t, env.Results)
	if len(results) == 0 {
		t.Error("expected at least 1 bulk product")
	}
}

func TestT011b_BulkGet_BUG005(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("bulk", "get", testBulkProduct, "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	// BUG-005: fields should NOT be empty.
	obj := parseResultsObject(t, env.Results)
	pid, _ := obj["productIdentifier"].(string)
	if pid == "" {
		t.Fatal("BUG-005 regression: productIdentifier is empty")
	}
	if pid != testBulkProduct {
		t.Errorf("expected productIdentifier=%s, got %s", testBulkProduct, pid)
	}
}

func TestT011c_BulkFiles_BUG005(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("bulk", "files", testBulkProduct, "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	// BUG-005: should return a non-zero file list.
	results := parseResultsArray(t, env.Results)
	if len(results) < 100 {
		t.Errorf("BUG-005 regression: expected >= 100 files, got %d", len(results))
	}
}
