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

package history

import (
	"fmt"
	"math"
	"time"

	"github.com/pborman/uuid"
	h "github.com/uber/cadence/.gen/go/history"
	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
)

const (
	emptyUUID = "emptyUuid"

	mutableStateInvalidHistoryActionMsg         = "invalid history builder state for action"
	mutableStateInvalidHistoryActionMsgTemplate = mutableStateInvalidHistoryActionMsg + ": %v"
)

var (
	// ErrWorkflowFinished indicates trying to mutate mutable state after workflow finished
	ErrWorkflowFinished = &workflow.InternalServiceError{Message: "invalid mutable state action: mutation after finish"}

	emptyTasks = []persistence.Task{}
)

type (
	mutableStateBuilder struct {
		pendingActivityInfoIDs          map[int64]*persistence.ActivityInfo    // Schedule Event ID -> Activity Info.
		pendingActivityInfoByActivityID map[string]int64                       // Activity ID -> Schedule Event ID of the activity.
		updateActivityInfos             map[*persistence.ActivityInfo]struct{} // Modified activities from last update.
		deleteActivityInfos             map[int64]struct{}                     // Deleted activities from last update.
		syncActivityTasks               map[int64]struct{}                     // Activity to be sync to remote

		pendingTimerInfoIDs map[string]*persistence.TimerInfo   // User Timer ID -> Timer Info.
		updateTimerInfos    map[*persistence.TimerInfo]struct{} // Modified timers from last update.
		deleteTimerInfos    map[string]struct{}                 // Deleted timers from last update.

		pendingChildExecutionInfoIDs map[int64]*persistence.ChildExecutionInfo    // Initiated Event ID -> Child Execution Info
		updateChildExecutionInfos    map[*persistence.ChildExecutionInfo]struct{} // Modified ChildExecution Infos since last update
		deleteChildExecutionInfo     *int64                                       // Deleted ChildExecution Info since last update

		pendingRequestCancelInfoIDs map[int64]*persistence.RequestCancelInfo    // Initiated Event ID -> RequestCancelInfo
		updateRequestCancelInfos    map[*persistence.RequestCancelInfo]struct{} // Modified RequestCancel Infos since last update, for persistence update
		deleteRequestCancelInfo     *int64                                      // Deleted RequestCancel Info since last update, for persistence update

		pendingSignalInfoIDs map[int64]*persistence.SignalInfo    // Initiated Event ID -> SignalInfo
		updateSignalInfos    map[*persistence.SignalInfo]struct{} // Modified SignalInfo since last update
		deleteSignalInfo     *int64                               // Deleted SignalInfo since last update

		pendingSignalRequestedIDs map[string]struct{} // Set of signaled requestIds
		updateSignalRequestedIDs  map[string]struct{} // Set of signaled requestIds since last update
		deleteSignalRequestedID   string              // Deleted signaled requestId

		bufferedEvents       []*workflow.HistoryEvent // buffered history events that are already persisted
		updateBufferedEvents []*workflow.HistoryEvent // buffered history events that needs to be persisted
		clearBufferedEvents  bool                     // delete buffered events from persistence

		executionInfo    *persistence.WorkflowExecutionInfo // Workflow mutable state info.
		replicationState *persistence.ReplicationState
		hBuilder         *historyBuilder

		// in memory only attributes
		// indicates whether there are buffered events in persistence
		hasBufferedEventsInPersistence bool
		// indicates the next event ID in DB, for condition update
		condition int64
		// indicate whether can do replication
		replicationPolicy cache.ReplicationPolicy

		insertTransferTasks    []persistence.Task
		insertReplicationTasks []persistence.Task
		insertTimerTasks       []persistence.Task

		shard           ShardContext
		clusterMetadata cluster.Metadata
		eventsCache     eventsCache
		config          *Config
		timeSource      clock.TimeSource
		logger          log.Logger
		domainName      string
	}
)

var _ mutableState = (*mutableStateBuilder)(nil)

func newMutableStateBuilder(
	shard ShardContext,
	eventsCache eventsCache,
	logger log.Logger,
	domainName string,
) *mutableStateBuilder {
	s := &mutableStateBuilder{
		updateActivityInfos:             make(map[*persistence.ActivityInfo]struct{}),
		pendingActivityInfoIDs:          make(map[int64]*persistence.ActivityInfo),
		pendingActivityInfoByActivityID: make(map[string]int64),
		deleteActivityInfos:             make(map[int64]struct{}),
		syncActivityTasks:               make(map[int64]struct{}),

		pendingTimerInfoIDs: make(map[string]*persistence.TimerInfo),
		updateTimerInfos:    make(map[*persistence.TimerInfo]struct{}),
		deleteTimerInfos:    make(map[string]struct{}),

		updateChildExecutionInfos:    make(map[*persistence.ChildExecutionInfo]struct{}),
		pendingChildExecutionInfoIDs: make(map[int64]*persistence.ChildExecutionInfo),
		deleteChildExecutionInfo:     nil,

		updateRequestCancelInfos:    make(map[*persistence.RequestCancelInfo]struct{}),
		pendingRequestCancelInfoIDs: make(map[int64]*persistence.RequestCancelInfo),
		deleteRequestCancelInfo:     nil,

		updateSignalInfos:    make(map[*persistence.SignalInfo]struct{}),
		pendingSignalInfoIDs: make(map[int64]*persistence.SignalInfo),
		deleteSignalInfo:     nil,

		updateSignalRequestedIDs:  make(map[string]struct{}),
		pendingSignalRequestedIDs: make(map[string]struct{}),
		deleteSignalRequestedID:   "",

		hasBufferedEventsInPersistence: false,
		condition:                      0,

		clusterMetadata: shard.GetClusterMetadata(),
		eventsCache:     eventsCache,
		shard:           shard,
		config:          shard.GetConfig(),
		timeSource:      shard.GetTimeSource(),
		logger:          logger,
		domainName:      domainName,
	}
	s.executionInfo = &persistence.WorkflowExecutionInfo{
		DecisionVersion:    common.EmptyVersion,
		DecisionScheduleID: common.EmptyEventID,
		DecisionStartedID:  common.EmptyEventID,
		DecisionRequestID:  emptyUUID,
		DecisionTimeout:    0,

		NextEventID:        common.FirstEventID,
		State:              persistence.WorkflowStateCreated,
		CloseStatus:        persistence.WorkflowCloseStatusNone,
		LastProcessedEvent: common.EmptyEventID,
	}
	s.hBuilder = newHistoryBuilder(s, logger)

	return s
}

func newMutableStateBuilderWithReplicationState(
	shard ShardContext,
	eventsCache eventsCache,
	logger log.Logger,
	version int64,
	replicationPolicy cache.ReplicationPolicy,
	domainName string,
) *mutableStateBuilder {
	s := newMutableStateBuilder(shard, eventsCache, logger, domainName)
	s.replicationState = &persistence.ReplicationState{
		StartVersion:        version,
		CurrentVersion:      version,
		LastWriteVersion:    common.EmptyVersion,
		LastWriteEventID:    common.EmptyEventID,
		LastReplicationInfo: make(map[string]*persistence.ReplicationInfo),
	}
	s.replicationPolicy = replicationPolicy
	return s
}

func (e *mutableStateBuilder) CopyToPersistence() *persistence.WorkflowMutableState {
	state := &persistence.WorkflowMutableState{}

	state.ActivityInfos = e.pendingActivityInfoIDs
	state.TimerInfos = e.pendingTimerInfoIDs
	state.ChildExecutionInfos = e.pendingChildExecutionInfoIDs
	state.RequestCancelInfos = e.pendingRequestCancelInfoIDs
	state.SignalInfos = e.pendingSignalInfoIDs
	state.SignalRequestedIDs = e.pendingSignalRequestedIDs
	state.ExecutionInfo = e.executionInfo
	state.ReplicationState = e.replicationState
	state.BufferedEvents = e.bufferedEvents
	return state
}

func (e *mutableStateBuilder) Load(state *persistence.WorkflowMutableState) {

	e.pendingActivityInfoIDs = state.ActivityInfos
	e.pendingTimerInfoIDs = state.TimerInfos
	e.pendingChildExecutionInfoIDs = state.ChildExecutionInfos
	e.pendingRequestCancelInfoIDs = state.RequestCancelInfos
	e.pendingSignalInfoIDs = state.SignalInfos
	e.pendingSignalRequestedIDs = state.SignalRequestedIDs
	e.executionInfo = state.ExecutionInfo

	e.replicationState = state.ReplicationState
	e.bufferedEvents = state.BufferedEvents
	for _, ai := range state.ActivityInfos {
		e.pendingActivityInfoByActivityID[ai.ActivityID] = ai.ScheduleID
	}

	e.hasBufferedEventsInPersistence = len(e.bufferedEvents) > 0
	e.condition = state.ExecutionInfo.NextEventID
}

func (e *mutableStateBuilder) GetEventStoreVersion() int32 {
	return e.GetExecutionInfo().EventStoreVersion
}

func (e *mutableStateBuilder) GetCurrentBranch() []byte {
	return e.executionInfo.GetCurrentBranch()
}

// set eventStoreVersion/treeID/historyBranches
func (e *mutableStateBuilder) SetHistoryTree(treeID string) error {
	initialBranchToken, err := persistence.NewHistoryBranchToken(treeID)
	if err != nil {
		return err
	}
	exeInfo := e.GetExecutionInfo()
	exeInfo.EventStoreVersion = persistence.EventStoreVersionV2
	exeInfo.BranchToken = initialBranchToken
	return nil
}

func (e *mutableStateBuilder) GetHistoryBuilder() *historyBuilder {
	return e.hBuilder
}

func (e *mutableStateBuilder) SetHistoryBuilder(hBuilder *historyBuilder) {
	e.hBuilder = hBuilder
}

func (e *mutableStateBuilder) GetExecutionInfo() *persistence.WorkflowExecutionInfo {
	return e.executionInfo
}

func (e *mutableStateBuilder) GetReplicationState() *persistence.ReplicationState {
	return e.replicationState
}

func (e *mutableStateBuilder) FlushBufferedEvents() error {
	// put new events into 2 buckets:
	//  1) if the event was added while there was in-flight decision, then put it in buffered bucket
	//  2) otherwise, put it in committed bucket
	var newBufferedEvents []*workflow.HistoryEvent
	var newCommittedEvents []*workflow.HistoryEvent
	for _, event := range e.hBuilder.history {
		if event.GetEventId() == common.BufferedEventID {
			newBufferedEvents = append(newBufferedEvents, event)
		} else {
			newCommittedEvents = append(newCommittedEvents, event)
		}
	}

	// Sometimes we see buffered events are out of order when read back from database.  This is mostly not an issue
	// except in the Activity case where ActivityStarted and ActivityCompleted gets out of order.  The following code
	// is added to reorder buffered events to guarantee all activity completion events will always be processed at the end.
	var reorderedEvents []*workflow.HistoryEvent
	reorderFunc := func(bufferedEvents []*workflow.HistoryEvent) {
		for _, event := range bufferedEvents {
			switch event.GetEventType() {
			case workflow.EventTypeActivityTaskCompleted,
				workflow.EventTypeActivityTaskFailed,
				workflow.EventTypeActivityTaskCanceled,
				workflow.EventTypeActivityTaskTimedOut:
				reorderedEvents = append(reorderedEvents, event)
			case workflow.EventTypeChildWorkflowExecutionCompleted,
				workflow.EventTypeChildWorkflowExecutionFailed,
				workflow.EventTypeChildWorkflowExecutionCanceled,
				workflow.EventTypeChildWorkflowExecutionTimedOut,
				workflow.EventTypeChildWorkflowExecutionTerminated:
				reorderedEvents = append(reorderedEvents, event)
			default:
				newCommittedEvents = append(newCommittedEvents, event)
			}
		}
	}

	// no decision in-flight, flush all buffered events to committed bucket
	if !e.HasInFlightDecisionTask() {
		// flush persisted buffered events
		if len(e.bufferedEvents) > 0 {
			reorderFunc(e.bufferedEvents)
			e.bufferedEvents = nil
		}
		if e.hasBufferedEventsInPersistence {
			e.clearBufferedEvents = true
		}

		// flush pending buffered events
		reorderFunc(e.updateBufferedEvents)
		// clear pending buffered events
		e.updateBufferedEvents = nil

		// Put back all the reordered buffer events at the end
		if len(reorderedEvents) > 0 {
			newCommittedEvents = append(newCommittedEvents, reorderedEvents...)
		}

		// flush new buffered events that were not saved to persistence yet
		newCommittedEvents = append(newCommittedEvents, newBufferedEvents...)
		newBufferedEvents = nil
	}

	newCommittedEvents = e.trimEventsAfterWorkflowClose(newCommittedEvents)
	e.hBuilder.history = newCommittedEvents
	// make sure all new committed events have correct EventID
	e.assignEventIDToBufferedEvents()
	if err := e.assignTaskIDToEvents(); err != nil {
		return err
	}

	// if decision is not closed yet, and there are new buffered events, then put those to the pending buffer
	if e.HasInFlightDecisionTask() && len(newBufferedEvents) > 0 {
		e.updateBufferedEvents = newBufferedEvents
	}

	return nil
}

func (e *mutableStateBuilder) GetStartVersion() int64 {
	if e.replicationState == nil {
		return common.EmptyVersion
	}
	return e.replicationState.StartVersion
}

func (e *mutableStateBuilder) GetCurrentVersion() int64 {
	if e.replicationState == nil {
		return common.EmptyVersion
	}
	return e.replicationState.CurrentVersion
}

func (e *mutableStateBuilder) GetLastWriteVersion() int64 {
	if e.replicationState == nil {
		return common.EmptyVersion
	}
	return e.replicationState.LastWriteVersion
}

func (e *mutableStateBuilder) UpdateReplicationPolicy(
	replicationPolicy cache.ReplicationPolicy,
) {

	e.replicationPolicy = replicationPolicy
}

func (e *mutableStateBuilder) UpdateReplicationStateVersion(
	version int64,
	forceUpdate bool,
) {

	if version > e.replicationState.CurrentVersion || forceUpdate {
		e.replicationState.CurrentVersion = version
	}
}

// Assumption: It is expected CurrentVersion on replication state is updated at the start of transaction when
// mutableState is loaded for this workflow execution.
func (e *mutableStateBuilder) UpdateReplicationStateLastEventID(
	lastWriteVersion,
	lastEventID int64,
) {
	e.replicationState.LastWriteVersion = lastWriteVersion
	e.replicationState.LastWriteEventID = lastEventID

	lastEventSourceCluster := e.clusterMetadata.ClusterNameForFailoverVersion(lastWriteVersion)
	currentCluster := e.clusterMetadata.GetCurrentClusterName()
	if lastEventSourceCluster != currentCluster {
		info, ok := e.replicationState.LastReplicationInfo[lastEventSourceCluster]
		if !ok {
			// ReplicationInfo doesn't exist for this cluster, create one
			info = &persistence.ReplicationInfo{}
			e.replicationState.LastReplicationInfo[lastEventSourceCluster] = info
		}

		info.Version = lastWriteVersion
		info.LastEventID = lastEventID
	}
}

func (e *mutableStateBuilder) checkAndClearTimerFiredEvent(timerID string) *workflow.HistoryEvent {
	var timerEvent *workflow.HistoryEvent

	e.bufferedEvents, timerEvent = checkAndClearTimerFiredEvent(e.bufferedEvents, timerID)
	if timerEvent != nil {
		return timerEvent
	}
	e.updateBufferedEvents, timerEvent = checkAndClearTimerFiredEvent(e.updateBufferedEvents, timerID)
	if timerEvent != nil {
		return timerEvent
	}
	e.hBuilder.history, timerEvent = checkAndClearTimerFiredEvent(e.hBuilder.history, timerID)
	return timerEvent
}

func checkAndClearTimerFiredEvent(
	events []*workflow.HistoryEvent,
	timerID string,
) ([]*workflow.HistoryEvent, *workflow.HistoryEvent) {
	// go over all history events. if we find a timer fired event for the given
	// timerID, clear it
	timerFiredIdx := -1
	for idx, event := range events {
		if event.GetEventType() == workflow.EventTypeTimerFired &&
			event.GetTimerFiredEventAttributes().GetTimerId() == timerID {
			timerFiredIdx = idx
			break
		}
	}
	if timerFiredIdx == -1 {
		return events, nil
	}

	timerEvent := events[timerFiredIdx]
	return append(events[:timerFiredIdx], events[timerFiredIdx+1:]...), timerEvent
}

func (e *mutableStateBuilder) trimEventsAfterWorkflowClose(input []*workflow.HistoryEvent) []*workflow.HistoryEvent {
	if len(input) == 0 {
		return input
	}

	nextIndex := 0

loop:
	for _, event := range input {
		nextIndex++

		switch event.GetEventType() {
		case workflow.EventTypeWorkflowExecutionCompleted,
			workflow.EventTypeWorkflowExecutionFailed,
			workflow.EventTypeWorkflowExecutionTimedOut,
			workflow.EventTypeWorkflowExecutionTerminated,
			workflow.EventTypeWorkflowExecutionContinuedAsNew,
			workflow.EventTypeWorkflowExecutionCanceled:

			break loop
		}
	}

	return input[0:nextIndex]
}

