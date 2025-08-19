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

package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

type configTestCase struct {
	key   dynamicproperties.Key
	value interface{}
}

func TestNewConfig(t *testing.T) {
	hostname := "hostname"
	fields := map[string]configTestCase{
		"PersistenceMaxQPS":                         {dynamicproperties.MatchingPersistenceMaxQPS, 1},
		"PersistenceGlobalMaxQPS":                   {dynamicproperties.MatchingPersistenceGlobalMaxQPS, 2},
		"EnableSyncMatch":                           {dynamicproperties.MatchingEnableSyncMatch, true},
		"UserRPS":                                   {dynamicproperties.MatchingUserRPS, 3},
		"WorkerRPS":                                 {dynamicproperties.MatchingWorkerRPS, 4},
		"DomainUserRPS":                             {dynamicproperties.MatchingDomainUserRPS, 5},
		"DomainWorkerRPS":                           {dynamicproperties.MatchingDomainWorkerRPS, 6},
		"RangeSize":                                 {nil, int64(100000)},
		"ReadRangeSize":                             {dynamicproperties.MatchingReadRangeSize, 50000},
		"GetTasksBatchSize":                         {dynamicproperties.MatchingGetTasksBatchSize, 7},
		"UpdateAckInterval":                         {dynamicproperties.MatchingUpdateAckInterval, time.Duration(8)},
		"IdleTasklistCheckInterval":                 {dynamicproperties.MatchingIdleTasklistCheckInterval, time.Duration(9)},
		"MaxTasklistIdleTime":                       {dynamicproperties.MaxTasklistIdleTime, time.Duration(10)},
		"LongPollExpirationInterval":                {dynamicproperties.MatchingLongPollExpirationInterval, time.Duration(11)},
		"MinTaskThrottlingBurstSize":                {dynamicproperties.MatchingMinTaskThrottlingBurstSize, 12},
		"MaxTaskDeleteBatchSize":                    {dynamicproperties.MatchingMaxTaskDeleteBatchSize, 13},
		"OutstandingTaskAppendsThreshold":           {dynamicproperties.MatchingOutstandingTaskAppendsThreshold, 14},
		"MaxTaskBatchSize":                          {dynamicproperties.MatchingMaxTaskBatchSize, 15},
		"ThrottledLogRPS":                           {dynamicproperties.MatchingThrottledLogRPS, 16},
		"NumTasklistWritePartitions":                {dynamicproperties.MatchingNumTasklistWritePartitions, 17},
		"NumTasklistReadPartitions":                 {dynamicproperties.MatchingNumTasklistReadPartitions, 18},
		"ForwarderMaxOutstandingPolls":              {dynamicproperties.MatchingForwarderMaxOutstandingPolls, 19},
		"ForwarderMaxOutstandingTasks":              {dynamicproperties.MatchingForwarderMaxOutstandingTasks, 20},
		"ForwarderMaxRatePerSecond":                 {dynamicproperties.MatchingForwarderMaxRatePerSecond, 21},
		"ForwarderMaxChildrenPerNode":               {dynamicproperties.MatchingForwarderMaxChildrenPerNode, 22},
		"ShutdownDrainDuration":                     {dynamicproperties.MatchingShutdownDrainDuration, time.Duration(23)},
		"EnableDebugMode":                           {dynamicproperties.EnableDebugMode, false},
		"EnableTaskInfoLogByDomainID":               {dynamicproperties.MatchingEnableTaskInfoLogByDomainID, true},
		"ActivityTaskSyncMatchWaitTime":             {dynamicproperties.MatchingActivityTaskSyncMatchWaitTime, time.Duration(24)},
		"EnableTasklistIsolation":                   {dynamicproperties.EnableTasklistIsolation, false},
		"AsyncTaskDispatchTimeout":                  {dynamicproperties.AsyncTaskDispatchTimeout, time.Duration(25)},
		"LocalPollWaitTime":                         {dynamicproperties.LocalPollWaitTime, time.Duration(10)},
		"LocalTaskWaitTime":                         {dynamicproperties.LocalTaskWaitTime, time.Duration(10)},
		"HostName":                                  {nil, hostname},
		"TaskDispatchRPS":                           {nil, 100000.0},
		"TaskDispatchRPSTTL":                        {nil, time.Minute},
		"MaxTimeBetweenTaskDeletes":                 {nil, time.Second},
		"AllIsolationGroups":                        {nil, []string{"zone-1", "zone-2"}},
		"EnableTasklistOwnershipGuard":              {dynamicproperties.MatchingEnableTasklistGuardAgainstOwnershipShardLoss, false},
		"EnableGetNumberOfPartitionsFromCache":      {dynamicproperties.MatchingEnableGetNumberOfPartitionsFromCache, false},
		"EnablePartitionEmptyCheck":                 {dynamicproperties.MatchingEnablePartitionEmptyCheck, true},
		"PartitionUpscaleRPS":                       {dynamicproperties.MatchingPartitionUpscaleRPS, 30},
		"PartitionDownscaleFactor":                  {dynamicproperties.MatchingPartitionDownscaleFactor, 31.0},
		"PartitionUpscaleSustainedDuration":         {dynamicproperties.MatchingPartitionUpscaleSustainedDuration, time.Duration(32)},
		"PartitionDownscaleSustainedDuration":       {dynamicproperties.MatchingPartitionDownscaleSustainedDuration, time.Duration(33)},
		"AdaptiveScalerUpdateInterval":              {dynamicproperties.MatchingAdaptiveScalerUpdateInterval, time.Duration(34)},
		"EnableAdaptiveScaler":                      {dynamicproperties.MatchingEnableAdaptiveScaler, true},
		"QPSTrackerInterval":                        {dynamicproperties.MatchingQPSTrackerInterval, 5 * time.Second},
		"EnableStandbyTaskCompletion":               {dynamicproperties.MatchingEnableStandbyTaskCompletion, false},
		"EnableClientAutoConfig":                    {dynamicproperties.MatchingEnableClientAutoConfig, false},
		"TaskIsolationDuration":                     {dynamicproperties.TaskIsolationDuration, time.Duration(35)},
		"TaskIsolationPollerWindow":                 {dynamicproperties.TaskIsolationPollerWindow, time.Duration(36)},
		"EnablePartitionIsolationGroupAssignment":   {dynamicproperties.EnablePartitionIsolationGroupAssignment, true},
		"IsolationGroupUpscaleSustainedDuration":    {dynamicproperties.MatchingIsolationGroupUpscaleSustainedDuration, time.Duration(37)},
		"IsolationGroupDownscaleSustainedDuration":  {dynamicproperties.MatchingIsolationGroupDownscaleSustainedDuration, time.Duration(38)},
		"IsolationGroupHasPollersSustainedDuration": {dynamicproperties.MatchingIsolationGroupHasPollersSustainedDuration, time.Duration(39)},
		"IsolationGroupNoPollersSustainedDuration":  {dynamicproperties.MatchingIsolationGroupNoPollersSustainedDuration, time.Duration(40)},
		"IsolationGroupsPerPartition":               {dynamicproperties.MatchingIsolationGroupsPerPartition, 41},
		"EnableReturnAllTaskListKinds":              {dynamicproperties.MatchingEnableReturnAllTaskListKinds, true},
	}
	client := dynamicconfig.NewInMemoryClient()
	for fieldName, expected := range fields {
		if expected.key != nil {
			err := client.UpdateValue(expected.key, expected.value)
			if err != nil {
				t.Errorf("Failed to update config for %s: %s", fieldName, err)
			}
		}
	}
	dc := dynamicconfig.NewCollection(client, testlogger.New(t))

	config := NewConfig(dc, hostname, isolationGroupsHelper)

	assertFieldsMatch(t, *config, fields)
}

