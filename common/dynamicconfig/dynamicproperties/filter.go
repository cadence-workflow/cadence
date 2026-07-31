// Copyright (c) 2021 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package dynamicproperties

import (
	"encoding/json"
	"fmt"

	"github.com/uber/cadence/common/types"
)

// Filter represents a filter on the dynamic config key
type Filter int

func (f Filter) String() string {
	if f > UnknownFilter && int(f) < len(filters) {
		return filters[f]
	}
	return filters[UnknownFilter]
}

func ParseFilter(filterName string) Filter {
	switch filterName {
	case "domainName":
		return DomainName
	case "domainID":
		return DomainID
	case "taskListName":
		return TaskListName
	case "taskType":
		return TaskType
	case "shardID":
		return ShardID
	case "clusterName":
		return ClusterName
	case "workflowID":
		return WorkflowID
	case "workflowType":
		return WorkflowType
	case "ratelimitKey":
		return RatelimitKey
	case "namespace":
		return Namespace
	case "shardIDPercentage":
		return ShardIDPercentage
	default:
		return UnknownFilter
	}
}

var filters = []string{
	"unknownFilter",
	"domainName",
	"domainID",
	"taskListName",
	"taskType",
	"shardID",
	"clusterName",
	"workflowID",
	"workflowType",
	"ratelimitKey",
	"namespace",
	"shardIDPercentage",
}

const (
	UnknownFilter Filter = iota
	// DomainName is the domain name
	DomainName
	// DomainID is the domain id
	DomainID
	// TaskListName is the tasklist name
	TaskListName
	// TaskType is the task type (0:Decision, 1:Activity)
	TaskType
	// ShardID is the shard id
	ShardID
	// ClusterName is the cluster name in a multi-region setup
	ClusterName
	// WorkflowID is the workflow id
	WorkflowID
	// WorkflowType is the workflow type name
	WorkflowType
	// RatelimitKey is the global ratelimit key (not a local key name)
	RatelimitKey
	// Namespace is the entity of independent shard distribution mechanism
	Namespace
	// ShardIDPercentage matches a percentage of shard IDs, for gradual rollouts
	ShardIDPercentage

	// LastFilterTypeForTest must be the last one in this const group for testing purpose
	LastFilterTypeForTest
)

// Matcher lets a filter-map value define its own comparison against a constraint,
// instead of match()/matchFilters()'s default equality check. Implement this on a
// filter's value type when a constraint isn't a plain exact match (e.g. a
// percentage threshold, as ShardIDPercentageValue does).
type Matcher interface {
	Matches(constraint interface{}) bool
}

// FilterOption is used to provide filters for dynamic config keys
type FilterOption func(filterMap map[Filter]interface{})

// ShardIDFilterOption builds a FilterOption once the shard id is known, for options
// that also depend on a value fixed at Collection-construction time (e.g. the
// cluster's shard count). Applied only within ShardID-scoped getters.
type ShardIDFilterOption func(shardID int) FilterOption

// CollectionFilterOption configures a Collection at construction time. Implemented
// by FilterOption (applied to every getter call) and ShardIDFilterOption (applied
// only within ShardID-scoped getters, once the shard id is known). Sealed to this
// package: the apply method is unexported.
type CollectionFilterOption interface {
	apply(*CollectionFilterOptions)
}

// CollectionFilterOptions is the classified result of a NewCollection(...) call's
// options.
type CollectionFilterOptions struct {
	FilterOptions        []FilterOption
	ShardIDFilterOptions []ShardIDFilterOption
}

func (f FilterOption) apply(s *CollectionFilterOptions) {
	s.FilterOptions = append(s.FilterOptions, f)
}

func (f ShardIDFilterOption) apply(s *CollectionFilterOptions) {
	s.ShardIDFilterOptions = append(s.ShardIDFilterOptions, f)
}

// NewCollectionFilterOptions classifies raw CollectionFilterOption values into their
// FilterOption / ShardIDFilterOption buckets.
func NewCollectionFilterOptions(opts ...CollectionFilterOption) *CollectionFilterOptions {
	s := &CollectionFilterOptions{}
	for _, opt := range opts {
		opt.apply(s)
	}
	return s
}

// TaskListFilter filters by task list name
func TaskListFilter(name string) FilterOption {
	return func(filterMap map[Filter]interface{}) {
		filterMap[TaskListName] = name
	}
}

// DomainFilter filters by domain name
func DomainFilter(name string) FilterOption {
	return func(filterMap map[Filter]interface{}) {
		filterMap[DomainName] = name
	}
}

// DomainIDFilter filters by domain id
func DomainIDFilter(domainID string) FilterOption {
	return func(filterMap map[Filter]interface{}) {
		filterMap[DomainID] = domainID
	}
}

