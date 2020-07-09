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

package metrics

import "github.com/uber-go/tally"

// types used/defined by the package
type (
	// MetricName is the name of the metric
	MetricName string

	// MetricType is the type of the metric
	MetricType int

	// metricDefinition contains the definition for a metric
	metricDefinition struct {
		//nolint
		metricType       MetricType    // metric type
		metricName       MetricName    // metric name
		metricRollupName MetricName    // optional. if non-empty, this name must be used for rolled-up version of this metric
		buckets          tally.Buckets // buckets if we are emitting histograms
	}

	// scopeDefinition holds the tag definitions for a scope
	scopeDefinition struct {
		operation string            // 'operation' tag for scope
		tags      map[string]string // additional tags for scope
	}

	// ServiceIdx is an index that uniquely identifies the service
	ServiceIdx int
)

// MetricTypes which are supported
const (
	Counter MetricType = iota
	Timer
	Gauge
)

// Service names for all services that emit metrics.
const (
	Common = iota
	Frontend
	History
	Matching
	Worker
	NumServices
)

// Common tags for all services
const (
	OperationTagName   = "operation"
	CadenceRoleTagName = "cadence_role"
	StatsTypeTagName   = "stats_type"
	CacheTypeTagName   = "cache_type"
)

// This package should hold all the metrics and tags for cadence
const (
	HistoryRoleTagValue       = "history"
	MatchingRoleTagValue      = "matching"
	FrontendRoleTagValue      = "frontend"
	AdminRoleTagValue         = "admin"
	DCRedirectionRoleTagValue = "dc_redirection"
	BlobstoreRoleTagValue     = "blobstore"

	SizeStatsTypeTagValue  = "size"
	CountStatsTypeTagValue = "count"

	MutableStateCacheTypeTagValue = "mutablestate"
	EventsCacheTypeTagValue       = "events"
)

// Common service base metrics
const (
	RestartCount         = "restarts"
	NumGoRoutinesGauge   = "num_goroutines"
	GoMaxProcsGauge      = "gomaxprocs"
	MemoryAllocatedGauge = "memory_allocated"
	MemoryHeapGauge      = "memory_heap"
	MemoryHeapIdleGauge  = "memory_heapidle"
	MemoryHeapInuseGauge = "memory_heapinuse"
	MemoryStackGauge     = "memory_stack"
	NumGCCounter         = "memory_num_gc"
	GcPauseMsTimer       = "memory_gc_pause_ms"
)

// ServiceMetrics are types for common service base metrics
var ServiceMetrics = map[MetricName]MetricType{
	RestartCount: Counter,
}

// GoRuntimeMetrics represent the runtime stats from go runtime
var GoRuntimeMetrics = map[MetricName]MetricType{
	NumGoRoutinesGauge:   Gauge,
	GoMaxProcsGauge:      Gauge,
	MemoryAllocatedGauge: Gauge,
	MemoryHeapGauge:      Gauge,
	MemoryHeapIdleGauge:  Gauge,
	MemoryHeapInuseGauge: Gauge,
	MemoryStackGauge:     Gauge,
	NumGCCounter:         Counter,
	GcPauseMsTimer:       Timer,
}

