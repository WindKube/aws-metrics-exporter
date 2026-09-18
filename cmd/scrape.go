package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/windkube/aws-metrics-exporter/internal/awsclient"
	"github.com/windkube/aws-metrics-exporter/internal/scraper"
	"github.com/windkube/aws-metrics-exporter/internal/storage"
	"github.com/windkube/aws-metrics-exporter/internal/temporal/workflows"
)

var (
	scrapeAccount  string
	scrapeResource string
	scrapeRegion   string
	scrapeDryRun   bool
)

var scrapeCmd = &cobra.Command{
	Use:   "scrape",
	Short: "Scrape one account directly, without Temporal",
	Long: `Runs a collector against a single account and writes the result to the configured backend.
Use it to check a new account's IAM permissions or to develop a collector without a Temporal server.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		rt, err := bootstrap(ctx)
		if err != nil {
			return err
		}
		defer rt.Close()

		account, ok := rt.cfg.Account(scrapeAccount)
		if !ok {
			return fmt.Errorf("account %q is not in %s", scrapeAccount, configPath)
		}

		if scrapeRegion != "" {
			account.Regions = []string{scrapeRegion}
		}
		if scrapeResource != "" {
			account.Resources = []string{scrapeResource}
		}

		targets, err := workflows.TargetsFor(account, rt.registry)
		if err != nil {
			return err
		}

		var store storage.Store
		if scrapeDryRun {
			store = countingStore{}
		} else {
			store, err = rt.newStore(ctx)
			if err != nil {
				return err
			}
		}
		defer func() { _ = store.Close(context.WithoutCancel(ctx)) }()

		factory, err := awsclient.NewFactory(ctx)
		if err != nil {
			return err
		}

		s := scraper.New(factory, rt.registry, store, rt.cfg.Storage.Elasticsearch.BatchSize)
		scrapedAt := time.Now().UTC()

		for _, target := range targets {
			result, err := s.Scrape(ctx, scraper.Request{
				Account:      account,
				ResourceType: target.ResourceType,
				Region:       target.Region,
				ScrapedAt:    scrapedAt,
			}, nil)
			if err != nil {
				return fmt.Errorf("scraping %s in %s: %w", target.ResourceType, target.Region, err)
			}

			log.Info().
				Str("account", account.Name).
				Str("resource_type", target.ResourceType).
				Str("region", target.Region).
				Int("count", result.Count).
				Bool("dry_run", scrapeDryRun).
				Msg("scraped")
		}

		return nil
	},
}

// countingStore stands in for the real backend under --dry-run so the AWS side can be exercised
// without writing anything.
type countingStore struct{}

func (countingStore) Write(_ context.Context, docs []storage.Document) error {
	for _, doc := range docs {
		log.Debug().Str("id", doc.ID()).Msg("would index")
	}
	return nil
}

func (countingStore) Ping(context.Context) error  { return nil }
func (countingStore) Close(context.Context) error { return nil }

func init() {
	scrapeCmd.Flags().StringVar(&scrapeAccount, "account", "", "account name from the config file (required)")
	scrapeCmd.Flags().StringVar(&scrapeResource, "resource", "", "resource type to scrape (default: every type configured for the account)")
	scrapeCmd.Flags().StringVar(&scrapeRegion, "region", "", "region to scrape (default: every region configured for the account)")
	scrapeCmd.Flags().BoolVar(&scrapeDryRun, "dry-run", false, "collect from AWS but do not write to the backend")
	_ = scrapeCmd.MarkFlagRequired("account")

	rootCmd.AddCommand(scrapeCmd)
}
