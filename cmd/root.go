package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/config"
	"github.com/smcronin/uspto-cli/internal/tmsearch"
	"github.com/smcronin/uspto-cli/internal/tsdr"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

var version = "dev"

// Global flag values accessible to all subcommands.
var (
	flagAPIKey     string
	flagTSDRAPIKey string
	flagDebug      bool
	flagFormat     string
	flagNoColor    bool
	flagQuiet      bool
	flagTimeout    int
	flagDryRun     bool
	flagMinify     bool
	activeCommand  string
	activePath     string
	activeProvider apiProvider
)

// rootCmd is the top-level command for the USPTO CLI.
var rootCmd = &cobra.Command{
	Use:     "uspto",
	Short:   "Agent-ready USPTO patent and trademark data access",
	Long:    "Agent-ready access to USPTO patent and trademark data.\n\nSearch patents through the Open Data Portal, search marks through the official\nTrademark Search companion service, and retrieve trademark status, documents,\nand images through TSDR. Patent ODP and trademark TSDR use separate API keys.\n\nRun `uspto config show` to inspect both credential slots.",
	Version: version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initConfig(cmd)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	// Global persistent flags available to all subcommands.
	pf := rootCmd.PersistentFlags()

	pf.StringVar(&flagAPIKey, "api-key", "", "USPTO API key (or set USPTO_API_KEY env var)")
	pf.StringVar(&flagTSDRAPIKey, "tsdr-api-key", "", "Trademark TSDR key (or set USPTO_TSDR_API_KEY)")
	pf.BoolVar(&flagDebug, "debug", false, "Enable debug logging")
	pf.StringVarP(&flagFormat, "format", "f", "table", "Output format: table, json, csv, ndjson")
	pf.BoolVar(&flagNoColor, "no-color", false, "Disable color output (also respects NO_COLOR env)")
	pf.BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress non-data output (counts, progress)")
	pf.IntVar(&flagTimeout, "timeout", 30, "Request timeout in seconds")
	pf.BoolVar(&flagDryRun, "dry-run", false, "Show the API request without executing it")
	pf.BoolVar(&flagMinify, "minify", false, "Compact JSON output (no indentation)")
}

// initConfig runs before every command. It loads environment variables,
// resolves the API key, configures color output, and sets up the API client.
func initConfig(cmd *cobra.Command) error {
	setActiveCommand(cmd)
	resolvedKey, err := resolveAPIKey()
	if err != nil {
		return err
	}
	flagAPIKey = resolvedKey
	resolvedTSDRKey, err := resolveTSDRAPIKey()
	if err != nil {
		return err
	}
	flagTSDRAPIKey = resolvedTSDRKey

	// Respect NO_COLOR convention (https://no-color.org/).
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		flagNoColor = true
	}
	if flagNoColor {
		color.NoColor = true
	}

	provider := providerForCommand(cmd)

	// Warn early if no patent ODP key is configured. Preserve the historical
	// warning-only behavior for patent commands.
	if flagAPIKey == "" && !flagDryRun && provider == providerODP {
		configPath, _ := config.ConfigFilePath()
		fmt.Fprintln(os.Stderr, "Warning: no API key configured. Requests will fail with 403.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  Set a key with: uspto config set-api-key <your-key>")
		fmt.Fprintln(os.Stderr, "  Or set USPTO_API_KEY in your environment / pass --api-key")
		if configPath != "" {
			fmt.Fprintf(os.Stderr, "  Global config path: %s\n", configPath)
		}
		fmt.Fprintln(os.Stderr, "  Get a key: https://data.uspto.gov/apis/getting-started")
		fmt.Fprintln(os.Stderr, "  Setup guide: https://github.com/smcronin/uspto-cli/blob/main/docs/api-key-setup.md")
		fmt.Fprintln(os.Stderr, "")
	}
	// TSDR never falls back to the patent ODP key. Fail before issuing a
	// guaranteed-to-fail request and give agents a deterministic recovery path.
	if flagTSDRAPIKey == "" && !flagDryRun && provider == providerTSDR {
		return &credentialError{
			provider: "TSDR",
			message:  "no trademark TSDR API key configured",
			hint:     "Get a separate key at https://account.uspto.gov/api-manager/ and set it with `uspto config set-tsdr-api-key`; the patent ODP key will not work",
		}
	}

	// Set up the API client singleton.
	if flagTimeout <= 0 {
		return &invalidArgsError{message: fmt.Sprintf("invalid --timeout %d: must be > 0 seconds", flagTimeout)}
	}

	opts := []api.ClientOption{
		api.WithDebug(flagDebug),
	}
	opts = append(opts, api.WithTimeout(time.Duration(flagTimeout)*time.Second))
	api.DefaultClient = api.NewClient(flagAPIKey, opts...)
	tsdr.DefaultClient = tsdr.NewClient(flagTSDRAPIKey,
		tsdr.WithDebug(flagDebug),
		tsdr.WithTimeout(time.Duration(flagTimeout)*time.Second),
		tsdr.WithDownloadTimeout(time.Duration(flagTimeout)*time.Second),
	)
	tmsearch.DefaultClient = tmsearch.NewClient(
		tmsearch.WithDebug(flagDebug),
		tmsearch.WithTimeout(time.Duration(flagTimeout)*time.Second),
	)

	return nil
}