// Scopes enum
const (
	// -- Common Operation scopes --

	// PersistenceCreateShardScope tracks CreateShard calls made by service to persistence layer
	PersistenceCreateShardScope = iota
	// PersistenceGetShardScope tracks GetShard calls made by service to persistence layer
	PersistenceGetShardScope
	// PersistenceUpdateShardScope tracks UpdateShard calls made by service to persistence layer
	PersistenceUpdateShardScope
	// PersistenceCreateWorkflowExecutionScope tracks CreateWorkflowExecution calls made by service to persistence layer
	PersistenceCreateWorkflowExecutionScope
	// PersistenceGetWorkflowExecutionScope tracks GetWorkflowExecution calls made by service to persistence layer
	PersistenceGetWorkflowExecutionScope
	// PersistenceUpdateWorkflowExecutionScope tracks UpdateWorkflowExecution calls made by service to persistence layer
	PersistenceUpdateWorkflowExecutionScope
	// PersistenceConflictResolveWorkflowExecutionScope tracks ConflictResolveWorkflowExecution calls made by service to persistence layer
	PersistenceConflictResolveWorkflowExecutionScope
	// PersistenceResetWorkflowExecutionScope tracks ResetWorkflowExecution calls made by service to persistence layer
	PersistenceResetWorkflowExecutionScope
	// PersistenceDeleteWorkflowExecutionScope tracks DeleteWorkflowExecution calls made by service to persistence layer
	PersistenceDeleteWorkflowExecutionScope
	// PersistenceDeleteCurrentWorkflowExecutionScope tracks DeleteCurrentWorkflowExecution calls made by service to persistence layer
	PersistenceDeleteCurrentWorkflowExecutionScope
	// PersistenceGetCurrentExecutionScope tracks GetCurrentExecution calls made by service to persistence layer
	PersistenceGetCurrentExecutionScope
	// PersistenceListConcreteExecutionsScope tracks ListConcreteExecutions calls made by service to persistence layer
	PersistenceListConcreteExecutionsScope
	// PersistenceGetTransferTasksScope tracks GetTransferTasks calls made by service to persistence layer
	PersistenceGetTransferTasksScope
	// PersistenceCompleteTransferTaskScope tracks CompleteTransferTasks calls made by service to persistence layer
	PersistenceCompleteTransferTaskScope
	// PersistenceRangeCompleteTransferTaskScope tracks CompleteTransferTasks calls made by service to persistence layer
	PersistenceRangeCompleteTransferTaskScope
	// PersistenceGetReplicationTasksScope tracks GetReplicationTasks calls made by service to persistence layer
	PersistenceGetReplicationTasksScope
	// PersistenceCompleteReplicationTaskScope tracks CompleteReplicationTasks calls made by service to persistence layer
	PersistenceCompleteReplicationTaskScope
	// PersistenceRangeCompleteReplicationTaskScope tracks RangeCompleteReplicationTasks calls made by service to persistence layer
	PersistenceRangeCompleteReplicationTaskScope
	// PersistencePutReplicationTaskToDLQScope tracks PersistencePutReplicationTaskToDLQScope calls made by service to persistence layer
	PersistencePutReplicationTaskToDLQScope
	// PersistenceGetReplicationTasksFromDLQScope tracks PersistenceGetReplicationTasksFromDLQScope calls made by service to persistence layer
	PersistenceGetReplicationTasksFromDLQScope
	// PersistenceDeleteReplicationTaskFromDLQScope tracks PersistenceDeleteReplicationTaskFromDLQScope calls made by service to persistence layer
	PersistenceDeleteReplicationTaskFromDLQScope
	// PersistenceRangeDeleteReplicationTaskFromDLQScope tracks PersistenceRangeDeleteReplicationTaskFromDLQScope calls made by service to persistence layer
	PersistenceRangeDeleteReplicationTaskFromDLQScope
	// PersistenceCreateFailoverMarkerTasksScope tracks CreateFailoverMarkerTasks calls made by service to persistence layer
	PersistenceCreateFailoverMarkerTasksScope
	// PersistenceGetTimerIndexTasksScope tracks GetTimerIndexTasks calls made by service to persistence layer
	PersistenceGetTimerIndexTasksScope
	// PersistenceCompleteTimerTaskScope tracks CompleteTimerTasks calls made by service to persistence layer
	PersistenceCompleteTimerTaskScope
	// PersistenceRangeCompleteTimerTaskScope tracks CompleteTimerTasks calls made by service to persistence layer
	PersistenceRangeCompleteTimerTaskScope
	// PersistenceCreateTaskScope tracks CreateTask calls made by service to persistence layer
	PersistenceCreateTaskScope
	// PersistenceGetTasksScope tracks GetTasks calls made by service to persistence layer
	PersistenceGetTasksScope
	// PersistenceCompleteTaskScope tracks CompleteTask calls made by service to persistence layer
	PersistenceCompleteTaskScope
	// PersistenceCompleteTasksLessThanScope is the metric scope for persistence.TaskManager.PersistenceCompleteTasksLessThan API
	PersistenceCompleteTasksLessThanScope
	// PersistenceLeaseTaskListScope tracks LeaseTaskList calls made by service to persistence layer
	PersistenceLeaseTaskListScope
	// PersistenceUpdateTaskListScope tracks PersistenceUpdateTaskListScope calls made by service to persistence layer
	PersistenceUpdateTaskListScope
	// PersistenceListTaskListScope is the metric scope for persistence.TaskManager.ListTaskList API
	PersistenceListTaskListScope
	// PersistenceDeleteTaskListScope is the metric scope for persistence.TaskManager.DeleteTaskList API
	PersistenceDeleteTaskListScope
	// PersistenceAppendHistoryEventsScope tracks AppendHistoryEvents calls made by service to persistence layer
	PersistenceAppendHistoryEventsScope
	// PersistenceGetWorkflowExecutionHistoryScope tracks GetWorkflowExecutionHistory calls made by service to persistence layer
	PersistenceGetWorkflowExecutionHistoryScope
	// PersistenceDeleteWorkflowExecutionHistoryScope tracks DeleteWorkflowExecutionHistory calls made by service to persistence layer
	PersistenceDeleteWorkflowExecutionHistoryScope
	// PersistenceCreateDomainScope tracks CreateDomain calls made by service to persistence layer
	PersistenceCreateDomainScope
	// PersistenceGetDomainScope tracks GetDomain calls made by service to persistence layer
	PersistenceGetDomainScope
	// PersistenceUpdateDomainScope tracks UpdateDomain calls made by service to persistence layer
	PersistenceUpdateDomainScope
	// PersistenceDeleteDomainScope tracks DeleteDomain calls made by service to persistence layer
	PersistenceDeleteDomainScope
	// PersistenceDeleteDomainByNameScope tracks DeleteDomainByName calls made by service to persistence layer
	PersistenceDeleteDomainByNameScope
	// PersistenceListDomainScope tracks DeleteDomainByName calls made by service to persistence layer
	PersistenceListDomainScope
	// PersistenceGetMetadataScope tracks DeleteDomainByName calls made by service to persistence layer
	PersistenceGetMetadataScope
	// PersistenceRecordWorkflowExecutionStartedScope tracks RecordWorkflowExecutionStarted calls made by service to persistence layer
	PersistenceRecordWorkflowExecutionStartedScope
	// PersistenceRecordWorkflowExecutionClosedScope tracks RecordWorkflowExecutionClosed calls made by service to persistence layer
	PersistenceRecordWorkflowExecutionClosedScope
	// PersistenceUpsertWorkflowExecutionScope tracks UpsertWorkflowExecution calls made by service to persistence layer
	PersistenceUpsertWorkflowExecutionScope
	// PersistenceListOpenWorkflowExecutionsScope tracks ListOpenWorkflowExecutions calls made by service to persistence layer
	PersistenceListOpenWorkflowExecutionsScope
	// PersistenceListClosedWorkflowExecutionsScope tracks ListClosedWorkflowExecutions calls made by service to persistence layer
	PersistenceListClosedWorkflowExecutionsScope
	// PersistenceListOpenWorkflowExecutionsByTypeScope tracks ListOpenWorkflowExecutionsByType calls made by service to persistence layer
	PersistenceListOpenWorkflowExecutionsByTypeScope
	// PersistenceListClosedWorkflowExecutionsByTypeScope tracks ListClosedWorkflowExecutionsByType calls made by service to persistence layer
	PersistenceListClosedWorkflowExecutionsByTypeScope
	// PersistenceListOpenWorkflowExecutionsByWorkflowIDScope tracks ListOpenWorkflowExecutionsByWorkflowID calls made by service to persistence layer
	PersistenceListOpenWorkflowExecutionsByWorkflowIDScope
	// PersistenceListClosedWorkflowExecutionsByWorkflowIDScope tracks ListClosedWorkflowExecutionsByWorkflowID calls made by service to persistence layer
	PersistenceListClosedWorkflowExecutionsByWorkflowIDScope
	// PersistenceListClosedWorkflowExecutionsByStatusScope tracks ListClosedWorkflowExecutionsByStatus calls made by service to persistence layer
	PersistenceListClosedWorkflowExecutionsByStatusScope
	// PersistenceGetClosedWorkflowExecutionScope tracks GetClosedWorkflowExecution calls made by service to persistence layer
	PersistenceGetClosedWorkflowExecutionScope
	// PersistenceVisibilityDeleteWorkflowExecutionScope is the metrics scope for persistence.VisibilityManager.DeleteWorkflowExecution
	PersistenceVisibilityDeleteWorkflowExecutionScope
	// PersistenceListWorkflowExecutionsScope tracks ListWorkflowExecutions calls made by service to persistence layer
	PersistenceListWorkflowExecutionsScope
	// PersistenceScanWorkflowExecutionsScope tracks ScanWorkflowExecutions calls made by service to persistence layer
	PersistenceScanWorkflowExecutionsScope
	// PersistenceCountWorkflowExecutionsScope tracks CountWorkflowExecutions calls made by service to persistence layer
	PersistenceCountWorkflowExecutionsScope
	// PersistenceEnqueueMessageScope tracks Enqueue calls made by service to persistence layer
	PersistenceEnqueueMessageScope
	// PersistenceEnqueueMessageToDLQScope tracks Enqueue DLQ calls made by service to persistence layer
	PersistenceEnqueueMessageToDLQScope
	// PersistenceReadQueueMessagesScope tracks ReadMessages calls made by service to persistence layer
	PersistenceReadQueueMessagesScope
	// PersistenceReadQueueMessagesFromDLQScope tracks ReadMessagesFromDLQ calls made by service to persistence layer
	PersistenceReadQueueMessagesFromDLQScope
	// PersistenceDeleteQueueMessagesScope tracks DeleteMessages calls made by service to persistence layer
	PersistenceDeleteQueueMessagesScope
	// PersistenceDeleteQueueMessageFromDLQScope tracks DeleteMessageFromDLQ calls made by service to persistence layer
	PersistenceDeleteQueueMessageFromDLQScope
	// PersistenceRangeDeleteMessagesFromDLQScope tracks RangeDeleteMessagesFromDLQ calls made by service to persistence layer
	PersistenceRangeDeleteMessagesFromDLQScope
	// PersistenceUpdateAckLevelScope tracks UpdateAckLevel calls made by service to persistence layer
	PersistenceUpdateAckLevelScope
	// PersistenceGetAckLevelScope tracks GetAckLevel calls made by service to persistence layer
	PersistenceGetAckLevelScope
	// PersistenceUpdateDLQAckLevelScope tracks UpdateDLQAckLevel calls made by service to persistence layer
	PersistenceUpdateDLQAckLevelScope
	// PersistenceGetDLQAckLevelScope tracks GetDLQAckLevel calls made by service to persistence layer
	PersistenceGetDLQAckLevelScope
	// HistoryClientStartWorkflowExecutionScope tracks RPC calls to history service
	HistoryClientStartWorkflowExecutionScope
	// HistoryClientRecordActivityTaskHeartbeatScope tracks RPC calls to history service
	HistoryClientRecordActivityTaskHeartbeatScope
	// HistoryClientRespondDecisionTaskCompletedScope tracks RPC calls to history service
	HistoryClientRespondDecisionTaskCompletedScope
	// HistoryClientRespondDecisionTaskFailedScope tracks RPC calls to history service
	HistoryClientRespondDecisionTaskFailedScope
	// HistoryClientRespondActivityTaskCompletedScope tracks RPC calls to history service
	HistoryClientRespondActivityTaskCompletedScope
	// HistoryClientRespondActivityTaskFailedScope tracks RPC calls to history service
	HistoryClientRespondActivityTaskFailedScope
	// HistoryClientRespondActivityTaskCanceledScope tracks RPC calls to history service
	HistoryClientRespondActivityTaskCanceledScope
	// HistoryClientGetMutableStateScope tracks RPC calls to history service
	HistoryClientGetMutableStateScope
	// HistoryClientPollMutableStateScope tracks RPC calls to history service
	HistoryClientPollMutableStateScope
	// HistoryClientResetStickyTaskListScope tracks RPC calls to history service
	HistoryClientResetStickyTaskListScope
	// HistoryClientDescribeWorkflowExecutionScope tracks RPC calls to history service
	HistoryClientDescribeWorkflowExecutionScope
	// HistoryClientRecordDecisionTaskStartedScope tracks RPC calls to history service
	HistoryClientRecordDecisionTaskStartedScope
	// HistoryClientRecordActivityTaskStartedScope tracks RPC calls to history service
	HistoryClientRecordActivityTaskStartedScope
	// HistoryClientRequestCancelWorkflowExecutionScope tracks RPC calls to history service
	HistoryClientRequestCancelWorkflowExecutionScope
	// HistoryClientSignalWorkflowExecutionScope tracks RPC calls to history service
	HistoryClientSignalWorkflowExecutionScope
	// HistoryClientSignalWithStartWorkflowExecutionScope tracks RPC calls to history service
	HistoryClientSignalWithStartWorkflowExecutionScope
	// HistoryClientRemoveSignalMutableStateScope tracks RPC calls to history service
	HistoryClientRemoveSignalMutableStateScope
	// HistoryClientTerminateWorkflowExecutionScope tracks RPC calls to history service
	HistoryClientTerminateWorkflowExecutionScope
	// HistoryClientResetWorkflowExecutionScope tracks RPC calls to history service
	HistoryClientResetWorkflowExecutionScope
	// HistoryClientScheduleDecisionTaskScope tracks RPC calls to history service
	HistoryClientScheduleDecisionTaskScope
	// HistoryClientRecordChildExecutionCompletedScope tracks RPC calls to history service
	HistoryClientRecordChildExecutionCompletedScope
	// HistoryClientReplicateEventsScope tracks RPC calls to history service
	HistoryClientReplicateEventsScope
	// HistoryClientReplicateRawEventsScope tracks RPC calls to history service
	HistoryClientReplicateRawEventsScope
	// HistoryClientSyncShardStatusScope tracks RPC calls to history service
	HistoryClientReplicateEventsV2Scope
	// HistoryClientReplicateRawEventsV2Scope tracks RPC calls to history service
	HistoryClientSyncShardStatusScope
	// HistoryClientSyncActivityScope tracks RPC calls to history service
	HistoryClientSyncActivityScope
	// HistoryClientGetReplicationTasksScope tracks RPC calls to history service
	HistoryClientGetReplicationTasksScope
	// HistoryClientGetDLQReplicationTasksScope tracks RPC calls to history service
	HistoryClientGetDLQReplicationTasksScope
	// HistoryClientQueryWorkflowScope tracks RPC calls to history service
	HistoryClientQueryWorkflowScope
	// HistoryClientReapplyEventsScope tracks RPC calls to history service
	HistoryClientReapplyEventsScope
	// HistoryClientReadDLQMessagesScope tracks RPC calls to history service
	HistoryClientReadDLQMessagesScope
	// HistoryClientPurgeDLQMessagesScope tracks RPC calls to history service
	HistoryClientPurgeDLQMessagesScope
	// HistoryClientMergeDLQMessagesScope tracks RPC calls to history service
	HistoryClientMergeDLQMessagesScope
	// HistoryClientRefreshWorkflowTasksScope tracks RPC calls to history service
	HistoryClientRefreshWorkflowTasksScope
	// HistoryClientNotifyFailoverMarkersScope tracks RPC calls to history service
	HistoryClientNotifyFailoverMarkersScope
	// MatchingClientPollForDecisionTaskScope tracks RPC calls to matching service
	MatchingClientPollForDecisionTaskScope
	// MatchingClientPollForActivityTaskScope tracks RPC calls to matching service
	MatchingClientPollForActivityTaskScope
	// MatchingClientAddActivityTaskScope tracks RPC calls to matching service
	MatchingClientAddActivityTaskScope
	// MatchingClientAddDecisionTaskScope tracks RPC calls to matching service
	MatchingClientAddDecisionTaskScope
	// MatchingClientQueryWorkflowScope tracks RPC calls to matching service
	MatchingClientQueryWorkflowScope
	// MatchingClientRespondQueryTaskCompletedScope tracks RPC calls to matching service
	MatchingClientRespondQueryTaskCompletedScope
	// MatchingClientCancelOutstandingPollScope tracks RPC calls to matching service
	MatchingClientCancelOutstandingPollScope
	// MatchingClientDescribeTaskListScope tracks RPC calls to matching service
	MatchingClientDescribeTaskListScope
	// MatchingClientListTaskListPartitionsScope tracks RPC calls to matching service
	MatchingClientListTaskListPartitionsScope
	// FrontendClientDeprecateDomainScope tracks RPC calls to frontend service
	FrontendClientDeprecateDomainScope
	// FrontendClientDescribeDomainScope tracks RPC calls to frontend service
	FrontendClientDescribeDomainScope
	// FrontendClientDescribeTaskListScope tracks RPC calls to frontend service
	FrontendClientDescribeTaskListScope
	// FrontendClientDescribeWorkflowExecutionScope tracks RPC calls to frontend service
	FrontendClientDescribeWorkflowExecutionScope
	// FrontendClientGetWorkflowExecutionHistoryScope tracks RPC calls to frontend service
	FrontendClientGetWorkflowExecutionHistoryScope
	// FrontendClientGetWorkflowExecutionRawHistoryScope tracks RPC calls to frontend service
	FrontendClientGetWorkflowExecutionRawHistoryScope
	// FrontendClientPollForWorkflowExecutionRawHistoryScope tracks RPC calls to frontend service
	FrontendClientPollForWorkflowExecutionRawHistoryScope
	// FrontendClientListArchivedWorkflowExecutionsScope tracks RPC calls to frontend service
	FrontendClientListArchivedWorkflowExecutionsScope
	// FrontendClientListClosedWorkflowExecutionsScope tracks RPC calls to frontend service
	FrontendClientListClosedWorkflowExecutionsScope
	// FrontendClientListDomainsScope tracks RPC calls to frontend service
	FrontendClientListDomainsScope
	// FrontendClientListOpenWorkflowExecutionsScope tracks RPC calls to frontend service
	FrontendClientListOpenWorkflowExecutionsScope
	// FrontendClientPollForActivityTaskScope tracks RPC calls to frontend service
	FrontendClientPollForActivityTaskScope
	// FrontendClientPollForDecisionTaskScope tracks RPC calls to frontend service
	FrontendClientPollForDecisionTaskScope
	// FrontendClientQueryWorkflowScope tracks RPC calls to frontend service
	FrontendClientQueryWorkflowScope
	// FrontendClientRecordActivityTaskHeartbeatScope tracks RPC calls to frontend service
	FrontendClientRecordActivityTaskHeartbeatScope
	// FrontendClientRecordActivityTaskHeartbeatByIDScope tracks RPC calls to frontend service
	FrontendClientRecordActivityTaskHeartbeatByIDScope
	// FrontendClientRegisterDomainScope tracks RPC calls to frontend service
	FrontendClientRegisterDomainScope
	// FrontendClientRequestCancelWorkflowExecutionScope tracks RPC calls to frontend service
	FrontendClientRequestCancelWorkflowExecutionScope
	// FrontendClientResetStickyTaskListScope tracks RPC calls to frontend service
	FrontendClientResetStickyTaskListScope
	// FrontendClientResetWorkflowExecutionScope tracks RPC calls to frontend service
	FrontendClientResetWorkflowExecutionScope
	// FrontendClientRespondActivityTaskCanceledScope tracks RPC calls to frontend service
	FrontendClientRespondActivityTaskCanceledScope
	// FrontendClientRespondActivityTaskCanceledByIDScope tracks RPC calls to frontend service
	FrontendClientRespondActivityTaskCanceledByIDScope
	// FrontendClientRespondActivityTaskCompletedScope tracks RPC calls to frontend service
	FrontendClientRespondActivityTaskCompletedScope
	// FrontendClientRespondActivityTaskCompletedByIDScope tracks RPC calls to frontend service
	FrontendClientRespondActivityTaskCompletedByIDScope
	// FrontendClientRespondActivityTaskFailedScope tracks RPC calls to frontend service
	FrontendClientRespondActivityTaskFailedScope
	// FrontendClientRespondActivityTaskFailedByIDScope tracks RPC calls to frontend service
	FrontendClientRespondActivityTaskFailedByIDScope
	// FrontendClientRespondDecisionTaskCompletedScope tracks RPC calls to frontend service
	FrontendClientRespondDecisionTaskCompletedScope
	// FrontendClientRespondDecisionTaskFailedScope tracks RPC calls to frontend service
	FrontendClientRespondDecisionTaskFailedScope
	// FrontendClientRespondQueryTaskCompletedScope tracks RPC calls to frontend service
	FrontendClientRespondQueryTaskCompletedScope
	// FrontendClientSignalWithStartWorkflowExecutionScope tracks RPC calls to frontend service
	FrontendClientSignalWithStartWorkflowExecutionScope
	// FrontendClientSignalWorkflowExecutionScope tracks RPC calls to frontend service
	FrontendClientSignalWorkflowExecutionScope
	// FrontendClientStartWorkflowExecutionScope tracks RPC calls to frontend service
	FrontendClientStartWorkflowExecutionScope
	// FrontendClientTerminateWorkflowExecutionScope tracks RPC calls to frontend service
	FrontendClientTerminateWorkflowExecutionScope
	// FrontendClientUpdateDomainScope tracks RPC calls to frontend service
	FrontendClientUpdateDomainScope
	// FrontendClientListWorkflowExecutionsScope tracks RPC calls to frontend service
	FrontendClientListWorkflowExecutionsScope
	// FrontendClientScanWorkflowExecutionsScope tracks RPC calls to frontend service
	FrontendClientScanWorkflowExecutionsScope
	// FrontendClientCountWorkflowExecutionsScope tracks RPC calls to frontend service
	FrontendClientCountWorkflowExecutionsScope
	// FrontendClientGetSearchAttributesScope tracks RPC calls to frontend service
	FrontendClientGetSearchAttributesScope
	// FrontendClientGetReplicationTasksScope tracks RPC calls to frontend service
	FrontendClientGetReplicationTasksScope
	// FrontendClientGetDomainReplicationTasksScope tracks RPC calls to frontend service
	FrontendClientGetDomainReplicationTasksScope
	// FrontendClientGetDLQReplicationTasksScope tracks RPC calls to frontend service
	FrontendClientGetDLQReplicationTasksScope
	// FrontendClientReapplyEventsScope tracks RPC calls to frontend service
	FrontendClientReapplyEventsScope
	// FrontendClientGetClusterInfoScope tracks RPC calls to frontend
	FrontendClientGetClusterInfoScope
	// FrontendClientListTaskListPartitionsScope tracks RPC calls to frontend service
	FrontendClientListTaskListPartitionsScope
	// AdminClientAddSearchAttributeScope tracks RPC calls to admin service
	AdminClientAddSearchAttributeScope
	// AdminClientCloseShardScope tracks RPC calls to admin service
	AdminClientCloseShardScope
	// AdminClientDescribeHistoryHostScope tracks RPC calls to admin service
	AdminClientDescribeHistoryHostScope
	// AdminClientDescribeWorkflowExecutionScope tracks RPC calls to admin service
	AdminClientDescribeWorkflowExecutionScope
	// AdminClientGetWorkflowExecutionRawHistoryScope tracks RPC calls to admin service
	AdminClientGetWorkflowExecutionRawHistoryScope
	// AdminClientGetWorkflowExecutionRawHistoryV2Scope tracks RPC calls to admin service
	AdminClientGetWorkflowExecutionRawHistoryV2Scope
	// AdminClientDescribeClusterScope tracks RPC calls to admin service
	AdminClientDescribeClusterScope
	// AdminClientReadDLQMessagesScope tracks RPC calls to admin service
	AdminClientReadDLQMessagesScope
	// AdminClientPurgeDLQMessagesScope tracks RPC calls to admin service
	AdminClientPurgeDLQMessagesScope
	// AdminClientMergeDLQMessagesScope tracks RPC calls to admin service
	AdminClientMergeDLQMessagesScope
	// AdminClientRefreshWorkflowTasksScope tracks RPC calls to admin service
	AdminClientRefreshWorkflowTasksScope
	// AdminClientResendReplicationTasksScope tracks RPC calls to admin service
	AdminClientResendReplicationTasksScope
	// DCRedirectionDeprecateDomainScope tracks RPC calls for dc redirection
	DCRedirectionDeprecateDomainScope
	// DCRedirectionDescribeDomainScope tracks RPC calls for dc redirection
	DCRedirectionDescribeDomainScope
	// DCRedirectionDescribeTaskListScope tracks RPC calls for dc redirection
	DCRedirectionDescribeTaskListScope
	// DCRedirectionDescribeWorkflowExecutionScope tracks RPC calls for dc redirection
	DCRedirectionDescribeWorkflowExecutionScope
	// DCRedirectionGetWorkflowExecutionHistoryScope tracks RPC calls for dc redirection
	DCRedirectionGetWorkflowExecutionHistoryScope
	// DCRedirectionGetWorkflowExecutionRawHistoryScope tracks RPC calls for dc redirection
	DCRedirectionGetWorkflowExecutionRawHistoryScope
	// DCRedirectionPollForWorklfowExecutionRawHistoryScope tracks RPC calls for dc redirection
	DCRedirectionPollForWorklfowExecutionRawHistoryScope
	// DCRedirectionListArchivedWorkflowExecutionsScope tracks RPC calls for dc redirection
	DCRedirectionListArchivedWorkflowExecutionsScope
	// DCRedirectionListClosedWorkflowExecutionsScope tracks RPC calls for dc redirection
	DCRedirectionListClosedWorkflowExecutionsScope
	// DCRedirectionListDomainsScope tracks RPC calls for dc redirection
	DCRedirectionListDomainsScope
	// DCRedirectionListOpenWorkflowExecutionsScope tracks RPC calls for dc redirection
	DCRedirectionListOpenWorkflowExecutionsScope
	// DCRedirectionListWorkflowExecutionsScope tracks RPC calls for dc redirection
	DCRedirectionListWorkflowExecutionsScope
	// DCRedirectionScanWorkflowExecutionsScope tracks RPC calls for dc redirection
	DCRedirectionScanWorkflowExecutionsScope
	// DCRedirectionCountWorkflowExecutionsScope tracks RPC calls for dc redirection
	DCRedirectionCountWorkflowExecutionsScope
	// DCRedirectionGetSearchAttributesScope tracks RPC calls for dc redirection
	DCRedirectionGetSearchAttributesScope
	// DCRedirectionPollForActivityTaskScope tracks RPC calls for dc redirection
	DCRedirectionPollForActivityTaskScope
	// DCRedirectionPollForDecisionTaskScope tracks RPC calls for dc redirection
	DCRedirectionPollForDecisionTaskScope
	// DCRedirectionQueryWorkflowScope tracks RPC calls for dc redirection
	DCRedirectionQueryWorkflowScope
	// DCRedirectionRecordActivityTaskHeartbeatScope tracks RPC calls for dc redirection
	DCRedirectionRecordActivityTaskHeartbeatScope
	// DCRedirectionRecordActivityTaskHeartbeatByIDScope tracks RPC calls for dc redirection
	DCRedirectionRecordActivityTaskHeartbeatByIDScope
	// DCRedirectionRegisterDomainScope tracks RPC calls for dc redirection
	DCRedirectionRegisterDomainScope
	// DCRedirectionRequestCancelWorkflowExecutionScope tracks RPC calls for dc redirection
	DCRedirectionRequestCancelWorkflowExecutionScope
	// DCRedirectionResetStickyTaskListScope tracks RPC calls for dc redirection
	DCRedirectionResetStickyTaskListScope
	// DCRedirectionResetWorkflowExecutionScope tracks RPC calls for dc redirection
	DCRedirectionResetWorkflowExecutionScope
	// DCRedirectionRespondActivityTaskCanceledScope tracks RPC calls for dc redirection
	DCRedirectionRespondActivityTaskCanceledScope
	// DCRedirectionRespondActivityTaskCanceledByIDScope tracks RPC calls for dc redirection
	DCRedirectionRespondActivityTaskCanceledByIDScope
	// DCRedirectionRespondActivityTaskCompletedScope tracks RPC calls for dc redirection
	DCRedirectionRespondActivityTaskCompletedScope
	// DCRedirectionRespondActivityTaskCompletedByIDScope tracks RPC calls for dc redirection
	DCRedirectionRespondActivityTaskCompletedByIDScope
	// DCRedirectionRespondActivityTaskFailedScope tracks RPC calls for dc redirection
	DCRedirectionRespondActivityTaskFailedScope
	// DCRedirectionRespondActivityTaskFailedByIDScope tracks RPC calls for dc redirection
	DCRedirectionRespondActivityTaskFailedByIDScope
	// DCRedirectionRespondDecisionTaskCompletedScope tracks RPC calls for dc redirection
	DCRedirectionRespondDecisionTaskCompletedScope
	// DCRedirectionRespondDecisionTaskFailedScope tracks RPC calls for dc redirection
	DCRedirectionRespondDecisionTaskFailedScope
	// DCRedirectionRespondQueryTaskCompletedScope tracks RPC calls for dc redirection
	DCRedirectionRespondQueryTaskCompletedScope
	// DCRedirectionSignalWithStartWorkflowExecutionScope tracks RPC calls for dc redirection
	DCRedirectionSignalWithStartWorkflowExecutionScope
	// DCRedirectionSignalWorkflowExecutionScope tracks RPC calls for dc redirection
	DCRedirectionSignalWorkflowExecutionScope
	// DCRedirectionStartWorkflowExecutionScope tracks RPC calls for dc redirection
	DCRedirectionStartWorkflowExecutionScope
	// DCRedirectionTerminateWorkflowExecutionScope tracks RPC calls for dc redirection
	DCRedirectionTerminateWorkflowExecutionScope
	// DCRedirectionUpdateDomainScope tracks RPC calls for dc redirection
	DCRedirectionUpdateDomainScope
	// DCRedirectionListTaskListPartitionsScope tracks RPC calls for dc redirection
	DCRedirectionListTaskListPartitionsScope

	// MessagingPublishScope tracks Publish calls made by service to messaging layer
	MessagingClientPublishScope
	// MessagingPublishBatchScope tracks Publish calls made by service to messaging layer
	MessagingClientPublishBatchScope

	// DomainCacheScope tracks domain cache callbacks
	DomainCacheScope
	// HistoryRereplicationByTransferTaskScope tracks history replication calls made by transfer task
	HistoryRereplicationByTransferTaskScope
	// HistoryRereplicationByTimerTaskScope tracks history replication calls made by timer task
	HistoryRereplicationByTimerTaskScope
	// HistoryRereplicationByHistoryReplicationScope tracks history replication calls made by history replication
	HistoryRereplicationByHistoryReplicationScope
	// HistoryRereplicationByHistoryMetadataReplicationScope tracks history replication calls made by history replication
	HistoryRereplicationByHistoryMetadataReplicationScope
	// HistoryRereplicationByActivityReplicationScope tracks history replication calls made by activity replication
	HistoryRereplicationByActivityReplicationScope

	// PersistenceAppendHistoryNodesScope tracks AppendHistoryNodes calls made by service to persistence layer
	PersistenceAppendHistoryNodesScope
	// PersistenceReadHistoryBranchScope tracks ReadHistoryBranch calls made by service to persistence layer
	PersistenceReadHistoryBranchScope
	// PersistenceForkHistoryBranchScope tracks ForkHistoryBranch calls made by service to persistence layer
	PersistenceForkHistoryBranchScope
	// PersistenceDeleteHistoryBranchScope tracks DeleteHistoryBranch calls made by service to persistence layer
	PersistenceDeleteHistoryBranchScope
	// PersistenceCompleteForkBranchScope tracks CompleteForkBranch calls made by service to persistence layer
	PersistenceCompleteForkBranchScope
	// PersistenceGetHistoryTreeScope tracks GetHistoryTree calls made by service to persistence layer
	PersistenceGetHistoryTreeScope
	// PersistenceGetAllHistoryTreeBranchesScope tracks GetHistoryTree calls made by service to persistence layer
	PersistenceGetAllHistoryTreeBranchesScope
	// PersistenceDomainReplicationQueueScope is the metrics scope for domain replication queue
	PersistenceDomainReplicationQueueScope

	// ClusterMetadataArchivalConfigScope tracks ArchivalConfig calls to ClusterMetadata
	ClusterMetadataArchivalConfigScope

	// ElasticsearchRecordWorkflowExecutionStartedScope tracks RecordWorkflowExecutionStarted calls made by service to persistence layer
	ElasticsearchRecordWorkflowExecutionStartedScope
	// ElasticsearchRecordWorkflowExecutionClosedScope tracks RecordWorkflowExecutionClosed calls made by service to persistence layer
	ElasticsearchRecordWorkflowExecutionClosedScope
	// ElasticsearchUpsertWorkflowExecutionScope tracks UpsertWorkflowExecution calls made by service to persistence layer
	ElasticsearchUpsertWorkflowExecutionScope
	// ElasticsearchListOpenWorkflowExecutionsScope tracks ListOpenWorkflowExecutions calls made by service to persistence layer
	ElasticsearchListOpenWorkflowExecutionsScope
	// ElasticsearchListClosedWorkflowExecutionsScope tracks ListClosedWorkflowExecutions calls made by service to persistence layer
	ElasticsearchListClosedWorkflowExecutionsScope
	// ElasticsearchListOpenWorkflowExecutionsByTypeScope tracks ListOpenWorkflowExecutionsByType calls made by service to persistence layer
	ElasticsearchListOpenWorkflowExecutionsByTypeScope
	// ElasticsearchListClosedWorkflowExecutionsByTypeScope tracks ListClosedWorkflowExecutionsByType calls made by service to persistence layer
	ElasticsearchListClosedWorkflowExecutionsByTypeScope
	// ElasticsearchListOpenWorkflowExecutionsByWorkflowIDScope tracks ListOpenWorkflowExecutionsByWorkflowID calls made by service to persistence layer
	ElasticsearchListOpenWorkflowExecutionsByWorkflowIDScope
	// ElasticsearchListClosedWorkflowExecutionsByWorkflowIDScope tracks ListClosedWorkflowExecutionsByWorkflowID calls made by service to persistence layer
	ElasticsearchListClosedWorkflowExecutionsByWorkflowIDScope
	// ElasticsearchListClosedWorkflowExecutionsByStatusScope tracks ListClosedWorkflowExecutionsByStatus calls made by service to persistence layer
	ElasticsearchListClosedWorkflowExecutionsByStatusScope
	// ElasticsearchGetClosedWorkflowExecutionScope tracks GetClosedWorkflowExecution calls made by service to persistence layer
	ElasticsearchGetClosedWorkflowExecutionScope
	// ElasticsearchListWorkflowExecutionsScope tracks ListWorkflowExecutions calls made by service to persistence layer
	ElasticsearchListWorkflowExecutionsScope
	// ElasticsearchScanWorkflowExecutionsScope tracks ScanWorkflowExecutions calls made by service to persistence layer
	ElasticsearchScanWorkflowExecutionsScope
	// ElasticsearchCountWorkflowExecutionsScope tracks CountWorkflowExecutions calls made by service to persistence layer
	ElasticsearchCountWorkflowExecutionsScope
	// ElasticsearchDeleteWorkflowExecutionsScope tracks DeleteWorkflowExecution calls made by service to persistence layer
	ElasticsearchDeleteWorkflowExecutionsScope

	// SequentialTaskProcessingScope is used by sequential task processing logic
	SequentialTaskProcessingScope
	// ParallelTaskProcessingScope is used by parallel task processing logic
	ParallelTaskProcessingScope
	// TaskSchedulerScope is used by task scheduler logic
	TaskSchedulerScope

	// HistoryArchiverScope is used by history archivers
	HistoryArchiverScope
	// VisibilityArchiverScope is used by visibility archivers
	VisibilityArchiverScope

	// The following metrics are only used by internal archiver implemention.
	// TODO: move them to internal repo once cadence plugin model is in place.

	// BlobstoreClientUploadScope tracks Upload calls to blobstore
	BlobstoreClientUploadScope
	// BlobstoreClientDownloadScope tracks Download calls to blobstore
	BlobstoreClientDownloadScope
	// BlobstoreClientGetMetadataScope tracks GetMetadata calls to blobstore
	BlobstoreClientGetMetadataScope
	// BlobstoreClientExistsScope tracks Exists calls to blobstore
	BlobstoreClientExistsScope
	// BlobstoreClientDeleteScope tracks Delete calls to blobstore
	BlobstoreClientDeleteScope
	// BlobstoreClientDirectoryExistsScope tracks DirectoryExists calls to blobstore
	BlobstoreClientDirectoryExistsScope

	// DomainFailoverScope is used in domain failover processor
	DomainFailoverScope

	NumCommonScopes
)

// -- Operation scopes for Admin service --
const (
	// AdminDescribeHistoryHostScope is the metric scope for admin.AdminDescribeHistoryHostScope
	AdminDescribeHistoryHostScope = iota + NumCommonScopes
	// AdminAddSearchAttributeScope is the metric scope for admin.AdminAddSearchAttributeScope
	AdminAddSearchAttributeScope
	// AdminDescribeWorkflowExecutionScope is the metric scope for admin.AdminDescribeWorkflowExecutionScope
	AdminDescribeWorkflowExecutionScope
	// AdminGetWorkflowExecutionRawHistoryScope is the metric scope for admin.GetWorkflowExecutionRawHistoryScope
	AdminGetWorkflowExecutionRawHistoryScope
	// AdminGetWorkflowExecutionRawHistoryV2Scope is the metric scope for admin.GetWorkflowExecutionRawHistoryScope
	AdminGetWorkflowExecutionRawHistoryV2Scope
	// AdminGetReplicationMessagesScope is the metric scope for admin.GetReplicationMessages
	AdminGetReplicationMessagesScope
	// AdminGetDomainReplicationMessagesScope is the metric scope for admin.GetDomainReplicationMessages
	AdminGetDomainReplicationMessagesScope
	// AdminGetDLQReplicationMessagesScope is the metric scope for admin.GetDLQReplicationMessages
	AdminGetDLQReplicationMessagesScope
	// AdminReapplyEventsScope is the metric scope for admin.ReapplyEvents
	AdminReapplyEventsScope
	// AdminRefreshWorkflowTasksScope is the metric scope for admin.RefreshWorkflowTasks
	AdminRefreshWorkflowTasksScope
	// AdminResendReplicationTasksScope is the metric scope for admin.ResendReplicationTasks
	AdminResendReplicationTasksScope
	// AdminRemoveTaskScope is the metric scope for admin.AdminRemoveTaskScope
	AdminRemoveTaskScope
	//AdminCloseShardTaskScope is the metric scope for admin.AdminRemoveTaskScope
	AdminCloseShardTaskScope
	//AdminReadDLQMessagesScope is the metric scope for admin.AdminReadDLQMessagesScope
	AdminReadDLQMessagesScope
	//AdminPurgeDLQMessagesScope is the metric scope for admin.AdminPurgeDLQMessagesScope
	AdminPurgeDLQMessagesScope
	//AdminMergeDLQMessagesScope is the metric scope for admin.AdminMergeDLQMessagesScope
	AdminMergeDLQMessagesScope

	NumAdminScopes
)

