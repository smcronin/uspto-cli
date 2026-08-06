package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// APIKeyEnvVar is the environment variable name used by USPTO APIs.
	APIKeyEnvVar = "USPTO_API_KEY"

	// TSDRAPIKeyEnvVar is the canonical environment variable used by the
	// Trademark Status and Document Retrieval API. TSDR uses a separate
	// credential and authentication system from the Open Data Portal.
	TSDRAPIKeyEnvVar = "USPTO_TSDR_API_KEY"

	// LegacyTSDRAPIKeyEnvVar is accepted for compatibility with early builds
	// and existing dotenv files. New configuration is written with the
	// canonical USPTO_TSDR_API_KEY name.
	LegacyTSDRAPIKeyEnvVar = "TSDR_API_KEY"

	// ConfigDirOverrideEnvVar lets tests override the OS config directory.
	ConfigDirOverrideEnvVar = "USPTO_CLI_CONFIG_DIR"

	configDirName       = "uspto"
	legacyConfigDirName = "uspto-cli"
	configFileName      = "config.env"
)

// ConfigFilePath returns the absolute path of the global config file.
func ConfigFilePath() (string, error) {
	base := strings.TrimSpace(os.Getenv(ConfigDirOverrideEnvVar))
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolving user config directory: %w", err)
		}
	}
	return filepath.Join(base, configDirName, configFileName), nil
}

// LoadAPIKey reads the API key from the global config file.
// It returns an empty string when the file does not exist or the key is unset.
func LoadAPIKey() (string, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return "", err
	}
	key, err := loadAPIKeyFromFile(path)
	if err != nil {
		return "", err
	}
	if key != "" {
		return key, nil
	}

	legacyPath, err := legacyConfigFilePath()
	if err != nil {
		return "", err
	}
	legacyKey, err := loadAPIKeyFromFile(legacyPath)
	if err != nil {
		return "", err
	}
	if legacyKey == "" {
		return "", nil
	}

	// Best-effort migration to new path; return key even if write fails.
	_ = saveAPIKeyToPath(path, legacyKey)
	return legacyKey, nil
}

// LoadTSDRAPIKey reads the trademark TSDR API key from global config.
// It returns an empty string when the file does not exist or the key is unset.
func LoadTSDRAPIKey() (string, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return "", err
	}
	key, err := loadEnvValueFromFile(path, TSDRAPIKeyEnvVar)
	if err != nil || key != "" {
		return key, err
	}
	return loadEnvValueFromFile(path, LegacyTSDRAPIKeyEnvVar)
}

// LoadAPIKeyFromDotEnv reads USPTO_API_KEY from a dotenv file path.
func LoadAPIKeyFromDotEnv(path string) (string, error) {
	return loadAPIKeyFromFile(path)
}

// LoadTSDRAPIKeyFromDotEnv reads TSDR_API_KEY from a dotenv file path.
func LoadTSDRAPIKeyFromDotEnv(path string) (string, error) {
	key, err := loadEnvValueFromFile(path, TSDRAPIKeyEnvVar)
	if err != nil || key != "" {
		return key, err
	}
	return loadEnvValueFromFile(path, LegacyTSDRAPIKeyEnvVar)
}

// SaveAPIKey writes the API key to the global config file and returns the path.
func SaveAPIKey(apiKey string) (string, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", fmt.Errorf("API key cannot be empty")
	}

	path, err := ConfigFilePath()
	if err != nil {
		return "", err
	}
	if err := saveAPIKeyToPath(path, apiKey); err != nil {
		return "", err
	}

	return path, nil
}

// SaveTSDRAPIKey writes the separate trademark TSDR key to global config
// without replacing an existing Open Data Portal key.
func SaveTSDRAPIKey(apiKey string) (string, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", fmt.Errorf("TSDR API key cannot be empty")
	}

	path, err := ConfigFilePath()
	if err != nil {
		return "", err
	}
	if err := saveConfigValue(path, TSDRAPIKeyEnvVar, apiKey); err != nil {
		return "", err
	}
	return path, nil
}

// MaskAPIKey returns a redacted version of an API key for display.
func MaskAPIKey(apiKey string) string {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return ""
	}
	if len(apiKey) <= 8 {
		return strings.Repeat("*", len(apiKey))
	}
	return apiKey[:4] + strings.Repeat("*", len(apiKey)-8) + apiKey[len(apiKey)-4:]
}

func loadAPIKeyFromFile(path string) (string, error) {
	return loadEnvValueFromFile(path, APIKeyEnvVar)
}

func loadEnvValueFromFile(path, envVar string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading %s: %w", path, err)
	}

	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if key != envVar {
			continue
		}
		return unquoteEnvValue(val), nil
	}

	return "", nil
}

func saveAPIKeyToPath(path, apiKey string) error {
	return saveConfigValue(path, APIKeyEnvVar, apiKey)
}

func saveConfigValue(path, envVar, value string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("securing config directory: %w", err)
	}
	release, err := acquireConfigLock(path + ".lock")
	if err != nil {
		return err
	}
	defer release()

	var lines []string
	if data, err := os.ReadFile(path); err == nil {
		lines = strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading config file: %w", err)
	}

	replacement := envVar + "=" + quoteEnvValue(value)
	replaced := false
	for i, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		if eq := strings.Index(line, "="); eq >= 0 && strings.TrimSpace(line[:eq]) == envVar {
			lines[i] = replacement
			replaced = true
			break
		}
	}

	if len(lines) == 0 || (len(lines) == 1 && strings.TrimSpace(lines[0]) == "") {
		lines = []string{"# uspto global config"}
	}
	if !replaced {
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, replacement)
	}
	content := strings.Join(lines, "\n") + "\n"
	temp, err := os.CreateTemp(dir, ".config.env-*")
	if err != nil {
		return fmt.Errorf("creating temporary config file: %w", err)
	}
	tempPath := temp.Name()
	committed := false
	defer func() {
		_ = temp.Close()
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		return fmt.Errorf("securing temporary config file: %w", err)
	}
	if _, err := temp.WriteString(content); err != nil {
		return fmt.Errorf("writing temporary config file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("syncing temporary config file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("closing temporary config file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("atomically replacing config file: %w", err)
	}
	committed = true
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("securing config file: %w", err)
	}
	if err := syncConfigDirectory(dir); err != nil {
		return fmt.Errorf("syncing config directory: %w", err)
	}
	return nil
}

func acquireConfigLock(path string) (func(), error) {
	deadline := time.Now().Add(10 * time.Second)
	for {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = file.WriteString(strconv.Itoa(os.Getpid()))
			_ = file.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("acquiring config lock: %w", err)
		}
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > 2*time.Minute {
			_ = os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for config lock %s", path)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func legacyConfigFilePath() (string, error) {
	base := strings.TrimSpace(os.Getenv(ConfigDirOverrideEnvVar))
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolving user config directory: %w", err)
		}
	}
	return filepath.Join(base, legacyConfigDirName, configFileName), nil
}

func quoteEnvValue(v string) string {
	v = strings.ReplaceAll(v, "\\", "\\\\")
	v = strings.ReplaceAll(v, "\"", "\\\"")
	return "\"" + v + "\""
}

func unquoteEnvValue(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			v = v[1 : len(v)-1]
		}
	}
	v = strings.ReplaceAll(v, "\\\"", "\"")
	v = strings.ReplaceAll(v, "\\\\", "\\")
	return strings.TrimSpace(v)
}
