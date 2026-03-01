package integration

import "testing"

func TestT003a_AppMeta(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "meta", testApp)
	assertExitCode(t, r, 0)
	assertContains(t, r.Stdout, "Patented Case")
	assertContains(t, r.Stdout, testPatentNum)
}

func TestT003b_AppMetaJSON(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "meta", testApp, "-f", "json", "-q")
	assertExitCode(t, r, 0)
	env := parseEnvelope(t, r.Stdout)
	if !env.OK {
		t.Error("expected ok=true")
	}
}

func TestT003c_InvalidAppNumber(t *testing.T) {
	r := runCLI("app", "meta", "abc123")
	if r.ExitCode == 0 {
		t.Error("expected non-zero exit code for invalid app number")
	}
	combined := r.Stdout + r.Stderr
	assertContains(t, combined, "invalid")
}

func TestT003d_NonexistentApp(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "meta", testInvalidApp)
	assertExitCode(t, r, 4) // ExitNotFound
}
