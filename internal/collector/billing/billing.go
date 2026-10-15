package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

const ResourceType = "aws_billing"

// API is the subset of the Cost Explorer client this collector uses.
type API interface {
	GetCostAndUsage(context.Context, *costexplorer.GetCostAndUsageInput, ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error)
}

// Cost is one month-to-date figure, either for a single service or, with Service empty, for the
// whole account.
type Cost struct {
	Period    *types.DateInterval
	Service   string `json:",omitempty"`
	Metrics   map[string]types.MetricValue
	Estimated bool
}

type Collector struct {
	newAPI func(aws.Config) API
	now    func() time.Time
}

func New() *Collector {
	return &Collector{
		newAPI: func(cfg aws.Config) API { return costexplorer.NewFromConfig(cfg) },
		now:    time.Now,
	}
}

func (c *Collector) Type() string           { return ResourceType }
func (c *Collector) Scope() collector.Scope { return collector.ScopeGlobal }

func (c *Collector) Collect(ctx context.Context, cfg aws.Config, emit collector.EmitFunc) error {
	// Cost Explorer is served from us-east-1 only.
	cfg.Region = "us-east-1"
	api := c.newAPI(cfg)

	today := c.now().UTC().Truncate(24 * time.Hour)
	period := &types.DateInterval{
		Start: aws.String(time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).Format(time.DateOnly)),
		End:   aws.String(today.AddDate(0, 0, 1).Format(time.DateOnly)),
	}
	metrics := []string{"UnblendedCost", "AmortizedCost"}
	month := today.Format("2006-01")

	var token *string
	for {
		out, err := api.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
			TimePeriod:    period,
			Granularity:   types.GranularityMonthly,
			Metrics:       metrics,
			GroupBy:       []types.GroupDefinition{{Type: types.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")}},
			NextPageToken: token,
		})
		if err != nil {
			return fmt.Errorf("getting cost and usage: %w", err)
		}

		for _, result := range out.ResultsByTime {
			for _, group := range result.Groups {
				if len(group.Keys) == 0 {
					continue
				}
				service := group.Keys[0]
				if err := emit(collector.Resource{
					ID:   month + ":service:" + service,
					Name: service,
					Data: Cost{Period: result.TimePeriod, Service: service, Metrics: group.Metrics, Estimated: result.Estimated},
				}); err != nil {
					return err
				}
			}
		}

		token = out.NextPageToken
		if token == nil {
			break
		}
	}

	out, err := api.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod:  period,
		Granularity: types.GranularityMonthly,
		Metrics:     metrics,
	})
	if err != nil {
		return fmt.Errorf("getting cost and usage: %w", err)
	}

	for _, result := range out.ResultsByTime {
		if err := emit(collector.Resource{
			ID:   month + ":total",
			Name: "total",
			Data: Cost{Period: result.TimePeriod, Metrics: result.Total, Estimated: result.Estimated},
		}); err != nil {
			return err
		}
	}

	return nil
}
