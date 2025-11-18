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

package constants

import (
	"time"

	"github.com/uber/cadence/.gen/go/shadower"
)

const (
	// FirstEventID is the id of the first event in the history
	FirstEventID int64 = 1
	// EmptyEventID is the id of the empty event
	EmptyEventID int64 = -23
	// EmptyVersion is used as the default value for failover version when no value is provided
	EmptyVersion int64 = -24
	// EndEventID is the id of the end event, here we use the int64 max
	EndEventID int64 = 1<<63 - 1
	// BufferedEventID is the id of the buffered event
	BufferedEventID int64 = -123
	// EmptyEventTaskID is uninitialized id of the task id within event
	EmptyEventTaskID int64 = -1234
	// TransientEventID is the id of the transient event
	TransientEventID int64 = -124
	// FirstBlobPageToken is the page token identifying the first blob for each history archival
	FirstBlobPageToken = 1
	// LastBlobNextPageToken is the next page token on the last blob for each history archival
	LastBlobNextPageToken = -1
	// InclusiveEndMessageID is the id of the end message, here we use the int64 max -1 because in some place we need to convert it to exclusive end message id
	InclusiveEndMessageID int64 = 1<<63 - 2
	// EmptyMessageID is the default start message ID for replication level
	EmptyMessageID = -1
	// InitialPreviousFailoverVersion is the initial previous failover version
	InitialPreviousFailoverVersion int64 = -1
)

const (
	// EmptyUUID is the placeholder for UUID when it's empty
	EmptyUUID = "emptyUuid"
)

// Data encoding types
const (
	EncodingTypeJSON           EncodingType = "json"
	EncodingTypeThriftRW       EncodingType = "thriftrw"
	EncodingTypeThriftRWSnappy EncodingType = "thriftrw_snappy"
	EncodingTypeGob            EncodingType = "gob"
	EncodingTypeUnknown        EncodingType = "unknow"
	EncodingTypeEmpty          EncodingType = ""
	EncodingTypeProto          EncodingType = "proto3"
)

type (
	// EncodingType is an enum that represents various data encoding types
	EncodingType string
)

// MaxTaskTimeout is maximum task timeout allowed. 366 days in seconds
const MaxTaskTimeout = 31622400

const (
	// GetHistoryMaxPageSize is the max page size for get history
	GetHistoryMaxPageSize = 1000
	// ReadDLQMessagesPageSize is the max page size for read DLQ messages
	ReadDLQMessagesPageSize = 1000
)

const (
	// VisibilityAppName is used to find kafka topics and ES indexName for visibility
	VisibilityAppName      = "visibility"
	PinotVisibilityAppName = "pinot-visibility"
)

const (
	// ESVisibilityStoreName is used to find es advanced visibility store
	ESVisibilityStoreName = "es-visibility"
	// PinotVisibilityStoreName is used to find pinot advanced visibility store
	PinotVisibilityStoreName = "pinot-visibility"
	// OSVisibilityStoreName is used to find opensearch advanced visibility store
	OSVisibilityStoreName = "os-visibility"
)

// This was flagged by salus as potentially hardcoded credentials. This is a false positive by the scanner and should be
// disregarded.
// #nosec
const (
	// SystemGlobalDomainName is global domain name for cadence system workflows running globally
	SystemGlobalDomainName = "cadence-system-global"
	// SystemDomainID is domain id for all cadence system workflows
	SystemDomainID = "32049b68-7872-4094-8e63-d0dd59896a83"
	// SystemLocalDomainName is domain name for cadence system workflows running in local cluster
	SystemLocalDomainName = "cadence-system"
	// SystemDomainRetentionDays is retention config for all cadence system workflows
	SystemDomainRetentionDays = 7
	// BatcherDomainID is domain id for batcher local domain
	BatcherDomainID = "3116607e-419b-4783-85fc-47726a4c3fe9"
	// BatcherLocalDomainName is domain name for batcher workflows running in local cluster
	// Batcher cannot use SystemLocalDomain because auth
	BatcherLocalDomainName = "cadence-batcher"
	// ShadowerDomainID is domain id for workflow shadower local domain
	ShadowerDomainID = "59c51119-1b41-4a28-986d-d6e377716f82"
	// ShadowerLocalDomainName
	ShadowerLocalDomainName = shadower.LocalDomainName
)

