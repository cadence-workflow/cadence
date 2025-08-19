package subdir

// Code generated ./internal/tools/metricsgen; DO NOT EDIT

import (
	"time"

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

// PutTags writes this set of tags (and its embedded parents) to the passed map.
func (d DoesThisWorkTags) PutTags(into map[string]string) {
	d.ServiceTags.PutTags(into)
	d.ShardTasklistTags.PutTags(into)
	into["something"] = d.Something
}

// GetTags is a minor helper to get a pre-allocated-and-filled map with room
// for reserved fields (i.e. 'struct{}' type fields, which only declare intent).
func (d DoesThisWorkTags) GetTags() map[string]string {
	tags := make(map[string]string, d.NumTags())
	d.PutTags(tags)
	return tags
}

// convenience methods

// Inc increments a named counter with the tags on this struct.
func (d DoesThisWorkTags) Inc(name string) {
	d.Count(name, 1)
}

// Count adds to a named counter with the tags on this struct.
func (d DoesThisWorkTags) Count(name string, num int) {
	d.Emitter.Count(d, name, num)
}

// Histogram records a histogram at the struct's nearest precision level
func (d DoesThisWorkTags) Histogram(name string, dur time.Duration) {
	d.Emitter.Histogram(d, name, dur)
}
