package integration

import "testing"

func TestT004a_TitleSearch(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("search", "--title", "wireless sensor network", "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	results := parseResultsArray(t, env.Results)
	if len(results) < 1 {
		t.Error("expected at least 1 result")
	}
	if env.Pagination == nil || env.Pagination.Total < 100 {
		t.Errorf("expected total > 100, got %+v", env.Pagination)
	}
}

func TestT004b_AssigneeSearch_BUG001(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("search", "--assignee", "Google", "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("BUG-001 regression: assignee search failed (was crashing on correspondenceAddress)")
	}
}

func TestT004c_PatentNumberSearch(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("search", "--patent", testPatentNum, "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	results := parseResultsArray(t, env.Results)
	if len(results) < 1 {
		t.Fatal("expected at least 1 result")
	}
	appNum, _ := results[0]["applicationNumberText"].(string)
	if appNum != testApp {
		t.Errorf("expected applicationNumberText=%s, got %s", testApp, appNum)
	}
}

func TestT004d_ExaminerSearch_BUG001(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("search", "--examiner", "SABOURI", "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("BUG-001 regression: examiner search failed")
	}
}

func TestT004e_FreeTextSearch_BUG001(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("search", "artificial intelligence", "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("BUG-001 regression: free-text search failed")
	}
}

func TestT004f_GrantedFilter(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("search", "--title", "battery", "--granted", "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
}

func TestT004g_DateRange(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("search", "--title", "drone", "--filed-within", "6m", "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
}

func TestT004h_DryRun(t *testing.T) {
	r := runCLI("search", "--title", "battery", "--limit", "3", "--dry-run")
	assertExitCode(t, r, 0)
	combined := r.Stdout + r.Stderr
	assertContains(t, combined, "/api/v1/")
}

func TestT004i_Pagination(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("search", "--title", "battery cathode", "--limit", "3", "--page", "2", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	if env.Pagination == nil || env.Pagination.Offset != 3 {
		t.Errorf("expected offset=3, got %+v", env.Pagination)
	}
}