func (e *mutableStateBuilder) assignEventIDToBufferedEvents() {
	newCommittedEvents := e.hBuilder.history

	scheduledIDToStartedID := make(map[int64]int64)
	for _, event := range newCommittedEvents {
		if event.GetEventId() != common.BufferedEventID {
			continue
		}

		eventID := e.executionInfo.NextEventID
		event.EventId = common.Int64Ptr(eventID)
		e.executionInfo.IncreaseNextEventID()

		switch event.GetEventType() {
		case workflow.EventTypeActivityTaskStarted:
			attributes := event.ActivityTaskStartedEventAttributes
			scheduledID := attributes.GetScheduledEventId()
			scheduledIDToStartedID[scheduledID] = eventID
			if ai, ok := e.GetActivityInfo(scheduledID); ok {
				ai.StartedID = eventID
				e.updateActivityInfos[ai] = struct{}{}
			}
		case workflow.EventTypeChildWorkflowExecutionStarted:
			attributes := event.ChildWorkflowExecutionStartedEventAttributes
			initiatedID := attributes.GetInitiatedEventId()
			scheduledIDToStartedID[initiatedID] = eventID
			if ci, ok := e.GetChildExecutionInfo(initiatedID); ok {
				ci.StartedID = eventID
				e.updateChildExecutionInfos[ci] = struct{}{}
			}
		case workflow.EventTypeActivityTaskCompleted:
			attributes := event.ActivityTaskCompletedEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetScheduledEventId()]; ok {
				attributes.StartedEventId = common.Int64Ptr(startedID)
			}
		case workflow.EventTypeActivityTaskFailed:
			attributes := event.ActivityTaskFailedEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetScheduledEventId()]; ok {
				attributes.StartedEventId = common.Int64Ptr(startedID)
			}
		case workflow.EventTypeActivityTaskTimedOut:
			attributes := event.ActivityTaskTimedOutEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetScheduledEventId()]; ok {
				attributes.StartedEventId = common.Int64Ptr(startedID)
			}
		case workflow.EventTypeActivityTaskCanceled:
			attributes := event.ActivityTaskCanceledEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetScheduledEventId()]; ok {
				attributes.StartedEventId = common.Int64Ptr(startedID)
			}
		case workflow.EventTypeChildWorkflowExecutionCompleted:
			attributes := event.ChildWorkflowExecutionCompletedEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetInitiatedEventId()]; ok {
				attributes.StartedEventId = common.Int64Ptr(startedID)
			}
		case workflow.EventTypeChildWorkflowExecutionFailed:
			attributes := event.ChildWorkflowExecutionFailedEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetInitiatedEventId()]; ok {
				attributes.StartedEventId = common.Int64Ptr(startedID)
			}
		case workflow.EventTypeChildWorkflowExecutionTimedOut:
			attributes := event.ChildWorkflowExecutionTimedOutEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetInitiatedEventId()]; ok {
				attributes.StartedEventId = common.Int64Ptr(startedID)
			}
		case workflow.EventTypeChildWorkflowExecutionCanceled:
			attributes := event.ChildWorkflowExecutionCanceledEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetInitiatedEventId()]; ok {
				attributes.StartedEventId = common.Int64Ptr(startedID)
			}
		case workflow.EventTypeChildWorkflowExecutionTerminated:
			attributes := event.ChildWorkflowExecutionTerminatedEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetInitiatedEventId()]; ok {
				attributes.StartedEventId = common.Int64Ptr(startedID)
			}
		}
	}
}

func (e *mutableStateBuilder) assignTaskIDToEvents() error {

	// assign task IDs to all history events
	// first transient events
	numTaskIDs := len(e.hBuilder.transientHistory)
	if numTaskIDs > 0 {
		taskIDs, err := e.shard.GenerateTransferTaskIDs(numTaskIDs)
		if err != nil {
			return err
		}

		for index, event := range e.hBuilder.transientHistory {
			if event.GetTaskId() == common.EmptyEventTaskID {
				taskID := taskIDs[index]
				event.TaskId = common.Int64Ptr(taskID)
				e.executionInfo.LastEventTaskID = taskID
			}
		}
	}

	// then normal events
	numTaskIDs = len(e.hBuilder.history)
	if numTaskIDs > 0 {
		taskIDs, err := e.shard.GenerateTransferTaskIDs(numTaskIDs)
		if err != nil {
			return err
		}

		for index, event := range e.hBuilder.history {
			if event.GetTaskId() == common.EmptyEventTaskID {
				taskID := taskIDs[index]
				event.TaskId = common.Int64Ptr(taskID)
				e.executionInfo.LastEventTaskID = taskID
			}
		}
	}

	return nil
}

func (e *mutableStateBuilder) IsStickyTaskListEnabled() bool {
	if e.executionInfo.StickyTaskList == "" {
		return false
	}
	ttl := e.config.StickyTTL(e.domainName)
	if e.timeSource.Now().After(e.executionInfo.LastUpdatedTimestamp.Add(ttl)) {
		return false
	}
	return true
}

func (e *mutableStateBuilder) CreateNewHistoryEvent(eventType workflow.EventType) *workflow.HistoryEvent {
	return e.CreateNewHistoryEventWithTimestamp(eventType, e.timeSource.Now().UnixNano())
}

func (e *mutableStateBuilder) CreateNewHistoryEventWithTimestamp(
	eventType workflow.EventType,
	timestamp int64,
) *workflow.HistoryEvent {
	eventID := e.executionInfo.NextEventID
	if e.shouldBufferEvent(eventType) {
		eventID = common.BufferedEventID
	} else {
		// only increase NextEventID if event is not buffered
		e.executionInfo.IncreaseNextEventID()
	}

	ts := common.Int64Ptr(timestamp)
	historyEvent := &workflow.HistoryEvent{}
	historyEvent.EventId = common.Int64Ptr(eventID)
	historyEvent.Timestamp = ts
	historyEvent.EventType = common.EventTypePtr(eventType)
	historyEvent.Version = common.Int64Ptr(e.GetCurrentVersion())
	historyEvent.TaskId = common.Int64Ptr(common.EmptyEventTaskID)

	return historyEvent
}

func (e *mutableStateBuilder) shouldBufferEvent(eventType workflow.EventType) bool {
	switch eventType {
	case // do not buffer for workflow state change
		workflow.EventTypeWorkflowExecutionStarted,
		workflow.EventTypeWorkflowExecutionCompleted,
		workflow.EventTypeWorkflowExecutionFailed,
		workflow.EventTypeWorkflowExecutionTimedOut,
		workflow.EventTypeWorkflowExecutionTerminated,
		workflow.EventTypeWorkflowExecutionContinuedAsNew,
		workflow.EventTypeWorkflowExecutionCanceled:
		return false
	case // decision event should not be buffered
		workflow.EventTypeDecisionTaskScheduled,
		workflow.EventTypeDecisionTaskStarted,
		workflow.EventTypeDecisionTaskCompleted,
		workflow.EventTypeDecisionTaskFailed,
		workflow.EventTypeDecisionTaskTimedOut:
		return false
	case // events generated directly from decisions should not be buffered
		// workflow complete, failed, cancelled and continue-as-new events are duplication of above
		// just put is here for reference
		// workflow.EventTypeWorkflowExecutionCompleted,
		// workflow.EventTypeWorkflowExecutionFailed,
		// workflow.EventTypeWorkflowExecutionCanceled,
		// workflow.EventTypeWorkflowExecutionContinuedAsNew,
		workflow.EventTypeActivityTaskScheduled,
		workflow.EventTypeActivityTaskCancelRequested,
		workflow.EventTypeTimerStarted,
		// DecisionTypeCancelTimer is an exception. This decision will be mapped
		// to either workflow.EventTypeTimerCanceled, or workflow.EventTypeCancelTimerFailed.
		// So both should not be buffered. Ref: historyEngine, search for "workflow.DecisionTypeCancelTimer"
		workflow.EventTypeTimerCanceled,
		workflow.EventTypeCancelTimerFailed,
		workflow.EventTypeRequestCancelExternalWorkflowExecutionInitiated,
		workflow.EventTypeMarkerRecorded,
		workflow.EventTypeStartChildWorkflowExecutionInitiated,
		workflow.EventTypeSignalExternalWorkflowExecutionInitiated,
		workflow.EventTypeUpsertWorkflowSearchAttributes:
		// do not buffer event if event is directly generated from a corresponding decision

		// sanity check there is no decision on the fly
		if e.HasInFlightDecisionTask() {
			msg := fmt.Sprintf("history mutable state is processing event: %v while there is decision pending. "+
				"domainID: %v, workflow ID: %v, run ID: %v.", eventType, e.executionInfo.DomainID, e.executionInfo.WorkflowID, e.executionInfo.RunID)
			panic(msg)
		}
		return false
	default:
		return true
	}
}

func (e *mutableStateBuilder) GetWorkflowType() *workflow.WorkflowType {
	wType := &workflow.WorkflowType{}
	wType.Name = common.StringPtr(e.executionInfo.WorkflowTypeName)

	return wType
}

func (e *mutableStateBuilder) GetActivityScheduledEvent(scheduleEventID int64) (*workflow.HistoryEvent, bool) {
	ai, ok := e.pendingActivityInfoIDs[scheduleEventID]
	if !ok {
		return nil, false
	}

	// Needed for backward compatibility reason
	if ai.ScheduledEvent != nil {
		return ai.ScheduledEvent, true
	}

	scheduledEvent, err := e.eventsCache.getEvent(e.executionInfo.DomainID, e.executionInfo.WorkflowID,
		e.executionInfo.RunID, ai.ScheduledEventBatchID, ai.ScheduleID, e.executionInfo.EventStoreVersion,
		e.executionInfo.GetCurrentBranch())
	if err != nil {
		return nil, false
	}
	return scheduledEvent, true
}

// GetActivityInfo gives details about an activity that is currently in progress.
func (e *mutableStateBuilder) GetActivityInfo(scheduleEventID int64) (*persistence.ActivityInfo, bool) {
	ai, ok := e.pendingActivityInfoIDs[scheduleEventID]
	return ai, ok
}

// GetActivityByActivityID gives details about an activity that is currently in progress.
func (e *mutableStateBuilder) GetActivityByActivityID(activityID string) (*persistence.ActivityInfo, bool) {
	eventID, ok := e.pendingActivityInfoByActivityID[activityID]
	if !ok {
		return nil, false
	}

	ai, ok := e.pendingActivityInfoIDs[eventID]
	return ai, ok
}

// GetScheduleIDByActivityID return scheduleID given activityID
func (e *mutableStateBuilder) GetScheduleIDByActivityID(activityID string) (int64, bool) {
	scheduleID, ok := e.pendingActivityInfoByActivityID[activityID]
	return scheduleID, ok
}

// GetChildExecutionInfo gives details about a child execution that is currently in progress.
func (e *mutableStateBuilder) GetChildExecutionInfo(initiatedEventID int64) (*persistence.ChildExecutionInfo, bool) {
	ci, ok := e.pendingChildExecutionInfoIDs[initiatedEventID]
	return ci, ok
}

// GetChildExecutionInitiatedEvent reads out the ChildExecutionInitiatedEvent from mutable state for in-progress child
// executions
func (e *mutableStateBuilder) GetChildExecutionInitiatedEvent(initiatedEventID int64) (*workflow.HistoryEvent, bool) {
	ci, ok := e.pendingChildExecutionInfoIDs[initiatedEventID]
	if !ok {
		return nil, false
	}

	// Needed for backward compatibility reason
	if ci.InitiatedEvent != nil {
		return ci.InitiatedEvent, true
	}

	initiatedEvent, err := e.eventsCache.getEvent(e.executionInfo.DomainID, e.executionInfo.WorkflowID,
		e.executionInfo.RunID, ci.InitiatedEventBatchID, ci.InitiatedID, e.executionInfo.EventStoreVersion,
		e.executionInfo.GetCurrentBranch())
	if err != nil {
		return nil, false
	}
	return initiatedEvent, true
}

// GetRequestCancelInfo gives details about a request cancellation that is currently in progress.
func (e *mutableStateBuilder) GetRequestCancelInfo(initiatedEventID int64) (*persistence.RequestCancelInfo, bool) {
	ri, ok := e.pendingRequestCancelInfoIDs[initiatedEventID]
	return ri, ok
}

func (e *mutableStateBuilder) GetRetryBackoffDuration(errReason string) time.Duration {
	info := e.executionInfo
	if !info.HasRetryPolicy {
		return backoff.NoBackoff
	}

	return getBackoffInterval(info.Attempt, info.MaximumAttempts, info.InitialInterval, info.MaximumInterval, info.BackoffCoefficient, e.timeSource.Now(), info.ExpirationTime, errReason, info.NonRetriableErrors)
}

func (e *mutableStateBuilder) GetCronBackoffDuration() time.Duration {
	info := e.executionInfo
	if len(info.CronSchedule) == 0 {
		return backoff.NoBackoff
	}
	return backoff.GetBackoffForNextSchedule(info.CronSchedule, e.executionInfo.StartTimestamp, e.timeSource.Now())
}

// GetSignalInfo get details about a signal request that is currently in progress.
func (e *mutableStateBuilder) GetSignalInfo(initiatedEventID int64) (*persistence.SignalInfo, bool) {
	ri, ok := e.pendingSignalInfoIDs[initiatedEventID]
	return ri, ok
}

func (e *mutableStateBuilder) GetAllSignalsToSend() map[int64]*persistence.SignalInfo {
	return e.pendingSignalInfoIDs
}

func (e *mutableStateBuilder) GetAllRequestCancels() map[int64]*persistence.RequestCancelInfo {
	return e.pendingRequestCancelInfoIDs
}

// GetCompletionEvent retrieves the workflow completion event from mutable state
func (e *mutableStateBuilder) GetCompletionEvent() (*workflow.HistoryEvent, bool) {
	if e.executionInfo.State != persistence.WorkflowStateCompleted {
		return nil, false
	}

	// Needed for backward compatibility reason
	if e.executionInfo.CompletionEvent != nil {
		return e.executionInfo.CompletionEvent, true
	}

	// Needed for backward compatibility reason
	if e.executionInfo.CompletionEventBatchID == common.EmptyEventID {
		return nil, false
	}

	// Completion EventID is always one less than NextEventID after workflow is completed
	completionEventID := e.executionInfo.NextEventID - 1
	firstEventID := e.executionInfo.CompletionEventBatchID
	completionEvent, err := e.eventsCache.getEvent(e.executionInfo.DomainID, e.executionInfo.WorkflowID,
		e.executionInfo.RunID, firstEventID, completionEventID, e.executionInfo.EventStoreVersion,
		e.executionInfo.GetCurrentBranch())
	if err != nil {
		return nil, false
	}

	return completionEvent, true
}

// GetStartEvent retrieves the workflow start event from mutable state
func (e *mutableStateBuilder) GetStartEvent() (*workflow.HistoryEvent, bool) {
	startEvent, err := e.eventsCache.getEvent(e.executionInfo.DomainID, e.executionInfo.WorkflowID,
		e.executionInfo.RunID, common.FirstEventID, common.FirstEventID, e.executionInfo.EventStoreVersion,
		e.executionInfo.GetCurrentBranch())
	if err != nil {
		return nil, false
	}
	return startEvent, true
}

// DeletePendingChildExecution deletes details about a ChildExecutionInfo.
func (e *mutableStateBuilder) DeletePendingChildExecution(initiatedEventID int64) {
	delete(e.pendingChildExecutionInfoIDs, initiatedEventID)
	e.deleteChildExecutionInfo = common.Int64Ptr(initiatedEventID)
}

// DeletePendingRequestCancel deletes details about a RequestCancelInfo.
func (e *mutableStateBuilder) DeletePendingRequestCancel(initiatedEventID int64) {
	delete(e.pendingRequestCancelInfoIDs, initiatedEventID)
	e.deleteRequestCancelInfo = common.Int64Ptr(initiatedEventID)
}

// DeletePendingSignal deletes details about a SignalInfo
func (e *mutableStateBuilder) DeletePendingSignal(initiatedEventID int64) {
	delete(e.pendingSignalInfoIDs, initiatedEventID)
	e.deleteSignalInfo = common.Int64Ptr(initiatedEventID)
}

func (e *mutableStateBuilder) writeEventToCache(event *workflow.HistoryEvent) {
	// For start event: store it within events cache so the recordWorkflowStarted transfer task doesn't need to
	// load it from database
	// For completion event: store it within events cache so we can communicate the result to parent execution
	// during the processing of DeleteTransferTask without loading this event from database
	e.eventsCache.putEvent(e.executionInfo.DomainID, e.executionInfo.WorkflowID, e.executionInfo.RunID,
		event.GetEventId(), event)
}

func (e *mutableStateBuilder) hasPendingTasks() bool {
	return len(e.pendingActivityInfoIDs) > 0 || len(e.pendingTimerInfoIDs) > 0
}

func (e *mutableStateBuilder) HasParentExecution() bool {
	return e.executionInfo.ParentDomainID != "" && e.executionInfo.ParentWorkflowID != ""
}

func (e *mutableStateBuilder) UpdateActivityProgress(
	ai *persistence.ActivityInfo,
	request *workflow.RecordActivityTaskHeartbeatRequest,
) {
	ai.Version = e.GetCurrentVersion()
	ai.Details = request.Details
	ai.LastHeartBeatUpdatedTime = e.timeSource.Now()
	e.updateActivityInfos[ai] = struct{}{}
	e.syncActivityTasks[ai.ScheduleID] = struct{}{}
}

