package ecs

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

const ResourceTypeTaskDefinition = "aws_ecs_task_definition"

// TaskDefinitionAPI is the subset of the ECS client this collector uses.
type TaskDefinitionAPI interface {
	ListTaskDefinitions(context.Context, *ecs.ListTaskDefinitionsInput, ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error)
	DescribeTaskDefinition(context.Context, *ecs.DescribeTaskDefinitionInput, ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
}

type TaskDefinition struct {
	types.TaskDefinition
	Tags []types.Tag `json:"Tags"`
}

type TaskDefinitionCollector struct {
	newAPI func(aws.Config) TaskDefinitionAPI
}

func NewTaskDefinition() *TaskDefinitionCollector {
	return &TaskDefinitionCollector{newAPI: func(cfg aws.Config) TaskDefinitionAPI { return ecs.NewFromConfig(cfg) }}
}

func (c *TaskDefinitionCollector) Type() string           { return ResourceTypeTaskDefinition }
func (c *TaskDefinitionCollector) Scope() collector.Scope { return collector.ScopeRegional }

func (c *TaskDefinitionCollector) Collect(ctx context.Context, cfg aws.Config, emit collector.EmitFunc) error {
	api := c.newAPI(cfg)

	pages := ecs.NewListTaskDefinitionsPaginator(api, &ecs.ListTaskDefinitionsInput{Status: types.TaskDefinitionStatusActive})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing ecs task definitions: %w", err)
		}

		for _, arn := range page.TaskDefinitionArns {
			out, err := api.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: aws.String(arn),
				Include:        []types.TaskDefinitionField{types.TaskDefinitionFieldTags},
			})
			if err != nil {
				return fmt.Errorf("describing ecs task definition %s: %w", arn, err)
			}

			td := out.TaskDefinition
			if err := emit(collector.Resource{
				ID:   arn,
				Name: fmt.Sprintf("%s:%d", aws.ToString(td.Family), td.Revision),
				Data: TaskDefinition{TaskDefinition: *td, Tags: out.Tags},
			}); err != nil {
				return err
			}
		}
	}

	return nil
}
