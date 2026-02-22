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
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
)

// Config represents configuration for cadence-history service
type Config struct {
	NumberOfShards                   int
	IsAdvancedVisConfigExist         bool
	RPS                              dynamicproperties.IntPropertyFn
	MaxIDLengthWarnLimit             dynamicproperties.IntPropertyFn
	DomainNameMaxLength              dynamicproperties.IntPropertyFnWithDomainFilter
	IdentityMaxLength                dynamicproperties.IntPropertyFnWithDomainFilter
	WorkflowIDMaxLength              dynamicproperties.IntPropertyFnWithDomainFilter
	SignalNameMaxLength              dynamicproperties.IntPropertyFnWithDomainFilter
	WorkflowTypeMaxLength            dynamicproperties.IntPropertyFnWithDomainFilter
	RequestIDMaxLength               dynamicproperties.IntPropertyFnWithDomainFilter
	TaskListNameMaxLength            dynamicproperties.IntPropertyFnWithDomainFilter
	ActivityIDMaxLength              dynamicproperties.IntPropertyFnWithDomainFilter
	ActivityTypeMaxLength            dynamicproperties.IntPropertyFnWithDomainFilter
	MarkerNameMaxLength              dynamicproperties.IntPropertyFnWithDomainFilter
	TimerIDMaxLength                 dynamicproperties.IntPropertyFnWithDomainFilter
	PersistenceMaxQPS                dynamicproperties.IntPropertyFn
	PersistenceGlobalMaxQPS          dynamicproperties.IntPropertyFn
	EnableVisibilitySampling         dynamicproperties.BoolPropertyFn
	EnableReadFromClosedExecutionV2  dynamicproperties.BoolPropertyFn
	VisibilityOpenMaxQPS             dynamicproperties.IntPropertyFnWithDomainFilter
	VisibilityClosedMaxQPS           dynamicproperties.IntPropertyFnWithDomainFilter
	WriteVisibilityStoreName         dynamicproperties.StringPropertyFn
	EmitShardDiffLog                 dynamicproperties.BoolPropertyFn
	MaxAutoResetPoints               dynamicproperties.IntPropertyFnWithDomainFilter
	ThrottledLogRPS                  dynamicproperties.IntPropertyFn
	EnableStickyQuery                dynamicproperties.BoolPropertyFnWithDomainFilter
	ShutdownDrainDuration            dynamicproperties.DurationPropertyFn
	WorkflowDeletionJitterRange      dynamicproperties.IntPropertyFnWithDomainFilter
	DeleteHistoryEventContextTimeout dynamicproperties.IntPropertyFn
	MaxResponseSize                  int

	// HistoryCache settings
	// Change of these configs require shard restart
	HistoryCacheInitialSize              dynamicproperties.IntPropertyFn
	HistoryCacheMaxSize                  dynamicproperties.IntPropertyFn
	HistoryCacheTTL                      dynamicproperties.DurationPropertyFn
	EnableSizeBasedHistoryExecutionCache dynamicproperties.BoolPropertyFn
	ExecutionCacheMaxByteSize            dynamicproperties.IntPropertyFn

	// EventsCache settings
	// Change of these configs require shard restart
	EventsCacheInitialCount          dynamicproperties.IntPropertyFn
	EventsCacheMaxCount              dynamicproperties.IntPropertyFn
	EventsCacheMaxSize               dynamicproperties.IntPropertyFn
	EventsCacheTTL                   dynamicproperties.DurationPropertyFn
	EventsCacheGlobalEnable          dynamicproperties.BoolPropertyFn
	EventsCacheGlobalInitialCount    dynamicproperties.IntPropertyFn
	EventsCacheGlobalMaxCount        dynamicproperties.IntPropertyFn
	EnableSizeBasedHistoryEventCache dynamicproperties.BoolPropertyFn

	// ShardController settings
	RangeSizeBits           uint
	AcquireShardInterval    dynamicproperties.DurationPropertyFn
	AcquireShardConcurrency dynamicproperties.IntPropertyFn

	// the artificial delay added to standby cluster's view of active cluster's time
	StandbyClusterDelay                  dynamicproperties.DurationPropertyFn
	StandbyTaskMissingEventsResendDelay  dynamicproperties.DurationPropertyFn
	StandbyTaskMissingEventsDiscardDelay dynamicproperties.DurationPropertyFn

	// Task process settings
	TaskProcessRPS                           dynamicproperties.IntPropertyFnWithDomainFilter
	TaskSchedulerType                        dynamicproperties.IntPropertyFn
	TaskSchedulerWorkerCount                 dynamicproperties.IntPropertyFn
	TaskSchedulerQueueSize                   dynamicproperties.IntPropertyFn
	TaskSchedulerDispatcherCount             dynamicproperties.IntPropertyFn
	TaskSchedulerRoundRobinWeights           dynamicproperties.MapPropertyFn
	TaskSchedulerDomainRoundRobinWeights     dynamicproperties.MapPropertyFnWithDomainFilter
	TaskSchedulerGlobalDomainRPS             dynamicproperties.IntPropertyFnWithDomainFilter
	TaskSchedulerEnableRateLimiter           dynamicproperties.BoolPropertyFn
	TaskSchedulerEnableRateLimiterShadowMode dynamicproperties.BoolPropertyFnWithDomainFilter
	TaskCriticalRetryCount                   dynamicproperties.IntPropertyFn
	ActiveTaskRedispatchInterval             dynamicproperties.DurationPropertyFn
	StandbyTaskRedispatchInterval            dynamicproperties.DurationPropertyFn
	StandbyTaskReReplicationContextTimeout   dynamicproperties.DurationPropertyFnWithDomainIDFilter
	EnableDropStuckTaskByDomainID            dynamicproperties.BoolPropertyFnWithDomainIDFilter
	ResurrectionCheckMinDelay                dynamicproperties.DurationPropertyFnWithDomainFilter

	// History Queue (v2) settings
	EnableTimerQueueV2                         dynamicproperties.BoolPropertyFnWithShardIDFilter
	EnableTransferQueueV2                      dynamicproperties.BoolPropertyFnWithShardIDFilter
	QueueMaxPendingTaskCount                   dynamicproperties.IntPropertyFn
	EnableTimerQueueV2PendingTaskCountAlert    dynamicproperties.BoolPropertyFnWithShardIDFilter
	EnableTransferQueueV2PendingTaskCountAlert dynamicproperties.BoolPropertyFnWithShardIDFilter
	QueueCriticalPendingTaskCount              dynamicproperties.IntPropertyFn
	QueueMaxVirtualQueueCount                  dynamicproperties.IntPropertyFn
	VirtualSliceForceAppendInterval            dynamicproperties.DurationPropertyFn

	// QueueProcessor settings
	QueueProcessorEnableSplit                          dynamicproperties.BoolPropertyFn
	QueueProcessorSplitMaxLevel                        dynamicproperties.IntPropertyFn
	QueueProcessorEnableRandomSplitByDomainID          dynamicproperties.BoolPropertyFnWithDomainIDFilter
	QueueProcessorRandomSplitProbability               dynamicproperties.FloatPropertyFn
	QueueProcessorEnablePendingTaskSplitByDomainID     dynamicproperties.BoolPropertyFnWithDomainIDFilter
	QueueProcessorPendingTaskSplitThreshold            dynamicproperties.MapPropertyFn
	QueueProcessorEnableStuckTaskSplitByDomainID       dynamicproperties.BoolPropertyFnWithDomainIDFilter
	QueueProcessorStuckTaskSplitThreshold              dynamicproperties.MapPropertyFn
	QueueProcessorSplitLookAheadDurationByDomainID     dynamicproperties.DurationPropertyFnWithDomainIDFilter
	QueueProcessorPollBackoffInterval                  dynamicproperties.DurationPropertyFn
	QueueProcessorPollBackoffIntervalJitterCoefficient dynamicproperties.FloatPropertyFn
	QueueProcessorEnablePersistQueueStates             dynamicproperties.BoolPropertyFn
	QueueProcessorEnableLoadQueueStates                dynamicproperties.BoolPropertyFn
	QueueProcessorEnableGracefulSyncShutdown           dynamicproperties.BoolPropertyFn

	// TimerQueueProcessor settings
	TimerTaskBatchSize                                dynamicproperties.IntPropertyFn
	TimerTaskDeleteBatchSize                          dynamicproperties.IntPropertyFn
	TimerProcessorGetFailureRetryCount                dynamicproperties.IntPropertyFn
	TimerProcessorCompleteTimerFailureRetryCount      dynamicproperties.IntPropertyFn
	TimerProcessorUpdateAckInterval                   dynamicproperties.DurationPropertyFn
	TimerProcessorUpdateAckIntervalJitterCoefficient  dynamicproperties.FloatPropertyFn
	TimerProcessorCompleteTimerInterval               dynamicproperties.DurationPropertyFn
	TimerProcessorFailoverMaxStartJitterInterval      dynamicproperties.DurationPropertyFn
	TimerProcessorFailoverMaxPollRPS                  dynamicproperties.IntPropertyFn
	TimerProcessorMaxPollRPS                          dynamicproperties.IntPropertyFn
	TimerProcessorMaxPollInterval                     dynamicproperties.DurationPropertyFn
	TimerProcessorMaxPollIntervalJitterCoefficient    dynamicproperties.FloatPropertyFn
	TimerProcessorSplitQueueInterval                  dynamicproperties.DurationPropertyFn
	TimerProcessorSplitQueueIntervalJitterCoefficient dynamicproperties.FloatPropertyFn
	TimerProcessorMaxRedispatchQueueSize              dynamicproperties.IntPropertyFn
	TimerProcessorMaxTimeShift                        dynamicproperties.DurationPropertyFn
	TimerProcessorInMemoryQueueMaxTimeShift           dynamicproperties.DurationPropertyFnWithShardIDFilter
	TimerProcessorHistoryArchivalSizeLimit            dynamicproperties.IntPropertyFn
	TimerProcessorArchivalTimeLimit                   dynamicproperties.DurationPropertyFn
	DisableTimerFailoverQueue                         dynamicproperties.BoolPropertyFn

	// TransferQueueProcessor settings
	TransferTaskBatchSize                                dynamicproperties.IntPropertyFn
	TransferTaskDeleteBatchSize                          dynamicproperties.IntPropertyFn
	TransferProcessorCompleteTransferFailureRetryCount   dynamicproperties.IntPropertyFn
	TransferProcessorFailoverMaxStartJitterInterval      dynamicproperties.DurationPropertyFn
	TransferProcessorFailoverMaxPollRPS                  dynamicproperties.IntPropertyFn
	TransferProcessorMaxPollRPS                          dynamicproperties.IntPropertyFn
	TransferProcessorMaxPollInterval                     dynamicproperties.DurationPropertyFn
	TransferProcessorMaxPollIntervalJitterCoefficient    dynamicproperties.FloatPropertyFn
	TransferProcessorSplitQueueInterval                  dynamicproperties.DurationPropertyFn
	TransferProcessorSplitQueueIntervalJitterCoefficient dynamicproperties.FloatPropertyFn
	TransferProcessorUpdateAckInterval                   dynamicproperties.DurationPropertyFn
	TransferProcessorUpdateAckIntervalJitterCoefficient  dynamicproperties.FloatPropertyFn
	TransferProcessorCompleteTransferInterval            dynamicproperties.DurationPropertyFn
	TransferProcessorMaxRedispatchQueueSize              dynamicproperties.IntPropertyFn
	TransferProcessorEnableValidator                     dynamicproperties.BoolPropertyFn
	TransferProcessorValidationInterval                  dynamicproperties.DurationPropertyFn
	TransferProcessorVisibilityArchivalTimeLimit         dynamicproperties.DurationPropertyFn
	DisableTransferFailoverQueue                         dynamicproperties.BoolPropertyFn

	// ReplicatorQueueProcessor settings
	ReplicatorTaskDeleteBatchSize            dynamicproperties.IntPropertyFn
	ReplicatorReadTaskMaxRetryCount          dynamicproperties.IntPropertyFn
	ReplicatorProcessorFetchTasksBatchSize   dynamicproperties.IntPropertyFnWithShardIDFilter
	ReplicatorProcessorMaxTaskBatchSize      dynamicproperties.IntPropertyFnWithShardIDFilter
	ReplicatorProcessorMinTaskBatchSize      dynamicproperties.IntPropertyFnWithShardIDFilter
	ReplicatorProcessorBatchSizeStepCount    dynamicproperties.IntPropertyFnWithShardIDFilter
	ReplicatorUpperLatency                   dynamicproperties.DurationPropertyFn
	ReplicatorCacheCapacity                  dynamicproperties.IntPropertyFn
	ReplicatorCacheMaxSize                   dynamicproperties.IntPropertyFn
	ReplicationBudgetManagerMaxSizeBytes     dynamicproperties.IntPropertyFn
	ReplicationBudgetManagerMaxSizeCount     dynamicproperties.IntPropertyFn
	ReplicationBudgetManagerSoftCapThreshold dynamicproperties.FloatPropertyFn

	// System Limits
	MaximumBufferedEventsBatch dynamicproperties.IntPropertyFn
	MaximumSignalsPerExecution dynamicproperties.IntPropertyFnWithDomainFilter

	// ShardUpdateMinInterval the minimal time interval which the shard info can be updated
	ShardUpdateMinInterval dynamicproperties.DurationPropertyFn
	// ShardSyncMinInterval the minimal time interval which the shard info should be sync to remote
	ShardSyncMinInterval            dynamicproperties.DurationPropertyFn
	ShardSyncTimerJitterCoefficient dynamicproperties.FloatPropertyFn

	// Time to hold a poll request before returning an empty response
	// right now only used by GetMutableState
	LongPollExpirationInterval dynamicproperties.DurationPropertyFnWithDomainFilter

	// encoding the history events
	EventEncodingType dynamicproperties.StringPropertyFnWithDomainFilter
	// whether or not using ParentClosePolicy
	EnableParentClosePolicy dynamicproperties.BoolPropertyFnWithDomainFilter
	// whether or not enable system workers for processing parent close policy task
	EnableParentClosePolicyWorker dynamicproperties.BoolPropertyFn
	// parent close policy will be processed by sys workers(if enabled) if
	// the number of children greater than or equal to this threshold
	ParentClosePolicyThreshold dynamicproperties.IntPropertyFnWithDomainFilter
	// the batch size of parent close policy processed by sys workers
	ParentClosePolicyBatchSize dynamicproperties.IntPropertyFnWithDomainFilter
	// total number of parentClosePolicy system workflows
	NumParentClosePolicySystemWorkflows dynamicproperties.IntPropertyFn

	// Archival settings
	NumArchiveSystemWorkflows        dynamicproperties.IntPropertyFn
	ArchiveRequestRPS                dynamicproperties.IntPropertyFn
	ArchiveInlineHistoryRPS          dynamicproperties.IntPropertyFn
	ArchiveInlineHistoryGlobalRPS    dynamicproperties.IntPropertyFn
	ArchiveInlineVisibilityRPS       dynamicproperties.IntPropertyFn
	ArchiveInlineVisibilityGlobalRPS dynamicproperties.IntPropertyFn
	AllowArchivingIncompleteHistory  dynamicproperties.BoolPropertyFn

	// Size limit related settings
	BlobSizeLimitError               dynamicproperties.IntPropertyFnWithDomainFilter
	BlobSizeLimitWarn                dynamicproperties.IntPropertyFnWithDomainFilter
	HistorySizeLimitError            dynamicproperties.IntPropertyFnWithDomainFilter
	HistorySizeLimitWarn             dynamicproperties.IntPropertyFnWithDomainFilter
	HistoryCountLimitError           dynamicproperties.IntPropertyFnWithDomainFilter
	HistoryCountLimitWarn            dynamicproperties.IntPropertyFnWithDomainFilter
	PendingActivitiesCountLimitError dynamicproperties.IntPropertyFn
	PendingActivitiesCountLimitWarn  dynamicproperties.IntPropertyFn
	PendingActivityValidationEnabled dynamicproperties.BoolPropertyFn

	// ValidSearchAttributes is legal indexed keys that can be used in list APIs
	EnableQueryAttributeValidation    dynamicproperties.BoolPropertyFn
	ValidSearchAttributes             dynamicproperties.MapPropertyFn
	SearchAttributesNumberOfKeysLimit dynamicproperties.IntPropertyFnWithDomainFilter
	SearchAttributesSizeOfValueLimit  dynamicproperties.IntPropertyFnWithDomainFilter
	SearchAttributesTotalSizeLimit    dynamicproperties.IntPropertyFnWithDomainFilter
	SearchAttributesHiddenValueKeys   dynamicproperties.MapPropertyFn

	// Decision settings
	// StickyTTL is to expire a sticky tasklist if no update more than this duration
	// TODO https://github.com/uber/cadence/issues/2357
	StickyTTL dynamicproperties.DurationPropertyFnWithDomainFilter
	// DecisionHeartbeatTimeout is to timeout behavior of: RespondDecisionTaskComplete with ForceCreateNewDecisionTask == true without any decisions
	// So that decision will be scheduled to another worker(by clear stickyness)
	DecisionHeartbeatTimeout dynamicproperties.DurationPropertyFnWithDomainFilter
	// MaxDecisionStartToCloseSeconds is the StartToCloseSeconds for decision
	MaxDecisionStartToCloseSeconds           dynamicproperties.IntPropertyFnWithDomainFilter
	DecisionRetryCriticalAttempts            dynamicproperties.IntPropertyFn
	DecisionRetryMaxAttempts                 dynamicproperties.IntPropertyFnWithDomainFilter
	EnforceDecisionTaskAttempts              dynamicproperties.BoolPropertyFnWithDomainFilter
	NormalDecisionScheduleToStartMaxAttempts dynamicproperties.IntPropertyFnWithDomainFilter
	NormalDecisionScheduleToStartTimeout     dynamicproperties.DurationPropertyFnWithDomainFilter

	// The following is used by the new RPC replication stack
	ReplicationTaskFetcherParallelism                    dynamicproperties.IntPropertyFn
	ReplicationTaskFetcherAggregationInterval            dynamicproperties.DurationPropertyFn
	ReplicationTaskFetcherTimerJitterCoefficient         dynamicproperties.FloatPropertyFn
	ReplicationTaskFetcherErrorRetryWait                 dynamicproperties.DurationPropertyFn
	ReplicationTaskFetcherServiceBusyWait                dynamicproperties.DurationPropertyFn
	ReplicationTaskProcessorErrorRetryWait               dynamicproperties.DurationPropertyFnWithShardIDFilter
	ReplicationTaskProcessorErrorRetryMaxAttempts        dynamicproperties.IntPropertyFnWithShardIDFilter
	ReplicationTaskProcessorErrorSecondRetryWait         dynamicproperties.DurationPropertyFnWithShardIDFilter
	ReplicationTaskProcessorErrorSecondRetryMaxWait      dynamicproperties.DurationPropertyFnWithShardIDFilter
	ReplicationTaskProcessorErrorSecondRetryExpiration   dynamicproperties.DurationPropertyFnWithShardIDFilter
	ReplicationTaskProcessorNoTaskRetryWait              dynamicproperties.DurationPropertyFnWithShardIDFilter
	ReplicationTaskProcessorCleanupInterval              dynamicproperties.DurationPropertyFnWithShardIDFilter
	ReplicationTaskProcessorCleanupJitterCoefficient     dynamicproperties.FloatPropertyFnWithShardIDFilter
	ReplicationTaskProcessorStartWait                    dynamicproperties.DurationPropertyFnWithShardIDFilter
	ReplicationTaskProcessorStartWaitJitterCoefficient   dynamicproperties.FloatPropertyFnWithShardIDFilter
	ReplicationTaskProcessorHostQPS                      dynamicproperties.FloatPropertyFn
	ReplicationTaskProcessorShardQPS                     dynamicproperties.FloatPropertyFn
	ReplicationTaskGenerationQPS                         dynamicproperties.FloatPropertyFn
	EnableReplicationTaskGeneration                      dynamicproperties.BoolPropertyFnWithDomainIDAndWorkflowIDFilter
	EnableRecordWorkflowExecutionUninitialized           dynamicproperties.BoolPropertyFnWithDomainFilter
	EnableCleanupOrphanedHistoryBranchOnWorkflowCreation dynamicproperties.BoolPropertyFnWithDomainFilter
	ReplicationTaskProcessorLatencyLogThreshold          dynamicproperties.DurationPropertyFn

	// The following are used by the history workflowID cache
	WorkflowIDExternalRPS dynamicproperties.IntPropertyFnWithDomainFilter
	WorkflowIDInternalRPS dynamicproperties.IntPropertyFnWithDomainFilter

	// The following are used by consistent query
	EnableConsistentQuery         dynamicproperties.BoolPropertyFn
	EnableConsistentQueryByDomain dynamicproperties.BoolPropertyFnWithDomainFilter
	MaxBufferedQueryCount         dynamicproperties.IntPropertyFn

	// EnableContextHeaderInVisibility whether to enable indexing context header in visibility
	EnableContextHeaderInVisibility dynamicproperties.BoolPropertyFnWithDomainFilter

	EnableCrossClusterOperationsForDomain dynamicproperties.BoolPropertyFnWithDomainFilter

	// Data integrity check related config knobs
	MutableStateChecksumGenProbability    dynamicproperties.IntPropertyFnWithDomainFilter
	MutableStateChecksumVerifyProbability dynamicproperties.IntPropertyFnWithDomainFilter
	MutableStateChecksumInvalidateBefore  dynamicproperties.FloatPropertyFn
	EnableRetryForChecksumFailure         dynamicproperties.BoolPropertyFnWithDomainFilter

	// History check for corruptions
	EnableHistoryCorruptionCheck dynamicproperties.BoolPropertyFnWithDomainFilter

	// Failover marker heartbeat
	NotifyFailoverMarkerInterval               dynamicproperties.DurationPropertyFn
	NotifyFailoverMarkerTimerJitterCoefficient dynamicproperties.FloatPropertyFn
	EnableGracefulFailover                     dynamicproperties.BoolPropertyFn

	// Allows worker to dispatch activity tasks through local tunnel after decisions are made. This is an performance optimization to skip activity scheduling efforts.
	EnableActivityLocalDispatchByDomain dynamicproperties.BoolPropertyFnWithDomainFilter
	// Max # of activity tasks to dispatch to matching before creating transfer tasks. This is an performance optimization to skip activity scheduling efforts.
	MaxActivityCountDispatchByDomain dynamicproperties.IntPropertyFnWithDomainFilter

	ActivityMaxScheduleToStartTimeoutForRetry dynamicproperties.DurationPropertyFnWithDomainFilter

	// Debugging configurations
	EnableDebugMode               bool // note that this value is initialized once on service start
	EnableTaskInfoLogByDomainID   dynamicproperties.BoolPropertyFnWithDomainIDFilter
	EnableTimerDebugLogByDomainID dynamicproperties.BoolPropertyFnWithDomainIDFilter

	// Hotshard stuff
	SampleLoggingRate                     dynamicproperties.IntPropertyFn
	EnableShardIDMetrics                  dynamicproperties.BoolPropertyFn
	LargeShardHistorySizeMetricThreshold  dynamicproperties.IntPropertyFn
	LargeShardHistoryEventMetricThreshold dynamicproperties.IntPropertyFn
	LargeShardHistoryBlobMetricThreshold  dynamicproperties.IntPropertyFn

	EnableStrongIdempotency            dynamicproperties.BoolPropertyFnWithDomainFilter
	EnableStrongIdempotencySanityCheck dynamicproperties.BoolPropertyFnWithDomainFilter

	// Global ratelimiter
	GlobalRatelimiterNewDataWeight  dynamicproperties.FloatPropertyFn
	GlobalRatelimiterUpdateInterval dynamicproperties.DurationPropertyFn
	GlobalRatelimiterDecayAfter     dynamicproperties.DurationPropertyFn
	GlobalRatelimiterGCAfter        dynamicproperties.DurationPropertyFn

	// HostName for machine running the service
	HostName string
}

