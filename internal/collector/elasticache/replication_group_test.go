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

type fakeElastiCache struct {
	pages    [][]types.ReplicationGroup
	nextPage int
	tagsErr  error
	tagsFor  []string
}

func (f *fakeElastiCache) DescribeReplicationGroups(context.Context, *elasticache.DescribeReplicationGroupsInput, ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error) {
	page := f.pages[f.nextPage]
	f.nextPage++

	out := &elasticache.DescribeReplicationGroupsOutput{ReplicationGroups: page}
	if f.nextPage < len(f.pages) {
		out.Marker = aws.String("next")
	}

	return out, nil
}

func (f *fakeElastiCache) ListTagsForResource(_ context.Context, in *elasticache.ListTagsForResourceInput, _ ...func(*elasticache.Options)) (*elasticache.ListTagsForResourceOutput, error) {
	if f.tagsErr != nil {
		return nil, f.tagsErr
	}
	f.tagsFor = append(f.tagsFor, aws.ToString(in.ResourceName))

	return &elasticache.ListTagsForResourceOutput{
		TagList: []types.Tag{{Key: aws.String("env"), Value: aws.String("prod")}},
	}, nil
}

func group(id string) types.ReplicationGroup {
	return types.ReplicationGroup{
		ReplicationGroupId: aws.String(id),
		ARN:                aws.String("arn:aws:elasticache:eu-west-1:111111111111:replicationgroup:" + id),
		Engine:             aws.String("redis"),
	}
}

func collectorWith(api ReplicationGroupAPI) *ReplicationGroupCollector {
	return &ReplicationGroupCollector{newAPI: func(aws.Config) ReplicationGroupAPI { return api }}
}

func TestReplicationGroupCollectWalksEveryPageAndAttachesTags(t *testing.T) {
	api := &fakeElastiCache{pages: [][]types.ReplicationGroup{{group("sessions")}, {group("cache")}}}

	var got []collector.Resource
	require.NoError(t, collectorWith(api).Collect(t.Context(), aws.Config{}, func(r collector.Resource) error {
		got = append(got, r)
		return nil
	}))

	require.Len(t, got, 2)
	assert.Equal(t, "sessions", got[0].Name)
	assert.Equal(t, "arn:aws:elasticache:eu-west-1:111111111111:replicationgroup:sessions", got[0].ID)
	assert.Equal(t, []string{got[0].ID, got[1].ID}, api.tagsFor)

	encoded, err := json.Marshal(got[0].Data)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, "redis", decoded["Engine"])
	assert.Len(t, decoded["Tags"], 1)
}

func TestReplicationGroupCollectStopsWhenTaggingFails(t *testing.T) {
	api := &fakeElastiCache{pages: [][]types.ReplicationGroup{{group("sessions")}}, tagsErr: errors.New("throttled")}

	err := collectorWith(api).Collect(t.Context(), aws.Config{}, func(collector.Resource) error { return nil })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing tags of elasticache replication group")
}

func TestReplicationGroupScopeIsRegional(t *testing.T) {
	assert.Equal(t, collector.ScopeRegional, NewReplicationGroup().Scope())
	assert.Equal(t, ResourceTypeReplicationGroup, NewReplicationGroup().Type())
}
