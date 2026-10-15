package ecs

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

func taskDefinitionCollectorWith(api TaskDefinitionAPI) *TaskDefinitionCollector {
	return &TaskDefinitionCollector{newAPI: func(aws.Config) TaskDefinitionAPI { return api }}
}

func TestTaskDefinitionCollectWalksEveryPageAndAttachesTags(t *testing.T) {
	api := &fakeECS{taskDefPages: [][]string{
		{arnPrefix + "task-definition/claims-api:7"},
		{arnPrefix + "task-definition/claims-api:8"},
	}}

	var got []collector.Resource
	require.NoError(t, taskDefinitionCollectorWith(api).Collect(t.Context(), aws.Config{}, func(r collector.Resource) error {
		got = append(got, r)
		return nil
	}))

	require.Len(t, got, 2)
	assert.Equal(t, arnPrefix+"task-definition/claims-api:7", got[0].ID)
	assert.Equal(t, "claims-api:7", got[0].Name)
	assert.Equal(t, arnPrefix+"task-definition/claims-api:8", got[1].ID)
	assert.Equal(t, []string{"TAGS"}, api.describeInclude[0])

	decoded := decode(t, got[0].Data)
	assert.Equal(t, "awsvpc", decoded["NetworkMode"])
	assert.Len(t, decoded["Tags"], 1)
}

func TestTaskDefinitionCollectReturnsDescribeError(t *testing.T) {
	api := &fakeECS{
		taskDefPages: [][]string{{arnPrefix + "task-definition/claims-api:7"}},
		describeErr:  errors.New("throttled"),
	}

	err := taskDefinitionCollectorWith(api).Collect(t.Context(), aws.Config{}, func(collector.Resource) error { return nil })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "describing ecs task definition")
}

func TestTaskDefinitionScopeIsRegional(t *testing.T) {
	assert.Equal(t, collector.ScopeRegional, NewTaskDefinition().Scope())
	assert.Equal(t, ResourceTypeTaskDefinition, NewTaskDefinition().Type())
}
