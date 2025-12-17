// Copyright (c) 2017 Uber Technologies, Inc.
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

package dynamicconfig

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

const (
	errCountLogThreshold = 1000
)

// NewCollection creates a new collection
func NewCollection(
	client Client,
	logger log.Logger,
	filterOptions ...dynamicproperties.FilterOption,
) *Collection {

	return &Collection{
		client:        client,
		logger:        logger,
		errCount:      -1,
		filterOptions: filterOptions,
	}
}

// Collection wraps dynamic config client with a closure so that across the code, the config values
// can be directly accessed by calling the function without propagating the client everywhere in
// code
type Collection struct {
	client        Client
	logger        log.Logger
	errCount      int64
	filterOptions []dynamicproperties.FilterOption
}

func (c *Collection) logError(
	key dynamicproperties.Key,
	filters map[dynamicproperties.Filter]interface{},
	err error,
) {
	errCount := atomic.AddInt64(&c.errCount, 1)
	if errCount%errCountLogThreshold == 0 {
		// log only every 'x' errors to reduce mem allocs and to avoid log noise
		filteredKey := getFilteredKeyAsString(key, filters)
		if _, ok := err.(*types.EntityNotExistsError); ok {
			c.logger.Debug("dynamic config not set, use default value", tag.Key(filteredKey))
		} else {
			c.logger.Warn("Failed to fetch key from dynamic config", tag.Key(filteredKey), tag.Error(err))
		}
	}
}

// GetProperty gets a interface property and returns defaultValue if property is not found
func (c *Collection) GetProperty(key dynamicproperties.Key) dynamicproperties.PropertyFn {
	return func() interface{} {
		val, err := c.client.GetValue(key)
		if err != nil {
			c.logError(key, nil, err)
			return key.DefaultValue()
		}
		return val
	}
}

// GetIntProperty gets property and asserts that it's an integer
func (c *Collection) GetIntProperty(key dynamicproperties.IntKey) dynamicproperties.IntPropertyFn {
	return func(opts ...dynamicproperties.FilterOption) int {
		filters := c.toFilterMap(opts...)
		val, err := c.client.GetIntValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultInt()
		}
		return val
	}
}

// GetIntPropertyFilteredByDomain gets property with domain filter and asserts that it's an integer
func (c *Collection) GetIntPropertyFilteredByDomain(key dynamicproperties.IntKey) dynamicproperties.IntPropertyFnWithDomainFilter {
	return func(domain string) int {
		filters := c.toFilterMap(dynamicproperties.DomainFilter(domain))
		val, err := c.client.GetIntValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultInt()
		}
		return val
	}
}

// GetIntPropertyFilteredByWorkflowType gets property with workflow type filter and asserts that it's an integer
func (c *Collection) GetIntPropertyFilteredByWorkflowType(key dynamicproperties.IntKey) dynamicproperties.IntPropertyFnWithWorkflowTypeFilter {
	return func(domainName string, workflowType string) int {
		filters := c.toFilterMap(
			dynamicproperties.DomainFilter(domainName),
			dynamicproperties.WorkflowTypeFilter(workflowType),
		)
		val, err := c.client.GetIntValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultInt()
		}
		return val
	}
}

// GetDurationPropertyFilteredByWorkflowType gets property with workflow type filter and asserts that it's a duration
func (c *Collection) GetDurationPropertyFilteredByWorkflowType(key dynamicproperties.DurationKey) dynamicproperties.DurationPropertyFnWithWorkflowTypeFilter {
	return func(domainName string, workflowType string) time.Duration {
		filters := c.toFilterMap(
			dynamicproperties.DomainFilter(domainName),
			dynamicproperties.WorkflowTypeFilter(workflowType),
		)
		val, err := c.client.GetDurationValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultDuration()
		}
		return val
	}
}

// GetIntPropertyFilteredByTaskListInfo gets property with taskListInfo as filters and asserts that it's an integer
func (c *Collection) GetIntPropertyFilteredByTaskListInfo(key dynamicproperties.IntKey) dynamicproperties.IntPropertyFnWithTaskListInfoFilters {
	return func(domain string, taskList string, taskType int) int {
		filters := c.toFilterMap(
			dynamicproperties.DomainFilter(domain),
			dynamicproperties.TaskListFilter(taskList),
			dynamicproperties.TaskTypeFilter(taskType),
		)
		val, err := c.client.GetIntValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultInt()
		}
		return val
	}
}

