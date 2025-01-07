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
	"github.com/uber/cadence/common/dynamicconfig"
)

type (
	// Config is a subset of the service dynamic config for single service
	Config struct {
		PersistenceMaxQPS       dynamicconfig.IntPropertyFn
		PersistenceGlobalMaxQPS dynamicconfig.IntPropertyFn
		ThrottledLoggerMaxRPS   dynamicconfig.IntPropertyFn

		// EnableReadVisibilityFromES is the read mode of visibility
		EnableReadVisibilityFromES dynamicconfig.BoolPropertyFnWithDomainFilter
		// AdvancedVisibilityWritingMode is the write mode of visibility
		AdvancedVisibilityWritingMode dynamicconfig.StringPropertyFn
		// AdvancedVisibilityWritingMode is the write mode of visibility during migration
		AdvancedVisibilityMigrationWritingMode dynamicconfig.StringPropertyFn
		// EnableReadVisibilityFromPinot is the read mode of visibility
		EnableReadVisibilityFromPinot dynamicconfig.BoolPropertyFnWithDomainFilter
		// EnableVisibilityDoubleRead is to enable double read for a latency comparison
		EnableVisibilityDoubleRead dynamicconfig.BoolPropertyFnWithDomainFilter
		// EnableLogCustomerQueryParameter is to enable log customer parameters
		EnableLogCustomerQueryParameter dynamicconfig.BoolPropertyFnWithDomainFilter

		// configs for db visibility
		EnableDBVisibilitySampling                  dynamicconfig.BoolPropertyFn                `yaml:"-" json:"-"`
		EnableReadDBVisibilityFromClosedExecutionV2 dynamicconfig.BoolPropertyFn                `yaml:"-" json:"-"`
		WriteDBVisibilityOpenMaxQPS                 dynamicconfig.IntPropertyFnWithDomainFilter `yaml:"-" json:"-"`
		WriteDBVisibilityClosedMaxQPS               dynamicconfig.IntPropertyFnWithDomainFilter `yaml:"-" json:"-"`
		DBVisibilityListMaxQPS                      dynamicconfig.IntPropertyFnWithDomainFilter `yaml:"-" json:"-"`

		// configs for es visibility
		ESIndexMaxResultWindow dynamicconfig.IntPropertyFn `yaml:"-" json:"-"`
		ValidSearchAttributes  dynamicconfig.MapPropertyFn `yaml:"-" json:"-"`
		// deprecated: never read from, all ES reads and writes erroneously use PersistenceMaxQPS
		ESVisibilityListMaxQPS dynamicconfig.IntPropertyFnWithDomainFilter `yaml:"-" json:"-"`

		IsErrorRetryableFunction backoff.IsRetryable
	}
)
