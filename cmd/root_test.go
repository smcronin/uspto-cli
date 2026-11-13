package cmd

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/config"
	"github.com/smcronin/uspto-cli/internal/tsdr"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

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

func TestProviderForCommand_NonAPICommands(t *testing.T) {
	if providerForCommand(rootCmd) != providerNone {
		t.Fatal("root command should be treated as non-API")
	}

	cfgCmd, _, err := rootCmd.Find([]string{"config", "show"})
	if err != nil {
		t.Fatalf("rootCmd.Find(config show): %v", err)
	}
	if providerForCommand(cfgCmd) != providerNone {
		t.Fatal("config subcommand should be treated as non-API")
	}

	updateCmd, _, err := rootCmd.Find([]string{"update"})
	if err != nil {
		t.Fatalf("rootCmd.Find(update): %v", err)
	}
	if providerForCommand(updateCmd) != providerNone {
		t.Fatal("update command should be treated as non-API")
	}

	searchCmd, _, err := rootCmd.Find([]string{"search"})
	if err != nil {
		t.Fatalf("rootCmd.Find(search): %v", err)
	}
	if providerForCommand(searchCmd) == providerNone {
		t.Fatal("search command should be treated as API command")
	}
}

func TestHandleError_ODPRateLimitReportsRetryAfter(t *testing.T) {
	origFormat, origMinify := flagFormat, flagMinify
	defer func() { flagFormat, flagMinify = origFormat, origMinify }()
	flagFormat, flagMinify = "json", true

	stdout, _, err := captureTrademarkOutput(t, func() error {
		code := handleError(&api.UsptoAPIError{StatusCode: 429, Message: "Too Many Requests", RetryAfter: 90 * time.Second})
		if code != types.ExitRateLimited {
			t.Errorf("handleError() code = %d, want %d", code, types.ExitRateLimited)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		OK    bool `json:"ok"`
		Error struct {
			Type              string `json:"type"`
			RetryAfterSeconds int64  `json:"retryAfterSeconds"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout)
	}
	if envelope.OK || envelope.Error.Type != "RATE_LIMITED" || envelope.Error.RetryAfterSeconds != 90 {
		t.Errorf("envelope = %+v, want RATE_LIMITED with retryAfterSeconds 90", envelope)
	}
}

// usageEnvelope runs handleError in JSON mode and returns the exit code and
// error.type it produced.
func usageEnvelope(t *testing.T, err error) (int, string) {
	t.Helper()
	origFormat, origMinify := flagFormat, flagMinify
	defer func() { flagFormat, flagMinify = origFormat, origMinify }()
	flagFormat, flagMinify = "json", true

	var code int
	stdout, _, captureErr := captureTrademarkOutput(t, func() error {
		code = handleError(err)
		return nil
	})
	if captureErr != nil {
		t.Fatal(captureErr)
	}
	var envelope struct {
		OK    bool `json:"ok"`
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if jsonErr := json.Unmarshal([]byte(stdout), &envelope); jsonErr != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", jsonErr, stdout)
	}
	if envelope.OK {
		t.Errorf("ok = true for an error envelope:\n%s", stdout)
	}
	return code, envelope.Error.Type
}

// Bad input is a usage error: exit 2 and INVALID_ARGUMENTS, so an agent
// fixes the command instead of retrying it.
func TestHandleError_ValidationErrorsAreUsageErrors(t *testing.T) {
	origSearch := searchFlags
	origPetition := petitionSearchFlags
	defer func() { searchFlags, petitionSearchFlags = origSearch, origPetition }()

	searchInputs := func(mutate func()) func() error {
		return func() error {
			resetSearchFlagsForTest()
			mutate()
			return validateSearchInputs()
		}
	}

	tests := []struct {
		name string
		err  func() error
	}{
		{"application number", func() error { return validateAppNumber("abc") }},
		{"impossible date", func() error { return validateISODate("--from", "2024-02-30") }},
		{"reversed date range", func() error { return validateDateRange("--from", "2024-02-01", "--to", "2024-01-01") }},
		{"sort with three parts", func() error { return validateSortExpr("--sort", "a:b:c") }},
		{"sort order", func() error { return validateSortExpr("--sort", "filingDate:up") }},
		{"search limit", searchInputs(func() { searchFlags.limit = 0 })},
		{"search offset", searchInputs(func() { searchFlags.offset = -1 })},
		{"search page", searchInputs(func() { searchFlags.page = -1 })},
		{"granted with pending", searchInputs(func() { searchFlags.granted, searchFlags.pending = true, true })},
		{"filter without value", searchInputs(func() { searchFlags.filters = []string{"novalue"} })},
		{"filter with empty value", searchInputs(func() { searchFlags.filters = []string{"field="} })},
		{"filed-after month 13", searchInputs(func() { searchFlags.filedAfter = "2024-13-01" })},
		{"filed-within unit", func() error {
			resetSearchFlagsForTest()
			searchFlags.filedWithin = "3w"
			return runSearch(searchCmd, nil)
		}},
		{"download format", func() error { _, _, _, err := resolveDownloadFormat("png"); return err }},
		{"text format", func() error { return validateTextFormatRequest("png") }},
		{"text from pdf", func() error { return validateTextFormatRequest("pdf") }},
		{"document sort field", func() error { _, err := sortDocumentsByDateExpr(nil, "code:asc"); return err }},
		{"document sort order", func() error { _, err := sortDocumentsByDateExpr(nil, "date:up"); return err }},
		{"document index out of range", func() error {
			_, _, err := resolveDocumentSelection([]types.Document{{DocumentIdentifier: "A"}}, "7")
			return err
		}},
		{"document identifier not found", func() error {
			_, _, err := resolveDocumentSelection([]types.Document{{DocumentIdentifier: "A"}}, "ZZZ")
			return err
		}},
		{"bundle id type", func() error { _, err := normalizeBundleIDType("foo"); return err }},
		{"petition limit", func() error {
			petitionSearchFlags = origPetition
			petitionSearchFlags.limit = 0
			return runPetitionSearch(petitionSearchCmd, nil)
		}},
		{"output format", func() error { return validateOutputFormat("yaml") }},
		{"flag value that does not parse", func() error {
			return searchCmd.FlagErrorFunc()(searchCmd, errors.New(`invalid argument "abc" for "--limit" flag`))
		}},
		{"unknown subcommand of app", func() error { return appCmd.Args(appCmd, []string{"nosuchsub"}) }},
		{"unknown subcommand of ptab", func() error { return ptabCmd.Args(ptabCmd, []string{"nosuchsub"}) }},
		{"unknown subcommand of bulk", func() error { return bulkCmd.Args(bulkCmd, []string{"nosuchsub"}) }},
		{"unknown subcommand of petition", func() error { return petitionCmd.Args(petitionCmd, []string{"nosuchsub"}) }},
		{"unknown subcommand of patent", func() error { return patentCmd.Args(patentCmd, []string{"nosuchsub"}) }},
		{"unknown subcommand of config", func() error { return configCmd.Args(configCmd, []string{"nosuchsub"}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.err()
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			code, errType := usageEnvelope(t, err)
			if code != types.ExitInvalidArgs || errType != "INVALID_ARGUMENTS" {
				t.Errorf("exit %d type %q for %q, want exit %d INVALID_ARGUMENTS", code, errType, err, types.ExitInvalidArgs)
			}
		})
	}
}

func TestValidateOutputFormat_AcceptsDocumentedFormats(t *testing.T) {
	for _, format := range []string{"table", "json", "csv", "ndjson"} {
		if err := validateOutputFormat(format); err != nil {
			t.Errorf("validateOutputFormat(%q) = %v, want nil", format, err)
		}
	}
}

// A group command invoked alone still prints its help; only an unknown
// subcommand is an error. Groups never need an API key.
func TestGroupCommands_HelpWithoutArgsAndNoProvider(t *testing.T) {
	for _, group := range []*cobra.Command{appCmd, ptabCmd, bulkCmd, petitionCmd, patentCmd, configCmd} {
		if err := group.Args(group, nil); err != nil {
			t.Errorf("%s with no arguments: %v", group.CommandPath(), err)
		}
		if got := providerForCommand(group); got != providerNone {
			t.Errorf("providerForCommand(%s) = %q, want %q", group.CommandPath(), got, providerNone)
		}
	}
	if got := providerForCommand(appMetaCmd); got != providerODP {
		t.Errorf("providerForCommand(app meta) = %q, want %q", got, providerODP)
	}
}

// initDownloadTimeouts runs initConfig for an ODP command with the given
// --timeout and --download-timeout and returns the two clients' file
// download limits.
func initDownloadTimeouts(t *testing.T, timeoutSeconds int, downloadTimeout downloadTimeoutFlag) (odp, trademark time.Duration) {
	t.Helper()
	origClient, origTSDR := api.DefaultClient, tsdr.DefaultClient
	origKey, origTSDRKey := flagAPIKey, flagTSDRAPIKey
	origTimeout, origDL, origDryRun, origFormat := flagTimeout, flagDownloadTimeout, flagDryRun, flagFormat
	defer func() {
		api.DefaultClient, tsdr.DefaultClient = origClient, origTSDR
		flagAPIKey, flagTSDRAPIKey = origKey, origTSDRKey
		flagTimeout, flagDownloadTimeout, flagDryRun, flagFormat = origTimeout, origDL, origDryRun, origFormat
	}()
	t.Setenv(config.ConfigDirOverrideEnvVar, t.TempDir())
	t.Setenv(config.APIKeyEnvVar, "")
	t.Setenv(config.TSDRAPIKeyEnvVar, "")
	t.Setenv(config.LegacyTSDRAPIKeyEnvVar, "")
	flagAPIKey, flagTSDRAPIKey = "odp-key", "tsdr-key"
	flagTimeout, flagDownloadTimeout, flagDryRun, flagFormat = timeoutSeconds, downloadTimeout, true, "table"

	if err := initConfig(appMetaCmd); err != nil {
		t.Fatalf("initConfig() error: %v", err)
	}
	return api.DefaultClient.DownloadTimeout(), tsdr.DefaultClient.DownloadTimeout()
}

// --timeout is the request limit. It used to be passed on as the TSDR
// download limit too, so every trademark PDF or ZIP had 30 seconds.
func TestInitConfig_TimeoutDoesNotSetDownloadLimits(t *testing.T) {
	odp, trademark := initDownloadTimeouts(t, 5, downloadTimeoutFlag{})
	if odp != 600*time.Second {
		t.Errorf("ODP download timeout = %s with --timeout 5, want the 10m0s default", odp)
	}
	if trademark != 10*time.Minute {
		t.Errorf("TSDR download timeout = %s with --timeout 5, want the package default 10m0s", trademark)
	}
}

func TestInitConfig_DownloadTimeoutSetsBothClients(t *testing.T) {
	tests := []struct {
		raw  string
		want time.Duration
	}{
		{"45m", 45 * time.Minute},
		{"1200", 20 * time.Minute},
		{"0", 0},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			var flag downloadTimeoutFlag
			if err := flag.Set(tt.raw); err != nil {
				t.Fatalf("Set(%q) error: %v", tt.raw, err)
			}
			odp, trademark := initDownloadTimeouts(t, 5, flag)
			if odp != tt.want || trademark != tt.want {
				t.Errorf("download timeouts = ODP %s, TSDR %s; want %s for both", odp, trademark, tt.want)
			}
		})
	}
}

func TestDownloadTimeoutFlag_Parsing(t *testing.T) {
	for _, raw := range []string{"abc", "-5", "-1m", "", "10 minutes"} {
		var flag downloadTimeoutFlag
		if err := flag.Set(raw); err == nil {
			t.Errorf("Set(%q) accepted, want an error", raw)
		}
		if flag.set {
			t.Errorf("Set(%q) failed but marked the flag as set", raw)
		}
	}
	if rootCmd.PersistentFlags().Lookup("download-timeout") == nil {
		t.Fatal("--download-timeout is not registered as a global flag")
	}
	// A value that does not parse is a usage error like any other flag.
	err := rootCmd.FlagErrorFunc()(rootCmd, errors.New(`invalid argument "abc" for "--download-timeout" flag`))
	if code, errType := usageEnvelope(t, err); code != types.ExitInvalidArgs || errType != "INVALID_ARGUMENTS" {
		t.Errorf("exit %d type %q, want exit 2 INVALID_ARGUMENTS", code, errType)
	}
}
