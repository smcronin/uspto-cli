package integration

import "testing"

func TestT007a_SummaryJSON(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("summary", testApp, "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	// Summary returns an object, not an array.
	obj := parseResultsObject(t, env.Results)
	if obj["title"] == nil {
		t.Error("expected results to have 'title' field")
	}
}
