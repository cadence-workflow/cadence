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
	numberOfShards := 8192
	maxMessageSize := 1024
	isAdvancedVisConfigExist := true
	fields := map[string]configTestCase{
		"NumberOfShards":                                       {nil, numberOfShards},
		"IsAdvancedVisConfigExist":                             {nil, isAdvancedVisConfigExist},
		"RPS":                                                  {dynamicproperties.HistoryRPS, 1},
		"MaxIDLengthWarnLimit":                                 {dynamicproperties.MaxIDLengthWarnLimit, 2},
		"DomainNameMaxLength":                                  {dynamicproperties.DomainNameMaxLength, 3},
		"IdentityMaxLength":                                    {dynamicproperties.IdentityMaxLength, 4},
		"WorkflowIDMaxLength":                                  {dynamicproperties.WorkflowIDMaxLength, 5},
		"SignalNameMaxLength":                                  {dynamicproperties.SignalNameMaxLength, 6},
		"WorkflowTypeMaxLength":                                {dynamicproperties.WorkflowTypeMaxLength, 7},
		"RequestIDMaxLength":                                   {dynamicproperties.RequestIDMaxLength, 8},
		"TaskListNameMaxLength":                                {dynamicproperties.TaskListNameMaxLength, 9},
		"ActivityIDMaxLength":                                  {dynamicproperties.ActivityIDMaxLength, 10},
		"ActivityTypeMaxLength":                                {dynamicproperties.ActivityTypeMaxLength, 11},
		"MarkerNameMaxLength":                                  {dynamicproperties.MarkerNameMaxLength, 12},
		"TimerIDMaxLength":                                     {dynamicproperties.TimerIDMaxLength, 13},
		"PersistenceMaxQPS":                                    {dynamicproperties.HistoryPersistenceMaxQPS, 14},
		"PersistenceGlobalMaxQPS":                              {dynamicproperties.HistoryPersistenceGlobalMaxQPS, 15},
		"EnableVisibilitySampling":                             {dynamicproperties.EnableVisibilitySampling, true},
		"EnableReadFromClosedExecutionV2":                      {dynamicproperties.EnableReadFromClosedExecutionV2, true},
		"VisibilityOpenMaxQPS":                                 {dynamicproperties.HistoryVisibilityOpenMaxQPS, 16},
		"VisibilityClosedMaxQPS":                               {dynamicproperties.HistoryVisibilityClosedMaxQPS, 17},
		"WriteVisibilityStoreName":                             {dynamicproperties.WriteVisibilityStoreName, "es"},
		"EmitShardDiffLog":                                     {dynamicproperties.EmitShardDiffLog, true},
		"MaxAutoResetPoints":                                   {dynamicproperties.HistoryMaxAutoResetPoints, 18},
		"ThrottledLogRPS":                                      {dynamicproperties.HistoryThrottledLogRPS, 19},
		"EnableStickyQuery":                                    {dynamicproperties.EnableStickyQuery, true},
		"ShutdownDrainDuration":                                {dynamicproperties.HistoryShutdownDrainDuration, time.Second},
		"WorkflowDeletionJitterRange":                          {dynamicproperties.WorkflowDeletionJitterRange, 20},
		"DeleteHistoryEventContextTimeout":                     {dynamicproperties.DeleteHistoryEventContextTimeout, 21},
		"MaxResponseSize":                                      {nil, maxMessageSize},
		"HistoryCacheInitialSize":                              {dynamicproperties.HistoryCacheInitialSize, 22},
		"HistoryCacheMaxSize":                                  {dynamicproperties.HistoryCacheMaxSize, 23},
		"HistoryCacheTTL":                                      {dynamicproperties.HistoryCacheTTL, time.Second},
		"EnableSizeBasedHistoryExecutionCache":                 {dynamicproperties.EnableSizeBasedHistoryExecutionCache, true},
		"EventsCacheInitialCount":                              {dynamicproperties.EventsCacheInitialCount, 24},
		"EventsCacheMaxCount":                                  {dynamicproperties.EventsCacheMaxCount, 25},
		"EventsCacheMaxSize":                                   {dynamicproperties.EventsCacheMaxSize, 26},
		"EventsCacheTTL":                                       {dynamicproperties.EventsCacheTTL, time.Second},
		"EventsCacheGlobalEnable":                              {dynamicproperties.EventsCacheGlobalEnable, true},
		"EventsCacheGlobalInitialCount":                        {dynamicproperties.EventsCacheGlobalInitialCount, 27},
		"EventsCacheGlobalMaxCount":                            {dynamicproperties.EventsCacheGlobalMaxCount, 28},
		"EnableSizeBasedHistoryEventCache":                     {dynamicproperties.EnableSizeBasedHistoryEventCache, true},
		"RangeSizeBits":                                        {nil, uint(20)},
		"AcquireShardInterval":                                 {dynamicproperties.AcquireShardInterval, time.Second},
		"AcquireShardConcurrency":                              {dynamicproperties.AcquireShardConcurrency, 29},
		"StandbyClusterDelay":                                  {dynamicproperties.StandbyClusterDelay, time.Second},
		"StandbyTaskMissingEventsResendDelay":                  {dynamicproperties.StandbyTaskMissingEventsResendDelay, time.Second},
		"StandbyTaskMissingEventsDiscardDelay":                 {dynamicproperties.StandbyTaskMissingEventsDiscardDelay, time.Second},
		"TaskProcessRPS":                                       {dynamicproperties.TaskProcessRPS, 30},
		"TaskSchedulerType":                                    {dynamicproperties.TaskSchedulerType, 31},
		"TaskSchedulerWorkerCount":                             {dynamicproperties.TaskSchedulerWorkerCount, 32},
		"TaskSchedulerQueueSize":                               {dynamicproperties.TaskSchedulerQueueSize, 34},
		"TaskSchedulerDispatcherCount":                         {dynamicproperties.TaskSchedulerDispatcherCount, 35},
		"TaskSchedulerRoundRobinWeights":                       {dynamicproperties.TaskSchedulerRoundRobinWeights, map[string]interface{}{"key": 1}},
		"TaskSchedulerDomainRoundRobinWeights":                 {dynamicproperties.TaskSchedulerDomainRoundRobinWeights, map[string]interface{}{"key": 2}},
		"TaskSchedulerEnableMigration":                         {dynamicproperties.TaskSchedulerEnableMigration, true},
		"TaskSchedulerMigrationRatio":                          {dynamicproperties.TaskSchedulerMigrationRatio, 36},
		"TaskCriticalRetryCount":                               {dynamicproperties.TaskCriticalRetryCount, 37},
		"ActiveTaskRedispatchInterval":                         {dynamicproperties.ActiveTaskRedispatchInterval, time.Second},
		"StandbyTaskRedispatchInterval":                        {dynamicproperties.StandbyTaskRedispatchInterval, time.Second},
		"StandbyTaskReReplicationContextTimeout":               {dynamicproperties.StandbyTaskReReplicationContextTimeout, time.Second},
		"EnableDropStuckTaskByDomainID":                        {dynamicproperties.EnableDropStuckTaskByDomainID, true},
		"ResurrectionCheckMinDelay":                            {dynamicproperties.ResurrectionCheckMinDelay, time.Second},
		"QueueProcessorEnableSplit":                            {dynamicproperties.QueueProcessorEnableSplit, true},
		"QueueProcessorSplitMaxLevel":                          {dynamicproperties.QueueProcessorSplitMaxLevel, 38},
		"QueueProcessorEnableRandomSplitByDomainID":            {dynamicproperties.QueueProcessorEnableRandomSplitByDomainID, true},
		"QueueProcessorRandomSplitProbability":                 {dynamicproperties.QueueProcessorRandomSplitProbability, 1.0},
		"QueueProcessorEnablePendingTaskSplitByDomainID":       {dynamicproperties.QueueProcessorEnablePendingTaskSplitByDomainID, true},
		"QueueProcessorPendingTaskSplitThreshold":              {dynamicproperties.QueueProcessorPendingTaskSplitThreshold, map[string]interface{}{"a": 100}},
		"QueueProcessorEnableStuckTaskSplitByDomainID":         {dynamicproperties.QueueProcessorEnableStuckTaskSplitByDomainID, true},
		"QueueProcessorStuckTaskSplitThreshold":                {dynamicproperties.QueueProcessorStuckTaskSplitThreshold, map[string]interface{}{"b": 1}},
		"QueueProcessorSplitLookAheadDurationByDomainID":       {dynamicproperties.QueueProcessorSplitLookAheadDurationByDomainID, time.Second},
		"QueueProcessorPollBackoffInterval":                    {dynamicproperties.QueueProcessorPollBackoffInterval, time.Second},
		"QueueProcessorPollBackoffIntervalJitterCoefficient":   {dynamicproperties.QueueProcessorPollBackoffIntervalJitterCoefficient, 1.0},
		"QueueProcessorEnableLoadQueueStates":                  {dynamicproperties.QueueProcessorEnableLoadQueueStates, true},
		"QueueProcessorEnableGracefulSyncShutdown":             {dynamicproperties.QueueProcessorEnableGracefulSyncShutdown, true},
		"QueueProcessorEnablePersistQueueStates":               {dynamicproperties.QueueProcessorEnablePersistQueueStates, true},
		"TimerTaskBatchSize":                                   {dynamicproperties.TimerTaskBatchSize, 39},
		"TimerTaskDeleteBatchSize":                             {dynamicproperties.TimerTaskDeleteBatchSize, 40},
		"TimerProcessorGetFailureRetryCount":                   {dynamicproperties.TimerProcessorGetFailureRetryCount, 41},
		"TimerProcessorCompleteTimerFailureRetryCount":         {dynamicproperties.TimerProcessorCompleteTimerFailureRetryCount, 42},
		"TimerProcessorUpdateAckInterval":                      {dynamicproperties.TimerProcessorUpdateAckInterval, time.Second},
		"TimerProcessorUpdateAckIntervalJitterCoefficient":     {dynamicproperties.TimerProcessorUpdateAckIntervalJitterCoefficient, 2.0},
		"TimerProcessorCompleteTimerInterval":                  {dynamicproperties.TimerProcessorCompleteTimerInterval, time.Second},
		"TimerProcessorFailoverMaxStartJitterInterval":         {dynamicproperties.TimerProcessorFailoverMaxStartJitterInterval, time.Second},
		"TimerProcessorFailoverMaxPollRPS":                     {dynamicproperties.TimerProcessorFailoverMaxPollRPS, 43},
		"TimerProcessorMaxPollRPS":                             {dynamicproperties.TimerProcessorMaxPollRPS, 44},
		"TimerProcessorMaxPollInterval":                        {dynamicproperties.TimerProcessorMaxPollInterval, time.Second},
		"TimerProcessorMaxPollIntervalJitterCoefficient":       {dynamicproperties.TimerProcessorMaxPollIntervalJitterCoefficient, 3.0},
		"TimerProcessorSplitQueueInterval":                     {dynamicproperties.TimerProcessorSplitQueueInterval, time.Second},
		"TimerProcessorSplitQueueIntervalJitterCoefficient":    {dynamicproperties.TimerProcessorSplitQueueIntervalJitterCoefficient, 4.0},
		"TimerProcessorMaxRedispatchQueueSize":                 {dynamicproperties.TimerProcessorMaxRedispatchQueueSize, 45},
		"TimerProcessorMaxTimeShift":                           {dynamicproperties.TimerProcessorMaxTimeShift, time.Second},
		"TimerProcessorHistoryArchivalSizeLimit":               {dynamicproperties.TimerProcessorHistoryArchivalSizeLimit, 46},
		"TimerProcessorArchivalTimeLimit":                      {dynamicproperties.TimerProcessorArchivalTimeLimit, time.Second},
		"TransferTaskBatchSize":                                {dynamicproperties.TransferTaskBatchSize, 47},
		"TransferTaskDeleteBatchSize":                          {dynamicproperties.TransferTaskDeleteBatchSize, 48},
		"TransferProcessorCompleteTransferFailureRetryCount":   {dynamicproperties.TransferProcessorCompleteTransferFailureRetryCount, 49},
		"TransferProcessorFailoverMaxStartJitterInterval":      {dynamicproperties.TransferProcessorFailoverMaxStartJitterInterval, time.Second},
		"TransferProcessorFailoverMaxPollRPS":                  {dynamicproperties.TransferProcessorFailoverMaxPollRPS, 50},
		"TransferProcessorMaxPollRPS":                          {dynamicproperties.TransferProcessorMaxPollRPS, 51},
		"TransferProcessorMaxPollInterval":                     {dynamicproperties.TransferProcessorMaxPollInterval, time.Second},
		"TransferProcessorMaxPollIntervalJitterCoefficient":    {dynamicproperties.TransferProcessorMaxPollIntervalJitterCoefficient, 8.0},
		"TransferProcessorSplitQueueInterval":                  {dynamicproperties.TransferProcessorSplitQueueInterval, time.Second},
		"TransferProcessorSplitQueueIntervalJitterCoefficient": {dynamicproperties.TransferProcessorSplitQueueIntervalJitterCoefficient, 6.0},
		"TransferProcessorUpdateAckInterval":                   {dynamicproperties.TransferProcessorUpdateAckInterval, time.Second},
		"TransferProcessorUpdateAckIntervalJitterCoefficient":  {dynamicproperties.TransferProcessorUpdateAckIntervalJitterCoefficient, 7.0},
		"TransferProcessorCompleteTransferInterval":            {dynamicproperties.TransferProcessorCompleteTransferInterval, time.Second},
		"TransferProcessorMaxRedispatchQueueSize":              {dynamicproperties.TransferProcessorMaxRedispatchQueueSize, 52},
		"TransferProcessorEnableValidator":                     {dynamicproperties.TransferProcessorEnableValidator, true},
		"TransferProcessorValidationInterval":                  {dynamicproperties.TransferProcessorValidationInterval, time.Second},
		"TransferProcessorVisibilityArchivalTimeLimit":         {dynamicproperties.TransferProcessorVisibilityArchivalTimeLimit, time.Second},
		"ReplicatorTaskDeleteBatchSize":                        {dynamicproperties.ReplicatorTaskDeleteBatchSize, 53},
		"ReplicatorReadTaskMaxRetryCount":                      {dynamicproperties.ReplicatorReadTaskMaxRetryCount, 54},
		"ReplicatorProcessorFetchTasksBatchSize":               {dynamicproperties.ReplicatorTaskBatchSize, 55},
		"ReplicatorProcessorMinTaskBatchSize":                  {dynamicproperties.ReplicatorMinTaskBatchSize, 1},
		"ReplicatorProcessorMaxTaskBatchSize":                  {dynamicproperties.ReplicatorMaxTaskBatchSize, 1000},
		"ReplicatorProcessorBatchSizeStepCount":                {dynamicproperties.ReplicatorTaskBatchStepCount, 10},
		"ReplicatorUpperLatency":                               {dynamicproperties.ReplicatorUpperLatency, time.Second},
		"ReplicatorCacheCapacity":                              {dynamicproperties.ReplicatorCacheCapacity, 56},
		"ReplicatorCacheMaxSize":                               {dynamicproperties.ReplicatorCacheMaxSize, 2000},
		"ReplicationBudgetManagerMaxSizeBytes":                 {dynamicproperties.ReplicationBudgetManagerMaxSizeBytes, 0},
		"ReplicationBudgetManagerMaxSizeCount":                 {dynamicproperties.ReplicationBudgetManagerMaxSizeCount, 0},
		"ReplicationBudgetManagerSoftCapThreshold":             {dynamicproperties.ReplicationBudgetManagerSoftCapThreshold, 1.0},
		"ExecutionMgrNumConns":                                 {dynamicproperties.ExecutionMgrNumConns, 57},
		"HistoryMgrNumConns":                                   {dynamicproperties.HistoryMgrNumConns, 58},
		"MaximumBufferedEventsBatch":                           {dynamicproperties.MaximumBufferedEventsBatch, 59},
		"MaximumSignalsPerExecution":                           {dynamicproperties.MaximumSignalsPerExecution, 60},
		"ShardUpdateMinInterval":                               {dynamicproperties.ShardUpdateMinInterval, time.Second},
		"ShardSyncMinInterval":                                 {dynamicproperties.ShardSyncMinInterval, time.Second},
		"ShardSyncTimerJitterCoefficient":                      {dynamicproperties.TransferProcessorMaxPollIntervalJitterCoefficient, 8.0},
		"LongPollExpirationInterval":                           {dynamicproperties.HistoryLongPollExpirationInterval, time.Second},
		"EventEncodingType":                                    {dynamicproperties.DefaultEventEncoding, "eventEncodingType"},
		"EnableParentClosePolicy":                              {dynamicproperties.EnableParentClosePolicy, true},
		"EnableParentClosePolicyWorker":                        {dynamicproperties.EnableParentClosePolicyWorker, true},
		"ParentClosePolicyThreshold":                           {dynamicproperties.ParentClosePolicyThreshold, 61},
		"ParentClosePolicyBatchSize":                           {dynamicproperties.ParentClosePolicyBatchSize, 62},
		"NumParentClosePolicySystemWorkflows":                  {dynamicproperties.NumParentClosePolicySystemWorkflows, 63},
		"NumArchiveSystemWorkflows":                            {dynamicproperties.NumArchiveSystemWorkflows, 64},
		"ArchiveRequestRPS":                                    {dynamicproperties.ArchiveRequestRPS, 65},
		"ArchiveInlineHistoryRPS":                              {dynamicproperties.ArchiveInlineHistoryRPS, 66},
		"ArchiveInlineHistoryGlobalRPS":                        {dynamicproperties.ArchiveInlineHistoryGlobalRPS, 67},
		"ArchiveInlineVisibilityRPS":                           {dynamicproperties.ArchiveInlineVisibilityRPS, 68},
		"ArchiveInlineVisibilityGlobalRPS":                     {dynamicproperties.ArchiveInlineVisibilityGlobalRPS, 69},
		"AllowArchivingIncompleteHistory":                      {dynamicproperties.AllowArchivingIncompleteHistory, true},
		"BlobSizeLimitError":                                   {dynamicproperties.BlobSizeLimitError, 70},
		"BlobSizeLimitWarn":                                    {dynamicproperties.BlobSizeLimitWarn, 71},
		"HistorySizeLimitError":                                {dynamicproperties.HistorySizeLimitError, 72},
		"HistorySizeLimitWarn":                                 {dynamicproperties.HistorySizeLimitWarn, 73},
		"HistoryCountLimitError":                               {dynamicproperties.HistoryCountLimitError, 74},
		"HistoryCountLimitWarn":                                {dynamicproperties.HistoryCountLimitWarn, 75},
		"PendingActivitiesCountLimitError":                     {dynamicproperties.PendingActivitiesCountLimitError, 76},
		"PendingActivitiesCountLimitWarn":                      {dynamicproperties.PendingActivitiesCountLimitWarn, 77},
		"PendingActivityValidationEnabled":                     {dynamicproperties.EnablePendingActivityValidation, true},
		"EnableQueryAttributeValidation":                       {dynamicproperties.EnableQueryAttributeValidation, true},
		"ValidSearchAttributes":                                {dynamicproperties.ValidSearchAttributes, map[string]interface{}{"key": 1}},
		"SearchAttributesNumberOfKeysLimit":                    {dynamicproperties.SearchAttributesNumberOfKeysLimit, 78},
		"SearchAttributesSizeOfValueLimit":                     {dynamicproperties.SearchAttributesSizeOfValueLimit, 79},
		"SearchAttributesTotalSizeLimit":                       {dynamicproperties.SearchAttributesTotalSizeLimit, 80},
		"StickyTTL":                                            {dynamicproperties.StickyTTL, time.Second},
		"DecisionHeartbeatTimeout":                             {dynamicproperties.DecisionHeartbeatTimeout, time.Second},
		"MaxDecisionStartToCloseSeconds":                       {dynamicproperties.MaxDecisionStartToCloseSeconds, 81},
		"DecisionRetryCriticalAttempts":                        {dynamicproperties.DecisionRetryCriticalAttempts, 82},
		"DecisionRetryMaxAttempts":                             {dynamicproperties.DecisionRetryMaxAttempts, 83},
		"DecisionTimeoutMaxAttempts":                           {dynamicproperties.DecisionTimeoutMaxAttempts, 5},
		"NormalDecisionScheduleToStartMaxAttempts":             {dynamicproperties.NormalDecisionScheduleToStartMaxAttempts, 84},
		"NormalDecisionScheduleToStartTimeout":                 {dynamicproperties.NormalDecisionScheduleToStartTimeout, time.Second},
		"ReplicationTaskFetcherParallelism":                    {dynamicproperties.ReplicationTaskFetcherParallelism, 85},
		"ReplicationTaskFetcherAggregationInterval":            {dynamicproperties.ReplicationTaskFetcherAggregationInterval, time.Second},
		"ReplicationTaskFetcherTimerJitterCoefficient":         {dynamicproperties.ReplicationTaskFetcherTimerJitterCoefficient, 9.0},
		"ReplicationTaskFetcherErrorRetryWait":                 {dynamicproperties.ReplicationTaskFetcherErrorRetryWait, time.Second},
		"ReplicationTaskFetcherServiceBusyWait":                {dynamicproperties.ReplicationTaskFetcherServiceBusyWait, time.Second},
		"ReplicationTaskProcessorErrorRetryWait":               {dynamicproperties.ReplicationTaskProcessorErrorRetryWait, time.Second},
		"ReplicationTaskProcessorErrorRetryMaxAttempts":        {dynamicproperties.ReplicationTaskProcessorErrorRetryMaxAttempts, 86},
		"ReplicationTaskProcessorErrorSecondRetryWait":         {dynamicproperties.ReplicationTaskProcessorErrorSecondRetryWait, time.Second},
		"ReplicationTaskProcessorErrorSecondRetryExpiration":   {dynamicproperties.ReplicationTaskProcessorErrorSecondRetryExpiration, time.Second},
		"ReplicationTaskProcessorErrorSecondRetryMaxWait":      {dynamicproperties.ReplicationTaskProcessorErrorSecondRetryMaxWait, time.Second},
		"ReplicationTaskProcessorNoTaskRetryWait":              {dynamicproperties.ReplicationTaskProcessorNoTaskInitialWait, time.Second},
		"ReplicationTaskProcessorCleanupInterval":              {dynamicproperties.ReplicationTaskProcessorCleanupInterval, time.Second},
		"ReplicationTaskProcessorCleanupJitterCoefficient":     {dynamicproperties.ReplicationTaskProcessorCleanupJitterCoefficient, 10.0},
		"ReplicationTaskProcessorStartWait":                    {dynamicproperties.ReplicationTaskProcessorStartWait, time.Second},
		"ReplicationTaskProcessorStartWaitJitterCoefficient":   {dynamicproperties.ReplicationTaskProcessorStartWaitJitterCoefficient, 11.0},
		"ReplicationTaskProcessorHostQPS":                      {dynamicproperties.ReplicationTaskProcessorHostQPS, 12.0},
		"ReplicationTaskProcessorShardQPS":                     {dynamicproperties.ReplicationTaskProcessorShardQPS, 13.0},
		"ReplicationTaskGenerationQPS":                         {dynamicproperties.ReplicationTaskGenerationQPS, 14.0},
		"EnableReplicationTaskGeneration":                      {dynamicproperties.EnableReplicationTaskGeneration, true},
		"EnableRecordWorkflowExecutionUninitialized":           {dynamicproperties.EnableRecordWorkflowExecutionUninitialized, true},
		"WorkflowIDExternalRPS":                                {dynamicproperties.WorkflowIDExternalRPS, 87},
		"WorkflowIDInternalRPS":                                {dynamicproperties.WorkflowIDInternalRPS, 88},
		"EnableConsistentQuery":                                {dynamicproperties.EnableConsistentQuery, true},
		"EnableConsistentQueryByDomain":                        {dynamicproperties.EnableConsistentQueryByDomain, true},
		"MaxBufferedQueryCount":                                {dynamicproperties.MaxBufferedQueryCount, 89},
		"EnableContextHeaderInVisibility":                      {dynamicproperties.EnableContextHeaderInVisibility, true},
		"EnableCrossClusterOperationsForDomain":                {dynamicproperties.EnableCrossClusterOperationsForDomain, true},
		"MutableStateChecksumGenProbability":                   {dynamicproperties.MutableStateChecksumGenProbability, 90},
		"MutableStateChecksumVerifyProbability":                {dynamicproperties.MutableStateChecksumVerifyProbability, 91},
		"MutableStateChecksumInvalidateBefore":                 {dynamicproperties.MutableStateChecksumInvalidateBefore, 15.0},
		"EnableRetryForChecksumFailure":                        {dynamicproperties.EnableRetryForChecksumFailure, true},
		"EnableHistoryCorruptionCheck":                         {dynamicproperties.EnableHistoryCorruptionCheck, true},
		"NotifyFailoverMarkerInterval":                         {dynamicproperties.NotifyFailoverMarkerInterval, time.Second},
		"NotifyFailoverMarkerTimerJitterCoefficient":           {dynamicproperties.NotifyFailoverMarkerTimerJitterCoefficient, 16.0},
		"EnableGracefulFailover":                               {dynamicproperties.EnableGracefulFailover, true},
		"EnableActivityLocalDispatchByDomain":                  {dynamicproperties.EnableActivityLocalDispatchByDomain, true},
		"MaxActivityCountDispatchByDomain":                     {dynamicproperties.MaxActivityCountDispatchByDomain, 92},
		"ActivityMaxScheduleToStartTimeoutForRetry":            {dynamicproperties.ActivityMaxScheduleToStartTimeoutForRetry, time.Second},
		"EnableDebugMode":                                      {dynamicproperties.EnableDebugMode, true},
		"EnableTaskInfoLogByDomainID":                          {dynamicproperties.HistoryEnableTaskInfoLogByDomainID, true},
		"EnableTimerDebugLogByDomainID":                        {dynamicproperties.EnableTimerDebugLogByDomainID, true},
		"SampleLoggingRate":                                    {dynamicproperties.SampleLoggingRate, 93},
		"EnableShardIDMetrics":                                 {dynamicproperties.EnableShardIDMetrics, true},
		"LargeShardHistorySizeMetricThreshold":                 {dynamicproperties.LargeShardHistorySizeMetricThreshold, 94},
		"LargeShardHistoryEventMetricThreshold":                {dynamicproperties.LargeShardHistoryEventMetricThreshold, 95},
		"LargeShardHistoryBlobMetricThreshold":                 {dynamicproperties.LargeShardHistoryBlobMetricThreshold, 96},
		"EnableStrongIdempotency":                              {dynamicproperties.EnableStrongIdempotency, true},
		"EnableStrongIdempotencySanityCheck":                   {dynamicproperties.EnableStrongIdempotencySanityCheck, true},
		"GlobalRatelimiterNewDataWeight":                       {dynamicproperties.HistoryGlobalRatelimiterNewDataWeight, 17.0},
		"GlobalRatelimiterUpdateInterval":                      {dynamicproperties.GlobalRatelimiterUpdateInterval, time.Second},
		"GlobalRatelimiterDecayAfter":                          {dynamicproperties.HistoryGlobalRatelimiterDecayAfter, time.Second},
		"GlobalRatelimiterGCAfter":                             {dynamicproperties.HistoryGlobalRatelimiterGCAfter, time.Second},
		"TaskSchedulerGlobalDomainRPS":                         {dynamicproperties.TaskSchedulerGlobalDomainRPS, 97},
		"TaskSchedulerEnableRateLimiterShadowMode":             {dynamicproperties.TaskSchedulerEnableRateLimiterShadowMode, false},
		"TaskSchedulerEnableRateLimiter":                       {dynamicproperties.TaskSchedulerEnableRateLimiter, true},
		"HostName":                                             {nil, hostname},
		"SearchAttributesHiddenValueKeys":                      {dynamicproperties.SearchAttributesHiddenValueKeys, map[string]interface{}{"CustomStringField": true}},
		"ExecutionCacheMaxByteSize":                            {dynamicproperties.ExecutionCacheMaxByteSize, 98},
		"DisableTransferFailoverQueue":                         {dynamicproperties.DisableTransferFailoverQueue, true},
		"DisableTimerFailoverQueue":                            {dynamicproperties.DisableTimerFailoverQueue, true},
		"EnableTransferQueueV2":                                {dynamicproperties.EnableTransferQueueV2, true},
		"EnableTimerQueueV2":                                   {dynamicproperties.EnableTimerQueueV2, true},
		"QueueMaxPendingTaskCount":                             {dynamicproperties.QueueMaxPendingTaskCount, 99},
		"EnableTimerQueueV2PendingTaskCountAlert":              {dynamicproperties.EnableTimerQueueV2PendingTaskCountAlert, true},
		"EnableTransferQueueV2PendingTaskCountAlert":           {dynamicproperties.EnableTransferQueueV2PendingTaskCountAlert, true},
		"QueueCriticalPendingTaskCount":                        {dynamicproperties.QueueCriticalPendingTaskCount, 100},
		"QueueMaxVirtualQueueCount":                            {dynamicproperties.QueueMaxVirtualQueueCount, 101},
		"VirtualSliceForceAppendInterval":                      {dynamicproperties.VirtualSliceForceAppendInterval, time.Second},
		"ReplicationTaskProcessorLatencyLogThreshold":          {dynamicproperties.ReplicationTaskProcessorLatencyLogThreshold, time.Duration(0)},
		"EnableCleanupOrphanedHistoryBranchOnWorkflowCreation": {dynamicproperties.EnableCleanupOrphanedHistoryBranchOnWorkflowCreation, true},
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
		case dynamicproperties.DurationPropertyFnWithDomainIDFilter:
			return fn("domain")
		case dynamicproperties.IntPropertyFnWithShardIDFilter:
			return fn(0)
		case dynamicproperties.StringPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicproperties.DurationPropertyFnWithShardIDFilter:
			return fn(0)
		case dynamicproperties.FloatPropertyFnWithShardIDFilter:
			return fn(0)
		case dynamicproperties.BoolPropertyFnWithDomainIDAndWorkflowIDFilter:
			return fn("domain", "workflowID")
		case dynamicproperties.MapPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicproperties.BoolPropertyFnWithShardIDFilter:
			return fn(0)
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
