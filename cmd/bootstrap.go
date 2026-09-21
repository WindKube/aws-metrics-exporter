package cmd

import (
	"context"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
	"github.com/windkube/aws-metrics-exporter/internal/collector/all"
	"github.com/windkube/aws-metrics-exporter/internal/config"
	"github.com/windkube/aws-metrics-exporter/internal/observability"
	"github.com/windkube/aws-metrics-exporter/internal/storage/elasticsearch"
)

type runtime struct {
	cfg      *config.Config
	registry *collector.Registry
	shutdown []func()
}

// bootstrap loads configuration and brings up logging, Sentry and tracing. The caller must defer
// runtime.Close.
func bootstrap(ctx context.Context) (*runtime, error) {
	registry := all.Registry()

	cfg, err := config.Load(configPath, registry.Types())
	if err != nil {
		return nil, err
	}

	sentryWriter, flushSentry, err := observability.InitSentry(version)
	if err != nil {
		return nil, err
	}

	if err = observability.ConfigureLogging(cfg.Global.Log, sentryWriter); err != nil {
		flushSentry()
		return nil, err
	}

	shutdownTracing, err := observability.InitTracing(ctx, version)
	if err != nil {
		flushSentry()
		return nil, err
	}

	return &runtime{
		cfg:      cfg,
		registry: registry,
		shutdown: []func(){
			func() { _ = shutdownTracing(context.WithoutCancel(ctx)) },
			flushSentry,
		},
	}, nil
}

func (r *runtime) Close() {
	for _, fn := range r.shutdown {
		fn()
	}
}

func (r *runtime) newStore() (*elasticsearch.Client, error) {
	return elasticsearch.New(r.cfg.Storage.Elasticsearch)
}
