package subdir

import "github.com/uber/cadence/service/history/metrics"

//go:generate metricsgen

type DoesThisWorkTags struct {
	metrics.ServiceTags
	metrics.ShardTasklistTags

	Something string   `tag:"something"`
	Reserved  struct{} `tag:"reserved"`

	cache map[string]string
}
