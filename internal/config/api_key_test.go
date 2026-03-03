package config

import (
	"os"
	"path/filepath"
	"strings"
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
