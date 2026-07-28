package cmd

import (
	"testing"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/config"
	"github.com/smcronin/uspto-cli/internal/types"
)

func TestHandleError_ClassifiesAuthAndInputFailures(t *testing.T) {
	if got := handleError(&api.UsptoAPIError{StatusCode: 401, Message: "Unauthorized"}); got != types.ExitAuthFailure {
		t.Fatalf("401 exit code = %d, want %d", got, types.ExitAuthFailure)
	}
	if got := handleError(invalidArgsf("invalid --limit -1: must be > 0")); got != types.ExitInvalidArgs {
		t.Fatalf("input error exit code = %d, want %d", got, types.ExitInvalidArgs)
	}
}

func TestResolveAPIKey_Precedence(t *testing.T) {
	origFlag := flagAPIKey
	defer func() {
		flagAPIKey = origFlag
	}()

	t.Run("flag beats env and global", func(t *testing.T) {
		t.Setenv(config.ConfigDirOverrideEnvVar, t.TempDir())
		t.Setenv(config.APIKeyEnvVar, "env-key")
		if _, err := config.SaveAPIKey("global-key"); err != nil {
			t.Fatalf("SaveAPIKey() error: %v", err)
		}

		flagAPIKey = "flag-key"
		got, err := resolveAPIKey()
		if err != nil {
			t.Fatalf("resolveAPIKey() error: %v", err)
		}
		if got != "flag-key" {
			t.Fatalf("resolveAPIKey() = %q, want %q", got, "flag-key")
		}
	})

	t.Run("env beats global", func(t *testing.T) {
		t.Setenv(config.ConfigDirOverrideEnvVar, t.TempDir())
		t.Setenv(config.APIKeyEnvVar, "env-key")
		if _, err := config.SaveAPIKey("global-key"); err != nil {
			t.Fatalf("SaveAPIKey() error: %v", err)
		}

		flagAPIKey = ""
		got, err := resolveAPIKey()
		if err != nil {
			t.Fatalf("resolveAPIKey() error: %v", err)
		}
		if got != "env-key" {
			t.Fatalf("resolveAPIKey() = %q, want %q", got, "env-key")
		}
	})

	t.Run("global used when flag and env absent", func(t *testing.T) {
		t.Setenv(config.ConfigDirOverrideEnvVar, t.TempDir())
		t.Setenv(config.APIKeyEnvVar, "")
		if _, err := config.SaveAPIKey("global-key"); err != nil {
			t.Fatalf("SaveAPIKey() error: %v", err)
		}

		flagAPIKey = ""
		got, err := resolveAPIKey()
		if err != nil {
			t.Fatalf("resolveAPIKey() error: %v", err)
		}
		if got != "global-key" {
			t.Fatalf("resolveAPIKey() = %q, want %q", got, "global-key")
		}
	})

	t.Run("empty when no source configured", func(t *testing.T) {
		t.Setenv(config.ConfigDirOverrideEnvVar, t.TempDir())
		t.Setenv(config.APIKeyEnvVar, "")
		flagAPIKey = ""

		got, err := resolveAPIKey()
		if err != nil {
			t.Fatalf("resolveAPIKey() error: %v", err)
		}
		if got != "" {
			t.Fatalf("resolveAPIKey() = %q, want empty", got)
		}
	})
}

func TestIsNonAPICommand(t *testing.T) {
	if !isNonAPICommand(rootCmd) {
		t.Fatal("root command should be treated as non-API")
	}

	cfgCmd, _, err := rootCmd.Find([]string{"config", "show"})
	if err != nil {
		t.Fatalf("rootCmd.Find(config show): %v", err)
	}
	if !isNonAPICommand(cfgCmd) {
		t.Fatal("config subcommand should be treated as non-API")
	}

	updateCmd, _, err := rootCmd.Find([]string{"update"})
	if err != nil {
		t.Fatalf("rootCmd.Find(update): %v", err)
	}
	if !isNonAPICommand(updateCmd) {
		t.Fatal("update command should be treated as non-API")
	}

	searchCmd, _, err := rootCmd.Find([]string{"search"})
	if err != nil {
		t.Fatalf("rootCmd.Find(search): %v", err)
	}
	if isNonAPICommand(searchCmd) {
		t.Fatal("search command should be treated as API command")
	}
}
