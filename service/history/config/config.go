// Copyright (c) 2020 Uber Technologies, Inc.
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

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
)

// Config represents configuration for cadence-history service
type Config struct {
	NumberOfShards                   int
	IsAdvancedVisConfigExist         bool
	RPS                              dynamicconfig.IntPropertyFn
	MaxIDLengthWarnLimit             dynamicconfig.IntPropertyFn
	DomainNameMaxLength              dynamicconfig.IntPropertyFnWithDomainFilter
	IdentityMaxLength                dynamicconfig.IntPropertyFnWithDomainFilter
	WorkflowIDMaxLength              dynamicconfig.IntPropertyFnWithDomainFilter
	SignalNameMaxLength              dynamicconfig.IntPropertyFnWithDomainFilter
	WorkflowTypeMaxLength            dynamicconfig.IntPropertyFnWithDomainFilter
	RequestIDMaxLength               dynamicconfig.IntPropertyFnWithDomainFilter
	TaskListNameMaxLength            dynamicconfig.IntPropertyFnWithDomainFilter
	ActivityIDMaxLength              dynamicconfig.IntPropertyFnWithDomainFilter
	ActivityTypeMaxLength            dynamicconfig.IntPropertyFnWithDomainFilter
	MarkerNameMaxLength              dynamicconfig.IntPropertyFnWithDomainFilter
	TimerIDMaxLength                 dynamicconfig.IntPropertyFnWithDomainFilter
	PersistenceMaxQPS                dynamicconfig.IntPropertyFn
	PersistenceGlobalMaxQPS          dynamicconfig.IntPropertyFn
	EnableVisibilitySampling         dynamicconfig.BoolPropertyFn
	EnableReadFromClosedExecutionV2  dynamicconfig.BoolPropertyFn
	VisibilityOpenMaxQPS             dynamicconfig.IntPropertyFnWithDomainFilter
	VisibilityClosedMaxQPS           dynamicconfig.IntPropertyFnWithDomainFilter
	WriteVisibilityStoreName         dynamicconfig.StringPropertyFn
	EmitShardDiffLog                 dynamicconfig.BoolPropertyFn
	MaxAutoResetPoints               dynamicconfig.IntPropertyFnWithDomainFilter
	ThrottledLogRPS                  dynamicconfig.IntPropertyFn
	EnableStickyQuery                dynamicconfig.BoolPropertyFnWithDomainFilter
	ShutdownDrainDuration            dynamicconfig.DurationPropertyFn
	WorkflowDeletionJitterRange      dynamicconfig.IntPropertyFnWithDomainFilter
	DeleteHistoryEventContextTimeout dynamicconfig.IntPropertyFn
	MaxResponseSize                  int

	// HistoryCache settings
	// Change of these configs require shard restart
	HistoryCacheInitialSize dynamicconfig.IntPropertyFn
	HistoryCacheMaxSize     dynamicconfig.IntPropertyFn
	HistoryCacheTTL         dynamicconfig.DurationPropertyFn

	// EventsCache settings
	// Change of these configs require shard restart
	EventsCacheInitialCount       dynamicconfig.IntPropertyFn
	EventsCacheMaxCount           dynamicconfig.IntPropertyFn
	EventsCacheMaxSize            dynamicconfig.IntPropertyFn
	EventsCacheTTL                dynamicconfig.DurationPropertyFn
	EventsCacheGlobalEnable       dynamicconfig.BoolPropertyFn
	EventsCacheGlobalInitialCount dynamicconfig.IntPropertyFn
	EventsCacheGlobalMaxCount     dynamicconfig.IntPropertyFn

	// ShardController settings
	RangeSizeBits           uint
	AcquireShardInterval    dynamicconfig.DurationPropertyFn
	AcquireShardConcurrency dynamicconfig.IntPropertyFn

	// the artificial delay added to standby cluster's view of active cluster's time
	StandbyClusterDelay                  dynamicconfig.DurationPropertyFn
	StandbyTaskMissingEventsResendDelay  dynamicconfig.DurationPropertyFn
	StandbyTaskMissingEventsDiscardDelay dynamicconfig.DurationPropertyFn

	// Task process settings
	TaskProcessRPS                           dynamicconfig.IntPropertyFnWithDomainFilter
	TaskSchedulerType                        dynamicconfig.IntPropertyFn
	TaskSchedulerWorkerCount                 dynamicconfig.IntPropertyFn
	TaskSchedulerQueueSize                   dynamicconfig.IntPropertyFn
	TaskSchedulerDispatcherCount             dynamicconfig.IntPropertyFn
	TaskSchedulerRoundRobinWeights           dynamicconfig.MapPropertyFn
	TaskSchedulerDomainRoundRobinWeights     dynamicconfig.MapPropertyFnWithDomainFilter
	TaskSchedulerGlobalDomainRPS             dynamicconfig.IntPropertyFnWithDomainFilter
	TaskSchedulerEnableRateLimiter           dynamicconfig.BoolPropertyFn
	TaskSchedulerEnableRateLimiterShadowMode dynamicconfig.BoolPropertyFnWithDomainFilter
	TaskSchedulerEnableMigration             dynamicconfig.BoolPropertyFn
	TaskSchedulerMigrationRatio              dynamicconfig.IntPropertyFn
	TaskCriticalRetryCount                   dynamicconfig.IntPropertyFn
	ActiveTaskRedispatchInterval             dynamicconfig.DurationPropertyFn
	StandbyTaskRedispatchInterval            dynamicconfig.DurationPropertyFn
	StandbyTaskReReplicationContextTimeout   dynamicconfig.DurationPropertyFnWithDomainIDFilter
	EnableDropStuckTaskByDomainID            dynamicconfig.BoolPropertyFnWithDomainIDFilter
	ResurrectionCheckMinDelay                dynamicconfig.DurationPropertyFnWithDomainFilter

	// QueueProcessor settings
	QueueProcessorEnableSplit                          dynamicconfig.BoolPropertyFn
	QueueProcessorSplitMaxLevel                        dynamicconfig.IntPropertyFn
	QueueProcessorEnableRandomSplitByDomainID          dynamicconfig.BoolPropertyFnWithDomainIDFilter
	QueueProcessorRandomSplitProbability               dynamicconfig.FloatPropertyFn
	QueueProcessorEnablePendingTaskSplitByDomainID     dynamicconfig.BoolPropertyFnWithDomainIDFilter
	QueueProcessorPendingTaskSplitThreshold            dynamicconfig.MapPropertyFn
	QueueProcessorEnableStuckTaskSplitByDomainID       dynamicconfig.BoolPropertyFnWithDomainIDFilter
	QueueProcessorStuckTaskSplitThreshold              dynamicconfig.MapPropertyFn
	QueueProcessorSplitLookAheadDurationByDomainID     dynamicconfig.DurationPropertyFnWithDomainIDFilter
	QueueProcessorPollBackoffInterval                  dynamicconfig.DurationPropertyFn
	QueueProcessorPollBackoffIntervalJitterCoefficient dynamicconfig.FloatPropertyFn
	QueueProcessorEnablePersistQueueStates             dynamicconfig.BoolPropertyFn
	QueueProcessorEnableLoadQueueStates                dynamicconfig.BoolPropertyFn
	QueueProcessorEnableGracefulSyncShutdown           dynamicconfig.BoolPropertyFn

	// TimerQueueProcessor settings
	TimerTaskBatchSize                                dynamicconfig.IntPropertyFn
	TimerTaskDeleteBatchSize                          dynamicconfig.IntPropertyFn
	TimerProcessorGetFailureRetryCount                dynamicconfig.IntPropertyFn
	TimerProcessorCompleteTimerFailureRetryCount      dynamicconfig.IntPropertyFn
	TimerProcessorUpdateAckInterval                   dynamicconfig.DurationPropertyFn
	TimerProcessorUpdateAckIntervalJitterCoefficient  dynamicconfig.FloatPropertyFn
	TimerProcessorCompleteTimerInterval               dynamicconfig.DurationPropertyFn
	TimerProcessorFailoverMaxStartJitterInterval      dynamicconfig.DurationPropertyFn
	TimerProcessorFailoverMaxPollRPS                  dynamicconfig.IntPropertyFn
	TimerProcessorMaxPollRPS                          dynamicconfig.IntPropertyFn
	TimerProcessorMaxPollInterval                     dynamicconfig.DurationPropertyFn
	TimerProcessorMaxPollIntervalJitterCoefficient    dynamicconfig.FloatPropertyFn
	TimerProcessorSplitQueueInterval                  dynamicconfig.DurationPropertyFn
	TimerProcessorSplitQueueIntervalJitterCoefficient dynamicconfig.FloatPropertyFn
	TimerProcessorMaxRedispatchQueueSize              dynamicconfig.IntPropertyFn
	TimerProcessorMaxTimeShift                        dynamicconfig.DurationPropertyFn
	TimerProcessorHistoryArchivalSizeLimit            dynamicconfig.IntPropertyFn
	TimerProcessorArchivalTimeLimit                   dynamicconfig.DurationPropertyFn

	// TransferQueueProcessor settings
	TransferTaskBatchSize                                dynamicconfig.IntPropertyFn
	TransferTaskDeleteBatchSize                          dynamicconfig.IntPropertyFn
	TransferProcessorCompleteTransferFailureRetryCount   dynamicconfig.IntPropertyFn
	TransferProcessorFailoverMaxStartJitterInterval      dynamicconfig.DurationPropertyFn
	TransferProcessorFailoverMaxPollRPS                  dynamicconfig.IntPropertyFn
	TransferProcessorMaxPollRPS                          dynamicconfig.IntPropertyFn
	TransferProcessorMaxPollInterval                     dynamicconfig.DurationPropertyFn
	TransferProcessorMaxPollIntervalJitterCoefficient    dynamicconfig.FloatPropertyFn
	TransferProcessorSplitQueueInterval                  dynamicconfig.DurationPropertyFn
	TransferProcessorSplitQueueIntervalJitterCoefficient dynamicconfig.FloatPropertyFn
	TransferProcessorUpdateAckInterval                   dynamicconfig.DurationPropertyFn
	TransferProcessorUpdateAckIntervalJitterCoefficient  dynamicconfig.FloatPropertyFn
	TransferProcessorCompleteTransferInterval            dynamicconfig.DurationPropertyFn
	TransferProcessorMaxRedispatchQueueSize              dynamicconfig.IntPropertyFn
	TransferProcessorEnableValidator                     dynamicconfig.BoolPropertyFn
	TransferProcessorValidationInterval                  dynamicconfig.DurationPropertyFn
	TransferProcessorVisibilityArchivalTimeLimit         dynamicconfig.DurationPropertyFn

	// ReplicatorQueueProcessor settings
	ReplicatorTaskDeleteBatchSize          dynamicconfig.IntPropertyFn
	ReplicatorReadTaskMaxRetryCount        dynamicconfig.IntPropertyFn
	ReplicatorProcessorFetchTasksBatchSize dynamicconfig.IntPropertyFnWithShardIDFilter
	ReplicatorUpperLatency                 dynamicconfig.DurationPropertyFn
	ReplicatorCacheCapacity                dynamicconfig.IntPropertyFn

	// System Limits
	MaximumBufferedEventsBatch dynamicconfig.IntPropertyFn
	MaximumSignalsPerExecution dynamicconfig.IntPropertyFnWithDomainFilter

	// ShardUpdateMinInterval the minimal time interval which the shard info can be updated
	ShardUpdateMinInterval dynamicconfig.DurationPropertyFn
	// ShardSyncMinInterval the minimal time interval which the shard info should be sync to remote
	ShardSyncMinInterval            dynamicconfig.DurationPropertyFn
	ShardSyncTimerJitterCoefficient dynamicconfig.FloatPropertyFn

	// Time to hold a poll request before returning an empty response
	// right now only used by GetMutableState
	LongPollExpirationInterval dynamicconfig.DurationPropertyFnWithDomainFilter

	// encoding the history events
	EventEncodingType dynamicconfig.StringPropertyFnWithDomainFilter
	// whether or not using ParentClosePolicy
	EnableParentClosePolicy dynamicconfig.BoolPropertyFnWithDomainFilter
	// whether or not enable system workers for processing parent close policy task
	EnableParentClosePolicyWorker dynamicconfig.BoolPropertyFn
	// parent close policy will be processed by sys workers(if enabled) if
	// the number of children greater than or equal to this threshold
	ParentClosePolicyThreshold dynamicconfig.IntPropertyFnWithDomainFilter
	// the batch size of parent close policy processed by sys workers
	ParentClosePolicyBatchSize dynamicconfig.IntPropertyFnWithDomainFilter
	// total number of parentClosePolicy system workflows
	NumParentClosePolicySystemWorkflows dynamicconfig.IntPropertyFn

	// Archival settings
	NumArchiveSystemWorkflows        dynamicconfig.IntPropertyFn
	ArchiveRequestRPS                dynamicconfig.IntPropertyFn
	ArchiveInlineHistoryRPS          dynamicconfig.IntPropertyFn
	ArchiveInlineHistoryGlobalRPS    dynamicconfig.IntPropertyFn
	ArchiveInlineVisibilityRPS       dynamicconfig.IntPropertyFn
	ArchiveInlineVisibilityGlobalRPS dynamicconfig.IntPropertyFn
	AllowArchivingIncompleteHistory  dynamicconfig.BoolPropertyFn

	// Size limit related settings
	BlobSizeLimitError               dynamicconfig.IntPropertyFnWithDomainFilter
	BlobSizeLimitWarn                dynamicconfig.IntPropertyFnWithDomainFilter
	HistorySizeLimitError            dynamicconfig.IntPropertyFnWithDomainFilter
	HistorySizeLimitWarn             dynamicconfig.IntPropertyFnWithDomainFilter
	HistoryCountLimitError           dynamicconfig.IntPropertyFnWithDomainFilter
	HistoryCountLimitWarn            dynamicconfig.IntPropertyFnWithDomainFilter
	PendingActivitiesCountLimitError dynamicconfig.IntPropertyFn
	PendingActivitiesCountLimitWarn  dynamicconfig.IntPropertyFn
	PendingActivityValidationEnabled dynamicconfig.BoolPropertyFn

	// ValidSearchAttributes is legal indexed keys that can be used in list APIs
	EnableQueryAttributeValidation    dynamicconfig.BoolPropertyFn
	ValidSearchAttributes             dynamicconfig.MapPropertyFn
	SearchAttributesNumberOfKeysLimit dynamicconfig.IntPropertyFnWithDomainFilter
	SearchAttributesSizeOfValueLimit  dynamicconfig.IntPropertyFnWithDomainFilter
	SearchAttributesTotalSizeLimit    dynamicconfig.IntPropertyFnWithDomainFilter

	// Decision settings
	// StickyTTL is to expire a sticky tasklist if no update more than this duration
	// TODO https://github.com/uber/cadence/issues/2357
	StickyTTL dynamicconfig.DurationPropertyFnWithDomainFilter
	// DecisionHeartbeatTimeout is to timeout behavior of: RespondDecisionTaskComplete with ForceCreateNewDecisionTask == true without any decisions
	// So that decision will be scheduled to another worker(by clear stickyness)
	DecisionHeartbeatTimeout dynamicconfig.DurationPropertyFnWithDomainFilter
	// MaxDecisionStartToCloseSeconds is the StartToCloseSeconds for decision
	MaxDecisionStartToCloseSeconds           dynamicconfig.IntPropertyFnWithDomainFilter
	DecisionRetryCriticalAttempts            dynamicconfig.IntPropertyFn
	DecisionRetryMaxAttempts                 dynamicconfig.IntPropertyFnWithDomainFilter
	NormalDecisionScheduleToStartMaxAttempts dynamicconfig.IntPropertyFnWithDomainFilter
	NormalDecisionScheduleToStartTimeout     dynamicconfig.DurationPropertyFnWithDomainFilter

	// The following is used by the new RPC replication stack
	ReplicationTaskFetcherParallelism                  dynamicconfig.IntPropertyFn
	ReplicationTaskFetcherAggregationInterval          dynamicconfig.DurationPropertyFn
	ReplicationTaskFetcherTimerJitterCoefficient       dynamicconfig.FloatPropertyFn
	ReplicationTaskFetcherErrorRetryWait               dynamicconfig.DurationPropertyFn
	ReplicationTaskFetcherServiceBusyWait              dynamicconfig.DurationPropertyFn
	ReplicationTaskProcessorErrorRetryWait             dynamicconfig.DurationPropertyFnWithShardIDFilter
	ReplicationTaskProcessorErrorRetryMaxAttempts      dynamicconfig.IntPropertyFnWithShardIDFilter
	ReplicationTaskProcessorErrorSecondRetryWait       dynamicconfig.DurationPropertyFnWithShardIDFilter
	ReplicationTaskProcessorErrorSecondRetryMaxWait    dynamicconfig.DurationPropertyFnWithShardIDFilter
	ReplicationTaskProcessorErrorSecondRetryExpiration dynamicconfig.DurationPropertyFnWithShardIDFilter
	ReplicationTaskProcessorNoTaskRetryWait            dynamicconfig.DurationPropertyFnWithShardIDFilter
	ReplicationTaskProcessorCleanupInterval            dynamicconfig.DurationPropertyFnWithShardIDFilter
	ReplicationTaskProcessorCleanupJitterCoefficient   dynamicconfig.FloatPropertyFnWithShardIDFilter
	ReplicationTaskProcessorStartWait                  dynamicconfig.DurationPropertyFnWithShardIDFilter
	ReplicationTaskProcessorStartWaitJitterCoefficient dynamicconfig.FloatPropertyFnWithShardIDFilter
	ReplicationTaskProcessorHostQPS                    dynamicconfig.FloatPropertyFn
	ReplicationTaskProcessorShardQPS                   dynamicconfig.FloatPropertyFn
	ReplicationTaskGenerationQPS                       dynamicconfig.FloatPropertyFn
	EnableReplicationTaskGeneration                    dynamicconfig.BoolPropertyFnWithDomainIDAndWorkflowIDFilter
	EnableRecordWorkflowExecutionUninitialized         dynamicconfig.BoolPropertyFnWithDomainFilter

	// The following are used by the history workflowID cache
	WorkflowIDExternalRPS dynamicconfig.IntPropertyFnWithDomainFilter
	WorkflowIDInternalRPS dynamicconfig.IntPropertyFnWithDomainFilter

	// The following are used by consistent query
	EnableConsistentQuery         dynamicconfig.BoolPropertyFn
	EnableConsistentQueryByDomain dynamicconfig.BoolPropertyFnWithDomainFilter
	MaxBufferedQueryCount         dynamicconfig.IntPropertyFn

	// EnableContextHeaderInVisibility whether to enable indexing context header in visibility
	EnableContextHeaderInVisibility dynamicconfig.BoolPropertyFnWithDomainFilter

	EnableCrossClusterOperationsForDomain dynamicconfig.BoolPropertyFnWithDomainFilter

	// Data integrity check related config knobs
	MutableStateChecksumGenProbability    dynamicconfig.IntPropertyFnWithDomainFilter
	MutableStateChecksumVerifyProbability dynamicconfig.IntPropertyFnWithDomainFilter
	MutableStateChecksumInvalidateBefore  dynamicconfig.FloatPropertyFn
	EnableRetryForChecksumFailure         dynamicconfig.BoolPropertyFnWithDomainFilter

	// History check for corruptions
	EnableHistoryCorruptionCheck dynamicconfig.BoolPropertyFnWithDomainFilter

	// Failover marker heartbeat
	NotifyFailoverMarkerInterval               dynamicconfig.DurationPropertyFn
	NotifyFailoverMarkerTimerJitterCoefficient dynamicconfig.FloatPropertyFn
	EnableGracefulFailover                     dynamicconfig.BoolPropertyFn

	// Allows worker to dispatch activity tasks through local tunnel after decisions are made. This is an performance optimization to skip activity scheduling efforts.
	EnableActivityLocalDispatchByDomain dynamicconfig.BoolPropertyFnWithDomainFilter
	// Max # of activity tasks to dispatch to matching before creating transfer tasks. This is an performance optimization to skip activity scheduling efforts.
	MaxActivityCountDispatchByDomain dynamicconfig.IntPropertyFnWithDomainFilter

	ActivityMaxScheduleToStartTimeoutForRetry dynamicconfig.DurationPropertyFnWithDomainFilter

	// Debugging configurations
	EnableDebugMode               bool // note that this value is initialized once on service start
	EnableTaskInfoLogByDomainID   dynamicconfig.BoolPropertyFnWithDomainIDFilter
	EnableTimerDebugLogByDomainID dynamicconfig.BoolPropertyFnWithDomainIDFilter

	// Hotshard stuff
	SampleLoggingRate                     dynamicconfig.IntPropertyFn
	EnableShardIDMetrics                  dynamicconfig.BoolPropertyFn
	LargeShardHistorySizeMetricThreshold  dynamicconfig.IntPropertyFn
	LargeShardHistoryEventMetricThreshold dynamicconfig.IntPropertyFn
	LargeShardHistoryBlobMetricThreshold  dynamicconfig.IntPropertyFn

	EnableStrongIdempotency            dynamicconfig.BoolPropertyFnWithDomainFilter
	EnableStrongIdempotencySanityCheck dynamicconfig.BoolPropertyFnWithDomainFilter

	// Global ratelimiter
	GlobalRatelimiterNewDataWeight  dynamicconfig.FloatPropertyFn
	GlobalRatelimiterUpdateInterval dynamicconfig.DurationPropertyFn
	GlobalRatelimiterDecayAfter     dynamicconfig.DurationPropertyFn
	GlobalRatelimiterGCAfter        dynamicconfig.DurationPropertyFn

	// HostName for machine running the service
	HostName string
}

