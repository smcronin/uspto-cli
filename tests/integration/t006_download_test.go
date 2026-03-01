package integration

import (
	"strings"
	"testing"
)

func TestT006a_DownloadAllDryRun_BUG002(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "dl-all", testApp, "--dry-run")
	assertExitCode(t, r, 0)
	combined := r.Stdout + r.Stderr
	// Dry-run should list files (shows DOWNLOAD lines) without actually saving them.
	assertContains(t, combined, "DOWNLOAD")
	// Verify multiple files listed (the test app has 50+ docs).
	lines := nonEmptyLines(combined)
	if len(lines) < 10 {
		t.Errorf("expected dry-run to list many files, got %d lines", len(lines))
	}
}

func TestT006b_DownloadAllFilenames_BUG002(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("app", "dl-all", testApp, "--dry-run")
	assertExitCode(t, r, 0)
	combined := r.Stdout + r.Stderr
	// BUG-002: filenames should use dashes not colons for timestamps.
	// Colons are invalid in Windows filenames.
	lines := nonEmptyLines(combined)
	for _, line := range lines {
		// Look for lines that appear to have file names with time components.
		// Colons between digits like "08:51:27" would be the bug.
		if strings.Contains(line, ".pdf") || strings.Contains(line, ".PDF") {
			if matched := colonTimestamp(line); matched {
				t.Errorf("BUG-002 regression: filename contains colon timestamp: %s", line)
			}
		}
	}
}

// colonTimestamp checks if a line contains a HH:MM:SS pattern that suggests
// colons in a filename timestamp.
func colonTimestamp(line string) bool {
	// Simple check: look for digit:digit:digit pattern.
	for i := 0; i+4 < len(line); i++ {
		if isDigit(line[i]) && line[i+1] == ':' && isDigit(line[i+2]) && line[i+3] == ':' && isDigit(line[i+4]) {
			return true
		}
	}
	return false
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
