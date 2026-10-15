package ecs

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

func clusterCollectorWith(api ClusterAPI) *ClusterCollector {
	return &ClusterCollector{newAPI: func(aws.Config) ClusterAPI { return api }}
}

func TestClusterCollectWalksEveryPage(t *testing.T) {
	api := &fakeECS{clusterPages: [][]string{{arnPrefix + "cluster/claims"}, {arnPrefix + "cluster/payments"}}}

	var got []collector.Resource
	require.NoError(t, clusterCollectorWith(api).Collect(t.Context(), aws.Config{}, func(r collector.Resource) error {
		got = append(got, r)
		return nil
	}))

	require.Len(t, got, 2)
	assert.Equal(t, "claims", got[0].Name)
	assert.Equal(t, arnPrefix+"cluster/claims", got[0].ID)
	assert.Equal(t, "payments", got[1].Name)
	assert.Contains(t, api.describeInclude[0], "TAGS")

	decoded := decode(t, got[0].Data)
	assert.Equal(t, "ACTIVE", decoded["Status"])
	assert.Len(t, decoded["Tags"], 1)
}

func TestClusterCollectReturnsDescribeError(t *testing.T) {
	api := &fakeECS{clusterPages: [][]string{{arnPrefix + "cluster/claims"}}, describeErr: errors.New("throttled")}

	err := clusterCollectorWith(api).Collect(t.Context(), aws.Config{}, func(collector.Resource) error { return nil })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "describing ecs clusters")
}

func TestClusterScopeIsRegional(t *testing.T) {
	assert.Equal(t, collector.ScopeRegional, NewCluster().Scope())
	assert.Equal(t, ResourceTypeCluster, NewCluster().Type())
}
