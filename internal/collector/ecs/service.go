package ecs

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

const ResourceTypeService = "aws_ecs_service"

// ServiceAPI is the subset of the ECS client this collector uses.
type ServiceAPI interface {
	ListClusters(context.Context, *ecs.ListClustersInput, ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	ListServices(context.Context, *ecs.ListServicesInput, ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	DescribeServices(context.Context, *ecs.DescribeServicesInput, ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
}

type ServiceCollector struct {
	newAPI func(aws.Config) ServiceAPI
}

func NewService() *ServiceCollector {
	return &ServiceCollector{newAPI: func(cfg aws.Config) ServiceAPI { return ecs.NewFromConfig(cfg) }}
}

func (c *ServiceCollector) Type() string           { return ResourceTypeService }
func (c *ServiceCollector) Scope() collector.Scope { return collector.ScopeRegional }

func (c *ServiceCollector) Collect(ctx context.Context, cfg aws.Config, emit collector.EmitFunc) error {
	api := c.newAPI(cfg)

	clusters, err := listClusterARNs(ctx, api)
	if err != nil {
		return err
	}

	for _, cluster := range clusters {
		// DescribeServices takes at most 10 services, which is also ListServices' default page size.
		pages := ecs.NewListServicesPaginator(api, &ecs.ListServicesInput{Cluster: aws.String(cluster)})
		for pages.HasMorePages() {
			page, err := pages.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("listing ecs services of cluster %s: %w", cluster, err)
			}
			if len(page.ServiceArns) == 0 {
				continue
			}

			out, err := api.DescribeServices(ctx, &ecs.DescribeServicesInput{
				Cluster:  aws.String(cluster),
				Services: page.ServiceArns,
				Include:  []types.ServiceField{types.ServiceFieldTags},
			})
			if err != nil {
				return fmt.Errorf("describing ecs services of cluster %s: %w", cluster, err)
			}

			for _, svc := range out.Services {
				if err := emit(collector.Resource{
					ID:   aws.ToString(svc.ServiceArn),
					Name: aws.ToString(svc.ServiceName),
					Data: svc,
				}); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
