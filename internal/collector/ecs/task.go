package ecs

import (
	"context"
	"fmt"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

const ResourceTypeTask = "aws_ecs_task"

// TaskAPI is the subset of the ECS client this collector uses.
type TaskAPI interface {
	ListClusters(context.Context, *ecs.ListClustersInput, ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	ListTasks(context.Context, *ecs.ListTasksInput, ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	DescribeTasks(context.Context, *ecs.DescribeTasksInput, ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
}

type TaskCollector struct {
	newAPI func(aws.Config) TaskAPI
}

func NewTask() *TaskCollector {
	return &TaskCollector{newAPI: func(cfg aws.Config) TaskAPI { return ecs.NewFromConfig(cfg) }}
}

func (c *TaskCollector) Type() string           { return ResourceTypeTask }
func (c *TaskCollector) Scope() collector.Scope { return collector.ScopeRegional }

func (c *TaskCollector) Collect(ctx context.Context, cfg aws.Config, emit collector.EmitFunc) error {
	api := c.newAPI(cfg)

	clusters, err := listClusterARNs(ctx, api)
	if err != nil {
		return err
	}

	for _, cluster := range clusters {
		pages := ecs.NewListTasksPaginator(api, &ecs.ListTasksInput{
			Cluster:       aws.String(cluster),
			DesiredStatus: types.DesiredStatusRunning,
		})
		for pages.HasMorePages() {
			page, err := pages.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("listing ecs tasks of cluster %s: %w", cluster, err)
			}
			if len(page.TaskArns) == 0 {
				continue
			}

			out, err := api.DescribeTasks(ctx, &ecs.DescribeTasksInput{
				Cluster: aws.String(cluster),
				Tasks:   page.TaskArns,
				Include: []types.TaskField{types.TaskFieldTags},
			})
			if err != nil {
				return fmt.Errorf("describing ecs tasks of cluster %s: %w", cluster, err)
			}

			for _, task := range out.Tasks {
				arn := aws.ToString(task.TaskArn)
				if err := emit(collector.Resource{
					ID:   arn,
					Name: path.Base(arn),
					Data: task,
				}); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
