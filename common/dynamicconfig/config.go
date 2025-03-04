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
	filterOptions ...FilterOption,
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
	filterOptions []FilterOption
}

func (c *Collection) logError(
	key Key,
	filters map[Filter]interface{},
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

// PropertyFn is a wrapper to get property from dynamic config
type PropertyFn func() interface{}

// IntPropertyFn is a wrapper to get int property from dynamic config
type IntPropertyFn func(opts ...FilterOption) int

// IntPropertyFnWithDomainFilter is a wrapper to get int property from dynamic config with domain as filter
type IntPropertyFnWithDomainFilter func(domain string) int

// IntPropertyFnWithTaskListInfoFilters is a wrapper to get int property from dynamic config with three filters: domain, taskList, taskType
type IntPropertyFnWithTaskListInfoFilters func(domain string, taskList string, taskType int) int

// IntPropertyFnWithShardIDFilter is a wrapper to get int property from dynamic config with shardID as filter
type IntPropertyFnWithShardIDFilter func(shardID int) int

// FloatPropertyFn is a wrapper to get float property from dynamic config
type FloatPropertyFn func(opts ...FilterOption) float64

// FloatPropertyFnWithShardIDFilter is a wrapper to get float property from dynamic config with shardID as filter
type FloatPropertyFnWithShardIDFilter func(shardID int) float64

// FloatPropertyFnWithTaskListInfoFilters is a wrapper to get duration property from dynamic config  with three filters: domain, taskList, taskType
type FloatPropertyFnWithTaskListInfoFilters func(domain string, taskList string, taskType int) float64

// DurationPropertyFn is a wrapper to get duration property from dynamic config
type DurationPropertyFn func(opts ...FilterOption) time.Duration

// DurationPropertyFnWithDomainFilter is a wrapper to get duration property from dynamic config with domain as filter
type DurationPropertyFnWithDomainFilter func(domain string) time.Duration

// DurationPropertyFnWithDomainIDFilter is a wrapper to get duration property from dynamic config with domainID as filter
type DurationPropertyFnWithDomainIDFilter func(domainID string) time.Duration

// DurationPropertyFnWithTaskListInfoFilters is a wrapper to get duration property from dynamic config  with three filters: domain, taskList, taskType
type DurationPropertyFnWithTaskListInfoFilters func(domain string, taskList string, taskType int) time.Duration

// DurationPropertyFnWithShardIDFilter is a wrapper to get duration property from dynamic config with shardID as filter
type DurationPropertyFnWithShardIDFilter func(shardID int) time.Duration

// BoolPropertyFn is a wrapper to get bool property from dynamic config
type BoolPropertyFn func(opts ...FilterOption) bool

// StringPropertyFn is a wrapper to get string property from dynamic config
type StringPropertyFn func(opts ...FilterOption) string

// MapPropertyFn is a wrapper to get map property from dynamic config
type MapPropertyFn func(opts ...FilterOption) map[string]interface{}

// MapPropertyFnWithDomainFilter is a wrapper to get map property from dynamic config with domainName as filter
type MapPropertyFnWithDomainFilter func(domain string) map[string]interface{}

// StringPropertyFnWithDomainFilter is a wrapper to get string property from dynamic config
type StringPropertyFnWithDomainFilter func(domain string) string

// StringPropertyFnWithTaskListInfoFilters is a wrapper to get string property from dynamic config with domainID as filter
type StringPropertyFnWithTaskListInfoFilters func(domain string, taskList string, taskType int) string

// BoolPropertyFnWithDomainFilter is a wrapper to get bool property from dynamic config with domain as filter
type BoolPropertyFnWithDomainFilter func(domain string) bool

// BoolPropertyFnWithDomainIDFilter is a wrapper to get bool property from dynamic config with domainID as filter
type BoolPropertyFnWithDomainIDFilter func(domainID string) bool

// BoolPropertyFnWithDomainIDAndWorkflowIDFilter is a wrapper to get bool property from dynamic config with domainID and workflowID as filter
type BoolPropertyFnWithDomainIDAndWorkflowIDFilter func(domainID string, workflowID string) bool

// BoolPropertyFnWithTaskListInfoFilters is a wrapper to get bool property from dynamic config with three filters: domain, taskList, taskType
type BoolPropertyFnWithTaskListInfoFilters func(domain string, taskList string, taskType int) bool

// IntPropertyFnWithWorkflowTypeFilter is a wrapper to get int property from dynamic config with domain as filter
type IntPropertyFnWithWorkflowTypeFilter func(domainName string, workflowType string) int

// DurationPropertyFnWithDomainFilter is a wrapper to get duration property from dynamic config with domain as filter
type DurationPropertyFnWithWorkflowTypeFilter func(domainName string, workflowType string) time.Duration

// ListPropertyFn is a wrapper to get a list property from dynamic config
type ListPropertyFn func(opts ...FilterOption) []interface{}

// StringPropertyWithRatelimitKeyFilter is a wrapper to get strings (currently global ratelimiter modes) per global ratelimit key
type StringPropertyWithRatelimitKeyFilter func(globalRatelimitKey string) string

// GetProperty gets a interface property and returns defaultValue if property is not found
func (c *Collection) GetProperty(key Key) PropertyFn {
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
func (c *Collection) GetIntProperty(key IntKey) IntPropertyFn {
	return func(opts ...FilterOption) int {
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
func (c *Collection) GetIntPropertyFilteredByDomain(key IntKey) IntPropertyFnWithDomainFilter {
	return func(domain string) int {
		filters := c.toFilterMap(DomainFilter(domain))
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
func (c *Collection) GetIntPropertyFilteredByWorkflowType(key IntKey) IntPropertyFnWithWorkflowTypeFilter {
	return func(domainName string, workflowType string) int {
		filters := c.toFilterMap(
			DomainFilter(domainName),
			WorkflowTypeFilter(workflowType),
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
func (c *Collection) GetDurationPropertyFilteredByWorkflowType(key DurationKey) DurationPropertyFnWithWorkflowTypeFilter {
	return func(domainName string, workflowType string) time.Duration {
		filters := c.toFilterMap(
			DomainFilter(domainName),
			WorkflowTypeFilter(workflowType),
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
func (c *Collection) GetIntPropertyFilteredByTaskListInfo(key IntKey) IntPropertyFnWithTaskListInfoFilters {
	return func(domain string, taskList string, taskType int) int {
		filters := c.toFilterMap(
			DomainFilter(domain),
			TaskListFilter(taskList),
			TaskTypeFilter(taskType),
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
func (c *Collection) GetIntPropertyFilteredByShardID(key IntKey) IntPropertyFnWithShardIDFilter {
	return func(shardID int) int {
		filters := c.toFilterMap(ShardIDFilter(shardID))
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

// GetFloat64Property gets property and asserts that it's a float64
func (c *Collection) GetFloat64Property(key FloatKey) FloatPropertyFn {
	return func(opts ...FilterOption) float64 {
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
func (c *Collection) GetFloat64PropertyFilteredByShardID(key FloatKey) FloatPropertyFnWithShardIDFilter {
	return func(shardID int) float64 {
		filters := c.toFilterMap(ShardIDFilter(shardID))
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
func (c *Collection) GetFloat64PropertyFilteredByTaskListInfo(key FloatKey) FloatPropertyFnWithTaskListInfoFilters {
	return func(domain string, taskList string, taskType int) float64 {
		filters := c.toFilterMap(
			DomainFilter(domain),
			TaskListFilter(taskList),
			TaskTypeFilter(taskType),
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
func (c *Collection) GetDurationProperty(key DurationKey) DurationPropertyFn {
	return func(opts ...FilterOption) time.Duration {
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
func (c *Collection) GetDurationPropertyFilteredByDomain(key DurationKey) DurationPropertyFnWithDomainFilter {
	return func(domain string) time.Duration {
		filters := c.toFilterMap(DomainFilter(domain))
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
func (c *Collection) GetDurationPropertyFilteredByDomainID(key DurationKey) DurationPropertyFnWithDomainIDFilter {
	return func(domainID string) time.Duration {
		filters := c.toFilterMap(DomainIDFilter(domainID))
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
func (c *Collection) GetDurationPropertyFilteredByTaskListInfo(key DurationKey) DurationPropertyFnWithTaskListInfoFilters {
	return func(domain string, taskList string, taskType int) time.Duration {
		filters := c.toFilterMap(
			DomainFilter(domain),
			TaskListFilter(taskList),
			TaskTypeFilter(taskType),
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
func (c *Collection) GetDurationPropertyFilteredByShardID(key DurationKey) DurationPropertyFnWithShardIDFilter {
	return func(shardID int) time.Duration {
		filters := c.toFilterMap(ShardIDFilter(shardID))
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
func (c *Collection) GetBoolProperty(key BoolKey) BoolPropertyFn {
	return func(opts ...FilterOption) bool {
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
func (c *Collection) GetStringProperty(key StringKey) StringPropertyFn {
	return func(opts ...FilterOption) string {
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
func (c *Collection) GetMapProperty(key MapKey) MapPropertyFn {
	return func(opts ...FilterOption) map[string]interface{} {
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
func (c *Collection) GetMapPropertyFilteredByDomain(key MapKey) MapPropertyFnWithDomainFilter {
	return func(domain string) map[string]interface{} {
		filters := c.toFilterMap(DomainFilter(domain))
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
func (c *Collection) GetStringPropertyFilteredByDomain(key StringKey) StringPropertyFnWithDomainFilter {
	return func(domain string) string {
		filters := c.toFilterMap(DomainFilter(domain))
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

func (c *Collection) GetStringPropertyFilteredByTaskListInfo(key StringKey) StringPropertyFnWithTaskListInfoFilters {
	return func(domain string, taskList string, taskType int) string {
		filters := c.toFilterMap(
			DomainFilter(domain),
			TaskListFilter(taskList),
			TaskTypeFilter(taskType),
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

// GetBoolPropertyFilteredByDomain gets property with domain filter and asserts that it's a bool
func (c *Collection) GetBoolPropertyFilteredByDomain(key BoolKey) BoolPropertyFnWithDomainFilter {
	return func(domain string) bool {
		filters := c.toFilterMap(DomainFilter(domain))
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
func (c *Collection) GetBoolPropertyFilteredByDomainID(key BoolKey) BoolPropertyFnWithDomainIDFilter {
	return func(domainID string) bool {
		filters := c.toFilterMap(DomainIDFilter(domainID))
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
func (c *Collection) GetBoolPropertyFilteredByDomainIDAndWorkflowID(key BoolKey) BoolPropertyFnWithDomainIDAndWorkflowIDFilter {
	return func(domainID string, workflowID string) bool {
		filters := c.toFilterMap(DomainIDFilter(domainID), WorkflowIDFilter(workflowID))
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
func (c *Collection) GetBoolPropertyFilteredByTaskListInfo(key BoolKey) BoolPropertyFnWithTaskListInfoFilters {
	return func(domain string, taskList string, taskType int) bool {
		filters := c.toFilterMap(
			DomainFilter(domain),
			TaskListFilter(taskList),
			TaskTypeFilter(taskType),
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

func (c *Collection) GetListProperty(key ListKey) ListPropertyFn {
	return func(opts ...FilterOption) []interface{} {
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

func (c *Collection) GetStringPropertyFilteredByRatelimitKey(key StringKey) StringPropertyWithRatelimitKeyFilter {
	return func(ratelimitKey string) string {
		filters := c.toFilterMap(RatelimitKeyFilter(ratelimitKey))
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

func (c *Collection) toFilterMap(opts ...FilterOption) map[Filter]interface{} {
	l := len(opts)
	m := make(map[Filter]interface{}, l)
	for _, opt := range opts {
		opt(m)
	}
	for _, opt := range c.filterOptions {
		opt(m)
	}
	return m
}

func (f IntPropertyFn) AsFloat64(opts ...FilterOption) func() float64 {
	return func() float64 { return float64(f(opts...)) }
}

func (f IntPropertyFnWithDomainFilter) AsFloat64(domain string) func() float64 {
	return func() float64 { return float64(f(domain)) }
}

func (f FloatPropertyFn) AsFloat64(opts ...FilterOption) func() float64 {
	return func() float64 { return float64(f(opts...)) }
}

func getFilteredKeyAsString(
	key Key,
	filters map[Filter]interface{},
) string {
	var sb strings.Builder
	sb.WriteString(key.String())
	for filter := UnknownFilter + 1; filter < LastFilterTypeForTest; filter++ {
		if value, ok := filters[filter]; ok {
			sb.WriteString(fmt.Sprintf(",%v:%v", filter.String(), value))
		}
	}
	return sb.String()
}
