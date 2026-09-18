package temporal

import (
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"

	"github.com/windkube/aws-metrics-exporter/internal/config"
	"github.com/windkube/aws-metrics-exporter/internal/observability"
)

func NewClient(cfg config.Temporal) (client.Client, error) {
	tracing, err := opentelemetry.NewTracingInterceptor(opentelemetry.TracerOptions{})
	if err != nil {
		return nil, err
	}

	return client.Dial(client.Options{
		HostPort:     cfg.Address,
		Namespace:    cfg.Namespace,
		Logger:       observability.TemporalLogger(),
		Interceptors: []interceptor.ClientInterceptor{tracing},
	})
}
