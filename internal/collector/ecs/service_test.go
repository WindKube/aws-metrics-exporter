package ecs

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

func serviceCollectorWith(api ServiceAPI) *ServiceCollector {
	return &ServiceCollector{newAPI: func(aws.Config) ServiceAPI { return api }}
}

func TestServiceCollectWalksEveryClusterAndPage(t *testing.T) {
	claims, payments := arnPrefix+"cluster/claims", arnPrefix+"cluster/payments"
	api := &fakeECS{
		clusterPages: [][]string{{claims}, {payments}},
		services: map[string][][]string{
			claims:   {{arnPrefix + "service/claims/api"}, {arnPrefix + "service/claims/worker"}},
			payments: {{arnPrefix + "service/payments/api"}},
		},
	}

	var got []collector.Resource
	require.NoError(t, serviceCollectorWith(api).Collect(t.Context(), aws.Config{}, func(r collector.Resource) error {
		got = append(got, r)
		return nil
	}))

	require.Len(t, got, 3)
	assert.Equal(t, "api", got[0].Name)
	assert.Equal(t, arnPrefix+"service/claims/api", got[0].ID)
	assert.Equal(t, "worker", got[1].Name)
	assert.Equal(t, arnPrefix+"service/payments/api", got[2].ID)
	assert.Equal(t, []string{"TAGS"}, api.describeInclude[0])

	decoded := decode(t, got[0].Data)
	assert.Equal(t, "FARGATE", decoded["LaunchType"])
	assert.Equal(t, claims, decoded["ClusterArn"])
	assert.Len(t, decoded["Tags"], 1)
}

func TestServiceCollectReturnsDescribeError(t *testing.T) {
	claims := arnPrefix + "cluster/claims"
	api := &fakeECS{
		clusterPages: [][]string{{claims}},
		services:     map[string][][]string{claims: {{arnPrefix + "service/claims/api"}}},
		describeErr:  errors.New("throttled"),
	}

	err := serviceCollectorWith(api).Collect(t.Context(), aws.Config{}, func(collector.Resource) error { return nil })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "describing ecs services of cluster "+claims)
}

func TestServiceScopeIsRegional(t *testing.T) {
	assert.Equal(t, collector.ScopeRegional, NewService().Scope())
	assert.Equal(t, ResourceTypeService, NewService().Type())
}
