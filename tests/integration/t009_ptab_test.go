package integration

import "testing"

func TestT009a_SearchProceedings(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("ptab", "search", "--type", "IPR", "--petitioner", "Apple", "--limit", "3", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	results := parseResultsArray(t, env.Results)
	if len(results) == 0 {
		t.Error("expected at least 1 proceeding")
	}
}

func TestT009b_GetProceeding(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("ptab", "get", "IPR2026-00243", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
}

func TestT009c_SearchDecisions_BUG004(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("ptab", "decisions", "--trial", testPTABTrial, "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	// BUG-004: results must NOT be null.
	if string(env.Results) == "null" {
		t.Fatal("BUG-004 regression: decisions results are null")
	}
}

func TestT009d_DecisionsFor_BUG004(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("ptab", "decisions-for", testPTABTrial, "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	// BUG-004: results must NOT be null.
	if string(env.Results) == "null" {
		t.Fatal("BUG-004 regression: decisions-for results are null")
	}
	results := parseResultsArray(t, env.Results)
	if len(results) < 1 {
		t.Error("expected at least 1 decision")
	}
}

func TestT009e_TrialDocuments(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("ptab", "docs-for", testPTABTrial, "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	results := parseResultsArray(t, env.Results)
	if len(results) < 20 {
		t.Errorf("expected >= 20 documents, got %d", len(results))
	}
}

func TestT009f_SearchAppeals(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("ptab", "appeals", "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
}

func TestT009g_GetAppeal(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("ptab", "appeal", testAppealID, "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
}

func TestT009h_SearchInterferences(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("ptab", "interferences", "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
}
