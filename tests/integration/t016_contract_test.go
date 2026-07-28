package integration

import "testing"

func TestT016a_ApplicationCommandResolvesPatentIdentifier(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "meta", testPatentNum, "--id-type", "patent", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	results := parseResultsArray(t, env.Results)
	if len(results) != 1 {
		t.Fatalf("expected one metadata result, got %d", len(results))
	}
	if got, _ := results[0]["applicationNumberText"].(string); got != testApp {
		t.Fatalf("applicationNumberText = %q, want %q", got, testApp)
	}
}

func TestT016b_PetitionAdvancedSearchContract(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("petition", "search", "--fields", "petitionDecisionRecordIdentifier,patentNumber", "--filter", "finalDecidingOfficeName=OFFICE OF PETITIONS", "--range", "petitionMailDate=2020-01-01:2026-12-31", "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
}

func TestT016c_PetitionDownloadContract(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("petition", "search", "--decision", "DENIED", "--limit", "1", "--download", "json")
	assertExitCode(t, r, 0)
	if len(r.Stdout) == 0 {
		t.Fatal("expected download response body")
	}
}
