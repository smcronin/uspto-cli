package integration

import (
	"testing"
)

func TestT008a_SimpleFamily(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("family", testApp, "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	obj := parseResultsObject(t, env.Results)
	// Should have allApplicationNumbers field.
	allApps, ok := obj["allApplicationNumbers"]
	if !ok {
		t.Fatal("expected 'allApplicationNumbers' in results")
	}
	apps, ok := allApps.([]interface{})
	if !ok {
		t.Fatalf("expected allApplicationNumbers to be array, got %T", allApps)
	}
	if len(apps) < 2 {
		t.Errorf("expected >= 2 family members, got %d", len(apps))
	}
}

func TestT008b_ComplexFamily_BUG006(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("family", testFamilyRoot, "--depth", "3", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	obj := parseResultsObject(t, env.Results)
	allApps, ok := obj["allApplicationNumbers"]
	if !ok {
		t.Fatal("expected 'allApplicationNumbers' in results")
	}
	apps, ok := allApps.([]interface{})
	if !ok {
		t.Fatalf("expected allApplicationNumbers to be array, got %T", allApps)
	}
	// BUG-006: should have exactly 16 unique members, not ~36 with dupes.
	if len(apps) < 14 || len(apps) > 20 {
		t.Errorf("BUG-006 regression: expected ~16 family members, got %d", len(apps))
	}
	// Verify no duplicates.
	seen := make(map[string]bool)
	for _, a := range apps {
		s, _ := a.(string)
		if seen[s] {
			t.Errorf("BUG-006 regression: duplicate family member: %s", s)
		}
		seen[s] = true
	}
}
