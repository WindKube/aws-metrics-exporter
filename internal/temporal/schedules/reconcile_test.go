package schedules_test

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
	"go.temporal.io/sdk/temporal"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
	"github.com/windkube/aws-metrics-exporter/internal/config"
	"github.com/windkube/aws-metrics-exporter/internal/temporal/schedules"
	"github.com/windkube/aws-metrics-exporter/internal/temporal/workflows"
)

type fakeCollector struct {
	resourceType string
	scope        collector.Scope
}

func (c fakeCollector) Type() string           { return c.resourceType }
func (c fakeCollector) Scope() collector.Scope { return c.scope }
func (fakeCollector) Collect(context.Context, aws.Config, collector.EmitFunc) error {
	return nil
}

func registry() *collector.Registry {
	return collector.NewRegistry(
		fakeCollector{resourceType: "aws_iam_user", scope: collector.ScopeGlobal},
		fakeCollector{resourceType: "aws_elasticache_replication_group", scope: collector.ScopeRegional},
	)
}

func configWith(accounts ...config.Account) *config.Config {
	return &config.Config{Accounts: accounts}
}

func account(name string) config.Account {
	return config.Account{
		Name:           name,
		ID:             "111111111111",
		RoleARN:        "arn:aws:iam::111111111111:role/aws-metrics-exporter",
		Regions:        []string{"eu-west-1"},
		Resources:      []string{"aws_iam_user", "aws_elasticache_replication_group"},
		ScrapeInterval: time.Hour,
	}
}

type listIterator struct{ ids []string }

func (i *listIterator) HasNext() bool { return len(i.ids) > 0 }

func (i *listIterator) Next() (*client.ScheduleListEntry, error) {
	entry := &client.ScheduleListEntry{ID: i.ids[0]}
	i.ids = i.ids[1:]
	return entry, nil
}

func newClient(t *testing.T, existing ...string) (*mocks.Client, *mocks.ScheduleClient) {
	t.Helper()

	scheduleClient := mocks.NewScheduleClient(t)
	scheduleClient.On("List", mock.Anything, mock.Anything).
		Return(&listIterator{ids: existing}, nil).Once()

	c := mocks.NewClient(t)
	c.On("ScheduleClient").Return(scheduleClient)

	return c, scheduleClient
}

func TestApplyCreatesASchedulePerAccount(t *testing.T) {
	c, scheduleClient := newClient(t)

	var created []client.ScheduleOptions
	scheduleClient.On("Create", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			created = append(created, args.Get(1).(client.ScheduleOptions))
		}).
		Return(mocks.NewScheduleHandle(t), nil).Twice()

	changes, err := schedules.NewReconciler(c, registry(), "aws-metrics-exporter").
		Apply(t.Context(), configWith(account("prod"), account("sandbox")))
	require.NoError(t, err)

	assert.ElementsMatch(t, []schedules.Change{
		{ID: "ame-acct-prod", Action: "created"},
		{ID: "ame-acct-sandbox", Action: "created"},
	}, changes)

	require.Len(t, created, 2)
	opts := created[0]
	assert.Equal(t, []client.ScheduleIntervalSpec{{Every: time.Hour}}, opts.Spec.Intervals)

	action := opts.Action.(*client.ScheduleWorkflowAction)
	assert.Equal(t, workflows.AccountScrapeName, action.Workflow)
	assert.Equal(t, "aws-metrics-exporter", action.TaskQueue)

	input := action.Args[0].(workflows.AccountScrapeInput)
	assert.Equal(t, []workflows.Target{
		{ResourceType: "aws_iam_user", Region: collector.GlobalRegion},
		{ResourceType: "aws_elasticache_replication_group", Region: "eu-west-1"},
	}, input.Targets)
}

// A worker restart re-runs Apply against schedules that already exist; it must update them rather
// than fail.
func TestApplyUpdatesAScheduleThatAlreadyExists(t *testing.T) {
	c, scheduleClient := newClient(t, "ame-acct-prod")

	scheduleClient.On("Create", mock.Anything, mock.Anything).
		Return(nil, temporal.ErrScheduleAlreadyRunning).Once()

	handle := mocks.NewScheduleHandle(t)
	handle.On("Update", mock.Anything, mock.Anything).Return(nil).Once()
	scheduleClient.On("GetHandle", mock.Anything, "ame-acct-prod").Return(handle).Once()

	changes, err := schedules.NewReconciler(c, registry(), "aws-metrics-exporter").
		Apply(t.Context(), configWith(account("prod")))
	require.NoError(t, err)

	assert.Equal(t, []schedules.Change{{ID: "ame-acct-prod", Action: "updated"}}, changes)
}

func TestApplyDeletesSchedulesForRemovedAccounts(t *testing.T) {
	c, scheduleClient := newClient(t, "ame-acct-prod", "ame-acct-gone")

	scheduleClient.On("Create", mock.Anything, mock.Anything).
		Return(nil, temporal.ErrScheduleAlreadyRunning).Once()

	prod := mocks.NewScheduleHandle(t)
	prod.On("Update", mock.Anything, mock.Anything).Return(nil).Once()
	scheduleClient.On("GetHandle", mock.Anything, "ame-acct-prod").Return(prod).Once()

	gone := mocks.NewScheduleHandle(t)
	gone.On("Delete", mock.Anything).Return(nil).Once()
	scheduleClient.On("GetHandle", mock.Anything, "ame-acct-gone").Return(gone).Once()

	changes, err := schedules.NewReconciler(c, registry(), "aws-metrics-exporter").
		Apply(t.Context(), configWith(account("prod")))
	require.NoError(t, err)

	assert.ElementsMatch(t, []schedules.Change{
		{ID: "ame-acct-prod", Action: "updated"},
		{ID: "ame-acct-gone", Action: "deleted"},
	}, changes)
}

func TestApplyLeavesForeignSchedulesAlone(t *testing.T) {
	c, scheduleClient := newClient(t, "some-other-teams-schedule")

	scheduleClient.On("Create", mock.Anything, mock.Anything).
		Return(mocks.NewScheduleHandle(t), nil).Once()

	changes, err := schedules.NewReconciler(c, registry(), "aws-metrics-exporter").
		Apply(t.Context(), configWith(account("prod")))
	require.NoError(t, err)

	assert.Equal(t, []schedules.Change{{ID: "ame-acct-prod", Action: "created"}}, changes)
}
