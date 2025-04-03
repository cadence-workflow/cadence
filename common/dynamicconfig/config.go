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
	"time"
)

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

func (f IntPropertyFn) AsFloat64(opts ...FilterOption) func() float64 {
	return func() float64 { return float64(f(opts...)) }
}

func (f IntPropertyFnWithDomainFilter) AsFloat64(domain string) func() float64 {
	return func() float64 { return float64(f(domain)) }
}

func (f FloatPropertyFn) AsFloat64(opts ...FilterOption) func() float64 {
	return func() float64 { return float64(f(opts...)) }
}