// GetIntPropertyFilteredByShardID gets property with shardID as filter and asserts that it's an integer
func (c *Collection) GetIntPropertyFilteredByShardID(key dynamicproperties.IntKey) dynamicproperties.IntPropertyFnWithShardIDFilter {
	return func(shardID int) int {
		filters := c.toFilterMap(dynamicproperties.ShardIDFilter(shardID))
		val, err := c.client.GetIntValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultInt()
		}
		return val
	}
}

// GetBoolPropertyFilteredByShardID gets property with shardID as filter and asserts that it's a bool
func (c *Collection) GetBoolPropertyFilteredByShardID(key dynamicproperties.BoolKey) dynamicproperties.BoolPropertyFnWithShardIDFilter {
	return func(shardID int) bool {
		filters := c.toFilterMap(dynamicproperties.ShardIDFilter(shardID))
		val, err := c.client.GetBoolValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultBool()
		}
		return val
	}
}

// GetFloat64Property gets property and asserts that it's a float64
func (c *Collection) GetFloat64Property(key dynamicproperties.FloatKey) dynamicproperties.FloatPropertyFn {
	return func(opts ...dynamicproperties.FilterOption) float64 {
		filters := c.toFilterMap(opts...)
		val, err := c.client.GetFloatValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultFloat()
		}
		return val
	}
}

// GetFloat64PropertyFilteredByShardID gets property with shardID filter and asserts that it's a float64
func (c *Collection) GetFloat64PropertyFilteredByShardID(key dynamicproperties.FloatKey) dynamicproperties.FloatPropertyFnWithShardIDFilter {
	return func(shardID int) float64 {
		filters := c.toFilterMap(dynamicproperties.ShardIDFilter(shardID))
		val, err := c.client.GetFloatValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultFloat()
		}
		return val
	}
}

// GetFloatPropertyFilteredByTaskListInfo gets property with taskListInfo as filters and asserts that it's a float64
func (c *Collection) GetFloat64PropertyFilteredByTaskListInfo(key dynamicproperties.FloatKey) dynamicproperties.FloatPropertyFnWithTaskListInfoFilters {
	return func(domain string, taskList string, taskType int) float64 {
		filters := c.toFilterMap(
			dynamicproperties.DomainFilter(domain),
			dynamicproperties.TaskListFilter(taskList),
			dynamicproperties.TaskTypeFilter(taskType),
		)
		val, err := c.client.GetFloatValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultFloat()
		}
		return val
	}
}

// GetDurationProperty gets property and asserts that it's a duration
func (c *Collection) GetDurationProperty(key dynamicproperties.DurationKey) dynamicproperties.DurationPropertyFn {
	return func(opts ...dynamicproperties.FilterOption) time.Duration {
		filters := c.toFilterMap(opts...)
		val, err := c.client.GetDurationValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultDuration()
		}
		return val
	}
}

// GetDurationPropertyFilteredByDomain gets property with domain filter and asserts that it's a duration
func (c *Collection) GetDurationPropertyFilteredByDomain(key dynamicproperties.DurationKey) dynamicproperties.DurationPropertyFnWithDomainFilter {
	return func(domain string) time.Duration {
		filters := c.toFilterMap(dynamicproperties.DomainFilter(domain))
		val, err := c.client.GetDurationValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultDuration()
		}
		return val
	}
}

// GetDurationPropertyFilteredByDomainID gets property with domainID filter and asserts that it's a duration
func (c *Collection) GetDurationPropertyFilteredByDomainID(key dynamicproperties.DurationKey) dynamicproperties.DurationPropertyFnWithDomainIDFilter {
	return func(domainID string) time.Duration {
		filters := c.toFilterMap(dynamicproperties.DomainIDFilter(domainID))
		val, err := c.client.GetDurationValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultDuration()
		}
		return val
	}
}

