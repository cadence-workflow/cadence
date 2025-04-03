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

package persistence

import (
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/collection"
)

type (
	// DynamicConfiguration represents dynamic configuration for persistence layer
	DynamicConfiguration struct {
		EnableSQLAsyncTransaction                dynamicconfig.BoolPropertyFn
		EnableCassandraAllConsistencyLevelDelete dynamicconfig.BoolPropertyFn
		PersistenceSampleLoggingRate             dynamicconfig.IntPropertyFn
		EnableShardIDMetrics                     dynamicconfig.BoolPropertyFn
		EnableHistoryTaskDualWriteMode           dynamicconfig.BoolPropertyFn
		ReadNoSQLHistoryTaskFromDataBlob         dynamicconfig.BoolPropertyFn
	}
)

// NewDynamicConfiguration returns new config with default values
func NewDynamicConfiguration(dc *collection.Collection) *DynamicConfiguration {
	return &DynamicConfiguration{
		EnableSQLAsyncTransaction:                dc.GetBoolProperty(dynamicconfig.EnableSQLAsyncTransaction),
		EnableCassandraAllConsistencyLevelDelete: dc.GetBoolProperty(dynamicconfig.EnableCassandraAllConsistencyLevelDelete),
		PersistenceSampleLoggingRate:             dc.GetIntProperty(dynamicconfig.SampleLoggingRate),
		EnableShardIDMetrics:                     dc.GetBoolProperty(dynamicconfig.EnableShardIDMetrics),
		EnableHistoryTaskDualWriteMode:           dc.GetBoolProperty(dynamicconfig.EnableNoSQLHistoryTaskDualWriteMode),
		ReadNoSQLHistoryTaskFromDataBlob:         dc.GetBoolProperty(dynamicconfig.ReadNoSQLHistoryTaskFromDataBlob),
	}
}
