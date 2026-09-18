package workflows_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	sdkworkflow "go.temporal.io/sdk/workflow"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
	"github.com/windkube/aws-metrics-exporter/internal/config"
	"github.com/windkube/aws-metrics-exporter/internal/temporal/activities"
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

func account() config.Account {
	return config.Account{
		Name:      "prod",
		ID:        "111111111111",
		Regions:   []string{"eu-west-1", "eu-central-1"},
		Resources: []string{"aws_iam_user", "aws_elasticache_replication_group"},
	}
}

func TestTargetsForScrapesGlobalTypesOnceAndRegionalOnesPerRegion(t *testing.T) {
	targets, err := workflows.TargetsFor(account(), registry())
	require.NoError(t, err)

	assert.Equal(t, []workflows.Target{
		{ResourceType: "aws_iam_user", Region: collector.GlobalRegion},
		{ResourceType: "aws_elasticache_replication_group", Region: "eu-west-1"},
		{ResourceType: "aws_elasticache_replication_group", Region: "eu-central-1"},
	}, targets)
}

func TestTargetsForKeepsGlobalTypesWhenNoRegionIsConfigured(t *testing.T) {
	acc := account()
	acc.Regions = nil

	targets, err := workflows.TargetsFor(acc, registry())
	require.NoError(t, err)

	assert.Equal(t, []workflows.Target{{ResourceType: "aws_iam_user", Region: collector.GlobalRegion}}, targets)
}

func TestTargetsForRejectsAnUnknownResourceType(t *testing.T) {
	acc := account()
	acc.Resources = []string{"aws_unicorn"}

	_, err := workflows.TargetsFor(acc, registry())

	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown resource type "aws_unicorn"`)
}

func newEnv(t *testing.T) *testsuite.TestWorkflowEnvironment {
	t.Helper()

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(workflows.AccountScrape, sdkworkflow.RegisterOptions{
		Name: workflows.AccountScrapeName,
	})
	env.RegisterActivityWithOptions(
		func(context.Context, activities.ScrapeInput) (activities.ScrapeOutput, error) {
			return activities.ScrapeOutput{}, nil
		},
		sdkactivity.RegisterOptions{Name: activities.ScrapeResourceName},
	)

	return env
}

func input(t *testing.T) workflows.AccountScrapeInput {
	t.Helper()

	targets, err := workflows.TargetsFor(account(), registry())
	require.NoError(t, err)

	return workflows.AccountScrapeInput{Account: account(), Targets: targets}
}

func TestAccountScrapeRunsOneActivityPerTarget(t *testing.T) {
	env := newEnv(t)
	env.OnActivity(activities.ScrapeResourceName, mock.Anything, mock.Anything).
		Return(activities.ScrapeOutput{Count: 7}, nil).Times(3)

	env.ExecuteWorkflow(workflows.AccountScrapeName, input(t))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result workflows.AccountScrapeResult
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, "prod", result.Account)
	assert.Equal(t, 21, result.Count)
	assert.Len(t, result.Targets, 3)
	env.AssertExpectations(t)
}

func TestAccountScrapeSucceedsWhenOnlySomeTargetsFail(t *testing.T) {
	env := newEnv(t)
	env.OnActivity(activities.ScrapeResourceName, mock.Anything, mock.MatchedBy(
		func(in activities.ScrapeInput) bool { return in.Region == "eu-central-1" },
	)).Return(activities.ScrapeOutput{}, errors.New("region unreachable"))
	env.OnActivity(activities.ScrapeResourceName, mock.Anything, mock.Anything).
		Return(activities.ScrapeOutput{Count: 4}, nil)

	env.ExecuteWorkflow(workflows.AccountScrapeName, input(t))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result workflows.AccountScrapeResult
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 8, result.Count)

	failures := 0
	for _, target := range result.Targets {
		if target.Error != "" {
			failures++
			assert.Equal(t, "eu-central-1", target.Target.Region)
		}
	}
	assert.Equal(t, 1, failures)
}

func TestAccountScrapeFailsWhenEveryTargetFails(t *testing.T) {
	env := newEnv(t)
	env.OnActivity(activities.ScrapeResourceName, mock.Anything, mock.Anything).
		Return(activities.ScrapeOutput{}, errors.New("role not assumable"))

	env.ExecuteWorkflow(workflows.AccountScrapeName, input(t))

	require.True(t, env.IsWorkflowCompleted())

	err := env.GetWorkflowError()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all 3 targets failed for account prod")
}