// New returns new service config with default values
func New(dc *dynamicconfig.Collection, numberOfShards int, maxMessageSize int, isAdvancedVisConfigExist bool, hostname string) *Config {
	cfg := &Config{
		NumberOfShards:                       numberOfShards,
		IsAdvancedVisConfigExist:             isAdvancedVisConfigExist,
		RPS:                                  dc.GetIntProperty(dynamicproperties.HistoryRPS),
		MaxIDLengthWarnLimit:                 dc.GetIntProperty(dynamicproperties.MaxIDLengthWarnLimit),
		DomainNameMaxLength:                  dc.GetIntPropertyFilteredByDomain(dynamicproperties.DomainNameMaxLength),
		IdentityMaxLength:                    dc.GetIntPropertyFilteredByDomain(dynamicproperties.IdentityMaxLength),
		WorkflowIDMaxLength:                  dc.GetIntPropertyFilteredByDomain(dynamicproperties.WorkflowIDMaxLength),
		SignalNameMaxLength:                  dc.GetIntPropertyFilteredByDomain(dynamicproperties.SignalNameMaxLength),
		WorkflowTypeMaxLength:                dc.GetIntPropertyFilteredByDomain(dynamicproperties.WorkflowTypeMaxLength),
		RequestIDMaxLength:                   dc.GetIntPropertyFilteredByDomain(dynamicproperties.RequestIDMaxLength),
		TaskListNameMaxLength:                dc.GetIntPropertyFilteredByDomain(dynamicproperties.TaskListNameMaxLength),
		ActivityIDMaxLength:                  dc.GetIntPropertyFilteredByDomain(dynamicproperties.ActivityIDMaxLength),
		ActivityTypeMaxLength:                dc.GetIntPropertyFilteredByDomain(dynamicproperties.ActivityTypeMaxLength),
		MarkerNameMaxLength:                  dc.GetIntPropertyFilteredByDomain(dynamicproperties.MarkerNameMaxLength),
		TimerIDMaxLength:                     dc.GetIntPropertyFilteredByDomain(dynamicproperties.TimerIDMaxLength),
		PersistenceMaxQPS:                    dc.GetIntProperty(dynamicproperties.HistoryPersistenceMaxQPS),
		PersistenceGlobalMaxQPS:              dc.GetIntProperty(dynamicproperties.HistoryPersistenceGlobalMaxQPS),
		ShutdownDrainDuration:                dc.GetDurationProperty(dynamicproperties.HistoryShutdownDrainDuration),
		EnableVisibilitySampling:             dc.GetBoolProperty(dynamicproperties.EnableVisibilitySampling),
		EnableReadFromClosedExecutionV2:      dc.GetBoolProperty(dynamicproperties.EnableReadFromClosedExecutionV2),
		VisibilityOpenMaxQPS:                 dc.GetIntPropertyFilteredByDomain(dynamicproperties.HistoryVisibilityOpenMaxQPS),
		VisibilityClosedMaxQPS:               dc.GetIntPropertyFilteredByDomain(dynamicproperties.HistoryVisibilityClosedMaxQPS),
		MaxAutoResetPoints:                   dc.GetIntPropertyFilteredByDomain(dynamicproperties.HistoryMaxAutoResetPoints),
		MaxDecisionStartToCloseSeconds:       dc.GetIntPropertyFilteredByDomain(dynamicproperties.MaxDecisionStartToCloseSeconds),
		WriteVisibilityStoreName:             dc.GetStringProperty(dynamicproperties.WriteVisibilityStoreName),
		EmitShardDiffLog:                     dc.GetBoolProperty(dynamicproperties.EmitShardDiffLog),
		HistoryCacheInitialSize:              dc.GetIntProperty(dynamicproperties.HistoryCacheInitialSize),
		HistoryCacheMaxSize:                  dc.GetIntProperty(dynamicproperties.HistoryCacheMaxSize),
		ExecutionCacheMaxByteSize:            dc.GetIntProperty(dynamicproperties.ExecutionCacheMaxByteSize),
		HistoryCacheTTL:                      dc.GetDurationProperty(dynamicproperties.HistoryCacheTTL),
		EnableSizeBasedHistoryExecutionCache: dc.GetBoolProperty(dynamicproperties.EnableSizeBasedHistoryExecutionCache),
		EventsCacheInitialCount:              dc.GetIntProperty(dynamicproperties.EventsCacheInitialCount),
		EventsCacheMaxCount:                  dc.GetIntProperty(dynamicproperties.EventsCacheMaxCount),
		EventsCacheMaxSize:                   dc.GetIntProperty(dynamicproperties.EventsCacheMaxSize),
		EventsCacheTTL:                       dc.GetDurationProperty(dynamicproperties.EventsCacheTTL),
		EventsCacheGlobalEnable:              dc.GetBoolProperty(dynamicproperties.EventsCacheGlobalEnable),
		EventsCacheGlobalInitialCount:        dc.GetIntProperty(dynamicproperties.EventsCacheGlobalInitialCount),
		EventsCacheGlobalMaxCount:            dc.GetIntProperty(dynamicproperties.EventsCacheGlobalMaxCount),
		EnableSizeBasedHistoryEventCache:     dc.GetBoolProperty(dynamicproperties.EnableSizeBasedHistoryEventCache),
		RangeSizeBits:                        20, // 20 bits for sequencer, 2^20 sequence number for any range
		AcquireShardInterval:                 dc.GetDurationProperty(dynamicproperties.AcquireShardInterval),
		AcquireShardConcurrency:              dc.GetIntProperty(dynamicproperties.AcquireShardConcurrency),
		StandbyClusterDelay:                  dc.GetDurationProperty(dynamicproperties.StandbyClusterDelay),
		StandbyTaskMissingEventsResendDelay:  dc.GetDurationProperty(dynamicproperties.StandbyTaskMissingEventsResendDelay),
		StandbyTaskMissingEventsDiscardDelay: dc.GetDurationProperty(dynamicproperties.StandbyTaskMissingEventsDiscardDelay),
		WorkflowDeletionJitterRange:          dc.GetIntPropertyFilteredByDomain(dynamicproperties.WorkflowDeletionJitterRange),
		DeleteHistoryEventContextTimeout:     dc.GetIntProperty(dynamicproperties.DeleteHistoryEventContextTimeout),
		MaxResponseSize:                      maxMessageSize,

		TaskProcessRPS:                           dc.GetIntPropertyFilteredByDomain(dynamicproperties.TaskProcessRPS),
		TaskSchedulerType:                        dc.GetIntProperty(dynamicproperties.TaskSchedulerType),
		TaskSchedulerWorkerCount:                 dc.GetIntProperty(dynamicproperties.TaskSchedulerWorkerCount),
		TaskSchedulerQueueSize:                   dc.GetIntProperty(dynamicproperties.TaskSchedulerQueueSize),
		TaskSchedulerDispatcherCount:             dc.GetIntProperty(dynamicproperties.TaskSchedulerDispatcherCount),
		TaskSchedulerRoundRobinWeights:           dc.GetMapProperty(dynamicproperties.TaskSchedulerRoundRobinWeights),
		TaskSchedulerDomainRoundRobinWeights:     dc.GetMapPropertyFilteredByDomain(dynamicproperties.TaskSchedulerDomainRoundRobinWeights),
		TaskSchedulerGlobalDomainRPS:             dc.GetIntPropertyFilteredByDomain(dynamicproperties.TaskSchedulerGlobalDomainRPS),
		TaskSchedulerEnableRateLimiter:           dc.GetBoolProperty(dynamicproperties.TaskSchedulerEnableRateLimiter),
		TaskSchedulerEnableRateLimiterShadowMode: dc.GetBoolPropertyFilteredByDomain(dynamicproperties.TaskSchedulerEnableRateLimiterShadowMode),
		TaskCriticalRetryCount:                   dc.GetIntProperty(dynamicproperties.TaskCriticalRetryCount),
		ActiveTaskRedispatchInterval:             dc.GetDurationProperty(dynamicproperties.ActiveTaskRedispatchInterval),
		StandbyTaskRedispatchInterval:            dc.GetDurationProperty(dynamicproperties.StandbyTaskRedispatchInterval),
		StandbyTaskReReplicationContextTimeout:   dc.GetDurationPropertyFilteredByDomainID(dynamicproperties.StandbyTaskReReplicationContextTimeout),
		EnableDropStuckTaskByDomainID:            dc.GetBoolPropertyFilteredByDomainID(dynamicproperties.EnableDropStuckTaskByDomainID),
		ResurrectionCheckMinDelay:                dc.GetDurationPropertyFilteredByDomain(dynamicproperties.ResurrectionCheckMinDelay),

		EnableTimerQueueV2:                         dc.GetBoolPropertyFilteredByShardID(dynamicproperties.EnableTimerQueueV2),
		EnableTransferQueueV2:                      dc.GetBoolPropertyFilteredByShardID(dynamicproperties.EnableTransferQueueV2),
		QueueMaxPendingTaskCount:                   dc.GetIntProperty(dynamicproperties.QueueMaxPendingTaskCount),
		EnableTimerQueueV2PendingTaskCountAlert:    dc.GetBoolPropertyFilteredByShardID(dynamicproperties.EnableTimerQueueV2PendingTaskCountAlert),
		EnableTransferQueueV2PendingTaskCountAlert: dc.GetBoolPropertyFilteredByShardID(dynamicproperties.EnableTransferQueueV2PendingTaskCountAlert),
		QueueCriticalPendingTaskCount:              dc.GetIntProperty(dynamicproperties.QueueCriticalPendingTaskCount),
		QueueMaxVirtualQueueCount:                  dc.GetIntProperty(dynamicproperties.QueueMaxVirtualQueueCount),
		VirtualSliceForceAppendInterval:            dc.GetDurationProperty(dynamicproperties.VirtualSliceForceAppendInterval),

		QueueProcessorEnableSplit:                          dc.GetBoolProperty(dynamicproperties.QueueProcessorEnableSplit),
		QueueProcessorSplitMaxLevel:                        dc.GetIntProperty(dynamicproperties.QueueProcessorSplitMaxLevel),
		QueueProcessorEnableRandomSplitByDomainID:          dc.GetBoolPropertyFilteredByDomainID(dynamicproperties.QueueProcessorEnableRandomSplitByDomainID),
		QueueProcessorRandomSplitProbability:               dc.GetFloat64Property(dynamicproperties.QueueProcessorRandomSplitProbability),
		QueueProcessorEnablePendingTaskSplitByDomainID:     dc.GetBoolPropertyFilteredByDomainID(dynamicproperties.QueueProcessorEnablePendingTaskSplitByDomainID),
		QueueProcessorPendingTaskSplitThreshold:            dc.GetMapProperty(dynamicproperties.QueueProcessorPendingTaskSplitThreshold),
		QueueProcessorEnableStuckTaskSplitByDomainID:       dc.GetBoolPropertyFilteredByDomainID(dynamicproperties.QueueProcessorEnableStuckTaskSplitByDomainID),
		QueueProcessorStuckTaskSplitThreshold:              dc.GetMapProperty(dynamicproperties.QueueProcessorStuckTaskSplitThreshold),
		QueueProcessorSplitLookAheadDurationByDomainID:     dc.GetDurationPropertyFilteredByDomainID(dynamicproperties.QueueProcessorSplitLookAheadDurationByDomainID),
		QueueProcessorPollBackoffInterval:                  dc.GetDurationProperty(dynamicproperties.QueueProcessorPollBackoffInterval),
		QueueProcessorPollBackoffIntervalJitterCoefficient: dc.GetFloat64Property(dynamicproperties.QueueProcessorPollBackoffIntervalJitterCoefficient),
		QueueProcessorEnablePersistQueueStates:             dc.GetBoolProperty(dynamicproperties.QueueProcessorEnablePersistQueueStates),
		QueueProcessorEnableLoadQueueStates:                dc.GetBoolProperty(dynamicproperties.QueueProcessorEnableLoadQueueStates),
		QueueProcessorEnableGracefulSyncShutdown:           dc.GetBoolProperty(dynamicproperties.QueueProcessorEnableGracefulSyncShutdown),

		TimerTaskBatchSize:                                   dc.GetIntProperty(dynamicproperties.TimerTaskBatchSize),
		TimerTaskDeleteBatchSize:                             dc.GetIntProperty(dynamicproperties.TimerTaskDeleteBatchSize),
		TimerProcessorGetFailureRetryCount:                   dc.GetIntProperty(dynamicproperties.TimerProcessorGetFailureRetryCount),
		TimerProcessorCompleteTimerFailureRetryCount:         dc.GetIntProperty(dynamicproperties.TimerProcessorCompleteTimerFailureRetryCount),
		TimerProcessorUpdateAckInterval:                      dc.GetDurationProperty(dynamicproperties.TimerProcessorUpdateAckInterval),
		TimerProcessorUpdateAckIntervalJitterCoefficient:     dc.GetFloat64Property(dynamicproperties.TimerProcessorUpdateAckIntervalJitterCoefficient),
		TimerProcessorCompleteTimerInterval:                  dc.GetDurationProperty(dynamicproperties.TimerProcessorCompleteTimerInterval),
		TimerProcessorFailoverMaxStartJitterInterval:         dc.GetDurationProperty(dynamicproperties.TimerProcessorFailoverMaxStartJitterInterval),
		TimerProcessorFailoverMaxPollRPS:                     dc.GetIntProperty(dynamicproperties.TimerProcessorFailoverMaxPollRPS),
		TimerProcessorMaxPollRPS:                             dc.GetIntProperty(dynamicproperties.TimerProcessorMaxPollRPS),
		TimerProcessorMaxPollInterval:                        dc.GetDurationProperty(dynamicproperties.TimerProcessorMaxPollInterval),
		TimerProcessorMaxPollIntervalJitterCoefficient:       dc.GetFloat64Property(dynamicproperties.TimerProcessorMaxPollIntervalJitterCoefficient),
		TimerProcessorSplitQueueInterval:                     dc.GetDurationProperty(dynamicproperties.TimerProcessorSplitQueueInterval),
		TimerProcessorSplitQueueIntervalJitterCoefficient:    dc.GetFloat64Property(dynamicproperties.TimerProcessorSplitQueueIntervalJitterCoefficient),
		TimerProcessorMaxRedispatchQueueSize:                 dc.GetIntProperty(dynamicproperties.TimerProcessorMaxRedispatchQueueSize),
		TimerProcessorMaxTimeShift:                           dc.GetDurationProperty(dynamicproperties.TimerProcessorMaxTimeShift),
		TimerProcessorInMemoryQueueMaxTimeShift:              dc.GetDurationPropertyFilteredByShardID(dynamicproperties.TimerProcessorInMemoryQueueMaxTimeShift),
		TimerProcessorHistoryArchivalSizeLimit:               dc.GetIntProperty(dynamicproperties.TimerProcessorHistoryArchivalSizeLimit),
		TimerProcessorArchivalTimeLimit:                      dc.GetDurationProperty(dynamicproperties.TimerProcessorArchivalTimeLimit),
		DisableTimerFailoverQueue:                            dc.GetBoolProperty(dynamicproperties.DisableTimerFailoverQueue),
		TransferTaskBatchSize:                                dc.GetIntProperty(dynamicproperties.TransferTaskBatchSize),
		TransferTaskDeleteBatchSize:                          dc.GetIntProperty(dynamicproperties.TransferTaskDeleteBatchSize),
		TransferProcessorFailoverMaxStartJitterInterval:      dc.GetDurationProperty(dynamicproperties.TransferProcessorFailoverMaxStartJitterInterval),
		TransferProcessorFailoverMaxPollRPS:                  dc.GetIntProperty(dynamicproperties.TransferProcessorFailoverMaxPollRPS),
		TransferProcessorMaxPollRPS:                          dc.GetIntProperty(dynamicproperties.TransferProcessorMaxPollRPS),
		TransferProcessorCompleteTransferFailureRetryCount:   dc.GetIntProperty(dynamicproperties.TransferProcessorCompleteTransferFailureRetryCount),
		TransferProcessorMaxPollInterval:                     dc.GetDurationProperty(dynamicproperties.TransferProcessorMaxPollInterval),
		TransferProcessorMaxPollIntervalJitterCoefficient:    dc.GetFloat64Property(dynamicproperties.TransferProcessorMaxPollIntervalJitterCoefficient),
		TransferProcessorSplitQueueInterval:                  dc.GetDurationProperty(dynamicproperties.TransferProcessorSplitQueueInterval),
		TransferProcessorSplitQueueIntervalJitterCoefficient: dc.GetFloat64Property(dynamicproperties.TransferProcessorSplitQueueIntervalJitterCoefficient),
		TransferProcessorUpdateAckInterval:                   dc.GetDurationProperty(dynamicproperties.TransferProcessorUpdateAckInterval),
		TransferProcessorUpdateAckIntervalJitterCoefficient:  dc.GetFloat64Property(dynamicproperties.TransferProcessorUpdateAckIntervalJitterCoefficient),
		TransferProcessorCompleteTransferInterval:            dc.GetDurationProperty(dynamicproperties.TransferProcessorCompleteTransferInterval),
		TransferProcessorMaxRedispatchQueueSize:              dc.GetIntProperty(dynamicproperties.TransferProcessorMaxRedispatchQueueSize),
		TransferProcessorEnableValidator:                     dc.GetBoolProperty(dynamicproperties.TransferProcessorEnableValidator),
		TransferProcessorValidationInterval:                  dc.GetDurationProperty(dynamicproperties.TransferProcessorValidationInterval),
		TransferProcessorVisibilityArchivalTimeLimit:         dc.GetDurationProperty(dynamicproperties.TransferProcessorVisibilityArchivalTimeLimit),
		DisableTransferFailoverQueue:                         dc.GetBoolProperty(dynamicproperties.DisableTransferFailoverQueue),

		ReplicatorTaskDeleteBatchSize:            dc.GetIntProperty(dynamicproperties.ReplicatorTaskDeleteBatchSize),
		ReplicatorReadTaskMaxRetryCount:          dc.GetIntProperty(dynamicproperties.ReplicatorReadTaskMaxRetryCount),
		ReplicatorProcessorFetchTasksBatchSize:   dc.GetIntPropertyFilteredByShardID(dynamicproperties.ReplicatorTaskBatchSize),
		ReplicatorProcessorMaxTaskBatchSize:      dc.GetIntPropertyFilteredByShardID(dynamicproperties.ReplicatorMaxTaskBatchSize),
		ReplicatorProcessorMinTaskBatchSize:      dc.GetIntPropertyFilteredByShardID(dynamicproperties.ReplicatorMinTaskBatchSize),
		ReplicatorProcessorBatchSizeStepCount:    dc.GetIntPropertyFilteredByShardID(dynamicproperties.ReplicatorTaskBatchStepCount),
		ReplicatorUpperLatency:                   dc.GetDurationProperty(dynamicproperties.ReplicatorUpperLatency),
		ReplicatorCacheCapacity:                  dc.GetIntProperty(dynamicproperties.ReplicatorCacheCapacity),
		ReplicatorCacheMaxSize:                   dc.GetIntProperty(dynamicproperties.ReplicatorCacheMaxSize),
		ReplicationBudgetManagerMaxSizeBytes:     dc.GetIntProperty(dynamicproperties.ReplicationBudgetManagerMaxSizeBytes),
		ReplicationBudgetManagerMaxSizeCount:     dc.GetIntProperty(dynamicproperties.ReplicationBudgetManagerMaxSizeCount),
		ReplicationBudgetManagerSoftCapThreshold: dc.GetFloat64Property(dynamicproperties.ReplicationBudgetManagerSoftCapThreshold),

		MaximumBufferedEventsBatch:      dc.GetIntProperty(dynamicproperties.MaximumBufferedEventsBatch),
		MaximumSignalsPerExecution:      dc.GetIntPropertyFilteredByDomain(dynamicproperties.MaximumSignalsPerExecution),
		ShardUpdateMinInterval:          dc.GetDurationProperty(dynamicproperties.ShardUpdateMinInterval),
		ShardSyncMinInterval:            dc.GetDurationProperty(dynamicproperties.ShardSyncMinInterval),
		ShardSyncTimerJitterCoefficient: dc.GetFloat64Property(dynamicproperties.TransferProcessorMaxPollIntervalJitterCoefficient),

		// history client: client/history/client.go set the client timeout 30s
		LongPollExpirationInterval:          dc.GetDurationPropertyFilteredByDomain(dynamicproperties.HistoryLongPollExpirationInterval),
		EventEncodingType:                   dc.GetStringPropertyFilteredByDomain(dynamicproperties.DefaultEventEncoding),
		EnableParentClosePolicy:             dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableParentClosePolicy),
		NumParentClosePolicySystemWorkflows: dc.GetIntProperty(dynamicproperties.NumParentClosePolicySystemWorkflows),
		EnableParentClosePolicyWorker:       dc.GetBoolProperty(dynamicproperties.EnableParentClosePolicyWorker),
		ParentClosePolicyThreshold:          dc.GetIntPropertyFilteredByDomain(dynamicproperties.ParentClosePolicyThreshold),
		ParentClosePolicyBatchSize:          dc.GetIntPropertyFilteredByDomain(dynamicproperties.ParentClosePolicyBatchSize),

		NumArchiveSystemWorkflows:        dc.GetIntProperty(dynamicproperties.NumArchiveSystemWorkflows),
		ArchiveRequestRPS:                dc.GetIntProperty(dynamicproperties.ArchiveRequestRPS),
		ArchiveInlineHistoryRPS:          dc.GetIntProperty(dynamicproperties.ArchiveInlineHistoryRPS),
		ArchiveInlineHistoryGlobalRPS:    dc.GetIntProperty(dynamicproperties.ArchiveInlineHistoryGlobalRPS),
		ArchiveInlineVisibilityRPS:       dc.GetIntProperty(dynamicproperties.ArchiveInlineVisibilityRPS),
		ArchiveInlineVisibilityGlobalRPS: dc.GetIntProperty(dynamicproperties.ArchiveInlineVisibilityGlobalRPS),
		AllowArchivingIncompleteHistory:  dc.GetBoolProperty(dynamicproperties.AllowArchivingIncompleteHistory),

		BlobSizeLimitError:               dc.GetIntPropertyFilteredByDomain(dynamicproperties.BlobSizeLimitError),
		BlobSizeLimitWarn:                dc.GetIntPropertyFilteredByDomain(dynamicproperties.BlobSizeLimitWarn),
		HistorySizeLimitError:            dc.GetIntPropertyFilteredByDomain(dynamicproperties.HistorySizeLimitError),
		HistorySizeLimitWarn:             dc.GetIntPropertyFilteredByDomain(dynamicproperties.HistorySizeLimitWarn),
		HistoryCountLimitError:           dc.GetIntPropertyFilteredByDomain(dynamicproperties.HistoryCountLimitError),
		HistoryCountLimitWarn:            dc.GetIntPropertyFilteredByDomain(dynamicproperties.HistoryCountLimitWarn),
		PendingActivitiesCountLimitError: dc.GetIntProperty(dynamicproperties.PendingActivitiesCountLimitError),
		PendingActivitiesCountLimitWarn:  dc.GetIntProperty(dynamicproperties.PendingActivitiesCountLimitWarn),
		PendingActivityValidationEnabled: dc.GetBoolProperty(dynamicproperties.EnablePendingActivityValidation),

		ThrottledLogRPS:   dc.GetIntProperty(dynamicproperties.HistoryThrottledLogRPS),
		EnableStickyQuery: dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableStickyQuery),

		EnableQueryAttributeValidation:           dc.GetBoolProperty(dynamicproperties.EnableQueryAttributeValidation),
		ValidSearchAttributes:                    dc.GetMapProperty(dynamicproperties.ValidSearchAttributes),
		SearchAttributesNumberOfKeysLimit:        dc.GetIntPropertyFilteredByDomain(dynamicproperties.SearchAttributesNumberOfKeysLimit),
		SearchAttributesSizeOfValueLimit:         dc.GetIntPropertyFilteredByDomain(dynamicproperties.SearchAttributesSizeOfValueLimit),
		SearchAttributesTotalSizeLimit:           dc.GetIntPropertyFilteredByDomain(dynamicproperties.SearchAttributesTotalSizeLimit),
		SearchAttributesHiddenValueKeys:          dc.GetMapProperty(dynamicproperties.SearchAttributesHiddenValueKeys),
		StickyTTL:                                dc.GetDurationPropertyFilteredByDomain(dynamicproperties.StickyTTL),
		DecisionHeartbeatTimeout:                 dc.GetDurationPropertyFilteredByDomain(dynamicproperties.DecisionHeartbeatTimeout),
		DecisionRetryCriticalAttempts:            dc.GetIntProperty(dynamicproperties.DecisionRetryCriticalAttempts),
		DecisionRetryMaxAttempts:                 dc.GetIntPropertyFilteredByDomain(dynamicproperties.DecisionRetryMaxAttempts),
		EnforceDecisionTaskAttempts:              dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnforceDecisionTaskAttempts),
		NormalDecisionScheduleToStartMaxAttempts: dc.GetIntPropertyFilteredByDomain(dynamicproperties.NormalDecisionScheduleToStartMaxAttempts),
		NormalDecisionScheduleToStartTimeout:     dc.GetDurationPropertyFilteredByDomain(dynamicproperties.NormalDecisionScheduleToStartTimeout),

		ReplicationTaskFetcherParallelism:                    dc.GetIntProperty(dynamicproperties.ReplicationTaskFetcherParallelism),
		ReplicationTaskFetcherAggregationInterval:            dc.GetDurationProperty(dynamicproperties.ReplicationTaskFetcherAggregationInterval),
		ReplicationTaskFetcherTimerJitterCoefficient:         dc.GetFloat64Property(dynamicproperties.ReplicationTaskFetcherTimerJitterCoefficient),
		ReplicationTaskFetcherErrorRetryWait:                 dc.GetDurationProperty(dynamicproperties.ReplicationTaskFetcherErrorRetryWait),
		ReplicationTaskFetcherServiceBusyWait:                dc.GetDurationProperty(dynamicproperties.ReplicationTaskFetcherServiceBusyWait),
		ReplicationTaskProcessorErrorRetryWait:               dc.GetDurationPropertyFilteredByShardID(dynamicproperties.ReplicationTaskProcessorErrorRetryWait),
		ReplicationTaskProcessorErrorRetryMaxAttempts:        dc.GetIntPropertyFilteredByShardID(dynamicproperties.ReplicationTaskProcessorErrorRetryMaxAttempts),
		ReplicationTaskProcessorErrorSecondRetryWait:         dc.GetDurationPropertyFilteredByShardID(dynamicproperties.ReplicationTaskProcessorErrorSecondRetryWait),
		ReplicationTaskProcessorErrorSecondRetryMaxWait:      dc.GetDurationPropertyFilteredByShardID(dynamicproperties.ReplicationTaskProcessorErrorSecondRetryMaxWait),
		ReplicationTaskProcessorErrorSecondRetryExpiration:   dc.GetDurationPropertyFilteredByShardID(dynamicproperties.ReplicationTaskProcessorErrorSecondRetryExpiration),
		ReplicationTaskProcessorNoTaskRetryWait:              dc.GetDurationPropertyFilteredByShardID(dynamicproperties.ReplicationTaskProcessorNoTaskInitialWait),
		ReplicationTaskProcessorCleanupInterval:              dc.GetDurationPropertyFilteredByShardID(dynamicproperties.ReplicationTaskProcessorCleanupInterval),
		ReplicationTaskProcessorCleanupJitterCoefficient:     dc.GetFloat64PropertyFilteredByShardID(dynamicproperties.ReplicationTaskProcessorCleanupJitterCoefficient),
		ReplicationTaskProcessorStartWait:                    dc.GetDurationPropertyFilteredByShardID(dynamicproperties.ReplicationTaskProcessorStartWait),
		ReplicationTaskProcessorStartWaitJitterCoefficient:   dc.GetFloat64PropertyFilteredByShardID(dynamicproperties.ReplicationTaskProcessorStartWaitJitterCoefficient),
		ReplicationTaskProcessorHostQPS:                      dc.GetFloat64Property(dynamicproperties.ReplicationTaskProcessorHostQPS),
		ReplicationTaskProcessorShardQPS:                     dc.GetFloat64Property(dynamicproperties.ReplicationTaskProcessorShardQPS),
		ReplicationTaskGenerationQPS:                         dc.GetFloat64Property(dynamicproperties.ReplicationTaskGenerationQPS),
		EnableReplicationTaskGeneration:                      dc.GetBoolPropertyFilteredByDomainIDAndWorkflowID(dynamicproperties.EnableReplicationTaskGeneration),
		EnableRecordWorkflowExecutionUninitialized:           dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableRecordWorkflowExecutionUninitialized),
		EnableCleanupOrphanedHistoryBranchOnWorkflowCreation: dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableCleanupOrphanedHistoryBranchOnWorkflowCreation),
		ReplicationTaskProcessorLatencyLogThreshold:          dc.GetDurationProperty(dynamicproperties.ReplicationTaskProcessorLatencyLogThreshold),

		WorkflowIDExternalRPS: dc.GetIntPropertyFilteredByDomain(dynamicproperties.WorkflowIDExternalRPS),
		WorkflowIDInternalRPS: dc.GetIntPropertyFilteredByDomain(dynamicproperties.WorkflowIDInternalRPS),

		EnableConsistentQuery:                 dc.GetBoolProperty(dynamicproperties.EnableConsistentQuery),
		EnableConsistentQueryByDomain:         dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableConsistentQueryByDomain),
		EnableContextHeaderInVisibility:       dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableContextHeaderInVisibility),
		EnableCrossClusterOperationsForDomain: dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableCrossClusterOperationsForDomain),
		MaxBufferedQueryCount:                 dc.GetIntProperty(dynamicproperties.MaxBufferedQueryCount),
		MutableStateChecksumGenProbability:    dc.GetIntPropertyFilteredByDomain(dynamicproperties.MutableStateChecksumGenProbability),
		MutableStateChecksumVerifyProbability: dc.GetIntPropertyFilteredByDomain(dynamicproperties.MutableStateChecksumVerifyProbability),
		MutableStateChecksumInvalidateBefore:  dc.GetFloat64Property(dynamicproperties.MutableStateChecksumInvalidateBefore),
		EnableRetryForChecksumFailure:         dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableRetryForChecksumFailure),

		EnableHistoryCorruptionCheck: dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableHistoryCorruptionCheck),

		NotifyFailoverMarkerInterval:               dc.GetDurationProperty(dynamicproperties.NotifyFailoverMarkerInterval),
		NotifyFailoverMarkerTimerJitterCoefficient: dc.GetFloat64Property(dynamicproperties.NotifyFailoverMarkerTimerJitterCoefficient),
		EnableGracefulFailover:                     dc.GetBoolProperty(dynamicproperties.EnableGracefulFailover),

		EnableActivityLocalDispatchByDomain: dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableActivityLocalDispatchByDomain),
		MaxActivityCountDispatchByDomain:    dc.GetIntPropertyFilteredByDomain(dynamicproperties.MaxActivityCountDispatchByDomain),

		ActivityMaxScheduleToStartTimeoutForRetry: dc.GetDurationPropertyFilteredByDomain(dynamicproperties.ActivityMaxScheduleToStartTimeoutForRetry),

		EnableDebugMode:               dc.GetBoolProperty(dynamicproperties.EnableDebugMode)(),
		EnableTaskInfoLogByDomainID:   dc.GetBoolPropertyFilteredByDomainID(dynamicproperties.HistoryEnableTaskInfoLogByDomainID),
		EnableTimerDebugLogByDomainID: dc.GetBoolPropertyFilteredByDomainID(dynamicproperties.EnableTimerDebugLogByDomainID),

		SampleLoggingRate:                     dc.GetIntProperty(dynamicproperties.SampleLoggingRate),
		EnableShardIDMetrics:                  dc.GetBoolProperty(dynamicproperties.EnableShardIDMetrics),
		LargeShardHistorySizeMetricThreshold:  dc.GetIntProperty(dynamicproperties.LargeShardHistorySizeMetricThreshold),
		LargeShardHistoryEventMetricThreshold: dc.GetIntProperty(dynamicproperties.LargeShardHistoryEventMetricThreshold),
		LargeShardHistoryBlobMetricThreshold:  dc.GetIntProperty(dynamicproperties.LargeShardHistoryBlobMetricThreshold),

		EnableStrongIdempotency:            dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableStrongIdempotency),
		EnableStrongIdempotencySanityCheck: dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableStrongIdempotencySanityCheck),

		GlobalRatelimiterNewDataWeight:  dc.GetFloat64Property(dynamicproperties.HistoryGlobalRatelimiterNewDataWeight),
		GlobalRatelimiterUpdateInterval: dc.GetDurationProperty(dynamicproperties.GlobalRatelimiterUpdateInterval),
		GlobalRatelimiterDecayAfter:     dc.GetDurationProperty(dynamicproperties.HistoryGlobalRatelimiterDecayAfter),
		GlobalRatelimiterGCAfter:        dc.GetDurationProperty(dynamicproperties.HistoryGlobalRatelimiterGCAfter),

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
	panicIfErr(inMem.UpdateValue(dynamicproperties.HistoryLongPollExpirationInterval, 10*time.Second))
	panicIfErr(inMem.UpdateValue(dynamicproperties.EnableConsistentQueryByDomain, true))
	panicIfErr(inMem.UpdateValue(dynamicproperties.ReplicationTaskProcessorHostQPS, float64(10000)))
	panicIfErr(inMem.UpdateValue(dynamicproperties.ReplicationTaskProcessorCleanupInterval, 20*time.Millisecond))
	panicIfErr(inMem.UpdateValue(dynamicproperties.ReplicatorTaskDeleteBatchSize, 50))
	panicIfErr(inMem.UpdateValue(dynamicproperties.ShardSyncMinInterval, 20*time.Millisecond))
	panicIfErr(inMem.UpdateValue(dynamicproperties.ReplicationTaskProcessorShardQPS, float64(10000)))
	panicIfErr(inMem.UpdateValue(dynamicproperties.ReplicationTaskProcessorStartWait, time.Nanosecond))
	panicIfErr(inMem.UpdateValue(dynamicproperties.EnableActivityLocalDispatchByDomain, true))
	panicIfErr(inMem.UpdateValue(dynamicproperties.MaxActivityCountDispatchByDomain, 0))
	panicIfErr(inMem.UpdateValue(dynamicproperties.EnableCrossClusterOperationsForDomain, true))
	panicIfErr(inMem.UpdateValue(dynamicproperties.NormalDecisionScheduleToStartMaxAttempts, 3))
	panicIfErr(inMem.UpdateValue(dynamicproperties.EnablePendingActivityValidation, true))
	panicIfErr(inMem.UpdateValue(dynamicproperties.QueueProcessorEnableGracefulSyncShutdown, true))
	panicIfErr(inMem.UpdateValue(dynamicproperties.QueueProcessorSplitMaxLevel, 2))
	panicIfErr(inMem.UpdateValue(dynamicproperties.QueueProcessorPendingTaskSplitThreshold, map[string]interface{}{
		"0": 1000,
		"1": 5000,
	}))
	panicIfErr(inMem.UpdateValue(dynamicproperties.QueueProcessorStuckTaskSplitThreshold, map[string]interface{}{
		"0": 10,
		"1": 50,
	}))
	panicIfErr(inMem.UpdateValue(dynamicproperties.QueueProcessorRandomSplitProbability, 0.5))
	panicIfErr(inMem.UpdateValue(dynamicproperties.EnableStrongIdempotency, true))

	dc := dynamicconfig.NewCollection(inMem, log.NewNoop())
	config := New(dc, shardNumber, 1024*1024, false, "")
	// reduce the duration of long poll to increase test speed
	config.LongPollExpirationInterval = dc.GetDurationPropertyFilteredByDomain(dynamicproperties.HistoryLongPollExpirationInterval)
	config.EnableConsistentQueryByDomain = dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableConsistentQueryByDomain)
	config.ReplicationTaskProcessorHostQPS = dc.GetFloat64Property(dynamicproperties.ReplicationTaskProcessorHostQPS)
	config.ReplicationTaskProcessorCleanupInterval = dc.GetDurationPropertyFilteredByShardID(dynamicproperties.ReplicationTaskProcessorCleanupInterval)
	config.ReplicatorTaskDeleteBatchSize = dc.GetIntProperty(dynamicproperties.ReplicatorTaskDeleteBatchSize)
	config.ShardSyncMinInterval = dc.GetDurationProperty(dynamicproperties.ShardSyncMinInterval)
	config.ReplicationTaskProcessorShardQPS = dc.GetFloat64Property(dynamicproperties.ReplicationTaskProcessorShardQPS)
	config.ReplicationTaskProcessorStartWait = dc.GetDurationPropertyFilteredByShardID(dynamicproperties.ReplicationTaskProcessorStartWait)
	config.EnableActivityLocalDispatchByDomain = dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableActivityLocalDispatchByDomain)
	config.MaxActivityCountDispatchByDomain = dc.GetIntPropertyFilteredByDomain(dynamicproperties.MaxActivityCountDispatchByDomain)
	config.EnableCrossClusterOperationsForDomain = dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableCrossClusterOperationsForDomain)
	config.NormalDecisionScheduleToStartMaxAttempts = dc.GetIntPropertyFilteredByDomain(dynamicproperties.NormalDecisionScheduleToStartMaxAttempts)
	config.NormalDecisionScheduleToStartTimeout = dc.GetDurationPropertyFilteredByDomain(dynamicproperties.NormalDecisionScheduleToStartTimeout)
	config.PendingActivityValidationEnabled = dc.GetBoolProperty(dynamicproperties.EnablePendingActivityValidation)
	config.QueueProcessorEnableGracefulSyncShutdown = dc.GetBoolProperty(dynamicproperties.QueueProcessorEnableGracefulSyncShutdown)
	config.QueueProcessorSplitMaxLevel = dc.GetIntProperty(dynamicproperties.QueueProcessorSplitMaxLevel)
	config.QueueProcessorPendingTaskSplitThreshold = dc.GetMapProperty(dynamicproperties.QueueProcessorPendingTaskSplitThreshold)
	config.QueueProcessorStuckTaskSplitThreshold = dc.GetMapProperty(dynamicproperties.QueueProcessorStuckTaskSplitThreshold)
	config.QueueProcessorRandomSplitProbability = dc.GetFloat64Property(dynamicproperties.QueueProcessorRandomSplitProbability)
	return config
}

// GetShardID return the corresponding shard ID for a given workflow ID
func (config *Config) GetShardID(workflowID string) int {
	return common.WorkflowIDToHistoryShard(workflowID, config.NumberOfShards)
}
