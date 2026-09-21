package elasticache

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

const ResourceTypeReplicationGroup = "aws_elasticache_replication_group"

// ReplicationGroupAPI is the subset of the ElastiCache client this collector uses.
type ReplicationGroupAPI interface {
	DescribeReplicationGroups(context.Context, *elasticache.DescribeReplicationGroupsInput, ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error)
	ListTagsForResource(context.Context, *elasticache.ListTagsForResourceInput, ...func(*elasticache.Options)) (*elasticache.ListTagsForResourceOutput, error)
}

type ReplicationGroup struct {
	types.ReplicationGroup
	Tags []types.Tag `json:"Tags"`
}

type ReplicationGroupCollector struct {
	newAPI func(aws.Config) ReplicationGroupAPI
}

func NewReplicationGroup() *ReplicationGroupCollector {
	return &ReplicationGroupCollector{newAPI: func(cfg aws.Config) ReplicationGroupAPI { return elasticache.NewFromConfig(cfg) }}
}

func (c *ReplicationGroupCollector) Type() string           { return ResourceTypeReplicationGroup }
func (c *ReplicationGroupCollector) Scope() collector.Scope { return collector.ScopeRegional }

func (c *ReplicationGroupCollector) Collect(ctx context.Context, cfg aws.Config, emit collector.EmitFunc) error {
	api := c.newAPI(cfg)

	pages := elasticache.NewDescribeReplicationGroupsPaginator(api, &elasticache.DescribeReplicationGroupsInput{})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("describing elasticache replication groups: %w", err)
		}

		for _, rg := range page.ReplicationGroups {
			arn := aws.ToString(rg.ARN)

			tags, err := api.ListTagsForResource(ctx, &elasticache.ListTagsForResourceInput{ResourceName: rg.ARN})
			if err != nil {
				return fmt.Errorf("listing tags of elasticache replication group %s: %w", arn, err)
			}

			if err := emit(collector.Resource{
				ID:   arn,
				Name: aws.ToString(rg.ReplicationGroupId),
				Data: ReplicationGroup{ReplicationGroup: rg, Tags: tags.TagList},
			}); err != nil {
				return err
			}
		}
	}

	return nil
}
