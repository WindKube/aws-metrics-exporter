package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/windkube/aws-metrics-exporter/internal/collector/all"
	"github.com/windkube/aws-metrics-exporter/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect and validate the configuration file",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the resolved configuration, with credentials omitted",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load(configPath, all.Registry().Types())
		if err != nil {
			return err
		}

		out, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return nil
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Check the configuration file and exit non-zero if it is invalid",
	RunE: func(cmd *cobra.Command, _ []string) error {
		registry := all.Registry()

		cfg, err := config.Load(configPath, registry.Types())
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%s is valid: %d accounts, resource types %s\n",
			configPath, len(cfg.Accounts), strings.Join(registry.Types(), ", "))
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd, configValidateCmd)
	rootCmd.AddCommand(configCmd)
}