// -- Operation scopes for Frontend service --
const (
	// FrontendStartWorkflowExecutionScope is the metric scope for frontend.StartWorkflowExecution
	FrontendStartWorkflowExecutionScope = iota + NumAdminScopes
	// PollForDecisionTaskScope is the metric scope for frontend.PollForDecisionTask
	FrontendPollForDecisionTaskScope
	// FrontendPollForActivityTaskScope is the metric scope for frontend.PollForActivityTask
	FrontendPollForActivityTaskScope
	// FrontendRecordActivityTaskHeartbeatScope is the metric scope for frontend.RecordActivityTaskHeartbeat
	FrontendRecordActivityTaskHeartbeatScope
	// FrontendRecordActivityTaskHeartbeatByIDScope is the metric scope for frontend.RespondDecisionTaskCompleted
	FrontendRecordActivityTaskHeartbeatByIDScope
	// FrontendRespondDecisionTaskCompletedScope is the metric scope for frontend.RespondDecisionTaskCompleted
	FrontendRespondDecisionTaskCompletedScope
	// FrontendRespondDecisionTaskFailedScope is the metric scope for frontend.RespondDecisionTaskFailed
	FrontendRespondDecisionTaskFailedScope
	// FrontendRespondQueryTaskCompletedScope is the metric scope for frontend.RespondQueryTaskCompleted
	FrontendRespondQueryTaskCompletedScope
	// FrontendRespondActivityTaskCompletedScope is the metric scope for frontend.RespondActivityTaskCompleted
	FrontendRespondActivityTaskCompletedScope
	// FrontendRespondActivityTaskFailedScope is the metric scope for frontend.RespondActivityTaskFailed
	FrontendRespondActivityTaskFailedScope
	// FrontendRespondActivityTaskCanceledScope is the metric scope for frontend.RespondActivityTaskCanceled
	FrontendRespondActivityTaskCanceledScope
	// FrontendRespondActivityTaskCompletedScope is the metric scope for frontend.RespondActivityTaskCompletedByID
	FrontendRespondActivityTaskCompletedByIDScope
	// FrontendRespondActivityTaskFailedScope is the metric scope for frontend.RespondActivityTaskFailedByID
	FrontendRespondActivityTaskFailedByIDScope
	// FrontendRespondActivityTaskCanceledScope is the metric scope for frontend.RespondActivityTaskCanceledByID
	FrontendRespondActivityTaskCanceledByIDScope
	// FrontendGetWorkflowExecutionHistoryScope is the metric scope for frontend.GetWorkflowExecutionHistory
	FrontendGetWorkflowExecutionHistoryScope
	// FrontendGetWorkflowExecutionRawHistoryScope is the metric scope for frontend.GetWorkflowExecutionRawHistory
	FrontendGetWorkflowExecutionRawHistoryScope
	// FrontendPollForWorklfowExecutionRawHistoryScope is the metric scope for frontend.GetWorkflowExecutionRawHistory
	FrontendPollForWorklfowExecutionRawHistoryScope
	// FrontendSignalWorkflowExecutionScope is the metric scope for frontend.SignalWorkflowExecution
	FrontendSignalWorkflowExecutionScope
	// FrontendSignalWithStartWorkflowExecutionScope is the metric scope for frontend.SignalWithStartWorkflowExecution
	FrontendSignalWithStartWorkflowExecutionScope
	// FrontendTerminateWorkflowExecutionScope is the metric scope for frontend.TerminateWorkflowExecution
	FrontendTerminateWorkflowExecutionScope
	// FrontendRequestCancelWorkflowExecutionScope is the metric scope for frontend.RequestCancelWorkflowExecution
	FrontendRequestCancelWorkflowExecutionScope
	// FrontendListArchivedWorkflowExecutionsScope is the metric scope for frontend.ListArchivedWorkflowExecutions
	FrontendListArchivedWorkflowExecutionsScope
	// FrontendListOpenWorkflowExecutionsScope is the metric scope for frontend.ListOpenWorkflowExecutions
	FrontendListOpenWorkflowExecutionsScope
	// FrontendListClosedWorkflowExecutionsScope is the metric scope for frontend.ListClosedWorkflowExecutions
	FrontendListClosedWorkflowExecutionsScope
	// FrontendListWorkflowExecutionsScope is the metric scope for frontend.ListWorkflowExecutions
	FrontendListWorkflowExecutionsScope
	// FrontendScanWorkflowExecutionsScope is the metric scope for frontend.ListWorkflowExecutions
	FrontendScanWorkflowExecutionsScope
	// FrontendCountWorkflowExecutionsScope is the metric scope for frontend.CountWorkflowExecutions
	FrontendCountWorkflowExecutionsScope
	// FrontendRegisterDomainScope is the metric scope for frontend.RegisterDomain
	FrontendRegisterDomainScope
	// FrontendDescribeDomainScope is the metric scope for frontend.DescribeDomain
	FrontendDescribeDomainScope
	// FrontendUpdateDomainScope is the metric scope for frontend.DescribeDomain
	FrontendUpdateDomainScope
	// FrontendDeprecateDomainScope is the metric scope for frontend.DeprecateDomain
	FrontendDeprecateDomainScope
	// FrontendQueryWorkflowScope is the metric scope for frontend.QueryWorkflow
	FrontendQueryWorkflowScope
	// FrontendDescribeWorkflowExecutionScope is the metric scope for frontend.DescribeWorkflowExecution
	FrontendDescribeWorkflowExecutionScope
	// FrontendDescribeTaskListScope is the metric scope for frontend.DescribeTaskList
	FrontendDescribeTaskListScope
	// FrontendResetStickyTaskListScope is the metric scope for frontend.ResetStickyTaskList
	FrontendListTaskListPartitionsScope
	// FrontendResetStickyTaskListScope is the metric scope for frontend.ResetStickyTaskList
	FrontendResetStickyTaskListScope
	// FrontendListDomainsScope is the metric scope for frontend.ListDomain
	FrontendListDomainsScope
	// FrontendResetWorkflowExecutionScope is the metric scope for frontend.ResetWorkflowExecution
	FrontendResetWorkflowExecutionScope
	// FrontendGetSearchAttributesScope is the metric scope for frontend.GetSearchAttributes
	FrontendGetSearchAttributesScope

	NumFrontendScopes
)

// -- Operation scopes for History service --
const (
	// HistoryStartWorkflowExecutionScope tracks StartWorkflowExecution API calls received by service
	HistoryStartWorkflowExecutionScope = iota + NumCommonScopes
	// HistoryRecordActivityTaskHeartbeatScope tracks RecordActivityTaskHeartbeat API calls received by service
	HistoryRecordActivityTaskHeartbeatScope
	// HistoryRespondDecisionTaskCompletedScope tracks RespondDecisionTaskCompleted API calls received by service
	HistoryRespondDecisionTaskCompletedScope
	// HistoryRespondDecisionTaskFailedScope tracks RespondDecisionTaskFailed API calls received by service
	HistoryRespondDecisionTaskFailedScope
	// HistoryRespondActivityTaskCompletedScope tracks RespondActivityTaskCompleted API calls received by service
	HistoryRespondActivityTaskCompletedScope
	// HistoryRespondActivityTaskFailedScope tracks RespondActivityTaskFailed API calls received by service
	HistoryRespondActivityTaskFailedScope
	// HistoryRespondActivityTaskCanceledScope tracks RespondActivityTaskCanceled API calls received by service
	HistoryRespondActivityTaskCanceledScope
	// HistoryGetMutableStateScope tracks GetMutableStateScope API calls received by service
	HistoryGetMutableStateScope
	// HistoryPollMutableStateScope tracks PollMutableStateScope API calls received by service
	HistoryPollMutableStateScope
	// HistoryResetStickyTaskListScope tracks ResetStickyTaskListScope API calls received by service
	HistoryResetStickyTaskListScope
	// HistoryDescribeWorkflowExecutionScope tracks DescribeWorkflowExecution API calls received by service
	HistoryDescribeWorkflowExecutionScope
	// HistoryRecordDecisionTaskStartedScope tracks RecordDecisionTaskStarted API calls received by service
	HistoryRecordDecisionTaskStartedScope
	// HistoryRecordActivityTaskStartedScope tracks RecordActivityTaskStarted API calls received by service
	HistoryRecordActivityTaskStartedScope
	// HistorySignalWorkflowExecutionScope tracks SignalWorkflowExecution API calls received by service
	HistorySignalWorkflowExecutionScope
	// HistorySignalWithStartWorkflowExecutionScope tracks SignalWithStartWorkflowExecution API calls received by service
	HistorySignalWithStartWorkflowExecutionScope
	// HistoryRemoveSignalMutableStateScope tracks RemoveSignalMutableState API calls received by service
	HistoryRemoveSignalMutableStateScope
	// HistoryTerminateWorkflowExecutionScope tracks TerminateWorkflowExecution API calls received by service
	HistoryTerminateWorkflowExecutionScope
	// HistoryScheduleDecisionTaskScope tracks ScheduleDecisionTask API calls received by service
	HistoryScheduleDecisionTaskScope
	// HistoryRecordChildExecutionCompletedScope tracks CompleteChildExecution API calls received by service
	HistoryRecordChildExecutionCompletedScope
	// HistoryRequestCancelWorkflowExecutionScope tracks RequestCancelWorkflowExecution API calls received by service
	HistoryRequestCancelWorkflowExecutionScope
	// HistoryReplicateEventsScope tracks ReplicateEvents API calls received by service
	HistoryReplicateEventsScope
	// HistoryReplicateRawEventsScope tracks ReplicateEvents API calls received by service
	HistoryReplicateRawEventsScope
	// HistoryReplicateEventsV2Scope tracks ReplicateEvents API calls received by service
	HistoryReplicateEventsV2Scope
	// HistorySyncShardStatusScope tracks HistorySyncShardStatus API calls received by service
	HistorySyncShardStatusScope
	// HistorySyncActivityScope tracks HistoryActivity API calls received by service
	HistorySyncActivityScope
	// HistoryDescribeMutableStateScope tracks HistoryActivity API calls received by service
	HistoryDescribeMutableStateScope
	// GetReplicationMessages tracks GetReplicationMessages API calls received by service
	HistoryGetReplicationMessagesScope
	// HistoryGetDLQReplicationMessagesScope tracks GetReplicationMessages API calls received by service
	HistoryGetDLQReplicationMessagesScope
	// HistoryReadDLQMessagesScope tracks ReadDLQMessages API calls received by service
	HistoryReadDLQMessagesScope
	// HistoryPurgeDLQMessagesScope tracks PurgeDLQMessages API calls received by service
	HistoryPurgeDLQMessagesScope
	// HistoryMergeDLQMessagesScope tracks MergeDLQMessages API calls received by service
	HistoryMergeDLQMessagesScope
	// HistoryShardControllerScope is the scope used by shard controller
	HistoryShardControllerScope
	// HistoryReapplyEventsScope is the scope used by event reapplication
	HistoryReapplyEventsScope
	// HistoryRefreshWorkflowTasksScope is the scope used by refresh workflow tasks API
	HistoryRefreshWorkflowTasksScope
	// HistoryNotifyFailoverMarkersScope is the scope used by refresh workflow tasks API
	HistoryNotifyFailoverMarkersScope
	// TaskPriorityAssignerScope is the scope used by all metric emitted by task priority assigner
	TaskPriorityAssignerScope
	// TransferQueueProcessorScope is the scope used by all metric emitted by transfer queue processor
	TransferQueueProcessorScope
	// TransferActiveQueueProcessorScope is the scope used by all metric emitted by transfer queue processor
	TransferActiveQueueProcessorScope
	// TransferStandbyQueueProcessorScope is the scope used by all metric emitted by transfer queue processor
	TransferStandbyQueueProcessorScope
	// TransferActiveTaskActivityScope is the scope used for activity task processing by transfer queue processor
	TransferActiveTaskActivityScope
	// TransferActiveTaskDecisionScope is the scope used for decision task processing by transfer queue processor
	TransferActiveTaskDecisionScope
	// TransferActiveTaskCloseExecutionScope is the scope used for close execution task processing by transfer queue processor
	TransferActiveTaskCloseExecutionScope
	// TransferActiveTaskCancelExecutionScope is the scope used for cancel execution task processing by transfer queue processor
	TransferActiveTaskCancelExecutionScope
	// TransferActiveTaskSignalExecutionScope is the scope used for signal execution task processing by transfer queue processor
	TransferActiveTaskSignalExecutionScope
	// TransferActiveTaskStartChildExecutionScope is the scope used for start child execution task processing by transfer queue processor
	TransferActiveTaskStartChildExecutionScope
	// TransferActiveTaskRecordWorkflowStartedScope is the scope used for record workflow started task processing by transfer queue processor
	TransferActiveTaskRecordWorkflowStartedScope
	// TransferActiveTaskResetWorkflowScope is the scope used for record workflow started task processing by transfer queue processor
	TransferActiveTaskResetWorkflowScope
	// TransferActiveTaskUpsertWorkflowSearchAttributesScope is the scope used for upsert search attributes processing by transfer queue processor
	TransferActiveTaskUpsertWorkflowSearchAttributesScope
	// TransferStandbyTaskResetWorkflowScope is the scope used for record workflow started task processing by transfer queue processor
	TransferStandbyTaskResetWorkflowScope
	// TransferStandbyTaskActivityScope is the scope used for activity task processing by transfer queue processor
	TransferStandbyTaskActivityScope
	// TransferStandbyTaskDecisionScope is the scope used for decision task processing by transfer queue processor
	TransferStandbyTaskDecisionScope
	// TransferStandbyTaskCloseExecutionScope is the scope used for close execution task processing by transfer queue processor
	TransferStandbyTaskCloseExecutionScope
	// TransferStandbyTaskCancelExecutionScope is the scope used for cancel execution task processing by transfer queue processor
	TransferStandbyTaskCancelExecutionScope
	// TransferStandbyTaskSignalExecutionScope is the scope used for signal execution task processing by transfer queue processor
	TransferStandbyTaskSignalExecutionScope
	// TransferStandbyTaskStartChildExecutionScope is the scope used for start child execution task processing by transfer queue processor
	TransferStandbyTaskStartChildExecutionScope
	// TransferStandbyTaskRecordWorkflowStartedScope is the scope used for record workflow started task processing by transfer queue processor
	TransferStandbyTaskRecordWorkflowStartedScope
	// TransferStandbyTaskUpsertWorkflowSearchAttributesScope is the scope used for upsert search attributes processing by transfer queue processor
	TransferStandbyTaskUpsertWorkflowSearchAttributesScope
	// TimerQueueProcessorScope is the scope used by all metric emitted by timer queue processor
	TimerQueueProcessorScope
	// TimerActiveQueueProcessorScope is the scope used by all metric emitted by timer queue processor
	TimerActiveQueueProcessorScope
	// TimerQueueProcessorScope is the scope used by all metric emitted by timer queue processor
	TimerStandbyQueueProcessorScope
	// TimerActiveTaskActivityTimeoutScope is the scope used by metric emitted by timer queue processor for processing activity timeouts
	TimerActiveTaskActivityTimeoutScope
	// TimerActiveTaskDecisionTimeoutScope is the scope used by metric emitted by timer queue processor for processing decision timeouts
	TimerActiveTaskDecisionTimeoutScope
	// TimerActiveTaskUserTimerScope is the scope used by metric emitted by timer queue processor for processing user timers
	TimerActiveTaskUserTimerScope
	// TimerActiveTaskWorkflowTimeoutScope is the scope used by metric emitted by timer queue processor for processing workflow timeouts.
	TimerActiveTaskWorkflowTimeoutScope
	// TimerActiveTaskActivityRetryTimerScope is the scope used by metric emitted by timer queue processor for processing retry task.
	TimerActiveTaskActivityRetryTimerScope
	// TimerActiveTaskWorkflowBackoffTimerScope is the scope used by metric emitted by timer queue processor for processing retry task.
	TimerActiveTaskWorkflowBackoffTimerScope
	// TimerActiveTaskDeleteHistoryEventScope is the scope used by metric emitted by timer queue processor for processing history event cleanup
	TimerActiveTaskDeleteHistoryEventScope
	// TimerStandbyTaskActivityTimeoutScope is the scope used by metric emitted by timer queue processor for processing activity timeouts
	TimerStandbyTaskActivityTimeoutScope
	// TimerStandbyTaskDecisionTimeoutScope is the scope used by metric emitted by timer queue processor for processing decision timeouts
	TimerStandbyTaskDecisionTimeoutScope
	// TimerStandbyTaskUserTimerScope is the scope used by metric emitted by timer queue processor for processing user timers
	TimerStandbyTaskUserTimerScope
	// TimerStandbyTaskWorkflowTimeoutScope is the scope used by metric emitted by timer queue processor for processing workflow timeouts.
	TimerStandbyTaskWorkflowTimeoutScope
	// TimerStandbyTaskActivityRetryTimerScope is the scope used by metric emitted by timer queue processor for processing retry task.
	TimerStandbyTaskActivityRetryTimerScope
	// TimerStandbyTaskDeleteHistoryEventScope is the scope used by metric emitted by timer queue processor for processing history event cleanup
	TimerStandbyTaskDeleteHistoryEventScope
	// TimerStandbyTaskWorkflowBackoffTimerScope is the scope used by metric emitted by timer queue processor for processing retry task.
	TimerStandbyTaskWorkflowBackoffTimerScope
	// HistoryEventNotificationScope is the scope used by shard history event nitification
	HistoryEventNotificationScope
	// ReplicatorQueueProcessorScope is the scope used by all metric emitted by replicator queue processor
	ReplicatorQueueProcessorScope
	// ReplicatorTaskHistoryScope is the scope used for history task processing by replicator queue processor
	ReplicatorTaskHistoryScope
	// ReplicatorTaskSyncActivityScope is the scope used for sync activity by replicator queue processor
	ReplicatorTaskSyncActivityScope
	// ReplicateHistoryEventsScope is the scope used by historyReplicator API for applying events
	ReplicateHistoryEventsScope
	// ShardInfoScope is the scope used when updating shard info
	ShardInfoScope
	// WorkflowContextScope is the scope used by WorkflowContext component
	WorkflowContextScope
	// HistoryCacheGetAndCreateScope is the scope used by history cache
	HistoryCacheGetAndCreateScope
	// HistoryCacheGetOrCreateScope is the scope used by history cache
	HistoryCacheGetOrCreateScope
	// HistoryCacheGetOrCreateCurrentScope is the scope used by history cache
	HistoryCacheGetOrCreateCurrentScope
	// HistoryCacheGetCurrentExecutionScope is the scope used by history cache for getting current execution
	HistoryCacheGetCurrentExecutionScope
	// EventsCacheGetEventScope is the scope used by events cache
	EventsCacheGetEventScope
	// EventsCachePutEventScope is the scope used by events cache
	EventsCachePutEventScope
	// EventsCacheGetFromStoreScope is the scope used by events cache
	EventsCacheGetFromStoreScope
	// ExecutionSizeStatsScope is the scope used for emiting workflow execution size related stats
	ExecutionSizeStatsScope
	// ExecutionCountStatsScope is the scope used for emiting workflow execution count related stats
	ExecutionCountStatsScope
	// SessionSizeStatsScope is the scope used for emiting session update size related stats
	SessionSizeStatsScope
	// SessionCountStatsScope is the scope used for emiting session update count related stats
	SessionCountStatsScope
	// HistoryResetWorkflowExecutionScope tracks ResetWorkflowExecution API calls received by service
	HistoryResetWorkflowExecutionScope
	// HistoryQueryWorkflowScope tracks QueryWorkflow API calls received by service
	HistoryQueryWorkflowScope
	// HistoryProcessDeleteHistoryEventScope tracks ProcessDeleteHistoryEvent processing calls
	HistoryProcessDeleteHistoryEventScope
	// WorkflowCompletionStatsScope tracks workflow completion updates
	WorkflowCompletionStatsScope
	// ArchiverClientScope is scope used by all metrics emitted by archiver.Client
	ArchiverClientScope
	// ReplicationTaskFetcherScope is scope used by all metrics emitted by ReplicationTaskFetcher
	ReplicationTaskFetcherScope
	// ReplicationTaskCleanupScope is scope used by all metrics emitted by ReplicationTaskProcessor cleanup
	ReplicationTaskCleanupScope
	// ReplicationDLQStatsScope is scope used by all metrics emitted related to replication DLQ
	ReplicationDLQStatsScope
	// HistoryFailoverMarkerScope is scope used by all metrics emitted related to failover marker
	HistoryFailoverMarkerScope

	NumHistoryScopes
)

// -- Operation scopes for Matching service --
const (
	// PollForDecisionTaskScope tracks PollForDecisionTask API calls received by service
	MatchingPollForDecisionTaskScope = iota + NumCommonScopes
	// PollForActivityTaskScope tracks PollForActivityTask API calls received by service
	MatchingPollForActivityTaskScope
	// MatchingAddActivityTaskScope tracks AddActivityTask API calls received by service
	MatchingAddActivityTaskScope
	// MatchingAddDecisionTaskScope tracks AddDecisionTask API calls received by service
	MatchingAddDecisionTaskScope
	// MatchingTaskListMgrScope is the metrics scope for matching.TaskListManager component
	MatchingTaskListMgrScope
	// MatchingQueryWorkflowScope tracks AddDecisionTask API calls received by service
	MatchingQueryWorkflowScope
	// MatchingRespondQueryTaskCompletedScope tracks AddDecisionTask API calls received by service
	MatchingRespondQueryTaskCompletedScope
	// MatchingCancelOutstandingPollScope tracks CancelOutstandingPoll API calls received by service
	MatchingCancelOutstandingPollScope
	// MatchingDescribeTaskListScope tracks DescribeTaskList API calls received by service
	MatchingDescribeTaskListScope
	// MatchingListTaskListPartitionsScope tracks ListTaskListPartitions API calls received by service
	MatchingListTaskListPartitionsScope

	NumMatchingScopes
)

