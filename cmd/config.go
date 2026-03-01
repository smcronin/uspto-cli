package cmd

import (
	"fmt"
	"os"

	"github.com/smcronin/uspto-cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	configFromDotEnvPath string
	configFromEnv        bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage global CLI configuration",
	Long: `Manage global uspto-cli configuration.

The API key is stored in a user-level config file, so commands work from
any directory without relying on a local .env file.`,
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
		if apiKey == "" {
			fmt.Fprintf(os.Stdout, "%s: not set\n", config.APIKeyEnvVar)
			return nil
		}
		fmt.Fprintf(os.Stdout, "%s: %s\n", config.APIKeyEnvVar, config.MaskAPIKey(apiKey))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetAPIKeyCmd)
	configCmd.AddCommand(configShowCmd)

	configSetAPIKeyCmd.Flags().BoolVar(&configFromEnv, "from-env", false, "Read API key from USPTO_API_KEY in current environment")
	configSetAPIKeyCmd.Flags().StringVar(&configFromDotEnvPath, "from-dotenv", "", "Read API key from a dotenv file path")
}
