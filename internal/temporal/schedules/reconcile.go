package schedules

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
	"github.com/windkube/aws-metrics-exporter/internal/config"
	"github.com/windkube/aws-metrics-exporter/internal/temporal/workflows"
)

// IDPrefix scopes the schedules this reconciler owns; anything else in the namespace is left alone.
const IDPrefix = "ame-acct-"

const catchupWindow = time.Hour

func ID(account string) string { return IDPrefix + account }

type Reconciler struct {
	client    client.Client
	registry  *collector.Registry
	taskQueue string
}

func NewReconciler(c client.Client, registry *collector.Registry, taskQueue string) *Reconciler {
	return &Reconciler{client: c, registry: registry, taskQueue: taskQueue}
}

type Change struct {
	ID     string
	Action string
}

// Apply makes the namespace's schedules match cfg: one per account, orphans removed. It is
// idempotent so several replicas can run it concurrently.
func (r *Reconciler) Apply(ctx context.Context, cfg *config.Config) ([]Change, error) {
	desired := make(map[string]config.Account, len(cfg.Accounts))
	for _, account := range cfg.Accounts {
		desired[ID(account.Name)] = account
	}

	existing, err := r.list(ctx)
	if err != nil {
		return nil, err
	}

	var changes []Change

	for id, account := range desired {
		action := "updated"
		if _, ok := existing[id]; !ok {
			action = "created"
		}
		if err := r.upsert(ctx, id, account); err != nil {
			return changes, err
		}
		changes = append(changes, Change{ID: id, Action: action})
	}

	for id := range existing {
		if _, ok := desired[id]; ok {
			continue
		}
		if err := r.client.ScheduleClient().GetHandle(ctx, id).Delete(ctx); err != nil {
			return changes, fmt.Errorf("deleting schedule %s: %w", id, err)
		}
		changes = append(changes, Change{ID: id, Action: "deleted"})
	}

	return changes, nil
}

func (r *Reconciler) list(ctx context.Context) (map[string]struct{}, error) {
	iter, err := r.client.ScheduleClient().List(ctx, client.ScheduleListOptions{PageSize: 100})
	if err != nil {
		return nil, err
	}

	ids := map[string]struct{}{}
	for iter.HasNext() {
		entry, err := iter.Next()
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(entry.ID, IDPrefix) {
			ids[entry.ID] = struct{}{}
		}
	}

	return ids, nil
}

func (r *Reconciler) upsert(ctx context.Context, id string, account config.Account) error {
	targets, err := workflows.TargetsFor(account, r.registry)
	if err != nil {
		return fmt.Errorf("resolving targets for account %s: %w", account.Name, err)
	}

	spec := client.ScheduleSpec{
		Intervals: []client.ScheduleIntervalSpec{{Every: account.ScrapeInterval}},
	}
	action := &client.ScheduleWorkflowAction{
		ID:        "ame-scrape-" + account.Name,
		Workflow:  workflows.AccountScrapeName,
		TaskQueue: r.taskQueue,
		Args: []any{workflows.AccountScrapeInput{
			Account: account,
			Targets: targets,
		}},
	}

	_, err = r.client.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID:            id,
		Spec:          spec,
		Action:        action,
		Overlap:       enumspb.SCHEDULE_OVERLAP_POLICY_SKIP,
		CatchupWindow: catchupWindow,
	})
	if err == nil {
		return nil
	}

	if !errors.Is(err, temporal.ErrScheduleAlreadyRunning) {
		return fmt.Errorf("creating schedule %s: %w", id, err)
	}

	err = r.client.ScheduleClient().GetHandle(ctx, id).Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(in client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			in.Description.Schedule.Spec = &spec
			in.Description.Schedule.Action = action
			if in.Description.Schedule.Policy == nil {
				in.Description.Schedule.Policy = &client.SchedulePolicies{}
			}
			in.Description.Schedule.Policy.Overlap = enumspb.SCHEDULE_OVERLAP_POLICY_SKIP
			in.Description.Schedule.Policy.CatchupWindow = catchupWindow
			return &client.ScheduleUpdate{Schedule: &in.Description.Schedule}, nil
		},
	})
	if err != nil {
		return fmt.Errorf("updating schedule %s: %w", id, err)
	}

	return nil
}
