package integration

import "testing"

func TestT013a_MissingRequiredArg(t *testing.T) {
	r := runCLI("app", "meta")
	if r.ExitCode == 0 {
		t.Error("expected non-zero exit code for missing required arg")
	}
	combined := r.Stdout + r.Stderr
	assertContains(t, combined, "arg")
}

func TestT013b_AuthError(t *testing.T) {
	r := runCLINoKey("app", "meta", testApp)
	// Should warn about missing API key or fail with auth error.
	if r.ExitCode == 0 {
		// Some commands may succeed without key for certain endpoints,
		// but app meta should require auth.
		combined := r.Stdout + r.Stderr
		assertContains(t, combined, "API")
	} else {
		assertExitCode(t, r, 3) // ExitAuthError
	}
}

func TestT013c_NotFoundExitCode(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "meta", testInvalidApp)
	assertExitCode(t, r, 4) // ExitNotFound
}

func TestT013d_DebugOutput(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "meta", testApp, "--debug", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	assertContains(t, r.Stderr, "[DEBUG]")
	assertContains(t, r.Stderr, "api.uspto.gov")
}
