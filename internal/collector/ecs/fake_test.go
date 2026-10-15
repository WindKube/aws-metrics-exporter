package ecs

import (
	"context"
	"encoding/json"
	"path"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/require"
)

const arnPrefix = "arn:aws:ecs:eu-west-1:111111111111:"

var prodTag = []types.Tag{{Key: aws.String("env"), Value: aws.String("prod")}}

// fakeECS serves one page per entry, so a slice of two pages exercises pagination.
type fakeECS struct {
	clusterPages [][]string
	services     map[string][][]string
	tasks        map[string][][]string
	taskDefPages [][]string

	describeErr     error
	listedStatuses  []types.DesiredStatus
	describeInclude [][]string
}

func (f *fakeECS) ListClusters(_ context.Context, in *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	page, next := pageOf(f.clusterPages, in.NextToken)
	return &ecs.ListClustersOutput{ClusterArns: page, NextToken: next}, nil
}

func (f *fakeECS) DescribeClusters(_ context.Context, in *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	if f.describeErr != nil {
		return nil, f.describeErr
	}
	f.describeInclude = append(f.describeInclude, stringsOf(in.Include))

	out := &ecs.DescribeClustersOutput{}
	for _, arn := range in.Clusters {
		out.Clusters = append(out.Clusters, types.Cluster{
			ClusterArn:  aws.String(arn),
			ClusterName: aws.String(path.Base(arn)),
			Status:      aws.String("ACTIVE"),
			Tags:        prodTag,
		})
	}
	return out, nil
}

func (f *fakeECS) ListServices(_ context.Context, in *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	page, next := pageOf(f.services[aws.ToString(in.Cluster)], in.NextToken)
	return &ecs.ListServicesOutput{ServiceArns: page, NextToken: next}, nil
}

func (f *fakeECS) DescribeServices(_ context.Context, in *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	if f.describeErr != nil {
		return nil, f.describeErr
	}
	f.describeInclude = append(f.describeInclude, stringsOf(in.Include))

	out := &ecs.DescribeServicesOutput{}
	for _, arn := range in.Services {
		out.Services = append(out.Services, types.Service{
			ServiceArn:  aws.String(arn),
			ServiceName: aws.String(path.Base(arn)),
			ClusterArn:  in.Cluster,
			LaunchType:  types.LaunchTypeFargate,
			Tags:        prodTag,
		})
	}
	return out, nil
}

func (f *fakeECS) ListTasks(_ context.Context, in *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	f.listedStatuses = append(f.listedStatuses, in.DesiredStatus)
	page, next := pageOf(f.tasks[aws.ToString(in.Cluster)], in.NextToken)
	return &ecs.ListTasksOutput{TaskArns: page, NextToken: next}, nil
}

func (f *fakeECS) DescribeTasks(_ context.Context, in *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	if f.describeErr != nil {
		return nil, f.describeErr
	}
	f.describeInclude = append(f.describeInclude, stringsOf(in.Include))

	out := &ecs.DescribeTasksOutput{}
	for _, arn := range in.Tasks {
		out.Tasks = append(out.Tasks, types.Task{
			TaskArn:    aws.String(arn),
			ClusterArn: in.Cluster,
			LastStatus: aws.String("RUNNING"),
			Tags:       prodTag,
		})
	}
	return out, nil
}

func (f *fakeECS) ListTaskDefinitions(_ context.Context, in *ecs.ListTaskDefinitionsInput, _ ...func(*ecs.Options)) (*ecs.ListTaskDefinitionsOutput, error) {
	page, next := pageOf(f.taskDefPages, in.NextToken)
	return &ecs.ListTaskDefinitionsOutput{TaskDefinitionArns: page, NextToken: next}, nil
}

func (f *fakeECS) DescribeTaskDefinition(_ context.Context, in *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	if f.describeErr != nil {
		return nil, f.describeErr
	}
	f.describeInclude = append(f.describeInclude, stringsOf(in.Include))

	return &ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: &types.TaskDefinition{
			TaskDefinitionArn: in.TaskDefinition,
			Family:            aws.String("claims-api"),
			Revision:          7,
			NetworkMode:       types.NetworkModeAwsvpc,
		},
		Tags: prodTag,
	}, nil
}

func pageOf(pages [][]string, token *string) (page []string, next *string) {
	i, _ := strconv.Atoi(aws.ToString(token))
	if i >= len(pages) {
		return nil, nil
	}
	if i+1 < len(pages) {
		next = aws.String(strconv.Itoa(i + 1))
	}
	return pages[i], next
}

func stringsOf[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

func decode(t *testing.T, data any) map[string]any {
	t.Helper()

	encoded, err := json.Marshal(data)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	return decoded
}
