package rds

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

type fakeClusters struct {
	pages    [][]types.DBCluster
	nextPage int
	err      error
}

func (f *fakeClusters) DescribeDBClusters(context.Context, *rds.DescribeDBClustersInput, ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	if f.err != nil {
		return nil, f.err
	}

	page := f.pages[f.nextPage]
	f.nextPage++

	out := &rds.DescribeDBClustersOutput{DBClusters: page}
	if f.nextPage < len(f.pages) {
		out.Marker = aws.String("next")
	}

	return out, nil
}

func dbCluster(id string) types.DBCluster {
	return types.DBCluster{
		DBClusterIdentifier: aws.String(id),
		DBClusterArn:        aws.String("arn:aws:rds:eu-west-1:111111111111:cluster:" + id),
		Engine:              aws.String("aurora-postgresql"),
		TagList:             []types.Tag{{Key: aws.String("env"), Value: aws.String("prod")}},
	}
}

func clusterCollectorWith(api ClusterAPI) *ClusterCollector {
	return &ClusterCollector{newAPI: func(aws.Config) ClusterAPI { return api }}
}

func TestClusterCollectWalksEveryPage(t *testing.T) {
	api := &fakeClusters{pages: [][]types.DBCluster{{dbCluster("claims")}, {dbCluster("payments")}}}

	var got []collector.Resource
	require.NoError(t, clusterCollectorWith(api).Collect(t.Context(), aws.Config{}, func(r collector.Resource) error {
		got = append(got, r)
		return nil
	}))

	require.Len(t, got, 2)
	assert.Equal(t, "claims", got[0].Name)
	assert.Equal(t, "arn:aws:rds:eu-west-1:111111111111:cluster:claims", got[0].ID)
	assert.Equal(t, "payments", got[1].Name)

	encoded, err := json.Marshal(got[0].Data)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, "aurora-postgresql", decoded["Engine"])
	assert.Len(t, decoded["TagList"], 1)
}

func TestClusterCollectReturnsDescribeError(t *testing.T) {
	api := &fakeClusters{err: errors.New("throttled")}

	err := clusterCollectorWith(api).Collect(t.Context(), aws.Config{}, func(collector.Resource) error { return nil })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "describing rds clusters")
}

func TestClusterScopeIsRegional(t *testing.T) {
	assert.Equal(t, collector.ScopeRegional, NewCluster().Scope())
	assert.Equal(t, ResourceTypeCluster, NewCluster().Type())
}
