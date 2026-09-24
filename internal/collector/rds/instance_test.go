package rds

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windkube/aws-metrics-exporter/internal/collector"
)

type fakeInstances struct {
	pages    [][]types.DBInstance
	nextPage int
	err      error
}

func (f *fakeInstances) DescribeDBInstances(context.Context, *rds.DescribeDBInstancesInput, ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	if f.err != nil {
		return nil, f.err
	}

	page := f.pages[f.nextPage]
	f.nextPage++

	out := &rds.DescribeDBInstancesOutput{DBInstances: page}
	if f.nextPage < len(f.pages) {
		out.Marker = aws.String("next")
	}

	return out, nil
}

func dbInstance(id string) types.DBInstance {
	return types.DBInstance{
		DBInstanceIdentifier: aws.String(id),
		DBInstanceArn:        aws.String("arn:aws:rds:eu-west-1:111111111111:db:" + id),
		Engine:               aws.String("postgres"),
		TagList:              []types.Tag{{Key: aws.String("env"), Value: aws.String("prod")}},
	}
}

func instanceCollectorWith(api InstanceAPI) *InstanceCollector {
	return &InstanceCollector{newAPI: func(aws.Config) InstanceAPI { return api }}
}

func TestInstanceCollectWalksEveryPage(t *testing.T) {
	api := &fakeInstances{pages: [][]types.DBInstance{{dbInstance("claims")}, {dbInstance("payments")}}}

	var got []collector.Resource
	require.NoError(t, instanceCollectorWith(api).Collect(t.Context(), aws.Config{}, func(r collector.Resource) error {
		got = append(got, r)
		return nil
	}))

	require.Len(t, got, 2)
	assert.Equal(t, "claims", got[0].Name)
	assert.Equal(t, "arn:aws:rds:eu-west-1:111111111111:db:claims", got[0].ID)
	assert.Equal(t, "payments", got[1].Name)

	encoded, err := json.Marshal(got[0].Data)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, "postgres", decoded["Engine"])
	assert.Len(t, decoded["TagList"], 1)
}

func TestInstanceCollectReturnsDescribeError(t *testing.T) {
	api := &fakeInstances{err: errors.New("throttled")}

	err := instanceCollectorWith(api).Collect(t.Context(), aws.Config{}, func(collector.Resource) error { return nil })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "describing rds instances")
}

func TestInstanceScopeIsRegional(t *testing.T) {
	assert.Equal(t, collector.ScopeRegional, NewInstance().Scope())
	assert.Equal(t, ResourceTypeInstance, NewInstance().Type())
}
