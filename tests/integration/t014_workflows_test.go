package integration

import (
	"encoding/json"
	"testing"
)

func TestT014a_PatentToAppNumber(t *testing.T) {
	requireAPIKey(t)
	// Search by patent number, extract applicationNumberText.
	r := runCLI("search", "--patent", testPatentNum, "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("search failed")
	}
	results := parseResultsArray(t, env.Results)
	if len(results) < 1 {
		t.Fatal("expected at least 1 result")
	}
	appNum, _ := results[0]["applicationNumberText"].(string)
	if appNum != testApp {
		t.Errorf("expected app number %s, got %s", testApp, appNum)
	}

	// Chain: use that app number to get summary.
	r2 := runCLI("summary", appNum, "-f", "json", "-q")
	assertExitCode(t, r2, 0)
	env2 := parseEnvelope(t, r2.Stdout)
	if !env2.OK {
		t.Fatal("summary failed for chained app number")
	}
}

func TestT014b_SearchToSummaryChain(t *testing.T) {
	requireAPIKey(t)
	// Search for a specific inventor+title combo.
	r := runCLI("search", "--inventor", "KANADA", "--title", "learning assistance", "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("search failed")
	}
	results := parseResultsArray(t, env.Results)
	if len(results) < 1 {
		t.Fatal("expected at least 1 result")
	}
	appNum, _ := results[0]["applicationNumberText"].(string)
	if appNum == "" {
		t.Fatal("no applicationNumberText in result")
	}

	// Get summary for the found app.
	r2 := runCLI("summary", appNum, "-f", "json", "-q")
	assertExitCode(t, r2, 0)
	env2 := parseEnvelope(t, r2.Stdout)
	if !env2.OK {
		t.Fatal("summary failed")
	}
	obj := parseResultsObject(t, env2.Results)
	title, _ := obj["title"].(string)
	assertContains(t, title, "LEARNING ASSISTANCE")
}

func TestT014c_PTABMonitoring(t *testing.T) {
	requireAPIKey(t)
	// Find an IPR from Apple.
	r := runCLI("ptab", "search", "--petitioner", "Apple", "--type", "IPR", "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)

	var env CLIEnvelope
	if err := json.Unmarshal([]byte(r.Stdout), &env); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if !env.OK {
		t.Fatal("ptab search failed")
	}
	results := parseResultsArray(t, env.Results)
	if len(results) < 1 {
		t.Fatal("expected at least 1 proceeding")
	}
	trialNum, _ := results[0]["trialNumber"].(string)
	if trialNum == "" {
		t.Error("expected non-empty trialNumber")
	}
}