// GetDurationPropertyFilteredByTaskListInfo gets property with taskListInfo as filters and asserts that it's a duration
func (c *Collection) GetDurationPropertyFilteredByTaskListInfo(key dynamicproperties.DurationKey) dynamicproperties.DurationPropertyFnWithTaskListInfoFilters {
	return func(domain string, taskList string, taskType int) time.Duration {
		filters := c.toFilterMap(
			dynamicproperties.DomainFilter(domain),
			dynamicproperties.TaskListFilter(taskList),
			dynamicproperties.TaskTypeFilter(taskType),
		)
		val, err := c.client.GetDurationValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultDuration()
		}
		return val
	}
}

// GetDurationPropertyFilteredByShardID gets property with shardID id as filter and asserts that it's a duration
func (c *Collection) GetDurationPropertyFilteredByShardID(key dynamicproperties.DurationKey) dynamicproperties.DurationPropertyFnWithShardIDFilter {
	return func(shardID int) time.Duration {
		filters := c.toFilterMap(dynamicproperties.ShardIDFilter(shardID))
		val, err := c.client.GetDurationValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultDuration()
		}
		return val
	}
}

// GetBoolProperty gets property and asserts that it's an bool
func (c *Collection) GetBoolProperty(key dynamicproperties.BoolKey) dynamicproperties.BoolPropertyFn {
	return func(opts ...dynamicproperties.FilterOption) bool {
		filters := c.toFilterMap(opts...)
		val, err := c.client.GetBoolValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultBool()
		}
		return val
	}
}

// GetStringProperty gets property and asserts that it's an string
func (c *Collection) GetStringProperty(key dynamicproperties.StringKey) dynamicproperties.StringPropertyFn {
	return func(opts ...dynamicproperties.FilterOption) string {
		filters := c.toFilterMap(opts...)
		val, err := c.client.GetStringValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultString()
		}
		return val
	}
}

// GetMapProperty gets property and asserts that it's a map
func (c *Collection) GetMapProperty(key dynamicproperties.MapKey) dynamicproperties.MapPropertyFn {
	return func(opts ...dynamicproperties.FilterOption) map[string]interface{} {
		filters := c.toFilterMap(opts...)
		val, err := c.client.GetMapValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultMap()
		}
		return val
	}
}

// GetMapPropertyFilteredByDomain gets property with domain filter and asserts that it's a map
func (c *Collection) GetMapPropertyFilteredByDomain(key dynamicproperties.MapKey) dynamicproperties.MapPropertyFnWithDomainFilter {
	return func(domain string) map[string]interface{} {
		filters := c.toFilterMap(dynamicproperties.DomainFilter(domain))
		val, err := c.client.GetMapValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultMap()
		}
		return val
	}
}

// GetStringPropertyFilteredByDomain gets property with domain filter and asserts that it's a string
func (c *Collection) GetStringPropertyFilteredByDomain(key dynamicproperties.StringKey) dynamicproperties.StringPropertyFnWithDomainFilter {
	return func(domain string) string {
		filters := c.toFilterMap(dynamicproperties.DomainFilter(domain))
		val, err := c.client.GetStringValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultString()
		}
		return val
	}
}

func (c *Collection) GetStringPropertyFilteredByTaskListInfo(key dynamicproperties.StringKey) dynamicproperties.StringPropertyFnWithTaskListInfoFilters {
	return func(domain string, taskList string, taskType int) string {
		filters := c.toFilterMap(
			dynamicproperties.DomainFilter(domain),
			dynamicproperties.TaskListFilter(taskList),
			dynamicproperties.TaskTypeFilter(taskType),
		)
		val, err := c.client.GetStringValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultString()
		}
		return val
	}
}

// GetStringPropertyFilteredByNamespace gets property with domain filter and asserts that it's a string
func (c *Collection) GetStringPropertyFilteredByNamespace(key dynamicproperties.StringKey) dynamicproperties.StringPropertyFnWithNamespaceFilters {
	return func(namespace string) string {
		filters := c.toFilterMap(dynamicproperties.NamespaceFilter(namespace))
		val, err := c.client.GetStringValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultString()
		}
		return val
	}
}

