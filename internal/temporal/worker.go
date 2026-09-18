package temporal

import (
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	sdkworkflow "go.temporal.io/sdk/workflow"

	"github.com/windkube/aws-metrics-exporter/internal/config"
	"github.com/windkube/aws-metrics-exporter/internal/temporal/activities"
	"github.com/windkube/aws-metrics-exporter/internal/temporal/workflows"
)

func NewWorker(c client.Client, cfg config.Temporal, acts *activities.Activities) worker.Worker {
	w := worker.New(c, cfg.TaskQueue, worker.Options{})

	w.RegisterWorkflowWithOptions(workflows.AccountScrape, sdkworkflow.RegisterOptions{
		Name: workflows.AccountScrapeName,
	})
	w.RegisterActivityWithOptions(acts.ScrapeResource, sdkactivity.RegisterOptions{
		Name: activities.ScrapeResourceName,
	})

	return w
}