// TaskTypeFilter filters by task type
func TaskTypeFilter(taskType int) FilterOption {
	return func(filterMap map[Filter]interface{}) {
		filterMap[TaskType] = taskType
	}
}

// ShardIDFilter filters by shard id
func ShardIDFilter(shardID int) FilterOption {
	return func(filterMap map[Filter]interface{}) {
		filterMap[ShardID] = shardID
	}
}

// ShardIDPercentageFilter carries the actual shard id and the cluster's real shard
// count, to be compared against a percentage threshold configured on the dynamic
// config constraint. numberOfShards is required to interpret the percentage
// correctly, since a shardIDPercentage constraint is relative to the real shard
// count, not a fixed bucket space.
func ShardIDPercentageFilter(shardID int, numberOfShards int) FilterOption {
	return func(filterMap map[Filter]interface{}) {
		filterMap[ShardIDPercentage] = ShardIDPercentageValue{ShardID: shardID, NumberOfShards: numberOfShards}
	}
}

// ShardIDPercentageFilterOption records the cluster's real shard count at Collection
// construction time, so ShardID-scoped getters can apply ShardIDPercentageFilter once
// the shard id becomes known at call time.
func ShardIDPercentageFilterOption(numberOfShards int) ShardIDFilterOption {
	return func(shardID int) FilterOption {
		return ShardIDPercentageFilter(shardID, numberOfShards)
	}
}

// ShardIDPercentageValue bundles the actual shard id together with the cluster's
// real shard count. Both are needed together to interpret a shardIDPercentage
// constraint, since the percentage is relative to the real shard count.
type ShardIDPercentageValue struct {
	ShardID        int
	NumberOfShards int
}

// Matches implements Matcher, so match()/matchFilters() dispatch shardIDPercentage
// constraints here instead of comparing via equality.
func (v ShardIDPercentageValue) Matches(constraint interface{}) bool {
	return ShardIDPercentageMatches(v, constraint)
}

// ClusterNameFilter filters by cluster name
func ClusterNameFilter(clusterName string) FilterOption {
	return func(filterMap map[Filter]interface{}) {
		filterMap[ClusterName] = clusterName
	}
}

// WorkflowIDFilter filters by workflowID
func WorkflowIDFilter(workflowID string) FilterOption {
	return func(filterMap map[Filter]interface{}) {
		filterMap[WorkflowID] = workflowID
	}
}

// WorkflowType filters by workflow type name
func WorkflowTypeFilter(name string) FilterOption {
	return func(filterMap map[Filter]interface{}) {
		filterMap[WorkflowType] = name
	}
}

// RatelimitKeyFilter filters on global ratelimiter keys (via the global name, not local names)
func RatelimitKeyFilter(key string) FilterOption {
	return func(filterMap map[Filter]interface{}) {
		filterMap[RatelimitKey] = key
	}
}

// ShardIDPercentageMatches returns true if shardIDValue (a ShardIDPercentageValue,
// bundling the actual shard id and the cluster's real shard count) falls within the
// given percentage (0.0-100.0) of shard ids. Deterministic and monotonic in
// percentage: raising percentage only ever adds shard ids to the matched set.
// An out-of-range percentage, a non-positive shard count, or a value of the wrong
// type, results in no match.
func ShardIDPercentageMatches(shardIDValue interface{}, percentageValue interface{}) bool {
	v, ok := shardIDValue.(ShardIDPercentageValue)
	if !ok || v.NumberOfShards <= 0 {
		return false
	}
	percentage, ok := toFloat64(percentageValue)
	if !ok || percentage < 0 || percentage > 100 {
		return false
	}
	bucket := v.ShardID % v.NumberOfShards
	return float64(bucket) < percentage/100*float64(v.NumberOfShards)
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// ToGetDynamicConfigFilterRequest generates a GetDynamicConfigRequest object
// by converting filters to DynamicConfigFilter objects and setting values
func ToGetDynamicConfigFilterRequest(configName string, filters []FilterOption) *types.GetDynamicConfigRequest {
	filterMap := make(map[Filter]interface{}, len(filters))
	for _, opt := range filters {
		opt(filterMap)
	}
	var dcFilters []*types.DynamicConfigFilter
	for f, entity := range filterMap {
		filter := &types.DynamicConfigFilter{
			Name: f.String(),
		}

		data, err := json.Marshal(entity)
		if err != nil {
			fmt.Errorf("could not marshall entity: %s", err)
		}

		encodingType := types.EncodingTypeJSON
		filter.Value = &types.DataBlob{
			EncodingType: &encodingType,
			Data:         data,
		}

		dcFilters = append(dcFilters, filter)
	}

	request := &types.GetDynamicConfigRequest{
		ConfigName: configName,
		Filters:    dcFilters,
	}

	return request
}
