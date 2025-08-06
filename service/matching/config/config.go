// Copyright (c) 2019 Uber Technologies, Inc.
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

package config

import (
	"time"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
)

type (
	// Config represents configuration for cadence-matching service
	Config struct {
		PersistenceMaxQPS       dynamicproperties.IntPropertyFn
		PersistenceGlobalMaxQPS dynamicproperties.IntPropertyFn
		EnableSyncMatch         dynamicproperties.BoolPropertyFnWithTaskListInfoFilters
		UserRPS                 dynamicproperties.IntPropertyFn
		WorkerRPS               dynamicproperties.IntPropertyFn
		DomainUserRPS           dynamicproperties.IntPropertyFnWithDomainFilter
		DomainWorkerRPS         dynamicproperties.IntPropertyFnWithDomainFilter
		ShutdownDrainDuration   dynamicproperties.DurationPropertyFn

		// taskListManager configuration
		RangeSize                                 int64
		ReadRangeSize                             dynamicproperties.IntPropertyFn
		GetTasksBatchSize                         dynamicproperties.IntPropertyFnWithTaskListInfoFilters
		UpdateAckInterval                         dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		IdleTasklistCheckInterval                 dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		MaxTasklistIdleTime                       dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		NumTasklistWritePartitions                dynamicproperties.IntPropertyFnWithTaskListInfoFilters
		NumTasklistReadPartitions                 dynamicproperties.IntPropertyFnWithTaskListInfoFilters
		ForwarderMaxOutstandingPolls              dynamicproperties.IntPropertyFnWithTaskListInfoFilters
		ForwarderMaxOutstandingTasks              dynamicproperties.IntPropertyFnWithTaskListInfoFilters
		ForwarderMaxRatePerSecond                 dynamicproperties.IntPropertyFnWithTaskListInfoFilters
		ForwarderMaxChildrenPerNode               dynamicproperties.IntPropertyFnWithTaskListInfoFilters
		AsyncTaskDispatchTimeout                  dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		LocalPollWaitTime                         dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		LocalTaskWaitTime                         dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		TaskIsolationDuration                     dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		TaskIsolationPollerWindow                 dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		EnableGetNumberOfPartitionsFromCache      dynamicproperties.BoolPropertyFnWithTaskListInfoFilters
		PartitionUpscaleRPS                       dynamicproperties.IntPropertyFnWithTaskListInfoFilters
		PartitionDownscaleFactor                  dynamicproperties.FloatPropertyFnWithTaskListInfoFilters
		PartitionUpscaleSustainedDuration         dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		PartitionDownscaleSustainedDuration       dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		AdaptiveScalerUpdateInterval              dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		EnableAdaptiveScaler                      dynamicproperties.BoolPropertyFnWithTaskListInfoFilters
		EnablePartitionEmptyCheck                 dynamicproperties.BoolPropertyFnWithTaskListInfoFilters
		EnableStandbyTaskCompletion               dynamicproperties.BoolPropertyFnWithTaskListInfoFilters
		EnableClientAutoConfig                    dynamicproperties.BoolPropertyFnWithTaskListInfoFilters
		QPSTrackerInterval                        dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		EnablePartitionIsolationGroupAssignment   dynamicproperties.BoolPropertyFnWithTaskListInfoFilters
		IsolationGroupUpscaleSustainedDuration    dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		IsolationGroupDownscaleSustainedDuration  dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		IsolationGroupHasPollersSustainedDuration dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		IsolationGroupNoPollersSustainedDuration  dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		IsolationGroupsPerPartition               dynamicproperties.IntPropertyFnWithTaskListInfoFilters

		// Time to hold a poll request before returning an empty response if there are no tasks
		LongPollExpirationInterval dynamicproperties.DurationPropertyFnWithTaskListInfoFilters
		MinTaskThrottlingBurstSize dynamicproperties.IntPropertyFnWithTaskListInfoFilters
		MaxTaskDeleteBatchSize     dynamicproperties.IntPropertyFnWithTaskListInfoFilters

		// taskWriter configuration
		OutstandingTaskAppendsThreshold dynamicproperties.IntPropertyFnWithTaskListInfoFilters
		MaxTaskBatchSize                dynamicproperties.IntPropertyFnWithTaskListInfoFilters

		ThrottledLogRPS dynamicproperties.IntPropertyFn

		// debugging configuration
		EnableDebugMode             bool // note that this value is initialized once on service start
		EnableTaskInfoLogByDomainID dynamicproperties.BoolPropertyFnWithDomainIDFilter

		ActivityTaskSyncMatchWaitTime dynamicproperties.DurationPropertyFnWithDomainFilter

		// isolation configuration
		EnableTasklistIsolation dynamicproperties.BoolPropertyFnWithDomainFilter
		AllIsolationGroups      func() []string
		// hostname info
		HostName string
		// rate limiter configuration
		TaskDispatchRPS    float64
		TaskDispatchRPSTTL time.Duration
		// task gc configuration
		MaxTimeBetweenTaskDeletes time.Duration

		EnableTasklistOwnershipGuard dynamicproperties.BoolPropertyFn
	}

	ForwarderConfig struct {
		ForwarderMaxOutstandingPolls func() int
		ForwarderMaxOutstandingTasks func() int
		ForwarderMaxRatePerSecond    func() int
		ForwarderMaxChildrenPerNode  func() int
	}

	TaskListConfig struct {
		ForwarderConfig
		EnableSyncMatch func() bool
		// Time to hold a poll request before returning an empty response if there are no tasks
		LongPollExpirationInterval                func() time.Duration
		RangeSize                                 int64
		ReadRangeSize                             dynamicproperties.IntPropertyFn
		ActivityTaskSyncMatchWaitTime             dynamicproperties.DurationPropertyFnWithDomainFilter
		GetTasksBatchSize                         func() int
		UpdateAckInterval                         func() time.Duration
		IdleTasklistCheckInterval                 func() time.Duration
		MaxTasklistIdleTime                       func() time.Duration
		MinTaskThrottlingBurstSize                func() int
		MaxTaskDeleteBatchSize                    func() int
		AsyncTaskDispatchTimeout                  func() time.Duration
		LocalPollWaitTime                         func() time.Duration
		LocalTaskWaitTime                         func() time.Duration
		PartitionUpscaleRPS                       func() int
		PartitionDownscaleFactor                  func() float64
		PartitionUpscaleSustainedDuration         func() time.Duration
		PartitionDownscaleSustainedDuration       func() time.Duration
		AdaptiveScalerUpdateInterval              func() time.Duration
		QPSTrackerInterval                        func() time.Duration
		EnablePartitionIsolationGroupAssignment   func() bool
		IsolationGroupUpscaleSustainedDuration    func() time.Duration
		IsolationGroupDownscaleSustainedDuration  func() time.Duration
		IsolationGroupHasPollersSustainedDuration func() time.Duration
		IsolationGroupNoPollersSustainedDuration  func() time.Duration
		IsolationGroupsPerPartition               func() int
		// taskWriter configuration
		OutstandingTaskAppendsThreshold      func() int
		MaxTaskBatchSize                     func() int
		NumWritePartitions                   func() int
		NumReadPartitions                    func() int
		EnableGetNumberOfPartitionsFromCache func() bool
		EnableAdaptiveScaler                 func() bool
		EnablePartitionEmptyCheck            func() bool
		// isolation configuration
		EnableTasklistIsolation func() bool
		// A function which returns all the isolation groups
		AllIsolationGroups        func() []string
		TaskIsolationDuration     func() time.Duration
		TaskIsolationPollerWindow func() time.Duration
		// hostname
		HostName string
		// rate limiter configuration
		TaskDispatchRPS    float64
		TaskDispatchRPSTTL time.Duration
		// task gc configuration
		MaxTimeBetweenTaskDeletes time.Duration
		// standby task completion configuration
		EnableStandbyTaskCompletion func() bool
		EnableClientAutoConfig      func() bool
	}
)

