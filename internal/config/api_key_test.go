package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSaveAndLoadAPIKey(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv(ConfigDirOverrideEnvVar, tempRoot)

	path, err := SaveAPIKey("abc123-secret")
	if err != nil {
		t.Fatalf("SaveAPIKey() error: %v", err)
	}

	if !strings.Contains(path, filepath.Join(tempRoot, configDirName)) {
		t.Fatalf("SaveAPIKey() path %q does not include expected root %q", path, tempRoot)
	}

	got, err := LoadAPIKey()
	if err != nil {
		t.Fatalf("LoadAPIKey() error: %v", err)
	}
	if got != "abc123-secret" {
		t.Fatalf("LoadAPIKey() = %q, want %q", got, "abc123-secret")
	}
}

func TestLoadAPIKey_FileMissing(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv(ConfigDirOverrideEnvVar, tempRoot)

	got, err := LoadAPIKey()
	if err != nil {
		t.Fatalf("LoadAPIKey() error: %v", err)
	}
	if got != "" {
		t.Fatalf("LoadAPIKey() = %q, want empty", got)
	}
}

func TestLoadAPIKey_LegacyConfigMigrates(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv(ConfigDirOverrideEnvVar, tempRoot)

	legacyPath := filepath.Join(tempRoot, legacyConfigDirName, configFileName)
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatalf("mkdir legacy config dir: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("USPTO_API_KEY=legacy-key\n"), 0o600); err != nil {
		t.Fatalf("write legacy config file: %v", err)
	}

	got, err := LoadAPIKey()
	if err != nil {
		t.Fatalf("LoadAPIKey() error: %v", err)
	}
	if got != "legacy-key" {
		t.Fatalf("LoadAPIKey() = %q, want %q", got, "legacy-key")
	}

	newPath := filepath.Join(tempRoot, configDirName, configFileName)
	migrated, err := loadAPIKeyFromFile(newPath)
	if err != nil {
		t.Fatalf("load migrated key: %v", err)
	}
	if migrated != "legacy-key" {
		t.Fatalf("migrated key = %q, want %q", migrated, "legacy-key")
	}
}

func TestLoadAPIKeyFromDotEnv(t *testing.T) {
	tempDir := t.TempDir()
	dotenvPath := filepath.Join(tempDir, ".env")
	content := `
# comment
FOO=bar
export USPTO_API_KEY="quoted-value"
`
	if err := os.WriteFile(dotenvPath, []byte(content), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	got, err := LoadAPIKeyFromDotEnv(dotenvPath)
	if err != nil {
		t.Fatalf("LoadAPIKeyFromDotEnv() error: %v", err)
	}
	if got != "quoted-value" {
		t.Fatalf("LoadAPIKeyFromDotEnv() = %q, want %q", got, "quoted-value")
	}
}

func TestSaveAndLoadTSDRAPIKey_PreservesODPKey(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv(ConfigDirOverrideEnvVar, tempRoot)

	if _, err := SaveAPIKey("odp-secret"); err != nil {
		t.Fatalf("SaveAPIKey() error: %v", err)
	}
	if _, err := SaveTSDRAPIKey("tsdr-secret"); err != nil {
		t.Fatalf("SaveTSDRAPIKey() error: %v", err)
	}

	odp, err := LoadAPIKey()
	if err != nil {
		t.Fatalf("LoadAPIKey() error: %v", err)
	}
	tsdr, err := LoadTSDRAPIKey()
	if err != nil {
		t.Fatalf("LoadTSDRAPIKey() error: %v", err)
	}
	if odp != "odp-secret" || tsdr != "tsdr-secret" {
		t.Fatalf("loaded keys = (%q, %q), want (%q, %q)", odp, tsdr, "odp-secret", "tsdr-secret")
	}
}

func TestLoadTSDRAPIKeyFromDotEnv(t *testing.T) {
	tempDir := t.TempDir()
	dotenvPath := filepath.Join(tempDir, ".env")
	content := "USPTO_API_KEY=odp-value\nexport TSDR_API_KEY='trademark-value'\n"
	if err := os.WriteFile(dotenvPath, []byte(content), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	got, err := LoadTSDRAPIKeyFromDotEnv(dotenvPath)
	if err != nil {
		t.Fatalf("LoadTSDRAPIKeyFromDotEnv() error: %v", err)
	}
	if got != "trademark-value" {
		t.Fatalf("LoadTSDRAPIKeyFromDotEnv() = %q, want %q", got, "trademark-value")
	}
}

func TestSaveAPIKey_PreservesTSDRKey(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv(ConfigDirOverrideEnvVar, tempRoot)

	if _, err := SaveTSDRAPIKey("trademark-value"); err != nil {
		t.Fatalf("SaveTSDRAPIKey() error: %v", err)
	}
	if _, err := SaveAPIKey("new-odp-value"); err != nil {
		t.Fatalf("SaveAPIKey() error: %v", err)
	}
	tsdr, err := LoadTSDRAPIKey()
	if err != nil {
		t.Fatalf("LoadTSDRAPIKey() error: %v", err)
	}
	if tsdr != "trademark-value" {
		t.Fatalf("TSDR key = %q, want preserved value", tsdr)
	}
}

func TestSaveAPIKey_Empty(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv(ConfigDirOverrideEnvVar, tempRoot)

	if _, err := SaveAPIKey("   "); err == nil {
		t.Fatal("SaveAPIKey() expected error for empty key, got nil")
	}
}

func TestMaskAPIKey(t *testing.T) {
	if got := MaskAPIKey(""); got != "" {
		t.Fatalf("MaskAPIKey(\"\") = %q, want empty", got)
	}
	if got := MaskAPIKey("12345678"); got != "********" {
		t.Fatalf("MaskAPIKey(short) = %q, want %q", got, "********")
	}
	if got := MaskAPIKey("1234567890abcdef"); got != "1234********cdef" {
		t.Fatalf("MaskAPIKey(long) = %q, want %q", got, "1234********cdef")
	}
}

func TestConcurrentProviderKeySavesPreserveBothValues(t *testing.T) {
	tempRoot := t.TempDir()
	t.Setenv(ConfigDirOverrideEnvVar, tempRoot)

	for iteration := 0; iteration < 20; iteration++ {
		var wait sync.WaitGroup
		wait.Add(2)
		errors := make(chan error, 2)
		go func() {
			defer wait.Done()
			_, err := SaveAPIKey("odp-concurrent")
			errors <- err
		}()
		go func() {
			defer wait.Done()
			_, err := SaveTSDRAPIKey("tsdr-concurrent")
			errors <- err
		}()
		wait.Wait()
		close(errors)
		for err := range errors {
			if err != nil {
				t.Fatalf("concurrent save: %v", err)
			}
		}
	}

	odp, err := LoadAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	trademark, err := LoadTSDRAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if odp != "odp-concurrent" || trademark != "tsdr-concurrent" {
		t.Fatalf("concurrent keys = %q/%q", odp, trademark)
	}
	path, _ := ConfigFilePath()
	if matches, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".config.env-*")); len(matches) != 0 {
		t.Fatalf("temporary config files left behind: %v", matches)
	}
}