// resolveTSDRAPIKey resolves the completely separate trademark credential:
// 1) --tsdr-api-key flag
// 2) USPTO_TSDR_API_KEY environment variable
// 3) legacy TSDR_API_KEY environment variable
// 4) global uspto config file
func resolveTSDRAPIKey() (string, error) {
	if flagTSDRAPIKey != "" {
		return strings.TrimSpace(flagTSDRAPIKey), nil
	}
	if envKey := strings.TrimSpace(os.Getenv(config.TSDRAPIKeyEnvVar)); envKey != "" {
		return envKey, nil
	}
	if envKey := strings.TrimSpace(os.Getenv(config.LegacyTSDRAPIKeyEnvVar)); envKey != "" {
		return envKey, nil
	}
	return config.LoadTSDRAPIKey()
}

type apiProvider string

const (
	providerNone     apiProvider = "none"
	providerODP      apiProvider = "odp"
	providerTMSearch apiProvider = "tmsearch"
	providerTSDR     apiProvider = "tsdr"
)

// providerForCommand makes credential behavior explicit. Trademark Search is
// an anonymous companion service; other trademark operations use TSDR.
func providerForCommand(cmd *cobra.Command) apiProvider {
	if cmd == nil || (cmd.Parent() == nil && cmd.Name() == "uspto") {
		return providerNone
	}
	path := strings.Fields(cmd.CommandPath())
	if len(path) >= 2 && path[1] == "trademark" {
		if len(path) == 2 || (len(path) == 3 && (path[2] == "case" || path[2] == "docs" || path[2] == "multimedia")) {
			return providerNone
		}
		if path[2] == "search" {
			return providerTMSearch
		}
		return providerTSDR
	}
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case "help", "completion", "version", "config", "update":
			return providerNone
		}
	}
	return providerODP
}

// resolveAPIKey resolves API key precedence:
// 1) --api-key flag
// 2) USPTO_API_KEY environment variable
// 3) global uspto config file
func resolveAPIKey() (string, error) {
	if flagAPIKey != "" {
		return strings.TrimSpace(flagAPIKey), nil
	}
	if envKey := strings.TrimSpace(os.Getenv(config.APIKeyEnvVar)); envKey != "" {
		return envKey, nil
	}
	return config.LoadAPIKey()
}

// isNonAPICommand returns true for commands that don't call the USPTO API.
func isNonAPICommand(cmd *cobra.Command) bool {
	return providerForCommand(cmd) == providerNone
}

type credentialError struct {
	provider string
	message  string
	hint     string
}

func (e *credentialError) Error() string { return e.message }

type invalidArgsError struct{ message string }

func (e *invalidArgsError) Error() string { return e.message }

func setActiveCommand(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	activeCommand = cmd.Name()
	activePath = cmd.CommandPath()
	activeProvider = providerForCommand(cmd)
}

