package metrics

// Code generated ./internal/tools/metricsgen; DO NOT EDIT

import (
	"fmt"
	"time"
)

// NewShardTasklistTags constructs a new metric-tag-holding ShardTasklistTags, and it must be used
// instead of custom initialization to ensure newly added tags can be detected
// at compile time instead of missing them at run time.
func NewShardTasklistTags(
	ServiceTags ServiceTags,
	Shard int,
	OtherShard int,
	Type TasklistType,
) ShardTasklistTags {
	s := ShardTasklistTags{
		ServiceTags: ServiceTags,
		Shard:       Shard,
		OtherShard:  OtherShard,
		Type:        Type,
	}
	return s
}

// NumTags returns the number of tags that are intended to be written in all
// cases.  This will include all embedded parent tags and all reserved tags,
// and is intended to be used to pre-allocate maps of tags.
func (s ShardTasklistTags) NumTags() int {
	num := 3 // num of self fields
	num += 1 // num of reserved fields
	num += s.ServiceTags.NumTags()
	return num
}

// Tags writes this set of tags (and its embedded parents) to the passed map.
func (s ShardTasklistTags) Tags(into map[string]string) {
	s.ServiceTags.Tags(into)
	into["shard"] = fmt.Sprintf("%d", s.Shard)
	into["other_shard"] = fmt.Sprintf("other-%v", s.OtherShard)
	into["tasklist_type"] = s.Type.String()
}

// GetTags is a minor helper to get a pre-allocated-and-filled map with room
// for reserved fields (i.e. 'struct{}' type fields, which only declare intent).
func (s ShardTasklistTags) GetTags() map[string]string {
	tags := make(map[string]string, s.NumTags())
	s.Tags(tags)
	return tags
}

// convenience methods

// Inc increments a named counter with the tags on this struct.
func (s ShardTasklistTags) Inc(name string) {
	s.Count(name, 1)
}

// Count adds to a named counter with the tags on this struct.
func (s ShardTasklistTags) Count(name string, num int) {
	s.Emitter.Count(s, name, num)
}

// Histogram records a histogram at the struct's nearest precision level
func (s ShardTasklistTags) Histogram(name string, dur time.Duration) {
	s.Emitter.Histogram(s, name, dur)
}
