package ecs

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

const ResourceTypeCluster = "aws_ecs_cluster"

// ClusterAPI is the subset of the ECS client this collector uses.
type ClusterAPI interface {
	ListClusters(context.Context, *ecs.ListClustersInput, ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	DescribeClusters(context.Context, *ecs.DescribeClustersInput, ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error)
}

type ClusterCollector struct {
	newAPI func(aws.Config) ClusterAPI
}

func NewCluster() *ClusterCollector {
	return &ClusterCollector{newAPI: func(cfg aws.Config) ClusterAPI { return ecs.NewFromConfig(cfg) }}
}

func (c *ClusterCollector) Type() string           { return ResourceTypeCluster }
func (c *ClusterCollector) Scope() collector.Scope { return collector.ScopeRegional }

func (c *ClusterCollector) Collect(ctx context.Context, cfg aws.Config, emit collector.EmitFunc) error {
	api := c.newAPI(cfg)

	pages := ecs.NewListClustersPaginator(api, &ecs.ListClustersInput{})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing ecs clusters: %w", err)
		}
		if len(page.ClusterArns) == 0 {
			continue
		}

		out, err := api.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: page.ClusterArns,
			Include: []types.ClusterField{
				types.ClusterFieldTags,
				types.ClusterFieldSettings,
				types.ClusterFieldConfigurations,
				types.ClusterFieldStatistics,
			},
		})
		if err != nil {
			return fmt.Errorf("describing ecs clusters: %w", err)
		}

		for _, cl := range out.Clusters {
			if err := emit(collector.Resource{
				ID:   aws.ToString(cl.ClusterArn),
				Name: aws.ToString(cl.ClusterName),
				Data: cl,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func listClusterARNs(ctx context.Context, api ecs.ListClustersAPIClient) ([]string, error) {
	var arns []string

	pages := ecs.NewListClustersPaginator(api, &ecs.ListClustersInput{})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing ecs clusters: %w", err)
		}
		arns = append(arns, page.ClusterArns...)
	}

	return arns, nil
}
