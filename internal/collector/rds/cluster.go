package rds

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

const ResourceTypeCluster = "aws_rds_cluster"

// ClusterAPI is the subset of the RDS client this collector uses.
type ClusterAPI interface {
	DescribeDBClusters(context.Context, *rds.DescribeDBClustersInput, ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error)
}

type ClusterCollector struct {
	newAPI func(aws.Config) ClusterAPI
}

func NewCluster() *ClusterCollector {
	return &ClusterCollector{newAPI: func(cfg aws.Config) ClusterAPI { return rds.NewFromConfig(cfg) }}
}

func (c *ClusterCollector) Type() string           { return ResourceTypeCluster }
func (c *ClusterCollector) Scope() collector.Scope { return collector.ScopeRegional }

func (c *ClusterCollector) Collect(ctx context.Context, cfg aws.Config, emit collector.EmitFunc) error {
	pages := rds.NewDescribeDBClustersPaginator(c.newAPI(cfg), &rds.DescribeDBClustersInput{})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("describing rds clusters: %w", err)
		}

		for _, dc := range page.DBClusters {
			if err := emit(collector.Resource{
				ID:   aws.ToString(dc.DBClusterArn),
				Name: aws.ToString(dc.DBClusterIdentifier),
				Data: dc,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}
