package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
	"github.com/windkube/aws-metrics-exporter/internal/config"
	"github.com/windkube/aws-metrics-exporter/internal/temporal/activities"
)

const AccountScrapeName = "AccountScrape"

type Target struct {
	ResourceType string `json:"resource_type"`
	Region       string `json:"region"`
}

// AccountScrapeInput is the Schedule action argument. Targets are resolved from configuration by
// the schedule reconciler rather than inside the workflow, so a replay never depends on the
// config the worker happens to hold.
type AccountScrapeInput struct {
	Account config.Account `json:"account"`
	Targets []Target       `json:"targets"`
}

type TargetResult struct {
	Target Target `json:"target"`
	Count  int    `json:"count"`
	Error  string `json:"error,omitempty"`
}

type AccountScrapeResult struct {
	Account string         `json:"account"`
	Count   int            `json:"count"`
	Targets []TargetResult `json:"targets"`
}

// TargetsFor expands an account into the units of work to run for it. Global resource types are
// scraped once; regional ones are scraped once per configured region.
func TargetsFor(account config.Account, registry *collector.Registry) ([]Target, error) {
	var targets []Target

	for _, resourceType := range account.Resources {
		c, err := registry.Get(resourceType)
		if err != nil {
			return nil, err
		}

		if c.Scope() == collector.ScopeGlobal {
			targets = append(targets, Target{ResourceType: resourceType, Region: collector.GlobalRegion})
			continue
		}

		for _, region := range account.Regions {
			targets = append(targets, Target{ResourceType: resourceType, Region: region})
		}
	}

	return targets, nil
}

func AccountScrape(ctx workflow.Context, in AccountScrapeInput) (AccountScrapeResult, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout:    15 * time.Minute,
		ScheduleToCloseTimeout: time.Hour,
		HeartbeatTimeout:       2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			MaximumInterval:    5 * time.Minute,
			BackoffCoefficient: 2,
		},
	})

	scrapedAt := workflow.Now(ctx).UTC()

	futures := make([]workflow.Future, 0, len(in.Targets))
	for _, target := range in.Targets {
		futures = append(futures, workflow.ExecuteActivity(ctx, activities.ScrapeResourceName, activities.ScrapeInput{
			Account:      in.Account,
			ResourceType: target.ResourceType,
			Region:       target.Region,
			ScrapedAt:    scrapedAt,
		}))
	}

	result := AccountScrapeResult{Account: in.Account.Name, Targets: make([]TargetResult, 0, len(futures))}
	failed := 0

	for i, future := range futures {
		var out activities.ScrapeOutput
		if err := future.Get(ctx, &out); err != nil {
			failed++
			result.Targets = append(result.Targets, TargetResult{Target: in.Targets[i], Error: err.Error()})
			continue
		}

		result.Count += out.Count
		result.Targets = append(result.Targets, TargetResult{Target: in.Targets[i], Count: out.Count})
	}

	// One broken region must not hide the accounts that did scrape, but a total failure should
	// surface as a failed workflow.
	if failed > 0 && failed == len(futures) {
		return result, fmt.Errorf("all %d targets failed for account %s", failed, in.Account.Name)
	}

	return result, nil
}
