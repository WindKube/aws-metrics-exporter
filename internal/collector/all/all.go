// Package all wires every collector into a registry. It is separate from the collector package so
// that collectors can depend on the Collector interface without an import cycle.
package all

import (
	"github.com/windkube/aws-metrics-exporter/internal/collector"
	"github.com/windkube/aws-metrics-exporter/internal/collector/elasticache"
	"github.com/windkube/aws-metrics-exporter/internal/collector/iamuser"
	"github.com/windkube/aws-metrics-exporter/internal/collector/rds"
)

func Registry() *collector.Registry {
	return collector.NewRegistry(
		iamuser.New(),
		elasticache.NewReplicationGroup(),
		elasticache.NewCluster(),
		rds.NewInstance(),
		rds.NewCluster(),
	)
}
