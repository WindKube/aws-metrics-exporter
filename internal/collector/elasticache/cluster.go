package elasticache

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

const ResourceTypeCluster = "aws_elasticache_cluster"

// ClusterAPI is the subset of the ElastiCache client this collector uses.
type ClusterAPI interface {
	DescribeCacheClusters(context.Context, *elasticache.DescribeCacheClustersInput, ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error)
	ListTagsForResource(context.Context, *elasticache.ListTagsForResourceInput, ...func(*elasticache.Options)) (*elasticache.ListTagsForResourceOutput, error)
}

type Cluster struct {
	types.CacheCluster
	Tags []types.Tag `json:"Tags"`
}

type ClusterCollector struct {
	newAPI func(aws.Config) ClusterAPI
}

func NewCluster() *ClusterCollector {
	return &ClusterCollector{newAPI: func(cfg aws.Config) ClusterAPI { return elasticache.NewFromConfig(cfg) }}
}

func (c *ClusterCollector) Type() string           { return ResourceTypeCluster }
func (c *ClusterCollector) Scope() collector.Scope { return collector.ScopeRegional }

func (c *ClusterCollector) Collect(ctx context.Context, cfg aws.Config, emit collector.EmitFunc) error {
	api := c.newAPI(cfg)

	// Without ShowCacheNodeInfo the per-node endpoints and availability zones are left out, which
	// is the bulk of what a Memcached cluster is.
	input := &elasticache.DescribeCacheClustersInput{ShowCacheNodeInfo: aws.Bool(true)}

	pages := elasticache.NewDescribeCacheClustersPaginator(api, input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("describing elasticache clusters: %w", err)
		}

		for _, cc := range page.CacheClusters {
			arn := aws.ToString(cc.ARN)

			tags, err := api.ListTagsForResource(ctx, &elasticache.ListTagsForResourceInput{ResourceName: cc.ARN})
			if err != nil {
				return fmt.Errorf("listing tags of elasticache cluster %s: %w", arn, err)
			}

			if err := emit(collector.Resource{
				ID:   arn,
				Name: aws.ToString(cc.CacheClusterId),
				Data: Cluster{CacheCluster: cc, Tags: tags.TagList},
			}); err != nil {
				return err
			}
		}
	}

	return nil
}
