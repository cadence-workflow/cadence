package subdir

// Code generated ./internal/tools/metricsgen; DO NOT EDIT

import (
	"github.com/uber/cadence/service/history/metrics"
)

// NewDoesThisWorkTags constructs a new metric-tag-holding DoesThisWorkTags, and it must be used
// instead of custom initialization to ensure newly added tags can be detected
// at compile time instead of missing them at run time.
func NewDoesThisWorkTags(
	ServiceTags metrics.ServiceTags,
	ShardTasklistTags metrics.ShardTasklistTags,
	Something string,
) DoesThisWorkTags {
	d := DoesThisWorkTags{
		ServiceTags:       ServiceTags,
		ShardTasklistTags: ShardTasklistTags,
		Something:         Something,
	}
	return d
}

// NumTags returns the number of tags that are intended to be written in all
// cases.  This will include all embedded parent tags and all reserved tags,
// and is intended to be used to pre-allocate maps of tags.
func (d DoesThisWorkTags) NumTags() int {
	num := 1 // num of self fields
	num += 1 // num of reserved fields
	num += d.ServiceTags.NumTags()
	num += d.ShardTasklistTags.NumTags()
	return num
}

// Tags writes this set of tags (and its embedded parents) to the passed map.
func (d DoesThisWorkTags) Tags(into map[string]string) {
	d.ServiceTags.Tags(into)
	d.ShardTasklistTags.Tags(into)
	into["something"] = d.Something
}

// GetTags is a minor helper to get a pre-allocated-and-filled map with room
// for reserved fields (i.e. 'struct{}' type fields, which only declare intent).
func (d DoesThisWorkTags) GetTags() map[string]string {
	tags := make(map[string]string, d.NumTags())
	d.Tags(tags)
	return tags
}