// NewConfig returns new service config with default values
func NewConfig(dc *dynamicconfig.Collection, hostName string, getIsolationGroups func() []string) *Config {
	return &Config{
		PersistenceMaxQPS:                         dc.GetIntProperty(dynamicproperties.MatchingPersistenceMaxQPS),
		PersistenceGlobalMaxQPS:                   dc.GetIntProperty(dynamicproperties.MatchingPersistenceGlobalMaxQPS),
		EnableSyncMatch:                           dc.GetBoolPropertyFilteredByTaskListInfo(dynamicproperties.MatchingEnableSyncMatch),
		UserRPS:                                   dc.GetIntProperty(dynamicproperties.MatchingUserRPS),
		WorkerRPS:                                 dc.GetIntProperty(dynamicproperties.MatchingWorkerRPS),
		DomainUserRPS:                             dc.GetIntPropertyFilteredByDomain(dynamicproperties.MatchingDomainUserRPS),
		DomainWorkerRPS:                           dc.GetIntPropertyFilteredByDomain(dynamicproperties.MatchingDomainWorkerRPS),
		RangeSize:                                 100000,
		ReadRangeSize:                             dc.GetIntProperty(dynamicproperties.MatchingReadRangeSize),
		GetTasksBatchSize:                         dc.GetIntPropertyFilteredByTaskListInfo(dynamicproperties.MatchingGetTasksBatchSize),
		UpdateAckInterval:                         dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.MatchingUpdateAckInterval),
		IdleTasklistCheckInterval:                 dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.MatchingIdleTasklistCheckInterval),
		MaxTasklistIdleTime:                       dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.MaxTasklistIdleTime),
		LongPollExpirationInterval:                dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.MatchingLongPollExpirationInterval),
		MinTaskThrottlingBurstSize:                dc.GetIntPropertyFilteredByTaskListInfo(dynamicproperties.MatchingMinTaskThrottlingBurstSize),
		MaxTaskDeleteBatchSize:                    dc.GetIntPropertyFilteredByTaskListInfo(dynamicproperties.MatchingMaxTaskDeleteBatchSize),
		OutstandingTaskAppendsThreshold:           dc.GetIntPropertyFilteredByTaskListInfo(dynamicproperties.MatchingOutstandingTaskAppendsThreshold),
		MaxTaskBatchSize:                          dc.GetIntPropertyFilteredByTaskListInfo(dynamicproperties.MatchingMaxTaskBatchSize),
		ThrottledLogRPS:                           dc.GetIntProperty(dynamicproperties.MatchingThrottledLogRPS),
		NumTasklistWritePartitions:                dc.GetIntPropertyFilteredByTaskListInfo(dynamicproperties.MatchingNumTasklistWritePartitions),
		NumTasklistReadPartitions:                 dc.GetIntPropertyFilteredByTaskListInfo(dynamicproperties.MatchingNumTasklistReadPartitions),
		ForwarderMaxOutstandingPolls:              dc.GetIntPropertyFilteredByTaskListInfo(dynamicproperties.MatchingForwarderMaxOutstandingPolls),
		ForwarderMaxOutstandingTasks:              dc.GetIntPropertyFilteredByTaskListInfo(dynamicproperties.MatchingForwarderMaxOutstandingTasks),
		ForwarderMaxRatePerSecond:                 dc.GetIntPropertyFilteredByTaskListInfo(dynamicproperties.MatchingForwarderMaxRatePerSecond),
		ForwarderMaxChildrenPerNode:               dc.GetIntPropertyFilteredByTaskListInfo(dynamicproperties.MatchingForwarderMaxChildrenPerNode),
		EnableGetNumberOfPartitionsFromCache:      dc.GetBoolPropertyFilteredByTaskListInfo(dynamicproperties.MatchingEnableGetNumberOfPartitionsFromCache),
		ShutdownDrainDuration:                     dc.GetDurationProperty(dynamicproperties.MatchingShutdownDrainDuration),
		EnableDebugMode:                           dc.GetBoolProperty(dynamicproperties.EnableDebugMode)(),
		EnableTaskInfoLogByDomainID:               dc.GetBoolPropertyFilteredByDomainID(dynamicproperties.MatchingEnableTaskInfoLogByDomainID),
		ActivityTaskSyncMatchWaitTime:             dc.GetDurationPropertyFilteredByDomain(dynamicproperties.MatchingActivityTaskSyncMatchWaitTime),
		EnableTasklistIsolation:                   dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableTasklistIsolation),
		AsyncTaskDispatchTimeout:                  dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.AsyncTaskDispatchTimeout),
		EnableTasklistOwnershipGuard:              dc.GetBoolProperty(dynamicproperties.MatchingEnableTasklistGuardAgainstOwnershipShardLoss),
		LocalPollWaitTime:                         dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.LocalPollWaitTime),
		LocalTaskWaitTime:                         dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.LocalTaskWaitTime),
		PartitionUpscaleRPS:                       dc.GetIntPropertyFilteredByTaskListInfo(dynamicproperties.MatchingPartitionUpscaleRPS),
		PartitionDownscaleFactor:                  dc.GetFloat64PropertyFilteredByTaskListInfo(dynamicproperties.MatchingPartitionDownscaleFactor),
		PartitionUpscaleSustainedDuration:         dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.MatchingPartitionUpscaleSustainedDuration),
		PartitionDownscaleSustainedDuration:       dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.MatchingPartitionDownscaleSustainedDuration),
		AdaptiveScalerUpdateInterval:              dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.MatchingAdaptiveScalerUpdateInterval),
		EnableAdaptiveScaler:                      dc.GetBoolPropertyFilteredByTaskListInfo(dynamicproperties.MatchingEnableAdaptiveScaler),
		EnablePartitionEmptyCheck:                 dc.GetBoolPropertyFilteredByTaskListInfo(dynamicproperties.MatchingEnablePartitionEmptyCheck),
		QPSTrackerInterval:                        dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.MatchingQPSTrackerInterval),
		EnablePartitionIsolationGroupAssignment:   dc.GetBoolPropertyFilteredByTaskListInfo(dynamicproperties.EnablePartitionIsolationGroupAssignment),
		IsolationGroupUpscaleSustainedDuration:    dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.MatchingIsolationGroupUpscaleSustainedDuration),
		IsolationGroupDownscaleSustainedDuration:  dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.MatchingIsolationGroupDownscaleSustainedDuration),
		IsolationGroupHasPollersSustainedDuration: dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.MatchingIsolationGroupHasPollersSustainedDuration),
		IsolationGroupNoPollersSustainedDuration:  dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.MatchingIsolationGroupNoPollersSustainedDuration),
		IsolationGroupsPerPartition:               dc.GetIntPropertyFilteredByTaskListInfo(dynamicproperties.MatchingIsolationGroupsPerPartition),
		TaskIsolationDuration:                     dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.TaskIsolationDuration),
		TaskIsolationPollerWindow:                 dc.GetDurationPropertyFilteredByTaskListInfo(dynamicproperties.TaskIsolationPollerWindow),
		HostName:                                  hostName,
		TaskDispatchRPS:                           100000.0,
		TaskDispatchRPSTTL:                        time.Minute,
		MaxTimeBetweenTaskDeletes:                 time.Second,
		AllIsolationGroups:                        getIsolationGroups,
		EnableStandbyTaskCompletion:               dc.GetBoolPropertyFilteredByTaskListInfo(dynamicproperties.MatchingEnableStandbyTaskCompletion),
		EnableClientAutoConfig:                    dc.GetBoolPropertyFilteredByTaskListInfo(dynamicproperties.MatchingEnableClientAutoConfig),
	}
}
