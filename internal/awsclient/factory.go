package awsclient

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
	"github.com/windkube/aws-metrics-exporter/internal/config"
)

const (
	sessionName   = "aws-metrics-exporter"
	defaultRegion = "us-east-1"
)

// Factory builds per-account, per-region AWS configs that assume the account's exporter role. The
// process credentials come from the ambient chain, which in Kubernetes is the IRSA web identity.
type Factory struct {
	base aws.Config

	mu     sync.Mutex
	cached map[string]aws.Config
}

func NewFactory(ctx context.Context) (*Factory, error) {
	base, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &Factory{base: base, cached: map[string]aws.Config{}}, nil
}

func (f *Factory) Config(account config.Account, region string) aws.Config {
	if region == "" || region == collector.GlobalRegion {
		region = f.base.Region
		if region == "" {
			region = defaultRegion
		}
	}

	key := account.RoleARN + "|" + region

	f.mu.Lock()
	defer f.mu.Unlock()

	if cfg, ok := f.cached[key]; ok {
		return cfg
	}

	cfg := f.base.Copy()
	cfg.Region = region

	// The STS client keeps the ambient credentials; only the returned config assumes the role.
	stsClient := sts.NewFromConfig(cfg)
	cfg.Credentials = aws.NewCredentialsCache(
		stscreds.NewAssumeRoleProvider(stsClient, account.RoleARN, func(o *stscreds.AssumeRoleOptions) {
			o.RoleSessionName = sessionName
			if account.ExternalID != "" {
				o.ExternalID = aws.String(account.ExternalID)
			}
		}),
	)
	otelaws.AppendMiddlewares(&cfg.APIOptions)

	f.cached[key] = cfg

	return cfg
}
