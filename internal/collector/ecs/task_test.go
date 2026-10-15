package ecs

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

func taskCollectorWith(api TaskAPI) *TaskCollector {
	return &TaskCollector{newAPI: func(aws.Config) TaskAPI { return api }}
}

func TestTaskCollectListsRunningTasksOfEveryCluster(t *testing.T) {
	claims, payments := arnPrefix+"cluster/claims", arnPrefix+"cluster/payments"
	api := &fakeECS{
		clusterPages: [][]string{{claims, payments}},
		tasks: map[string][][]string{
			claims:   {{arnPrefix + "task/claims/aaa"}, {arnPrefix + "task/claims/bbb"}},
			payments: {{arnPrefix + "task/payments/ccc"}},
		},
	}

	var got []collector.Resource
	require.NoError(t, taskCollectorWith(api).Collect(t.Context(), aws.Config{}, func(r collector.Resource) error {
		got = append(got, r)
		return nil
	}))

	require.Len(t, got, 3)
	assert.Equal(t, "aaa", got[0].Name)
	assert.Equal(t, arnPrefix+"task/claims/aaa", got[0].ID)
	assert.Equal(t, "ccc", got[2].Name)
	assert.Equal(t, []types.DesiredStatus{
		types.DesiredStatusRunning, types.DesiredStatusRunning, types.DesiredStatusRunning,
	}, api.listedStatuses)
	assert.Equal(t, []string{"TAGS"}, api.describeInclude[0])

	decoded := decode(t, got[0].Data)
	assert.Equal(t, "RUNNING", decoded["LastStatus"])
	assert.Len(t, decoded["Tags"], 1)
}

func TestTaskCollectReturnsDescribeError(t *testing.T) {
	claims := arnPrefix + "cluster/claims"
	api := &fakeECS{
		clusterPages: [][]string{{claims}},
		tasks:        map[string][][]string{claims: {{arnPrefix + "task/claims/aaa"}}},
		describeErr:  errors.New("throttled"),
	}

	err := taskCollectorWith(api).Collect(t.Context(), aws.Config{}, func(collector.Resource) error { return nil })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "describing ecs tasks of cluster "+claims)
}

func TestTaskScopeIsRegional(t *testing.T) {
	assert.Equal(t, collector.ScopeRegional, NewTask().Scope())
	assert.Equal(t, ResourceTypeTask, NewTask().Type())
}
