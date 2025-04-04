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

package service

import (
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
)

type (
	// Config is a subset of the service dynamic config for single service
	Config struct {
		PersistenceMaxQPS       dynamicproperties.IntPropertyFn
		PersistenceGlobalMaxQPS dynamicproperties.IntPropertyFn
		ThrottledLoggerMaxRPS   dynamicproperties.IntPropertyFn

		// WriteVisibilityStoreName is the write mode of visibility
		WriteVisibilityStoreName dynamicproperties.StringPropertyFn
		// EnableLogCustomerQueryParameter is to enable log customer parameters
		EnableLogCustomerQueryParameter dynamicproperties.BoolPropertyFnWithDomainFilter
		// ReadVisibilityStoreName is the read store for visibility
		ReadVisibilityStoreName dynamicproperties.StringPropertyFnWithDomainFilter

		// configs for db visibility
		EnableDBVisibilitySampling                  dynamicproperties.BoolPropertyFn                `yaml:"-" json:"-"`
		EnableReadDBVisibilityFromClosedExecutionV2 dynamicproperties.BoolPropertyFn                `yaml:"-" json:"-"`
		WriteDBVisibilityOpenMaxQPS                 dynamicproperties.IntPropertyFnWithDomainFilter `yaml:"-" json:"-"`
		WriteDBVisibilityClosedMaxQPS               dynamicproperties.IntPropertyFnWithDomainFilter `yaml:"-" json:"-"`
		DBVisibilityListMaxQPS                      dynamicproperties.IntPropertyFnWithDomainFilter `yaml:"-" json:"-"`

		// configs for es visibility
		ESIndexMaxResultWindow          dynamicproperties.IntPropertyFn `yaml:"-" json:"-"`
		ValidSearchAttributes           dynamicproperties.MapPropertyFn `yaml:"-" json:"-"`
		PinotOptimizedQueryColumns      dynamicproperties.MapPropertyFn `yaml:"-" json:"-"`
		SearchAttributesHiddenValueKeys dynamicproperties.MapPropertyFn `yaml:"-" json:"-"`
		// deprecated: never read from, all ES reads and writes erroneously use PersistenceMaxQPS
		ESVisibilityListMaxQPS dynamicproperties.IntPropertyFnWithDomainFilter `yaml:"-" json:"-"`

		IsErrorRetryableFunction backoff.IsRetryable
	}
)