const (
	// MinLongPollTimeout is the minimum context timeout for long poll API, below which
	// the request won't be processed
	MinLongPollTimeout = time.Second * 2
	// CriticalLongPollTimeout is a threshold for the context timeout passed into long poll API,
	// below which a warning will be logged
	CriticalLongPollTimeout = time.Second * 20
)

const (
	// DefaultTransactionSizeLimit is the largest allowed transaction size to persistence
	DefaultTransactionSizeLimit = 14 * 1024 * 1024
)

const (
	// DefaultIDLengthWarnLimit is the warning length for various ID types
	DefaultIDLengthWarnLimit = 128
	// DefaultIDLengthErrorLimit is the maximum length allowed for various ID types
	DefaultIDLengthErrorLimit = 1000
)

const (
	// ArchivalEnabled is the status for enabling archival
	ArchivalEnabled = "enabled"
	// ArchivalDisabled is the status for disabling archival
	ArchivalDisabled = "disabled"
	// ArchivalPaused is the status for pausing archival
	ArchivalPaused = "paused"
)

// dynamic config AdvancedVisibility write/read mode
const (
	// AdvancedVisibilityModeOff means do not use advanced visibility store
	AdvancedVisibilityModeOff = "off"
	// AdvancedVisibilityModeES means ElasticSearch visibility mode
	VisibilityModeES = "es"
	// AdvancedVisibilityModePinot means Pinot visibility mode
	VisibilityModePinot = "pinot"
	// AdvancedVisibilityModeOS means OpenSearch visibility mode
	VisibilityModeOS = "os"
	// AdvancedVisibilityModeDB means db visibility mode
	VisibilityModeDB = "db"
)

const (
	ESPersistenceName    = "elasticsearch"
	PinotPersistenceName = "pinot"
	DBPersistenceName    = "db"
)

const (
	// DomainDataKeyForManagedFailover is key of DomainData for managed failover
	DomainDataKeyForManagedFailover = "IsManagedByCadence"
	// DomainDataKeyForPreferredCluster is the key of DomainData for domain rebalance
	DomainDataKeyForPreferredCluster = "PreferredCluster"
	// DomainDataKeyForFailoverHistory is the key of DomainData for failover history
	DomainDataKeyForFailoverHistory = "FailoverHistory"
	// DomainDataKeyForReadGroups stores which groups have read permission of the domain API
	DomainDataKeyForReadGroups = "READ_GROUPS"
	// DomainDataKeyForWriteGroups stores which groups have write permission of the domain API
	DomainDataKeyForWriteGroups = "WRITE_GROUPS"
	// DomainDataKeyForProcessGroups stores which groups have process permission of the domain API
	DomainDataKeyForProcessGroups = "PROCESS_GROUPS"
)

type (
	// TaskType is the enum for representing different task types
	TaskType int
)

const (
	// TaskTypeTransfer is the task type for transfer task
	// starting from 2 here to be consistent with the row type define for cassandra
	// TODO: we can remove +2 from the following definition
	// we don't have to make them consistent with cassandra definition
	// there's also no row type for sql or other nosql persistence implementation
	TaskTypeTransfer TaskType = iota + 2
	// TaskTypeTimer is the task type for timer task
	TaskTypeTimer
	// TaskTypeReplication is the task type for replication task
	TaskTypeReplication
	// Deprecated: TaskTypeCrossCluster is the task type for cross cluster task
	// as of June 2024, this feature is no longer supported. Keeping the enum here
	// to avoid future reuse of the ID and/or confusion
	TaskTypeCrossCluster TaskType = 6
)

