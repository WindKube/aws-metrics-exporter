package temporal

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

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

	tlsConfig, err := tlsConfig(cfg.TLS)
	if err != nil {
		return nil, err
	}

	return client.Dial(client.Options{
		HostPort:          cfg.Address,
		Namespace:         cfg.Namespace,
		Logger:            observability.TemporalLogger(),
		Interceptors:      []interceptor.ClientInterceptor{tracing},
		ConnectionOptions: client.ConnectionOptions{TLS: tlsConfig},
	})
}

func tlsConfig(cfg config.TemporalTLS) (*tls.Config, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	out := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec // opt-in, off by default
	}

	if cfg.CertFile != "" || cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("temporal tls: client key pair: %w", err)
		}
		out.Certificates = []tls.Certificate{cert}
	}

	if cfg.CAFile != "" {
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("temporal tls: ca file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("temporal tls: ca file %s holds no certificate", cfg.CAFile)
		}
		out.RootCAs = pool
	}

	return out, nil
}
