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

	// LastFilterTypeForTest must be the last one in this const group for testing purpose
	LastFilterTypeForTest
)

// FilterOption is used to provide filters for dynamic config keys
type FilterOption func(filterMap map[Filter]interface{})

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

// NamespaceFilter filters by namespace
func NamespaceFilter(namespace string) FilterOption {
	return func(filterMap map[Filter]interface{}) {
		filterMap[Namespace] = namespace
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
