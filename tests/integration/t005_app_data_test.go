package integration

import "testing"

func TestT005a_Continuity(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "continuity", testApp)
	assertExitCode(t, r, 0)
	assertContains(t, r.Stdout, "17130468")
}

func TestT005b_Assignments_BUG001(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "assignments", testApp, "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("BUG-001 regression: assignments failed (was crashing on correspondenceAddress)")
	}
}

func TestT005c_Attorney(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "attorney", testApp)
	assertExitCode(t, r, 0)
	assertContains(t, r.Stdout, "BIRCH")
}

func TestT005d_Documents(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "docs", testApp)
	assertExitCode(t, r, 0)
	lines := nonEmptyLines(r.Stdout)
	// Expect at least 50 docs (was 52, then 56). Header + separator + rows.
	if len(lines) < 50 {
		t.Errorf("expected >= 50 output lines for docs, got %d", len(lines))
	}
}

func TestT005e_Transactions(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "transactions", testApp)
	assertExitCode(t, r, 0)
	lines := nonEmptyLines(r.Stdout)
	// Expect at least 55 transactions (was 58, then 62). Allow growth.
	if len(lines) < 55 {
		t.Errorf("expected >= 55 output lines for transactions, got %d", len(lines))
	}
}

func TestT005f_PatentTermAdjustment(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "adjustment", testApp)
	assertExitCode(t, r, 0)
	assertContains(t, r.Stdout, "127")
}

func TestT005g_ForeignPriority(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "foreign-priority", testApp)
	assertExitCode(t, r, 0)
	assertContains(t, r.Stdout, "2017-187096")
}

func TestT005h_AssociatedDocs(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "associated-docs", testApp)
	assertExitCode(t, r, 0)
	assertContains(t, r.Stdout, "Grant")
}

func TestT005i_FullApplication_BUG001(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "get", testApp, "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("BUG-001 regression: app get failed")
	}
}
