// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package defaultisolationgroupstate

import (
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/types"
)

// IsolationGroups is an internal convenience return type of a collection of IsolationGroup configurations
type isolationGroups struct {
	Global types.IsolationGroupConfiguration
	Domain types.IsolationGroupConfiguration
}

// defaultConfig values for the partitioning library for segmenting portions of workflows into isolation-groups - a resiliency
// concept meant to help move workflows around and away from failure zones.
type defaultConfig struct {
	// IsolationGroupEnabled is a domain-based configuration value for whether this feature is enabled at all
	IsolationGroupEnabled dynamicconfig.BoolPropertyFnWithDomainFilter
	// AllIsolationGroups is all the possible isolation-groups available for a region
	AllIsolationGroups func() []string
}
