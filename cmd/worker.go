package cmd

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"go.temporal.io/sdk/client"

	"github.com/windkube/aws-metrics-exporter/internal/awsclient"
	"github.com/windkube/aws-metrics-exporter/internal/health"
	"github.com/windkube/aws-metrics-exporter/internal/scraper"
	temporalclient "github.com/windkube/aws-metrics-exporter/internal/temporal"
	"github.com/windkube/aws-metrics-exporter/internal/temporal/activities"
	"github.com/windkube/aws-metrics-exporter/internal/temporal/schedules"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Run the Temporal worker, reconcile schedules and serve health probes",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()

		rt, err := bootstrap(ctx)
		if err != nil {
			return err
		}
		defer rt.Close()

		store, err := rt.newStore()
		if err != nil {
			return err
		}
		defer func() { _ = store.Close(context.WithoutCancel(ctx)) }()

		temporalClient, err := temporalclient.NewClient(rt.cfg.Global.Temporal)
		if err != nil {
			return err
		}
		defer temporalClient.Close()

		factory, err := awsclient.NewFactory(ctx)
		if err != nil {
			return err
		}

		s := scraper.New(factory, rt.registry, store, rt.cfg.Storage.Elasticsearch.BatchSize)
		w := temporalclient.NewWorker(temporalClient, rt.cfg.Global.Temporal, activities.New(s))

		if err = w.Start(); err != nil {
			return err
		}
		defer w.Stop()

		changes, err := schedules.NewReconciler(temporalClient, rt.registry, rt.cfg.Global.Temporal.TaskQueue).
			Apply(ctx, rt.cfg)
		if err != nil {
			return err
		}
		for _, change := range changes {
			log.Info().Str("schedule", change.ID).Str("action", change.Action).Msg("reconciled schedule")
		}

		prober := health.NewProber(rt.cfg.Global.Health.ProbeInterval,
			health.Check{Name: "storage", Func: store.Ping},
			health.Check{Name: "temporal", Func: func(ctx context.Context) error {
				_, err := temporalClient.CheckHealth(ctx, &client.CheckHealthRequest{})
				return err
			}},
		)
		go prober.Run(ctx)

		server := health.NewServer(rt.cfg.Global.Health.Address, prober)
		go func() {
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error().Err(err).Msg("health server failed")
			}
		}()

		log.Info().
			Str("task_queue", rt.cfg.Global.Temporal.TaskQueue).
			Str("health_address", rt.cfg.Global.Health.Address).
			Msg("worker started")

		<-ctx.Done()
		log.Info().Msg("shutting down")

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()

		return server.Shutdown(shutdownCtx)
	},
}

func init() {
	rootCmd.AddCommand(workerCmd)
}