// ReplicateActivityInfo replicate the necessary activity information
func (e *mutableStateBuilder) ReplicateActivityInfo(
	request *h.SyncActivityRequest,
	resetActivityTimerTaskStatus bool,
) error {
	ai, ok := e.pendingActivityInfoIDs[request.GetScheduledId()]
	if !ok {
		return fmt.Errorf("unable to find activity with schedule event id: %v in mutable state", ai.ScheduleID)
	}

	ai.Version = request.GetVersion()
	ai.ScheduledTime = time.Unix(0, request.GetScheduledTime())
	ai.StartedID = request.GetStartedId()
	ai.LastHeartBeatUpdatedTime = time.Unix(0, request.GetLastHeartbeatTime())
	if ai.StartedID == common.EmptyEventID {
		ai.StartedTime = time.Time{}
	} else {
		ai.StartedTime = time.Unix(0, request.GetStartedTime())
	}
	ai.Details = request.GetDetails()
	ai.Attempt = request.GetAttempt()
	ai.LastFailureReason = request.GetLastFailureReason()
	ai.LastWorkerIdentity = request.GetLastWorkerIdentity()

	if resetActivityTimerTaskStatus {
		ai.TimerTaskStatus = TimerTaskStatusNone
	}

	e.updateActivityInfos[ai] = struct{}{}
	return nil
}

// UpdateActivity updates an activity
func (e *mutableStateBuilder) UpdateActivity(ai *persistence.ActivityInfo) error {
	_, ok := e.pendingActivityInfoIDs[ai.ScheduleID]
	if !ok {
		return fmt.Errorf("unable to find activity with schedule event id: %v in mutable state", ai.ScheduleID)
	}
	e.updateActivityInfos[ai] = struct{}{}
	return nil
}

// DeleteActivity deletes details about an activity.
func (e *mutableStateBuilder) DeleteActivity(scheduleEventID int64) error {
	a, ok := e.pendingActivityInfoIDs[scheduleEventID]
	if !ok {
		errorMsg := fmt.Sprintf("Unable to find activity with schedule event id: %v in mutable state", scheduleEventID)
		e.logger.Error(errorMsg, tag.ErrorTypeInvalidMutableStateAction)
		return errors.NewInternalFailureError(errorMsg)
	}
	delete(e.pendingActivityInfoIDs, scheduleEventID)

	_, ok = e.pendingActivityInfoByActivityID[a.ActivityID]
	if !ok {
		errorMsg := fmt.Sprintf("Unable to find activity: %v in mutable state", a.ActivityID)
		e.logger.Error(errorMsg, tag.ErrorTypeInvalidMutableStateAction)
		return errors.NewInternalFailureError(errorMsg)
	}
	delete(e.pendingActivityInfoByActivityID, a.ActivityID)

	e.deleteActivityInfos[scheduleEventID] = struct{}{}
	return nil
}

// GetUserTimer gives details about a user timer.
func (e *mutableStateBuilder) GetUserTimer(timerID string) (bool, *persistence.TimerInfo) {
	a, ok := e.pendingTimerInfoIDs[timerID]
	return ok, a
}

// UpdateUserTimer updates the user timer in progress.
func (e *mutableStateBuilder) UpdateUserTimer(timerID string, ti *persistence.TimerInfo) {
	e.pendingTimerInfoIDs[timerID] = ti
	e.updateTimerInfos[ti] = struct{}{}
}

// DeleteUserTimer deletes an user timer.
func (e *mutableStateBuilder) DeleteUserTimer(timerID string) {
	delete(e.pendingTimerInfoIDs, timerID)
	e.deleteTimerInfos[timerID] = struct{}{}
}

func (e *mutableStateBuilder) getDecisionInfo() *decisionInfo {
	return &decisionInfo{
		Version:                    e.executionInfo.DecisionVersion,
		ScheduleID:                 e.executionInfo.DecisionScheduleID,
		StartedID:                  e.executionInfo.DecisionStartedID,
		RequestID:                  e.executionInfo.DecisionRequestID,
		DecisionTimeout:            e.executionInfo.DecisionTimeout,
		Attempt:                    e.executionInfo.DecisionAttempt,
		StartedTimestamp:           e.executionInfo.DecisionStartedTimestamp,
		ScheduledTimestamp:         e.executionInfo.DecisionScheduledTimestamp,
		OriginalScheduledTimestamp: e.executionInfo.DecisionOriginalScheduledTimestamp,
	}
}

// GetPendingDecision returns details about the in-progress decision task
func (e *mutableStateBuilder) GetPendingDecision(scheduleEventID int64) (*decisionInfo, bool) {
	di := e.getDecisionInfo()
	if scheduleEventID == di.ScheduleID {
		return di, true
	}
	return nil, false
}

func (e *mutableStateBuilder) GetPendingActivityInfos() map[int64]*persistence.ActivityInfo {
	return e.pendingActivityInfoIDs
}

func (e *mutableStateBuilder) GetPendingTimerInfos() map[string]*persistence.TimerInfo {
	return e.pendingTimerInfoIDs
}

func (e *mutableStateBuilder) GetPendingChildExecutionInfos() map[int64]*persistence.ChildExecutionInfo {
	return e.pendingChildExecutionInfoIDs
}

func (e *mutableStateBuilder) HasPendingDecisionTask() bool {
	return e.executionInfo.DecisionScheduleID != common.EmptyEventID
}

func (e *mutableStateBuilder) HasProcessedOrPendingDecisionTask() bool {
	return e.HasPendingDecisionTask() || e.GetPreviousStartedEventID() != common.EmptyEventID
}

func (e *mutableStateBuilder) HasInFlightDecisionTask() bool {
	return e.executionInfo.DecisionStartedID > 0
}

func (e *mutableStateBuilder) GetInFlightDecisionTask() (*decisionInfo, bool) {
	if e.executionInfo.DecisionScheduleID == common.EmptyEventID ||
		e.executionInfo.DecisionStartedID == common.EmptyEventID {
		return nil, false
	}

	di := e.getDecisionInfo()
	return di, true
}

func (e *mutableStateBuilder) HasBufferedEvents() bool {
	if len(e.bufferedEvents) > 0 || len(e.updateBufferedEvents) > 0 {
		return true
	}

	for _, event := range e.hBuilder.history {
		if event.GetEventId() == common.BufferedEventID {
			return true
		}
	}

	return false
}

// UpdateDecision updates a decision task.
func (e *mutableStateBuilder) UpdateDecision(di *decisionInfo) {
	e.executionInfo.DecisionVersion = di.Version
	e.executionInfo.DecisionScheduleID = di.ScheduleID
	e.executionInfo.DecisionStartedID = di.StartedID
	e.executionInfo.DecisionRequestID = di.RequestID
	e.executionInfo.DecisionTimeout = di.DecisionTimeout
	e.executionInfo.DecisionAttempt = di.Attempt
	e.executionInfo.DecisionStartedTimestamp = di.StartedTimestamp
	e.executionInfo.DecisionScheduledTimestamp = di.ScheduledTimestamp
	e.executionInfo.DecisionOriginalScheduledTimestamp = di.OriginalScheduledTimestamp

	e.logger.Debug(fmt.Sprintf("Decision Updated: {Schedule: %v, Started: %v, ID: %v, Timeout: %v, Attempt: %v, Timestamp: %v}",
		di.ScheduleID, di.StartedID, di.RequestID, di.DecisionTimeout, di.Attempt, di.StartedTimestamp))
}

// DeleteDecision deletes a decision task.
func (e *mutableStateBuilder) DeleteDecision() {
	emptyDecisionInfo := &decisionInfo{
		Version:            common.EmptyVersion,
		ScheduleID:         common.EmptyEventID,
		StartedID:          common.EmptyEventID,
		RequestID:          emptyUUID,
		DecisionTimeout:    0,
		Attempt:            0,
		StartedTimestamp:   0,
		ScheduledTimestamp: 0,
	}
	e.UpdateDecision(emptyDecisionInfo)
}

func (e *mutableStateBuilder) FailDecision(incrementAttempt bool) {
	// Clear stickiness whenever decision fails
	e.ClearStickyness()

	failDecisionInfo := &decisionInfo{
		Version:          common.EmptyVersion,
		ScheduleID:       common.EmptyEventID,
		StartedID:        common.EmptyEventID,
		RequestID:        emptyUUID,
		DecisionTimeout:  0,
		StartedTimestamp: 0,
	}
	if incrementAttempt {
		failDecisionInfo.Attempt = e.executionInfo.DecisionAttempt + 1
		failDecisionInfo.ScheduledTimestamp = e.timeSource.Now().UnixNano()
	}
	e.UpdateDecision(failDecisionInfo)
}

func (e *mutableStateBuilder) ClearStickyness() {
	e.executionInfo.StickyTaskList = ""
	e.executionInfo.StickyScheduleToStartTimeout = 0
	e.executionInfo.ClientLibraryVersion = ""
	e.executionInfo.ClientFeatureVersion = ""
	e.executionInfo.ClientImpl = ""
}

// GetLastFirstEventID returns last first event ID
// first event ID is the ID of a batch of events in a single history events record
func (e *mutableStateBuilder) GetLastFirstEventID() int64 {
	return e.executionInfo.LastFirstEventID
}

// GetNextEventID returns next event ID
func (e *mutableStateBuilder) GetNextEventID() int64 {
	return e.executionInfo.NextEventID
}

// GetPreviousStartedEventID returns last started decision task event ID
func (e *mutableStateBuilder) GetPreviousStartedEventID() int64 {
	return e.executionInfo.LastProcessedEvent
}

func (e *mutableStateBuilder) IsWorkflowExecutionRunning() bool {
	return e.executionInfo.State != persistence.WorkflowStateCompleted
}

func (e *mutableStateBuilder) IsCancelRequested() (bool, string) {
	if e.executionInfo.CancelRequested {
		return e.executionInfo.CancelRequested, e.executionInfo.CancelRequestID
	}

	return false, ""
}

func (e *mutableStateBuilder) IsSignalRequested(requestID string) bool {
	if _, ok := e.pendingSignalRequestedIDs[requestID]; ok {
		return true
	}
	return false
}

func (e *mutableStateBuilder) AddSignalRequested(requestID string) {
	if e.pendingSignalRequestedIDs == nil {
		e.pendingSignalRequestedIDs = make(map[string]struct{})
	}
	if e.updateSignalRequestedIDs == nil {
		e.updateSignalRequestedIDs = make(map[string]struct{})
	}
	e.pendingSignalRequestedIDs[requestID] = struct{}{} // add requestID to set
	e.updateSignalRequestedIDs[requestID] = struct{}{}
}

func (e *mutableStateBuilder) DeleteSignalRequested(requestID string) {
	delete(e.pendingSignalRequestedIDs, requestID)
	e.deleteSignalRequestedID = requestID
}

func (e *mutableStateBuilder) addWorkflowExecutionStartedEventForContinueAsNew(
	domainEntry *cache.DomainCacheEntry,
	parentExecutionInfo *h.ParentExecutionInfo,
	execution workflow.WorkflowExecution,
	previousExecutionState mutableState,
	attributes *workflow.ContinueAsNewWorkflowExecutionDecisionAttributes,
	firstRunID string,
) (*workflow.HistoryEvent, error) {

	previousExecutionInfo := previousExecutionState.GetExecutionInfo()
	taskList := previousExecutionInfo.TaskList
	if attributes.TaskList != nil {
		taskList = attributes.TaskList.GetName()
	}
	tl := &workflow.TaskList{}
	tl.Name = common.StringPtr(taskList)

	workflowType := previousExecutionInfo.WorkflowTypeName
	if attributes.WorkflowType != nil {
		workflowType = attributes.WorkflowType.GetName()
	}
	wType := &workflow.WorkflowType{}
	wType.Name = common.StringPtr(workflowType)

	decisionTimeout := previousExecutionInfo.DecisionTimeoutValue
	if attributes.TaskStartToCloseTimeoutSeconds != nil {
		decisionTimeout = attributes.GetTaskStartToCloseTimeoutSeconds()
	}

	createRequest := &workflow.StartWorkflowExecutionRequest{
		RequestId:                           common.StringPtr(uuid.New()),
		Domain:                              common.StringPtr(domainEntry.GetInfo().Name),
		WorkflowId:                          execution.WorkflowId,
		TaskList:                            tl,
		WorkflowType:                        wType,
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(decisionTimeout),
		ExecutionStartToCloseTimeoutSeconds: attributes.ExecutionStartToCloseTimeoutSeconds,
		Input:                               attributes.Input,
		Header:                              attributes.Header,
		RetryPolicy:                         attributes.RetryPolicy,
		CronSchedule:                        attributes.CronSchedule,
		Memo:                                attributes.Memo,
		SearchAttributes:                    attributes.SearchAttributes,
	}

	req := &h.StartWorkflowExecutionRequest{
		DomainUUID:                      common.StringPtr(domainEntry.GetInfo().ID),
		StartRequest:                    createRequest,
		ParentExecutionInfo:             parentExecutionInfo,
		LastCompletionResult:            attributes.LastCompletionResult,
		ContinuedFailureReason:          attributes.FailureReason,
		ContinuedFailureDetails:         attributes.FailureDetails,
		ContinueAsNewInitiator:          attributes.Initiator,
		FirstDecisionTaskBackoffSeconds: attributes.BackoffStartIntervalInSeconds,
	}
	if attributes.GetInitiator() == workflow.ContinueAsNewInitiatorRetryPolicy {
		req.Attempt = common.Int32Ptr(previousExecutionState.GetExecutionInfo().Attempt + 1)
		expirationTime := previousExecutionState.GetExecutionInfo().ExpirationTime
		if !expirationTime.IsZero() {
			req.ExpirationTimestamp = common.Int64Ptr(expirationTime.UnixNano())
		}
	} else {
		// ContinueAsNew by decider or cron
		req.Attempt = common.Int32Ptr(0)
		if attributes.RetryPolicy != nil && attributes.RetryPolicy.GetExpirationIntervalInSeconds() > 0 {
			// has retry policy and expiration time.
			expirationSeconds := attributes.RetryPolicy.GetExpirationIntervalInSeconds() + req.GetFirstDecisionTaskBackoffSeconds()
			expirationTime := e.timeSource.Now().Add(time.Second * time.Duration(expirationSeconds))
			req.ExpirationTimestamp = common.Int64Ptr(expirationTime.UnixNano())
		}
	}

	// History event only has domainName so domainID has to be passed in explicitly to update the mutable state
	var parentDomainID *string
	if parentExecutionInfo != nil {
		parentDomainID = parentExecutionInfo.DomainUUID
	}

	event := e.hBuilder.AddWorkflowExecutionStartedEvent(req, previousExecutionInfo, firstRunID, execution.GetRunId())
	if err := e.ReplicateWorkflowExecutionStartedEvent(
		domainEntry,
		parentDomainID,
		execution,
		createRequest.GetRequestId(),
		event); err != nil {
		return nil, err
	}

	return event, nil
}