const (
	// DefaultESAnalyzerPause controls if we want to dynamically pause the analyzer
	DefaultESAnalyzerPause = false
	// DefaultESAnalyzerTimeWindow controls how many days to go back for ElasticSearch Analyzer
	DefaultESAnalyzerTimeWindow = time.Hour * 24 * 30
	// DefaultESAnalyzerMaxNumDomains controls how many domains to check
	DefaultESAnalyzerMaxNumDomains = 500
	// DefaultESAnalyzerMaxNumWorkflowTypes controls how many workflow types per domain to check
	DefaultESAnalyzerMaxNumWorkflowTypes = 100
	// DefaultESAnalyzerNumWorkflowsToRefresh controls how many workflows per workflow type should be refreshed
	DefaultESAnalyzerNumWorkflowsToRefresh = 100
	// DefaultESAnalyzerBufferWaitTime controls min time required to consider a worklow stuck
	DefaultESAnalyzerBufferWaitTime = time.Minute * 30
	// DefaultESAnalyzerMinNumWorkflowsForAvg controls how many workflows to have at least to rely on workflow run time avg per type
	DefaultESAnalyzerMinNumWorkflowsForAvg = 100
	// DefaultESAnalyzerLimitToTypes controls if we want to limit ESAnalyzer only to some workflow types
	DefaultESAnalyzerLimitToTypes = ""
	// DefaultESAnalyzerEnableAvgDurationBasedChecks controls if we want to enable avg duration based refreshes
	DefaultESAnalyzerEnableAvgDurationBasedChecks = false
	// DefaultESAnalyzerLimitToDomains controls if we want to limit ESAnalyzer only to some domains
	DefaultESAnalyzerLimitToDomains = ""
	// DefaultESAnalyzerWorkflowDurationWarnThreshold defines warning threshold for a workflow duration
	DefaultESAnalyzerWorkflowDurationWarnThresholds = ""
)

// StickyTaskConditionFailedErrorMsg error msg for sticky task ConditionFailedError
const StickyTaskConditionFailedErrorMsg = "StickyTaskConditionFailedError"

// MemoKeyForOperator is the memo key for operator
const MemoKeyForOperator = "operator"

// ReservedTaskListPrefix is the required naming prefix for any task list partition other than partition 0
const ReservedTaskListPrefix = "/__cadence_sys/"

type (
	// VisibilityOperation is an enum that represents visibility message types
	VisibilityOperation string
)

// Enum for visibility message type
const (
	RecordStarted          VisibilityOperation = "RecordStarted"
	RecordClosed           VisibilityOperation = "RecordClosed"
	UpsertSearchAttributes VisibilityOperation = "UpsertSearchAttributes"
)

const (
	numBitsPerLevel = 3
)

const (
	// HighPriorityClass is the priority class for high priority tasks
	HighPriorityClass = iota << numBitsPerLevel
	// DefaultPriorityClass is the priority class for default priority tasks
	DefaultPriorityClass
	// LowPriorityClass is the priority class for low priority tasks
	LowPriorityClass
)

const (
	// HighPrioritySubclass is the priority subclass for high priority tasks
	HighPrioritySubclass = iota
	// DefaultPrioritySubclass is the priority subclass for high priority tasks
	DefaultPrioritySubclass
	// LowPrioritySubclass is the priority subclass for high priority tasks
	LowPrioritySubclass
)

const (
	// DefaultHistoryMaxAutoResetPoints is the default maximum number for auto reset points
	DefaultHistoryMaxAutoResetPoints = 20
)

const (
	// WorkflowIDRateLimitReason is the reason set in ServiceBusyError when workflow ID rate limit is exceeded
	WorkflowIDRateLimitReason = "external-workflow-id-rate-limit"
)

type (
	// FailoverType is the enum for representing different failover types
	FailoverType int
)

const (
	FailoverTypeForce FailoverType = iota + 1
	FailoverTypeGrace
)

func (v FailoverType) String() string {
	switch v {
	case FailoverTypeForce:
		return "Force"
	case FailoverTypeGrace:
		return "Grace"
	default:
		return "Unknown"
	}
}

const (
	StringSizeOverheadBytes = 16
)

// GetTaskPriority returns priority given a task's priority class and subclass
func GetTaskPriority(
	class int,
	subClass int,
) int {
	return class | subClass
}

// GRPCConnectionClosingError is the error message returned when a gRPC client connection is closing
const GRPCConnectionClosingError = "grpc: the client connection is closing"
