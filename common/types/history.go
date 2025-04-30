// Copyright (c) 2017-2020 Uber Technologies Inc.
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

package types

// DescribeMutableStateRequest is an internal type (TBD...)
type DescribeMutableStateRequest struct {
	DomainUUID string             `json:"domainUUID,omitempty"`
	Execution  *WorkflowExecution `json:"execution,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *DescribeMutableStateRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// DescribeMutableStateResponse is an internal type (TBD...)
type DescribeMutableStateResponse struct {
	MutableStateInCache    string `json:"mutableStateInCache,omitempty"`
	MutableStateInDatabase string `json:"mutableStateInDatabase,omitempty"`
}

// HistoryDescribeWorkflowExecutionRequest is an internal type (TBD...)
type HistoryDescribeWorkflowExecutionRequest struct {
	DomainUUID string                            `json:"domainUUID,omitempty"`
	Request    *DescribeWorkflowExecutionRequest `json:"request,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistoryDescribeWorkflowExecutionRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// DomainFilter is an internal type (TBD...)
type DomainFilter struct {
	DomainIDs    []string `json:"domainIDs,omitempty"`
	ReverseMatch bool     `json:"reverseMatch,omitempty"`
}

// GetDomainIDs is an internal getter (TBD...)
func (v *DomainFilter) GetDomainIDs() (o []string) {
	if v != nil && v.DomainIDs != nil {
		return v.DomainIDs
	}
	return
}

// GetReverseMatch is an internal getter (TBD...)
func (v *DomainFilter) GetReverseMatch() (o bool) {
	if v != nil {
		return v.ReverseMatch
	}
	return
}

// FailoverMarkerToken is an internal type (TBD...)
type FailoverMarkerToken struct {
	ShardIDs       []int32                   `json:"shardIDs,omitempty"`
	FailoverMarker *FailoverMarkerAttributes `json:"failoverMarker,omitempty"`
}

// GetShardIDs is an internal getter (TBD...)
func (v *FailoverMarkerToken) GetShardIDs() (o []int32) {
	if v != nil && v.ShardIDs != nil {
		return v.ShardIDs
	}
	return
}

// GetFailoverMarker is an internal getter (TBD...)
func (v *FailoverMarkerToken) GetFailoverMarker() (o *FailoverMarkerAttributes) {
	if v != nil && v.FailoverMarker != nil {
		return v.FailoverMarker
	}
	return
}

// GetMutableStateRequest is an internal type (TBD...)
type GetMutableStateRequest struct {
	DomainUUID          string              `json:"domainUUID,omitempty"`
	Execution           *WorkflowExecution  `json:"execution,omitempty"`
	ExpectedNextEventID int64               `json:"expectedNextEventId,omitempty"`
	CurrentBranchToken  []byte              `json:"currentBranchToken,omitempty"`
	VersionHistoryItem  *VersionHistoryItem `json:"versionHistoryItem,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *GetMutableStateRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// GetExpectedNextEventID is an internal getter (TBD...)
func (v *GetMutableStateRequest) GetExpectedNextEventID() (o int64) {
	if v != nil {
		return v.ExpectedNextEventID
	}
	return
}

// GetMutableStateResponse is an internal type (TBD...)
type GetMutableStateResponse struct {
	Execution                            *WorkflowExecution `json:"execution,omitempty"`
	WorkflowType                         *WorkflowType      `json:"workflowType,omitempty"`
	NextEventID                          int64              `json:"NextEventId,omitempty"`
	PreviousStartedEventID               *int64             `json:"PreviousStartedEventId,omitempty"`
	LastFirstEventID                     int64              `json:"LastFirstEventId,omitempty"`
	TaskList                             *TaskList          `json:"taskList,omitempty"`
	StickyTaskList                       *TaskList          `json:"stickyTaskList,omitempty"`
	ClientLibraryVersion                 string             `json:"clientLibraryVersion,omitempty"`
	ClientFeatureVersion                 string             `json:"clientFeatureVersion,omitempty"`
	ClientImpl                           string             `json:"clientImpl,omitempty"`
	IsWorkflowRunning                    bool               `json:"isWorkflowRunning,omitempty"`
	StickyTaskListScheduleToStartTimeout *int32             `json:"stickyTaskListScheduleToStartTimeout,omitempty"`
	EventStoreVersion                    int32              `json:"eventStoreVersion,omitempty"`
	CurrentBranchToken                   []byte             `json:"currentBranchToken,omitempty"`
	WorkflowState                        *int32             `json:"workflowState,omitempty"`
	WorkflowCloseState                   *int32             `json:"workflowCloseState,omitempty"`
	VersionHistories                     *VersionHistories  `json:"versionHistories,omitempty"`
	IsStickyTaskListEnabled              bool               `json:"isStickyTaskListEnabled,omitempty"`
	HistorySize                          int64              `json:"historySize,omitempty"`
}

// GetNextEventID is an internal getter (TBD...)
func (v *GetMutableStateResponse) GetNextEventID() (o int64) {
	if v != nil {
		return v.NextEventID
	}
	return
}

// GetPreviousStartedEventID is an internal getter (TBD...)
func (v *GetMutableStateResponse) GetPreviousStartedEventID() (o int64) {
	if v != nil && v.PreviousStartedEventID != nil {
		return *v.PreviousStartedEventID
	}
	return
}

// GetStickyTaskList is an internal getter (TBD...)
func (v *GetMutableStateResponse) GetStickyTaskList() (o *TaskList) {
	if v != nil && v.StickyTaskList != nil {
		return v.StickyTaskList
	}
	return
}

// GetClientFeatureVersion is an internal getter (TBD...)
func (v *GetMutableStateResponse) GetClientFeatureVersion() (o string) {
	if v != nil {
		return v.ClientFeatureVersion
	}
	return
}

// GetClientImpl is an internal getter (TBD...)
func (v *GetMutableStateResponse) GetClientImpl() (o string) {
	if v != nil {
		return v.ClientImpl
	}
	return
}

// GetIsWorkflowRunning is an internal getter (TBD...)
func (v *GetMutableStateResponse) GetIsWorkflowRunning() (o bool) {
	if v != nil {
		return v.IsWorkflowRunning
	}
	return
}

// GetStickyTaskListScheduleToStartTimeout is an internal getter (TBD...)
func (v *GetMutableStateResponse) GetStickyTaskListScheduleToStartTimeout() (o int32) {
	if v != nil && v.StickyTaskListScheduleToStartTimeout != nil {
		return *v.StickyTaskListScheduleToStartTimeout
	}
	return
}

// GetCurrentBranchToken is an internal getter (TBD...)
func (v *GetMutableStateResponse) GetCurrentBranchToken() (o []byte) {
	if v != nil && v.CurrentBranchToken != nil {
		return v.CurrentBranchToken
	}
	return
}

// GetWorkflowCloseState is an internal getter (TBD...)
func (v *GetMutableStateResponse) GetWorkflowCloseState() (o int32) {
	if v != nil && v.WorkflowCloseState != nil {
		return *v.WorkflowCloseState
	}
	return
}

// GetVersionHistories is an internal getter (TBD...)
func (v *GetMutableStateResponse) GetVersionHistories() (o *VersionHistories) {
	if v != nil && v.VersionHistories != nil {
		return v.VersionHistories
	}
	return
}

// GetIsStickyTaskListEnabled is an internal getter (TBD...)
func (v *GetMutableStateResponse) GetIsStickyTaskListEnabled() (o bool) {
	if v != nil {
		return v.IsStickyTaskListEnabled
	}
	return
}

// NotifyFailoverMarkersRequest is an internal type (TBD...)
type NotifyFailoverMarkersRequest struct {
	FailoverMarkerTokens []*FailoverMarkerToken `json:"failoverMarkerTokens,omitempty"`
}

// GetFailoverMarkerTokens is an internal getter (TBD...)
func (v *NotifyFailoverMarkersRequest) GetFailoverMarkerTokens() (o []*FailoverMarkerToken) {
	if v != nil && v.FailoverMarkerTokens != nil {
		return v.FailoverMarkerTokens
	}
	return
}

// ParentExecutionInfo is an internal type (TBD...)
type ParentExecutionInfo struct {
	DomainUUID  string             `json:"domainUUID,omitempty"`
	Domain      string             `json:"domain,omitempty"`
	Execution   *WorkflowExecution `json:"execution,omitempty"`
	InitiatedID int64              `json:"initiatedId,omitempty"`
}

// GetDomain is an internal getter (TBD...)
func (v *ParentExecutionInfo) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

// GetExecution is an internal getter (TBD...)
func (v *ParentExecutionInfo) GetExecution() (o *WorkflowExecution) {
	if v != nil && v.Execution != nil {
		return v.Execution
	}
	return
}

// PollMutableStateRequest is an internal type (TBD...)
type PollMutableStateRequest struct {
	DomainUUID          string              `json:"domainUUID,omitempty"`
	Execution           *WorkflowExecution  `json:"execution,omitempty"`
	ExpectedNextEventID int64               `json:"expectedNextEventId,omitempty"`
	CurrentBranchToken  []byte              `json:"currentBranchToken,omitempty"`
	VersionHistoryItem  *VersionHistoryItem `json:"versionHistoryItem,omitempty"`
}

func (p *PollMutableStateRequest) GetVersionHistoryItem() *VersionHistoryItem {
	if p.VersionHistoryItem == nil {
		return nil
	}
	return p.VersionHistoryItem
}

// GetDomainUUID is an internal getter (TBD...)
func (p *PollMutableStateRequest) GetDomainUUID() (o string) {
	if p != nil {
		return p.DomainUUID
	}
	return
}

// PollMutableStateResponse is an internal type (TBD...)
type PollMutableStateResponse struct {
	Execution                            *WorkflowExecution `json:"execution,omitempty"`
	WorkflowType                         *WorkflowType      `json:"workflowType,omitempty"`
	NextEventID                          int64              `json:"NextEventId,omitempty"`
	PreviousStartedEventID               *int64             `json:"PreviousStartedEventId,omitempty"`
	LastFirstEventID                     int64              `json:"LastFirstEventId,omitempty"`
	TaskList                             *TaskList          `json:"taskList,omitempty"`
	StickyTaskList                       *TaskList          `json:"stickyTaskList,omitempty"`
	ClientLibraryVersion                 string             `json:"clientLibraryVersion,omitempty"`
	ClientFeatureVersion                 string             `json:"clientFeatureVersion,omitempty"`
	ClientImpl                           string             `json:"clientImpl,omitempty"`
	StickyTaskListScheduleToStartTimeout *int32             `json:"stickyTaskListScheduleToStartTimeout,omitempty"`
	CurrentBranchToken                   []byte             `json:"currentBranchToken,omitempty"`
	VersionHistories                     *VersionHistories  `json:"versionHistories,omitempty"`
	WorkflowState                        *int32             `json:"workflowState,omitempty"`
	WorkflowCloseState                   *int32             `json:"workflowCloseState,omitempty"`
}

// GetNextEventID is an internal getter (TBD...)
func (v *PollMutableStateResponse) GetNextEventID() (o int64) {
	if v != nil {
		return v.NextEventID
	}
	return
}

// GetLastFirstEventID is an internal getter (TBD...)
func (v *PollMutableStateResponse) GetLastFirstEventID() (o int64) {
	if v != nil {
		return v.LastFirstEventID
	}
	return
}

// GetWorkflowCloseState is an internal getter (TBD...)
func (v *PollMutableStateResponse) GetWorkflowCloseState() (o int32) {
	if v != nil && v.WorkflowCloseState != nil {
		return *v.WorkflowCloseState
	}
	return
}

type QueueState struct {
	VirtualQueueStates    map[int64]*VirtualQueueState
	ExclusiveMaxReadLevel *TaskKey
}

func (q *QueueState) Copy() (o *QueueState) {
	if q == nil {
		return
	}
	var virtualQueueStates map[int64]*VirtualQueueState
	if q.VirtualQueueStates != nil {
		virtualQueueStates = make(map[int64]*VirtualQueueState)
		for key, value := range q.VirtualQueueStates {
			virtualQueueStates[key] = value.Copy()
		}
	}
	o = &QueueState{
		VirtualQueueStates:    virtualQueueStates,
		ExclusiveMaxReadLevel: q.ExclusiveMaxReadLevel.Copy(),
	}
	return
}

type VirtualQueueState struct {
	VirtualSliceStates []*VirtualSliceState
}

func (v *VirtualQueueState) Copy() (o *VirtualQueueState) {
	if v == nil {
		return
	}
	var virtualSliceStates []*VirtualSliceState
	if v.VirtualSliceStates != nil {
		virtualSliceStates = make([]*VirtualSliceState, len(v.VirtualSliceStates))
		for i, slice := range v.VirtualSliceStates {
			virtualSliceStates[i] = slice.Copy()
		}
	}
	o = &VirtualQueueState{
		VirtualSliceStates: virtualSliceStates,
	}
	return
}

type VirtualSliceState struct {
	TaskRange *TaskRange
}

func (v *VirtualSliceState) Copy() (o *VirtualSliceState) {
	if v == nil {
		return
	}
	o = &VirtualSliceState{
		TaskRange: v.TaskRange.Copy(),
	}
	return
}

type TaskRange struct {
	InclusiveMin *TaskKey
	ExclusiveMax *TaskKey
}

func (t *TaskRange) Copy() (o *TaskRange) {
	if t == nil {
		return
	}
	o = &TaskRange{
		InclusiveMin: t.InclusiveMin.Copy(),
		ExclusiveMax: t.ExclusiveMax.Copy(),
	}
	return
}

type TaskKey struct {
	ScheduledTimeNano int64
	TaskID            int64
}

func (t *TaskKey) Copy() (o *TaskKey) {
	if t == nil {
		return
	}
	o = &TaskKey{
		ScheduledTimeNano: t.ScheduledTimeNano,
		TaskID:            t.TaskID,
	}
	return
}

// ProcessingQueueState is an internal type (TBD...)
type ProcessingQueueState struct {
	Level        *int32        `json:"level,omitempty"`
	AckLevel     *int64        `json:"ackLevel,omitempty"`
	MaxLevel     *int64        `json:"maxLevel,omitempty"`
	DomainFilter *DomainFilter `json:"domainFilter,omitempty"`
}

// GetLevel is an internal getter (TBD...)
func (v *ProcessingQueueState) GetLevel() (o int32) {
	if v != nil && v.Level != nil {
		return *v.Level
	}
	return
}

// GetAckLevel is an internal getter (TBD...)
func (v *ProcessingQueueState) GetAckLevel() (o int64) {
	if v != nil && v.AckLevel != nil {
		return *v.AckLevel
	}
	return
}

// GetMaxLevel is an internal getter (TBD...)
func (v *ProcessingQueueState) GetMaxLevel() (o int64) {
	if v != nil && v.MaxLevel != nil {
		return *v.MaxLevel
	}
	return
}

// GetDomainFilter is an internal getter (TBD...)
func (v *ProcessingQueueState) GetDomainFilter() (o *DomainFilter) {
	if v != nil && v.DomainFilter != nil {
		return v.DomainFilter
	}
	return
}

// ProcessingQueueStates is an internal type (TBD...)
type ProcessingQueueStates struct {
	StatesByCluster map[string][]*ProcessingQueueState `json:"statesByCluster,omitempty"`
}

// HistoryQueryWorkflowRequest is an internal type (TBD...)
type HistoryQueryWorkflowRequest struct {
	DomainUUID string                `json:"domainUUID,omitempty"`
	Request    *QueryWorkflowRequest `json:"request,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistoryQueryWorkflowRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// GetRequest is an internal getter (TBD...)
func (v *HistoryQueryWorkflowRequest) GetRequest() (o *QueryWorkflowRequest) {
	if v != nil && v.Request != nil {
		return v.Request
	}
	return
}

// HistoryQueryWorkflowResponse is an internal type (TBD...)
type HistoryQueryWorkflowResponse struct {
	Response *QueryWorkflowResponse `json:"response,omitempty"`
}

// GetResponse is an internal getter (TBD...)
func (v *HistoryQueryWorkflowResponse) GetResponse() (o *QueryWorkflowResponse) {
	if v != nil && v.Response != nil {
		return v.Response
	}
	return
}

// HistoryReapplyEventsRequest is an internal type (TBD...)
type HistoryReapplyEventsRequest struct {
	DomainUUID string                `json:"domainUUID,omitempty"`
	Request    *ReapplyEventsRequest `json:"request,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistoryReapplyEventsRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// GetRequest is an internal getter (TBD...)
func (v *HistoryReapplyEventsRequest) GetRequest() (o *ReapplyEventsRequest) {
	if v != nil && v.Request != nil {
		return v.Request
	}
	return
}

// HistoryRecordActivityTaskHeartbeatRequest is an internal type (TBD...)
type HistoryRecordActivityTaskHeartbeatRequest struct {
	DomainUUID       string                              `json:"domainUUID,omitempty"`
	HeartbeatRequest *RecordActivityTaskHeartbeatRequest `json:"heartbeatRequest,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistoryRecordActivityTaskHeartbeatRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// RecordActivityTaskStartedRequest is an internal type (TBD...)
type RecordActivityTaskStartedRequest struct {
	DomainUUID        string                      `json:"domainUUID,omitempty"`
	WorkflowExecution *WorkflowExecution          `json:"workflowExecution,omitempty"`
	ScheduleID        int64                       `json:"scheduleId,omitempty"`
	TaskID            int64                       `json:"taskId,omitempty"`
	RequestID         string                      `json:"requestId,omitempty"`
	PollRequest       *PollForActivityTaskRequest `json:"pollRequest,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *RecordActivityTaskStartedRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// GetScheduleID is an internal getter (TBD...)
func (v *RecordActivityTaskStartedRequest) GetScheduleID() (o int64) {
	if v != nil {
		return v.ScheduleID
	}
	return
}

// GetTaskID is an internal getter (TBD...)
func (v *RecordActivityTaskStartedRequest) GetTaskID() (o int64) {
	if v != nil {
		return v.TaskID
	}
	return
}

// GetRequestID is an internal getter (TBD...)
func (v *RecordActivityTaskStartedRequest) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// RecordActivityTaskStartedResponse is an internal type (TBD...)
type RecordActivityTaskStartedResponse struct {
	ScheduledEvent                  *HistoryEvent `json:"scheduledEvent,omitempty"`
	StartedTimestamp                *int64        `json:"startedTimestamp,omitempty"`
	Attempt                         int64         `json:"attempt,omitempty"`
	ScheduledTimestampOfThisAttempt *int64        `json:"scheduledTimestampOfThisAttempt,omitempty"`
	HeartbeatDetails                []byte        `json:"heartbeatDetails,omitempty"`
	WorkflowType                    *WorkflowType `json:"workflowType,omitempty"`
	WorkflowDomain                  string        `json:"workflowDomain,omitempty"`
}

// GetAttempt is an internal getter (TBD...)
func (v *RecordActivityTaskStartedResponse) GetAttempt() (o int64) {
	if v != nil {
		return v.Attempt
	}
	return
}

// GetScheduledTimestampOfThisAttempt is an internal getter (TBD...)
func (v *RecordActivityTaskStartedResponse) GetScheduledTimestampOfThisAttempt() (o int64) {
	if v != nil && v.ScheduledTimestampOfThisAttempt != nil {
		return *v.ScheduledTimestampOfThisAttempt
	}
	return
}

// RecordChildExecutionCompletedRequest is an internal type (TBD...)
type RecordChildExecutionCompletedRequest struct {
	DomainUUID         string             `json:"domainUUID,omitempty"`
	WorkflowExecution  *WorkflowExecution `json:"workflowExecution,omitempty"`
	InitiatedID        int64              `json:"initiatedId,omitempty"`
	CompletedExecution *WorkflowExecution `json:"completedExecution,omitempty"`
	CompletionEvent    *HistoryEvent      `json:"completionEvent,omitempty"`
	StartedID          int64              `json:"startedId,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *RecordChildExecutionCompletedRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// RecordDecisionTaskStartedRequest is an internal type (TBD...)
type RecordDecisionTaskStartedRequest struct {
	DomainUUID        string                      `json:"domainUUID,omitempty"`
	WorkflowExecution *WorkflowExecution          `json:"workflowExecution,omitempty"`
	ScheduleID        int64                       `json:"scheduleId,omitempty"`
	TaskID            int64                       `json:"taskId,omitempty"`
	RequestID         string                      `json:"requestId,omitempty"`
	PollRequest       *PollForDecisionTaskRequest `json:"pollRequest,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *RecordDecisionTaskStartedRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// GetScheduleID is an internal getter (TBD...)
func (v *RecordDecisionTaskStartedRequest) GetScheduleID() (o int64) {
	if v != nil {
		return v.ScheduleID
	}
	return
}

// GetRequestID is an internal getter (TBD...)
func (v *RecordDecisionTaskStartedRequest) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// RecordDecisionTaskStartedResponse is an internal type (TBD...)
type RecordDecisionTaskStartedResponse struct {
	WorkflowType              *WorkflowType             `json:"workflowType,omitempty"`
	PreviousStartedEventID    *int64                    `json:"previousStartedEventId,omitempty"`
	ScheduledEventID          int64                     `json:"scheduledEventId,omitempty"`
	StartedEventID            int64                     `json:"startedEventId,omitempty"`
	NextEventID               int64                     `json:"nextEventId,omitempty"`
	Attempt                   int64                     `json:"attempt,omitempty"`
	StickyExecutionEnabled    bool                      `json:"stickyExecutionEnabled,omitempty"`
	DecisionInfo              *TransientDecisionInfo    `json:"decisionInfo,omitempty"`
	WorkflowExecutionTaskList *TaskList                 `json:"WorkflowExecutionTaskList,omitempty"`
	EventStoreVersion         int32                     `json:"eventStoreVersion,omitempty"`
	BranchToken               []byte                    `json:"branchToken,omitempty"`
	ScheduledTimestamp        *int64                    `json:"scheduledTimestamp,omitempty"`
	StartedTimestamp          *int64                    `json:"startedTimestamp,omitempty"`
	Queries                   map[string]*WorkflowQuery `json:"queries,omitempty"`
	HistorySize               int64                     `json:"historySize,omitempty"`
}

// GetPreviousStartedEventID is an internal getter (TBD...)
func (v *RecordDecisionTaskStartedResponse) GetPreviousStartedEventID() (o int64) {
	if v != nil && v.PreviousStartedEventID != nil {
		return *v.PreviousStartedEventID
	}
	return
}

// GetScheduledEventID is an internal getter (TBD...)
func (v *RecordDecisionTaskStartedResponse) GetScheduledEventID() (o int64) {
	if v != nil {
		return v.ScheduledEventID
	}
	return
}

// GetAttempt is an internal getter (TBD...)
func (v *RecordDecisionTaskStartedResponse) GetAttempt() (o int64) {
	if v != nil {
		return v.Attempt
	}
	return
}

// HistoryRefreshWorkflowTasksRequest is an internal type (TBD...)
type HistoryRefreshWorkflowTasksRequest struct {
	DomainUIID string                       `json:"domainUIID,omitempty"`
	Request    *RefreshWorkflowTasksRequest `json:"request,omitempty"`
}

// GetRequest is an internal getter (TBD...)
func (v *HistoryRefreshWorkflowTasksRequest) GetRequest() (o *RefreshWorkflowTasksRequest) {
	if v != nil && v.Request != nil {
		return v.Request
	}
	return
}

// RemoveSignalMutableStateRequest is an internal type (TBD...)
type RemoveSignalMutableStateRequest struct {
	DomainUUID        string             `json:"domainUUID,omitempty"`
	WorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
	RequestID         string             `json:"requestId,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *RemoveSignalMutableStateRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// GetRequestID is an internal getter (TBD...)
func (v *RemoveSignalMutableStateRequest) GetRequestID() (o string) {
	if v != nil {
		return v.RequestID
	}
	return
}

// ReplicateEventsV2Request is an internal type (TBD...)
type ReplicateEventsV2Request struct {
	DomainUUID          string                `json:"domainUUID,omitempty"`
	WorkflowExecution   *WorkflowExecution    `json:"workflowExecution,omitempty"`
	VersionHistoryItems []*VersionHistoryItem `json:"versionHistoryItems,omitempty"`
	Events              *DataBlob             `json:"events,omitempty"`
	NewRunEvents        *DataBlob             `json:"newRunEvents,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *ReplicateEventsV2Request) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// HistoryRequestCancelWorkflowExecutionRequest is an internal type (TBD...)
type HistoryRequestCancelWorkflowExecutionRequest struct {
	DomainUUID                string                                 `json:"domainUUID,omitempty"`
	CancelRequest             *RequestCancelWorkflowExecutionRequest `json:"cancelRequest,omitempty"`
	ExternalInitiatedEventID  *int64                                 `json:"externalInitiatedEventId,omitempty"`
	ExternalWorkflowExecution *WorkflowExecution                     `json:"externalWorkflowExecution,omitempty"`
	ChildWorkflowOnly         bool                                   `json:"childWorkflowOnly,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistoryRequestCancelWorkflowExecutionRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// GetCancelRequest is an internal getter (TBD...)
func (v *HistoryRequestCancelWorkflowExecutionRequest) GetCancelRequest() (o *RequestCancelWorkflowExecutionRequest) {
	if v != nil && v.CancelRequest != nil {
		return v.CancelRequest
	}
	return
}

// GetExternalWorkflowExecution is an internal getter (TBD...)
func (v *HistoryRequestCancelWorkflowExecutionRequest) GetExternalWorkflowExecution() (o *WorkflowExecution) {
	if v != nil && v.ExternalWorkflowExecution != nil {
		return v.ExternalWorkflowExecution
	}
	return
}

// GetChildWorkflowOnly is an internal getter (TBD...)
func (v *HistoryRequestCancelWorkflowExecutionRequest) GetChildWorkflowOnly() (o bool) {
	if v != nil {
		return v.ChildWorkflowOnly
	}
	return
}

// HistoryResetStickyTaskListRequest is an internal type (TBD...)
type HistoryResetStickyTaskListRequest struct {
	DomainUUID string             `json:"domainUUID,omitempty"`
	Execution  *WorkflowExecution `json:"execution,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistoryResetStickyTaskListRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// HistoryResetStickyTaskListResponse is an internal type (TBD...)
type HistoryResetStickyTaskListResponse struct {
}

// HistoryResetWorkflowExecutionRequest is an internal type (TBD...)
type HistoryResetWorkflowExecutionRequest struct {
	DomainUUID   string                         `json:"domainUUID,omitempty"`
	ResetRequest *ResetWorkflowExecutionRequest `json:"resetRequest,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistoryResetWorkflowExecutionRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// HistoryRespondActivityTaskCanceledRequest is an internal type (TBD...)
type HistoryRespondActivityTaskCanceledRequest struct {
	DomainUUID    string                              `json:"domainUUID,omitempty"`
	CancelRequest *RespondActivityTaskCanceledRequest `json:"cancelRequest,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistoryRespondActivityTaskCanceledRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// HistoryRespondActivityTaskCompletedRequest is an internal type (TBD...)
type HistoryRespondActivityTaskCompletedRequest struct {
	DomainUUID      string                               `json:"domainUUID,omitempty"`
	CompleteRequest *RespondActivityTaskCompletedRequest `json:"completeRequest,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistoryRespondActivityTaskCompletedRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// HistoryRespondActivityTaskFailedRequest is an internal type (TBD...)
type HistoryRespondActivityTaskFailedRequest struct {
	DomainUUID    string                            `json:"domainUUID,omitempty"`
	FailedRequest *RespondActivityTaskFailedRequest `json:"failedRequest,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistoryRespondActivityTaskFailedRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// HistoryRespondDecisionTaskCompletedRequest is an internal type (TBD...)
type HistoryRespondDecisionTaskCompletedRequest struct {
	DomainUUID      string                               `json:"domainUUID,omitempty"`
	CompleteRequest *RespondDecisionTaskCompletedRequest `json:"completeRequest,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistoryRespondDecisionTaskCompletedRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// GetCompleteRequest is an internal getter (TBD...)
func (v *HistoryRespondDecisionTaskCompletedRequest) GetCompleteRequest() (o *RespondDecisionTaskCompletedRequest) {
	if v != nil && v.CompleteRequest != nil {
		return v.CompleteRequest
	}
	return
}

// HistoryRespondDecisionTaskCompletedResponse is an internal type (TBD...)
type HistoryRespondDecisionTaskCompletedResponse struct {
	StartedResponse             *RecordDecisionTaskStartedResponse    `json:"startedResponse,omitempty"`
	ActivitiesToDispatchLocally map[string]*ActivityLocalDispatchInfo `json:"activitiesToDispatchLocally,omitempty"`
}

// HistoryRespondDecisionTaskFailedRequest is an internal type (TBD...)
type HistoryRespondDecisionTaskFailedRequest struct {
	DomainUUID    string                            `json:"domainUUID,omitempty"`
	FailedRequest *RespondDecisionTaskFailedRequest `json:"failedRequest,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistoryRespondDecisionTaskFailedRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// ScheduleDecisionTaskRequest is an internal type (TBD...)
type ScheduleDecisionTaskRequest struct {
	DomainUUID        string             `json:"domainUUID,omitempty"`
	WorkflowExecution *WorkflowExecution `json:"workflowExecution,omitempty"`
	IsFirstDecision   bool               `json:"isFirstDecision,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *ScheduleDecisionTaskRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// ShardOwnershipLostError is an internal type (TBD...)
type ShardOwnershipLostError struct {
	Message string `json:"message,omitempty"`
	Owner   string `json:"owner,omitempty"`
}

// GetOwner is an internal getter (TBD...)
func (v *ShardOwnershipLostError) GetOwner() (o string) {
	if v != nil {
		return v.Owner
	}
	return
}

// HistorySignalWithStartWorkflowExecutionRequest is an internal type (TBD...)
type HistorySignalWithStartWorkflowExecutionRequest struct {
	DomainUUID             string                                   `json:"domainUUID,omitempty"`
	SignalWithStartRequest *SignalWithStartWorkflowExecutionRequest `json:"signalWithStartRequest,omitempty"`
	PartitionConfig        map[string]string
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistorySignalWithStartWorkflowExecutionRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// GetPartitionConfig is an internal getter (TBD...)
func (v *HistorySignalWithStartWorkflowExecutionRequest) GetPartitionConfig() (o map[string]string) {
	if v != nil && v.PartitionConfig != nil {
		return v.PartitionConfig
	}
	return
}

// HistorySignalWorkflowExecutionRequest is an internal type (TBD...)
type HistorySignalWorkflowExecutionRequest struct {
	DomainUUID                string                          `json:"domainUUID,omitempty"`
	SignalRequest             *SignalWorkflowExecutionRequest `json:"signalRequest,omitempty"`
	ExternalWorkflowExecution *WorkflowExecution              `json:"externalWorkflowExecution,omitempty"`
	ChildWorkflowOnly         bool                            `json:"childWorkflowOnly,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistorySignalWorkflowExecutionRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// GetChildWorkflowOnly is an internal getter (TBD...)
func (v *HistorySignalWorkflowExecutionRequest) GetChildWorkflowOnly() (o bool) {
	if v != nil {
		return v.ChildWorkflowOnly
	}
	return
}

// HistoryStartWorkflowExecutionRequest is an internal type (TBD...)
type HistoryStartWorkflowExecutionRequest struct {
	DomainUUID                      string                         `json:"domainUUID,omitempty"`
	StartRequest                    *StartWorkflowExecutionRequest `json:"startRequest,omitempty"`
	ParentExecutionInfo             *ParentExecutionInfo           `json:"parentExecutionInfo,omitempty"`
	Attempt                         int32                          `json:"attempt,omitempty"`
	ExpirationTimestamp             *int64                         `json:"expirationTimestamp,omitempty"`
	ContinueAsNewInitiator          *ContinueAsNewInitiator        `json:"continueAsNewInitiator,omitempty"`
	ContinuedFailureReason          *string                        `json:"continuedFailureReason,omitempty"`
	ContinuedFailureDetails         []byte                         `json:"continuedFailureDetails,omitempty"`
	LastCompletionResult            []byte                         `json:"lastCompletionResult,omitempty"`
	FirstDecisionTaskBackoffSeconds *int32                         `json:"firstDecisionTaskBackoffSeconds,omitempty"`
	PartitionConfig                 map[string]string
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistoryStartWorkflowExecutionRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// GetAttempt is an internal getter (TBD...)
func (v *HistoryStartWorkflowExecutionRequest) GetAttempt() (o int32) {
	if v != nil {
		return v.Attempt
	}
	return
}

// GetExpirationTimestamp is an internal getter (TBD...)
func (v *HistoryStartWorkflowExecutionRequest) GetExpirationTimestamp() (o int64) {
	if v != nil && v.ExpirationTimestamp != nil {
		return *v.ExpirationTimestamp
	}
	return
}

// GetFirstDecisionTaskBackoffSeconds is an internal getter (TBD...)
func (v *HistoryStartWorkflowExecutionRequest) GetFirstDecisionTaskBackoffSeconds() (o int32) {
	if v != nil && v.FirstDecisionTaskBackoffSeconds != nil {
		return *v.FirstDecisionTaskBackoffSeconds
	}
	return
}

// GetPartitionConfig is an internal getter (TBD...)
func (v *HistoryStartWorkflowExecutionRequest) GetPartitionConfig() (o map[string]string) {
	if v != nil && v.PartitionConfig != nil {
		return v.PartitionConfig
	}
	return
}

// SyncActivityRequest is an internal type (TBD...)
type SyncActivityRequest struct {
	DomainID           string          `json:"domainId,omitempty"`
	WorkflowID         string          `json:"workflowId,omitempty"`
	RunID              string          `json:"runId,omitempty"`
	Version            int64           `json:"version,omitempty"`
	ScheduledID        int64           `json:"scheduledId,omitempty"`
	ScheduledTime      *int64          `json:"scheduledTime,omitempty"`
	StartedID          int64           `json:"startedId,omitempty"`
	StartedTime        *int64          `json:"startedTime,omitempty"`
	LastHeartbeatTime  *int64          `json:"lastHeartbeatTime,omitempty"`
	Details            []byte          `json:"details,omitempty"`
	Attempt            int32           `json:"attempt,omitempty"`
	LastFailureReason  *string         `json:"lastFailureReason,omitempty"`
	LastWorkerIdentity string          `json:"lastWorkerIdentity,omitempty"`
	LastFailureDetails []byte          `json:"lastFailureDetails,omitempty"`
	VersionHistory     *VersionHistory `json:"versionHistory,omitempty"`
}

// GetDomainID is an internal getter (TBD...)
func (v *SyncActivityRequest) GetDomainID() (o string) {
	if v != nil {
		return v.DomainID
	}
	return
}

// GetWorkflowID is an internal getter (TBD...)
func (v *SyncActivityRequest) GetWorkflowID() (o string) {
	if v != nil {
		return v.WorkflowID
	}
	return
}

// GetRunID is an internal getter (TBD...)
func (v *SyncActivityRequest) GetRunID() (o string) {
	if v != nil {
		return v.RunID
	}
	return
}

// GetVersion is an internal getter (TBD...)
func (v *SyncActivityRequest) GetVersion() (o int64) {
	if v != nil {
		return v.Version
	}
	return
}

// GetScheduledID is an internal getter (TBD...)
func (v *SyncActivityRequest) GetScheduledID() (o int64) {
	if v != nil {
		return v.ScheduledID
	}
	return
}

// GetScheduledTime is an internal getter (TBD...)
func (v *SyncActivityRequest) GetScheduledTime() (o int64) {
	if v != nil && v.ScheduledTime != nil {
		return *v.ScheduledTime
	}
	return
}

// GetStartedID is an internal getter (TBD...)
func (v *SyncActivityRequest) GetStartedID() (o int64) {
	if v != nil {
		return v.StartedID
	}
	return
}

// GetStartedTime is an internal getter (TBD...)
func (v *SyncActivityRequest) GetStartedTime() (o int64) {
	if v != nil && v.StartedTime != nil {
		return *v.StartedTime
	}
	return
}

// GetLastHeartbeatTime is an internal getter (TBD...)
func (v *SyncActivityRequest) GetLastHeartbeatTime() (o int64) {
	if v != nil && v.LastHeartbeatTime != nil {
		return *v.LastHeartbeatTime
	}
	return
}

// GetDetails is an internal getter (TBD...)
func (v *SyncActivityRequest) GetDetails() (o []byte) {
	if v != nil && v.Details != nil {
		return v.Details
	}
	return
}

// GetAttempt is an internal getter (TBD...)
func (v *SyncActivityRequest) GetAttempt() (o int32) {
	if v != nil {
		return v.Attempt
	}
	return
}

// GetLastFailureReason is an internal getter (TBD...)
func (v *SyncActivityRequest) GetLastFailureReason() (o string) {
	if v != nil && v.LastFailureReason != nil {
		return *v.LastFailureReason
	}
	return
}

// GetLastWorkerIdentity is an internal getter (TBD...)
func (v *SyncActivityRequest) GetLastWorkerIdentity() (o string) {
	if v != nil {
		return v.LastWorkerIdentity
	}
	return
}

// GetLastFailureDetails is an internal getter (TBD...)
func (v *SyncActivityRequest) GetLastFailureDetails() (o []byte) {
	if v != nil && v.LastFailureDetails != nil {
		return v.LastFailureDetails
	}
	return
}

// GetVersionHistory is an internal getter (TBD...)
func (v *SyncActivityRequest) GetVersionHistory() (o *VersionHistory) {
	if v != nil && v.VersionHistory != nil {
		return v.VersionHistory
	}
	return
}

// SyncShardStatusRequest is an internal type (TBD...)
type SyncShardStatusRequest struct {
	SourceCluster string `json:"sourceCluster,omitempty"`
	ShardID       int64  `json:"shardId,omitempty"`
	Timestamp     *int64 `json:"timestamp,omitempty"`
}

// GetSourceCluster is an internal getter (TBD...)
func (v *SyncShardStatusRequest) GetSourceCluster() (o string) {
	if v != nil {
		return v.SourceCluster
	}
	return
}

// GetShardID is an internal getter (TBD...)
func (v *SyncShardStatusRequest) GetShardID() (o int64) {
	if v != nil {
		return v.ShardID
	}
	return
}

// GetTimestamp is an internal getter (TBD...)
func (v *SyncShardStatusRequest) GetTimestamp() (o int64) {
	if v != nil && v.Timestamp != nil {
		return *v.Timestamp
	}
	return
}

// HistoryTerminateWorkflowExecutionRequest is an internal type (TBD...)
type HistoryTerminateWorkflowExecutionRequest struct {
	DomainUUID                string                             `json:"domainUUID,omitempty"`
	TerminateRequest          *TerminateWorkflowExecutionRequest `json:"terminateRequest,omitempty"`
	ExternalWorkflowExecution *WorkflowExecution                 `json:"externalWorkflowExecution,omitempty"`
	ChildWorkflowOnly         bool                               `json:"childWorkflowOnly,omitempty"`
}

// GetDomainUUID is an internal getter (TBD...)
func (v *HistoryTerminateWorkflowExecutionRequest) GetDomainUUID() (o string) {
	if v != nil {
		return v.DomainUUID
	}
	return
}

// GetTerminateRequest is an internal getter (TBD...)
func (v *HistoryTerminateWorkflowExecutionRequest) GetTerminateRequest() (o *TerminateWorkflowExecutionRequest) {
	if v != nil && v.TerminateRequest != nil {
		return v.TerminateRequest
	}
	return
}

// GetExternalWorkflowExecution is an internal getter (TBD...)
func (v *HistoryTerminateWorkflowExecutionRequest) GetExternalWorkflowExecution() (o *WorkflowExecution) {
	if v != nil && v.ExternalWorkflowExecution != nil {
		return v.ExternalWorkflowExecution
	}
	return
}

// GetChildWorkflowOnly is an internal getter (TBD...)
func (v *HistoryTerminateWorkflowExecutionRequest) GetChildWorkflowOnly() (o bool) {
	if v != nil {
		return v.ChildWorkflowOnly
	}
	return
}

// GetFailoverInfoRequest is an internal type (TBD...)
type GetFailoverInfoRequest struct {
	DomainID string `json:"domainID,omitempty"`
}

// GetDomainID is an internal getter (TBD...)
func (v *GetFailoverInfoRequest) GetDomainID() (o string) {
	if v != nil {
		return v.DomainID
	}
	return
}

// GetFailoverInfoResponse is an internal type (TBD...)
type GetFailoverInfoResponse struct {
	CompletedShardCount int32   `json:"completedShardCount,omitempty"`
	PendingShards       []int32 `json:"pendingShards,omitempty"`
}

// GetCompletedShardCount is an internal getter (TBD...)
func (v *GetFailoverInfoResponse) GetCompletedShardCount() (o int32) {
	if v != nil {
		return v.CompletedShardCount
	}
	return
}

// GetPendingShards is an internal getter (TBD...)
func (v *GetFailoverInfoResponse) GetPendingShards() (o []int32) {
	if v != nil {
		return v.PendingShards
	}
	return
}

type RatelimitUpdateRequest struct {
	Any *Any `json:"any"`
}

type RatelimitUpdateResponse struct {
	Any *Any `json:"any"`
}
