package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// APIKeyEnvVar is the environment variable name used by USPTO APIs.
	APIKeyEnvVar = "USPTO_API_KEY"

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

// LoadAPIKeyFromDotEnv reads USPTO_API_KEY from a dotenv file path.
func LoadAPIKeyFromDotEnv(path string) (string, error) {
	return loadAPIKeyFromFile(path)
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
		if key != APIKeyEnvVar {
			continue
		}
		return unquoteEnvValue(val), nil
	}

	return "", nil
}

func saveAPIKeyToPath(path, apiKey string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	content := "# uspto global config\n" + APIKeyEnvVar + "=" + quoteEnvValue(apiKey) + "\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
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
