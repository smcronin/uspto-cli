package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Package-level vars set by TestMain.
var (
	binaryPath  string
	projectRoot string
	hasAPIKey   bool
)

// Test fixture constants — known-good application/trial IDs.
const (
	testApp         = "16123456"                                   // FUJIFILM, Pat. 10902286
	testPatentNum   = "10902286"                                   // Corresponding patent number
	testFamilyRoot  = "12477075"                                   // Apple slide-to-unlock, 16 members
	testPTABTrial   = "IPR2016-00134"                              // NVIDIA v. Samsung
	testAppealID    = "650ad83e-56f0-4d7d-843a-5f238d16951f"       // Known PTAB appeal UUID
	testPetitionID  = "0d5f5afa-d456-52b4-81e2-d4e51d7c801b"       // Known petition UUID
	testBulkProduct = "PTGRXML"                                    // Patent Grant XML bulk product
	testInvalidApp  = "99999999"                                   // Nonexistent app → 404
)

func TestMain(m *testing.M) {
	// Determine binary name based on OS.
	binName := "uspto-cli-test"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}

	// Build binary from project root.
	projectRoot, _ = filepath.Abs(filepath.Join("..", ".."))
	binaryPath = filepath.Join(projectRoot, binName)

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to build test binary: %v\n", err)
		os.Exit(1)
	}

	// Check for API key — either from environment or from .env file.
	hasAPIKey = os.Getenv("USPTO_API_KEY") != ""
	if !hasAPIKey {
		if data, err := os.ReadFile(filepath.Join(projectRoot, ".env")); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "USPTO_API_KEY=") {
					val := strings.TrimPrefix(line, "USPTO_API_KEY=")
					val = strings.Trim(val, `"'`)
					if val != "" {
						hasAPIKey = true
					}
					break
				}
			}
		}
	}

	// Run tests.
	code := m.Run()

	// Cleanup.
	os.Remove(binaryPath)
	os.Exit(code)
}