func (e *mutableStateBuilder) AddWorkflowExecutionStartedEvent(
	domainEntry *cache.DomainCacheEntry,
	execution workflow.WorkflowExecution,
	startRequest *h.StartWorkflowExecutionRequest,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowStarted
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	request := startRequest.StartRequest
	eventID := e.GetNextEventID()
	if eventID != common.FirstEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(eventID),
			tag.ErrorTypeInvalidHistoryAction)
		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddWorkflowExecutionStartedEvent(startRequest, nil, execution.GetRunId(), execution.GetRunId())

	var parentDomainID *string
	if startRequest.ParentExecutionInfo != nil {
		parentDomainID = startRequest.ParentExecutionInfo.DomainUUID
	}
	if err := e.ReplicateWorkflowExecutionStartedEvent(
		domainEntry,
		parentDomainID,
		execution,
		request.GetRequestId(),
		event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionStartedEvent(
	domainEntry *cache.DomainCacheEntry,
	parentDomainID *string,
	execution workflow.WorkflowExecution,
	requestID string,
	startEvent *workflow.HistoryEvent,
) error {

	event := startEvent.WorkflowExecutionStartedEventAttributes
	e.executionInfo.CreateRequestID = requestID
	e.executionInfo.DomainID = domainEntry.GetInfo().ID
	e.executionInfo.WorkflowID = execution.GetWorkflowId()
	e.executionInfo.RunID = execution.GetRunId()
	e.executionInfo.TaskList = event.TaskList.GetName()
	e.executionInfo.WorkflowTypeName = event.WorkflowType.GetName()
	e.executionInfo.WorkflowTimeout = event.GetExecutionStartToCloseTimeoutSeconds()
	e.executionInfo.DecisionTimeoutValue = event.GetTaskStartToCloseTimeoutSeconds()

	e.executionInfo.State = persistence.WorkflowStateCreated
	e.executionInfo.CloseStatus = persistence.WorkflowCloseStatusNone
	e.executionInfo.LastProcessedEvent = common.EmptyEventID
	e.executionInfo.LastFirstEventID = startEvent.GetEventId()

	e.executionInfo.DecisionVersion = common.EmptyVersion
	e.executionInfo.DecisionScheduleID = common.EmptyEventID
	e.executionInfo.DecisionStartedID = common.EmptyEventID
	e.executionInfo.DecisionRequestID = emptyUUID
	e.executionInfo.DecisionTimeout = 0

	e.executionInfo.CronSchedule = event.GetCronSchedule()

	if parentDomainID != nil {
		e.executionInfo.ParentDomainID = *parentDomainID
	}
	if event.ParentWorkflowExecution != nil {
		e.executionInfo.ParentWorkflowID = event.ParentWorkflowExecution.GetWorkflowId()
		e.executionInfo.ParentRunID = event.ParentWorkflowExecution.GetRunId()
	}
	if event.ParentInitiatedEventId != nil {
		e.executionInfo.InitiatedID = event.GetParentInitiatedEventId()
	} else {
		e.executionInfo.InitiatedID = common.EmptyEventID
	}

	e.executionInfo.Attempt = event.GetAttempt()
	if event.GetExpirationTimestamp() != 0 {
		e.executionInfo.ExpirationTime = time.Unix(0, event.GetExpirationTimestamp())
	}
	if event.RetryPolicy != nil {
		e.executionInfo.HasRetryPolicy = true
		e.executionInfo.BackoffCoefficient = event.RetryPolicy.GetBackoffCoefficient()
		e.executionInfo.ExpirationSeconds = event.RetryPolicy.GetExpirationIntervalInSeconds()
		e.executionInfo.InitialInterval = event.RetryPolicy.GetInitialIntervalInSeconds()
		e.executionInfo.MaximumAttempts = event.RetryPolicy.GetMaximumAttempts()
		e.executionInfo.MaximumInterval = event.RetryPolicy.GetMaximumIntervalInSeconds()
		e.executionInfo.NonRetriableErrors = event.RetryPolicy.NonRetriableErrorReasons
	}

	e.executionInfo.AutoResetPoints = rolloverAutoResetPointsWithExpiringTime(
		event.GetPrevAutoResetPoints(),
		event.GetContinuedExecutionRunId(),
		startEvent.GetTimestamp(),
		domainEntry.GetRetentionDays(e.executionInfo.WorkflowID),
	)

	if event.Memo != nil {
		e.executionInfo.Memo = event.Memo.GetFields()
	}
	if event.SearchAttributes != nil {
		e.executionInfo.SearchAttributes = event.SearchAttributes.GetIndexedFields()
	}

	e.writeEventToCache(startEvent)
	return nil
}

// originalScheduledTimestamp is to record the first scheduled decision during decision heartbeat.
func (e *mutableStateBuilder) AddDecisionTaskScheduledEventAsHeartbeat(
	originalScheduledTimestamp int64,
) (*decisionInfo, error) {
	opTag := tag.WorkflowActionDecisionTaskScheduled
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	if e.HasPendingDecisionTask() {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(e.executionInfo.DecisionScheduleID))
		return nil, e.createInternalServerError(opTag)
	}

	// set workflow state to running
	// since decision is scheduled
	e.executionInfo.State = persistence.WorkflowStateRunning

	// Tasklist and decision timeout should already be set from workflow execution started event
	taskList := e.executionInfo.TaskList
	if e.IsStickyTaskListEnabled() {
		taskList = e.executionInfo.StickyTaskList
	}
	startToCloseTimeoutSeconds := e.executionInfo.DecisionTimeoutValue

	// Flush any buffered events before creating the decision, otherwise it will result in invalid IDs for transient
	// decision and will cause in timeout processing to not work for transient decisions
	if e.HasBufferedEvents() {
		// if creating a decision and in the mean time events are flushed from buffered events
		// than this decision cannot be a transient decision
		e.executionInfo.DecisionAttempt = 0
		if err := e.FlushBufferedEvents(); err != nil {
			return nil, err
		}
	}

	var newDecisionEvent *workflow.HistoryEvent
	scheduleID := e.GetNextEventID() // we will generate the schedule event later for repeatedly failing decisions
	// Avoid creating new history events when decisions are continuously failing
	scheduleTime := e.timeSource.Now().UnixNano()
	if e.executionInfo.DecisionAttempt == 0 {
		newDecisionEvent = e.hBuilder.AddDecisionTaskScheduledEvent(taskList, startToCloseTimeoutSeconds,
			e.executionInfo.DecisionAttempt)
		scheduleID = newDecisionEvent.GetEventId()
		scheduleTime = newDecisionEvent.GetTimestamp()
	}

	return e.ReplicateDecisionTaskScheduledEvent(
		e.GetCurrentVersion(),
		scheduleID,
		taskList,
		startToCloseTimeoutSeconds,
		e.executionInfo.DecisionAttempt,
		scheduleTime,
		originalScheduledTimestamp,
	)
}

func (e *mutableStateBuilder) AddDecisionTaskScheduledEvent() (*decisionInfo, error) {
	return e.AddDecisionTaskScheduledEventAsHeartbeat(time.Now().UnixNano())
}

func (e *mutableStateBuilder) ReplicateTransientDecisionTaskScheduled() (*decisionInfo, error) {
	if e.HasPendingDecisionTask() || e.GetExecutionInfo().DecisionAttempt == 0 {
		return nil, nil
	}

	// the schedule ID for this decision is guaranteed to be wrong
	// since the next event ID is assigned at the very end of when
	// all events are applied for replication.
	// this is OK
	// 1. if a failover happen just after this transient decision,
	// AddDecisionTaskStartedEvent will handle the correction of schedule ID
	// and set the attempt to 0
	// 2. if no failover happen during the life time of this transient decision
	// then ReplicateDecisionTaskScheduledEvent will overwrite everything
	// including the decision schedule ID
	di := &decisionInfo{
		Version:            e.GetCurrentVersion(),
		ScheduleID:         e.GetNextEventID(),
		StartedID:          common.EmptyEventID,
		RequestID:          emptyUUID,
		DecisionTimeout:    e.GetExecutionInfo().DecisionTimeoutValue,
		TaskList:           e.GetExecutionInfo().TaskList,
		Attempt:            e.GetExecutionInfo().DecisionAttempt,
		ScheduledTimestamp: e.timeSource.Now().UnixNano(),
		StartedTimestamp:   0,
	}

	e.UpdateDecision(di)
	return di, nil
}

func (e *mutableStateBuilder) ReplicateDecisionTaskScheduledEvent(
	version int64,
	scheduleID int64,
	taskList string,
	startToCloseTimeoutSeconds int32,
	attempt int64,
	scheduleTimestamp int64,
	originalScheduledTimestamp int64,
) (*decisionInfo, error) {
	di := &decisionInfo{
		Version:                    version,
		ScheduleID:                 scheduleID,
		StartedID:                  common.EmptyEventID,
		RequestID:                  emptyUUID,
		DecisionTimeout:            startToCloseTimeoutSeconds,
		TaskList:                   taskList,
		Attempt:                    attempt,
		ScheduledTimestamp:         scheduleTimestamp,
		StartedTimestamp:           0,
		OriginalScheduledTimestamp: originalScheduledTimestamp,
	}

	e.UpdateDecision(di)
	return di, nil
}

func (e *mutableStateBuilder) AddDecisionTaskStartedEvent(
	scheduleEventID int64,
	requestID string,
	request *workflow.PollForDecisionTaskRequest,
) (*workflow.HistoryEvent, *decisionInfo, error) {

	opTag := tag.WorkflowActionDecisionTaskStarted
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, err
	}

	hasPendingDecision := e.HasPendingDecisionTask()
	di, ok := e.GetPendingDecision(scheduleEventID)
	if !hasPendingDecision || !ok || di.StartedID != common.EmptyEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(hasPendingDecision),
			tag.WorkflowScheduleID(scheduleEventID))
		return nil, nil, e.createInternalServerError(opTag)
	}

	var event *workflow.HistoryEvent
	scheduleID := di.ScheduleID
	startedID := scheduleID + 1
	tasklist := request.TaskList.GetName()
	timestamp := e.timeSource.Now().UnixNano()
	// First check to see if new events came since transient decision was scheduled
	if di.Attempt > 0 && di.ScheduleID != e.GetNextEventID() {
		// Also create a new DecisionTaskScheduledEvent since new events came in when it was scheduled
		scheduleEvent := e.hBuilder.AddDecisionTaskScheduledEvent(tasklist, di.DecisionTimeout, 0)
		scheduleID = scheduleEvent.GetEventId()
		di.Attempt = 0
	}

	// Avoid creating new history events when decisions are continuously failing
	if di.Attempt == 0 {
		// Now create DecisionTaskStartedEvent
		event = e.hBuilder.AddDecisionTaskStartedEvent(scheduleID, requestID, request.GetIdentity())
		startedID = event.GetEventId()
		timestamp = event.GetTimestamp()
	}

	di, err := e.ReplicateDecisionTaskStartedEvent(di, e.GetCurrentVersion(), scheduleID, startedID, requestID, timestamp)
	return event, di, err
}

func (e *mutableStateBuilder) ReplicateDecisionTaskStartedEvent(
	di *decisionInfo,
	version int64,
	scheduleID int64,
	startedID int64,
	requestID string,
	timestamp int64,
) (*decisionInfo, error) {
	// Replicator calls it with a nil decision info, and it is safe to always lookup the decision in this case as it
	// does not have to deal with transient decision case.
	var ok bool
	if di == nil {
		di, ok = e.GetPendingDecision(scheduleID)
		if !ok {
			return nil, errors.NewInternalFailureError(fmt.Sprintf("unable to find decision: %v", scheduleID))
		}
		// setting decision attempt to 0 for decision task replication
		// this mainly handles transient decision completion
		// for transient decision, active side will write 2 batch in a "transaction"
		// 1. decision task scheduled & decision task started
		// 2. decision task completed & other events
		// since we need to treat each individual event batch as one transaction
		// certain "magic" needs to be done, i.e. setting attempt to 0 so
		// if first batch is replicated, but not the second one, decision can be correctly timed out
		di.Attempt = 0
	}

	e.executionInfo.State = persistence.WorkflowStateRunning
	// Update mutable decision state
	di = &decisionInfo{
		Version:            version,
		ScheduleID:         scheduleID,
		StartedID:          startedID,
		RequestID:          requestID,
		DecisionTimeout:    di.DecisionTimeout,
		Attempt:            di.Attempt,
		StartedTimestamp:   timestamp,
		ScheduledTimestamp: di.ScheduledTimestamp,
	}

	e.UpdateDecision(di)
	return di, nil
}

func (e *mutableStateBuilder) CreateTransientDecisionEvents(
	di *decisionInfo,
	identity string,
) (*workflow.HistoryEvent, *workflow.HistoryEvent) {
	tasklist := e.executionInfo.TaskList
	scheduledEvent := newDecisionTaskScheduledEventWithInfo(di.ScheduleID, di.ScheduledTimestamp, tasklist, di.DecisionTimeout,
		di.Attempt)
	startedEvent := newDecisionTaskStartedEventWithInfo(di.StartedID, di.StartedTimestamp, di.ScheduleID, di.RequestID,
		identity)

	return scheduledEvent, startedEvent
}

func (e *mutableStateBuilder) beforeAddDecisionTaskCompletedEvent() {
	// Make sure to delete decision before adding events.  Otherwise they are buffered rather than getting appended
	e.DeleteDecision()
}

func (e *mutableStateBuilder) afterAddDecisionTaskCompletedEvent(
	event *workflow.HistoryEvent,
	maxResetPoints int,
) {
	e.executionInfo.LastProcessedEvent = event.GetDecisionTaskCompletedEventAttributes().GetStartedEventId()
	e.addBinaryCheckSumIfNotExists(event, maxResetPoints)
}

// add BinaryCheckSum for the first decisionTaskCompletedID for auto-reset
func (e *mutableStateBuilder) addBinaryCheckSumIfNotExists(
	event *workflow.HistoryEvent,
	maxResetPoints int,
) {
	binChecksum := event.GetDecisionTaskCompletedEventAttributes().GetBinaryChecksum()
	if len(binChecksum) == 0 {
		return
	}
	exeInfo := e.executionInfo
	var currResetPoints []*workflow.ResetPointInfo
	if exeInfo.AutoResetPoints != nil && exeInfo.AutoResetPoints.Points != nil {
		currResetPoints = e.executionInfo.AutoResetPoints.Points
	} else {
		currResetPoints = make([]*workflow.ResetPointInfo, 0, 1)
	}

	for _, rp := range currResetPoints {
		if rp.GetBinaryChecksum() == binChecksum {
			// this checksum already exists
			return
		}
	}

	if len(currResetPoints) == maxResetPoints {
		// if exceeding the max limit, do rotation by taking the oldest one out
		currResetPoints = currResetPoints[1:]
	}

	resettable := true
	err := e.CheckResettable()
	if err != nil {
		resettable = false
	}
	info := &workflow.ResetPointInfo{
		BinaryChecksum:           common.StringPtr(binChecksum),
		RunId:                    common.StringPtr(exeInfo.RunID),
		FirstDecisionCompletedId: common.Int64Ptr(event.GetEventId()),
		CreatedTimeNano:          common.Int64Ptr(e.timeSource.Now().UnixNano()),
		Resettable:               common.BoolPtr(resettable),
	}
	currResetPoints = append(currResetPoints, info)
	exeInfo.AutoResetPoints = &workflow.ResetPoints{
		Points: currResetPoints,
	}
}

// TODO: we will release the restriction when reset API allow those pending
func (e *mutableStateBuilder) CheckResettable() (retError error) {
	if e.GetEventStoreVersion() != persistence.EventStoreVersionV2 {
		retError = &workflow.BadRequestError{
			Message: fmt.Sprintf("reset API is not supported for V1 history events, runID"),
		}
		return
	}
	if len(e.GetPendingChildExecutionInfos()) > 0 {
		retError = &workflow.BadRequestError{
			Message: fmt.Sprintf("it is not allowed resetting to a point that workflow has pending child workflow."),
		}
		return
	}
	if len(e.GetAllRequestCancels()) > 0 {
		retError = &workflow.BadRequestError{
			Message: fmt.Sprintf("it is not allowed resetting to a point that workflow has pending request cancel."),
		}
		return
	}
	if len(e.GetAllSignalsToSend()) > 0 {
		retError = &workflow.BadRequestError{
			Message: fmt.Sprintf("it is not allowed resetting to a point that workflow has pending signals to send, pending signal: %+v ", e.GetAllSignalsToSend()),
		}
	}
	return nil
}

func (e *mutableStateBuilder) AddDecisionTaskCompletedEvent(
	scheduleEventID int64,
	startedEventID int64,
	request *workflow.RespondDecisionTaskCompletedRequest,
	maxResetPoints int,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionDecisionTaskCompleted
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	hasPendingDecision := e.HasPendingDecisionTask()
	di, ok := e.GetPendingDecision(scheduleEventID)
	if !hasPendingDecision || !ok || di.StartedID != startedEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(hasPendingDecision),
			tag.WorkflowScheduleID(scheduleEventID),
			tag.WorkflowStartedID(startedEventID))

		return nil, e.createInternalServerError(opTag)
	}

	e.beforeAddDecisionTaskCompletedEvent()
	if di.Attempt > 0 {
		// Create corresponding DecisionTaskSchedule and DecisionTaskStarted events for decisions we have been retrying
		scheduledEvent := e.hBuilder.AddTransientDecisionTaskScheduledEvent(e.executionInfo.TaskList, di.DecisionTimeout,
			di.Attempt, di.ScheduledTimestamp)
		startedEvent := e.hBuilder.AddTransientDecisionTaskStartedEvent(scheduledEvent.GetEventId(), di.RequestID,
			request.GetIdentity(), di.StartedTimestamp)
		startedEventID = startedEvent.GetEventId()
	}
	// Now write the completed event
	event := e.hBuilder.AddDecisionTaskCompletedEvent(scheduleEventID, startedEventID, request)

	e.afterAddDecisionTaskCompletedEvent(event, maxResetPoints)
	return event, nil
}

func (e *mutableStateBuilder) ReplicateDecisionTaskCompletedEvent(event *workflow.HistoryEvent) error {
	e.beforeAddDecisionTaskCompletedEvent()
	e.afterAddDecisionTaskCompletedEvent(event, math.MaxInt32)
	return nil
}

func (e *mutableStateBuilder) AddDecisionTaskTimedOutEvent(
	scheduleEventID int64,
	startedEventID int64,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionDecisionTaskTimedOut
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	hasPendingDecision := e.HasPendingDecisionTask()
	dt, ok := e.GetPendingDecision(scheduleEventID)
	if !hasPendingDecision || !ok || dt.StartedID != startedEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(hasPendingDecision),
			tag.WorkflowScheduleID(scheduleEventID),
			tag.WorkflowStartedID(startedEventID))
		return nil, e.createInternalServerError(opTag)
	}

	var event *workflow.HistoryEvent
	// Avoid creating new history events when decisions are continuously timing out
	if dt.Attempt == 0 {
		event = e.hBuilder.AddDecisionTaskTimedOutEvent(scheduleEventID, startedEventID, workflow.TimeoutTypeStartToClose)
	}

	if err := e.ReplicateDecisionTaskTimedOutEvent(workflow.TimeoutTypeStartToClose); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateDecisionTaskTimedOutEvent(timeoutType workflow.TimeoutType) error {
	incrementAttempt := true
	// Do not increment decision attempt in the case of sticky timeout to prevent creating next decision as transient
	if timeoutType == workflow.TimeoutTypeScheduleToStart {
		incrementAttempt = false
	}
	e.FailDecision(incrementAttempt)
	return nil
}

