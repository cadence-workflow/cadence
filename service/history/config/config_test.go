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
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

type configTestCase struct {
	key   dynamicconfig.Key
	value interface{}
}

func TestNewConfig(t *testing.T) {
	hostname := "hostname"
	numberOfShards := 8192
	maxMessageSize := 1024
	isAdvancedVisConfigExist := true
	fields := map[string]configTestCase{
		"NumberOfShards":                                       {nil, numberOfShards},
		"IsAdvancedVisConfigExist":                             {nil, isAdvancedVisConfigExist},
		"RPS":                                                  {dynamicconfig.HistoryRPS, 1},
		"MaxIDLengthWarnLimit":                                 {dynamicconfig.MaxIDLengthWarnLimit, 2},
		"DomainNameMaxLength":                                  {dynamicconfig.DomainNameMaxLength, 3},
		"IdentityMaxLength":                                    {dynamicconfig.IdentityMaxLength, 4},
		"WorkflowIDMaxLength":                                  {dynamicconfig.WorkflowIDMaxLength, 5},
		"SignalNameMaxLength":                                  {dynamicconfig.SignalNameMaxLength, 6},
		"WorkflowTypeMaxLength":                                {dynamicconfig.WorkflowTypeMaxLength, 7},
		"RequestIDMaxLength":                                   {dynamicconfig.RequestIDMaxLength, 8},
		"TaskListNameMaxLength":                                {dynamicconfig.TaskListNameMaxLength, 9},
		"ActivityIDMaxLength":                                  {dynamicconfig.ActivityIDMaxLength, 10},
		"ActivityTypeMaxLength":                                {dynamicconfig.ActivityTypeMaxLength, 11},
		"MarkerNameMaxLength":                                  {dynamicconfig.MarkerNameMaxLength, 12},
		"TimerIDMaxLength":                                     {dynamicconfig.TimerIDMaxLength, 13},
		"PersistenceMaxQPS":                                    {dynamicconfig.HistoryPersistenceMaxQPS, 14},
		"PersistenceGlobalMaxQPS":                              {dynamicconfig.HistoryPersistenceGlobalMaxQPS, 15},
		"EnableVisibilitySampling":                             {dynamicconfig.EnableVisibilitySampling, true},
		"EnableReadFromClosedExecutionV2":                      {dynamicconfig.EnableReadFromClosedExecutionV2, true},
		"VisibilityOpenMaxQPS":                                 {dynamicconfig.HistoryVisibilityOpenMaxQPS, 16},
		"VisibilityClosedMaxQPS":                               {dynamicconfig.HistoryVisibilityClosedMaxQPS, 17},
		"WriteVisibilityStoreName":                             {dynamicconfig.WriteVisibilityStoreName, "es"},
		"EmitShardDiffLog":                                     {dynamicconfig.EmitShardDiffLog, true},
		"MaxAutoResetPoints":                                   {dynamicconfig.HistoryMaxAutoResetPoints, 18},
		"ThrottledLogRPS":                                      {dynamicconfig.HistoryThrottledLogRPS, 19},
		"EnableStickyQuery":                                    {dynamicconfig.EnableStickyQuery, true},
		"ShutdownDrainDuration":                                {dynamicconfig.HistoryShutdownDrainDuration, time.Second},
		"WorkflowDeletionJitterRange":                          {dynamicconfig.WorkflowDeletionJitterRange, 20},
		"DeleteHistoryEventContextTimeout":                     {dynamicconfig.DeleteHistoryEventContextTimeout, 21},
		"MaxResponseSize":                                      {nil, maxMessageSize},
		"HistoryCacheInitialSize":                              {dynamicconfig.HistoryCacheInitialSize, 22},
		"HistoryCacheMaxSize":                                  {dynamicconfig.HistoryCacheMaxSize, 23},
		"HistoryCacheTTL":                                      {dynamicconfig.HistoryCacheTTL, time.Second},
		"EventsCacheInitialCount":                              {dynamicconfig.EventsCacheInitialCount, 24},
		"EventsCacheMaxCount":                                  {dynamicconfig.EventsCacheMaxCount, 25},
		"EventsCacheMaxSize":                                   {dynamicconfig.EventsCacheMaxSize, 26},
		"EventsCacheTTL":                                       {dynamicconfig.EventsCacheTTL, time.Second},
		"EventsCacheGlobalEnable":                              {dynamicconfig.EventsCacheGlobalEnable, true},
		"EventsCacheGlobalInitialCount":                        {dynamicconfig.EventsCacheGlobalInitialCount, 27},
		"EventsCacheGlobalMaxCount":                            {dynamicconfig.EventsCacheGlobalMaxCount, 28},
		"RangeSizeBits":                                        {nil, uint(20)},
		"AcquireShardInterval":                                 {dynamicconfig.AcquireShardInterval, time.Second},
		"AcquireShardConcurrency":                              {dynamicconfig.AcquireShardConcurrency, 29},
		"StandbyClusterDelay":                                  {dynamicconfig.StandbyClusterDelay, time.Second},
		"StandbyTaskMissingEventsResendDelay":                  {dynamicconfig.StandbyTaskMissingEventsResendDelay, time.Second},
		"StandbyTaskMissingEventsDiscardDelay":                 {dynamicconfig.StandbyTaskMissingEventsDiscardDelay, time.Second},
		"TaskProcessRPS":                                       {dynamicconfig.TaskProcessRPS, 30},
		"TaskSchedulerType":                                    {dynamicconfig.TaskSchedulerType, 31},
		"TaskSchedulerWorkerCount":                             {dynamicconfig.TaskSchedulerWorkerCount, 32},
		"TaskSchedulerQueueSize":                               {dynamicconfig.TaskSchedulerQueueSize, 34},
		"TaskSchedulerDispatcherCount":                         {dynamicconfig.TaskSchedulerDispatcherCount, 35},
		"TaskSchedulerRoundRobinWeights":                       {dynamicconfig.TaskSchedulerRoundRobinWeights, map[string]interface{}{"key": 1}},
		"TaskSchedulerDomainRoundRobinWeights":                 {dynamicconfig.TaskSchedulerDomainRoundRobinWeights, map[string]interface{}{"key": 2}},
		"TaskSchedulerEnableMigration":                         {dynamicconfig.TaskSchedulerEnableMigration, true},
		"TaskSchedulerMigrationRatio":                          {dynamicconfig.TaskSchedulerMigrationRatio, 36},
		"TaskCriticalRetryCount":                               {dynamicconfig.TaskCriticalRetryCount, 37},
		"ActiveTaskRedispatchInterval":                         {dynamicconfig.ActiveTaskRedispatchInterval, time.Second},
		"StandbyTaskRedispatchInterval":                        {dynamicconfig.StandbyTaskRedispatchInterval, time.Second},
		"StandbyTaskReReplicationContextTimeout":               {dynamicconfig.StandbyTaskReReplicationContextTimeout, time.Second},
		"EnableDropStuckTaskByDomainID":                        {dynamicconfig.EnableDropStuckTaskByDomainID, true},
		"ResurrectionCheckMinDelay":                            {dynamicconfig.ResurrectionCheckMinDelay, time.Second},
		"QueueProcessorEnableSplit":                            {dynamicconfig.QueueProcessorEnableSplit, true},
		"QueueProcessorSplitMaxLevel":                          {dynamicconfig.QueueProcessorSplitMaxLevel, 38},
		"QueueProcessorEnableRandomSplitByDomainID":            {dynamicconfig.QueueProcessorEnableRandomSplitByDomainID, true},
		"QueueProcessorRandomSplitProbability":                 {dynamicconfig.QueueProcessorRandomSplitProbability, 1.0},
		"QueueProcessorEnablePendingTaskSplitByDomainID":       {dynamicconfig.QueueProcessorEnablePendingTaskSplitByDomainID, true},
		"QueueProcessorPendingTaskSplitThreshold":              {dynamicconfig.QueueProcessorPendingTaskSplitThreshold, map[string]interface{}{"a": 100}},
		"QueueProcessorEnableStuckTaskSplitByDomainID":         {dynamicconfig.QueueProcessorEnableStuckTaskSplitByDomainID, true},
		"QueueProcessorStuckTaskSplitThreshold":                {dynamicconfig.QueueProcessorStuckTaskSplitThreshold, map[string]interface{}{"b": 1}},
		"QueueProcessorSplitLookAheadDurationByDomainID":       {dynamicconfig.QueueProcessorSplitLookAheadDurationByDomainID, time.Second},
		"QueueProcessorPollBackoffInterval":                    {dynamicconfig.QueueProcessorPollBackoffInterval, time.Second},
		"QueueProcessorPollBackoffIntervalJitterCoefficient":   {dynamicconfig.QueueProcessorPollBackoffIntervalJitterCoefficient, 1.0},
		"QueueProcessorEnableLoadQueueStates":                  {dynamicconfig.QueueProcessorEnableLoadQueueStates, true},
		"QueueProcessorEnableGracefulSyncShutdown":             {dynamicconfig.QueueProcessorEnableGracefulSyncShutdown, true},
		"QueueProcessorEnablePersistQueueStates":               {dynamicconfig.QueueProcessorEnablePersistQueueStates, true},
		"TimerTaskBatchSize":                                   {dynamicconfig.TimerTaskBatchSize, 39},
		"TimerTaskDeleteBatchSize":                             {dynamicconfig.TimerTaskDeleteBatchSize, 40},
		"TimerProcessorGetFailureRetryCount":                   {dynamicconfig.TimerProcessorGetFailureRetryCount, 41},
		"TimerProcessorCompleteTimerFailureRetryCount":         {dynamicconfig.TimerProcessorCompleteTimerFailureRetryCount, 42},
		"TimerProcessorUpdateAckInterval":                      {dynamicconfig.TimerProcessorUpdateAckInterval, time.Second},
		"TimerProcessorUpdateAckIntervalJitterCoefficient":     {dynamicconfig.TimerProcessorUpdateAckIntervalJitterCoefficient, 2.0},
		"TimerProcessorCompleteTimerInterval":                  {dynamicconfig.TimerProcessorCompleteTimerInterval, time.Second},
		"TimerProcessorFailoverMaxStartJitterInterval":         {dynamicconfig.TimerProcessorFailoverMaxStartJitterInterval, time.Second},
		"TimerProcessorFailoverMaxPollRPS":                     {dynamicconfig.TimerProcessorFailoverMaxPollRPS, 43},
		"TimerProcessorMaxPollRPS":                             {dynamicconfig.TimerProcessorMaxPollRPS, 44},
		"TimerProcessorMaxPollInterval":                        {dynamicconfig.TimerProcessorMaxPollInterval, time.Second},
		"TimerProcessorMaxPollIntervalJitterCoefficient":       {dynamicconfig.TimerProcessorMaxPollIntervalJitterCoefficient, 3.0},
		"TimerProcessorSplitQueueInterval":                     {dynamicconfig.TimerProcessorSplitQueueInterval, time.Second},
		"TimerProcessorSplitQueueIntervalJitterCoefficient":    {dynamicconfig.TimerProcessorSplitQueueIntervalJitterCoefficient, 4.0},
		"TimerProcessorMaxRedispatchQueueSize":                 {dynamicconfig.TimerProcessorMaxRedispatchQueueSize, 45},
		"TimerProcessorMaxTimeShift":                           {dynamicconfig.TimerProcessorMaxTimeShift, time.Second},
		"TimerProcessorHistoryArchivalSizeLimit":               {dynamicconfig.TimerProcessorHistoryArchivalSizeLimit, 46},
		"TimerProcessorArchivalTimeLimit":                      {dynamicconfig.TimerProcessorArchivalTimeLimit, time.Second},
		"TransferTaskBatchSize":                                {dynamicconfig.TransferTaskBatchSize, 47},
		"TransferTaskDeleteBatchSize":                          {dynamicconfig.TransferTaskDeleteBatchSize, 48},
		"TransferProcessorCompleteTransferFailureRetryCount":   {dynamicconfig.TransferProcessorCompleteTransferFailureRetryCount, 49},
		"TransferProcessorFailoverMaxStartJitterInterval":      {dynamicconfig.TransferProcessorFailoverMaxStartJitterInterval, time.Second},
		"TransferProcessorFailoverMaxPollRPS":                  {dynamicconfig.TransferProcessorFailoverMaxPollRPS, 50},
		"TransferProcessorMaxPollRPS":                          {dynamicconfig.TransferProcessorMaxPollRPS, 51},
		"TransferProcessorMaxPollInterval":                     {dynamicconfig.TransferProcessorMaxPollInterval, time.Second},
		"TransferProcessorMaxPollIntervalJitterCoefficient":    {dynamicconfig.TransferProcessorMaxPollIntervalJitterCoefficient, 8.0},
		"TransferProcessorSplitQueueInterval":                  {dynamicconfig.TransferProcessorSplitQueueInterval, time.Second},
		"TransferProcessorSplitQueueIntervalJitterCoefficient": {dynamicconfig.TransferProcessorSplitQueueIntervalJitterCoefficient, 6.0},
		"TransferProcessorUpdateAckInterval":                   {dynamicconfig.TransferProcessorUpdateAckInterval, time.Second},
		"TransferProcessorUpdateAckIntervalJitterCoefficient":  {dynamicconfig.TransferProcessorUpdateAckIntervalJitterCoefficient, 7.0},
		"TransferProcessorCompleteTransferInterval":            {dynamicconfig.TransferProcessorCompleteTransferInterval, time.Second},
		"TransferProcessorMaxRedispatchQueueSize":              {dynamicconfig.TransferProcessorMaxRedispatchQueueSize, 52},
		"TransferProcessorEnableValidator":                     {dynamicconfig.TransferProcessorEnableValidator, true},
		"TransferProcessorValidationInterval":                  {dynamicconfig.TransferProcessorValidationInterval, time.Second},
		"TransferProcessorVisibilityArchivalTimeLimit":         {dynamicconfig.TransferProcessorVisibilityArchivalTimeLimit, time.Second},
		"ReplicatorTaskDeleteBatchSize":                        {dynamicconfig.ReplicatorTaskDeleteBatchSize, 53},
		"ReplicatorReadTaskMaxRetryCount":                      {dynamicconfig.ReplicatorReadTaskMaxRetryCount, 54},
		"ReplicatorProcessorFetchTasksBatchSize":               {dynamicconfig.ReplicatorTaskBatchSize, 55},
		"ReplicatorUpperLatency":                               {dynamicconfig.ReplicatorUpperLatency, time.Second},
		"ReplicatorCacheCapacity":                              {dynamicconfig.ReplicatorCacheCapacity, 56},
		"ExecutionMgrNumConns":                                 {dynamicconfig.ExecutionMgrNumConns, 57},
		"HistoryMgrNumConns":                                   {dynamicconfig.HistoryMgrNumConns, 58},
		"MaximumBufferedEventsBatch":                           {dynamicconfig.MaximumBufferedEventsBatch, 59},
		"MaximumSignalsPerExecution":                           {dynamicconfig.MaximumSignalsPerExecution, 60},
		"ShardUpdateMinInterval":                               {dynamicconfig.ShardUpdateMinInterval, time.Second},
		"ShardSyncMinInterval":                                 {dynamicconfig.ShardSyncMinInterval, time.Second},
		"ShardSyncTimerJitterCoefficient":                      {dynamicconfig.TransferProcessorMaxPollIntervalJitterCoefficient, 8.0},
		"LongPollExpirationInterval":                           {dynamicconfig.HistoryLongPollExpirationInterval, time.Second},
		"EventEncodingType":                                    {dynamicconfig.DefaultEventEncoding, "eventEncodingType"},
		"EnableParentClosePolicy":                              {dynamicconfig.EnableParentClosePolicy, true},
		"EnableParentClosePolicyWorker":                        {dynamicconfig.EnableParentClosePolicyWorker, true},
		"ParentClosePolicyThreshold":                           {dynamicconfig.ParentClosePolicyThreshold, 61},
		"ParentClosePolicyBatchSize":                           {dynamicconfig.ParentClosePolicyBatchSize, 62},
		"NumParentClosePolicySystemWorkflows":                  {dynamicconfig.NumParentClosePolicySystemWorkflows, 63},
		"NumArchiveSystemWorkflows":                            {dynamicconfig.NumArchiveSystemWorkflows, 64},
		"ArchiveRequestRPS":                                    {dynamicconfig.ArchiveRequestRPS, 65},
		"ArchiveInlineHistoryRPS":                              {dynamicconfig.ArchiveInlineHistoryRPS, 66},
		"ArchiveInlineHistoryGlobalRPS":                        {dynamicconfig.ArchiveInlineHistoryGlobalRPS, 67},
		"ArchiveInlineVisibilityRPS":                           {dynamicconfig.ArchiveInlineVisibilityRPS, 68},
		"ArchiveInlineVisibilityGlobalRPS":                     {dynamicconfig.ArchiveInlineVisibilityGlobalRPS, 69},
		"AllowArchivingIncompleteHistory":                      {dynamicconfig.AllowArchivingIncompleteHistory, true},
		"BlobSizeLimitError":                                   {dynamicconfig.BlobSizeLimitError, 70},
		"BlobSizeLimitWarn":                                    {dynamicconfig.BlobSizeLimitWarn, 71},
		"HistorySizeLimitError":                                {dynamicconfig.HistorySizeLimitError, 72},
		"HistorySizeLimitWarn":                                 {dynamicconfig.HistorySizeLimitWarn, 73},
		"HistoryCountLimitError":                               {dynamicconfig.HistoryCountLimitError, 74},
		"HistoryCountLimitWarn":                                {dynamicconfig.HistoryCountLimitWarn, 75},
		"PendingActivitiesCountLimitError":                     {dynamicconfig.PendingActivitiesCountLimitError, 76},
		"PendingActivitiesCountLimitWarn":                      {dynamicconfig.PendingActivitiesCountLimitWarn, 77},
		"PendingActivityValidationEnabled":                     {dynamicconfig.EnablePendingActivityValidation, true},
		"EnableQueryAttributeValidation":                       {dynamicconfig.EnableQueryAttributeValidation, true},
		"ValidSearchAttributes":                                {dynamicconfig.ValidSearchAttributes, map[string]interface{}{"key": 1}},
		"SearchAttributesNumberOfKeysLimit":                    {dynamicconfig.SearchAttributesNumberOfKeysLimit, 78},
		"SearchAttributesSizeOfValueLimit":                     {dynamicconfig.SearchAttributesSizeOfValueLimit, 79},
		"SearchAttributesTotalSizeLimit":                       {dynamicconfig.SearchAttributesTotalSizeLimit, 80},
		"StickyTTL":                                            {dynamicconfig.StickyTTL, time.Second},
		"DecisionHeartbeatTimeout":                             {dynamicconfig.DecisionHeartbeatTimeout, time.Second},
		"MaxDecisionStartToCloseSeconds":                       {dynamicconfig.MaxDecisionStartToCloseSeconds, 81},
		"DecisionRetryCriticalAttempts":                        {dynamicconfig.DecisionRetryCriticalAttempts, 82},
		"DecisionRetryMaxAttempts":                             {dynamicconfig.DecisionRetryMaxAttempts, 83},
		"NormalDecisionScheduleToStartMaxAttempts":             {dynamicconfig.NormalDecisionScheduleToStartMaxAttempts, 84},
		"NormalDecisionScheduleToStartTimeout":                 {dynamicconfig.NormalDecisionScheduleToStartTimeout, time.Second},
		"ReplicationTaskFetcherParallelism":                    {dynamicconfig.ReplicationTaskFetcherParallelism, 85},
		"ReplicationTaskFetcherAggregationInterval":            {dynamicconfig.ReplicationTaskFetcherAggregationInterval, time.Second},
		"ReplicationTaskFetcherTimerJitterCoefficient":         {dynamicconfig.ReplicationTaskFetcherTimerJitterCoefficient, 9.0},
		"ReplicationTaskFetcherErrorRetryWait":                 {dynamicconfig.ReplicationTaskFetcherErrorRetryWait, time.Second},
		"ReplicationTaskFetcherServiceBusyWait":                {dynamicconfig.ReplicationTaskFetcherServiceBusyWait, time.Second},
		"ReplicationTaskProcessorErrorRetryWait":               {dynamicconfig.ReplicationTaskProcessorErrorRetryWait, time.Second},
		"ReplicationTaskProcessorErrorRetryMaxAttempts":        {dynamicconfig.ReplicationTaskProcessorErrorRetryMaxAttempts, 86},
		"ReplicationTaskProcessorErrorSecondRetryWait":         {dynamicconfig.ReplicationTaskProcessorErrorSecondRetryWait, time.Second},
		"ReplicationTaskProcessorErrorSecondRetryExpiration":   {dynamicconfig.ReplicationTaskProcessorErrorSecondRetryExpiration, time.Second},
		"ReplicationTaskProcessorErrorSecondRetryMaxWait":      {dynamicconfig.ReplicationTaskProcessorErrorSecondRetryMaxWait, time.Second},
		"ReplicationTaskProcessorNoTaskRetryWait":              {dynamicconfig.ReplicationTaskProcessorNoTaskInitialWait, time.Second},
		"ReplicationTaskProcessorCleanupInterval":              {dynamicconfig.ReplicationTaskProcessorCleanupInterval, time.Second},
		"ReplicationTaskProcessorCleanupJitterCoefficient":     {dynamicconfig.ReplicationTaskProcessorCleanupJitterCoefficient, 10.0},
		"ReplicationTaskProcessorStartWait":                    {dynamicconfig.ReplicationTaskProcessorStartWait, time.Second},
		"ReplicationTaskProcessorStartWaitJitterCoefficient":   {dynamicconfig.ReplicationTaskProcessorStartWaitJitterCoefficient, 11.0},
		"ReplicationTaskProcessorHostQPS":                      {dynamicconfig.ReplicationTaskProcessorHostQPS, 12.0},
		"ReplicationTaskProcessorShardQPS":                     {dynamicconfig.ReplicationTaskProcessorShardQPS, 13.0},
		"ReplicationTaskGenerationQPS":                         {dynamicconfig.ReplicationTaskGenerationQPS, 14.0},
		"EnableReplicationTaskGeneration":                      {dynamicconfig.EnableReplicationTaskGeneration, true},
		"EnableRecordWorkflowExecutionUninitialized":           {dynamicconfig.EnableRecordWorkflowExecutionUninitialized, true},
		"WorkflowIDExternalRPS":                                {dynamicconfig.WorkflowIDExternalRPS, 87},
		"WorkflowIDInternalRPS":                                {dynamicconfig.WorkflowIDInternalRPS, 88},
		"EnableConsistentQuery":                                {dynamicconfig.EnableConsistentQuery, true},
		"EnableConsistentQueryByDomain":                        {dynamicconfig.EnableConsistentQueryByDomain, true},
		"MaxBufferedQueryCount":                                {dynamicconfig.MaxBufferedQueryCount, 89},
		"EnableContextHeaderInVisibility":                      {dynamicconfig.EnableContextHeaderInVisibility, true},
		"EnableCrossClusterOperationsForDomain":                {dynamicconfig.EnableCrossClusterOperationsForDomain, true},
		"MutableStateChecksumGenProbability":                   {dynamicconfig.MutableStateChecksumGenProbability, 90},
		"MutableStateChecksumVerifyProbability":                {dynamicconfig.MutableStateChecksumVerifyProbability, 91},
		"MutableStateChecksumInvalidateBefore":                 {dynamicconfig.MutableStateChecksumInvalidateBefore, 15.0},
		"EnableRetryForChecksumFailure":                        {dynamicconfig.EnableRetryForChecksumFailure, true},
		"EnableHistoryCorruptionCheck":                         {dynamicconfig.EnableHistoryCorruptionCheck, true},
		"NotifyFailoverMarkerInterval":                         {dynamicconfig.NotifyFailoverMarkerInterval, time.Second},
		"NotifyFailoverMarkerTimerJitterCoefficient":           {dynamicconfig.NotifyFailoverMarkerTimerJitterCoefficient, 16.0},
		"EnableGracefulFailover":                               {dynamicconfig.EnableGracefulFailover, true},
		"EnableActivityLocalDispatchByDomain":                  {dynamicconfig.EnableActivityLocalDispatchByDomain, true},
		"MaxActivityCountDispatchByDomain":                     {dynamicconfig.MaxActivityCountDispatchByDomain, 92},
		"ActivityMaxScheduleToStartTimeoutForRetry":            {dynamicconfig.ActivityMaxScheduleToStartTimeoutForRetry, time.Second},
		"EnableDebugMode":                                      {dynamicconfig.EnableDebugMode, true},
		"EnableTaskInfoLogByDomainID":                          {dynamicconfig.HistoryEnableTaskInfoLogByDomainID, true},
		"EnableTimerDebugLogByDomainID":                        {dynamicconfig.EnableTimerDebugLogByDomainID, true},
		"SampleLoggingRate":                                    {dynamicconfig.SampleLoggingRate, 93},
		"EnableShardIDMetrics":                                 {dynamicconfig.EnableShardIDMetrics, true},
		"LargeShardHistorySizeMetricThreshold":                 {dynamicconfig.LargeShardHistorySizeMetricThreshold, 94},
		"LargeShardHistoryEventMetricThreshold":                {dynamicconfig.LargeShardHistoryEventMetricThreshold, 95},
		"LargeShardHistoryBlobMetricThreshold":                 {dynamicconfig.LargeShardHistoryBlobMetricThreshold, 96},
		"EnableStrongIdempotency":                              {dynamicconfig.EnableStrongIdempotency, true},
		"EnableStrongIdempotencySanityCheck":                   {dynamicconfig.EnableStrongIdempotencySanityCheck, true},
		"GlobalRatelimiterNewDataWeight":                       {dynamicconfig.HistoryGlobalRatelimiterNewDataWeight, 17.0},
		"GlobalRatelimiterUpdateInterval":                      {dynamicconfig.GlobalRatelimiterUpdateInterval, time.Second},
		"GlobalRatelimiterDecayAfter":                          {dynamicconfig.HistoryGlobalRatelimiterDecayAfter, time.Second},
		"GlobalRatelimiterGCAfter":                             {dynamicconfig.HistoryGlobalRatelimiterGCAfter, time.Second},
		"TaskSchedulerGlobalDomainRPS":                         {dynamicconfig.TaskSchedulerGlobalDomainRPS, 97},
		"TaskSchedulerEnableRateLimiterShadowMode":             {dynamicconfig.TaskSchedulerEnableRateLimiterShadowMode, false},
		"TaskSchedulerEnableRateLimiter":                       {dynamicconfig.TaskSchedulerEnableRateLimiter, true},
		"HostName":                                             {nil, hostname},
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

	config := New(dc, numberOfShards, maxMessageSize, isAdvancedVisConfigExist, hostname)

	assertFieldsMatch(t, *config, fields)
}

func TestNewForTest(t *testing.T) {
	cfg := NewForTest()
	assert.NotNil(t, cfg)
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
		case dynamicconfig.DurationPropertyFnWithDomainIDFilter:
			return fn("domain")
		case dynamicconfig.IntPropertyFnWithShardIDFilter:
			return fn(0)
		case dynamicconfig.StringPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicconfig.DurationPropertyFnWithShardIDFilter:
			return fn(0)
		case dynamicconfig.FloatPropertyFnWithShardIDFilter:
			return fn(0)
		case dynamicconfig.BoolPropertyFnWithDomainIDAndWorkflowIDFilter:
			return fn("domain", "workflowID")
		case dynamicconfig.MapPropertyFnWithDomainFilter:
			return fn("domain")
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
