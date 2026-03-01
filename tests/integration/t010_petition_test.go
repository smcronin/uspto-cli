package integration

import "testing"

func TestT010a_PetitionSearch_BUG003(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("petition", "search", "--limit", "3", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("BUG-003 regression: petition search failed (was failing on prosecutionStatusCode type)")
	}
}

func TestT010b_PetitionGet(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("petition", "get", testPetitionID, "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
}