func (e *mutableStateBuilder) AddDecisionTaskScheduleToStartTimeoutEvent(
	scheduleEventID int64,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionDecisionTaskTimedOut
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	if e.executionInfo.DecisionScheduleID != scheduleEventID || e.executionInfo.DecisionStartedID > 0 {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(scheduleEventID),
		)
		return nil, e.createInternalServerError(opTag)
	}

	// Clear stickiness whenever decision fails
	e.ClearStickyness()

	event := e.hBuilder.AddDecisionTaskTimedOutEvent(scheduleEventID, 0, workflow.TimeoutTypeScheduleToStart)

	if err := e.ReplicateDecisionTaskTimedOutEvent(workflow.TimeoutTypeScheduleToStart); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) AddDecisionTaskFailedEvent(
	scheduleEventID int64,
	startedEventID int64,
	cause workflow.DecisionTaskFailedCause,
	details []byte,
	identity string,
	reason string,
	baseRunID string,
	newRunID string,
	forkEventVersion int64,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionDecisionTaskFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	attr := workflow.DecisionTaskFailedEventAttributes{
		ScheduledEventId: common.Int64Ptr(scheduleEventID),
		StartedEventId:   common.Int64Ptr(startedEventID),
		Cause:            common.DecisionTaskFailedCausePtr(cause),
		Details:          details,
		Identity:         common.StringPtr(identity),
		Reason:           common.StringPtr(reason),
		BaseRunId:        common.StringPtr(baseRunID),
		NewRunId:         common.StringPtr(newRunID),
		ForkEventVersion: common.Int64Ptr(forkEventVersion),
	}
	hasPendingDecision := e.HasPendingDecisionTask()

	dt, ok := e.GetPendingDecision(scheduleEventID)
	if !hasPendingDecision || !ok || dt.StartedID != startedEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(scheduleEventID),
			tag.WorkflowStartedID(startedEventID))
		return nil, e.createInternalServerError(opTag)
	}

	var event *workflow.HistoryEvent
	// Only emit DecisionTaskFailedEvent for the very first time
	if dt.Attempt == 0 || cause == workflow.DecisionTaskFailedCauseResetWorkflow {
		event = e.hBuilder.AddDecisionTaskFailedEvent(attr)
	}

	if err := e.ReplicateDecisionTaskFailedEvent(); err != nil {
		return nil, err
	}

	// always clear decision attempt for reset
	if cause == workflow.DecisionTaskFailedCauseResetWorkflow {
		e.executionInfo.DecisionAttempt = 0
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateDecisionTaskFailedEvent() error {
	e.FailDecision(true)
	return nil
}

func (e *mutableStateBuilder) AddActivityTaskScheduledEvent(
	decisionCompletedEventID int64,
	attributes *workflow.ScheduleActivityTaskDecisionAttributes,
) (*workflow.HistoryEvent, *persistence.ActivityInfo, error) {

	opTag := tag.WorkflowActionActivityTaskScheduled
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, err
	}

	ai, isActivityRunning := e.GetActivityByActivityID(attributes.GetActivityId())
	if isActivityRunning {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(ai.ScheduleID),
			tag.WorkflowStartedID(ai.StartedID),
			tag.Bool(isActivityRunning))
		return nil, nil, e.createCallerError(opTag)
	}

	event := e.hBuilder.AddActivityTaskScheduledEvent(decisionCompletedEventID, attributes)

	// Write the event to cache only on active cluster for processing on activity started or retried
	e.eventsCache.putEvent(e.executionInfo.DomainID, e.executionInfo.WorkflowID, e.executionInfo.RunID,
		event.GetEventId(), event)

	ai, err := e.ReplicateActivityTaskScheduledEvent(decisionCompletedEventID, event)
	return event, ai, err
}

func (e *mutableStateBuilder) ReplicateActivityTaskScheduledEvent(
	firstEventID int64,
	event *workflow.HistoryEvent,
) (*persistence.ActivityInfo, error) {

	attributes := event.ActivityTaskScheduledEventAttributes

	scheduleEventID := event.GetEventId()
	scheduleToCloseTimeout := attributes.GetScheduleToCloseTimeoutSeconds()

	ai := &persistence.ActivityInfo{
		Version:                  event.GetVersion(),
		ScheduleID:               scheduleEventID,
		ScheduledEventBatchID:    firstEventID,
		ScheduledTime:            time.Unix(0, *event.Timestamp),
		StartedID:                common.EmptyEventID,
		StartedTime:              time.Time{},
		ActivityID:               common.StringDefault(attributes.ActivityId),
		ScheduleToStartTimeout:   attributes.GetScheduleToStartTimeoutSeconds(),
		ScheduleToCloseTimeout:   scheduleToCloseTimeout,
		StartToCloseTimeout:      attributes.GetStartToCloseTimeoutSeconds(),
		HeartbeatTimeout:         attributes.GetHeartbeatTimeoutSeconds(),
		CancelRequested:          false,
		CancelRequestID:          common.EmptyEventID,
		LastHeartBeatUpdatedTime: time.Time{},
		TimerTaskStatus:          TimerTaskStatusNone,
		TaskList:                 attributes.TaskList.GetName(),
		HasRetryPolicy:           attributes.RetryPolicy != nil,
	}
	ai.ExpirationTime = ai.ScheduledTime.Add(time.Duration(scheduleToCloseTimeout) * time.Second)
	if ai.HasRetryPolicy {
		ai.InitialInterval = attributes.RetryPolicy.GetInitialIntervalInSeconds()
		ai.BackoffCoefficient = attributes.RetryPolicy.GetBackoffCoefficient()
		ai.MaximumInterval = attributes.RetryPolicy.GetMaximumIntervalInSeconds()
		ai.MaximumAttempts = attributes.RetryPolicy.GetMaximumAttempts()
		ai.NonRetriableErrors = attributes.RetryPolicy.NonRetriableErrorReasons
		if attributes.RetryPolicy.GetExpirationIntervalInSeconds() > scheduleToCloseTimeout {
			ai.ExpirationTime = ai.ScheduledTime.Add(time.Duration(attributes.RetryPolicy.GetExpirationIntervalInSeconds()) * time.Second)
		}
	}

	e.pendingActivityInfoIDs[scheduleEventID] = ai
	e.pendingActivityInfoByActivityID[ai.ActivityID] = scheduleEventID
	e.updateActivityInfos[ai] = struct{}{}

	return ai, nil
}

func (e *mutableStateBuilder) addTransientActivityStartedEvent(scheduleEventID int64) error {
	if ai, ok := e.GetActivityInfo(scheduleEventID); ok && ai.StartedID == common.TransientEventID {
		// activity task was started (as transient event), we need to add it now.
		event := e.hBuilder.AddActivityTaskStartedEvent(scheduleEventID, ai.Attempt, ai.RequestID, ai.StartedIdentity)
		if !ai.StartedTime.IsZero() {
			// overwrite started event time to the one recorded in ActivityInfo
			event.Timestamp = common.Int64Ptr(ai.StartedTime.UnixNano())
		}
		return e.ReplicateActivityTaskStartedEvent(event)
	}
	return nil
}

func (e *mutableStateBuilder) AddActivityTaskStartedEvent(
	ai *persistence.ActivityInfo,
	scheduleEventID int64,
	requestID string,
	identity string,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionActivityTaskStarted
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	if !ai.HasRetryPolicy {
		event := e.hBuilder.AddActivityTaskStartedEvent(scheduleEventID, ai.Attempt, requestID, identity)
		if err := e.ReplicateActivityTaskStartedEvent(event); err != nil {
			return nil, err
		}
		return event, nil
	}

	// we might need to retry, so do not append started event just yet,
	// instead update mutable state and will record started event when activity task is closed
	ai.Version = e.GetCurrentVersion()
	ai.StartedID = common.TransientEventID
	ai.RequestID = requestID
	ai.StartedTime = e.timeSource.Now()
	ai.LastHeartBeatUpdatedTime = ai.StartedTime
	ai.StartedIdentity = identity
	if err := e.UpdateActivity(ai); err != nil {
		return nil, err
	}
	e.syncActivityTasks[ai.ScheduleID] = struct{}{}
	return nil, nil
}

func (e *mutableStateBuilder) ReplicateActivityTaskStartedEvent(event *workflow.HistoryEvent) error {
	attributes := event.ActivityTaskStartedEventAttributes
	scheduleID := attributes.GetScheduledEventId()
	ai, _ := e.GetActivityInfo(scheduleID)

	ai.Version = event.GetVersion()
	ai.StartedID = event.GetEventId()
	ai.RequestID = attributes.GetRequestId()
	ai.StartedTime = time.Unix(0, event.GetTimestamp())
	ai.LastHeartBeatUpdatedTime = ai.StartedTime
	e.updateActivityInfos[ai] = struct{}{}
	return nil
}

func (e *mutableStateBuilder) AddActivityTaskCompletedEvent(
	scheduleEventID,
	startedEventID int64,
	request *workflow.RespondActivityTaskCompletedRequest,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionActivityTaskCompleted
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	if ai, ok := e.GetActivityInfo(scheduleEventID); !ok || ai.StartedID != startedEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowScheduleID(scheduleEventID),
			tag.WorkflowStartedID(startedEventID))
		return nil, e.createInternalServerError(opTag)
	}

	if err := e.addTransientActivityStartedEvent(scheduleEventID); err != nil {
		return nil, err
	}
	event := e.hBuilder.AddActivityTaskCompletedEvent(scheduleEventID, startedEventID, request)
	if err := e.ReplicateActivityTaskCompletedEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}

func (e *mutableStateBuilder) ReplicateActivityTaskCompletedEvent(event *workflow.HistoryEvent) error {
	attributes := event.ActivityTaskCompletedEventAttributes
	scheduleID := attributes.GetScheduledEventId()

	return e.DeleteActivity(scheduleID)
}

func (e *mutableStateBuilder) AddActivityTaskFailedEvent(
	scheduleEventID int64,
	startedEventID int64,
	request *workflow.RespondActivityTaskFailedRequest,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionActivityTaskFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	if ai, ok := e.GetActivityInfo(scheduleEventID); !ok || ai.StartedID != startedEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowScheduleID(scheduleEventID),
			tag.WorkflowStartedID(startedEventID))
		return nil, e.createInternalServerError(opTag)
	}

	if err := e.addTransientActivityStartedEvent(scheduleEventID); err != nil {
		return nil, err
	}
	event := e.hBuilder.AddActivityTaskFailedEvent(scheduleEventID, startedEventID, request)
	if err := e.ReplicateActivityTaskFailedEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}

func (e *mutableStateBuilder) ReplicateActivityTaskFailedEvent(event *workflow.HistoryEvent) error {
	attributes := event.ActivityTaskFailedEventAttributes
	scheduleID := attributes.GetScheduledEventId()

	return e.DeleteActivity(scheduleID)
}

func (e *mutableStateBuilder) AddActivityTaskTimedOutEvent(
	scheduleEventID int64,
	startedEventID int64,
	timeoutType workflow.TimeoutType,
	lastHeartBeatDetails []byte,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionActivityTaskTimedOut
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	if ai, ok := e.GetActivityInfo(scheduleEventID); !ok ||
		ai.StartedID != startedEventID ||
		((timeoutType == workflow.TimeoutTypeStartToClose || timeoutType == workflow.TimeoutTypeHeartbeat) && ai.StartedID == common.EmptyEventID) {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowScheduleID(ai.ScheduleID),
			tag.WorkflowStartedID(ai.StartedID),
			tag.WorkflowTimeoutType(int64(timeoutType)))
		return nil, e.createInternalServerError(opTag)
	}

	if err := e.addTransientActivityStartedEvent(scheduleEventID); err != nil {
		return nil, err
	}
	event := e.hBuilder.AddActivityTaskTimedOutEvent(scheduleEventID, startedEventID, timeoutType, lastHeartBeatDetails)
	if err := e.ReplicateActivityTaskTimedOutEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}

func (e *mutableStateBuilder) ReplicateActivityTaskTimedOutEvent(event *workflow.HistoryEvent) error {
	attributes := event.ActivityTaskTimedOutEventAttributes
	scheduleID := attributes.GetScheduledEventId()

	return e.DeleteActivity(scheduleID)
}

func (e *mutableStateBuilder) AddActivityTaskCancelRequestedEvent(
	decisionCompletedEventID int64,
	activityID string,
	identity string,
) (*workflow.HistoryEvent, *persistence.ActivityInfo, error) {

	opTag := tag.WorkflowActionActivityTaskCancelRequested
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, err
	}

	// we need to add the cancel request event even if activity not in mutable state
	// if activity not in mutable state or already cancel requested,
	// we do not need to call the replication function
	actCancelReqEvent := e.hBuilder.AddActivityTaskCancelRequestedEvent(decisionCompletedEventID, activityID)

	ai, isRunning := e.GetActivityByActivityID(activityID)
	if !isRunning || ai.CancelRequested {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(isRunning),
			tag.WorkflowActivityID(activityID))

		return nil, nil, e.createCallerError(opTag)
	}

	if err := e.ReplicateActivityTaskCancelRequestedEvent(actCancelReqEvent); err != nil {
		return nil, nil, err
	}

	return actCancelReqEvent, ai, nil
}

func (e *mutableStateBuilder) ReplicateActivityTaskCancelRequestedEvent(event *workflow.HistoryEvent) error {
	attributes := event.ActivityTaskCancelRequestedEventAttributes
	activityID := attributes.GetActivityId()
	ai, ok := e.GetActivityByActivityID(activityID)
	if !ok {
		return errors.NewInternalFailureError(fmt.Sprintf("unable to find activity: %v", activityID))
	}

	ai.Version = event.GetVersion()

	// - We have the activity dispatched to worker.
	// - The activity might not be heartbeat'ing, but the activity can still call RecordActivityHeartBeat()
	//   to see cancellation while reporting progress of the activity.
	ai.CancelRequested = true

	ai.CancelRequestID = event.GetEventId()
	e.updateActivityInfos[ai] = struct{}{}
	return nil
}

func (e *mutableStateBuilder) AddRequestCancelActivityTaskFailedEvent(
	decisionCompletedEventID int64,
	activityID string,
	cause string,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionActivityTaskCancelFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	return e.hBuilder.AddRequestCancelActivityTaskFailedEvent(decisionCompletedEventID, activityID, cause), nil
}

func (e *mutableStateBuilder) AddActivityTaskCanceledEvent(
	scheduleEventID int64,
	startedEventID int64,
	latestCancelRequestedEventID int64,
	details []byte,
	identity string,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionActivityTaskCanceled
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ai, ok := e.GetActivityInfo(scheduleEventID)
	if !ok || ai.StartedID != startedEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(scheduleEventID))
		return nil, e.createInternalServerError(opTag)
	}

	// Verify cancel request as well.
	if !ai.CancelRequested {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(scheduleEventID),
			tag.WorkflowActivityID(ai.ActivityID),
			tag.WorkflowStartedID(ai.StartedID))
		return nil, e.createInternalServerError(opTag)
	}

	if err := e.addTransientActivityStartedEvent(scheduleEventID); err != nil {
		return nil, err
	}
	event := e.hBuilder.AddActivityTaskCanceledEvent(scheduleEventID, startedEventID, latestCancelRequestedEventID,
		details, identity)
	if err := e.ReplicateActivityTaskCanceledEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}

func (e *mutableStateBuilder) ReplicateActivityTaskCanceledEvent(event *workflow.HistoryEvent) error {
	attributes := event.ActivityTaskCanceledEventAttributes
	scheduleID := attributes.GetScheduledEventId()

	return e.DeleteActivity(scheduleID)
}

