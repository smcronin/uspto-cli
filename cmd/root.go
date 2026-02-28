package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/sethcronin/uspto-cli/internal/api"
	"github.com/sethcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

const version = "0.2.0"

// Global flag values accessible to all subcommands.
var (
	flagAPIKey  string
	flagDebug   bool
	flagFormat  string
	flagNoColor bool
	flagQuiet   bool
	flagTimeout int
	flagDryRun  bool
	flagMinify  bool
)

// rootCmd is the top-level command for the USPTO CLI.
var rootCmd = &cobra.Command{
	Use:     "uspto",
	Short:   "USPTO Open Data Portal CLI - Agent-ready patent data access",
	Long:    "USPTO Open Data Portal CLI - Agent-ready patent data access.\n\nAccess patent applications, PTAB proceedings, petition decisions,\nassignments, and more from the USPTO Open Data Portal API.\n\nSet your API key via --api-key or the USPTO_API_KEY environment variable.",
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
	// Load .env file if present; ignore error if missing.
	_ = godotenv.Load()

	// Resolve API key: flag takes precedence over env var.
	if flagAPIKey == "" {
		flagAPIKey = os.Getenv("USPTO_API_KEY")
	}

	// Respect NO_COLOR convention (https://no-color.org/).
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		flagNoColor = true
	}
	if flagNoColor {
		color.NoColor = true
	}

	// Set up the API client singleton.
	opts := []api.ClientOption{
		api.WithDebug(flagDebug),
	}
	if flagTimeout > 0 {
		opts = append(opts, api.WithTimeout(time.Duration(flagTimeout)*time.Second))
	}
	api.DefaultClient = api.NewClient(flagAPIKey, opts...)

	return nil
}

// Execute runs the root command and exits with the appropriate code.
func Execute() {
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

	if apiErr, ok := err.(*api.UsptoAPIError); ok {
		errInfo.Code = apiErr.StatusCode
		errInfo.Message = apiErr.Message

		switch {
		case apiErr.StatusCode == 403:
			code = types.ExitAuthFailure
			errInfo.Type = "AUTH_FAILURE"
		case apiErr.StatusCode == 404:
			code = types.ExitNotFound
			errInfo.Type = "NOT_FOUND"
		case apiErr.StatusCode == 429:
			code = types.ExitRateLimited
			errInfo.Type = "RATE_LIMITED"
		case apiErr.StatusCode >= 500:
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
			fmt.Fprintln(os.Stderr, "Check your API key. Set USPTO_API_KEY or use --api-key.")
		} else if errInfo.Type == "RATE_LIMITED" {
			fmt.Fprintln(os.Stderr, "Rate limit exceeded. Wait a moment and retry.")
		}
	}

	return code
}