// GetIntPropertyFilteredByNamespace gets property with domain filter and asserts that it's a string
func (c *Collection) GetIntPropertyFilteredByNamespace(key dynamicproperties.IntKey) dynamicproperties.IntPropertyFnWithNamespaceFilters {
	return func(namespace string) int {
		filters := c.toFilterMap(dynamicproperties.NamespaceFilter(namespace))
		val, err := c.client.GetIntValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultInt()
		}
		return val
	}
}

// GetBoolPropertyFilteredByDomain gets property with domain filter and asserts that it's a bool
func (c *Collection) GetBoolPropertyFilteredByDomain(key dynamicproperties.BoolKey) dynamicproperties.BoolPropertyFnWithDomainFilter {
	return func(domain string) bool {
		filters := c.toFilterMap(dynamicproperties.DomainFilter(domain))
		val, err := c.client.GetBoolValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultBool()
		}
		return val
	}
}

// GetBoolPropertyFilteredByDomainID gets property with domainID filter and asserts that it's a bool
func (c *Collection) GetBoolPropertyFilteredByDomainID(key dynamicproperties.BoolKey) dynamicproperties.BoolPropertyFnWithDomainIDFilter {
	return func(domainID string) bool {
		filters := c.toFilterMap(dynamicproperties.DomainIDFilter(domainID))
		val, err := c.client.GetBoolValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultBool()
		}
		return val
	}
}

// GetBoolPropertyFilteredByDomainIDAndWorkflowID gets property with domainID and workflowID filters and asserts that it's a bool
func (c *Collection) GetBoolPropertyFilteredByDomainIDAndWorkflowID(key dynamicproperties.BoolKey) dynamicproperties.BoolPropertyFnWithDomainIDAndWorkflowIDFilter {
	return func(domainID string, workflowID string) bool {
		filters := c.toFilterMap(dynamicproperties.DomainIDFilter(domainID), dynamicproperties.WorkflowIDFilter(workflowID))
		val, err := c.client.GetBoolValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultBool()
		}
		return val
	}
}

// GetBoolPropertyFilteredByTaskListInfo gets property with taskListInfo as filters and asserts that it's an bool
func (c *Collection) GetBoolPropertyFilteredByTaskListInfo(key dynamicproperties.BoolKey) dynamicproperties.BoolPropertyFnWithTaskListInfoFilters {
	return func(domain string, taskList string, taskType int) bool {
		filters := c.toFilterMap(
			dynamicproperties.DomainFilter(domain),
			dynamicproperties.TaskListFilter(taskList),
			dynamicproperties.TaskTypeFilter(taskType),
		)
		val, err := c.client.GetBoolValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultBool()
		}
		return val
	}
}

func (c *Collection) GetListProperty(key dynamicproperties.ListKey) dynamicproperties.ListPropertyFn {
	return func(opts ...dynamicproperties.FilterOption) []interface{} {
		filters := c.toFilterMap(opts...)
		val, err := c.client.GetListValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultList()
		}
		return val
	}
}

func (c *Collection) GetStringPropertyFilteredByRatelimitKey(key dynamicproperties.StringKey) dynamicproperties.StringPropertyWithRatelimitKeyFilter {
	return func(ratelimitKey string) string {
		filters := c.toFilterMap(dynamicproperties.RatelimitKeyFilter(ratelimitKey))
		val, err := c.client.GetStringValue(
			key,
			filters,
		)
		if err != nil {
			c.logError(key, filters, err)
			return key.DefaultString()
		}
		return val
	}
}

func (c *Collection) toFilterMap(opts ...dynamicproperties.FilterOption) map[dynamicproperties.Filter]interface{} {
	l := len(opts)
	m := make(map[dynamicproperties.Filter]interface{}, l)
	for _, opt := range opts {
		opt(m)
	}
	for _, opt := range c.filterOptions {
		opt(m)
	}
	return m
}

func getFilteredKeyAsString(
	key dynamicproperties.Key,
	filters map[dynamicproperties.Filter]interface{},
) string {
	var sb strings.Builder
	sb.WriteString(key.String())
	for filter := dynamicproperties.UnknownFilter + 1; filter < dynamicproperties.LastFilterTypeForTest; filter++ {
		if value, ok := filters[filter]; ok {
			sb.WriteString(fmt.Sprintf(",%v:%v", filter.String(), value))
		}
	}
	return sb.String()
}