// Execute runs the root command and exits with the appropriate code.
func Execute() {
	if command, _, err := rootCmd.Find(os.Args[1:]); err == nil && command != nil {
		setActiveCommand(command)
	}
	if err := rootCmd.Execute(); err != nil {
		exitCode := handleError(err)
		os.Exit(exitCode)
	}
}

// handleError inspects an error and returns the appropriate exit code.
// When the output format is JSON, it also writes a structured error
// envelope to stdout so agents can parse failures programmatically.
func handleError(err error) int {
	code := types.ExitGeneralError
	errInfo := &types.CLIError{
		Code:    0,
		Type:    "GENERAL_ERROR",
		Message: err.Error(),
	}
	var argsErr *invalidArgsError
	if errors.As(err, &argsErr) {
		code = types.ExitInvalidArgs
		errInfo.Type = "INVALID_ARGUMENTS"
		errInfo.Hint = "Run the command with --help to inspect its required nesting and arguments."
	}
	if isCobraUsageError(err) {
		code = types.ExitInvalidArgs
		errInfo.Type = "INVALID_ARGUMENTS"
		errInfo.Hint = "Run the command with --help to inspect its required arguments and flags."
	}

	var authErr *credentialError
	if errors.As(err, &authErr) {
		code = types.ExitAuthFailure
		errInfo.Type = "AUTH_FAILURE"
		errInfo.Message = authErr.message
		errInfo.Hint = authErr.hint
	}

	var apiErr *api.UsptoAPIError
	if errors.As(err, &apiErr) {
		errInfo.Code = apiErr.StatusCode
		errInfo.Message = apiErr.Message

		switch {
		case apiErr.StatusCode == 401 || apiErr.StatusCode == 403:
			code = types.ExitAuthFailure
			errInfo.Type = "AUTH_FAILURE"
			errInfo.Hint = "Set USPTO_API_KEY or use --api-key. Get a key at https://data.uspto.gov/apis/getting-started"
		case apiErr.StatusCode == 404:
			code = types.ExitNotFound
			errInfo.Type = "NOT_FOUND"
		case apiErr.StatusCode == 429:
			code = types.ExitRateLimited
			errInfo.Type = "RATE_LIMITED"
			errInfo.Hint = "Rate limit exceeded. Wait a moment and retry."
		case apiErr.StatusCode >= 500:
			code = types.ExitServerError
			errInfo.Type = "SERVER_ERROR"
		}
	}

	var tsdrErr *tsdr.APIError
	if errors.As(err, &tsdrErr) {
		errInfo.Code = tsdrErr.StatusCode
		// APIError bounds the upstream body and truncates its printable detail.
		// Preserve that context in structured mode so agents see the same useful
		// parameter-level explanation as text-mode users.
		errInfo.Message = tsdrErr.Error()
		switch {
		case tsdrErr.StatusCode == 400:
			code = types.ExitInvalidArgs
			errInfo.Type = "INVALID_ARGUMENTS"
			errInfo.Hint = "Fix the trademark identifier, selector, filter, or raw request parameters and retry."
		case tsdrErr.StatusCode == 406:
			code = types.ExitInvalidArgs
			errInfo.Type = "NOT_ACCEPTABLE"
			errInfo.Hint = "Use the route's explicit representation suffix and a matching Accept header."
		case tsdrErr.StatusCode == 401:
			code = types.ExitAuthFailure
			errInfo.Type = "AUTH_FAILURE"
			errInfo.Hint = "Set USPTO_TSDR_API_KEY or use --tsdr-api-key. Get a separate TSDR key at https://account.uspto.gov/api-manager/; the patent ODP key will not work."
		case tsdrErr.StatusCode == 403:
			code = types.ExitAuthFailure
			errInfo.Type = "ACCESS_FORBIDDEN"
			errInfo.Hint = "Verify the TSDR key with `uspto trademark api-spec -f json`. If that GET succeeds, the requested route or Swagger-advertised POST method may be gateway-blocked; prefer its GET alias rather than replacing a valid key."
		case tsdrErr.StatusCode == 404:
			code = types.ExitNotFound
			errInfo.Type = "NOT_FOUND"
			errInfo.Hint = "Verify the identifier namespace and route. TSDR may also return 404 for an invalid key; run `uspto trademark api-spec -f json` to verify the credential."
		case tsdrErr.StatusCode == 429:
			code = types.ExitRateLimited
			errInfo.Type = "RATE_LIMITED"
			errInfo.RetryAfterSeconds = retryAfterSeconds(tsdrErr.RetryAfter)
			errInfo.Hint = "TSDR rate limit exceeded. Resume no earlier than retryAfterSeconds when present; metadata permits 60 requests/minute and PDF/ZIP bundles permit 4/minute during peak hours."
		case tsdrErr.StatusCode >= 500:
			code = types.ExitServerError
			errInfo.Type = "SERVER_ERROR"
		}
	}

	var searchErr *tmsearch.HTTPError
	if errors.As(err, &searchErr) {
		errInfo.Code = searchErr.StatusCode
		errInfo.Message = searchErr.Error()
		switch {
		case searchErr.StatusCode == 400:
			code = types.ExitInvalidArgs
			errInfo.Type = "INVALID_ARGUMENTS"
			errInfo.Hint = "Fix the Trademark Search query, fields, filters, sorting, or paging arguments and retry."
		case searchErr.StatusCode == 406:
			code = types.ExitInvalidArgs
			errInfo.Type = "NOT_ACCEPTABLE"
			errInfo.Hint = "Use a response representation accepted by the Trademark Search service."
		case searchErr.StatusCode == 429:
			code = types.ExitRateLimited
			errInfo.Type = "RATE_LIMITED"
			errInfo.RetryAfterSeconds = retryAfterSeconds(searchErr.RetryAfter())
			errInfo.Hint = "The public Trademark Search service is rate limited; resume no earlier than retryAfterSeconds when present. No TSDR key is required for search."
		case searchErr.StatusCode == 404:
			code = types.ExitNotFound
			errInfo.Type = "NOT_FOUND"
		case searchErr.StatusCode >= 500:
			code = types.ExitServerError
			errInfo.Type = "SERVER_ERROR"
		}
	}

	// In JSON mode, output structured error to stdout for agent parsing.
	if flagFormat == "json" || flagFormat == "ndjson" {
		outputErrorJSON(errInfo)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		if errInfo.Type == "AUTH_FAILURE" {
			if authErr != nil && authErr.provider == "TSDR" || tsdrErr != nil {
				fmt.Fprintln(os.Stderr, "Trademark retrieval uses its own TSDR credential; the patent ODP key will not work.")
				fmt.Fprintln(os.Stderr, "Get a key: https://account.uspto.gov/api-manager/")
				fmt.Fprintln(os.Stderr, "Set it: uspto config set-tsdr-api-key <your-key>")
			} else {
				fmt.Fprintln(os.Stderr, "Check your API key. Set USPTO_API_KEY or use --api-key.")
				fmt.Fprintln(os.Stderr, "Need a key? https://data.uspto.gov/apis/getting-started")
			}
		} else if errInfo.Type == "RATE_LIMITED" {
			fmt.Fprintln(os.Stderr, "Rate limit exceeded. Wait a moment and retry.")
			if errInfo.RetryAfterSeconds > 0 {
				fmt.Fprintf(os.Stderr, "Retry after at least %d seconds.\n", errInfo.RetryAfterSeconds)
			}
		} else if errInfo.Type == "NOT_FOUND" && tsdrErr != nil {
			fmt.Fprintln(os.Stderr, "Verify the trademark identifier prefix (sn:, rn:, ir:, ref:, or pn:).")
			fmt.Fprintln(os.Stderr, "If identifiers look correct, verify the TSDR key with: uspto trademark api-spec -f json")
		} else if errInfo.Hint != "" {
			fmt.Fprintf(os.Stderr, "Hint: %s\n", errInfo.Hint)
		}
	}

	return code
}

func retryAfterSeconds(delay time.Duration) int64 {
	if delay <= 0 {
		return 0
	}
	return int64((delay + time.Second - 1) / time.Second)
}

func isCobraUsageError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, prefix := range []string{
		"accepts ", "requires ", "unknown flag:", "unknown shorthand flag:",
		"required flag(s) ", "unknown command ",
	} {
		if strings.HasPrefix(message, prefix) {
			return true
		}
	}
	return false
}