func assertFieldsMatch(t *testing.T, config interface{}, fields map[string]configTestCase) {
	configType := reflect.ValueOf(config)

	for i := 0; i < configType.NumField(); i++ {
		f := configType.Field(i)
		fieldName := configType.Type().Field(i).Name

		if expected, ok := fields[fieldName]; ok {
			actual := getValue(&f)
			if f.Kind() == reflect.Slice {
				assert.ElementsMatch(t, expected.value, actual, "Incorrect value for field: %s", fieldName)
			} else {
				assert.Equal(t, expected.value, actual, "Incorrect value for field: %s", fieldName)
			}

		} else {
			t.Errorf("Unknown property on Config: %s", fieldName)
		}
	}
}

func getValue(f *reflect.Value) interface{} {
	switch f.Kind() {
	case reflect.Func:
		switch fn := f.Interface().(type) {
		case dynamicproperties.IntPropertyFn:
			return fn()
		case dynamicproperties.IntPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicproperties.IntPropertyFnWithTaskListInfoFilters:
			return fn("domain", "tasklist", int(types.TaskListTypeDecision))
		case dynamicproperties.BoolPropertyFn:
			return fn()
		case dynamicproperties.BoolPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicproperties.BoolPropertyFnWithDomainIDFilter:
			return fn("domain")
		case dynamicproperties.BoolPropertyFnWithTaskListInfoFilters:
			return fn("domain", "tasklist", int(types.TaskListTypeDecision))
		case dynamicproperties.DurationPropertyFn:
			return fn()
		case dynamicproperties.DurationPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicproperties.DurationPropertyFnWithTaskListInfoFilters:
			return fn("domain", "tasklist", int(types.TaskListTypeDecision))
		case dynamicproperties.FloatPropertyFn:
			return fn()
		case dynamicproperties.MapPropertyFn:
			return fn()
		case dynamicproperties.StringPropertyFn:
			return fn()
		case dynamicproperties.FloatPropertyFnWithTaskListInfoFilters:
			return fn("domain", "tasklist", int(types.TaskListTypeDecision))
		case func() []string:
			return fn()
		default:
			panic("Unable to handle type: " + f.Type().Name())
		}
	default:
		return f.Interface()
	}
}

func isolationGroupsHelper() []string {
	return []string{"zone-1", "zone-2"}
}
