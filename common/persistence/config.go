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
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
)

type (
	// DynamicConfiguration represents dynamic configuration for persistence layer
	DynamicConfiguration struct {
		EnableExecutionInfoTracking              dynamicproperties.BoolPropertyFn
		EnableSQLAsyncTransaction                dynamicproperties.BoolPropertyFn
		EnableCassandraAllConsistencyLevelDelete dynamicproperties.BoolPropertyFn
		PersistenceSampleLoggingRate             dynamicproperties.IntPropertyFn
		EnableShardIDMetrics                     dynamicproperties.BoolPropertyFn
		EnableHistoryTaskDualWriteMode           dynamicproperties.BoolPropertyFn
		ReadNoSQLHistoryTaskFromDataBlob         dynamicproperties.BoolPropertyFn
		ReadNoSQLShardFromDataBlob               dynamicproperties.BoolPropertyFn
		SerializationEncoding                    dynamicproperties.StringPropertyFn
		DomainAuditLogTTL                        dynamicproperties.DurationPropertyFnWithDomainIDFilter
		HistoryNodeDeleteBatchSize               dynamicproperties.IntPropertyFn
	}
)

// NewDynamicConfiguration returns new config with default values
func NewDynamicConfiguration(dc *dynamicconfig.Collection) *DynamicConfiguration {
	return &DynamicConfiguration{
		EnableExecutionInfoTracking:              dc.GetBoolProperty(dynamicproperties.EnableExecutionInfoTracking),
		EnableSQLAsyncTransaction:                dc.GetBoolProperty(dynamicproperties.EnableSQLAsyncTransaction),
		EnableCassandraAllConsistencyLevelDelete: dc.GetBoolProperty(dynamicproperties.EnableCassandraAllConsistencyLevelDelete),
		PersistenceSampleLoggingRate:             dc.GetIntProperty(dynamicproperties.SampleLoggingRate),
		EnableShardIDMetrics:                     dc.GetBoolProperty(dynamicproperties.EnableShardIDMetrics),
		EnableHistoryTaskDualWriteMode:           dc.GetBoolProperty(dynamicproperties.EnableNoSQLHistoryTaskDualWriteMode),
		ReadNoSQLHistoryTaskFromDataBlob:         dc.GetBoolProperty(dynamicproperties.ReadNoSQLHistoryTaskFromDataBlob),
		ReadNoSQLShardFromDataBlob:               dc.GetBoolProperty(dynamicproperties.ReadNoSQLShardFromDataBlob),
		SerializationEncoding:                    dc.GetStringProperty(dynamicproperties.SerializationEncoding),
		DomainAuditLogTTL:                        dc.GetDurationPropertyFilteredByDomainID(dynamicproperties.DomainAuditLogTTL),
		HistoryNodeDeleteBatchSize:               dc.GetIntProperty(dynamicproperties.HistoryNodeDeleteBatchSize),
	}
}