// New returns new service config with default values
func New(dc *dynamicconfig.Collection, numberOfShards int, maxMessageSize int, isAdvancedVisConfigExist bool, hostname string) *Config {
	cfg := &Config{
		NumberOfShards:                       numberOfShards,
		IsAdvancedVisConfigExist:             isAdvancedVisConfigExist,
		RPS:                                  dc.GetIntProperty(dynamicconfig.HistoryRPS),
		MaxIDLengthWarnLimit:                 dc.GetIntProperty(dynamicconfig.MaxIDLengthWarnLimit),
		DomainNameMaxLength:                  dc.GetIntPropertyFilteredByDomain(dynamicconfig.DomainNameMaxLength),
		IdentityMaxLength:                    dc.GetIntPropertyFilteredByDomain(dynamicconfig.IdentityMaxLength),
		WorkflowIDMaxLength:                  dc.GetIntPropertyFilteredByDomain(dynamicconfig.WorkflowIDMaxLength),
		SignalNameMaxLength:                  dc.GetIntPropertyFilteredByDomain(dynamicconfig.SignalNameMaxLength),
		WorkflowTypeMaxLength:                dc.GetIntPropertyFilteredByDomain(dynamicconfig.WorkflowTypeMaxLength),
		RequestIDMaxLength:                   dc.GetIntPropertyFilteredByDomain(dynamicconfig.RequestIDMaxLength),
		TaskListNameMaxLength:                dc.GetIntPropertyFilteredByDomain(dynamicconfig.TaskListNameMaxLength),
		ActivityIDMaxLength:                  dc.GetIntPropertyFilteredByDomain(dynamicconfig.ActivityIDMaxLength),
		ActivityTypeMaxLength:                dc.GetIntPropertyFilteredByDomain(dynamicconfig.ActivityTypeMaxLength),
		MarkerNameMaxLength:                  dc.GetIntPropertyFilteredByDomain(dynamicconfig.MarkerNameMaxLength),
		TimerIDMaxLength:                     dc.GetIntPropertyFilteredByDomain(dynamicconfig.TimerIDMaxLength),
		PersistenceMaxQPS:                    dc.GetIntProperty(dynamicconfig.HistoryPersistenceMaxQPS),
		PersistenceGlobalMaxQPS:              dc.GetIntProperty(dynamicconfig.HistoryPersistenceGlobalMaxQPS),
		ShutdownDrainDuration:                dc.GetDurationProperty(dynamicconfig.HistoryShutdownDrainDuration),
		EnableVisibilitySampling:             dc.GetBoolProperty(dynamicconfig.EnableVisibilitySampling),
		EnableReadFromClosedExecutionV2:      dc.GetBoolProperty(dynamicconfig.EnableReadFromClosedExecutionV2),
		VisibilityOpenMaxQPS:                 dc.GetIntPropertyFilteredByDomain(dynamicconfig.HistoryVisibilityOpenMaxQPS),
		VisibilityClosedMaxQPS:               dc.GetIntPropertyFilteredByDomain(dynamicconfig.HistoryVisibilityClosedMaxQPS),
		MaxAutoResetPoints:                   dc.GetIntPropertyFilteredByDomain(dynamicconfig.HistoryMaxAutoResetPoints),
		MaxDecisionStartToCloseSeconds:       dc.GetIntPropertyFilteredByDomain(dynamicconfig.MaxDecisionStartToCloseSeconds),
		WriteVisibilityStoreName:             dc.GetStringProperty(dynamicconfig.WriteVisibilityStoreName),
		EmitShardDiffLog:                     dc.GetBoolProperty(dynamicconfig.EmitShardDiffLog),
		HistoryCacheInitialSize:              dc.GetIntProperty(dynamicconfig.HistoryCacheInitialSize),
		HistoryCacheMaxSize:                  dc.GetIntProperty(dynamicconfig.HistoryCacheMaxSize),
		HistoryCacheTTL:                      dc.GetDurationProperty(dynamicconfig.HistoryCacheTTL),
		EventsCacheInitialCount:              dc.GetIntProperty(dynamicconfig.EventsCacheInitialCount),
		EventsCacheMaxCount:                  dc.GetIntProperty(dynamicconfig.EventsCacheMaxCount),
		EventsCacheMaxSize:                   dc.GetIntProperty(dynamicconfig.EventsCacheMaxSize),
		EventsCacheTTL:                       dc.GetDurationProperty(dynamicconfig.EventsCacheTTL),
		EventsCacheGlobalEnable:              dc.GetBoolProperty(dynamicconfig.EventsCacheGlobalEnable),
		EventsCacheGlobalInitialCount:        dc.GetIntProperty(dynamicconfig.EventsCacheGlobalInitialCount),
		EventsCacheGlobalMaxCount:            dc.GetIntProperty(dynamicconfig.EventsCacheGlobalMaxCount),
		RangeSizeBits:                        20, // 20 bits for sequencer, 2^20 sequence number for any range
		AcquireShardInterval:                 dc.GetDurationProperty(dynamicconfig.AcquireShardInterval),
		AcquireShardConcurrency:              dc.GetIntProperty(dynamicconfig.AcquireShardConcurrency),
		StandbyClusterDelay:                  dc.GetDurationProperty(dynamicconfig.StandbyClusterDelay),
		StandbyTaskMissingEventsResendDelay:  dc.GetDurationProperty(dynamicconfig.StandbyTaskMissingEventsResendDelay),
		StandbyTaskMissingEventsDiscardDelay: dc.GetDurationProperty(dynamicconfig.StandbyTaskMissingEventsDiscardDelay),
		WorkflowDeletionJitterRange:          dc.GetIntPropertyFilteredByDomain(dynamicconfig.WorkflowDeletionJitterRange),
		DeleteHistoryEventContextTimeout:     dc.GetIntProperty(dynamicconfig.DeleteHistoryEventContextTimeout),
		MaxResponseSize:                      maxMessageSize,

		TaskProcessRPS:                           dc.GetIntPropertyFilteredByDomain(dynamicconfig.TaskProcessRPS),
		TaskSchedulerType:                        dc.GetIntProperty(dynamicconfig.TaskSchedulerType),
		TaskSchedulerWorkerCount:                 dc.GetIntProperty(dynamicconfig.TaskSchedulerWorkerCount),
		TaskSchedulerQueueSize:                   dc.GetIntProperty(dynamicconfig.TaskSchedulerQueueSize),
		TaskSchedulerDispatcherCount:             dc.GetIntProperty(dynamicconfig.TaskSchedulerDispatcherCount),
		TaskSchedulerRoundRobinWeights:           dc.GetMapProperty(dynamicconfig.TaskSchedulerRoundRobinWeights),
		TaskSchedulerDomainRoundRobinWeights:     dc.GetMapPropertyFilteredByDomain(dynamicconfig.TaskSchedulerDomainRoundRobinWeights),
		TaskSchedulerGlobalDomainRPS:             dc.GetIntPropertyFilteredByDomain(dynamicconfig.TaskSchedulerGlobalDomainRPS),
		TaskSchedulerEnableRateLimiter:           dc.GetBoolProperty(dynamicconfig.TaskSchedulerEnableRateLimiter),
		TaskSchedulerEnableRateLimiterShadowMode: dc.GetBoolPropertyFilteredByDomain(dynamicconfig.TaskSchedulerEnableRateLimiterShadowMode),
		TaskSchedulerEnableMigration:             dc.GetBoolProperty(dynamicconfig.TaskSchedulerEnableMigration),
		TaskSchedulerMigrationRatio:              dc.GetIntProperty(dynamicconfig.TaskSchedulerMigrationRatio),
		TaskCriticalRetryCount:                   dc.GetIntProperty(dynamicconfig.TaskCriticalRetryCount),
		ActiveTaskRedispatchInterval:             dc.GetDurationProperty(dynamicconfig.ActiveTaskRedispatchInterval),
		StandbyTaskRedispatchInterval:            dc.GetDurationProperty(dynamicconfig.StandbyTaskRedispatchInterval),
		StandbyTaskReReplicationContextTimeout:   dc.GetDurationPropertyFilteredByDomainID(dynamicconfig.StandbyTaskReReplicationContextTimeout),
		EnableDropStuckTaskByDomainID:            dc.GetBoolPropertyFilteredByDomainID(dynamicconfig.EnableDropStuckTaskByDomainID),
		ResurrectionCheckMinDelay:                dc.GetDurationPropertyFilteredByDomain(dynamicconfig.ResurrectionCheckMinDelay),

		QueueProcessorEnableSplit:                          dc.GetBoolProperty(dynamicconfig.QueueProcessorEnableSplit),
		QueueProcessorSplitMaxLevel:                        dc.GetIntProperty(dynamicconfig.QueueProcessorSplitMaxLevel),
		QueueProcessorEnableRandomSplitByDomainID:          dc.GetBoolPropertyFilteredByDomainID(dynamicconfig.QueueProcessorEnableRandomSplitByDomainID),
		QueueProcessorRandomSplitProbability:               dc.GetFloat64Property(dynamicconfig.QueueProcessorRandomSplitProbability),
		QueueProcessorEnablePendingTaskSplitByDomainID:     dc.GetBoolPropertyFilteredByDomainID(dynamicconfig.QueueProcessorEnablePendingTaskSplitByDomainID),
		QueueProcessorPendingTaskSplitThreshold:            dc.GetMapProperty(dynamicconfig.QueueProcessorPendingTaskSplitThreshold),
		QueueProcessorEnableStuckTaskSplitByDomainID:       dc.GetBoolPropertyFilteredByDomainID(dynamicconfig.QueueProcessorEnableStuckTaskSplitByDomainID),
		QueueProcessorStuckTaskSplitThreshold:              dc.GetMapProperty(dynamicconfig.QueueProcessorStuckTaskSplitThreshold),
		QueueProcessorSplitLookAheadDurationByDomainID:     dc.GetDurationPropertyFilteredByDomainID(dynamicconfig.QueueProcessorSplitLookAheadDurationByDomainID),
		QueueProcessorPollBackoffInterval:                  dc.GetDurationProperty(dynamicconfig.QueueProcessorPollBackoffInterval),
		QueueProcessorPollBackoffIntervalJitterCoefficient: dc.GetFloat64Property(dynamicconfig.QueueProcessorPollBackoffIntervalJitterCoefficient),
		QueueProcessorEnablePersistQueueStates:             dc.GetBoolProperty(dynamicconfig.QueueProcessorEnablePersistQueueStates),
		QueueProcessorEnableLoadQueueStates:                dc.GetBoolProperty(dynamicconfig.QueueProcessorEnableLoadQueueStates),
		QueueProcessorEnableGracefulSyncShutdown:           dc.GetBoolProperty(dynamicconfig.QueueProcessorEnableGracefulSyncShutdown),

		TimerTaskBatchSize:                                dc.GetIntProperty(dynamicconfig.TimerTaskBatchSize),
		TimerTaskDeleteBatchSize:                          dc.GetIntProperty(dynamicconfig.TimerTaskDeleteBatchSize),
		TimerProcessorGetFailureRetryCount:                dc.GetIntProperty(dynamicconfig.TimerProcessorGetFailureRetryCount),
		TimerProcessorCompleteTimerFailureRetryCount:      dc.GetIntProperty(dynamicconfig.TimerProcessorCompleteTimerFailureRetryCount),
		TimerProcessorUpdateAckInterval:                   dc.GetDurationProperty(dynamicconfig.TimerProcessorUpdateAckInterval),
		TimerProcessorUpdateAckIntervalJitterCoefficient:  dc.GetFloat64Property(dynamicconfig.TimerProcessorUpdateAckIntervalJitterCoefficient),
		TimerProcessorCompleteTimerInterval:               dc.GetDurationProperty(dynamicconfig.TimerProcessorCompleteTimerInterval),
		TimerProcessorFailoverMaxStartJitterInterval:      dc.GetDurationProperty(dynamicconfig.TimerProcessorFailoverMaxStartJitterInterval),
		TimerProcessorFailoverMaxPollRPS:                  dc.GetIntProperty(dynamicconfig.TimerProcessorFailoverMaxPollRPS),
		TimerProcessorMaxPollRPS:                          dc.GetIntProperty(dynamicconfig.TimerProcessorMaxPollRPS),
		TimerProcessorMaxPollInterval:                     dc.GetDurationProperty(dynamicconfig.TimerProcessorMaxPollInterval),
		TimerProcessorMaxPollIntervalJitterCoefficient:    dc.GetFloat64Property(dynamicconfig.TimerProcessorMaxPollIntervalJitterCoefficient),
		TimerProcessorSplitQueueInterval:                  dc.GetDurationProperty(dynamicconfig.TimerProcessorSplitQueueInterval),
		TimerProcessorSplitQueueIntervalJitterCoefficient: dc.GetFloat64Property(dynamicconfig.TimerProcessorSplitQueueIntervalJitterCoefficient),
		TimerProcessorMaxRedispatchQueueSize:              dc.GetIntProperty(dynamicconfig.TimerProcessorMaxRedispatchQueueSize),
		TimerProcessorMaxTimeShift:                        dc.GetDurationProperty(dynamicconfig.TimerProcessorMaxTimeShift),
		TimerProcessorHistoryArchivalSizeLimit:            dc.GetIntProperty(dynamicconfig.TimerProcessorHistoryArchivalSizeLimit),
		TimerProcessorArchivalTimeLimit:                   dc.GetDurationProperty(dynamicconfig.TimerProcessorArchivalTimeLimit),

		TransferTaskBatchSize:                                dc.GetIntProperty(dynamicconfig.TransferTaskBatchSize),
		TransferTaskDeleteBatchSize:                          dc.GetIntProperty(dynamicconfig.TransferTaskDeleteBatchSize),
		TransferProcessorFailoverMaxStartJitterInterval:      dc.GetDurationProperty(dynamicconfig.TransferProcessorFailoverMaxStartJitterInterval),
		TransferProcessorFailoverMaxPollRPS:                  dc.GetIntProperty(dynamicconfig.TransferProcessorFailoverMaxPollRPS),
		TransferProcessorMaxPollRPS:                          dc.GetIntProperty(dynamicconfig.TransferProcessorMaxPollRPS),
		TransferProcessorCompleteTransferFailureRetryCount:   dc.GetIntProperty(dynamicconfig.TransferProcessorCompleteTransferFailureRetryCount),
		TransferProcessorMaxPollInterval:                     dc.GetDurationProperty(dynamicconfig.TransferProcessorMaxPollInterval),
		TransferProcessorMaxPollIntervalJitterCoefficient:    dc.GetFloat64Property(dynamicconfig.TransferProcessorMaxPollIntervalJitterCoefficient),
		TransferProcessorSplitQueueInterval:                  dc.GetDurationProperty(dynamicconfig.TransferProcessorSplitQueueInterval),
		TransferProcessorSplitQueueIntervalJitterCoefficient: dc.GetFloat64Property(dynamicconfig.TransferProcessorSplitQueueIntervalJitterCoefficient),
		TransferProcessorUpdateAckInterval:                   dc.GetDurationProperty(dynamicconfig.TransferProcessorUpdateAckInterval),
		TransferProcessorUpdateAckIntervalJitterCoefficient:  dc.GetFloat64Property(dynamicconfig.TransferProcessorUpdateAckIntervalJitterCoefficient),
		TransferProcessorCompleteTransferInterval:            dc.GetDurationProperty(dynamicconfig.TransferProcessorCompleteTransferInterval),
		TransferProcessorMaxRedispatchQueueSize:              dc.GetIntProperty(dynamicconfig.TransferProcessorMaxRedispatchQueueSize),
		TransferProcessorEnableValidator:                     dc.GetBoolProperty(dynamicconfig.TransferProcessorEnableValidator),
		TransferProcessorValidationInterval:                  dc.GetDurationProperty(dynamicconfig.TransferProcessorValidationInterval),
		TransferProcessorVisibilityArchivalTimeLimit:         dc.GetDurationProperty(dynamicconfig.TransferProcessorVisibilityArchivalTimeLimit),

		ReplicatorTaskDeleteBatchSize:          dc.GetIntProperty(dynamicconfig.ReplicatorTaskDeleteBatchSize),
		ReplicatorReadTaskMaxRetryCount:        dc.GetIntProperty(dynamicconfig.ReplicatorReadTaskMaxRetryCount),
		ReplicatorProcessorFetchTasksBatchSize: dc.GetIntPropertyFilteredByShardID(dynamicconfig.ReplicatorTaskBatchSize),
		ReplicatorUpperLatency:                 dc.GetDurationProperty(dynamicconfig.ReplicatorUpperLatency),
		ReplicatorCacheCapacity:                dc.GetIntProperty(dynamicconfig.ReplicatorCacheCapacity),

		MaximumBufferedEventsBatch:      dc.GetIntProperty(dynamicconfig.MaximumBufferedEventsBatch),
		MaximumSignalsPerExecution:      dc.GetIntPropertyFilteredByDomain(dynamicconfig.MaximumSignalsPerExecution),
		ShardUpdateMinInterval:          dc.GetDurationProperty(dynamicconfig.ShardUpdateMinInterval),
		ShardSyncMinInterval:            dc.GetDurationProperty(dynamicconfig.ShardSyncMinInterval),
		ShardSyncTimerJitterCoefficient: dc.GetFloat64Property(dynamicconfig.TransferProcessorMaxPollIntervalJitterCoefficient),

		// history client: client/history/client.go set the client timeout 30s
		LongPollExpirationInterval:          dc.GetDurationPropertyFilteredByDomain(dynamicconfig.HistoryLongPollExpirationInterval),
		EventEncodingType:                   dc.GetStringPropertyFilteredByDomain(dynamicconfig.DefaultEventEncoding),
		EnableParentClosePolicy:             dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableParentClosePolicy),
		NumParentClosePolicySystemWorkflows: dc.GetIntProperty(dynamicconfig.NumParentClosePolicySystemWorkflows),
		EnableParentClosePolicyWorker:       dc.GetBoolProperty(dynamicconfig.EnableParentClosePolicyWorker),
		ParentClosePolicyThreshold:          dc.GetIntPropertyFilteredByDomain(dynamicconfig.ParentClosePolicyThreshold),
		ParentClosePolicyBatchSize:          dc.GetIntPropertyFilteredByDomain(dynamicconfig.ParentClosePolicyBatchSize),

		NumArchiveSystemWorkflows:        dc.GetIntProperty(dynamicconfig.NumArchiveSystemWorkflows),
		ArchiveRequestRPS:                dc.GetIntProperty(dynamicconfig.ArchiveRequestRPS),
		ArchiveInlineHistoryRPS:          dc.GetIntProperty(dynamicconfig.ArchiveInlineHistoryRPS),
		ArchiveInlineHistoryGlobalRPS:    dc.GetIntProperty(dynamicconfig.ArchiveInlineHistoryGlobalRPS),
		ArchiveInlineVisibilityRPS:       dc.GetIntProperty(dynamicconfig.ArchiveInlineVisibilityRPS),
		ArchiveInlineVisibilityGlobalRPS: dc.GetIntProperty(dynamicconfig.ArchiveInlineVisibilityGlobalRPS),
		AllowArchivingIncompleteHistory:  dc.GetBoolProperty(dynamicconfig.AllowArchivingIncompleteHistory),

		BlobSizeLimitError:               dc.GetIntPropertyFilteredByDomain(dynamicconfig.BlobSizeLimitError),
		BlobSizeLimitWarn:                dc.GetIntPropertyFilteredByDomain(dynamicconfig.BlobSizeLimitWarn),
		HistorySizeLimitError:            dc.GetIntPropertyFilteredByDomain(dynamicconfig.HistorySizeLimitError),
		HistorySizeLimitWarn:             dc.GetIntPropertyFilteredByDomain(dynamicconfig.HistorySizeLimitWarn),
		HistoryCountLimitError:           dc.GetIntPropertyFilteredByDomain(dynamicconfig.HistoryCountLimitError),
		HistoryCountLimitWarn:            dc.GetIntPropertyFilteredByDomain(dynamicconfig.HistoryCountLimitWarn),
		PendingActivitiesCountLimitError: dc.GetIntProperty(dynamicconfig.PendingActivitiesCountLimitError),
		PendingActivitiesCountLimitWarn:  dc.GetIntProperty(dynamicconfig.PendingActivitiesCountLimitWarn),
		PendingActivityValidationEnabled: dc.GetBoolProperty(dynamicconfig.EnablePendingActivityValidation),

		ThrottledLogRPS:   dc.GetIntProperty(dynamicconfig.HistoryThrottledLogRPS),
		EnableStickyQuery: dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableStickyQuery),

		EnableQueryAttributeValidation:           dc.GetBoolProperty(dynamicconfig.EnableQueryAttributeValidation),
		ValidSearchAttributes:                    dc.GetMapProperty(dynamicconfig.ValidSearchAttributes),
		SearchAttributesNumberOfKeysLimit:        dc.GetIntPropertyFilteredByDomain(dynamicconfig.SearchAttributesNumberOfKeysLimit),
		SearchAttributesSizeOfValueLimit:         dc.GetIntPropertyFilteredByDomain(dynamicconfig.SearchAttributesSizeOfValueLimit),
		SearchAttributesTotalSizeLimit:           dc.GetIntPropertyFilteredByDomain(dynamicconfig.SearchAttributesTotalSizeLimit),
		StickyTTL:                                dc.GetDurationPropertyFilteredByDomain(dynamicconfig.StickyTTL),
		DecisionHeartbeatTimeout:                 dc.GetDurationPropertyFilteredByDomain(dynamicconfig.DecisionHeartbeatTimeout),
		DecisionRetryCriticalAttempts:            dc.GetIntProperty(dynamicconfig.DecisionRetryCriticalAttempts),
		DecisionRetryMaxAttempts:                 dc.GetIntPropertyFilteredByDomain(dynamicconfig.DecisionRetryMaxAttempts),
		NormalDecisionScheduleToStartMaxAttempts: dc.GetIntPropertyFilteredByDomain(dynamicconfig.NormalDecisionScheduleToStartMaxAttempts),
		NormalDecisionScheduleToStartTimeout:     dc.GetDurationPropertyFilteredByDomain(dynamicconfig.NormalDecisionScheduleToStartTimeout),

		ReplicationTaskFetcherParallelism:                  dc.GetIntProperty(dynamicconfig.ReplicationTaskFetcherParallelism),
		ReplicationTaskFetcherAggregationInterval:          dc.GetDurationProperty(dynamicconfig.ReplicationTaskFetcherAggregationInterval),
		ReplicationTaskFetcherTimerJitterCoefficient:       dc.GetFloat64Property(dynamicconfig.ReplicationTaskFetcherTimerJitterCoefficient),
		ReplicationTaskFetcherErrorRetryWait:               dc.GetDurationProperty(dynamicconfig.ReplicationTaskFetcherErrorRetryWait),
		ReplicationTaskFetcherServiceBusyWait:              dc.GetDurationProperty(dynamicconfig.ReplicationTaskFetcherServiceBusyWait),
		ReplicationTaskProcessorErrorRetryWait:             dc.GetDurationPropertyFilteredByShardID(dynamicconfig.ReplicationTaskProcessorErrorRetryWait),
		ReplicationTaskProcessorErrorRetryMaxAttempts:      dc.GetIntPropertyFilteredByShardID(dynamicconfig.ReplicationTaskProcessorErrorRetryMaxAttempts),
		ReplicationTaskProcessorErrorSecondRetryWait:       dc.GetDurationPropertyFilteredByShardID(dynamicconfig.ReplicationTaskProcessorErrorSecondRetryWait),
		ReplicationTaskProcessorErrorSecondRetryMaxWait:    dc.GetDurationPropertyFilteredByShardID(dynamicconfig.ReplicationTaskProcessorErrorSecondRetryMaxWait),
		ReplicationTaskProcessorErrorSecondRetryExpiration: dc.GetDurationPropertyFilteredByShardID(dynamicconfig.ReplicationTaskProcessorErrorSecondRetryExpiration),
		ReplicationTaskProcessorNoTaskRetryWait:            dc.GetDurationPropertyFilteredByShardID(dynamicconfig.ReplicationTaskProcessorNoTaskInitialWait),
		ReplicationTaskProcessorCleanupInterval:            dc.GetDurationPropertyFilteredByShardID(dynamicconfig.ReplicationTaskProcessorCleanupInterval),
		ReplicationTaskProcessorCleanupJitterCoefficient:   dc.GetFloat64PropertyFilteredByShardID(dynamicconfig.ReplicationTaskProcessorCleanupJitterCoefficient),
		ReplicationTaskProcessorStartWait:                  dc.GetDurationPropertyFilteredByShardID(dynamicconfig.ReplicationTaskProcessorStartWait),
		ReplicationTaskProcessorStartWaitJitterCoefficient: dc.GetFloat64PropertyFilteredByShardID(dynamicconfig.ReplicationTaskProcessorStartWaitJitterCoefficient),
		ReplicationTaskProcessorHostQPS:                    dc.GetFloat64Property(dynamicconfig.ReplicationTaskProcessorHostQPS),
		ReplicationTaskProcessorShardQPS:                   dc.GetFloat64Property(dynamicconfig.ReplicationTaskProcessorShardQPS),
		ReplicationTaskGenerationQPS:                       dc.GetFloat64Property(dynamicconfig.ReplicationTaskGenerationQPS),
		EnableReplicationTaskGeneration:                    dc.GetBoolPropertyFilteredByDomainIDAndWorkflowID(dynamicconfig.EnableReplicationTaskGeneration),
		EnableRecordWorkflowExecutionUninitialized:         dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableRecordWorkflowExecutionUninitialized),

		WorkflowIDExternalRPS: dc.GetIntPropertyFilteredByDomain(dynamicconfig.WorkflowIDExternalRPS),
		WorkflowIDInternalRPS: dc.GetIntPropertyFilteredByDomain(dynamicconfig.WorkflowIDInternalRPS),

		EnableConsistentQuery:                 dc.GetBoolProperty(dynamicconfig.EnableConsistentQuery),
		EnableConsistentQueryByDomain:         dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableConsistentQueryByDomain),
		EnableContextHeaderInVisibility:       dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableContextHeaderInVisibility),
		EnableCrossClusterOperationsForDomain: dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableCrossClusterOperationsForDomain),
		MaxBufferedQueryCount:                 dc.GetIntProperty(dynamicconfig.MaxBufferedQueryCount),
		MutableStateChecksumGenProbability:    dc.GetIntPropertyFilteredByDomain(dynamicconfig.MutableStateChecksumGenProbability),
		MutableStateChecksumVerifyProbability: dc.GetIntPropertyFilteredByDomain(dynamicconfig.MutableStateChecksumVerifyProbability),
		MutableStateChecksumInvalidateBefore:  dc.GetFloat64Property(dynamicconfig.MutableStateChecksumInvalidateBefore),
		EnableRetryForChecksumFailure:         dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableRetryForChecksumFailure),

		EnableHistoryCorruptionCheck: dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableHistoryCorruptionCheck),

		NotifyFailoverMarkerInterval:               dc.GetDurationProperty(dynamicconfig.NotifyFailoverMarkerInterval),
		NotifyFailoverMarkerTimerJitterCoefficient: dc.GetFloat64Property(dynamicconfig.NotifyFailoverMarkerTimerJitterCoefficient),
		EnableGracefulFailover:                     dc.GetBoolProperty(dynamicconfig.EnableGracefulFailover),

		EnableActivityLocalDispatchByDomain: dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableActivityLocalDispatchByDomain),
		MaxActivityCountDispatchByDomain:    dc.GetIntPropertyFilteredByDomain(dynamicconfig.MaxActivityCountDispatchByDomain),

		ActivityMaxScheduleToStartTimeoutForRetry: dc.GetDurationPropertyFilteredByDomain(dynamicconfig.ActivityMaxScheduleToStartTimeoutForRetry),

		EnableDebugMode:               dc.GetBoolProperty(dynamicconfig.EnableDebugMode)(),
		EnableTaskInfoLogByDomainID:   dc.GetBoolPropertyFilteredByDomainID(dynamicconfig.HistoryEnableTaskInfoLogByDomainID),
		EnableTimerDebugLogByDomainID: dc.GetBoolPropertyFilteredByDomainID(dynamicconfig.EnableTimerDebugLogByDomainID),

		SampleLoggingRate:                     dc.GetIntProperty(dynamicconfig.SampleLoggingRate),
		EnableShardIDMetrics:                  dc.GetBoolProperty(dynamicconfig.EnableShardIDMetrics),
		LargeShardHistorySizeMetricThreshold:  dc.GetIntProperty(dynamicconfig.LargeShardHistorySizeMetricThreshold),
		LargeShardHistoryEventMetricThreshold: dc.GetIntProperty(dynamicconfig.LargeShardHistoryEventMetricThreshold),
		LargeShardHistoryBlobMetricThreshold:  dc.GetIntProperty(dynamicconfig.LargeShardHistoryBlobMetricThreshold),

		EnableStrongIdempotency:            dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableStrongIdempotency),
		EnableStrongIdempotencySanityCheck: dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableStrongIdempotencySanityCheck),

		GlobalRatelimiterNewDataWeight:  dc.GetFloat64Property(dynamicconfig.HistoryGlobalRatelimiterNewDataWeight),
		GlobalRatelimiterUpdateInterval: dc.GetDurationProperty(dynamicconfig.GlobalRatelimiterUpdateInterval),
		GlobalRatelimiterDecayAfter:     dc.GetDurationProperty(dynamicconfig.HistoryGlobalRatelimiterDecayAfter),
		GlobalRatelimiterGCAfter:        dc.GetDurationProperty(dynamicconfig.HistoryGlobalRatelimiterGCAfter),

		HostName: hostname,
	}

	return cfg
}

