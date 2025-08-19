package metrics

// Code generated ./internal/tools/metricsgen; DO NOT EDIT

import (
	"time"
)

// NewServiceTags constructs a new metric-tag-holding ServiceTags, and it must be used
// instead of custom initialization to ensure newly added tags can be detected
// at compile time instead of missing them at run time.
func NewServiceTags(
	Hostname string,
	RuntimeEnv string,
) ServiceTags {
	s := ServiceTags{
		Hostname:   Hostname,
		RuntimeEnv: RuntimeEnv,
	}
	return s
}

// NumTags returns the number of tags that are intended to be written in all
// cases.  This will include all embedded parent tags and all reserved tags,
// and is intended to be used to pre-allocate maps of tags.
func (s ServiceTags) NumTags() int {
	num := 2 // num of self fields
	num += 0 // num of reserved fields
	return num
}

// PutTags writes this set of tags (and its embedded parents) to the passed map.
func (s ServiceTags) PutTags(into map[string]string) {
	into["host"] = s.Hostname
	into["env"] = s.RuntimeEnv
}

// GetTags is a minor helper to get a pre-allocated-and-filled map with room
// for reserved fields (i.e. 'struct{}' type fields, which only declare intent).
func (s ServiceTags) GetTags() map[string]string {
	tags := make(map[string]string, s.NumTags())
	s.PutTags(tags)
	return tags
}

// convenience methods

// Inc increments a named counter with the tags on this struct.
func (s ServiceTags) Inc(name string) {
	s.Count(name, 1)
}

// Count adds to a named counter with the tags on this struct.
func (s ServiceTags) Count(name string, num int) {
	s.Emitter.Count(s, name, num)
}

// Histogram records a histogram at the struct's nearest precision level
func (s ServiceTags) Histogram(name string, dur time.Duration) {
	s.Emitter.Histogram(s, name, dur)
}

// NewUsernameTags constructs a new metric-tag-holding UsernameTags, and it must be used
// instead of custom initialization to ensure newly added tags can be detected
// at compile time instead of missing them at run time.
func NewUsernameTags() UsernameTags {
	u := UsernameTags{}
	return u
}

// NumTags returns the number of tags that are intended to be written in all
// cases.  This will include all embedded parent tags and all reserved tags,
// and is intended to be used to pre-allocate maps of tags.
func (u UsernameTags) NumTags() int {
	num := 0 // num of self fields
	num += 1 // num of reserved fields
	return num
}

// GetTags is a minor helper to get a pre-allocated-and-filled map with room
// for reserved fields (i.e. 'struct{}' type fields, which only declare intent).
func (u UsernameTags) GetTags() map[string]string {
	tags := make(map[string]string, u.NumTags())
	u.PutTags(tags)
	return tags
}
