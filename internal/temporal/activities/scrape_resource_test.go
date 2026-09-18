package activities_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
	"github.com/windkube/aws-metrics-exporter/internal/config"
	"github.com/windkube/aws-metrics-exporter/internal/scraper"
	"github.com/windkube/aws-metrics-exporter/internal/storage"
	"github.com/windkube/aws-metrics-exporter/internal/temporal/activities"
)

type stubFactory struct{}

func (stubFactory) Config(config.Account, string) aws.Config { return aws.Config{} }

type fakeCollector struct {
	count int
	err   error
}

func (fakeCollector) Type() string           { return "fake" }
func (fakeCollector) Scope() collector.Scope { return collector.ScopeRegional }

func (c fakeCollector) Collect(_ context.Context, _ aws.Config, emit collector.EmitFunc) error {
	for i := range c.count {
		if err := emit(collector.Resource{ID: fmt.Sprintf("arn:%d", i), Data: map[string]int{"i": i}}); err != nil {
			return err
		}
	}
	return c.err
}

type nopStore struct{}

func (nopStore) Write(context.Context, []storage.Document) error { return nil }
func (nopStore) Ping(context.Context) error                      { return nil }
func (nopStore) Close(context.Context) error                     { return nil }

func newEnv(t *testing.T, c collector.Collector) *testsuite.TestActivityEnvironment {
	t.Helper()

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()

	acts := activities.New(scraper.New(stubFactory{}, collector.NewRegistry(c), nopStore{}, 2))
	env.RegisterActivityWithOptions(acts.ScrapeResource, sdkactivity.RegisterOptions{
		Name: activities.ScrapeResourceName,
	})

	return env
}

func input() activities.ScrapeInput {
	return activities.ScrapeInput{
		Account:      config.Account{Name: "prod", ID: "111111111111"},
		ResourceType: "fake",
		Region:       "eu-west-1",
		ScrapedAt:    time.Date(2026, 9, 18, 1, 0, 0, 0, time.UTC),
	}
}

func TestScrapeResourceHeartbeatsAfterEveryBatch(t *testing.T) {
	env := newEnv(t, fakeCollector{count: 5})

	var heartbeats []int
	env.SetOnActivityHeartbeatListener(func(_ *sdkactivity.Info, details converter.EncodedValues) {
		var count int
		require.NoError(t, details.Get(&count))
		heartbeats = append(heartbeats, count)
	})

	encoded, err := env.ExecuteActivity(activities.ScrapeResourceName, input())
	require.NoError(t, err)

	var out activities.ScrapeOutput
	require.NoError(t, encoded.Get(&out))
	assert.Equal(t, 5, out.Count)
	assert.Equal(t, 3, out.Batches)

	// The SDK throttles heartbeats, so only the first one is guaranteed to reach the listener.
	require.NotEmpty(t, heartbeats)
	assert.Equal(t, 2, heartbeats[0])
}

func TestScrapeResourceMakesPermissionFailuresNonRetryable(t *testing.T) {
	env := newEnv(t, fakeCollector{err: &smithy.GenericAPIError{
		Code:    "AccessDenied",
		Message: "not authorized to perform iam:ListUsers",
	}})

	_, err := env.ExecuteActivity(activities.ScrapeResourceName, input())
	require.Error(t, err)

	var appErr *temporal.ApplicationError
	require.ErrorAs(t, err, &appErr)
	assert.True(t, appErr.NonRetryable())
	assert.Equal(t, activities.ErrTypePermission, appErr.Type())
}

func TestScrapeResourceLeavesOtherFailuresRetryable(t *testing.T) {
	env := newEnv(t, fakeCollector{err: &smithy.GenericAPIError{Code: "Throttling", Message: "rate exceeded"}})

	_, err := env.ExecuteActivity(activities.ScrapeResourceName, input())
	require.Error(t, err)

	var appErr *temporal.ApplicationError
	require.ErrorAs(t, err, &appErr)
	assert.False(t, appErr.NonRetryable())
	assert.Equal(t, activities.ErrTypeAWS, appErr.Type())
}

// The AWS SDK nests wrapped errors deeply enough that Temporal rejects the serialised failure for
// exceeding the size limit, so the activity must return a flat one.
func TestScrapeResourceFlattensNestedAwsErrors(t *testing.T) {
	nested := fmt.Errorf("listing iam users: %w",
		fmt.Errorf("get identity: %w",
			fmt.Errorf("refresh credentials: %w",
				&smithy.GenericAPIError{Code: "Throttling", Message: "rate exceeded"})))

	env := newEnv(t, fakeCollector{err: nested})

	_, err := env.ExecuteActivity(activities.ScrapeResourceName, input())
	require.Error(t, err)

	var appErr *temporal.ApplicationError
	require.ErrorAs(t, err, &appErr)
	assert.Nil(t, appErr.Unwrap(), "the failure must not carry the SDK's wrap chain")
	assert.Contains(t, appErr.Error(), "listing iam users")
	assert.Contains(t, appErr.Error(), "rate exceeded")
}