// NewForTest create new history service config for test
func NewForTest() *Config {
	return NewForTestByShardNumber(1)
}

// NewForTestByShardNumber create new history service config for test
func NewForTestByShardNumber(shardNumber int) *Config {
	panicIfErr := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	inMem := dynamicconfig.NewInMemoryClient()
	panicIfErr(inMem.UpdateValue(dynamicconfig.HistoryLongPollExpirationInterval, 10*time.Second))
	panicIfErr(inMem.UpdateValue(dynamicconfig.EnableConsistentQueryByDomain, true))
	panicIfErr(inMem.UpdateValue(dynamicconfig.ReplicationTaskProcessorHostQPS, float64(10000)))
	panicIfErr(inMem.UpdateValue(dynamicconfig.ReplicationTaskProcessorCleanupInterval, 20*time.Millisecond))
	panicIfErr(inMem.UpdateValue(dynamicconfig.ReplicatorTaskDeleteBatchSize, 50))
	panicIfErr(inMem.UpdateValue(dynamicconfig.ShardSyncMinInterval, 20*time.Millisecond))
	panicIfErr(inMem.UpdateValue(dynamicconfig.ReplicationTaskProcessorShardQPS, float64(10000)))
	panicIfErr(inMem.UpdateValue(dynamicconfig.ReplicationTaskProcessorStartWait, time.Nanosecond))
	panicIfErr(inMem.UpdateValue(dynamicconfig.EnableActivityLocalDispatchByDomain, true))
	panicIfErr(inMem.UpdateValue(dynamicconfig.MaxActivityCountDispatchByDomain, 0))
	panicIfErr(inMem.UpdateValue(dynamicconfig.EnableCrossClusterOperationsForDomain, true))
	panicIfErr(inMem.UpdateValue(dynamicconfig.NormalDecisionScheduleToStartMaxAttempts, 3))
	panicIfErr(inMem.UpdateValue(dynamicconfig.EnablePendingActivityValidation, true))
	panicIfErr(inMem.UpdateValue(dynamicconfig.QueueProcessorEnableGracefulSyncShutdown, true))
	panicIfErr(inMem.UpdateValue(dynamicconfig.QueueProcessorSplitMaxLevel, 2))
	panicIfErr(inMem.UpdateValue(dynamicconfig.QueueProcessorPendingTaskSplitThreshold, map[string]interface{}{
		"0": 1000,
		"1": 5000,
	}))
	panicIfErr(inMem.UpdateValue(dynamicconfig.QueueProcessorStuckTaskSplitThreshold, map[string]interface{}{
		"0": 10,
		"1": 50,
	}))
	panicIfErr(inMem.UpdateValue(dynamicconfig.QueueProcessorRandomSplitProbability, 0.5))
	panicIfErr(inMem.UpdateValue(dynamicconfig.EnableStrongIdempotency, true))

	dc := dynamicconfig.NewCollection(inMem, log.NewNoop())
	config := New(dc, shardNumber, 1024*1024, false, "")
	// reduce the duration of long poll to increase test speed
	config.LongPollExpirationInterval = dc.GetDurationPropertyFilteredByDomain(dynamicconfig.HistoryLongPollExpirationInterval)
	config.EnableConsistentQueryByDomain = dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableConsistentQueryByDomain)
	config.ReplicationTaskProcessorHostQPS = dc.GetFloat64Property(dynamicconfig.ReplicationTaskProcessorHostQPS)
	config.ReplicationTaskProcessorCleanupInterval = dc.GetDurationPropertyFilteredByShardID(dynamicconfig.ReplicationTaskProcessorCleanupInterval)
	config.ReplicatorTaskDeleteBatchSize = dc.GetIntProperty(dynamicconfig.ReplicatorTaskDeleteBatchSize)
	config.ShardSyncMinInterval = dc.GetDurationProperty(dynamicconfig.ShardSyncMinInterval)
	config.ReplicationTaskProcessorShardQPS = dc.GetFloat64Property(dynamicconfig.ReplicationTaskProcessorShardQPS)
	config.ReplicationTaskProcessorStartWait = dc.GetDurationPropertyFilteredByShardID(dynamicconfig.ReplicationTaskProcessorStartWait)
	config.EnableActivityLocalDispatchByDomain = dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableActivityLocalDispatchByDomain)
	config.MaxActivityCountDispatchByDomain = dc.GetIntPropertyFilteredByDomain(dynamicconfig.MaxActivityCountDispatchByDomain)
	config.EnableCrossClusterOperationsForDomain = dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableCrossClusterOperationsForDomain)
	config.NormalDecisionScheduleToStartMaxAttempts = dc.GetIntPropertyFilteredByDomain(dynamicconfig.NormalDecisionScheduleToStartMaxAttempts)
	config.NormalDecisionScheduleToStartTimeout = dc.GetDurationPropertyFilteredByDomain(dynamicconfig.NormalDecisionScheduleToStartTimeout)
	config.PendingActivityValidationEnabled = dc.GetBoolProperty(dynamicconfig.EnablePendingActivityValidation)
	config.QueueProcessorEnableGracefulSyncShutdown = dc.GetBoolProperty(dynamicconfig.QueueProcessorEnableGracefulSyncShutdown)
	config.QueueProcessorSplitMaxLevel = dc.GetIntProperty(dynamicconfig.QueueProcessorSplitMaxLevel)
	config.QueueProcessorPendingTaskSplitThreshold = dc.GetMapProperty(dynamicconfig.QueueProcessorPendingTaskSplitThreshold)
	config.QueueProcessorStuckTaskSplitThreshold = dc.GetMapProperty(dynamicconfig.QueueProcessorStuckTaskSplitThreshold)
	config.QueueProcessorRandomSplitProbability = dc.GetFloat64Property(dynamicconfig.QueueProcessorRandomSplitProbability)
	return config
}

// GetShardID return the corresponding shard ID for a given workflow ID
func (config *Config) GetShardID(workflowID string) int {
	return common.WorkflowIDToHistoryShard(workflowID, config.NumberOfShards)
}
