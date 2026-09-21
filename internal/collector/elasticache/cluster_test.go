package elasticache

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

type fakeClusters struct {
	pages    [][]types.CacheCluster
	nextPage int
	nodeInfo []bool
	tagsErr  error
	tagsFor  []string
}

func (f *fakeClusters) DescribeCacheClusters(_ context.Context, in *elasticache.DescribeCacheClustersInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
	f.nodeInfo = append(f.nodeInfo, aws.ToBool(in.ShowCacheNodeInfo))

	page := f.pages[f.nextPage]
	f.nextPage++

	out := &elasticache.DescribeCacheClustersOutput{CacheClusters: page}
	if f.nextPage < len(f.pages) {
		out.Marker = aws.String("next")
	}

	return out, nil
}

func (f *fakeClusters) ListTagsForResource(_ context.Context, in *elasticache.ListTagsForResourceInput, _ ...func(*elasticache.Options)) (*elasticache.ListTagsForResourceOutput, error) {
	if f.tagsErr != nil {
		return nil, f.tagsErr
	}
	f.tagsFor = append(f.tagsFor, aws.ToString(in.ResourceName))

	return &elasticache.ListTagsForResourceOutput{
		TagList: []types.Tag{{Key: aws.String("env"), Value: aws.String("prod")}},
	}, nil
}

func cacheCluster(id string) types.CacheCluster {
	return types.CacheCluster{
		CacheClusterId: aws.String(id),
		ARN:            aws.String("arn:aws:elasticache:eu-west-1:111111111111:cluster:" + id),
		Engine:         aws.String("memcached"),
	}
}

func clusterCollectorWith(api ClusterAPI) *ClusterCollector {
	return &ClusterCollector{newAPI: func(aws.Config) ClusterAPI { return api }}
}

func TestClusterCollectWalksEveryPageAndAttachesTags(t *testing.T) {
	api := &fakeClusters{pages: [][]types.CacheCluster{{cacheCluster("sessions")}, {cacheCluster("cache")}}}

	var got []collector.Resource
	require.NoError(t, clusterCollectorWith(api).Collect(t.Context(), aws.Config{}, func(r collector.Resource) error {
		got = append(got, r)
		return nil
	}))

	require.Len(t, got, 2)
	assert.Equal(t, "sessions", got[0].Name)
	assert.Equal(t, "arn:aws:elasticache:eu-west-1:111111111111:cluster:sessions", got[0].ID)
	assert.Equal(t, []string{got[0].ID, got[1].ID}, api.tagsFor)
	assert.Equal(t, []bool{true, true}, api.nodeInfo)

	encoded, err := json.Marshal(got[0].Data)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, "memcached", decoded["Engine"])
	assert.Len(t, decoded["Tags"], 1)
}

func TestClusterCollectStopsWhenTaggingFails(t *testing.T) {
	api := &fakeClusters{pages: [][]types.CacheCluster{{cacheCluster("sessions")}}, tagsErr: errors.New("throttled")}

	err := clusterCollectorWith(api).Collect(t.Context(), aws.Config{}, func(collector.Resource) error { return nil })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing tags of elasticache cluster")
}

func TestClusterScopeIsRegional(t *testing.T) {
	assert.Equal(t, collector.ScopeRegional, NewCluster().Scope())
	assert.Equal(t, ResourceTypeCluster, NewCluster().Type())
}
