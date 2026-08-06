package cmd

import (
	"fmt"
	"os"

	"github.com/smcronin/uspto-cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	configFromDotEnvPath     string
	configFromEnv            bool
	tsdrConfigFromDotEnvPath string
	tsdrConfigFromEnv        bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage global CLI configuration",
	Long: `Manage global uspto configuration.

The API key is stored in a user-level config file, so commands work from
any directory without relying on a local .env file.

Patent Open Data Portal and trademark TSDR credentials are separate. A key
issued for one service does not authenticate to the other.`,
}

var configSetAPIKeyCmd = &cobra.Command{
	Use:   "set-api-key [apiKey]",
	Short: "Persist your USPTO API key in global config",
	Long: `Persist your USPTO API key in global config.

Provide the key as an argument, load it from your current environment
(--from-env), or import it from a dotenv file (--from-dotenv).`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sources := 0
		if len(args) == 1 {
			sources++
		}
		if configFromEnv {
			sources++
		}
		if configFromDotEnvPath != "" {
			sources++
		}
		if sources == 0 {
			return fmt.Errorf("provide an API key, or use --from-env / --from-dotenv")
		}
		if sources > 1 {
			return fmt.Errorf("use only one source: argument, --from-env, or --from-dotenv")
		}

		var apiKey string
		switch {
		case len(args) == 1:
			apiKey = args[0]
		case configFromEnv:
			apiKey = os.Getenv(config.APIKeyEnvVar)
			if apiKey == "" {
				return fmt.Errorf("%s is not set in the environment", config.APIKeyEnvVar)
			}
		case configFromDotEnvPath != "":
			var err error
			apiKey, err = config.LoadAPIKeyFromDotEnv(configFromDotEnvPath)
			if err != nil {
				return fmt.Errorf("reading dotenv file: %w", err)
			}
			if apiKey == "" {
				return fmt.Errorf("no %s found in %s", config.APIKeyEnvVar, configFromDotEnvPath)
			}
		}

		path, err := config.ConfigFilePath()
		if err != nil {
			return err
		}
		if flagDryRun {
			fmt.Fprintf(os.Stdout, "Would save %s to: %s\n", config.APIKeyEnvVar, path)
			return nil
		}

		path, err = config.SaveAPIKey(apiKey)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "Saved API key to global config: %s\n", path)
		fmt.Fprintf(os.Stdout, "Stored key: %s\n", config.MaskAPIKey(apiKey))
		return nil
	},
}

var configSetTSDRAPIKeyCmd = &cobra.Command{
	Use:   "set-tsdr-api-key [apiKey]",
	Short: "Persist the separate trademark TSDR API key",
	Long: `Persist the separate USPTO Trademark Status and Document Retrieval
(TSDR) API key in global config.

Get this key from https://account.uspto.gov/api-manager/. The Open Data Portal
USPTO_API_KEY will not work with TSDR. Provide the key as an argument, load it
from USPTO_TSDR_API_KEY (or legacy TSDR_API_KEY) with --from-env, or import it
from a dotenv file with --from-dotenv.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sources := 0
		if len(args) == 1 {
			sources++
		}
		if tsdrConfigFromEnv {
			sources++
		}
		if tsdrConfigFromDotEnvPath != "" {
			sources++
		}
		if sources == 0 {
			return fmt.Errorf("provide a TSDR API key, or use --from-env / --from-dotenv")
		}
		if sources > 1 {
			return fmt.Errorf("use only one source: argument, --from-env, or --from-dotenv")
		}

		var apiKey string
		switch {
		case len(args) == 1:
			apiKey = args[0]
		case tsdrConfigFromEnv:
			apiKey = os.Getenv(config.TSDRAPIKeyEnvVar)
			if apiKey == "" {
				apiKey = os.Getenv(config.LegacyTSDRAPIKeyEnvVar)
			}
			if apiKey == "" {
				return fmt.Errorf("neither %s nor %s is set in the environment", config.TSDRAPIKeyEnvVar, config.LegacyTSDRAPIKeyEnvVar)
			}
		case tsdrConfigFromDotEnvPath != "":
			var err error
			apiKey, err = config.LoadTSDRAPIKeyFromDotEnv(tsdrConfigFromDotEnvPath)
			if err != nil {
				return fmt.Errorf("reading dotenv file: %w", err)
			}
			if apiKey == "" {
				return fmt.Errorf("no %s or %s found in %s", config.TSDRAPIKeyEnvVar, config.LegacyTSDRAPIKeyEnvVar, tsdrConfigFromDotEnvPath)
			}
		}

		path, err := config.ConfigFilePath()
		if err != nil {
			return err
		}
		if flagDryRun {
			fmt.Fprintf(os.Stdout, "Would save %s to: %s\n", config.TSDRAPIKeyEnvVar, path)
			return nil
		}

		path, err = config.SaveTSDRAPIKey(apiKey)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Saved trademark TSDR API key to global config: %s\n", path)
		fmt.Fprintf(os.Stdout, "Stored key: %s\n", config.MaskAPIKey(apiKey))
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show global config path and API key status",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.ConfigFilePath()
		if err != nil {
			return err
		}
		apiKey, err := config.LoadAPIKey()
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "Config file: %s\n", path)
		tsdrAPIKey, err := config.LoadTSDRAPIKey()
		if err != nil {
			return err
		}

		if apiKey == "" {
			fmt.Fprintf(os.Stdout, "%s (patent ODP): not set\n", config.APIKeyEnvVar)
		} else {
			fmt.Fprintf(os.Stdout, "%s (patent ODP): %s\n", config.APIKeyEnvVar, config.MaskAPIKey(apiKey))
		}
		if tsdrAPIKey == "" {
			fmt.Fprintf(os.Stdout, "%s (trademark TSDR): not set\n", config.TSDRAPIKeyEnvVar)
		} else {
			fmt.Fprintf(os.Stdout, "%s (trademark TSDR): %s\n", config.TSDRAPIKeyEnvVar, config.MaskAPIKey(tsdrAPIKey))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetAPIKeyCmd)
	configCmd.AddCommand(configSetTSDRAPIKeyCmd)
	configCmd.AddCommand(configShowCmd)

	configSetAPIKeyCmd.Flags().BoolVar(&configFromEnv, "from-env", false, "Read API key from USPTO_API_KEY in current environment")
	configSetAPIKeyCmd.Flags().StringVar(&configFromDotEnvPath, "from-dotenv", "", "Read API key from a dotenv file path")
	configSetTSDRAPIKeyCmd.Flags().BoolVar(&tsdrConfigFromEnv, "from-env", false, "Read key from USPTO_TSDR_API_KEY or TSDR_API_KEY")
	configSetTSDRAPIKeyCmd.Flags().StringVar(&tsdrConfigFromDotEnvPath, "from-dotenv", "", "Read trademark TSDR key from a dotenv file path")
}
