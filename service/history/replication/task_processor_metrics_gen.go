package replication

// Code generated ./internal/tools/metricsgen; DO NOT EDIT

import (
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/metrics/structured"
)

// newPerDomainTaskMetricTags constructs a new metric-tag-holding perDomainTaskMetricTags,
// and it must be used, instead of custom initialization to ensure newly added
// tags can be detected at compile time instead of missing them at run time.
func newPerDomainTaskMetricTags(
	emitter structured.Emitter,
	ts clock.TimeSource,
	TargetCluster string,
) perDomainTaskMetricTags {
	p := perDomainTaskMetricTags{
		Emitter:       emitter,
		ts:            ts,
		TargetCluster: TargetCluster,
	}
	return p
}

// NumTags returns the number of tags that are intended to be written in all
// cases.  This will include all embedded parent tags and all reserved tags,
// and is intended to be used to pre-allocate maps of tags.
func (p perDomainTaskMetricTags) NumTags() int {
	num := 1 // num of self fields
	num += 1 // num of reserved fields
	num += p.DynamicOperationTags.NumTags()
	return num
}

// PutTags writes this set of tags (and its embedded parents) to the passed map.
func (p perDomainTaskMetricTags) PutTags(operation int, into structured.DynamicTags) {
	p.DynamicOperationTags.PutTags(operation, into)
	into["target_cluster"] = p.TargetCluster
}

// GetTags is a minor helper to get a pre-allocated-and-filled map with room
// for reserved fields (i.e. 'struct{}' type fields, which only declare intent).
func (p perDomainTaskMetricTags) GetTags(operation int) structured.DynamicTags {
	tags := make(structured.DynamicTags, p.NumTags())
	p.PutTags(operation, tags)
	return tags
}
