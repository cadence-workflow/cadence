// Copyright (c) 2021 Uber Technologies Inc.
//
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

package testdata

import (
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

const (
	WorkflowID = "WorkflowID"
	RunID      = "RunID"
	RunID1     = "RunID1"
	RunID2     = "RunID2"
	RunID3     = "RunID3"

	ActivityID = "ActivityID"
	RequestID  = "RequestID"
	TimerID    = "TimerID"

	WorkflowTypeName     = "WorkflowTypeName"
	ActivityTypeName     = "ActivityTypeName"
	TaskListName         = "TaskListName"
	MarkerName           = "MarkerName"
	SignalName           = "SignalName"
	QueryType            = "QueryType"
	HostName             = "HostName"
	HostName2            = "HostName2"
	Identity             = "Identity"
	CronSchedule         = "CronSchedule"
	Checksum             = "Checksum"
	Reason               = "Reason"
	Cause                = "Cause"
	SecurityToken        = "SecurityToken"
	VisibilityQuery      = "VisibilityQuery"
	FeatureVersion       = "FeatureVersion"
	ClientImpl           = "ClientImpl"
	ClientLibraryVersion = "ClientLibraryVersion"
	SupportedVersions    = "SupportedVersions"
	FeatureFlag          = "FeatureFlag"
	Namespace            = "Namespace"

	Attempt            = 2
	ScheduleID         = 5
	PageSize           = 10
	HistoryLength      = 20
	BacklogCountHint   = 30
	AckLevel           = 1001
	ReadLevel          = 1002
	RatePerSecond      = 3.14
	TaskID             = 444
	ShardID            = 12345
	MessageID1         = 50001
	MessageID2         = 50002
	EventStoreVersion  = 333
	HistorySizeInBytes = 77779

	EventID1 = int64(1)
	EventID2 = int64(2)
	EventID3 = int64(3)
	EventID4 = int64(4)

	Version1 = int64(11)
	Version2 = int64(22)
	Version3 = int64(33)

	IsolationGroup = "dca1"
)

var (
	Timestamp  = time.Now().Unix()
	Timestamp1 = Timestamp + 1
	Timestamp2 = Timestamp + 2
	Timestamp3 = Timestamp + 3
	Timestamp4 = Timestamp + 4
	Timestamp5 = Timestamp + 5
)

var (
	Duration1 = int32(11)
	Duration2 = int32(12)
	Duration3 = int32(13)
	Duration4 = int32(14)
)

var (
	Token1 = []byte{1, 0}
	Token2 = []byte{2, 0}
	Token3 = []byte{3, 0}
)

var (
	Payload1 = []byte{10, 0}
	Payload2 = []byte{20, 0}
	Payload3 = []byte{30, 0}
)

var Attempt2 = int64(2)

var (
	ExecutionContext = []byte{110, 0}
	Control          = []byte{120, 0}
	NextPageToken    = []byte{130, 0}
	TaskToken        = []byte{140, 0}
	BranchToken      = []byte{150, 0}
	QueryArgs        = []byte{9, 9, 9}
)

var (
	FailureReason  = "FailureReason"
	FailureDetails = []byte{190, 0}
)

var PartitionConfig = map[string]string{
	"zone": IsolationGroup,
}

var (
	HealthStatus = types.HealthStatus{
		Ok:  true,
		Msg: "HealthStatusMessage",
	}
	WorkflowExecution = types.WorkflowExecution{
		WorkflowID: WorkflowID,
		RunID:      RunID,
	}
	WorkflowType = types.WorkflowType{
		Name: WorkflowTypeName,
	}
	ActivityType = types.ActivityType{
		Name: ActivityTypeName,
	}
	TaskList = types.TaskList{
		Name: TaskListName,
		Kind: types.TaskListKindNormal.Ptr(),
	}
	RetryPolicy = types.RetryPolicy{
		InitialIntervalInSeconds:    1,
		BackoffCoefficient:          1.1,
		MaximumIntervalInSeconds:    2,
		MaximumAttempts:             3,
		NonRetriableErrorReasons:    []string{"a", "b"},
		ExpirationIntervalInSeconds: 4,
	}
	Header = types.Header{
		Fields: map[string][]byte{
			"HeaderField1": {211, 0},
			"HeaderField2": {212, 0},
		},
	}
	Memo = types.Memo{
		Fields: map[string][]byte{
			"MemoField1": {221, 0},
			"MemoField2": {222, 0},
		},
	}
	SearchAttributes = types.SearchAttributes{
		IndexedFields: map[string][]byte{
			"IndexedField1": {231, 0},
			"IndexedField2": {232, 0},
		},
	}
	PayloadMap = map[string][]byte{
		"Payload1": Payload1,
		"Payload2": Payload2,
	}
	ResetPointInfo = types.ResetPointInfo{
		BinaryChecksum:           Checksum,
		RunID:                    RunID1,
		FirstDecisionCompletedID: EventID1,
		CreatedTimeNano:          &Timestamp1,
		ExpiringTimeNano:         &Timestamp2,
		Resettable:               true,
	}
	ResetPointInfoArray = []*types.ResetPointInfo{
		&ResetPointInfo,
	}
	ResetPoints = types.ResetPoints{
		Points: ResetPointInfoArray,
	}
	DataBlob = types.DataBlob{
		EncodingType: &EncodingType,
		Data:         []byte{7, 7, 7},
	}
	DataBlobArray = []*types.DataBlob{
		&DataBlob,
	}
	StickyExecutionAttributes = types.StickyExecutionAttributes{
		WorkerTaskList:                &TaskList,
		ScheduleToStartTimeoutSeconds: &Duration1,
	}
	ActivityLocalDispatchInfo = types.ActivityLocalDispatchInfo{
		ActivityID:                      ActivityID,
		ScheduledTimestamp:              &Timestamp1,
		StartedTimestamp:                &Timestamp2,
		ScheduledTimestampOfThisAttempt: &Timestamp3,
		TaskToken:                       TaskToken,
	}
	ActivityLocalDispatchInfoMap = map[string]*types.ActivityLocalDispatchInfo{
		ActivityID: &ActivityLocalDispatchInfo,
	}
	TaskListMetadata = types.TaskListMetadata{
		MaxTasksPerSecond: common.Float64Ptr(RatePerSecond),
	}
	WorkerVersionInfo = types.WorkerVersionInfo{
		Impl:           ClientImpl,
		FeatureVersion: FeatureVersion,
	}
	PollerInfo = types.PollerInfo{
		LastAccessTime: &Timestamp1,
		Identity:       Identity,
		RatePerSecond:  RatePerSecond,
	}
	PollerInfoArray = []*types.PollerInfo{
		&PollerInfo,
	}
	TaskListStatus = types.TaskListStatus{
		BacklogCountHint: BacklogCountHint,
		ReadLevel:        ReadLevel,
		AckLevel:         AckLevel,
		RatePerSecond:    RatePerSecond,
		TaskIDBlock:      &TaskIDBlock,
		IsolationGroupMetrics: map[string]*types.IsolationGroupMetrics{
			"dca": {
				NewTasksPerSecond: 10,
				PollerCount:       1,
			},
		},
		NewTasksPerSecond: 10,
	}
	TaskIDBlock = types.TaskIDBlock{
		StartID: 551,
		EndID:   559,
	}
	TaskListPartitionMetadata = types.TaskListPartitionMetadata{
		Key:           "Key",
		OwnerHostName: "HostName",
	}
	TaskListPartitionMetadataArray = []*types.TaskListPartitionMetadata{
		&TaskListPartitionMetadata,
	}
	SupportedClientVersions = types.SupportedClientVersions{
		GoSdk:   "GoSDK",
		JavaSdk: "JavaSDK",
	}
	IndexedValueTypeMap = map[string]types.IndexedValueType{
		"IndexedValueType1": IndexedValueType,
	}
	IsolationGroupConfiguration = types.IsolationGroupConfiguration{
		IsolationGroup: {
			Name:  IsolationGroup,
			State: types.IsolationGroupStateHealthy,
		},
		"zone 1": {
			Name:  "zone 1",
			State: types.IsolationGroupStateDrained,
		},
	}
	ParentExecutionInfo = types.ParentExecutionInfo{
		DomainUUID:  DomainID,
		Domain:      DomainName,
		Execution:   &WorkflowExecution,
		InitiatedID: EventID1,
	}
	ClusterInfo = types.ClusterInfo{
		SupportedClientVersions: &SupportedClientVersions,
	}
	MembershipInfo = types.MembershipInfo{
		CurrentHost:      &HostInfo,
		ReachableMembers: []string{HostName},
		Rings:            RingInfoArray,
	}
	HostInfo = types.HostInfo{
		Identity: HostName,
	}
	HostInfoArray = []*types.HostInfo{
		&HostInfo,
	}
	RingInfo = types.RingInfo{
		Role:        "Role",
		MemberCount: 1,
		Members:     HostInfoArray,
	}
	RingInfoArray = []*types.RingInfo{
		&RingInfo,
	}
	DomainCacheInfo = types.DomainCacheInfo{
		NumOfItemsInCacheByID:   int64(2000),
		NumOfItemsInCacheByName: int64(3000),
	}
	VersionHistoryItem = types.VersionHistoryItem{
		EventID: EventID1,
		Version: Version1,
	}
	VersionHistoryItemArray = []*types.VersionHistoryItem{
		&VersionHistoryItem,
	}
	VersionHistory = types.VersionHistory{
		BranchToken: BranchToken,
		Items:       VersionHistoryItemArray,
	}
	VersionHistoryArray = []*types.VersionHistory{
		&VersionHistory,
	}
	VersionHistories = types.VersionHistories{
		CurrentVersionHistoryIndex: 1,
		Histories:                  VersionHistoryArray,
	}
	TransientDecisionInfo = types.TransientDecisionInfo{
		ScheduledEvent: &HistoryEvent_WorkflowExecutionStarted,
		StartedEvent:   &HistoryEvent_WorkflowExecutionStarted,
	}
	FailoverMarkerToken = types.FailoverMarkerToken{
		ShardIDs:       []int32{ShardID},
		FailoverMarker: &FailoverMarkerAttributes,
	}
	FailoverMarkerTokenArray = []*types.FailoverMarkerToken{
		&FailoverMarkerToken,
	}
	WorkflowExecutionFilter = types.WorkflowExecutionFilter{
		WorkflowID: WorkflowID,
		RunID:      RunID,
	}
	WorkflowTypeFilter = types.WorkflowTypeFilter{
		Name: WorkflowTypeName,
	}
	StartTimeFilter = types.StartTimeFilter{
		EarliestTime: &Timestamp1,
		LatestTime:   &Timestamp2,
	}
	WorkflowExecutionInfo = types.WorkflowExecutionInfo{
		Execution:         &WorkflowExecution,
		Type:              &WorkflowType,
		StartTime:         &Timestamp1,
		CloseTime:         &Timestamp2,
		CloseStatus:       &WorkflowExecutionCloseStatus,
		HistoryLength:     HistoryLength,
		ParentDomainID:    common.StringPtr(DomainID),
		ParentDomain:      common.StringPtr(DomainName),
		ParentExecution:   &WorkflowExecution,
		ParentInitiatedID: common.Int64Ptr(EventID1),
		ExecutionTime:     &Timestamp3,
		Memo:              &Memo,
		SearchAttributes:  &SearchAttributes,
		AutoResetPoints:   &ResetPoints,
		TaskList:          TaskListName,
		PartitionConfig:   PartitionConfig,
	}
	CronWorkflowExecutionInfo = types.WorkflowExecutionInfo{
		Execution:         &WorkflowExecution,
		Type:              &WorkflowType,
		StartTime:         &Timestamp1,
		CloseTime:         &Timestamp2,
		CloseStatus:       &WorkflowExecutionCloseStatus,
		HistoryLength:     HistoryLength,
		ParentDomainID:    common.StringPtr(DomainID),
		ParentDomain:      common.StringPtr(DomainName),
		ParentExecution:   &WorkflowExecution,
		ParentInitiatedID: common.Int64Ptr(EventID1),
		ExecutionTime:     &Timestamp3,
		Memo:              &Memo,
		SearchAttributes:  &SearchAttributes,
		AutoResetPoints:   &ResetPoints,
		TaskList:          TaskListName,
		PartitionConfig:   PartitionConfig,
		IsCron:            true,
	}
	WorkflowExecutionInfoArray = []*types.WorkflowExecutionInfo{&WorkflowExecutionInfo}

	WorkflowQuery = types.WorkflowQuery{
		QueryType: QueryType,
		QueryArgs: QueryArgs,
	}
	WorkflowQueryMap = map[string]*types.WorkflowQuery{
		"WorkflowQuery1": &WorkflowQuery,
	}
	WorkflowQueryResult = types.WorkflowQueryResult{
		ResultType:   &QueryResultType,
		Answer:       Payload1,
		ErrorMessage: ErrorMessage,
	}
	WorkflowQueryResultMap = map[string]*types.WorkflowQueryResult{
		"WorkflowQuery1": &WorkflowQueryResult,
	}
	QueryRejected = types.QueryRejected{
		CloseStatus: &WorkflowExecutionCloseStatus,
	}
	WorkflowExecutionConfiguration = types.WorkflowExecutionConfiguration{
		TaskList:                            &TaskList,
		ExecutionStartToCloseTimeoutSeconds: &Duration1,
		TaskStartToCloseTimeoutSeconds:      &Duration2,
	}
	PendingActivityInfo = types.PendingActivityInfo{
		ActivityID:             ActivityID,
		ActivityType:           &ActivityType,
		State:                  &PendingActivityState,
		HeartbeatDetails:       Payload1,
		LastHeartbeatTimestamp: &Timestamp1,
		LastStartedTimestamp:   &Timestamp2,
		Attempt:                Attempt,
		MaximumAttempts:        3,
		ScheduledTimestamp:     &Timestamp3,
		ExpirationTimestamp:    &Timestamp4,
		LastFailureReason:      &FailureReason,
		LastWorkerIdentity:     Identity,
		LastFailureDetails:     FailureDetails,
		StartedWorkerIdentity:  Identity,
		ScheduleID:             ScheduleID,
	}
	PendingActivityInfoArray = []*types.PendingActivityInfo{
		&PendingActivityInfo,
	}
	PendingChildExecutionInfo = types.PendingChildExecutionInfo{
		Domain:            DomainName,
		WorkflowID:        WorkflowID,
		RunID:             RunID,
		WorkflowTypeName:  WorkflowTypeName,
		InitiatedID:       EventID1,
		ParentClosePolicy: &ParentClosePolicy,
	}
	PendingChildExecutionInfoArray = []*types.PendingChildExecutionInfo{
		&PendingChildExecutionInfo,
	}
	PendingDecisionInfo = types.PendingDecisionInfo{
		State:                      &PendingDecisionState,
		ScheduledTimestamp:         &Timestamp1,
		StartedTimestamp:           &Timestamp2,
		Attempt:                    Attempt,
		OriginalScheduledTimestamp: &Timestamp3,
		ScheduleID:                 ScheduleID,
	}
	AutoConfigHint = types.AutoConfigHint{
		EnableAutoConfig:   false,
		PollerWaitTimeInMs: 10,
	}
	TaskKey = types.TaskKey{
		TaskID:            TaskID,
		ScheduledTimeNano: Timestamp1,
	}
	TaskKey2 = types.TaskKey{
		TaskID:            TaskID + 1,
		ScheduledTimeNano: Timestamp2,
	}
	TaskRange = types.TaskRange{
		InclusiveMin: &TaskKey,
		ExclusiveMax: &TaskKey2,
	}
	VirtualSliceState = types.VirtualSliceState{
		TaskRange: &TaskRange,
	}
	VirtualQueueState = types.VirtualQueueState{
		VirtualSliceStates: []*types.VirtualSliceState{&VirtualSliceState},
	}
	QueueState = types.QueueState{
		VirtualQueueStates: map[int64]*types.VirtualQueueState{
			0: &VirtualQueueState,
		},
		ExclusiveMaxReadLevel: &TaskKey,
	}
)
