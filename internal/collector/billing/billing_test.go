package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

type fakeCostExplorer struct {
	pages    [][]string
	nextPage int
	inputs   []*costexplorer.GetCostAndUsageInput
	err      error
}

func (f *fakeCostExplorer) GetCostAndUsage(_ context.Context, in *costexplorer.GetCostAndUsageInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error) {
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return nil, f.err
	}

	result := types.ResultByTime{TimePeriod: in.TimePeriod, Estimated: true}

	if len(in.GroupBy) == 0 {
		result.Total = map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("42.5"), Unit: aws.String("USD")}}
		return &costexplorer.GetCostAndUsageOutput{ResultsByTime: []types.ResultByTime{result}}, nil
	}

	for _, service := range f.pages[f.nextPage] {
		result.Groups = append(result.Groups, types.Group{
			Keys:    []string{service},
			Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("1"), Unit: aws.String("USD")}},
		})
	}
	f.nextPage++

	out := &costexplorer.GetCostAndUsageOutput{ResultsByTime: []types.ResultByTime{result}}
	if f.nextPage < len(f.pages) {
		out.NextPageToken = aws.String("next")
	}

	return out, nil
}

func collectorWith(api API, now time.Time) (*Collector, *aws.Config) {
	var seen aws.Config
	return &Collector{
		newAPI: func(cfg aws.Config) API { seen = cfg; return api },
		now:    func() time.Time { return now },
	}, &seen
}

func collect(t *testing.T, c *Collector) []collector.Resource {
	t.Helper()

	var got []collector.Resource
	require.NoError(t, c.Collect(t.Context(), aws.Config{Region: "eu-west-1"}, func(r collector.Resource) error {
		got = append(got, r)
		return nil
	}))

	return got
}

func TestCollectEmitsEveryServiceAndTheAccountTotal(t *testing.T) {
	api := &fakeCostExplorer{pages: [][]string{{"Amazon EC2"}, {"Amazon RDS"}}}
	c, cfg := collectorWith(api, time.Date(2026, 10, 15, 13, 0, 0, 0, time.UTC))

	got := collect(t, c)

	require.Len(t, got, 3)
	assert.Equal(t, "2026-10:service:Amazon EC2", got[0].ID)
	assert.Equal(t, "Amazon EC2", got[0].Name)
	assert.Equal(t, "2026-10:service:Amazon RDS", got[1].ID)
	assert.Equal(t, "2026-10:total", got[2].ID)
	assert.Equal(t, "total", got[2].Name)
	assert.Equal(t, "42.5", aws.ToString(got[2].Data.(Cost).Metrics["UnblendedCost"].Amount))
	assert.True(t, got[2].Data.(Cost).Estimated)

	require.Len(t, api.inputs, 3)
	assert.Equal(t, aws.String("next"), api.inputs[1].NextPageToken)
	assert.Equal(t, "us-east-1", cfg.Region)
}

func TestCollectQueriesMonthToDate(t *testing.T) {
	for name, tc := range map[string]struct {
		now        time.Time
		start, end string
	}{
		"mid month":        {time.Date(2026, 10, 15, 13, 0, 0, 0, time.UTC), "2026-10-01", "2026-10-16"},
		"first of month":   {time.Date(2026, 10, 1, 0, 30, 0, 0, time.UTC), "2026-10-01", "2026-10-02"},
		"last day of year": {time.Date(2026, 12, 31, 23, 0, 0, 0, time.UTC), "2026-12-01", "2027-01-01"},
	} {
		t.Run(name, func(t *testing.T) {
			api := &fakeCostExplorer{pages: [][]string{{}}}
			c, _ := collectorWith(api, tc.now)

			collect(t, c)

			for _, in := range api.inputs {
				assert.Equal(t, tc.start, aws.ToString(in.TimePeriod.Start))
				assert.Equal(t, tc.end, aws.ToString(in.TimePeriod.End))
				assert.Equal(t, types.GranularityMonthly, in.Granularity)
			}
		})
	}
}

func TestCollectReturnsCostExplorerError(t *testing.T) {
	c, _ := collectorWith(&fakeCostExplorer{err: errors.New("throttled")}, time.Now())

	err := c.Collect(t.Context(), aws.Config{}, func(collector.Resource) error { return nil })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting cost and usage")
}

func TestScopeIsGlobal(t *testing.T) {
	assert.Equal(t, collector.ScopeGlobal, New().Scope())
	assert.Equal(t, ResourceType, New().Type())
}
