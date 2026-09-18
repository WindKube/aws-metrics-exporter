package collector

import (
	"context"
	"fmt"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type Scope string

const (
	ScopeGlobal   Scope = "global"
	ScopeRegional Scope = "regional"
)

// GlobalRegion labels objects belonging to a service that has no region, such as IAM.
const GlobalRegion = "global"

// Resource is a single scraped AWS object. Data is marshalled as-is, so it carries the AWS JSON
// representation of the object.
type Resource struct {
	ID   string
	Name string
	Data any
}

// EmitFunc receives every scraped object. Collect must stop and return the error it returns.
type EmitFunc func(Resource) error

type Collector interface {
	Type() string
	Scope() Scope
	Collect(ctx context.Context, cfg aws.Config, emit EmitFunc) error
}

type Registry struct {
	byType map[string]Collector
}

func NewRegistry(collectors ...Collector) *Registry {
	byType := make(map[string]Collector, len(collectors))
	for _, c := range collectors {
		byType[c.Type()] = c
	}
	return &Registry{byType: byType}
}

func (r *Registry) Get(resourceType string) (Collector, error) {
	c, ok := r.byType[resourceType]
	if !ok {
		return nil, fmt.Errorf("unknown resource type %q", resourceType)
	}
	return c, nil
}

func (r *Registry) Types() []string {
	types := make([]string, 0, len(r.byType))
	for t := range r.byType {
		types = append(types, t)
	}
	slices.Sort(types)
	return types
}