func (e *mutableStateBuilder) AddCompletedWorkflowEvent(
	decisionCompletedEventID int64,
	attributes *workflow.CompleteWorkflowExecutionDecisionAttributes,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowCompleted
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	event := e.hBuilder.AddCompletedWorkflowEvent(decisionCompletedEventID, attributes)
	if err := e.ReplicateWorkflowExecutionCompletedEvent(decisionCompletedEventID, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionCompletedEvent(
	firstEventID int64,
	event *workflow.HistoryEvent,
) error {

	e.executionInfo.State = persistence.WorkflowStateCompleted
	e.executionInfo.CloseStatus = persistence.WorkflowCloseStatusCompleted
	e.executionInfo.CompletionEventBatchID = firstEventID // Used when completion event needs to be loaded from database
	e.ClearStickyness()
	e.writeEventToCache(event)
	return nil
}

func (e *mutableStateBuilder) AddFailWorkflowEvent(
	decisionCompletedEventID int64,
	attributes *workflow.FailWorkflowExecutionDecisionAttributes,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	event := e.hBuilder.AddFailWorkflowEvent(decisionCompletedEventID, attributes)
	if err := e.ReplicateWorkflowExecutionFailedEvent(decisionCompletedEventID, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionFailedEvent(
	firstEventID int64,
	event *workflow.HistoryEvent,
) error {

	e.executionInfo.State = persistence.WorkflowStateCompleted
	e.executionInfo.CloseStatus = persistence.WorkflowCloseStatusFailed
	e.executionInfo.CompletionEventBatchID = firstEventID // Used when completion event needs to be loaded from database
	e.ClearStickyness()
	e.writeEventToCache(event)
	return nil
}

func (e *mutableStateBuilder) AddTimeoutWorkflowEvent() (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowTimeout
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	event := e.hBuilder.AddTimeoutWorkflowEvent()
	if err := e.ReplicateWorkflowExecutionTimedoutEvent(event.GetEventId(), event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionTimedoutEvent(
	firstEventID int64,
	event *workflow.HistoryEvent,
) error {

	e.executionInfo.State = persistence.WorkflowStateCompleted
	e.executionInfo.CloseStatus = persistence.WorkflowCloseStatusTimedOut
	e.executionInfo.CompletionEventBatchID = firstEventID // Used when completion event needs to be loaded from database
	e.ClearStickyness()
	e.writeEventToCache(event)
	return nil
}

func (e *mutableStateBuilder) AddWorkflowExecutionCancelRequestedEvent(
	cause string,
	request *h.RequestCancelWorkflowExecutionRequest,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowCancelRequested
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	if e.executionInfo.CancelRequested {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowState(e.executionInfo.State),
			tag.Bool(e.executionInfo.CancelRequested),
			tag.Key(e.executionInfo.CancelRequestID),
		)
		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddWorkflowExecutionCancelRequestedEvent(cause, request)
	if err := e.ReplicateWorkflowExecutionCancelRequestedEvent(event); err != nil {
		return nil, err
	}

	// Set the CancelRequestID on the active cluster.  This information is not part of the history event.
	e.executionInfo.CancelRequestID = request.CancelRequest.GetRequestId()
	return event, nil
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionCancelRequestedEvent(event *workflow.HistoryEvent) error {
	e.executionInfo.CancelRequested = true
	return nil
}

func (e *mutableStateBuilder) AddWorkflowExecutionCanceledEvent(
	decisionTaskCompletedEventID int64,
	attributes *workflow.CancelWorkflowExecutionDecisionAttributes,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowCanceled
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	event := e.hBuilder.AddWorkflowExecutionCanceledEvent(decisionTaskCompletedEventID, attributes)
	if err := e.ReplicateWorkflowExecutionCanceledEvent(decisionTaskCompletedEventID, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionCanceledEvent(
	firstEventID int64,
	event *workflow.HistoryEvent,
) error {
	e.executionInfo.State = persistence.WorkflowStateCompleted
	e.executionInfo.CloseStatus = persistence.WorkflowCloseStatusCanceled
	e.executionInfo.CompletionEventBatchID = firstEventID // Used when completion event needs to be loaded from database
	e.ClearStickyness()
	e.writeEventToCache(event)
	return nil
}

func (e *mutableStateBuilder) AddRequestCancelExternalWorkflowExecutionInitiatedEvent(
	decisionCompletedEventID int64,
	cancelRequestID string,
	request *workflow.RequestCancelExternalWorkflowExecutionDecisionAttributes,
) (*workflow.HistoryEvent, *persistence.RequestCancelInfo, error) {

	opTag := tag.WorkflowActionExternalWorkflowCancelInitiated
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, err
	}

	event := e.hBuilder.AddRequestCancelExternalWorkflowExecutionInitiatedEvent(decisionCompletedEventID, request)
	rci, err := e.ReplicateRequestCancelExternalWorkflowExecutionInitiatedEvent(event, cancelRequestID)
	if err != nil {
		return nil, nil, err
	}
	return event, rci, nil
}

func (e *mutableStateBuilder) ReplicateRequestCancelExternalWorkflowExecutionInitiatedEvent(
	event *workflow.HistoryEvent,
	cancelRequestID string,
) (*persistence.RequestCancelInfo, error) {

	// TODO: Evaluate if we need cancelRequestID also part of history event
	initiatedEventID := event.GetEventId()
	rci := &persistence.RequestCancelInfo{
		Version:         event.GetVersion(),
		InitiatedID:     initiatedEventID,
		CancelRequestID: cancelRequestID,
	}

	e.pendingRequestCancelInfoIDs[initiatedEventID] = rci
	e.updateRequestCancelInfos[rci] = struct{}{}

	return rci, nil
}

func (e *mutableStateBuilder) AddExternalWorkflowExecutionCancelRequested(
	initiatedID int64,
	domain string,
	workflowID string,
	runID string,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionExternalWorkflowCancelRequested
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	_, ok := e.GetRequestCancelInfo(initiatedID)
	if !ok {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowInitiatedID(initiatedID),
			tag.Bool(ok))
		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddExternalWorkflowExecutionCancelRequested(initiatedID, domain, workflowID, runID)
	if err := e.ReplicateExternalWorkflowExecutionCancelRequested(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateExternalWorkflowExecutionCancelRequested(event *workflow.HistoryEvent) error {
	initiatedID := event.ExternalWorkflowExecutionCancelRequestedEventAttributes.GetInitiatedEventId()
	e.DeletePendingRequestCancel(initiatedID)
	return nil
}

func (e *mutableStateBuilder) AddRequestCancelExternalWorkflowExecutionFailedEvent(
	decisionTaskCompletedEventID int64,
	initiatedID int64,
	domain string,
	workflowID string,
	runID string,
	cause workflow.CancelExternalWorkflowExecutionFailedCause,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionExternalWorkflowCancelFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	_, ok := e.GetRequestCancelInfo(initiatedID)
	if !ok {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowInitiatedID(initiatedID),
			tag.Bool(ok))

		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddRequestCancelExternalWorkflowExecutionFailedEvent(decisionTaskCompletedEventID, initiatedID,
		domain, workflowID, runID, cause)
	if err := e.ReplicateRequestCancelExternalWorkflowExecutionFailedEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateRequestCancelExternalWorkflowExecutionFailedEvent(event *workflow.HistoryEvent) error {
	initiatedID := event.RequestCancelExternalWorkflowExecutionFailedEventAttributes.GetInitiatedEventId()
	e.DeletePendingRequestCancel(initiatedID)
	return nil
}

func (e *mutableStateBuilder) AddSignalExternalWorkflowExecutionInitiatedEvent(
	decisionCompletedEventID int64,
	signalRequestID string,
	request *workflow.SignalExternalWorkflowExecutionDecisionAttributes,
) (*workflow.HistoryEvent, *persistence.SignalInfo, error) {

	opTag := tag.WorkflowActionExternalWorkflowSignalInitiated
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, err
	}

	event := e.hBuilder.AddSignalExternalWorkflowExecutionInitiatedEvent(decisionCompletedEventID, request)
	si, err := e.ReplicateSignalExternalWorkflowExecutionInitiatedEvent(event, signalRequestID)
	if err != nil {
		return nil, nil, err
	}
	return event, si, nil
}

func (e *mutableStateBuilder) ReplicateSignalExternalWorkflowExecutionInitiatedEvent(
	event *workflow.HistoryEvent,
	signalRequestID string,
) (*persistence.SignalInfo, error) {

	// TODO: Consider also writing signalRequestID to history event
	initiatedEventID := event.GetEventId()
	attributes := event.SignalExternalWorkflowExecutionInitiatedEventAttributes
	si := &persistence.SignalInfo{
		Version:         event.GetVersion(),
		InitiatedID:     initiatedEventID,
		SignalRequestID: signalRequestID,
		SignalName:      attributes.GetSignalName(),
		Input:           attributes.Input,
		Control:         attributes.Control,
	}

	e.pendingSignalInfoIDs[initiatedEventID] = si
	e.updateSignalInfos[si] = struct{}{}
	return si, nil
}

func (e *mutableStateBuilder) AddUpsertWorkflowSearchAttributesEvent(
	decisionCompletedEventID int64,
	request *workflow.UpsertWorkflowSearchAttributesDecisionAttributes,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionUpsertWorkflowSearchAttributes
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	event := e.hBuilder.AddUpsertWorkflowSearchAttributesEvent(decisionCompletedEventID, request)
	e.ReplicateUpsertWorkflowSearchAttributesEvent(event)
	return event, nil
}

func (e *mutableStateBuilder) ReplicateUpsertWorkflowSearchAttributesEvent(
	event *workflow.HistoryEvent) {
	upsertSearchAttr := event.UpsertWorkflowSearchAttributesEventAttributes.GetSearchAttributes().GetIndexedFields()
	currentSearchAttr := e.GetExecutionInfo().SearchAttributes

	e.executionInfo.SearchAttributes = mergeMapOfByteArray(currentSearchAttr, upsertSearchAttr)
}

func mergeMapOfByteArray(current, upsert map[string][]byte) map[string][]byte {
	if current == nil {
		current = make(map[string][]byte)
	}
	for k, v := range upsert {
		current[k] = v
	}
	return current
}

func (e *mutableStateBuilder) AddExternalWorkflowExecutionSignaled(
	initiatedID int64,
	domain string,
	workflowID string,
	runID string,
	control []byte,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionExternalWorkflowSignalRequested
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	_, ok := e.GetSignalInfo(initiatedID)
	if !ok {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowInitiatedID(initiatedID),
			tag.Bool(ok))
		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddExternalWorkflowExecutionSignaled(initiatedID, domain, workflowID, runID, control)
	if err := e.ReplicateExternalWorkflowExecutionSignaled(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateExternalWorkflowExecutionSignaled(event *workflow.HistoryEvent) error {
	initiatedID := event.ExternalWorkflowExecutionSignaledEventAttributes.GetInitiatedEventId()
	e.DeletePendingSignal(initiatedID)
	return nil
}

func (e *mutableStateBuilder) AddSignalExternalWorkflowExecutionFailedEvent(
	decisionTaskCompletedEventID int64,
	initiatedID int64,
	domain string,
	workflowID string,
	runID string,
	control []byte,
	cause workflow.SignalExternalWorkflowExecutionFailedCause,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionExternalWorkflowSignalFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	_, ok := e.GetSignalInfo(initiatedID)
	if !ok {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowInitiatedID(initiatedID),
			tag.Bool(ok))

		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddSignalExternalWorkflowExecutionFailedEvent(decisionTaskCompletedEventID, initiatedID, domain,
		workflowID, runID, control, cause)
	if err := e.ReplicateSignalExternalWorkflowExecutionFailedEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateSignalExternalWorkflowExecutionFailedEvent(event *workflow.HistoryEvent) error {
	initiatedID := event.SignalExternalWorkflowExecutionFailedEventAttributes.GetInitiatedEventId()
	e.DeletePendingSignal(initiatedID)
	return nil
}

func (e *mutableStateBuilder) AddTimerStartedEvent(
	decisionCompletedEventID int64,
	request *workflow.StartTimerDecisionAttributes,
) (*workflow.HistoryEvent, *persistence.TimerInfo, error) {

	opTag := tag.WorkflowActionTimerStarted
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, err
	}

	timerID := request.GetTimerId()
	isTimerRunning, ti := e.GetUserTimer(timerID)
	if isTimerRunning {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowStartedID(ti.StartedID),
			tag.Bool(isTimerRunning))
		return nil, nil, e.createCallerError(opTag)
	}

	event := e.hBuilder.AddTimerStartedEvent(decisionCompletedEventID, request)
	ti, err := e.ReplicateTimerStartedEvent(event)
	if err != nil {
		return nil, nil, err
	}
	return event, ti, err
}

func (e *mutableStateBuilder) ReplicateTimerStartedEvent(
	event *workflow.HistoryEvent,
) (*persistence.TimerInfo, error) {

	attributes := event.TimerStartedEventAttributes
	timerID := attributes.GetTimerId()

	startToFireTimeout := attributes.GetStartToFireTimeoutSeconds()
	fireTimeout := time.Duration(startToFireTimeout) * time.Second
	// TODO: Time skew need to be taken in to account.
	expiryTime := time.Unix(0, event.GetTimestamp()).Add(fireTimeout) // should use the event time, not now
	ti := &persistence.TimerInfo{
		Version:    event.GetVersion(),
		TimerID:    timerID,
		ExpiryTime: expiryTime,
		StartedID:  event.GetEventId(),
		TaskID:     TimerTaskStatusNone,
	}

	e.pendingTimerInfoIDs[timerID] = ti
	e.updateTimerInfos[ti] = struct{}{}

	return ti, nil
}

func (e *mutableStateBuilder) AddTimerFiredEvent(
	startedEventID int64,
	timerID string,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionTimerFired
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	isTimerRunning, _ := e.GetUserTimer(timerID)
	if !isTimerRunning {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowStartedID(startedEventID),
			tag.Bool(isTimerRunning),
			tag.WorkflowTimerID(timerID))
		return nil, e.createInternalServerError(opTag)
	}

	// Timer is running.
	event := e.hBuilder.AddTimerFiredEvent(startedEventID, timerID)
	if err := e.ReplicateTimerFiredEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateTimerFiredEvent(event *workflow.HistoryEvent) error {
	attributes := event.TimerFiredEventAttributes
	timerID := attributes.GetTimerId()

	e.DeleteUserTimer(timerID)
	return nil
}

func (e *mutableStateBuilder) AddTimerCanceledEvent(
	decisionCompletedEventID int64,
	attributes *workflow.CancelTimerDecisionAttributes,
	identity string,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionTimerCanceled
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	var timerStartedID int64
	timerID := attributes.GetTimerId()
	isTimerRunning, ti := e.GetUserTimer(timerID)
	if !isTimerRunning {
		// if timer is not running then check if it has fired in the mutable state.
		// If so clear the timer from the mutable state. We need to check both the
		// bufferedEvents and the history builder
		timerFiredEvent := e.checkAndClearTimerFiredEvent(timerID)
		if timerFiredEvent == nil {
			e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
				tag.WorkflowEventID(e.GetNextEventID()),
				tag.ErrorTypeInvalidHistoryAction,
				tag.Bool(isTimerRunning),
				tag.WorkflowTimerID(timerID))
			return nil, e.createCallerError(opTag)
		}
		timerStartedID = timerFiredEvent.TimerFiredEventAttributes.GetStartedEventId()
	} else {
		timerStartedID = ti.StartedID
	}

	// Timer is running.
	event := e.hBuilder.AddTimerCanceledEvent(timerStartedID, decisionCompletedEventID, timerID, identity)
	if isTimerRunning {
		if err := e.ReplicateTimerCanceledEvent(event); err != nil {
			return nil, err
		}
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateTimerCanceledEvent(event *workflow.HistoryEvent) error {
	attributes := event.TimerCanceledEventAttributes
	timerID := attributes.GetTimerId()

	e.DeleteUserTimer(timerID)
	return nil
}

func (e *mutableStateBuilder) AddCancelTimerFailedEvent(
	decisionCompletedEventID int64,
	attributes *workflow.CancelTimerDecisionAttributes,
	identity string,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionTimerCancelFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	// No Operation: We couldn't cancel it probably TIMER_ID_UNKNOWN
	timerID := attributes.GetTimerId()
	return e.hBuilder.AddCancelTimerFailedEvent(timerID, decisionCompletedEventID,
		timerCancellationMsgTimerIDUnknown, identity), nil
}

func (e *mutableStateBuilder) AddRecordMarkerEvent(
	decisionCompletedEventID int64,
	attributes *workflow.RecordMarkerDecisionAttributes,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowRecordMarker
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	return e.hBuilder.AddMarkerRecordedEvent(decisionCompletedEventID, attributes), nil
}

func (e *mutableStateBuilder) AddWorkflowExecutionTerminatedEvent(
	reason string, details []byte, identity string,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowTerminated
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	event := e.hBuilder.AddWorkflowExecutionTerminatedEvent(reason, details, identity)
	if err := e.ReplicateWorkflowExecutionTerminatedEvent(event.GetEventId(), event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionTerminatedEvent(
	firstEventID int64,
	event *workflow.HistoryEvent,
) error {

	e.executionInfo.State = persistence.WorkflowStateCompleted
	e.executionInfo.CloseStatus = persistence.WorkflowCloseStatusTerminated
	e.executionInfo.CompletionEventBatchID = firstEventID // Used when completion event needs to be loaded from database
	e.ClearStickyness()
	e.writeEventToCache(event)
	return nil
}

func (e *mutableStateBuilder) AddWorkflowExecutionSignaled(
	signalName string,
	input []byte,
	identity string,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowSignaled
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	event := e.hBuilder.AddWorkflowExecutionSignaledEvent(signalName, input, identity)
	if err := e.ReplicateWorkflowExecutionSignaled(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionSignaled(event *workflow.HistoryEvent) error {
	// Increment signal count in mutable state for this workflow execution
	e.executionInfo.SignalCount++
	return nil
}

func (e *mutableStateBuilder) AddContinueAsNewEvent(
	firstEventID int64,
	decisionCompletedEventID int64,
	domainEntry *cache.DomainCacheEntry,
	parentDomainName string,
	attributes *workflow.ContinueAsNewWorkflowExecutionDecisionAttributes,
	eventStoreVersion int32,
) (*workflow.HistoryEvent, mutableState, error) {

	opTag := tag.WorkflowActionWorkflowContinueAsNew
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, err
	}

	var err error
	newRunID := uuid.New()
	newExecution := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr(e.executionInfo.WorkflowID),
		RunId:      common.StringPtr(newRunID),
	}

	// Extract ParentExecutionInfo from current run so it can be passed down to the next
	var parentInfo *h.ParentExecutionInfo
	if e.HasParentExecution() {
		parentInfo = &h.ParentExecutionInfo{
			DomainUUID: common.StringPtr(e.executionInfo.ParentDomainID),
			Domain:     common.StringPtr(parentDomainName),
			Execution: &workflow.WorkflowExecution{
				WorkflowId: common.StringPtr(e.executionInfo.ParentWorkflowID),
				RunId:      common.StringPtr(e.executionInfo.ParentRunID),
			},
			InitiatedId: common.Int64Ptr(e.executionInfo.InitiatedID),
		}
	}

	continueAsNewEvent := e.hBuilder.AddContinuedAsNewEvent(decisionCompletedEventID, newRunID, attributes)
	currentStartEvent, found := e.GetStartEvent()
	if !found {
		return nil, nil, &workflow.InternalServiceError{Message: "Failed to load start event."}
	}
	firstRunID := currentStartEvent.GetWorkflowExecutionStartedEventAttributes().GetFirstExecutionRunId()

	var newStateBuilder *mutableStateBuilder
	if e.GetReplicationState() != nil {
		// continued as new workflow should have the same replication properties
		newStateBuilder = newMutableStateBuilderWithReplicationState(
			e.shard,
			e.eventsCache,
			e.logger,
			e.GetCurrentVersion(),
			e.replicationPolicy,
			e.domainName,
		)
	} else {
		newStateBuilder = newMutableStateBuilder(e.shard, e.eventsCache, e.logger, e.domainName)
	}
	domainID := domainEntry.GetInfo().ID
	startedEvent, err := newStateBuilder.addWorkflowExecutionStartedEventForContinueAsNew(domainEntry, parentInfo, newExecution, e, attributes, firstRunID)
	if err != nil {
		return nil, nil, &workflow.InternalServiceError{Message: "Failed to add workflow execution started event."}
	}

	var di *decisionInfo
	// First decision for retry will be created by a backoff timer
	if attributes.GetBackoffStartIntervalInSeconds() == 0 {
		di, err = newStateBuilder.AddDecisionTaskScheduledEvent()
		if err != nil {
			return nil, nil, &workflow.InternalServiceError{Message: "Failed to add decision started event."}
		}
	}

	if err = e.ReplicateWorkflowExecutionContinuedAsNewEvent(
		firstEventID,
		domainID,
		continueAsNewEvent,
		startedEvent,
		di,
		newStateBuilder,
		eventStoreVersion,
	); err != nil {
		return nil, nil, err
	}

	return continueAsNewEvent, newStateBuilder, nil
}

func rolloverAutoResetPointsWithExpiringTime(
	resetPoints *workflow.ResetPoints,
	prevRunID string,
	nowNano int64,
	domainRetentionDays int32,
) *workflow.ResetPoints {

	if resetPoints == nil || resetPoints.Points == nil {
		return resetPoints
	}
	newPoints := make([]*workflow.ResetPointInfo, 0, len(resetPoints.Points))
	expiringTimeNano := nowNano + int64(time.Duration(domainRetentionDays)*time.Hour*24)
	for _, rp := range resetPoints.Points {
		if rp.GetRunId() == prevRunID {
			rp.ExpiringTimeNano = common.Int64Ptr(expiringTimeNano)
		}
		newPoints = append(newPoints, rp)
	}
	return &workflow.ResetPoints{
		Points: newPoints,
	}
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionContinuedAsNewEvent(
	firstEventID int64,
	domainID string,
	continueAsNewEvent *workflow.HistoryEvent,
	newStartedEvent *workflow.HistoryEvent,
	newDecision *decisionInfo,
	newStateBuilder mutableState,
	newEventStoreVersion int32,
) error {

	e.executionInfo.State = persistence.WorkflowStateCompleted
	e.executionInfo.CloseStatus = persistence.WorkflowCloseStatusContinuedAsNew
	e.executionInfo.CompletionEventBatchID = firstEventID // Used when completion event needs to be loaded from database
	e.ClearStickyness()
	e.writeEventToCache(continueAsNewEvent)

	newStartedTime := time.Unix(0, newStartedEvent.GetTimestamp())
	newStartAttr := newStartedEvent.WorkflowExecutionStartedEventAttributes
	if newDecision != nil {
		newStateBuilder.AddTransferTasks(&persistence.DecisionTask{
			DomainID:   domainID,
			TaskList:   newStateBuilder.GetExecutionInfo().TaskList,
			ScheduleID: newDecision.ScheduleID,
		})

		if newStateBuilder.GetReplicationState() != nil {
			newStateBuilder.UpdateReplicationStateLastEventID(newDecision.Version, newDecision.ScheduleID)
		}
	} else {
		backoffTimer := &persistence.WorkflowBackoffTimerTask{
			VisibilityTimestamp: newStartedTime.Add(time.Second * time.Duration(newStartAttr.GetFirstDecisionTaskBackoffSeconds())),
		}
		if newStartAttr.GetInitiator() == workflow.ContinueAsNewInitiatorRetryPolicy {
			backoffTimer.TimeoutType = persistence.WorkflowBackoffTimeoutTypeRetry
		} else if newStartAttr.GetInitiator() == workflow.ContinueAsNewInitiatorCronSchedule {
			backoffTimer.TimeoutType = persistence.WorkflowBackoffTimeoutTypeCron
		}
		newStateBuilder.AddTimerTasks(backoffTimer)

		if newStateBuilder.GetReplicationState() != nil {
			newStateBuilder.UpdateReplicationStateLastEventID(newStartedEvent.GetVersion(), newStartedEvent.GetEventId())
		}
	}

	// timeout includes workflow_timeout + backoff_interval
	timeoutInSeconds := newStartAttr.GetExecutionStartToCloseTimeoutSeconds() + newStartAttr.GetFirstDecisionTaskBackoffSeconds()
	timeoutDuration := time.Duration(timeoutInSeconds) * time.Second
	timeoutDeadline := newStartedTime.Add(timeoutDuration)
	if !newStateBuilder.GetExecutionInfo().ExpirationTime.IsZero() && timeoutDeadline.After(newStateBuilder.GetExecutionInfo().ExpirationTime) {
		timeoutDeadline = newStateBuilder.GetExecutionInfo().ExpirationTime
	}
	newStateBuilder.AddTransferTasks(&persistence.RecordWorkflowStartedTask{})
	newStateBuilder.AddTimerTasks(&persistence.WorkflowTimeoutTask{
		VisibilityTimestamp: timeoutDeadline,
	})
	if newEventStoreVersion == persistence.EventStoreVersionV2 {
		if err := newStateBuilder.SetHistoryTree(newStateBuilder.GetExecutionInfo().RunID); err != nil {
			return err
		}
	}

	return nil
}

func (e *mutableStateBuilder) AddStartChildWorkflowExecutionInitiatedEvent(decisionCompletedEventID int64, createRequestID string,
	attributes *workflow.StartChildWorkflowExecutionDecisionAttributes) (*workflow.HistoryEvent, *persistence.ChildExecutionInfo, error) {

	opTag := tag.WorkflowActionChildWorkflowInitiated
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, err
	}

	event := e.hBuilder.AddStartChildWorkflowExecutionInitiatedEvent(decisionCompletedEventID, attributes)
	// Write the event to cache only on active cluster
	e.eventsCache.putEvent(e.executionInfo.DomainID, e.executionInfo.WorkflowID, e.executionInfo.RunID,
		event.GetEventId(), event)

	ci, err := e.ReplicateStartChildWorkflowExecutionInitiatedEvent(decisionCompletedEventID, event, createRequestID)
	if err != nil {
		return nil, nil, err
	}
	return event, ci, nil
}

func (e *mutableStateBuilder) ReplicateStartChildWorkflowExecutionInitiatedEvent(
	firstEventID int64,
	event *workflow.HistoryEvent,
	createRequestID string,
) (*persistence.ChildExecutionInfo, error) {

	initiatedEventID := event.GetEventId()
	attributes := event.StartChildWorkflowExecutionInitiatedEventAttributes
	ci := &persistence.ChildExecutionInfo{
		Version:               event.GetVersion(),
		InitiatedID:           initiatedEventID,
		InitiatedEventBatchID: firstEventID,
		StartedID:             common.EmptyEventID,
		StartedWorkflowID:     attributes.GetWorkflowId(),
		CreateRequestID:       createRequestID,
		DomainName:            attributes.GetDomain(),
		WorkflowTypeName:      attributes.GetWorkflowType().GetName(),
	}

	e.pendingChildExecutionInfoIDs[initiatedEventID] = ci
	e.updateChildExecutionInfos[ci] = struct{}{}

	return ci, nil
}

func (e *mutableStateBuilder) AddChildWorkflowExecutionStartedEvent(
	domain *string,
	execution *workflow.WorkflowExecution,
	workflowType *workflow.WorkflowType,
	initiatedID int64,
	header *workflow.Header,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionChildWorkflowStarted
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok || ci.StartedID != common.EmptyEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowInitiatedID(ci.InitiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddChildWorkflowExecutionStartedEvent(domain, execution, workflowType, initiatedID, header)
	if err := e.ReplicateChildWorkflowExecutionStartedEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateChildWorkflowExecutionStartedEvent(event *workflow.HistoryEvent) error {
	attributes := event.ChildWorkflowExecutionStartedEventAttributes
	initiatedID := attributes.GetInitiatedEventId()

	ci, _ := e.GetChildExecutionInfo(initiatedID)
	ci.StartedID = event.GetEventId()
	ci.StartedRunID = attributes.GetWorkflowExecution().GetRunId()
	e.updateChildExecutionInfos[ci] = struct{}{}

	return nil
}

func (e *mutableStateBuilder) AddStartChildWorkflowExecutionFailedEvent(
	initiatedID int64,
	cause workflow.ChildWorkflowExecutionFailedCause,
	initiatedEventAttributes *workflow.StartChildWorkflowExecutionInitiatedEventAttributes,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionChildWorkflowInitiationFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok || ci.StartedID != common.EmptyEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowInitiatedID(initiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddStartChildWorkflowExecutionFailedEvent(initiatedID, cause, initiatedEventAttributes)
	if err := e.ReplicateStartChildWorkflowExecutionFailedEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateStartChildWorkflowExecutionFailedEvent(event *workflow.HistoryEvent) error {
	attributes := event.StartChildWorkflowExecutionFailedEventAttributes
	initiatedID := attributes.GetInitiatedEventId()

	e.DeletePendingChildExecution(initiatedID)
	return nil
}

func (e *mutableStateBuilder) AddChildWorkflowExecutionCompletedEvent(
	initiatedID int64,
	childExecution *workflow.WorkflowExecution,
	attributes *workflow.WorkflowExecutionCompletedEventAttributes,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionChildWorkflowCompleted
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok || ci.StartedID == common.EmptyEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowInitiatedID(ci.InitiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	var domain *string
	if len(ci.DomainName) > 0 {
		domain = &ci.DomainName
	}
	workflowType := &workflow.WorkflowType{
		Name: common.StringPtr(ci.WorkflowTypeName),
	}

	event := e.hBuilder.AddChildWorkflowExecutionCompletedEvent(domain, childExecution, workflowType, ci.InitiatedID,
		ci.StartedID, attributes)
	if err := e.ReplicateChildWorkflowExecutionCompletedEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateChildWorkflowExecutionCompletedEvent(event *workflow.HistoryEvent) error {
	attributes := event.ChildWorkflowExecutionCompletedEventAttributes
	initiatedID := attributes.GetInitiatedEventId()

	e.DeletePendingChildExecution(initiatedID)
	return nil
}

func (e *mutableStateBuilder) AddChildWorkflowExecutionFailedEvent(
	initiatedID int64,
	childExecution *workflow.WorkflowExecution,
	attributes *workflow.WorkflowExecutionFailedEventAttributes,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionChildWorkflowFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok || ci.StartedID == common.EmptyEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowInitiatedID(ci.InitiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	var domain *string
	if len(ci.DomainName) > 0 {
		domain = &ci.DomainName
	}
	workflowType := &workflow.WorkflowType{
		Name: common.StringPtr(ci.WorkflowTypeName),
	}

	event := e.hBuilder.AddChildWorkflowExecutionFailedEvent(domain, childExecution, workflowType, ci.InitiatedID,
		ci.StartedID, attributes)
	if err := e.ReplicateChildWorkflowExecutionFailedEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateChildWorkflowExecutionFailedEvent(event *workflow.HistoryEvent) error {
	attributes := event.ChildWorkflowExecutionFailedEventAttributes
	initiatedID := attributes.GetInitiatedEventId()

	e.DeletePendingChildExecution(initiatedID)
	return nil
}

func (e *mutableStateBuilder) AddChildWorkflowExecutionCanceledEvent(
	initiatedID int64,
	childExecution *workflow.WorkflowExecution,
	attributes *workflow.WorkflowExecutionCanceledEventAttributes,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionChildWorkflowCanceled
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok || ci.StartedID == common.EmptyEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowInitiatedID(ci.InitiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	var domain *string
	if len(ci.DomainName) > 0 {
		domain = &ci.DomainName
	}
	workflowType := &workflow.WorkflowType{
		Name: common.StringPtr(ci.WorkflowTypeName),
	}

	event := e.hBuilder.AddChildWorkflowExecutionCanceledEvent(domain, childExecution, workflowType, ci.InitiatedID,
		ci.StartedID, attributes)
	if err := e.ReplicateChildWorkflowExecutionCanceledEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateChildWorkflowExecutionCanceledEvent(event *workflow.HistoryEvent) error {
	attributes := event.ChildWorkflowExecutionCanceledEventAttributes
	initiatedID := attributes.GetInitiatedEventId()

	e.DeletePendingChildExecution(initiatedID)
	return nil
}

func (e *mutableStateBuilder) AddChildWorkflowExecutionTerminatedEvent(
	initiatedID int64,
	childExecution *workflow.WorkflowExecution,
	attributes *workflow.WorkflowExecutionTerminatedEventAttributes,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionChildWorkflowTerminated
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok || ci.StartedID == common.EmptyEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowInitiatedID(ci.InitiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	var domain *string
	if len(ci.DomainName) > 0 {
		domain = &ci.DomainName
	}
	workflowType := &workflow.WorkflowType{
		Name: common.StringPtr(ci.WorkflowTypeName),
	}

	event := e.hBuilder.AddChildWorkflowExecutionTerminatedEvent(domain, childExecution, workflowType, ci.InitiatedID,
		ci.StartedID, attributes)
	if err := e.ReplicateChildWorkflowExecutionTerminatedEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateChildWorkflowExecutionTerminatedEvent(event *workflow.HistoryEvent) error {
	attributes := event.ChildWorkflowExecutionTerminatedEventAttributes
	initiatedID := attributes.GetInitiatedEventId()

	e.DeletePendingChildExecution(initiatedID)
	return nil
}

func (e *mutableStateBuilder) AddChildWorkflowExecutionTimedOutEvent(
	initiatedID int64,
	childExecution *workflow.WorkflowExecution,
	attributes *workflow.WorkflowExecutionTimedOutEventAttributes,
) (*workflow.HistoryEvent, error) {

	opTag := tag.WorkflowActionChildWorkflowTimedOut
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ci, ok := e.GetChildExecutionInfo(initiatedID)
	if !ok || ci.StartedID == common.EmptyEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowInitiatedID(ci.InitiatedID))
		return nil, e.createInternalServerError(opTag)
	}

	var domain *string
	if len(ci.DomainName) > 0 {
		domain = &ci.DomainName
	}
	workflowType := &workflow.WorkflowType{
		Name: common.StringPtr(ci.WorkflowTypeName),
	}

	event := e.hBuilder.AddChildWorkflowExecutionTimedOutEvent(domain, childExecution, workflowType, ci.InitiatedID,
		ci.StartedID, attributes)
	if err := e.ReplicateChildWorkflowExecutionTimedOutEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateChildWorkflowExecutionTimedOutEvent(event *workflow.HistoryEvent) error {
	attributes := event.ChildWorkflowExecutionTimedOutEventAttributes
	initiatedID := attributes.GetInitiatedEventId()

	e.DeletePendingChildExecution(initiatedID)
	return nil
}

func (e *mutableStateBuilder) CreateActivityRetryTimer(
	ai *persistence.ActivityInfo,
	failureReason string,
) persistence.Task {

	retryTask := prepareActivityNextRetryWithTime(e.GetCurrentVersion(), ai, failureReason, e.timeSource.Now())
	if retryTask != nil {
		e.updateActivityInfos[ai] = struct{}{}
		e.syncActivityTasks[ai.ScheduleID] = struct{}{}
	}

	return retryTask
}

// TODO mutable state should generate corresponding transfer / timer tasks according to
//  updates accumulated, while currently all transfer / timer tasks are managed manually

// TODO convert AddTransferTasks to prepareTransferTasks
func (e *mutableStateBuilder) AddTransferTasks(
	transferTasks ...persistence.Task,
) {

	e.insertTransferTasks = append(e.insertTransferTasks, transferTasks...)
}

// TODO convert AddTransferTasks to prepareTimerTasks
func (e *mutableStateBuilder) AddTimerTasks(
	timerTasks ...persistence.Task,
) {

	e.insertTimerTasks = append(e.insertTimerTasks, timerTasks...)
}

func (e *mutableStateBuilder) SetUpdateCondition(condition int64) {
	e.condition = condition
}

func (e *mutableStateBuilder) GetUpdateCondition() int64 {
	return e.condition
}

func (e *mutableStateBuilder) CloseTransactionAsMutation(
	now time.Time,
	transactionPolicy transactionPolicy,
) (*persistence.WorkflowMutation, []*persistence.WorkflowEvents, error) {

	if err := e.prepareTransaction(transactionPolicy); err != nil {
		return nil, nil, err
	}

	workflowEventsSeq, err := e.prepareEventsAndReplicationTasks(transactionPolicy)
	if err != nil {
		return nil, nil, err
	}

	if len(workflowEventsSeq) > 0 {
		lastEvents := workflowEventsSeq[len(workflowEventsSeq)-1].Events
		lastEvent := lastEvents[len(lastEvents)-1]
		e.GetExecutionInfo().LastEventTaskID = lastEvent.GetTaskId()
		if e.GetReplicationState() != nil {
			e.UpdateReplicationStateLastEventID(lastEvent.GetVersion(), lastEvent.GetEventId())
		}
	}

	setTaskInfo(e.GetCurrentVersion(), now, e.insertTransferTasks, e.insertTimerTasks)

	// update last update time
	e.executionInfo.LastUpdatedTimestamp = now

	workflowMutation := &persistence.WorkflowMutation{
		ExecutionInfo:    e.executionInfo,
		ReplicationState: e.replicationState,

		UpsertActivityInfos:       convertUpdateActivityInfos(e.updateActivityInfos),
		DeleteActivityInfos:       convertDeleteActivityInfos(e.deleteActivityInfos),
		UpserTimerInfos:           convertUpdateTimerInfos(e.updateTimerInfos),
		DeleteTimerInfos:          convertDeleteTimerInfos(e.deleteTimerInfos),
		UpsertChildExecutionInfos: convertUpdateChildExecutionInfos(e.updateChildExecutionInfos),
		DeleteChildExecutionInfo:  e.deleteChildExecutionInfo,
		UpsertRequestCancelInfos:  convertUpdateRequestCancelInfos(e.updateRequestCancelInfos),
		DeleteRequestCancelInfo:   e.deleteRequestCancelInfo,
		UpsertSignalInfos:         convertUpdateSignalInfos(e.updateSignalInfos),
		DeleteSignalInfo:          e.deleteSignalInfo,
		UpsertSignalRequestedIDs:  convertSignalRequestedIDs(e.updateSignalRequestedIDs),
		DeleteSignalRequestedID:   e.deleteSignalRequestedID,
		NewBufferedEvents:         e.updateBufferedEvents,
		ClearBufferedEvents:       e.clearBufferedEvents,

		TransferTasks:    e.insertTransferTasks,
		ReplicationTasks: e.insertReplicationTasks,
		TimerTasks:       e.insertTimerTasks,

		Condition: e.condition,
	}

	if err := e.cleanupTransaction(transactionPolicy); err != nil {
		return nil, nil, err
	}
	return workflowMutation, workflowEventsSeq, nil
}

func (e *mutableStateBuilder) CloseTransactionAsSnapshot(
	now time.Time,
	transactionPolicy transactionPolicy,
) (*persistence.WorkflowSnapshot, []*persistence.WorkflowEvents, error) {

	if err := e.prepareTransaction(transactionPolicy); err != nil {
		return nil, nil, err
	}

	workflowEventsSeq, err := e.prepareEventsAndReplicationTasks(transactionPolicy)
	if err != nil {
		return nil, nil, err
	}

	if len(workflowEventsSeq) > 1 {
		return nil, nil, &workflow.InternalServiceError{
			Message: "cannot generate workflow snapshot with transient events",
		}
	}
	if len(e.bufferedEvents) > 0 {
		// TODO do we need the functionality to generate snapshot with buffered events?
		return nil, nil, &workflow.InternalServiceError{
			Message: "cannot generate workflow snapshot with buffered events",
		}
	}

	if len(workflowEventsSeq) > 0 {
		lastEvents := workflowEventsSeq[len(workflowEventsSeq)-1].Events
		lastEvent := lastEvents[len(lastEvents)-1]
		e.GetExecutionInfo().LastEventTaskID = lastEvent.GetTaskId()
		if e.GetReplicationState() != nil {
			e.UpdateReplicationStateLastEventID(lastEvent.GetVersion(), lastEvent.GetEventId())
		}
	}

	setTaskInfo(e.GetCurrentVersion(), now, e.insertTransferTasks, e.insertTimerTasks)

	// update last update time
	e.executionInfo.LastUpdatedTimestamp = now

	workflowSnapshot := &persistence.WorkflowSnapshot{
		ExecutionInfo:    e.executionInfo,
		ReplicationState: e.replicationState,

		ActivityInfos:       convertPendingActivityInfos(e.pendingActivityInfoIDs),
		TimerInfos:          convertPendingTimerInfos(e.pendingTimerInfoIDs),
		ChildExecutionInfos: convertPendingChildExecutionInfos(e.pendingChildExecutionInfoIDs),
		RequestCancelInfos:  convertPendingRequestCancelInfos(e.pendingRequestCancelInfoIDs),
		SignalInfos:         convertPendingSignalInfos(e.pendingSignalInfoIDs),
		SignalRequestedIDs:  convertSignalRequestedIDs(e.pendingSignalRequestedIDs),

		TransferTasks:    e.insertTransferTasks,
		ReplicationTasks: e.insertReplicationTasks,
		TimerTasks:       e.insertTimerTasks,

		Condition: e.condition,
	}

	if err := e.cleanupTransaction(transactionPolicy); err != nil {
		return nil, nil, err
	}
	return workflowSnapshot, workflowEventsSeq, nil
}

func (e *mutableStateBuilder) prepareTransaction(
	transactionPolicy transactionPolicy,
) error {

	if err := e.closeTransactionWithPolicyCheck(
		transactionPolicy,
	); err != nil {
		return err
	}

	if err := e.closeTransactionHandleDecisionFailover(
		transactionPolicy,
	); err != nil {
		return err
	}

	if err := e.closeTransactionHandleBufferedEventsLimit(
		transactionPolicy,
	); err != nil {
		return err
	}

	// flushing buffered events should happen at very last
	if transactionPolicy == transactionPolicyActive {
		if err := e.FlushBufferedEvents(); err != nil {
			return err
		}
	}

	return nil
}

func (e *mutableStateBuilder) cleanupTransaction(
	transactionPolicy transactionPolicy,
) error {

	// Clear all updates to prepare for the next session
	e.hBuilder = newHistoryBuilder(e, e.logger)

	e.updateActivityInfos = make(map[*persistence.ActivityInfo]struct{})
	e.deleteActivityInfos = make(map[int64]struct{})
	e.syncActivityTasks = make(map[int64]struct{})

	e.updateTimerInfos = make(map[*persistence.TimerInfo]struct{})
	e.deleteTimerInfos = make(map[string]struct{})

	e.updateChildExecutionInfos = make(map[*persistence.ChildExecutionInfo]struct{})
	e.deleteChildExecutionInfo = nil

	e.updateRequestCancelInfos = make(map[*persistence.RequestCancelInfo]struct{})
	e.deleteRequestCancelInfo = nil

	e.updateSignalInfos = make(map[*persistence.SignalInfo]struct{})
	e.deleteSignalInfo = nil

	e.updateSignalRequestedIDs = make(map[string]struct{})
	e.deleteSignalRequestedID = ""

	e.clearBufferedEvents = false
	if e.updateBufferedEvents != nil {
		e.bufferedEvents = append(e.bufferedEvents, e.updateBufferedEvents...)
		e.updateBufferedEvents = nil
	}

	e.hasBufferedEventsInPersistence = len(e.bufferedEvents) > 0
	e.condition = e.GetNextEventID()

	e.insertTransferTasks = nil
	e.insertReplicationTasks = nil
	e.insertTimerTasks = nil

	return nil
}

func (e *mutableStateBuilder) prepareEventsAndReplicationTasks(
	transactionPolicy transactionPolicy,
) ([]*persistence.WorkflowEvents, error) {

	workflowEventsSeq := []*persistence.WorkflowEvents{}
	if len(e.hBuilder.transientHistory) != 0 {
		workflowEventsSeq = append(workflowEventsSeq, &persistence.WorkflowEvents{
			DomainID:    e.executionInfo.DomainID,
			WorkflowID:  e.executionInfo.WorkflowID,
			RunID:       e.executionInfo.RunID,
			BranchToken: e.GetCurrentBranch(),
			Events:      e.hBuilder.transientHistory,
		})
	}
	if len(e.hBuilder.history) != 0 {
		workflowEventsSeq = append(workflowEventsSeq, &persistence.WorkflowEvents{
			DomainID:    e.executionInfo.DomainID,
			WorkflowID:  e.executionInfo.WorkflowID,
			RunID:       e.executionInfo.RunID,
			BranchToken: e.GetCurrentBranch(),
			Events:      e.hBuilder.history,
		})
	}

	if err := e.validateNoEventsAfterWorkflowFinish(
		transactionPolicy,
		e.hBuilder.history,
	); err != nil {
		return nil, err
	}

	for _, workflowEvents := range workflowEventsSeq {
		e.insertReplicationTasks = append(e.insertReplicationTasks,
			e.eventsToReplicationTask(transactionPolicy, workflowEvents.Events)...,
		)
	}

	e.insertReplicationTasks = append(e.insertReplicationTasks,
		e.syncActivityToReplicationTask(transactionPolicy)...,
	)

	if transactionPolicy == transactionPolicyPassive && len(e.insertReplicationTasks) > 0 {
		return nil, &workflow.InternalServiceError{
			Message: "should not generate replication task when close transaction as passive",
		}
	}

	return workflowEventsSeq, nil
}

func (e *mutableStateBuilder) eventsToReplicationTask(
	transactionPolicy transactionPolicy,
	events []*workflow.HistoryEvent,
) []persistence.Task {

	if transactionPolicy == transactionPolicyPassive ||
		!e.canReplicateEvents() ||
		len(events) == 0 {
		return emptyTasks
	}

	firstEvent := events[0]
	lastEvent := events[len(events)-1]
	version := firstEvent.GetVersion()

	sourceCluster := e.clusterMetadata.ClusterNameForFailoverVersion(version)
	currentCluster := e.clusterMetadata.GetCurrentClusterName()

	if currentCluster != sourceCluster {
		return emptyTasks
	}
	return []persistence.Task{
		&persistence.HistoryReplicationTask{
			FirstEventID:            firstEvent.GetEventId(),
			NextEventID:             lastEvent.GetEventId() + 1,
			Version:                 firstEvent.GetVersion(),
			LastReplicationInfo:     e.GetReplicationState().LastReplicationInfo,
			EventStoreVersion:       e.GetEventStoreVersion(),
			BranchToken:             e.GetCurrentBranch(),
			NewRunEventStoreVersion: 0,
			NewRunBranchToken:       nil,
		},
	}
}

func (e *mutableStateBuilder) syncActivityToReplicationTask(
	transactionPolicy transactionPolicy,
) []persistence.Task {

	if transactionPolicy == transactionPolicyPassive ||
		!e.canReplicateEvents() {
		return emptyTasks
	}

	return convertSyncActivityInfos(
		e.pendingActivityInfoIDs,
		e.syncActivityTasks,
	)
}

func (e *mutableStateBuilder) canReplicateEvents() bool {
	return e.GetReplicationState() != nil &&
		e.replicationPolicy == cache.ReplicationPolicyMultiCluster
}

// validateNoEventsAfterWorkflowFinish perform check on history event batch
// NOTE: do not apply this check on every batch, since transient
// decision && workflow finish will be broken (the first batch)
func (e *mutableStateBuilder) validateNoEventsAfterWorkflowFinish(
	transactionPolicy transactionPolicy,
	events []*workflow.HistoryEvent,
) error {

	if transactionPolicy == transactionPolicyPassive ||
		len(events) == 0 {
		return nil
	}

	// only do check if workflow is finished
	if e.GetExecutionInfo().State != persistence.WorkflowStateCompleted {
		return nil
	}

	// workflow close
	// this will perform check on the last event of last batch
	// NOTE: do not apply this check on every batch, since transient
	// decision && workflow finish will be broken (the first batch)
	lastEvent := events[len(events)-1]
	switch lastEvent.GetEventType() {
	case workflow.EventTypeWorkflowExecutionCompleted,
		workflow.EventTypeWorkflowExecutionFailed,
		workflow.EventTypeWorkflowExecutionTimedOut,
		workflow.EventTypeWorkflowExecutionTerminated,
		workflow.EventTypeWorkflowExecutionContinuedAsNew,
		workflow.EventTypeWorkflowExecutionCanceled:
		return nil

	default:
		executionInfo := e.GetExecutionInfo()
		e.logger.Error(
			"encounter case where events appears after workflow finish.",
			tag.WorkflowDomainID(executionInfo.DomainID),
			tag.WorkflowID(executionInfo.WorkflowID),
			tag.WorkflowRunID(executionInfo.RunID),
		)
		return ErrEventsAterWorkflowFinish
	}
}

func (e *mutableStateBuilder) closeTransactionWithPolicyCheck(
	transactionPolicy transactionPolicy,
) error {

	if transactionPolicy == transactionPolicyPassive ||
		e.GetReplicationState() == nil {
		return nil
	}

	activeCluster := e.clusterMetadata.ClusterNameForFailoverVersion(e.GetCurrentVersion())
	currentCluster := e.clusterMetadata.GetCurrentClusterName()

	if activeCluster != currentCluster {
		domainID := e.GetExecutionInfo().DomainID
		return errors.NewDomainNotActiveError(domainID, currentCluster, activeCluster)
	}
	return nil
}

func (e *mutableStateBuilder) closeTransactionHandleDecisionFailover(
	transactionPolicy transactionPolicy,
) error {

	if transactionPolicy == transactionPolicyPassive ||
		!e.IsWorkflowExecutionRunning() ||
		e.GetReplicationState() == nil {
		return nil
	}

	// Handling mutable state turn from standby to active, while having a decision on the fly
	di, ok := e.GetInFlightDecisionTask()
	if ok && di.Version < e.GetCurrentVersion() {
		// we have a decision on the fly with a lower version, fail it
		if err := failDecision(
			e,
			di,
			workflow.DecisionTaskFailedCauseFailoverCloseDecision,
		); err != nil {
			return err
		}

		err := scheduleDecision(e, e.timeSource, e.logger)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *mutableStateBuilder) closeTransactionHandleBufferedEventsLimit(
	transactionPolicy transactionPolicy,
) error {

	if transactionPolicy == transactionPolicyPassive ||
		!e.IsWorkflowExecutionRunning() {
		return nil
	}

	if len(e.bufferedEvents) < e.config.MaximumBufferedEventsBatch() {
		return nil
	}

	// Handling buffered events size issue
	if di, ok := e.GetInFlightDecisionTask(); ok {
		// we have a decision on the fly with a lower version, fail it
		if err := failDecision(
			e,
			di,
			workflow.DecisionTaskFailedCauseForceCloseDecision,
		); err != nil {
			return err
		}

		err := scheduleDecision(e, e.timeSource, e.logger)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *mutableStateBuilder) closeTransactionHandleWorkflowReset(
	transactionPolicy transactionPolicy,
) error {

	if transactionPolicy == transactionPolicyPassive ||
		!e.IsWorkflowExecutionRunning() {
		return nil
	}

	// compare with bad client binary checksum and schedule a reset task

	// only schedule reset task if current doesn't have childWFs.
	// TODO: This will be removed once our reset allows childWFs
	if len(e.GetPendingChildExecutionInfos()) != 0 {
		return nil
	}

	executionInfo := e.GetExecutionInfo()
	domainEntry, err := e.shard.GetDomainCache().GetDomainByID(executionInfo.DomainID)
	if err != nil {
		return err
	}
	if _, pt := FindAutoResetPoint(
		e.timeSource,
		&domainEntry.GetConfig().BadBinaries,
		e.GetExecutionInfo().AutoResetPoints,
	); pt != nil {
		e.AddTransferTasks(&persistence.ResetWorkflowTask{})
		e.logger.Info("Auto-Reset task is scheduled",
			tag.WorkflowDomainName(domainEntry.GetInfo().Name),
			tag.WorkflowID(executionInfo.WorkflowID),
			tag.WorkflowRunID(executionInfo.RunID),
			tag.WorkflowResetBaseRunID(pt.GetRunId()),
			tag.WorkflowEventID(pt.GetFirstDecisionCompletedId()),
			tag.WorkflowBinaryChecksum(pt.GetBinaryChecksum()),
		)
	}
	return nil
}

func (e *mutableStateBuilder) checkMutability(
	actionTag tag.Tag,
) error {

	if !e.IsWorkflowExecutionRunning() {
		e.logger.Warn(
			mutableStateInvalidHistoryActionMsg,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowState(e.executionInfo.State),
			actionTag,
		)
		return ErrWorkflowFinished
	}
	return nil
}

func (e *mutableStateBuilder) createInternalServerError(
	actionTag tag.Tag,
) error {

	return &workflow.InternalServiceError{Message: actionTag.Field().String + " operation failed"}
}

func (e *mutableStateBuilder) createCallerError(
	actionTag tag.Tag,
) error {

	return &workflow.BadRequestError{
		Message: fmt.Sprintf(mutableStateInvalidHistoryActionMsgTemplate, actionTag.Field().String),
	}
}
