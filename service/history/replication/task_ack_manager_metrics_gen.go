package replication

// Code generated ./internal/tools/metricsgen; DO NOT EDIT

import (
	"fmt"
	"time"

	"github.com/uber-go/tally"

	"github.com/uber/cadence/common/metrics/structured"
)

// newAckTags constructs a new metric-tag-holding ackTags, and it must be used
// instead of custom initialization to ensure newly added tags can be detected
// at compile time instead of missing them at run time.
func newAckTags(
	emitter structured.Emitter,
	OperationTags structured.OperationTags,
	Shard int,
) ackTags {
	a := ackTags{
		Emitter:       emitter,
		OperationTags: OperationTags,
		Shard:         Shard,
	}
	return a
}

// NumTags returns the number of tags that are intended to be written in all
// cases.  This will include all embedded parent tags and all reserved tags,
// and is intended to be used to pre-allocate maps of tags.
func (a ackTags) NumTags() int {
	num := 1 // num of self fields
	num += 0 // num of reserved fields
	num += a.OperationTags.NumTags()
	return num
}

// PutTags writes this set of tags (and its embedded parents) to the passed map.
func (a ackTags) PutTags(into map[string]string) {
	a.OperationTags.PutTags(into)
	into["shard"] = fmt.Sprintf("%d", a.Shard)
}

// GetTags is a minor helper to get a pre-allocated-and-filled map with room
// for reserved fields (i.e. 'struct{}' type fields, which only declare intent).
func (a ackTags) GetTags() map[string]string {
	tags := make(map[string]string, a.NumTags())
	a.PutTags(tags)
	return tags
}

// convenience methods

// Inc increments a named counter with the tags on this struct.
func (a ackTags) Inc(name string) {
	a.Count(name, 1)
}

// Count adds to a named counter with the tags on this struct.
func (a ackTags) Count(name string, num int) {
	a.Emitter.Count(a, name, num)
}

// Histogram records a histogram with the specified buckets.
//
// Buckets should essentially ALWAYS be one of the pre-defined values in [github.com/uber/cadence/common/metrics/structured],
// or pass nil to choose the [github.com/uber/cadence/common/metrics/structured.Default1ms10m] default value.
func (a ackTags) Histogram(name string, buckets tally.DurationBuckets, dur time.Duration) {
	a.Emitter.Histogram(a, name, buckets, dur)
}
