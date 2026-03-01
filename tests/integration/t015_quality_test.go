package integration

import "testing"

func TestT015a_DryRunAppGetDoesNotExecute(t *testing.T) {
	r := runCLI("app", "get", testApp, "--dry-run")
	assertExitCode(t, r, 0)
	assertContains(t, r.Stderr, "GET /api/v1/patent/applications/"+testApp)
	assertNotContains(t, r.Stdout, "Application #")
}

func TestT015b_DryRunPtabDecisionsDoesNotExecute(t *testing.T) {
	r := runCLI("ptab", "decisions", "--trial", testPTABTrial, "--dry-run")
	assertExitCode(t, r, 0)
	assertContains(t, r.Stderr, "GET /api/v1/patent/trials/decisions/search")
	assertNotContains(t, r.Stdout, "TRIALNUMBER")
}

func TestT015c_SearchRejectsConflictingPendingGranted(t *testing.T) {
	r := runCLI("search", "--pending", "--granted", "--limit", "1", "-f", "json", "--minify", "--quiet")
	if r.ExitCode == 0 {
		t.Fatalf("expected failure for conflicting flags, got exit 0\nstdout=%s\nstderr=%s", r.Stdout, r.Stderr)
	}
	assertContains(t, r.Stdout+r.Stderr, "cannot combine --granted and --pending")
}

func TestT015d_AppDocsRejectsInvalidDateLocally(t *testing.T) {
	r := runCLI("app", "docs", testApp, "--from", "2026-99-99", "-f", "json", "--minify", "--quiet")
	if r.ExitCode == 0 {
		t.Fatalf("expected failure for invalid date, got exit 0\nstdout=%s\nstderr=%s", r.Stdout, r.Stderr)
	}
	assertContains(t, r.Stdout+r.Stderr, "expected YYYY-MM-DD")
}

func TestT015e_TimeoutMustBePositive(t *testing.T) {
	r := runCLI("status", "150", "--timeout", "0", "-f", "json", "--minify", "--quiet")
	if r.ExitCode == 0 {
		t.Fatalf("expected failure for timeout=0, got exit 0\nstdout=%s\nstderr=%s", r.Stdout, r.Stderr)
	}
	assertContains(t, r.Stdout+r.Stderr, "invalid --timeout 0")
}

func TestT015f_PetitionDecisionFilterDeniedWorks(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("petition", "search", "--decision", "DENIED", "--limit", "1", "-f", "json", "--minify", "--quiet")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatalf("expected ok=true; stdout=%s stderr=%s", r.Stdout, r.Stderr)
	}
	results := parseResultsArray(t, env.Results)
	if len(results) == 0 {
		t.Fatalf("expected at least 1 result for DENIED decisions")
	}
}
