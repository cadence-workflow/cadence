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
)

type (
	// Config represents configuration for cadence-matching service
	Config struct {
		PersistenceMaxQPS       dynamicconfig.IntPropertyFn
		PersistenceGlobalMaxQPS dynamicconfig.IntPropertyFn
		EnableSyncMatch         dynamicconfig.BoolPropertyFnWithTaskListInfoFilters
		UserRPS                 dynamicconfig.IntPropertyFn
		WorkerRPS               dynamicconfig.IntPropertyFn
		DomainUserRPS           dynamicconfig.IntPropertyFnWithDomainFilter
		DomainWorkerRPS         dynamicconfig.IntPropertyFnWithDomainFilter
		ShutdownDrainDuration   dynamicconfig.DurationPropertyFn

		// taskListManager configuration
		RangeSize                            int64
		GetTasksBatchSize                    dynamicconfig.IntPropertyFnWithTaskListInfoFilters
		UpdateAckInterval                    dynamicconfig.DurationPropertyFnWithTaskListInfoFilters
		IdleTasklistCheckInterval            dynamicconfig.DurationPropertyFnWithTaskListInfoFilters
		MaxTasklistIdleTime                  dynamicconfig.DurationPropertyFnWithTaskListInfoFilters
		NumTasklistWritePartitions           dynamicconfig.IntPropertyFnWithTaskListInfoFilters
		NumTasklistReadPartitions            dynamicconfig.IntPropertyFnWithTaskListInfoFilters
		ForwarderMaxOutstandingPolls         dynamicconfig.IntPropertyFnWithTaskListInfoFilters
		ForwarderMaxOutstandingTasks         dynamicconfig.IntPropertyFnWithTaskListInfoFilters
		ForwarderMaxRatePerSecond            dynamicconfig.IntPropertyFnWithTaskListInfoFilters
		ForwarderMaxChildrenPerNode          dynamicconfig.IntPropertyFnWithTaskListInfoFilters
		AsyncTaskDispatchTimeout             dynamicconfig.DurationPropertyFnWithTaskListInfoFilters
		LocalPollWaitTime                    dynamicconfig.DurationPropertyFnWithTaskListInfoFilters
		LocalTaskWaitTime                    dynamicconfig.DurationPropertyFnWithTaskListInfoFilters
		EnableGetNumberOfPartitionsFromCache dynamicconfig.BoolPropertyFnWithTaskListInfoFilters
		PartitionUpscaleRPS                  dynamicconfig.IntPropertyFnWithTaskListInfoFilters
		PartitionDownscaleFactor             dynamicconfig.FloatPropertyFnWithTaskListInfoFilters
		PartitionUpscaleSustainedDuration    dynamicconfig.DurationPropertyFnWithTaskListInfoFilters
		PartitionDownscaleSustainedDuration  dynamicconfig.DurationPropertyFnWithTaskListInfoFilters
		AdaptiveScalerUpdateInterval         dynamicconfig.DurationPropertyFnWithTaskListInfoFilters
		EnableAdaptiveScaler                 dynamicconfig.BoolPropertyFnWithTaskListInfoFilters
		EnableStandbyTaskCompletion          dynamicconfig.BoolPropertyFnWithTaskListInfoFilters
		EnableClientAutoConfig               dynamicconfig.BoolPropertyFnWithTaskListInfoFilters

		// Time to hold a poll request before returning an empty response if there are no tasks
		LongPollExpirationInterval dynamicconfig.DurationPropertyFnWithTaskListInfoFilters
		MinTaskThrottlingBurstSize dynamicconfig.IntPropertyFnWithTaskListInfoFilters
		MaxTaskDeleteBatchSize     dynamicconfig.IntPropertyFnWithTaskListInfoFilters

		// taskWriter configuration
		OutstandingTaskAppendsThreshold dynamicconfig.IntPropertyFnWithTaskListInfoFilters
		MaxTaskBatchSize                dynamicconfig.IntPropertyFnWithTaskListInfoFilters

		ThrottledLogRPS dynamicconfig.IntPropertyFn

		// debugging configuration
		EnableDebugMode             bool // note that this value is initialized once on service start
		EnableTaskInfoLogByDomainID dynamicconfig.BoolPropertyFnWithDomainIDFilter

		ActivityTaskSyncMatchWaitTime dynamicconfig.DurationPropertyFnWithDomainFilter

		// isolation configuration
		EnableTasklistIsolation dynamicconfig.BoolPropertyFnWithDomainFilter
		AllIsolationGroups      func() []string
		// hostname info
		HostName string
		// rate limiter configuration
		TaskDispatchRPS    float64
		TaskDispatchRPSTTL time.Duration
		// task gc configuration
		MaxTimeBetweenTaskDeletes time.Duration

		EnableTasklistOwnershipGuard dynamicconfig.BoolPropertyFn
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
		LongPollExpirationInterval          func() time.Duration
		RangeSize                           int64
		ActivityTaskSyncMatchWaitTime       dynamicconfig.DurationPropertyFnWithDomainFilter
		GetTasksBatchSize                   func() int
		UpdateAckInterval                   func() time.Duration
		IdleTasklistCheckInterval           func() time.Duration
		MaxTasklistIdleTime                 func() time.Duration
		MinTaskThrottlingBurstSize          func() int
		MaxTaskDeleteBatchSize              func() int
		AsyncTaskDispatchTimeout            func() time.Duration
		LocalPollWaitTime                   func() time.Duration
		LocalTaskWaitTime                   func() time.Duration
		PartitionUpscaleRPS                 func() int
		PartitionDownscaleFactor            func() float64
		PartitionUpscaleSustainedDuration   func() time.Duration
		PartitionDownscaleSustainedDuration func() time.Duration
		AdaptiveScalerUpdateInterval        func() time.Duration
		// taskWriter configuration
		OutstandingTaskAppendsThreshold      func() int
		MaxTaskBatchSize                     func() int
		NumWritePartitions                   func() int
		NumReadPartitions                    func() int
		EnableGetNumberOfPartitionsFromCache func() bool
		EnableAdaptiveScaler                 func() bool
		// isolation configuration
		EnableTasklistIsolation func() bool
		// A function which returns all the isolation groups
		AllIsolationGroups func() []string
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
		PersistenceMaxQPS:                    dc.GetIntProperty(dynamicconfig.MatchingPersistenceMaxQPS),
		PersistenceGlobalMaxQPS:              dc.GetIntProperty(dynamicconfig.MatchingPersistenceGlobalMaxQPS),
		EnableSyncMatch:                      dc.GetBoolPropertyFilteredByTaskListInfo(dynamicconfig.MatchingEnableSyncMatch),
		UserRPS:                              dc.GetIntProperty(dynamicconfig.MatchingUserRPS),
		WorkerRPS:                            dc.GetIntProperty(dynamicconfig.MatchingWorkerRPS),
		DomainUserRPS:                        dc.GetIntPropertyFilteredByDomain(dynamicconfig.MatchingDomainUserRPS),
		DomainWorkerRPS:                      dc.GetIntPropertyFilteredByDomain(dynamicconfig.MatchingDomainWorkerRPS),
		RangeSize:                            100000,
		GetTasksBatchSize:                    dc.GetIntPropertyFilteredByTaskListInfo(dynamicconfig.MatchingGetTasksBatchSize),
		UpdateAckInterval:                    dc.GetDurationPropertyFilteredByTaskListInfo(dynamicconfig.MatchingUpdateAckInterval),
		IdleTasklistCheckInterval:            dc.GetDurationPropertyFilteredByTaskListInfo(dynamicconfig.MatchingIdleTasklistCheckInterval),
		MaxTasklistIdleTime:                  dc.GetDurationPropertyFilteredByTaskListInfo(dynamicconfig.MaxTasklistIdleTime),
		LongPollExpirationInterval:           dc.GetDurationPropertyFilteredByTaskListInfo(dynamicconfig.MatchingLongPollExpirationInterval),
		MinTaskThrottlingBurstSize:           dc.GetIntPropertyFilteredByTaskListInfo(dynamicconfig.MatchingMinTaskThrottlingBurstSize),
		MaxTaskDeleteBatchSize:               dc.GetIntPropertyFilteredByTaskListInfo(dynamicconfig.MatchingMaxTaskDeleteBatchSize),
		OutstandingTaskAppendsThreshold:      dc.GetIntPropertyFilteredByTaskListInfo(dynamicconfig.MatchingOutstandingTaskAppendsThreshold),
		MaxTaskBatchSize:                     dc.GetIntPropertyFilteredByTaskListInfo(dynamicconfig.MatchingMaxTaskBatchSize),
		ThrottledLogRPS:                      dc.GetIntProperty(dynamicconfig.MatchingThrottledLogRPS),
		NumTasklistWritePartitions:           dc.GetIntPropertyFilteredByTaskListInfo(dynamicconfig.MatchingNumTasklistWritePartitions),
		NumTasklistReadPartitions:            dc.GetIntPropertyFilteredByTaskListInfo(dynamicconfig.MatchingNumTasklistReadPartitions),
		ForwarderMaxOutstandingPolls:         dc.GetIntPropertyFilteredByTaskListInfo(dynamicconfig.MatchingForwarderMaxOutstandingPolls),
		ForwarderMaxOutstandingTasks:         dc.GetIntPropertyFilteredByTaskListInfo(dynamicconfig.MatchingForwarderMaxOutstandingTasks),
		ForwarderMaxRatePerSecond:            dc.GetIntPropertyFilteredByTaskListInfo(dynamicconfig.MatchingForwarderMaxRatePerSecond),
		ForwarderMaxChildrenPerNode:          dc.GetIntPropertyFilteredByTaskListInfo(dynamicconfig.MatchingForwarderMaxChildrenPerNode),
		EnableGetNumberOfPartitionsFromCache: dc.GetBoolPropertyFilteredByTaskListInfo(dynamicconfig.MatchingEnableGetNumberOfPartitionsFromCache),
		ShutdownDrainDuration:                dc.GetDurationProperty(dynamicconfig.MatchingShutdownDrainDuration),
		EnableDebugMode:                      dc.GetBoolProperty(dynamicconfig.EnableDebugMode)(),
		EnableTaskInfoLogByDomainID:          dc.GetBoolPropertyFilteredByDomainID(dynamicconfig.MatchingEnableTaskInfoLogByDomainID),
		ActivityTaskSyncMatchWaitTime:        dc.GetDurationPropertyFilteredByDomain(dynamicconfig.MatchingActivityTaskSyncMatchWaitTime),
		EnableTasklistIsolation:              dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableTasklistIsolation),
		AsyncTaskDispatchTimeout:             dc.GetDurationPropertyFilteredByTaskListInfo(dynamicconfig.AsyncTaskDispatchTimeout),
		EnableTasklistOwnershipGuard:         dc.GetBoolProperty(dynamicconfig.MatchingEnableTasklistGuardAgainstOwnershipShardLoss),
		LocalPollWaitTime:                    dc.GetDurationPropertyFilteredByTaskListInfo(dynamicconfig.LocalPollWaitTime),
		LocalTaskWaitTime:                    dc.GetDurationPropertyFilteredByTaskListInfo(dynamicconfig.LocalTaskWaitTime),
		PartitionUpscaleRPS:                  dc.GetIntPropertyFilteredByTaskListInfo(dynamicconfig.MatchingPartitionUpscaleRPS),
		PartitionDownscaleFactor:             dc.GetFloat64PropertyFilteredByTaskListInfo(dynamicconfig.MatchingPartitionDownscaleFactor),
		PartitionUpscaleSustainedDuration:    dc.GetDurationPropertyFilteredByTaskListInfo(dynamicconfig.MatchingPartitionUpscaleSustainedDuration),
		PartitionDownscaleSustainedDuration:  dc.GetDurationPropertyFilteredByTaskListInfo(dynamicconfig.MatchingPartitionDownscaleSustainedDuration),
		AdaptiveScalerUpdateInterval:         dc.GetDurationPropertyFilteredByTaskListInfo(dynamicconfig.MatchingAdaptiveScalerUpdateInterval),
		EnableAdaptiveScaler:                 dc.GetBoolPropertyFilteredByTaskListInfo(dynamicconfig.MatchingEnableAdaptiveScaler),
		HostName:                             hostName,
		TaskDispatchRPS:                      100000.0,
		TaskDispatchRPSTTL:                   time.Minute,
		MaxTimeBetweenTaskDeletes:            time.Second,
		AllIsolationGroups:                   getIsolationGroups,
		EnableStandbyTaskCompletion:          dc.GetBoolPropertyFilteredByTaskListInfo(dynamicconfig.MatchingEnableStandbyTaskCompletion),
		EnableClientAutoConfig:               dc.GetBoolPropertyFilteredByTaskListInfo(dynamicconfig.MatchingEnableClientAutoConfig),
	}
}
