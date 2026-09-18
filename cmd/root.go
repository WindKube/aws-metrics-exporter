package cmd

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	configPath string
	version    = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "aws-metrics-exporter",
	Short: "Scrape AWS account inventory and export it to a storage backend",
	Long: `aws-metrics-exporter periodically scrapes AWS objects from every configured account and
writes their AWS JSON representation to a storage backend, using Temporal to split the work per
account and resource type.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute(ctx context.Context, v string) int {
	version = v

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		log.Error().Err(err).Msg("command failed")
		return 1
	}

	return 0
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "config.yaml", "path to the config file")
}
