package rds

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

const ResourceTypeInstance = "aws_db_instance"

// InstanceAPI is the subset of the RDS client this collector uses.
type InstanceAPI interface {
	DescribeDBInstances(context.Context, *rds.DescribeDBInstancesInput, ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
}

type InstanceCollector struct {
	newAPI func(aws.Config) InstanceAPI
}

func NewInstance() *InstanceCollector {
	return &InstanceCollector{newAPI: func(cfg aws.Config) InstanceAPI { return rds.NewFromConfig(cfg) }}
}

func (c *InstanceCollector) Type() string           { return ResourceTypeInstance }
func (c *InstanceCollector) Scope() collector.Scope { return collector.ScopeRegional }

func (c *InstanceCollector) Collect(ctx context.Context, cfg aws.Config, emit collector.EmitFunc) error {
	pages := rds.NewDescribeDBInstancesPaginator(c.newAPI(cfg), &rds.DescribeDBInstancesInput{})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("describing rds instances: %w", err)
		}

		for _, db := range page.DBInstances {
			if err := emit(collector.Resource{
				ID:   aws.ToString(db.DBInstanceArn),
				Name: aws.ToString(db.DBInstanceIdentifier),
				Data: db,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}