// -- Operation scopes for Worker service --
const (
	// ReplicationScope is the scope used by all metric emitted by replicator
	ReplicatorScope = iota + NumCommonScopes
	// DomainReplicationTaskScope is the scope used by domain task replication processing
	DomainReplicationTaskScope
	// HistoryReplicationTaskScope is the scope used by history task replication processing
	HistoryReplicationTaskScope
	// HistoryMetadataReplicationTaskScope is the scope used by history metadata task replication processing
	HistoryMetadataReplicationTaskScope
	// HistoryReplicationV2TaskScope is the scope used by history task replication processing
	HistoryReplicationV2TaskScope
	// SyncShardTaskScope is the scope used by sync shrad information processing
	SyncShardTaskScope
	// SyncActivityTaskScope is the scope used by sync activity information processing
	SyncActivityTaskScope
	// ESProcessorScope is scope used by all metric emitted by esProcessor
	ESProcessorScope
	// IndexProcessorScope is scope used by all metric emitted by index processor
	IndexProcessorScope
	// ArchiverDeleteHistoryActivityScope is scope used by all metrics emitted by archiver.DeleteHistoryActivity
	ArchiverDeleteHistoryActivityScope
	// ArchiverUploadHistoryActivityScope is scope used by all metrics emitted by archiver.UploadHistoryActivity
	ArchiverUploadHistoryActivityScope
	// ArchiverArchiveVisibilityActivityScope is scope used by all metrics emitted by archiver.ArchiveVisibilityActivity
	ArchiverArchiveVisibilityActivityScope
	// ArchiverScope is scope used by all metrics emitted by archiver.Archiver
	ArchiverScope
	// ArchiverPumpScope is scope used by all metrics emitted by archiver.Pump
	ArchiverPumpScope
	// ArchiverArchivalWorkflowScope is scope used by all metrics emitted by archiver.ArchivalWorkflow
	ArchiverArchivalWorkflowScope
	// TaskListScavengerScope is scope used by all metrics emitted by worker.tasklist.Scavenger module
	TaskListScavengerScope
	// ExecutionsScannerScope is scope used by all metrics emitted by worker.executions.Scanner module
	ExecutionsScannerScope
	// ExecutionsFixerScope is the scope used by all metrics emitted by worker.executions.Fixer module
	ExecutionsFixerScope
	// BatcherScope is scope used by all metrics emitted by worker.Batcher module
	BatcherScope
	// HistoryScavengerScope is scope used by all metrics emitted by worker.history.Scavenger module
	HistoryScavengerScope
	// ParentClosePolicyProcessorScope is scope used by all metrics emitted by worker.ParentClosePolicyProcessor
	ParentClosePolicyProcessorScope

	NumWorkerScopes
)

