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
	"github.com/uber/cadence/common/dynamicconfig/collection"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

type configTestCase struct {
	key   dynamicconfig.Key
	value interface{}
}

func TestNewConfig(t *testing.T) {
	hostname := "hostname"
	fields := map[string]configTestCase{
		"PersistenceMaxQPS":                    {dynamicconfig.MatchingPersistenceMaxQPS, 1},
		"PersistenceGlobalMaxQPS":              {dynamicconfig.MatchingPersistenceGlobalMaxQPS, 2},
		"EnableSyncMatch":                      {dynamicconfig.MatchingEnableSyncMatch, true},
		"UserRPS":                              {dynamicconfig.MatchingUserRPS, 3},
		"WorkerRPS":                            {dynamicconfig.MatchingWorkerRPS, 4},
		"DomainUserRPS":                        {dynamicconfig.MatchingDomainUserRPS, 5},
		"DomainWorkerRPS":                      {dynamicconfig.MatchingDomainWorkerRPS, 6},
		"RangeSize":                            {nil, int64(100000)},
		"ReadRangeSize":                        {dynamicconfig.MatchingReadRangeSize, 50000},
		"GetTasksBatchSize":                    {dynamicconfig.MatchingGetTasksBatchSize, 7},
		"UpdateAckInterval":                    {dynamicconfig.MatchingUpdateAckInterval, time.Duration(8)},
		"IdleTasklistCheckInterval":            {dynamicconfig.MatchingIdleTasklistCheckInterval, time.Duration(9)},
		"MaxTasklistIdleTime":                  {dynamicconfig.MaxTasklistIdleTime, time.Duration(10)},
		"LongPollExpirationInterval":           {dynamicconfig.MatchingLongPollExpirationInterval, time.Duration(11)},
		"MinTaskThrottlingBurstSize":           {dynamicconfig.MatchingMinTaskThrottlingBurstSize, 12},
		"MaxTaskDeleteBatchSize":               {dynamicconfig.MatchingMaxTaskDeleteBatchSize, 13},
		"OutstandingTaskAppendsThreshold":      {dynamicconfig.MatchingOutstandingTaskAppendsThreshold, 14},
		"MaxTaskBatchSize":                     {dynamicconfig.MatchingMaxTaskBatchSize, 15},
		"ThrottledLogRPS":                      {dynamicconfig.MatchingThrottledLogRPS, 16},
		"NumTasklistWritePartitions":           {dynamicconfig.MatchingNumTasklistWritePartitions, 17},
		"NumTasklistReadPartitions":            {dynamicconfig.MatchingNumTasklistReadPartitions, 18},
		"ForwarderMaxOutstandingPolls":         {dynamicconfig.MatchingForwarderMaxOutstandingPolls, 19},
		"ForwarderMaxOutstandingTasks":         {dynamicconfig.MatchingForwarderMaxOutstandingTasks, 20},
		"ForwarderMaxRatePerSecond":            {dynamicconfig.MatchingForwarderMaxRatePerSecond, 21},
		"ForwarderMaxChildrenPerNode":          {dynamicconfig.MatchingForwarderMaxChildrenPerNode, 22},
		"ShutdownDrainDuration":                {dynamicconfig.MatchingShutdownDrainDuration, time.Duration(23)},
		"EnableDebugMode":                      {dynamicconfig.EnableDebugMode, false},
		"EnableTaskInfoLogByDomainID":          {dynamicconfig.MatchingEnableTaskInfoLogByDomainID, true},
		"ActivityTaskSyncMatchWaitTime":        {dynamicconfig.MatchingActivityTaskSyncMatchWaitTime, time.Duration(24)},
		"EnableTasklistIsolation":              {dynamicconfig.EnableTasklistIsolation, false},
		"AsyncTaskDispatchTimeout":             {dynamicconfig.AsyncTaskDispatchTimeout, time.Duration(25)},
		"LocalPollWaitTime":                    {dynamicconfig.LocalPollWaitTime, time.Duration(10)},
		"LocalTaskWaitTime":                    {dynamicconfig.LocalTaskWaitTime, time.Duration(10)},
		"HostName":                             {nil, hostname},
		"TaskDispatchRPS":                      {nil, 100000.0},
		"TaskDispatchRPSTTL":                   {nil, time.Minute},
		"MaxTimeBetweenTaskDeletes":            {nil, time.Second},
		"AllIsolationGroups":                   {nil, []string{"zone-1", "zone-2"}},
		"EnableTasklistOwnershipGuard":         {dynamicconfig.MatchingEnableTasklistGuardAgainstOwnershipShardLoss, false},
		"EnableGetNumberOfPartitionsFromCache": {dynamicconfig.MatchingEnableGetNumberOfPartitionsFromCache, false},
		"PartitionUpscaleRPS":                  {dynamicconfig.MatchingPartitionUpscaleRPS, 30},
		"PartitionDownscaleFactor":             {dynamicconfig.MatchingPartitionDownscaleFactor, 31.0},
		"PartitionUpscaleSustainedDuration":    {dynamicconfig.MatchingPartitionUpscaleSustainedDuration, time.Duration(32)},
		"PartitionDownscaleSustainedDuration":  {dynamicconfig.MatchingPartitionDownscaleSustainedDuration, time.Duration(33)},
		"AdaptiveScalerUpdateInterval":         {dynamicconfig.MatchingAdaptiveScalerUpdateInterval, time.Duration(34)},
		"EnableAdaptiveScaler":                 {dynamicconfig.MatchingEnableAdaptiveScaler, true},
		"QPSTrackerInterval":                   {dynamicconfig.MatchingQPSTrackerInterval, 5 * time.Second},
		"EnableStandbyTaskCompletion":          {dynamicconfig.MatchingEnableStandbyTaskCompletion, false},
		"EnableClientAutoConfig":               {dynamicconfig.MatchingEnableClientAutoConfig, false},
		"TaskIsolationDuration":                {dynamicconfig.TaskIsolationDuration, time.Duration(35)},
		"TaskIsolationPollerWindow":            {dynamicconfig.TaskIsolationPollerWindow, time.Duration(36)},
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
	dc := collection.NewCollection(client, testlogger.New(t))

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
		case dynamicconfig.IntPropertyFn:
			return fn()
		case dynamicconfig.IntPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicconfig.IntPropertyFnWithTaskListInfoFilters:
			return fn("domain", "tasklist", int(types.TaskListTypeDecision))
		case dynamicconfig.BoolPropertyFn:
			return fn()
		case dynamicconfig.BoolPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicconfig.BoolPropertyFnWithDomainIDFilter:
			return fn("domain")
		case dynamicconfig.BoolPropertyFnWithTaskListInfoFilters:
			return fn("domain", "tasklist", int(types.TaskListTypeDecision))
		case dynamicconfig.DurationPropertyFn:
			return fn()
		case dynamicconfig.DurationPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicconfig.DurationPropertyFnWithTaskListInfoFilters:
			return fn("domain", "tasklist", int(types.TaskListTypeDecision))
		case dynamicconfig.FloatPropertyFn:
			return fn()
		case dynamicconfig.MapPropertyFn:
			return fn()
		case dynamicconfig.StringPropertyFn:
			return fn()
		case dynamicconfig.FloatPropertyFnWithTaskListInfoFilters:
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
