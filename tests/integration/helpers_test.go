package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// cmdResult captures the output of a CLI invocation.
type cmdResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CLIEnvelope mirrors types.CLIResponse but uses json.RawMessage for Results
// so tests can parse results as array or object as needed.
type CLIEnvelope struct {
	OK         bool            `json:"ok"`
	Command    string          `json:"command"`
	Pagination *PaginationMeta `json:"pagination,omitempty"`
	Results    json.RawMessage `json:"results"`
	Version    string          `json:"version"`
	Error      *CLIError       `json:"error,omitempty"`
}

// PaginationMeta mirrors types.PaginationMeta for test parsing.
type PaginationMeta struct {
	Offset  int  `json:"offset"`
	Limit   int  `json:"limit"`
	Total   int  `json:"total"`
	HasMore bool `json:"hasMore"`
}

// CLIError mirrors types.CLIError for test parsing.
type CLIError struct {
	Code    int    `json:"code"`
	Type    string `json:"type"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// runCLI executes the test binary with the given args and captures output.
// It inherits the current environment (including USPTO_API_KEY).
func runCLI(args ...string) cmdResult {
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = projectRoot
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return cmdResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// runCLINoKey executes the test binary with USPTO_API_KEY explicitly unset.
func runCLINoKey(args ...string) cmdResult {
	cmd := exec.Command(binaryPath, args...)
	// Use temp dir and strip USPTO_API_KEY to guarantee a no-key invocation.
	cmd.Dir = os.TempDir()
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Copy env but strip USPTO_API_KEY.
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "USPTO_API_KEY=") {
			filtered = append(filtered, e)
		}
	}
	cfgDir, _ := os.MkdirTemp("", "uspto-cli-no-key-config-*")
	if cfgDir != "" {
		defer os.RemoveAll(cfgDir)
		filtered = append(filtered, "USPTO_CLI_CONFIG_DIR="+cfgDir)
	}
	cmd.Env = filtered

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return cmdResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// parseEnvelope parses stdout as a CLI JSON envelope.
func parseEnvelope(t *testing.T, stdout string) CLIEnvelope {
	t.Helper()
	var env CLIEnvelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("failed to parse JSON envelope: %v\nstdout: %s", err, stdout)
	}
	return env
}

// parseResultsArray parses the Results field as a JSON array of objects.
func parseResultsArray(t *testing.T, raw json.RawMessage) []map[string]interface{} {
	t.Helper()
	var arr []map[string]interface{}
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("failed to parse results as array: %v\nraw: %s", err, string(raw))
	}
	return arr
}

// parseResultsObject parses the Results field as a single JSON object.
func parseResultsObject(t *testing.T, raw json.RawMessage) map[string]interface{} {
	t.Helper()
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("failed to parse results as object: %v\nraw: %s", err, string(raw))
	}
	return obj
}

// requireAPIKey skips the test if USPTO_API_KEY is not set.
func requireAPIKey(t *testing.T) {
	t.Helper()
	if !hasAPIKey {
		t.Skip("USPTO_API_KEY not set, skipping API test")
	}
}

// assertExitCode asserts the exit code matches expected.
func assertExitCode(t *testing.T, r cmdResult, expected int) {
	t.Helper()
	if r.ExitCode != expected {
		t.Errorf("exit code = %d, want %d\nstdout: %s\nstderr: %s",
			r.ExitCode, expected, r.Stdout, r.Stderr)
	}
}

// assertContains asserts that s contains substr.
func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected output to contain %q, got:\n%s", substr, truncate(s, 500))
	}
}

// assertNotContains asserts that s does NOT contain substr.
func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf("expected output NOT to contain %q, got:\n%s", substr, truncate(s, 500))
	}
}

// nonEmptyLines returns lines from s that are non-empty after trimming whitespace.
func nonEmptyLines(s string) []string {
	lines := strings.Split(s, "\n")
	var result []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			result = append(result, l)
		}
	}
	return result
}

// truncate shortens s to maxLen characters for readable test output.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}