// ScopeDefs record the scopes for all services
var ScopeDefs = map[ServiceIdx]map[int]scopeDefinition{
	// common scope Names
	Common: {
		PersistenceCreateShardScope:                              {operation: "CreateShard"},
		PersistenceGetShardScope:                                 {operation: "GetShard"},
		PersistenceUpdateShardScope:                              {operation: "UpdateShard"},
		PersistenceCreateWorkflowExecutionScope:                  {operation: "CreateWorkflowExecution"},
		PersistenceGetWorkflowExecutionScope:                     {operation: "GetWorkflowExecution"},
		PersistenceUpdateWorkflowExecutionScope:                  {operation: "UpdateWorkflowExecution"},
		PersistenceConflictResolveWorkflowExecutionScope:         {operation: "ConflictResolveWorkflowExecution"},
		PersistenceResetWorkflowExecutionScope:                   {operation: "ResetWorkflowExecution"},
		PersistenceDeleteWorkflowExecutionScope:                  {operation: "DeleteWorkflowExecution"},
		PersistenceDeleteCurrentWorkflowExecutionScope:           {operation: "DeleteCurrentWorkflowExecution"},
		PersistenceGetCurrentExecutionScope:                      {operation: "GetCurrentExecution"},
		PersistenceListConcreteExecutionsScope:                   {operation: "ListConcreteExecutions"},
		PersistenceGetTransferTasksScope:                         {operation: "GetTransferTasks"},
		PersistenceCompleteTransferTaskScope:                     {operation: "CompleteTransferTask"},
		PersistenceRangeCompleteTransferTaskScope:                {operation: "RangeCompleteTransferTask"},
		PersistenceGetReplicationTasksScope:                      {operation: "GetReplicationTasks"},
		PersistenceCompleteReplicationTaskScope:                  {operation: "CompleteReplicationTask"},
		PersistenceRangeCompleteReplicationTaskScope:             {operation: "RangeCompleteReplicationTask"},
		PersistencePutReplicationTaskToDLQScope:                  {operation: "PutReplicationTaskToDLQ"},
		PersistenceGetReplicationTasksFromDLQScope:               {operation: "GetReplicationTasksFromDLQ"},
		PersistenceDeleteReplicationTaskFromDLQScope:             {operation: "DeleteReplicationTaskFromDLQ"},
		PersistenceRangeDeleteReplicationTaskFromDLQScope:        {operation: "RangeDeleteReplicationTaskFromDLQ"},
		PersistenceCreateFailoverMarkerTasksScope:                {operation: "CreateFailoverMarkerTasks"},
		PersistenceGetTimerIndexTasksScope:                       {operation: "GetTimerIndexTasks"},
		PersistenceCompleteTimerTaskScope:                        {operation: "CompleteTimerTask"},
		PersistenceRangeCompleteTimerTaskScope:                   {operation: "RangeCompleteTimerTask"},
		PersistenceCreateTaskScope:                               {operation: "CreateTask"},
		PersistenceGetTasksScope:                                 {operation: "GetTasks"},
		PersistenceCompleteTaskScope:                             {operation: "CompleteTask"},
		PersistenceCompleteTasksLessThanScope:                    {operation: "CompleteTasksLessThan"},
		PersistenceLeaseTaskListScope:                            {operation: "LeaseTaskList"},
		PersistenceUpdateTaskListScope:                           {operation: "UpdateTaskList"},
		PersistenceListTaskListScope:                             {operation: "ListTaskList"},
		PersistenceDeleteTaskListScope:                           {operation: "DeleteTaskList"},
		PersistenceAppendHistoryEventsScope:                      {operation: "AppendHistoryEvents"},
		PersistenceGetWorkflowExecutionHistoryScope:              {operation: "GetWorkflowExecutionHistory"},
		PersistenceDeleteWorkflowExecutionHistoryScope:           {operation: "DeleteWorkflowExecutionHistory"},
		PersistenceCreateDomainScope:                             {operation: "CreateDomain"},
		PersistenceGetDomainScope:                                {operation: "GetDomain"},
		PersistenceUpdateDomainScope:                             {operation: "UpdateDomain"},
		PersistenceDeleteDomainScope:                             {operation: "DeleteDomain"},
		PersistenceDeleteDomainByNameScope:                       {operation: "DeleteDomainByName"},
		PersistenceListDomainScope:                               {operation: "ListDomain"},
		PersistenceGetMetadataScope:                              {operation: "GetMetadata"},
		PersistenceRecordWorkflowExecutionStartedScope:           {operation: "RecordWorkflowExecutionStarted"},
		PersistenceRecordWorkflowExecutionClosedScope:            {operation: "RecordWorkflowExecutionClosed"},
		PersistenceUpsertWorkflowExecutionScope:                  {operation: "UpsertWorkflowExecution"},
		PersistenceListOpenWorkflowExecutionsScope:               {operation: "ListOpenWorkflowExecutions"},
		PersistenceListClosedWorkflowExecutionsScope:             {operation: "ListClosedWorkflowExecutions"},
		PersistenceListOpenWorkflowExecutionsByTypeScope:         {operation: "ListOpenWorkflowExecutionsByType"},
		PersistenceListClosedWorkflowExecutionsByTypeScope:       {operation: "ListClosedWorkflowExecutionsByType"},
		PersistenceListOpenWorkflowExecutionsByWorkflowIDScope:   {operation: "ListOpenWorkflowExecutionsByWorkflowID"},
		PersistenceListClosedWorkflowExecutionsByWorkflowIDScope: {operation: "ListClosedWorkflowExecutionsByWorkflowID"},
		PersistenceListClosedWorkflowExecutionsByStatusScope:     {operation: "ListClosedWorkflowExecutionsByStatus"},
		PersistenceGetClosedWorkflowExecutionScope:               {operation: "GetClosedWorkflowExecution"},
		PersistenceVisibilityDeleteWorkflowExecutionScope:        {operation: "VisibilityDeleteWorkflowExecution"},
		PersistenceListWorkflowExecutionsScope:                   {operation: "ListWorkflowExecutions"},
		PersistenceScanWorkflowExecutionsScope:                   {operation: "ScanWorkflowExecutions"},
		PersistenceCountWorkflowExecutionsScope:                  {operation: "CountWorkflowExecutions"},
		PersistenceAppendHistoryNodesScope:                       {operation: "AppendHistoryNodes"},
		PersistenceReadHistoryBranchScope:                        {operation: "ReadHistoryBranch"},
		PersistenceForkHistoryBranchScope:                        {operation: "ForkHistoryBranch"},
		PersistenceDeleteHistoryBranchScope:                      {operation: "DeleteHistoryBranch"},
		PersistenceCompleteForkBranchScope:                       {operation: "CompleteForkBranch"},
		PersistenceGetHistoryTreeScope:                           {operation: "GetHistoryTree"},
		PersistenceGetAllHistoryTreeBranchesScope:                {operation: "GetAllHistoryTreeBranches"},
		PersistenceEnqueueMessageScope:                           {operation: "EnqueueMessage"},
		PersistenceEnqueueMessageToDLQScope:                      {operation: "EnqueueMessageToDLQ"},
		PersistenceReadQueueMessagesScope:                        {operation: "ReadQueueMessages"},
		PersistenceReadQueueMessagesFromDLQScope:                 {operation: "ReadQueueMessagesFromDLQ"},
		PersistenceDeleteQueueMessagesScope:                      {operation: "DeleteQueueMessages"},
		PersistenceDeleteQueueMessageFromDLQScope:                {operation: "DeleteQueueMessageFromDLQ"},
		PersistenceRangeDeleteMessagesFromDLQScope:               {operation: "RangeDeleteMessagesFromDLQ"},
		PersistenceUpdateAckLevelScope:                           {operation: "UpdateAckLevel"},
		PersistenceGetAckLevelScope:                              {operation: "GetAckLevel"},
		PersistenceUpdateDLQAckLevelScope:                        {operation: "UpdateDLQAckLevel"},
		PersistenceGetDLQAckLevelScope:                           {operation: "GetDLQAckLevel"},
		PersistenceDomainReplicationQueueScope:                   {operation: "DomainReplicationQueue"},

		ClusterMetadataArchivalConfigScope: {operation: "ArchivalConfig"},

		HistoryClientStartWorkflowExecutionScope:              {operation: "HistoryClientStartWorkflowExecution", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientRecordActivityTaskHeartbeatScope:         {operation: "HistoryClientRecordActivityTaskHeartbeat", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientRespondDecisionTaskCompletedScope:        {operation: "HistoryClientRespondDecisionTaskCompleted", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientRespondDecisionTaskFailedScope:           {operation: "HistoryClientRespondDecisionTaskFailed", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientRespondActivityTaskCompletedScope:        {operation: "HistoryClientRespondActivityTaskCompleted", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientRespondActivityTaskFailedScope:           {operation: "HistoryClientRespondActivityTaskFailed", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientRespondActivityTaskCanceledScope:         {operation: "HistoryClientRespondActivityTaskCanceled", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientGetMutableStateScope:                     {operation: "HistoryClientGetMutableState", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientPollMutableStateScope:                    {operation: "HistoryClientPollMutableState", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientResetStickyTaskListScope:                 {operation: "HistoryClientResetStickyTaskListScope", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientDescribeWorkflowExecutionScope:           {operation: "HistoryClientDescribeWorkflowExecution", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientRecordDecisionTaskStartedScope:           {operation: "HistoryClientRecordDecisionTaskStarted", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientRecordActivityTaskStartedScope:           {operation: "HistoryClientRecordActivityTaskStarted", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientRequestCancelWorkflowExecutionScope:      {operation: "HistoryClientRequestCancelWorkflowExecution", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientSignalWorkflowExecutionScope:             {operation: "HistoryClientSignalWorkflowExecution", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientSignalWithStartWorkflowExecutionScope:    {operation: "HistoryClientSignalWithStartWorkflowExecution", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientRemoveSignalMutableStateScope:            {operation: "HistoryClientRemoveSignalMutableStateScope", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientTerminateWorkflowExecutionScope:          {operation: "HistoryClientTerminateWorkflowExecution", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientResetWorkflowExecutionScope:              {operation: "HistoryClientResetWorkflowExecution", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientScheduleDecisionTaskScope:                {operation: "HistoryClientScheduleDecisionTask", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientRecordChildExecutionCompletedScope:       {operation: "HistoryClientRecordChildExecutionCompleted", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientReplicateEventsScope:                     {operation: "HistoryClientReplicateEvents", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientReplicateRawEventsScope:                  {operation: "HistoryClientReplicateRawEvents", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientReplicateEventsV2Scope:                   {operation: "HistoryClientReplicateEventsV2", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientSyncShardStatusScope:                     {operation: "HistoryClientSyncShardStatusScope", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientSyncActivityScope:                        {operation: "HistoryClientSyncActivityScope", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientGetReplicationTasksScope:                 {operation: "HistoryClientGetReplicationTasksScope", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientGetDLQReplicationTasksScope:              {operation: "HistoryClientGetDLQReplicationTasksScope", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientQueryWorkflowScope:                       {operation: "HistoryClientQueryWorkflowScope", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientReapplyEventsScope:                       {operation: "HistoryClientReapplyEventsScope", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientReadDLQMessagesScope:                     {operation: "HistoryClientReadDLQMessagesScope", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientPurgeDLQMessagesScope:                    {operation: "HistoryClientPurgeDLQMessagesScope", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientMergeDLQMessagesScope:                    {operation: "HistoryClientMergeDLQMessagesScope", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientRefreshWorkflowTasksScope:                {operation: "HistoryClientRefreshWorkflowTasksScope", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		HistoryClientNotifyFailoverMarkersScope:               {operation: "HistoryClientNotifyFailoverMarkersScope", tags: map[string]string{CadenceRoleTagName: HistoryRoleTagValue}},
		MatchingClientPollForDecisionTaskScope:                {operation: "MatchingClientPollForDecisionTask", tags: map[string]string{CadenceRoleTagName: MatchingRoleTagValue}},
		MatchingClientPollForActivityTaskScope:                {operation: "MatchingClientPollForActivityTask", tags: map[string]string{CadenceRoleTagName: MatchingRoleTagValue}},
		MatchingClientAddActivityTaskScope:                    {operation: "MatchingClientAddActivityTask", tags: map[string]string{CadenceRoleTagName: MatchingRoleTagValue}},
		MatchingClientAddDecisionTaskScope:                    {operation: "MatchingClientAddDecisionTask", tags: map[string]string{CadenceRoleTagName: MatchingRoleTagValue}},
		MatchingClientQueryWorkflowScope:                      {operation: "MatchingClientQueryWorkflow", tags: map[string]string{CadenceRoleTagName: MatchingRoleTagValue}},
		MatchingClientRespondQueryTaskCompletedScope:          {operation: "MatchingClientRespondQueryTaskCompleted", tags: map[string]string{CadenceRoleTagName: MatchingRoleTagValue}},
		MatchingClientCancelOutstandingPollScope:              {operation: "MatchingClientCancelOutstandingPoll", tags: map[string]string{CadenceRoleTagName: MatchingRoleTagValue}},
		MatchingClientDescribeTaskListScope:                   {operation: "MatchingClientDescribeTaskList", tags: map[string]string{CadenceRoleTagName: MatchingRoleTagValue}},
		MatchingClientListTaskListPartitionsScope:             {operation: "MatchingClientListTaskListPartitions", tags: map[string]string{CadenceRoleTagName: MatchingRoleTagValue}},
		FrontendClientDeprecateDomainScope:                    {operation: "FrontendClientDeprecateDomain", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientDescribeDomainScope:                     {operation: "FrontendClientDescribeDomain", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientDescribeTaskListScope:                   {operation: "FrontendClientDescribeTaskList", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientDescribeWorkflowExecutionScope:          {operation: "FrontendClientDescribeWorkflowExecution", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientGetWorkflowExecutionHistoryScope:        {operation: "FrontendClientGetWorkflowExecutionHistory", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientGetWorkflowExecutionRawHistoryScope:     {operation: "FrontendClientGetWorkflowExecutionRawHistory", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientPollForWorkflowExecutionRawHistoryScope: {operation: "FrontendClientPollForWorkflowExecutionRawHistory", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientListArchivedWorkflowExecutionsScope:     {operation: "FrontendClientListArchivedWorkflowExecutions", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientListClosedWorkflowExecutionsScope:       {operation: "FrontendClientListClosedWorkflowExecutions", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientListDomainsScope:                        {operation: "FrontendClientListDomains", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientListOpenWorkflowExecutionsScope:         {operation: "FrontendClientListOpenWorkflowExecutions", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientPollForActivityTaskScope:                {operation: "FrontendClientPollForActivityTask", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientPollForDecisionTaskScope:                {operation: "FrontendClientPollForDecisionTask", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientQueryWorkflowScope:                      {operation: "FrontendClientQueryWorkflow", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientRecordActivityTaskHeartbeatScope:        {operation: "FrontendClientRecordActivityTaskHeartbeat", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientRecordActivityTaskHeartbeatByIDScope:    {operation: "FrontendClientRecordActivityTaskHeartbeatByID", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientRegisterDomainScope:                     {operation: "FrontendClientRegisterDomain", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientRequestCancelWorkflowExecutionScope:     {operation: "FrontendClientRequestCancelWorkflowExecution", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientResetStickyTaskListScope:                {operation: "FrontendClientResetStickyTaskList", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientResetWorkflowExecutionScope:             {operation: "FrontendClientResetWorkflowExecution", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientRespondActivityTaskCanceledScope:        {operation: "FrontendClientRespondActivityTaskCanceled", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientRespondActivityTaskCanceledByIDScope:    {operation: "FrontendClientRespondActivityTaskCanceledByID", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientRespondActivityTaskCompletedScope:       {operation: "FrontendClientRespondActivityTaskCompleted", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientRespondActivityTaskCompletedByIDScope:   {operation: "FrontendClientRespondActivityTaskCompletedByID", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientRespondActivityTaskFailedScope:          {operation: "FrontendClientRespondActivityTaskFailed", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientRespondActivityTaskFailedByIDScope:      {operation: "FrontendClientRespondActivityTaskFailedByID", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientRespondDecisionTaskCompletedScope:       {operation: "FrontendClientRespondDecisionTaskCompleted", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientRespondDecisionTaskFailedScope:          {operation: "FrontendClientRespondDecisionTaskFailed", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientRespondQueryTaskCompletedScope:          {operation: "FrontendClientRespondQueryTaskCompleted", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientSignalWithStartWorkflowExecutionScope:   {operation: "FrontendClientSignalWithStartWorkflowExecution", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientSignalWorkflowExecutionScope:            {operation: "FrontendClientSignalWorkflowExecution", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientStartWorkflowExecutionScope:             {operation: "FrontendClientStartWorkflowExecution", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientTerminateWorkflowExecutionScope:         {operation: "FrontendClientTerminateWorkflowExecution", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientUpdateDomainScope:                       {operation: "FrontendClientUpdateDomain", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientListWorkflowExecutionsScope:             {operation: "FrontendClientListWorkflowExecutions", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientScanWorkflowExecutionsScope:             {operation: "FrontendClientScanWorkflowExecutions", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientCountWorkflowExecutionsScope:            {operation: "FrontendClientCountWorkflowExecutions", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientGetSearchAttributesScope:                {operation: "FrontendClientGetSearchAttributes", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientGetReplicationTasksScope:                {operation: "FrontendClientGetReplicationTasksScope", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientGetDomainReplicationTasksScope:          {operation: "FrontendClientGetDomainReplicationTasksScope", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientGetDLQReplicationTasksScope:             {operation: "FrontendClientGetDLQReplicationTasksScope", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientReapplyEventsScope:                      {operation: "FrontendClientReapplyEventsScope", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientGetClusterInfoScope:                     {operation: "FrontendClientGetClusterInfoScope", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		FrontendClientListTaskListPartitionsScope:             {operation: "FrontendClientListTaskListPartitions", tags: map[string]string{CadenceRoleTagName: FrontendRoleTagValue}},
		AdminClientAddSearchAttributeScope:                    {operation: "AdminClientAddSearchAttribute", tags: map[string]string{CadenceRoleTagName: AdminRoleTagValue}},
		AdminClientDescribeHistoryHostScope:                   {operation: "AdminClientDescribeHistoryHost", tags: map[string]string{CadenceRoleTagName: AdminRoleTagValue}},
		AdminClientDescribeWorkflowExecutionScope:             {operation: "AdminClientDescribeWorkflowExecution", tags: map[string]string{CadenceRoleTagName: AdminRoleTagValue}},
		AdminClientGetWorkflowExecutionRawHistoryScope:        {operation: "AdminClientGetWorkflowExecutionRawHistory", tags: map[string]string{CadenceRoleTagName: AdminRoleTagValue}},
		AdminClientGetWorkflowExecutionRawHistoryV2Scope:      {operation: "AdminClientGetWorkflowExecutionRawHistoryV2", tags: map[string]string{CadenceRoleTagName: AdminRoleTagValue}},
		AdminClientDescribeClusterScope:                       {operation: "AdminClientDescribeCluster", tags: map[string]string{CadenceRoleTagName: AdminRoleTagValue}},
		AdminClientRefreshWorkflowTasksScope:                  {operation: "AdminClientRefreshWorkflowTasks", tags: map[string]string{CadenceRoleTagName: AdminRoleTagValue}},
		AdminClientResendReplicationTasksScope:                {operation: "AdminClientResendReplicationTasks", tags: map[string]string{CadenceRoleTagName: AdminRoleTagValue}},
		AdminClientCloseShardScope:                            {operation: "AdminClientCloseShard", tags: map[string]string{CadenceRoleTagName: AdminRoleTagValue}},
		AdminClientReadDLQMessagesScope:                       {operation: "AdminClientReadDLQMessages", tags: map[string]string{CadenceRoleTagName: AdminRoleTagValue}},
		AdminClientPurgeDLQMessagesScope:                      {operation: "AdminClientPurgeDLQMessages", tags: map[string]string{CadenceRoleTagName: AdminRoleTagValue}},
		AdminClientMergeDLQMessagesScope:                      {operation: "AdminClientMergeDLQMessages", tags: map[string]string{CadenceRoleTagName: AdminRoleTagValue}},
		DCRedirectionDeprecateDomainScope:                     {operation: "DCRedirectionDeprecateDomain", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionDescribeDomainScope:                      {operation: "DCRedirectionDescribeDomain", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionDescribeTaskListScope:                    {operation: "DCRedirectionDescribeTaskList", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionDescribeWorkflowExecutionScope:           {operation: "DCRedirectionDescribeWorkflowExecution", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionGetWorkflowExecutionHistoryScope:         {operation: "DCRedirectionGetWorkflowExecutionHistory", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionGetWorkflowExecutionRawHistoryScope:      {operation: "DCRedirectionGetWorkflowExecutionRawHistoryScope", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionPollForWorklfowExecutionRawHistoryScope:  {operation: "DCRedirectionPollForWorklfowExecutionRawHistory", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionListArchivedWorkflowExecutionsScope:      {operation: "DCRedirectionListArchivedWorkflowExecutions", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionListClosedWorkflowExecutionsScope:        {operation: "DCRedirectionListClosedWorkflowExecutions", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionListDomainsScope:                         {operation: "DCRedirectionListDomains", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionListOpenWorkflowExecutionsScope:          {operation: "DCRedirectionListOpenWorkflowExecutions", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionListWorkflowExecutionsScope:              {operation: "DCRedirectionListWorkflowExecutions", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionScanWorkflowExecutionsScope:              {operation: "DCRedirectionScanWorkflowExecutions", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionCountWorkflowExecutionsScope:             {operation: "DCRedirectionCountWorkflowExecutions", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionGetSearchAttributesScope:                 {operation: "DCRedirectionGetSearchAttributes", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionPollForActivityTaskScope:                 {operation: "DCRedirectionPollForActivityTask", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionPollForDecisionTaskScope:                 {operation: "DCRedirectionPollForDecisionTask", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionQueryWorkflowScope:                       {operation: "DCRedirectionQueryWorkflow", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionRecordActivityTaskHeartbeatScope:         {operation: "DCRedirectionRecordActivityTaskHeartbeat", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionRecordActivityTaskHeartbeatByIDScope:     {operation: "DCRedirectionRecordActivityTaskHeartbeatByID", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionRegisterDomainScope:                      {operation: "DCRedirectionRegisterDomain", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionRequestCancelWorkflowExecutionScope:      {operation: "DCRedirectionRequestCancelWorkflowExecution", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionResetStickyTaskListScope:                 {operation: "DCRedirectionResetStickyTaskList", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionResetWorkflowExecutionScope:              {operation: "DCRedirectionResetWorkflowExecution", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionRespondActivityTaskCanceledScope:         {operation: "DCRedirectionRespondActivityTaskCanceled", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionRespondActivityTaskCanceledByIDScope:     {operation: "DCRedirectionRespondActivityTaskCanceledByID", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionRespondActivityTaskCompletedScope:        {operation: "DCRedirectionRespondActivityTaskCompleted", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionRespondActivityTaskCompletedByIDScope:    {operation: "DCRedirectionRespondActivityTaskCompletedByID", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionRespondActivityTaskFailedScope:           {operation: "DCRedirectionRespondActivityTaskFailed", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionRespondActivityTaskFailedByIDScope:       {operation: "DCRedirectionRespondActivityTaskFailedByID", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionRespondDecisionTaskCompletedScope:        {operation: "DCRedirectionRespondDecisionTaskCompleted", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionRespondDecisionTaskFailedScope:           {operation: "DCRedirectionRespondDecisionTaskFailed", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionRespondQueryTaskCompletedScope:           {operation: "DCRedirectionRespondQueryTaskCompleted", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionSignalWithStartWorkflowExecutionScope:    {operation: "DCRedirectionSignalWithStartWorkflowExecution", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionSignalWorkflowExecutionScope:             {operation: "DCRedirectionSignalWorkflowExecution", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionStartWorkflowExecutionScope:              {operation: "DCRedirectionStartWorkflowExecution", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionTerminateWorkflowExecutionScope:          {operation: "DCRedirectionTerminateWorkflowExecution", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionUpdateDomainScope:                        {operation: "DCRedirectionUpdateDomain", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},
		DCRedirectionListTaskListPartitionsScope:              {operation: "DCRedirectionListTaskListPartitions", tags: map[string]string{CadenceRoleTagName: DCRedirectionRoleTagValue}},

		MessagingClientPublishScope:      {operation: "MessagingClientPublish"},
		MessagingClientPublishBatchScope: {operation: "MessagingClientPublishBatch"},

		DomainCacheScope:                                      {operation: "DomainCache"},
		HistoryRereplicationByTransferTaskScope:               {operation: "HistoryRereplicationByTransferTask"},
		HistoryRereplicationByTimerTaskScope:                  {operation: "HistoryRereplicationByTimerTask"},
		HistoryRereplicationByHistoryReplicationScope:         {operation: "HistoryRereplicationByHistoryReplication"},
		HistoryRereplicationByHistoryMetadataReplicationScope: {operation: "HistoryRereplicationByHistoryMetadataReplication"},
		HistoryRereplicationByActivityReplicationScope:        {operation: "HistoryRereplicationByActivityReplication"},

		ElasticsearchRecordWorkflowExecutionStartedScope:           {operation: "RecordWorkflowExecutionStarted"},
		ElasticsearchRecordWorkflowExecutionClosedScope:            {operation: "RecordWorkflowExecutionClosed"},
		ElasticsearchUpsertWorkflowExecutionScope:                  {operation: "UpsertWorkflowExecution"},
		ElasticsearchListOpenWorkflowExecutionsScope:               {operation: "ListOpenWorkflowExecutions"},
		ElasticsearchListClosedWorkflowExecutionsScope:             {operation: "ListClosedWorkflowExecutions"},
		ElasticsearchListOpenWorkflowExecutionsByTypeScope:         {operation: "ListOpenWorkflowExecutionsByType"},
		ElasticsearchListClosedWorkflowExecutionsByTypeScope:       {operation: "ListClosedWorkflowExecutionsByType"},
		ElasticsearchListOpenWorkflowExecutionsByWorkflowIDScope:   {operation: "ListOpenWorkflowExecutionsByWorkflowID"},
		ElasticsearchListClosedWorkflowExecutionsByWorkflowIDScope: {operation: "ListClosedWorkflowExecutionsByWorkflowID"},
		ElasticsearchListClosedWorkflowExecutionsByStatusScope:     {operation: "ListClosedWorkflowExecutionsByStatus"},
		ElasticsearchGetClosedWorkflowExecutionScope:               {operation: "GetClosedWorkflowExecution"},
		ElasticsearchListWorkflowExecutionsScope:                   {operation: "ListWorkflowExecutions"},
		ElasticsearchScanWorkflowExecutionsScope:                   {operation: "ScanWorkflowExecutions"},
		ElasticsearchCountWorkflowExecutionsScope:                  {operation: "CountWorkflowExecutions"},
		ElasticsearchDeleteWorkflowExecutionsScope:                 {operation: "DeleteWorkflowExecution"},
		SequentialTaskProcessingScope:                              {operation: "SequentialTaskProcessing"},
		ParallelTaskProcessingScope:                                {operation: "ParallelTaskProcessing"},
		TaskSchedulerScope:                                         {operation: "TaskScheduler"},

		HistoryArchiverScope:    {operation: "HistoryArchiver"},
		VisibilityArchiverScope: {operation: "VisibilityArchiver"},

		BlobstoreClientUploadScope:          {operation: "BlobstoreClientUpload", tags: map[string]string{CadenceRoleTagName: BlobstoreRoleTagValue}},
		BlobstoreClientDownloadScope:        {operation: "BlobstoreClientDownload", tags: map[string]string{CadenceRoleTagName: BlobstoreRoleTagValue}},
		BlobstoreClientGetMetadataScope:     {operation: "BlobstoreClientGetMetadata", tags: map[string]string{CadenceRoleTagName: BlobstoreRoleTagValue}},
		BlobstoreClientExistsScope:          {operation: "BlobstoreClientExists", tags: map[string]string{CadenceRoleTagName: BlobstoreRoleTagValue}},
		BlobstoreClientDeleteScope:          {operation: "BlobstoreClientDelete", tags: map[string]string{CadenceRoleTagName: BlobstoreRoleTagValue}},
		BlobstoreClientDirectoryExistsScope: {operation: "BlobstoreClientDirectoryExists", tags: map[string]string{CadenceRoleTagName: BlobstoreRoleTagValue}},

		DomainFailoverScope: {operation: "DomainFailover"},
	},
	// Frontend Scope Names
	Frontend: {
		// Admin API scope co-locates with with frontend
		AdminRemoveTaskScope:                       {operation: "AdminRemoveTask"},
		AdminCloseShardTaskScope:                   {operation: "AdminCloseShardTask"},
		AdminReadDLQMessagesScope:                  {operation: "AdminReadDLQMessages"},
		AdminPurgeDLQMessagesScope:                 {operation: "AdminPurgeDLQMessages"},
		AdminMergeDLQMessagesScope:                 {operation: "AdminMergeDLQMessages"},
		AdminDescribeHistoryHostScope:              {operation: "DescribeHistoryHost"},
		AdminAddSearchAttributeScope:               {operation: "AddSearchAttribute"},
		AdminDescribeWorkflowExecutionScope:        {operation: "DescribeWorkflowExecution"},
		AdminGetWorkflowExecutionRawHistoryScope:   {operation: "GetWorkflowExecutionRawHistory"},
		AdminGetWorkflowExecutionRawHistoryV2Scope: {operation: "GetWorkflowExecutionRawHistoryV2"},
		AdminGetReplicationMessagesScope:           {operation: "GetReplicationMessages"},
		AdminGetDomainReplicationMessagesScope:     {operation: "GetDomainReplicationMessages"},
		AdminGetDLQReplicationMessagesScope:        {operation: "AdminGetDLQReplicationMessages"},
		AdminReapplyEventsScope:                    {operation: "ReapplyEvents"},
		AdminRefreshWorkflowTasksScope:             {operation: "RefreshWorkflowTasks"},
		AdminResendReplicationTasksScope:           {operation: "ResendReplicationTasks"},

		FrontendStartWorkflowExecutionScope:             {operation: "StartWorkflowExecution"},
		FrontendPollForDecisionTaskScope:                {operation: "PollForDecisionTask"},
		FrontendPollForActivityTaskScope:                {operation: "PollForActivityTask"},
		FrontendRecordActivityTaskHeartbeatScope:        {operation: "RecordActivityTaskHeartbeat"},
		FrontendRecordActivityTaskHeartbeatByIDScope:    {operation: "RecordActivityTaskHeartbeatByID"},
		FrontendRespondDecisionTaskCompletedScope:       {operation: "RespondDecisionTaskCompleted"},
		FrontendRespondDecisionTaskFailedScope:          {operation: "RespondDecisionTaskFailed"},
		FrontendRespondQueryTaskCompletedScope:          {operation: "RespondQueryTaskCompleted"},
		FrontendRespondActivityTaskCompletedScope:       {operation: "RespondActivityTaskCompleted"},
		FrontendRespondActivityTaskFailedScope:          {operation: "RespondActivityTaskFailed"},
		FrontendRespondActivityTaskCanceledScope:        {operation: "RespondActivityTaskCanceled"},
		FrontendRespondActivityTaskCompletedByIDScope:   {operation: "RespondActivityTaskCompletedByID"},
		FrontendRespondActivityTaskFailedByIDScope:      {operation: "RespondActivityTaskFailedByID"},
		FrontendRespondActivityTaskCanceledByIDScope:    {operation: "RespondActivityTaskCanceledByID"},
		FrontendGetWorkflowExecutionHistoryScope:        {operation: "GetWorkflowExecutionHistory"},
		FrontendGetWorkflowExecutionRawHistoryScope:     {operation: "GetWorkflowExecutionRawHistory"},
		FrontendPollForWorklfowExecutionRawHistoryScope: {operation: "PollForWorklfowExecutionRawHistory"},
		FrontendSignalWorkflowExecutionScope:            {operation: "SignalWorkflowExecution"},
		FrontendSignalWithStartWorkflowExecutionScope:   {operation: "SignalWithStartWorkflowExecution"},
		FrontendTerminateWorkflowExecutionScope:         {operation: "TerminateWorkflowExecution"},
		FrontendResetWorkflowExecutionScope:             {operation: "ResetWorkflowExecution"},
		FrontendRequestCancelWorkflowExecutionScope:     {operation: "RequestCancelWorkflowExecution"},
		FrontendListArchivedWorkflowExecutionsScope:     {operation: "ListArchivedWorkflowExecutions"},
		FrontendListOpenWorkflowExecutionsScope:         {operation: "ListOpenWorkflowExecutions"},
		FrontendListClosedWorkflowExecutionsScope:       {operation: "ListClosedWorkflowExecutions"},
		FrontendListWorkflowExecutionsScope:             {operation: "ListWorkflowExecutions"},
		FrontendScanWorkflowExecutionsScope:             {operation: "ScanWorkflowExecutions"},
		FrontendCountWorkflowExecutionsScope:            {operation: "CountWorkflowExecutions"},
		FrontendRegisterDomainScope:                     {operation: "RegisterDomain"},
		FrontendDescribeDomainScope:                     {operation: "DescribeDomain"},
		FrontendListDomainsScope:                        {operation: "ListDomain"},
		FrontendUpdateDomainScope:                       {operation: "UpdateDomain"},
		FrontendDeprecateDomainScope:                    {operation: "DeprecateDomain"},
		FrontendQueryWorkflowScope:                      {operation: "QueryWorkflow"},
		FrontendDescribeWorkflowExecutionScope:          {operation: "DescribeWorkflowExecution"},
		FrontendListTaskListPartitionsScope:             {operation: "FrontendListTaskListPartitions"},
		FrontendDescribeTaskListScope:                   {operation: "DescribeTaskList"},
		FrontendResetStickyTaskListScope:                {operation: "ResetStickyTaskList"},
		FrontendGetSearchAttributesScope:                {operation: "GetSearchAttributes"},
	},
	// History Scope Names
	History: {
		HistoryStartWorkflowExecutionScope:                     {operation: "StartWorkflowExecution"},
		HistoryRecordActivityTaskHeartbeatScope:                {operation: "RecordActivityTaskHeartbeat"},
		HistoryRespondDecisionTaskCompletedScope:               {operation: "RespondDecisionTaskCompleted"},
		HistoryRespondDecisionTaskFailedScope:                  {operation: "RespondDecisionTaskFailed"},
		HistoryRespondActivityTaskCompletedScope:               {operation: "RespondActivityTaskCompleted"},
		HistoryRespondActivityTaskFailedScope:                  {operation: "RespondActivityTaskFailed"},
		HistoryRespondActivityTaskCanceledScope:                {operation: "RespondActivityTaskCanceled"},
		HistoryGetMutableStateScope:                            {operation: "GetMutableState"},
		HistoryPollMutableStateScope:                           {operation: "PollMutableState"},
		HistoryResetStickyTaskListScope:                        {operation: "ResetStickyTaskListScope"},
		HistoryDescribeWorkflowExecutionScope:                  {operation: "DescribeWorkflowExecution"},
		HistoryRecordDecisionTaskStartedScope:                  {operation: "RecordDecisionTaskStarted"},
		HistoryRecordActivityTaskStartedScope:                  {operation: "RecordActivityTaskStarted"},
		HistorySignalWorkflowExecutionScope:                    {operation: "SignalWorkflowExecution"},
		HistorySignalWithStartWorkflowExecutionScope:           {operation: "SignalWithStartWorkflowExecution"},
		HistoryRemoveSignalMutableStateScope:                   {operation: "RemoveSignalMutableState"},
		HistoryTerminateWorkflowExecutionScope:                 {operation: "TerminateWorkflowExecution"},
		HistoryResetWorkflowExecutionScope:                     {operation: "ResetWorkflowExecution"},
		HistoryQueryWorkflowScope:                              {operation: "QueryWorkflow"},
		HistoryProcessDeleteHistoryEventScope:                  {operation: "ProcessDeleteHistoryEvent"},
		HistoryScheduleDecisionTaskScope:                       {operation: "ScheduleDecisionTask"},
		HistoryRecordChildExecutionCompletedScope:              {operation: "RecordChildExecutionCompleted"},
		HistoryRequestCancelWorkflowExecutionScope:             {operation: "RequestCancelWorkflowExecution"},
		HistoryReplicateEventsScope:                            {operation: "ReplicateEvents"},
		HistoryReplicateRawEventsScope:                         {operation: "ReplicateRawEvents"},
		HistoryReplicateEventsV2Scope:                          {operation: "ReplicateEventsV2"},
		HistorySyncShardStatusScope:                            {operation: "SyncShardStatus"},
		HistorySyncActivityScope:                               {operation: "SyncActivity"},
		HistoryDescribeMutableStateScope:                       {operation: "DescribeMutableState"},
		HistoryGetReplicationMessagesScope:                     {operation: "GetReplicationMessages"},
		HistoryGetDLQReplicationMessagesScope:                  {operation: "GetDLQReplicationMessages"},
		HistoryReadDLQMessagesScope:                            {operation: "ReadDLQMessages"},
		HistoryPurgeDLQMessagesScope:                           {operation: "PurgeDLQMessages"},
		HistoryMergeDLQMessagesScope:                           {operation: "MergeDLQMessages"},
		HistoryShardControllerScope:                            {operation: "ShardController"},
		HistoryReapplyEventsScope:                              {operation: "EventReapplication"},
		HistoryRefreshWorkflowTasksScope:                       {operation: "RefreshWorkflowTasks"},
		HistoryNotifyFailoverMarkersScope:                      {operation: "NotifyFailoverMarkers"},
		TaskPriorityAssignerScope:                              {operation: "TaskPriorityAssigner"},
		TransferQueueProcessorScope:                            {operation: "TransferQueueProcessor"},
		TransferActiveQueueProcessorScope:                      {operation: "TransferActiveQueueProcessor"},
		TransferStandbyQueueProcessorScope:                     {operation: "TransferStandbyQueueProcessor"},
		TransferActiveTaskActivityScope:                        {operation: "TransferActiveTaskActivity"},
		TransferActiveTaskDecisionScope:                        {operation: "TransferActiveTaskDecision"},
		TransferActiveTaskCloseExecutionScope:                  {operation: "TransferActiveTaskCloseExecution"},
		TransferActiveTaskCancelExecutionScope:                 {operation: "TransferActiveTaskCancelExecution"},
		TransferActiveTaskSignalExecutionScope:                 {operation: "TransferActiveTaskSignalExecution"},
		TransferActiveTaskStartChildExecutionScope:             {operation: "TransferActiveTaskStartChildExecution"},
		TransferActiveTaskRecordWorkflowStartedScope:           {operation: "TransferActiveTaskRecordWorkflowStarted"},
		TransferActiveTaskResetWorkflowScope:                   {operation: "TransferActiveTaskResetWorkflow"},
		TransferActiveTaskUpsertWorkflowSearchAttributesScope:  {operation: "TransferActiveTaskUpsertWorkflowSearchAttributes"},
		TransferStandbyTaskActivityScope:                       {operation: "TransferStandbyTaskActivity"},
		TransferStandbyTaskDecisionScope:                       {operation: "TransferStandbyTaskDecision"},
		TransferStandbyTaskCloseExecutionScope:                 {operation: "TransferStandbyTaskCloseExecution"},
		TransferStandbyTaskCancelExecutionScope:                {operation: "TransferStandbyTaskCancelExecution"},
		TransferStandbyTaskSignalExecutionScope:                {operation: "TransferStandbyTaskSignalExecution"},
		TransferStandbyTaskStartChildExecutionScope:            {operation: "TransferStandbyTaskStartChildExecution"},
		TransferStandbyTaskRecordWorkflowStartedScope:          {operation: "TransferStandbyTaskRecordWorkflowStarted"},
		TransferStandbyTaskResetWorkflowScope:                  {operation: "TransferStandbyTaskResetWorkflow"},
		TransferStandbyTaskUpsertWorkflowSearchAttributesScope: {operation: "TransferStandbyTaskUpsertWorkflowSearchAttributes"},
		TimerQueueProcessorScope:                               {operation: "TimerQueueProcessor"},
		TimerActiveQueueProcessorScope:                         {operation: "TimerActiveQueueProcessor"},
		TimerStandbyQueueProcessorScope:                        {operation: "TimerStandbyQueueProcessor"},
		TimerActiveTaskActivityTimeoutScope:                    {operation: "TimerActiveTaskActivityTimeout"},
		TimerActiveTaskDecisionTimeoutScope:                    {operation: "TimerActiveTaskDecisionTimeout"},
		TimerActiveTaskUserTimerScope:                          {operation: "TimerActiveTaskUserTimer"},
		TimerActiveTaskWorkflowTimeoutScope:                    {operation: "TimerActiveTaskWorkflowTimeout"},
		TimerActiveTaskActivityRetryTimerScope:                 {operation: "TimerActiveTaskActivityRetryTimer"},
		TimerActiveTaskWorkflowBackoffTimerScope:               {operation: "TimerActiveTaskWorkflowBackoffTimer"},
		TimerActiveTaskDeleteHistoryEventScope:                 {operation: "TimerActiveTaskDeleteHistoryEvent"},
		TimerStandbyTaskActivityTimeoutScope:                   {operation: "TimerStandbyTaskActivityTimeout"},
		TimerStandbyTaskDecisionTimeoutScope:                   {operation: "TimerStandbyTaskDecisionTimeout"},
		TimerStandbyTaskUserTimerScope:                         {operation: "TimerStandbyTaskUserTimer"},
		TimerStandbyTaskWorkflowTimeoutScope:                   {operation: "TimerStandbyTaskWorkflowTimeout"},
		TimerStandbyTaskActivityRetryTimerScope:                {operation: "TimerStandbyTaskActivityRetryTimer"},
		TimerStandbyTaskWorkflowBackoffTimerScope:              {operation: "TimerStandbyTaskWorkflowBackoffTimer"},
		TimerStandbyTaskDeleteHistoryEventScope:                {operation: "TimerStandbyTaskDeleteHistoryEvent"},
		HistoryEventNotificationScope:                          {operation: "HistoryEventNotification"},
		ReplicatorQueueProcessorScope:                          {operation: "ReplicatorQueueProcessor"},
		ReplicatorTaskHistoryScope:                             {operation: "ReplicatorTaskHistory"},
		ReplicatorTaskSyncActivityScope:                        {operation: "ReplicatorTaskSyncActivity"},
		ReplicateHistoryEventsScope:                            {operation: "ReplicateHistoryEvents"},
		ShardInfoScope:                                         {operation: "ShardInfo"},
		WorkflowContextScope:                                   {operation: "WorkflowContext"},
		HistoryCacheGetAndCreateScope:                          {operation: "HistoryCacheGetAndCreate", tags: map[string]string{CacheTypeTagName: MutableStateCacheTypeTagValue}},
		HistoryCacheGetOrCreateScope:                           {operation: "HistoryCacheGetOrCreate", tags: map[string]string{CacheTypeTagName: MutableStateCacheTypeTagValue}},
		HistoryCacheGetOrCreateCurrentScope:                    {operation: "HistoryCacheGetOrCreateCurrent", tags: map[string]string{CacheTypeTagName: MutableStateCacheTypeTagValue}},
		HistoryCacheGetCurrentExecutionScope:                   {operation: "HistoryCacheGetCurrentExecution", tags: map[string]string{CacheTypeTagName: MutableStateCacheTypeTagValue}},
		EventsCacheGetEventScope:                               {operation: "EventsCacheGetEvent", tags: map[string]string{CacheTypeTagName: EventsCacheTypeTagValue}},
		EventsCachePutEventScope:                               {operation: "EventsCachePutEvent", tags: map[string]string{CacheTypeTagName: EventsCacheTypeTagValue}},
		EventsCacheGetFromStoreScope:                           {operation: "EventsCacheGetFromStore", tags: map[string]string{CacheTypeTagName: EventsCacheTypeTagValue}},
		ExecutionSizeStatsScope:                                {operation: "ExecutionStats", tags: map[string]string{StatsTypeTagName: SizeStatsTypeTagValue}},
		ExecutionCountStatsScope:                               {operation: "ExecutionStats", tags: map[string]string{StatsTypeTagName: CountStatsTypeTagValue}},
		SessionSizeStatsScope:                                  {operation: "SessionStats", tags: map[string]string{StatsTypeTagName: SizeStatsTypeTagValue}},
		SessionCountStatsScope:                                 {operation: "SessionStats", tags: map[string]string{StatsTypeTagName: CountStatsTypeTagValue}},
		WorkflowCompletionStatsScope:                           {operation: "CompletionStats", tags: map[string]string{StatsTypeTagName: CountStatsTypeTagValue}},
		ArchiverClientScope:                                    {operation: "ArchiverClient"},
		ReplicationTaskFetcherScope:                            {operation: "ReplicationTaskFetcher"},
		ReplicationTaskCleanupScope:                            {operation: "ReplicationTaskCleanup"},
		ReplicationDLQStatsScope:                               {operation: "ReplicationDLQStats"},
		HistoryFailoverMarkerScope:                             {operation: "FailoverMarker"},
	},
	// Matching Scope Names
	Matching: {
		MatchingPollForDecisionTaskScope:       {operation: "PollForDecisionTask"},
		MatchingPollForActivityTaskScope:       {operation: "PollForActivityTask"},
		MatchingAddActivityTaskScope:           {operation: "AddActivityTask"},
		MatchingAddDecisionTaskScope:           {operation: "AddDecisionTask"},
		MatchingTaskListMgrScope:               {operation: "TaskListMgr"},
		MatchingQueryWorkflowScope:             {operation: "QueryWorkflow"},
		MatchingRespondQueryTaskCompletedScope: {operation: "RespondQueryTaskCompleted"},
		MatchingCancelOutstandingPollScope:     {operation: "CancelOutstandingPoll"},
		MatchingDescribeTaskListScope:          {operation: "DescribeTaskList"},
		MatchingListTaskListPartitionsScope:    {operation: "ListTaskListPartitions"},
	},
	// Worker Scope Names
	Worker: {
		ReplicatorScope:                        {operation: "Replicator"},
		DomainReplicationTaskScope:             {operation: "DomainReplicationTask"},
		HistoryReplicationTaskScope:            {operation: "HistoryReplicationTask"},
		HistoryMetadataReplicationTaskScope:    {operation: "HistoryMetadataReplicationTask"},
		HistoryReplicationV2TaskScope:          {operation: "HistoryReplicationV2Task"},
		SyncShardTaskScope:                     {operation: "SyncShardTask"},
		SyncActivityTaskScope:                  {operation: "SyncActivityTask"},
		ESProcessorScope:                       {operation: "ESProcessor"},
		IndexProcessorScope:                    {operation: "IndexProcessor"},
		ArchiverDeleteHistoryActivityScope:     {operation: "ArchiverDeleteHistoryActivity"},
		ArchiverUploadHistoryActivityScope:     {operation: "ArchiverUploadHistoryActivity"},
		ArchiverArchiveVisibilityActivityScope: {operation: "ArchiverArchiveVisibilityActivity"},
		ArchiverScope:                          {operation: "Archiver"},
		ArchiverPumpScope:                      {operation: "ArchiverPump"},
		ArchiverArchivalWorkflowScope:          {operation: "ArchiverArchivalWorkflow"},
		TaskListScavengerScope:                 {operation: "tasklistscavenger"},
		ExecutionsScannerScope:                 {operation: "ExecutionsScanner"},
		ExecutionsFixerScope:                   {operation: "ExecutionsFixer"},
		HistoryScavengerScope:                  {operation: "historyscavenger"},
		BatcherScope:                           {operation: "batcher"},
		ParentClosePolicyProcessorScope:        {operation: "ParentClosePolicyProcessor"},
	},
}

// Common Metrics enum
const (
	CadenceRequests = iota
	CadenceFailures
	CadenceCriticalFailures
	CadenceLatency
	CadenceErrBadRequestCounter
	CadenceErrDomainNotActiveCounter
	CadenceErrServiceBusyCounter
	CadenceErrEntityNotExistsCounter
	CadenceErrExecutionAlreadyStartedCounter
	CadenceErrDomainAlreadyExistsCounter
	CadenceErrCancellationAlreadyRequestedCounter
	CadenceErrQueryFailedCounter
	CadenceErrLimitExceededCounter
	CadenceErrContextTimeoutCounter
	CadenceErrRetryTaskCounter
	CadenceErrBadBinaryCounter
	CadenceErrClientVersionNotSupportedCounter
	CadenceErrIncompleteHistoryCounter
	CadenceErrNonDeterministicCounter
	CadenceErrUnauthorizedCounter
	CadenceErrAuthorizeFailedCounter
	CadenceErrRemoteSyncMatchFailedCounter
	PersistenceRequests
	PersistenceFailures
	PersistenceLatency
	PersistenceErrShardExistsCounter
	PersistenceErrShardOwnershipLostCounter
	PersistenceErrConditionFailedCounter
	PersistenceErrCurrentWorkflowConditionFailedCounter
	PersistenceErrTimeoutCounter
	PersistenceErrBusyCounter
	PersistenceErrEntityNotExistsCounter
	PersistenceErrExecutionAlreadyStartedCounter
	PersistenceErrDomainAlreadyExistsCounter
	PersistenceErrBadRequestCounter
	PersistenceSampledCounter

	CadenceClientRequests
	CadenceClientFailures
	CadenceClientLatency

	CadenceDcRedirectionClientRequests
	CadenceDcRedirectionClientFailures
	CadenceDcRedirectionClientLatency

	CadenceAuthorizationLatency

	DomainCachePrepareCallbacksLatency
	DomainCacheCallbacksLatency

	HistorySize
	HistoryCount
	EventBlobSize

	ArchivalConfigFailures

	ElasticsearchRequests
	ElasticsearchFailures
	ElasticsearchLatency
	ElasticsearchErrBadRequestCounter
	ElasticsearchErrBusyCounter

	SequentialTaskSubmitRequest
	SequentialTaskSubmitRequestTaskQueueExist
	SequentialTaskSubmitRequestTaskQueueMissing
	SequentialTaskSubmitLatency
	SequentialTaskQueueSize
	SequentialTaskQueueProcessingLatency
	SequentialTaskTaskProcessingLatency

	ParallelTaskSubmitRequest
	ParallelTaskSubmitLatency
	ParallelTaskTaskProcessingLatency

	PriorityTaskSubmitRequest
	PriorityTaskSubmitLatency

	HistoryArchiverArchiveNonRetryableErrorCount
	HistoryArchiverArchiveTransientErrorCount
	HistoryArchiverArchiveSuccessCount
	HistoryArchiverHistoryMutatedCount
	HistoryArchiverTotalUploadSize
	HistoryArchiverHistorySize

	// The following metrics are only used by internal history archiver implemention.
	// TODO: move them to internal repo once cadence plugin model is in place.
	HistoryArchiverBlobExistsCount
	HistoryArchiverBlobSize
	HistoryArchiverRunningDeterministicConstructionCheckCount
	HistoryArchiverDeterministicConstructionCheckFailedCount
	HistoryArchiverRunningBlobIntegrityCheckCount
	HistoryArchiverBlobIntegrityCheckFailedCount
	HistoryArchiverDuplicateArchivalsCount

	HistoryFailoverMarkerInsertFailure

	VisibilityArchiverArchiveNonRetryableErrorCount
	VisibilityArchiverArchiveTransientErrorCount
	VisibilityArchiveSuccessCount

	MatchingClientForwardedCounter
	MatchingClientInvalidTaskListName

	DomainReplicationTaskAckLevelGauge
	DomainReplicationDLQAckLevelGauge
	DomainReplicationDLQMaxLevelGauge

	// common metrics that are emitted per task list
	CadenceRequestsPerTaskList
	CadenceFailuresPerTaskList
	CadenceLatencyPerTaskList
	CadenceErrBadRequestPerTaskListCounter
	CadenceErrDomainNotActivePerTaskListCounter
	CadenceErrServiceBusyPerTaskListCounter
	CadenceErrEntityNotExistsPerTaskListCounter
	CadenceErrExecutionAlreadyStartedPerTaskListCounter
	CadenceErrDomainAlreadyExistsPerTaskListCounter
	CadenceErrCancellationAlreadyRequestedPerTaskListCounter
	CadenceErrQueryFailedPerTaskListCounter
	CadenceErrLimitExceededPerTaskListCounter
	CadenceErrContextTimeoutPerTaskListCounter
	CadenceErrRetryTaskPerTaskListCounter
	CadenceErrBadBinaryPerTaskListCounter
	CadenceErrClientVersionNotSupportedPerTaskListCounter
	CadenceErrIncompleteHistoryPerTaskListCounter
	CadenceErrNonDeterministicPerTaskListCounter
	CadenceErrUnauthorizedPerTaskListCounter
	CadenceErrAuthorizeFailedPerTaskListCounter
	CadenceErrRemoteSyncMatchFailedPerTaskListCounter

	CadenceShardSuccessGauge
	CadenceShardFailureGauge

	NumCommonMetrics // Needs to be last on this list for iota numbering
)

// History Metrics enum
const (
	TaskRequests = iota + NumCommonMetrics
	TaskLatency
	TaskFailures
	TaskDiscarded
	TaskAttemptTimer
	TaskStandbyRetryCounter
	TaskNotActiveCounter
	TaskLimitExceededCounter
	TaskBatchCompleteCounter
	TaskProcessingLatency
	TaskQueueLatency
	TaskRedispatchQueuePendingTasksTimer

	TransferTaskThrottledCounter
	TimerTaskThrottledCounter

	ActivityE2ELatency
	AckLevelUpdateCounter
	AckLevelUpdateFailedCounter
	DecisionTypeScheduleActivityCounter
	DecisionTypeCompleteWorkflowCounter
	DecisionTypeFailWorkflowCounter
	DecisionTypeCancelWorkflowCounter
	DecisionTypeStartTimerCounter
	DecisionTypeCancelActivityCounter
	DecisionTypeCancelTimerCounter
	DecisionTypeRecordMarkerCounter
	DecisionTypeCancelExternalWorkflowCounter
	DecisionTypeChildWorkflowCounter
	DecisionTypeContinueAsNewCounter
	DecisionTypeSignalExternalWorkflowCounter
	DecisionTypeUpsertWorkflowSearchAttributesCounter
	EmptyCompletionDecisionsCounter
	MultipleCompletionDecisionsCounter
	FailedDecisionsCounter
	StaleMutableStateCounter
	AutoResetPointsLimitExceededCounter
	AutoResetPointCorruptionCounter
	ConcurrencyUpdateFailureCounter
	CadenceErrEventAlreadyStartedCounter
	CadenceErrShardOwnershipLostCounter
	HeartbeatTimeoutCounter
	ScheduleToStartTimeoutCounter
	StartToCloseTimeoutCounter
	ScheduleToCloseTimeoutCounter
	NewTimerCounter
	NewTimerNotifyCounter
	AcquireShardsCounter
	AcquireShardsLatency
	ShardClosedCounter
	ShardItemCreatedCounter
	ShardItemRemovedCounter
	ShardItemAcquisitionLatency
	ShardInfoReplicationPendingTasksTimer
	ShardInfoTransferActivePendingTasksTimer
	ShardInfoTransferStandbyPendingTasksTimer
	ShardInfoTimerActivePendingTasksTimer
	ShardInfoTimerStandbyPendingTasksTimer
	ShardInfoReplicationLagTimer
	ShardInfoTransferLagTimer
	ShardInfoTimerLagTimer
	ShardInfoTransferDiffTimer
	ShardInfoTimerDiffTimer
	ShardInfoTransferFailoverInProgressTimer
	ShardInfoTimerFailoverInProgressTimer
	ShardInfoTransferFailoverLatencyTimer
	ShardInfoTimerFailoverLatencyTimer
	SyncShardFromRemoteCounter
	SyncShardFromRemoteFailure
	MembershipChangedCounter
	NumShardsGauge
	GetEngineForShardErrorCounter
	GetEngineForShardLatency
	RemoveEngineForShardLatency
	CompleteDecisionWithStickyEnabledCounter
	CompleteDecisionWithStickyDisabledCounter
	DecisionHeartbeatTimeoutCounter
	HistoryEventNotificationQueueingLatency
	HistoryEventNotificationFanoutLatency
	HistoryEventNotificationInFlightMessageGauge
	HistoryEventNotificationFailDeliveryCount
	EmptyReplicationEventsCounter
	DuplicateReplicationEventsCounter
	StaleReplicationEventsCounter
	ReplicationEventsSizeTimer
	BufferReplicationTaskTimer
	UnbufferReplicationTaskTimer
	HistoryConflictsCounter
	CompleteTaskFailedCounter
	CacheRequests
	CacheFailures
	CacheLatency
	CacheMissCounter
	AcquireLockFailedCounter
	WorkflowContextCleared
	MutableStateSize
	ExecutionInfoSize
	ActivityInfoSize
	TimerInfoSize
	ChildInfoSize
	SignalInfoSize
	BufferedEventsSize
	ActivityInfoCount
	TimerInfoCount
	ChildInfoCount
	SignalInfoCount
	RequestCancelInfoCount
	BufferedEventsCount
	DeleteActivityInfoCount
	DeleteTimerInfoCount
	DeleteChildInfoCount
	DeleteSignalInfoCount
	DeleteRequestCancelInfoCount
	WorkflowRetryBackoffTimerCount
	WorkflowCronBackoffTimerCount
	WorkflowCleanupDeleteCount
	WorkflowCleanupArchiveCount
	WorkflowCleanupNopCount
	WorkflowCleanupDeleteHistoryInlineCount
	WorkflowSuccessCount
	WorkflowCancelCount
	WorkflowFailedCount
	WorkflowTimeoutCount
	WorkflowTerminateCount
	ArchiverClientSendSignalCount
	ArchiverClientSendSignalFailureCount
	ArchiverClientHistoryRequestCount
	ArchiverClientHistoryInlineArchiveAttemptCount
	ArchiverClientHistoryInlineArchiveFailureCount
	ArchiverClientVisibilityRequestCount
	ArchiverClientVisibilityInlineArchiveAttemptCount
	ArchiverClientVisibilityInlineArchiveFailureCount
	LastRetrievedMessageID
	LastProcessedMessageID
	ReplicationTasksApplied
	ReplicationTasksFailed
	ReplicationTasksLag
	ReplicationTasksFetched
	ReplicationTasksReturned
	ReplicationDLQFailed
	ReplicationDLQMaxLevelGauge
	ReplicationDLQAckLevelGauge
	GetReplicationMessagesForShardLatency
	GetDLQReplicationMessagesLatency
	EventReapplySkippedCount
	DirectQueryDispatchLatency
	DirectQueryDispatchStickyLatency
	DirectQueryDispatchNonStickyLatency
	DirectQueryDispatchStickySuccessCount
	DirectQueryDispatchNonStickySuccessCount
	DirectQueryDispatchClearStickinessLatency
	DirectQueryDispatchClearStickinessSuccessCount
	DirectQueryDispatchTimeoutBeforeNonStickyCount
	DecisionTaskQueryLatency
	ConsistentQueryTimeoutCount
	QueryBeforeFirstDecisionCount
	QueryBufferExceededCount
	QueryRegistryInvalidStateCount
	WorkerNotSupportsConsistentQueryCount
	DecisionStartToCloseTimeoutOverrideCount
	ReplicationTaskCleanupCount
	ReplicationTaskCleanupFailure
	MutableStateChecksumMismatch
	MutableStateChecksumInvalidated

	NumHistoryMetrics
)

// Matching metrics enum
const (
	PollSuccessPerTaskListCounter = iota + NumCommonMetrics
	PollTimeoutPerTaskListCounter
	PollSuccessWithSyncPerTaskListCounter
	LeaseRequestPerTaskListCounter
	LeaseFailurePerTaskListCounter
	ConditionFailedErrorPerTaskListCounter
	RespondQueryTaskFailedPerTaskListCounter
	SyncThrottlePerTaskListCounter
	BufferThrottlePerTaskListCounter
	SyncMatchLatencyPerTaskList
	AsyncMatchLatencyPerTaskList
	ExpiredTasksPerTaskListCounter
	ForwardedPerTaskListCounter
	ForwardTaskCallsPerTaskList
	ForwardTaskErrorsPerTaskList
	ForwardTaskLatencyPerTaskList
	ForwardQueryCallsPerTaskList
	ForwardQueryErrorsPerTaskList
	ForwardQueryLatencyPerTaskList
	ForwardPollCallsPerTaskList
	ForwardPollErrorsPerTaskList
	ForwardPollLatencyPerTaskList
	LocalToLocalMatchPerTaskListCounter
	LocalToRemoteMatchPerTaskListCounter
	RemoteToLocalMatchPerTaskListCounter
	RemoteToRemoteMatchPerTaskListCounter

	NumMatchingMetrics
)

// Worker metrics enum
const (
	ReplicatorMessages = iota + NumCommonMetrics
	ReplicatorFailures
	ReplicatorMessagesDropped
	ReplicatorLatency
	ReplicatorDLQFailures
	ESProcessorRequests
	ESProcessorRetries
	ESProcessorFailures
	ESProcessorCorruptedData
	ESProcessorProcessMsgLatency
	IndexProcessorCorruptedData
	IndexProcessorProcessMsgLatency
	ArchiverNonRetryableErrorCount
	ArchiverStartedCount
	ArchiverStoppedCount
	ArchiverCoroutineStartedCount
	ArchiverCoroutineStoppedCount
	ArchiverHandleHistoryRequestLatency
	ArchiverHandleVisibilityRequestLatency
	ArchiverUploadWithRetriesLatency
	ArchiverDeleteWithRetriesLatency
	ArchiverUploadFailedAllRetriesCount
	ArchiverUploadSuccessCount
	ArchiverDeleteFailedAllRetriesCount
	ArchiverDeleteSuccessCount
	ArchiverHandleVisibilityFailedAllRetiresCount
	ArchiverHandleVisibilitySuccessCount
	ArchiverBacklogSizeGauge
	ArchiverPumpTimeoutCount
	ArchiverPumpSignalThresholdCount
	ArchiverPumpTimeoutWithoutSignalsCount
	ArchiverPumpSignalChannelClosedCount
	ArchiverWorkflowStartedCount
	ArchiverNumPumpedRequestsCount
	ArchiverNumHandledRequestsCount
	ArchiverPumpedNotEqualHandledCount
	ArchiverHandleAllRequestsLatency
	ArchiverWorkflowStoppingCount
	TaskProcessedCount
	TaskDeletedCount
	TaskListProcessedCount
	TaskListDeletedCount
	TaskListOutstandingCount
	ExecutionsOutstandingCount
	StartedCount
	StoppedCount
	ExecutorTasksDeferredCount
	ExecutorTasksDroppedCount
	BatcherProcessorSuccess
	BatcherProcessorFailures
	HistoryScavengerSuccessCount
	HistoryScavengerErrorCount
	HistoryScavengerSkipCount
	ParentClosePolicyProcessorSuccess
	ParentClosePolicyProcessorFailures
	DomainReplicationEnqueueDLQCount
	ScannerExecutionsGauge
	ScannerCorruptedGauge
	ScannerCheckFailedGauge
	ScannerCorruptionByTypeGauge
	ScannerCorruptedOpenExecutionGauge
	ScannerShardSizeMax
	ScannerShardSizeMedian
	ScannerShardSizeMin
	ScannerShardSizeP90
	ScannerShardSizeP75
	ScannerShardSizeP25
	ScannerShardSizeP10

	NumWorkerMetrics
)

// MetricDefs record the metrics for all services
var MetricDefs = map[ServiceIdx]map[int]metricDefinition{
	Common: {
		CadenceRequests:                                     {metricName: "cadence_requests", metricType: Counter},
		CadenceFailures:                                     {metricName: "cadence_errors", metricType: Counter},
		CadenceCriticalFailures:                             {metricName: "cadence_errors_critical", metricType: Counter},
		CadenceLatency:                                      {metricName: "cadence_latency", metricType: Timer},
		CadenceErrBadRequestCounter:                         {metricName: "cadence_errors_bad_request", metricType: Counter},
		CadenceErrDomainNotActiveCounter:                    {metricName: "cadence_errors_domain_not_active", metricType: Counter},
		CadenceErrServiceBusyCounter:                        {metricName: "cadence_errors_service_busy", metricType: Counter},
		CadenceErrEntityNotExistsCounter:                    {metricName: "cadence_errors_entity_not_exists", metricType: Counter},
		CadenceErrExecutionAlreadyStartedCounter:            {metricName: "cadence_errors_execution_already_started", metricType: Counter},
		CadenceErrDomainAlreadyExistsCounter:                {metricName: "cadence_errors_domain_already_exists", metricType: Counter},
		CadenceErrCancellationAlreadyRequestedCounter:       {metricName: "cadence_errors_cancellation_already_requested", metricType: Counter},
		CadenceErrQueryFailedCounter:                        {metricName: "cadence_errors_query_failed", metricType: Counter},
		CadenceErrLimitExceededCounter:                      {metricName: "cadence_errors_limit_exceeded", metricType: Counter},
		CadenceErrContextTimeoutCounter:                     {metricName: "cadence_errors_context_timeout", metricType: Counter},
		CadenceErrRetryTaskCounter:                          {metricName: "cadence_errors_retry_task", metricType: Counter},
		CadenceErrBadBinaryCounter:                          {metricName: "cadence_errors_bad_binary", metricType: Counter},
		CadenceErrClientVersionNotSupportedCounter:          {metricName: "cadence_errors_client_version_not_supported", metricType: Counter},
		CadenceErrIncompleteHistoryCounter:                  {metricName: "cadence_errors_incomplete_history", metricType: Counter},
		CadenceErrNonDeterministicCounter:                   {metricName: "cadence_errors_nondeterministic", metricType: Counter},
		CadenceErrUnauthorizedCounter:                       {metricName: "cadence_errors_unauthorized", metricType: Counter},
		CadenceErrAuthorizeFailedCounter:                    {metricName: "cadence_errors_authorize_failed", metricType: Counter},
		CadenceErrRemoteSyncMatchFailedCounter:              {metricName: "cadence_errors_remote_syncmatch_failed", metricType: Counter},
		PersistenceRequests:                                 {metricName: "persistence_requests", metricType: Counter},
		PersistenceFailures:                                 {metricName: "persistence_errors", metricType: Counter},
		PersistenceLatency:                                  {metricName: "persistence_latency", metricType: Timer},
		PersistenceErrShardExistsCounter:                    {metricName: "persistence_errors_shard_exists", metricType: Counter},
		PersistenceErrShardOwnershipLostCounter:             {metricName: "persistence_errors_shard_ownership_lost", metricType: Counter},
		PersistenceErrConditionFailedCounter:                {metricName: "persistence_errors_condition_failed", metricType: Counter},
		PersistenceErrCurrentWorkflowConditionFailedCounter: {metricName: "persistence_errors_current_workflow_condition_failed", metricType: Counter},
		PersistenceErrTimeoutCounter:                        {metricName: "persistence_errors_timeout", metricType: Counter},
		PersistenceErrBusyCounter:                           {metricName: "persistence_errors_busy", metricType: Counter},
		PersistenceErrEntityNotExistsCounter:                {metricName: "persistence_errors_entity_not_exists", metricType: Counter},
		PersistenceErrExecutionAlreadyStartedCounter:        {metricName: "persistence_errors_execution_already_started", metricType: Counter},
		PersistenceErrDomainAlreadyExistsCounter:            {metricName: "persistence_errors_domain_already_exists", metricType: Counter},
		PersistenceErrBadRequestCounter:                     {metricName: "persistence_errors_bad_request", metricType: Counter},
		PersistenceSampledCounter:                           {metricName: "persistence_sampled", metricType: Counter},
		CadenceClientRequests:                               {metricName: "cadence_client_requests", metricType: Counter},
		CadenceClientFailures:                               {metricName: "cadence_client_errors", metricType: Counter},
		CadenceClientLatency:                                {metricName: "cadence_client_latency", metricType: Timer},
		CadenceDcRedirectionClientRequests:                  {metricName: "cadence_client_requests_redirection", metricType: Counter},
		CadenceDcRedirectionClientFailures:                  {metricName: "cadence_client_errors_redirection", metricType: Counter},
		CadenceDcRedirectionClientLatency:                   {metricName: "cadence_client_latency_redirection", metricType: Timer},
		CadenceAuthorizationLatency:                         {metricName: "cadence_authorization_latency", metricType: Timer},
		DomainCachePrepareCallbacksLatency:                  {metricName: "domain_cache_prepare_callbacks_latency", metricType: Timer},
		DomainCacheCallbacksLatency:                         {metricName: "domain_cache_callbacks_latency", metricType: Timer},
		HistorySize:                                         {metricName: "history_size", metricType: Timer},
		HistoryCount:                                        {metricName: "history_count", metricType: Timer},
		EventBlobSize:                                       {metricName: "event_blob_size", metricType: Timer},
		ArchivalConfigFailures:                              {metricName: "archivalconfig_failures", metricType: Counter},
		ElasticsearchRequests:                               {metricName: "elasticsearch_requests", metricType: Counter},
		ElasticsearchFailures:                               {metricName: "elasticsearch_errors", metricType: Counter},
		ElasticsearchLatency:                                {metricName: "elasticsearch_latency", metricType: Timer},
		ElasticsearchErrBadRequestCounter:                   {metricName: "elasticsearch_errors_bad_request", metricType: Counter},
		ElasticsearchErrBusyCounter:                         {metricName: "elasticsearch_errors_busy", metricType: Counter},
		SequentialTaskSubmitRequest:                         {metricName: "sequentialtask_submit_request", metricType: Counter},
		SequentialTaskSubmitRequestTaskQueueExist:           {metricName: "sequentialtask_submit_request_taskqueue_exist", metricType: Counter},
		SequentialTaskSubmitRequestTaskQueueMissing:         {metricName: "sequentialtask_submit_request_taskqueue_missing", metricType: Counter},
		SequentialTaskSubmitLatency:                         {metricName: "sequentialtask_submit_latency", metricType: Timer},
		SequentialTaskQueueSize:                             {metricName: "sequentialtask_queue_size", metricType: Timer},
		SequentialTaskQueueProcessingLatency:                {metricName: "sequentialtask_queue_processing_latency", metricType: Timer},
		SequentialTaskTaskProcessingLatency:                 {metricName: "sequentialtask_task_processing_latency", metricType: Timer},
		ParallelTaskSubmitRequest:                           {metricName: "paralleltask_submit_request", metricType: Counter},
		ParallelTaskSubmitLatency:                           {metricName: "paralleltask_submit_latency", metricType: Timer},
		ParallelTaskTaskProcessingLatency:                   {metricName: "paralleltask_task_processing_latency", metricType: Timer},
		PriorityTaskSubmitRequest:                           {metricName: "prioritytask_submit_request", metricType: Counter},
		PriorityTaskSubmitLatency:                           {metricName: "prioritytask_submit_latency", metricType: Timer},

		HistoryArchiverArchiveNonRetryableErrorCount:              {metricName: "history_archiver_archive_non_retryable_error", metricType: Counter},
		HistoryArchiverArchiveTransientErrorCount:                 {metricName: "history_archiver_archive_transient_error", metricType: Counter},
		HistoryArchiverArchiveSuccessCount:                        {metricName: "history_archiver_archive_success", metricType: Counter},
		HistoryArchiverHistoryMutatedCount:                        {metricName: "history_archiver_history_mutated", metricType: Counter},
		HistoryArchiverTotalUploadSize:                            {metricName: "history_archiver_total_upload_size", metricType: Timer},
		HistoryArchiverHistorySize:                                {metricName: "history_archiver_history_size", metricType: Timer},
		HistoryArchiverBlobExistsCount:                            {metricName: "history_archiver_blob_exists", metricType: Counter},
		HistoryArchiverBlobSize:                                   {metricName: "history_archiver_blob_size", metricType: Timer},
		HistoryArchiverRunningDeterministicConstructionCheckCount: {metricName: "history_archiver_running_deterministic_construction_check", metricType: Counter},
		HistoryArchiverDeterministicConstructionCheckFailedCount:  {metricName: "history_archiver_deterministic_construction_check_failed", metricType: Counter},
		HistoryArchiverRunningBlobIntegrityCheckCount:             {metricName: "history_archiver_running_blob_integrity_check", metricType: Counter},
		HistoryArchiverBlobIntegrityCheckFailedCount:              {metricName: "history_archiver_blob_integrity_check_failed", metricType: Counter},
		HistoryArchiverDuplicateArchivalsCount:                    {metricName: "history_archiver_duplicate_archivals", metricType: Counter},
		HistoryFailoverMarkerInsertFailure:                        {metricName: "history_failover_marker_insert_failures", metricType: Counter},
		VisibilityArchiverArchiveNonRetryableErrorCount:           {metricName: "visibility_archiver_archive_non_retryable_error", metricType: Counter},
		VisibilityArchiverArchiveTransientErrorCount:              {metricName: "visibility_archiver_archive_transient_error", metricType: Counter},
		VisibilityArchiveSuccessCount:                             {metricName: "visibility_archiver_archive_success", metricType: Counter},
		MatchingClientForwardedCounter:                            {metricName: "forwarded", metricType: Counter},
		MatchingClientInvalidTaskListName:                         {metricName: "invalid_task_list_name", metricType: Counter},

		DomainReplicationTaskAckLevelGauge: {metricName: "domain_replication_task_ack_level", metricType: Gauge},
		DomainReplicationDLQAckLevelGauge:  {metricName: "domain_dlq_ack_level", metricType: Gauge},
		DomainReplicationDLQMaxLevelGauge:  {metricName: "domain_dlq_max_level", metricType: Gauge},

		// per task list common metrics

		CadenceRequestsPerTaskList: {
			metricName: "cadence_requests_per_tl", metricRollupName: "cadence_requests", metricType: Counter,
		},
		CadenceFailuresPerTaskList: {
			metricName: "cadence_errors_per_tl", metricRollupName: "cadence_errors", metricType: Counter,
		},
		CadenceLatencyPerTaskList: {
			metricName: "cadence_latency_per_tl", metricRollupName: "cadence_latency", metricType: Timer,
		},
		CadenceErrBadRequestPerTaskListCounter: {
			metricName: "cadence_errors_bad_request_per_tl", metricRollupName: "cadence_errors_bad_request", metricType: Counter,
		},
		CadenceErrDomainNotActivePerTaskListCounter: {
			metricName: "cadence_errors_domain_not_active_per_tl", metricRollupName: "cadence_errors_domain_not_active", metricType: Counter,
		},
		CadenceErrServiceBusyPerTaskListCounter: {
			metricName: "cadence_errors_service_busy_per_tl", metricRollupName: "cadence_errors_service_busy", metricType: Counter,
		},
		CadenceErrEntityNotExistsPerTaskListCounter: {
			metricName: "cadence_errors_entity_not_exists_per_tl", metricRollupName: "cadence_errors_entity_not_exists", metricType: Counter,
		},
		CadenceErrExecutionAlreadyStartedPerTaskListCounter: {
			metricName: "cadence_errors_execution_already_started_per_tl", metricRollupName: "cadence_errors_execution_already_started", metricType: Counter,
		},
		CadenceErrDomainAlreadyExistsPerTaskListCounter: {
			metricName: "cadence_errors_domain_already_exists_per_tl", metricRollupName: "cadence_errors_domain_already_exists", metricType: Counter,
		},
		CadenceErrCancellationAlreadyRequestedPerTaskListCounter: {
			metricName: "cadence_errors_cancellation_already_requested_per_tl", metricRollupName: "cadence_errors_cancellation_already_requested", metricType: Counter,
		},
		CadenceErrQueryFailedPerTaskListCounter: {
			metricName: "cadence_errors_query_failed_per_tl", metricRollupName: "cadence_errors_query_failed", metricType: Counter,
		},
		CadenceErrLimitExceededPerTaskListCounter: {
			metricName: "cadence_errors_limit_exceeded_per_tl", metricRollupName: "cadence_errors_limit_exceeded", metricType: Counter,
		},
		CadenceErrContextTimeoutPerTaskListCounter: {
			metricName: "cadence_errors_context_timeout_per_tl", metricRollupName: "cadence_errors_context_timeout", metricType: Counter,
		},
		CadenceErrRetryTaskPerTaskListCounter: {
			metricName: "cadence_errors_retry_task_per_tl", metricRollupName: "cadence_errors_retry_task", metricType: Counter,
		},
		CadenceErrBadBinaryPerTaskListCounter: {
			metricName: "cadence_errors_bad_binary_per_tl", metricRollupName: "cadence_errors_bad_binary", metricType: Counter,
		},
		CadenceErrClientVersionNotSupportedPerTaskListCounter: {
			metricName: "cadence_errors_client_version_not_supported_per_tl", metricRollupName: "cadence_errors_client_version_not_supported", metricType: Counter,
		},
		CadenceErrIncompleteHistoryPerTaskListCounter: {
			metricName: "cadence_errors_incomplete_history_per_tl", metricRollupName: "cadence_errors_incomplete_history", metricType: Counter,
		},
		CadenceErrNonDeterministicPerTaskListCounter: {
			metricName: "cadence_errors_nondeterministic_per_tl", metricRollupName: "cadence_errors_nondeterministic", metricType: Counter,
		},
		CadenceErrUnauthorizedPerTaskListCounter: {
			metricName: "cadence_errors_unauthorized_per_tl", metricRollupName: "cadence_errors_unauthorized", metricType: Counter,
		},
		CadenceErrAuthorizeFailedPerTaskListCounter: {
			metricName: "cadence_errors_authorize_failed_per_tl", metricRollupName: "cadence_errors_authorize_failed", metricType: Counter,
		},
		CadenceErrRemoteSyncMatchFailedPerTaskListCounter: {
			metricName: "cadence_errors_remote_syncmatch_failed_per_tl", metricRollupName: "cadence_errors_remote_syncmatch_failed", metricType: Counter,
		},
		CadenceShardSuccessGauge: {metricName: "cadence_shard_success", metricType: Gauge},
		CadenceShardFailureGauge: {metricName: "cadence_shard_failure", metricType: Gauge},
	},
	History: {
		TaskRequests:                                      {metricName: "task_requests", metricType: Counter},
		TaskLatency:                                       {metricName: "task_latency", metricType: Timer},
		TaskAttemptTimer:                                  {metricName: "task_attempt", metricType: Timer},
		TaskFailures:                                      {metricName: "task_errors", metricType: Counter},
		TaskDiscarded:                                     {metricName: "task_errors_discarded", metricType: Counter},
		TaskStandbyRetryCounter:                           {metricName: "task_errors_standby_retry_counter", metricType: Counter},
		TaskNotActiveCounter:                              {metricName: "task_errors_not_active_counter", metricType: Counter},
		TaskLimitExceededCounter:                          {metricName: "task_errors_limit_exceeded_counter", metricType: Counter},
		TaskProcessingLatency:                             {metricName: "task_latency_processing", metricType: Timer},
		TaskQueueLatency:                                  {metricName: "task_latency_queue", metricType: Timer},
		TaskBatchCompleteCounter:                          {metricName: "task_batch_complete_counter", metricType: Counter},
		TaskRedispatchQueuePendingTasksTimer:              {metricName: "task_redispatch_queue_pending_tasks", metricType: Timer},
		TransferTaskThrottledCounter:                      {metricName: "transfer_task_throttled_counter", metricType: Counter},
		TimerTaskThrottledCounter:                         {metricName: "timer_task_throttled_counter", metricType: Counter},
		ActivityE2ELatency:                                {metricName: "activity_end_to_end_latency", metricType: Timer},
		AckLevelUpdateCounter:                             {metricName: "ack_level_update", metricType: Counter},
		AckLevelUpdateFailedCounter:                       {metricName: "ack_level_update_failed", metricType: Counter},
		DecisionTypeScheduleActivityCounter:               {metricName: "schedule_activity_decision", metricType: Counter},
		DecisionTypeCompleteWorkflowCounter:               {metricName: "complete_workflow_decision", metricType: Counter},
		DecisionTypeFailWorkflowCounter:                   {metricName: "fail_workflow_decision", metricType: Counter},
		DecisionTypeCancelWorkflowCounter:                 {metricName: "cancel_workflow_decision", metricType: Counter},
		DecisionTypeStartTimerCounter:                     {metricName: "start_timer_decision", metricType: Counter},
		DecisionTypeCancelActivityCounter:                 {metricName: "cancel_activity_decision", metricType: Counter},
		DecisionTypeCancelTimerCounter:                    {metricName: "cancel_timer_decision", metricType: Counter},
		DecisionTypeRecordMarkerCounter:                   {metricName: "record_marker_decision", metricType: Counter},
		DecisionTypeCancelExternalWorkflowCounter:         {metricName: "cancel_external_workflow_decision", metricType: Counter},
		DecisionTypeContinueAsNewCounter:                  {metricName: "continue_as_new_decision", metricType: Counter},
		DecisionTypeSignalExternalWorkflowCounter:         {metricName: "signal_external_workflow_decision", metricType: Counter},
		DecisionTypeUpsertWorkflowSearchAttributesCounter: {metricName: "upsert_workflow_search_attributes_decision", metricType: Counter},
		DecisionTypeChildWorkflowCounter:                  {metricName: "child_workflow_decision", metricType: Counter},
		EmptyCompletionDecisionsCounter:                   {metricName: "empty_completion_decisions", metricType: Counter},
		MultipleCompletionDecisionsCounter:                {metricName: "multiple_completion_decisions", metricType: Counter},
		FailedDecisionsCounter:                            {metricName: "failed_decisions", metricType: Counter},
		StaleMutableStateCounter:                          {metricName: "stale_mutable_state", metricType: Counter},
		AutoResetPointsLimitExceededCounter:               {metricName: "auto_reset_points_exceed_limit", metricType: Counter},
		AutoResetPointCorruptionCounter:                   {metricName: "auto_reset_point_corruption", metricType: Counter},
		ConcurrencyUpdateFailureCounter:                   {metricName: "concurrency_update_failure", metricType: Counter},
		CadenceErrShardOwnershipLostCounter:               {metricName: "cadence_errors_shard_ownership_lost", metricType: Counter},
		CadenceErrEventAlreadyStartedCounter:              {metricName: "cadence_errors_event_already_started", metricType: Counter},
		HeartbeatTimeoutCounter:                           {metricName: "heartbeat_timeout", metricType: Counter},
		ScheduleToStartTimeoutCounter:                     {metricName: "schedule_to_start_timeout", metricType: Counter},
		StartToCloseTimeoutCounter:                        {metricName: "start_to_close_timeout", metricType: Counter},
		ScheduleToCloseTimeoutCounter:                     {metricName: "schedule_to_close_timeout", metricType: Counter},
		NewTimerCounter:                                   {metricName: "new_timer", metricType: Counter},
		NewTimerNotifyCounter:                             {metricName: "new_timer_notifications", metricType: Counter},
		AcquireShardsCounter:                              {metricName: "acquire_shards_count", metricType: Counter},
		AcquireShardsLatency:                              {metricName: "acquire_shards_latency", metricType: Timer},
		ShardClosedCounter:                                {metricName: "shard_closed_count", metricType: Counter},
		ShardItemCreatedCounter:                           {metricName: "sharditem_created_count", metricType: Counter},
		ShardItemRemovedCounter:                           {metricName: "sharditem_removed_count", metricType: Counter},
		ShardItemAcquisitionLatency:                       {metricName: "sharditem_acquisition_latency", metricType: Timer},
		ShardInfoReplicationPendingTasksTimer:             {metricName: "shardinfo_replication_pending_task", metricType: Timer},
		ShardInfoTransferActivePendingTasksTimer:          {metricName: "shardinfo_transfer_active_pending_task", metricType: Timer},
		ShardInfoTransferStandbyPendingTasksTimer:         {metricName: "shardinfo_transfer_standby_pending_task", metricType: Timer},
		ShardInfoTimerActivePendingTasksTimer:             {metricName: "shardinfo_timer_active_pending_task", metricType: Timer},
		ShardInfoTimerStandbyPendingTasksTimer:            {metricName: "shardinfo_timer_standby_pending_task", metricType: Timer},
		ShardInfoReplicationLagTimer:                      {metricName: "shardinfo_replication_lag", metricType: Timer},
		ShardInfoTransferLagTimer:                         {metricName: "shardinfo_transfer_lag", metricType: Timer},
		ShardInfoTimerLagTimer:                            {metricName: "shardinfo_timer_lag", metricType: Timer},
		ShardInfoTransferDiffTimer:                        {metricName: "shardinfo_transfer_diff", metricType: Timer},
		ShardInfoTimerDiffTimer:                           {metricName: "shardinfo_timer_diff", metricType: Timer},
		ShardInfoTransferFailoverInProgressTimer:          {metricName: "shardinfo_transfer_failover_in_progress", metricType: Timer},
		ShardInfoTimerFailoverInProgressTimer:             {metricName: "shardinfo_timer_failover_in_progress", metricType: Timer},
		ShardInfoTransferFailoverLatencyTimer:             {metricName: "shardinfo_transfer_failover_latency", metricType: Timer},
		ShardInfoTimerFailoverLatencyTimer:                {metricName: "shardinfo_timer_failover_latency", metricType: Timer},
		SyncShardFromRemoteCounter:                        {metricName: "syncshard_remote_count", metricType: Counter},
		SyncShardFromRemoteFailure:                        {metricName: "syncshard_remote_failed", metricType: Counter},
		MembershipChangedCounter:                          {metricName: "membership_changed_count", metricType: Counter},
		NumShardsGauge:                                    {metricName: "numshards_gauge", metricType: Gauge},
		GetEngineForShardErrorCounter:                     {metricName: "get_engine_for_shard_errors", metricType: Counter},
		GetEngineForShardLatency:                          {metricName: "get_engine_for_shard_latency", metricType: Timer},
		RemoveEngineForShardLatency:                       {metricName: "remove_engine_for_shard_latency", metricType: Timer},
		CompleteDecisionWithStickyEnabledCounter:          {metricName: "complete_decision_sticky_enabled_count", metricType: Counter},
		CompleteDecisionWithStickyDisabledCounter:         {metricName: "complete_decision_sticky_disabled_count", metricType: Counter},
		DecisionHeartbeatTimeoutCounter:                   {metricName: "decision_heartbeat_timeout_count", metricType: Counter},
		HistoryEventNotificationQueueingLatency:           {metricName: "history_event_notification_queueing_latency", metricType: Timer},
		HistoryEventNotificationFanoutLatency:             {metricName: "history_event_notification_fanout_latency", metricType: Timer},
		HistoryEventNotificationInFlightMessageGauge:      {metricName: "history_event_notification_inflight_message_gauge", metricType: Gauge},
		HistoryEventNotificationFailDeliveryCount:         {metricName: "history_event_notification_fail_delivery_count", metricType: Counter},
		EmptyReplicationEventsCounter:                     {metricName: "empty_replication_events", metricType: Counter},
		DuplicateReplicationEventsCounter:                 {metricName: "duplicate_replication_events", metricType: Counter},
		StaleReplicationEventsCounter:                     {metricName: "stale_replication_events", metricType: Counter},
		ReplicationEventsSizeTimer:                        {metricName: "replication_events_size", metricType: Timer},
		BufferReplicationTaskTimer:                        {metricName: "buffer_replication_tasks", metricType: Timer},
		UnbufferReplicationTaskTimer:                      {metricName: "unbuffer_replication_tasks", metricType: Timer},
		HistoryConflictsCounter:                           {metricName: "history_conflicts", metricType: Counter},
		CompleteTaskFailedCounter:                         {metricName: "complete_task_fail_count", metricType: Counter},
		CacheRequests:                                     {metricName: "cache_requests", metricType: Counter},
		CacheFailures:                                     {metricName: "cache_errors", metricType: Counter},
		CacheLatency:                                      {metricName: "cache_latency", metricType: Timer},
		CacheMissCounter:                                  {metricName: "cache_miss", metricType: Counter},
		AcquireLockFailedCounter:                          {metricName: "acquire_lock_failed", metricType: Counter},
		WorkflowContextCleared:                            {metricName: "workflow_context_cleared", metricType: Counter},
		MutableStateSize:                                  {metricName: "mutable_state_size", metricType: Timer},
		ExecutionInfoSize:                                 {metricName: "execution_info_size", metricType: Timer},
		ActivityInfoSize:                                  {metricName: "activity_info_size", metricType: Timer},
		TimerInfoSize:                                     {metricName: "timer_info_size", metricType: Timer},
		ChildInfoSize:                                     {metricName: "child_info_size", metricType: Timer},
		SignalInfoSize:                                    {metricName: "signal_info", metricType: Timer},
		BufferedEventsSize:                                {metricName: "buffered_events_size", metricType: Timer},
		ActivityInfoCount:                                 {metricName: "activity_info_count", metricType: Timer},
		TimerInfoCount:                                    {metricName: "timer_info_count", metricType: Timer},
		ChildInfoCount:                                    {metricName: "child_info_count", metricType: Timer},
		SignalInfoCount:                                   {metricName: "signal_info_count", metricType: Timer},
		RequestCancelInfoCount:                            {metricName: "request_cancel_info_count", metricType: Timer},
		BufferedEventsCount:                               {metricName: "buffered_events_count", metricType: Timer},
		DeleteActivityInfoCount:                           {metricName: "delete_activity_info", metricType: Timer},
		DeleteTimerInfoCount:                              {metricName: "delete_timer_info", metricType: Timer},
		DeleteChildInfoCount:                              {metricName: "delete_child_info", metricType: Timer},
		DeleteSignalInfoCount:                             {metricName: "delete_signal_info", metricType: Timer},
		DeleteRequestCancelInfoCount:                      {metricName: "delete_request_cancel_info", metricType: Timer},
		WorkflowRetryBackoffTimerCount:                    {metricName: "workflow_retry_backoff_timer", metricType: Counter},
		WorkflowCronBackoffTimerCount:                     {metricName: "workflow_cron_backoff_timer", metricType: Counter},
		WorkflowCleanupDeleteCount:                        {metricName: "workflow_cleanup_delete", metricType: Counter},
		WorkflowCleanupArchiveCount:                       {metricName: "workflow_cleanup_archive", metricType: Counter},
		WorkflowCleanupNopCount:                           {metricName: "workflow_cleanup_nop", metricType: Counter},
		WorkflowCleanupDeleteHistoryInlineCount:           {metricName: "workflow_cleanup_delete_history_inline", metricType: Counter},
		WorkflowSuccessCount:                              {metricName: "workflow_success", metricType: Counter},
		WorkflowCancelCount:                               {metricName: "workflow_cancel", metricType: Counter},
		WorkflowFailedCount:                               {metricName: "workflow_failed", metricType: Counter},
		WorkflowTimeoutCount:                              {metricName: "workflow_timeout", metricType: Counter},
		WorkflowTerminateCount:                            {metricName: "workflow_terminate", metricType: Counter},
		ArchiverClientSendSignalCount:                     {metricName: "archiver_client_sent_signal", metricType: Counter},
		ArchiverClientSendSignalFailureCount:              {metricName: "archiver_client_send_signal_error", metricType: Counter},
		ArchiverClientHistoryRequestCount:                 {metricName: "archiver_client_history_request", metricType: Counter},
		ArchiverClientHistoryInlineArchiveAttemptCount:    {metricName: "archiver_client_history_inline_archive_attempt", metricType: Counter},
		ArchiverClientHistoryInlineArchiveFailureCount:    {metricName: "archiver_client_history_inline_archive_failure", metricType: Counter},
		ArchiverClientVisibilityRequestCount:              {metricName: "archiver_client_visibility_request", metricType: Counter},
		ArchiverClientVisibilityInlineArchiveAttemptCount: {metricName: "archiver_client_visibility_inline_archive_attempt", metricType: Counter},
		ArchiverClientVisibilityInlineArchiveFailureCount: {metricName: "archiver_client_visibility_inline_archive_failure", metricType: Counter},
		LastRetrievedMessageID:                            {metricName: "last_retrieved_message_id", metricType: Gauge},
		LastProcessedMessageID:                            {metricName: "last_processed_message_id", metricType: Gauge},
		ReplicationTasksApplied:                           {metricName: "replication_tasks_applied", metricType: Counter},
		ReplicationTasksFailed:                            {metricName: "replication_tasks_failed", metricType: Counter},
		ReplicationTasksLag:                               {metricName: "replication_tasks_lag", metricType: Timer},
		ReplicationTasksFetched:                           {metricName: "replication_tasks_fetched", metricType: Timer},
		ReplicationTasksReturned:                          {metricName: "replication_tasks_returned", metricType: Timer},
		ReplicationDLQFailed:                              {metricName: "replication_dlq_enqueue_failed", metricType: Counter},
		ReplicationDLQMaxLevelGauge:                       {metricName: "replication_dlq_max_level", metricType: Gauge},
		ReplicationDLQAckLevelGauge:                       {metricName: "replication_dlq_ack_level", metricType: Gauge},
		GetReplicationMessagesForShardLatency:             {metricName: "get_replication_messages_for_shard", metricType: Timer},
		GetDLQReplicationMessagesLatency:                  {metricName: "get_dlq_replication_messages", metricType: Timer},
		EventReapplySkippedCount:                          {metricName: "event_reapply_skipped_count", metricType: Counter},
		DirectQueryDispatchLatency:                        {metricName: "direct_query_dispatch_latency", metricType: Timer},
		DirectQueryDispatchStickyLatency:                  {metricName: "direct_query_dispatch_sticky_latency", metricType: Timer},
		DirectQueryDispatchNonStickyLatency:               {metricName: "direct_query_dispatch_non_sticky_latency", metricType: Timer},
		DirectQueryDispatchStickySuccessCount:             {metricName: "direct_query_dispatch_sticky_success", metricType: Counter},
		DirectQueryDispatchNonStickySuccessCount:          {metricName: "direct_query_dispatch_non_sticky_success", metricType: Counter},
		DirectQueryDispatchClearStickinessLatency:         {metricName: "direct_query_dispatch_clear_stickiness_latency", metricType: Timer},
		DirectQueryDispatchClearStickinessSuccessCount:    {metricName: "direct_query_dispatch_clear_stickiness_success", metricType: Counter},
		DirectQueryDispatchTimeoutBeforeNonStickyCount:    {metricName: "direct_query_dispatch_timeout_before_non_sticky", metricType: Counter},
		DecisionTaskQueryLatency:                          {metricName: "decision_task_query_latency", metricType: Timer},
		ConsistentQueryTimeoutCount:                       {metricName: "consistent_query_timeout", metricType: Counter},
		QueryBeforeFirstDecisionCount:                     {metricName: "query_before_first_decision", metricType: Counter},
		QueryBufferExceededCount:                          {metricName: "query_buffer_exceeded", metricType: Counter},
		QueryRegistryInvalidStateCount:                    {metricName: "query_registry_invalid_state", metricType: Counter},
		WorkerNotSupportsConsistentQueryCount:             {metricName: "worker_not_supports_consistent_query", metricType: Counter},
		DecisionStartToCloseTimeoutOverrideCount:          {metricName: "decision_start_to_close_timeout_overrides", metricType: Counter},
		ReplicationTaskCleanupCount:                       {metricName: "replication_task_cleanup_count", metricType: Counter},
		ReplicationTaskCleanupFailure:                     {metricName: "replication_task_cleanup_failed", metricType: Counter},
		MutableStateChecksumMismatch:                      {metricName: "mutable_state_checksum_mismatch", metricType: Counter},
		MutableStateChecksumInvalidated:                   {metricName: "mutable_state_checksum_invalidated", metricType: Counter},
	},
	Matching: {
		PollSuccessPerTaskListCounter:            {metricName: "poll_success_per_tl", metricRollupName: "poll_success"},
		PollTimeoutPerTaskListCounter:            {metricName: "poll_timeouts_per_tl", metricRollupName: "poll_timeouts"},
		PollSuccessWithSyncPerTaskListCounter:    {metricName: "poll_success_sync_per_tl", metricRollupName: "poll_success_sync"},
		LeaseRequestPerTaskListCounter:           {metricName: "lease_requests_per_tl", metricRollupName: "lease_requests"},
		LeaseFailurePerTaskListCounter:           {metricName: "lease_failures_per_tl", metricRollupName: "lease_failures"},
		ConditionFailedErrorPerTaskListCounter:   {metricName: "condition_failed_errors_per_tl", metricRollupName: "condition_failed_errors"},
		RespondQueryTaskFailedPerTaskListCounter: {metricName: "respond_query_failed_per_tl", metricRollupName: "respond_query_failed"},
		SyncThrottlePerTaskListCounter:           {metricName: "sync_throttle_count_per_tl", metricRollupName: "sync_throttle_count"},
		BufferThrottlePerTaskListCounter:         {metricName: "buffer_throttle_count_per_tl", metricRollupName: "buffer_throttle_count"},
		ExpiredTasksPerTaskListCounter:           {metricName: "tasks_expired_per_tl", metricRollupName: "tasks_expired"},
		ForwardedPerTaskListCounter:              {metricName: "forwarded_per_tl", metricRollupName: "forwarded"},
		ForwardTaskCallsPerTaskList:              {metricName: "forward_task_calls_per_tl", metricRollupName: "forward_task_calls"},
		ForwardTaskErrorsPerTaskList:             {metricName: "forward_task_errors_per_tl", metricRollupName: "forward_task_errors"},
		ForwardQueryCallsPerTaskList:             {metricName: "forward_query_calls_per_tl", metricRollupName: "forward_query_calls"},
		ForwardQueryErrorsPerTaskList:            {metricName: "forward_query_errors_per_tl", metricRollupName: "forward_query_errors"},
		ForwardPollCallsPerTaskList:              {metricName: "forward_poll_calls_per_tl", metricRollupName: "forward_poll_calls"},
		ForwardPollErrorsPerTaskList:             {metricName: "forward_poll_errors_per_tl", metricRollupName: "forward_poll_errors"},
		SyncMatchLatencyPerTaskList:              {metricName: "syncmatch_latency_per_tl", metricRollupName: "syncmatch_latency", metricType: Timer},
		AsyncMatchLatencyPerTaskList:             {metricName: "asyncmatch_latency_per_tl", metricRollupName: "asyncmatch_latency", metricType: Timer},
		ForwardTaskLatencyPerTaskList:            {metricName: "forward_task_latency_per_tl", metricRollupName: "forward_task_latency"},
		ForwardQueryLatencyPerTaskList:           {metricName: "forward_query_latency_per_tl", metricRollupName: "forward_query_latency"},
		ForwardPollLatencyPerTaskList:            {metricName: "forward_poll_latency_per_tl", metricRollupName: "forward_poll_latency"},
		LocalToLocalMatchPerTaskListCounter:      {metricName: "local_to_local_matches_per_tl", metricRollupName: "local_to_local_matches"},
		LocalToRemoteMatchPerTaskListCounter:     {metricName: "local_to_remote_matches_per_tl", metricRollupName: "local_to_remote_matches"},
		RemoteToLocalMatchPerTaskListCounter:     {metricName: "remote_to_local_matches_per_tl", metricRollupName: "remote_to_local_matches"},
		RemoteToRemoteMatchPerTaskListCounter:    {metricName: "remote_to_remote_matches_per_tl", metricRollupName: "remote_to_remote_matches"},
	},
	Worker: {
		ReplicatorMessages:                            {metricName: "replicator_messages"},
		ReplicatorFailures:                            {metricName: "replicator_errors"},
		ReplicatorMessagesDropped:                     {metricName: "replicator_messages_dropped"},
		ReplicatorLatency:                             {metricName: "replicator_latency"},
		ReplicatorDLQFailures:                         {metricName: "replicator_dlq_enqueue_fails", metricType: Counter},
		ESProcessorRequests:                           {metricName: "es_processor_requests"},
		ESProcessorRetries:                            {metricName: "es_processor_retries"},
		ESProcessorFailures:                           {metricName: "es_processor_errors"},
		ESProcessorCorruptedData:                      {metricName: "es_processor_corrupted_data"},
		ESProcessorProcessMsgLatency:                  {metricName: "es_processor_process_msg_latency", metricType: Timer},
		IndexProcessorCorruptedData:                   {metricName: "index_processor_corrupted_data"},
		IndexProcessorProcessMsgLatency:               {metricName: "index_processor_process_msg_latency", metricType: Timer},
		ArchiverNonRetryableErrorCount:                {metricName: "archiver_non_retryable_error"},
		ArchiverStartedCount:                          {metricName: "archiver_started"},
		ArchiverStoppedCount:                          {metricName: "archiver_stopped"},
		ArchiverCoroutineStartedCount:                 {metricName: "archiver_coroutine_started"},
		ArchiverCoroutineStoppedCount:                 {metricName: "archiver_coroutine_stopped"},
		ArchiverHandleHistoryRequestLatency:           {metricName: "archiver_handle_history_request_latency"},
		ArchiverHandleVisibilityRequestLatency:        {metricName: "archiver_handle_visibility_request_latency"},
		ArchiverUploadWithRetriesLatency:              {metricName: "archiver_upload_with_retries_latency"},
		ArchiverDeleteWithRetriesLatency:              {metricName: "archiver_delete_with_retries_latency"},
		ArchiverUploadFailedAllRetriesCount:           {metricName: "archiver_upload_failed_all_retries"},
		ArchiverUploadSuccessCount:                    {metricName: "archiver_upload_success"},
		ArchiverDeleteFailedAllRetriesCount:           {metricName: "archiver_delete_failed_all_retries"},
		ArchiverDeleteSuccessCount:                    {metricName: "archiver_delete_success"},
		ArchiverHandleVisibilityFailedAllRetiresCount: {metricName: "archiver_handle_visibility_failed_all_retries"},
		ArchiverHandleVisibilitySuccessCount:          {metricName: "archiver_handle_visibility_success"},
		ArchiverBacklogSizeGauge:                      {metricName: "archiver_backlog_size"},
		ArchiverPumpTimeoutCount:                      {metricName: "archiver_pump_timeout"},
		ArchiverPumpSignalThresholdCount:              {metricName: "archiver_pump_signal_threshold"},
		ArchiverPumpTimeoutWithoutSignalsCount:        {metricName: "archiver_pump_timeout_without_signals"},
		ArchiverPumpSignalChannelClosedCount:          {metricName: "archiver_pump_signal_channel_closed"},
		ArchiverWorkflowStartedCount:                  {metricName: "archiver_workflow_started"},
		ArchiverNumPumpedRequestsCount:                {metricName: "archiver_num_pumped_requests"},
		ArchiverNumHandledRequestsCount:               {metricName: "archiver_num_handled_requests"},
		ArchiverPumpedNotEqualHandledCount:            {metricName: "archiver_pumped_not_equal_handled"},
		ArchiverHandleAllRequestsLatency:              {metricName: "archiver_handle_all_requests_latency"},
		ArchiverWorkflowStoppingCount:                 {metricName: "archiver_workflow_stopping"},
		TaskProcessedCount:                            {metricName: "task_processed", metricType: Gauge},
		TaskDeletedCount:                              {metricName: "task_deleted", metricType: Gauge},
		TaskListProcessedCount:                        {metricName: "tasklist_processed", metricType: Gauge},
		TaskListDeletedCount:                          {metricName: "tasklist_deleted", metricType: Gauge},
		TaskListOutstandingCount:                      {metricName: "tasklist_outstanding", metricType: Gauge},
		ExecutionsOutstandingCount:                    {metricName: "executions_outstanding", metricType: Gauge},
		StartedCount:                                  {metricName: "started", metricType: Counter},
		StoppedCount:                                  {metricName: "stopped", metricType: Counter},
		ExecutorTasksDeferredCount:                    {metricName: "executor_deferred", metricType: Counter},
		ExecutorTasksDroppedCount:                     {metricName: "executor_dropped", metricType: Counter},
		BatcherProcessorSuccess:                       {metricName: "batcher_processor_requests", metricType: Counter},
		BatcherProcessorFailures:                      {metricName: "batcher_processor_errors", metricType: Counter},
		HistoryScavengerSuccessCount:                  {metricName: "scavenger_success", metricType: Counter},
		HistoryScavengerErrorCount:                    {metricName: "scavenger_errors", metricType: Counter},
		HistoryScavengerSkipCount:                     {metricName: "scavenger_skips", metricType: Counter},
		ParentClosePolicyProcessorSuccess:             {metricName: "parent_close_policy_processor_requests", metricType: Counter},
		ParentClosePolicyProcessorFailures:            {metricName: "parent_close_policy_processor_errors", metricType: Counter},
		DomainReplicationEnqueueDLQCount:              {metricName: "domain_replication_dlq_enqueue_requests", metricType: Counter},
		ScannerExecutionsGauge:                        {metricName: "scanner_executions", metricType: Gauge},
		ScannerCorruptedGauge:                         {metricName: "scanner_corrupted", metricType: Gauge},
		ScannerCheckFailedGauge:                       {metricName: "scanner_check_failed", metricType: Gauge},
		ScannerCorruptionByTypeGauge:                  {metricName: "scanner_corruption_by_type", metricType: Gauge},
		ScannerCorruptedOpenExecutionGauge:            {metricName: "scanner_corrupted_open_execution", metricType: Gauge},
		ScannerShardSizeMax:                           {metricName: "scanner_shard_size_max", metricType: Gauge},
		ScannerShardSizeMedian:                        {metricName: "scanner_shard_size_median", metricType: Gauge},
		ScannerShardSizeMin:                           {metricName: "scanner_shard_size_min", metricType: Gauge},
		ScannerShardSizeP90:                           {metricName: "scanner_shard_size_p90", metricType: Gauge},
		ScannerShardSizeP75:                           {metricName: "scanner_shard_size_p75", metricType: Gauge},
		ScannerShardSizeP25:                           {metricName: "scanner_shard_size_p25", metricType: Gauge},
		ScannerShardSizeP10:                           {metricName: "scanner_shard_size_max", metricType: Gauge},
	},
}

// ErrorClass is an enum to help with classifying SLA vs. non-SLA errors (SLA = "service level agreement")
type ErrorClass uint8

const (
	// NoError indicates that there is no error (error should be nil)
	NoError = ErrorClass(iota)
	// UserError indicates that this is NOT an SLA-reportable error
	UserError
	// InternalError indicates that this is an SLA-reportable error
	InternalError
)

// Empty returns true if the metricName is an empty string
func (mn MetricName) Empty() bool {
	return mn == ""
}

// String returns string representation of this metric name
func (mn MetricName) String() string {
	return string(mn)
}
