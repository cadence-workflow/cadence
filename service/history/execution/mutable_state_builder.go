// Copyright (c) 2017-2020 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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

package execution

import (
	"context"
	"fmt"
	"math/rand"
	"runtime/debug"
	"time"

	"github.com/pborman/uuid"
	"golang.org/x/exp/maps"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/query"
	"github.com/uber/cadence/service/history/shard"
)

const (
	mutableStateInvalidHistoryActionMsg         = "invalid history builder state for action"
	mutableStateInvalidHistoryActionMsgTemplate = mutableStateInvalidHistoryActionMsg + ": %v"

	timerCancellationMsgTimerIDUnknown = "TIMER_ID_UNKNOWN"
)

var (
	// ErrWorkflowFinished indicates trying to mutate mutable state after workflow finished
	ErrWorkflowFinished = &types.InternalServiceError{Message: "invalid mutable state action: mutation after finish"}
	// ErrMissingTimerInfo indicates missing timer info
	ErrMissingTimerInfo = &types.InternalServiceError{Message: "unable to get timer info"}
	// ErrMissingActivityInfo indicates missing activity info
	ErrMissingActivityInfo = &types.InternalServiceError{Message: "unable to get activity info"}
	// ErrMissingChildWorkflowInfo indicates missing child workflow info
	ErrMissingChildWorkflowInfo = &types.InternalServiceError{Message: "unable to get child workflow info"}
	// ErrMissingWorkflowStartEvent indicates missing workflow start event
	ErrMissingWorkflowStartEvent = &types.InternalServiceError{Message: "unable to get workflow start event"}
	// ErrMissingWorkflowCompletionEvent indicates missing workflow completion event
	ErrMissingWorkflowCompletionEvent = &types.InternalServiceError{Message: "unable to get workflow completion event"}
	// ErrMissingActivityScheduledEvent indicates missing workflow activity scheduled event
	ErrMissingActivityScheduledEvent = &types.InternalServiceError{Message: "unable to get activity scheduled event"}
	// ErrMissingChildWorkflowInitiatedEvent indicates missing child workflow initiated event
	ErrMissingChildWorkflowInitiatedEvent = &types.InternalServiceError{Message: "unable to get child workflow initiated event"}
	// ErrEventsAfterWorkflowFinish is the error indicating server error trying to write events after workflow finish event
	ErrEventsAfterWorkflowFinish = &types.InternalServiceError{Message: "error validating last event being workflow finish event"}
	// ErrMissingVersionHistories is the error indicating cadence failed to process 2dc workflow type.
	ErrMissingVersionHistories = &types.BadRequestError{Message: "versionHistories is empty, which is required for NDC feature. It's probably from deprecated 2dc workflows"}
	// ErrTooManyPendingActivities is the error that currently there are too many pending activities in the workflow
	ErrTooManyPendingActivities = &types.InternalServiceError{Message: "Too many pending activities"}
)

type (
	mutableStateBuilder struct {
		pendingActivityInfoIDs     map[int64]*persistence.ActivityInfo // Schedule Event ID -> Activity Info.
		pendingActivityIDToEventID map[string]int64                    // Activity ID -> Schedule Event ID of the activity.
		updateActivityInfos        map[int64]*persistence.ActivityInfo // Modified activities from last update.
		deleteActivityInfos        map[int64]struct{}                  // Deleted activities from last update.
		syncActivityTasks          map[int64]struct{}                  // Activity to be sync to remote

		pendingTimerInfoIDs     map[string]*persistence.TimerInfo // User Timer ID -> Timer Info.
		pendingTimerEventIDToID map[int64]string                  // User Timer Start Event ID -> User Timer ID.
		updateTimerInfos        map[string]*persistence.TimerInfo // Modified timers from last update.
		deleteTimerInfos        map[string]struct{}               // Deleted timers from last update.

		pendingChildExecutionInfoIDs map[int64]*persistence.ChildExecutionInfo // Initiated Event ID -> Child Execution Info
		updateChildExecutionInfos    map[int64]*persistence.ChildExecutionInfo // Modified ChildExecution Infos since last update
		deleteChildExecutionInfos    map[int64]struct{}                        // Deleted ChildExecution Infos since last update

		pendingRequestCancelInfoIDs map[int64]*persistence.RequestCancelInfo // Initiated Event ID -> RequestCancelInfo
		updateRequestCancelInfos    map[int64]*persistence.RequestCancelInfo // Modified RequestCancel Infos since last update, for persistence update
		deleteRequestCancelInfos    map[int64]struct{}                       // Deleted RequestCancel Infos since last update, for persistence update

		pendingSignalInfoIDs map[int64]*persistence.SignalInfo // Initiated Event ID -> SignalInfo
		updateSignalInfos    map[int64]*persistence.SignalInfo // Modified SignalInfo since last update
		deleteSignalInfos    map[int64]struct{}                // Deleted SignalInfos since last update

		pendingSignalRequestedIDs map[string]struct{} // Set of signaled requestIds
		updateSignalRequestedIDs  map[string]struct{} // Set of signaled requestIds since last update
		deleteSignalRequestedIDs  map[string]struct{} // Deleted signaled requestIds

		bufferedEvents       []*types.HistoryEvent // buffered history events that are already persisted
		updateBufferedEvents []*types.HistoryEvent // buffered history events that needs to be persisted
		clearBufferedEvents  bool                  // delete buffered events from persistence

		// This section includes Workflow Execution Info parameters like StartTimestamp,
		// which are only visible after the workflow has begun.
		// However, there are other parameters such as LastEventTimestamp,
		// which are updated as the execution progresses through various cycles.
		// It's common to encounter null values for these timestamps initially,
		// as they are populated once the update cycles are initiated.
		executionInfo    *persistence.WorkflowExecutionInfo // Workflow mutable state info.
		versionHistories *persistence.VersionHistories
		// TODO: remove this struct after all 2DC workflows complete
		replicationState *persistence.ReplicationState
		hBuilder         *HistoryBuilder

		// in memory only attributes
		// indicate the current version
		currentVersion int64
		// indicates whether there are buffered events in persistence
		hasBufferedEventsInDB bool
		// indicates the workflow state in DB, can be used to calculate
		// whether this workflow is pointed by current workflow record
		stateInDB int
		// indicates the next event ID in DB, for conditional update
		nextEventIDInDB int64
		// domain entry contains a snapshot of domain
		// NOTE: do not use the failover version inside, use currentVersion above
		domainEntry *cache.DomainCacheEntry
		// record if a event has been applied to mutable state
		// TODO: persist this to db
		appliedEvents map[string]struct{}

		insertTransferTasks    []persistence.Task
		insertReplicationTasks []persistence.Task
		insertTimerTasks       []persistence.Task

		workflowRequests map[persistence.WorkflowRequest]struct{}

		// do not rely on this, this is only updated on
		// Load() and closeTransactionXXX methods. So when
		// a transaction is in progress, this value will be
		// wrong. This exist primarily for visibility via CLI
		checksum checksum.Checksum

		taskGenerator       MutableStateTaskGenerator
		decisionTaskManager mutableStateDecisionTaskManager
		queryRegistry       query.Registry

		shard                      shard.Context
		clusterMetadata            cluster.Metadata
		eventsCache                events.Cache
		config                     *config.Config
		timeSource                 clock.TimeSource
		logger                     log.Logger
		metricsClient              metrics.Client
		pendingActivityWarningSent bool

		executionStats *persistence.ExecutionStats
	}
)

var _ MutableState = (*mutableStateBuilder)(nil)

// NewMutableStateBuilder creates a new workflow mutable state builder
func NewMutableStateBuilder(
	shard shard.Context,
	logger log.Logger,
	domainEntry *cache.DomainCacheEntry,
) MutableState {
	return newMutableStateBuilder(shard, logger, domainEntry)
}

func newMutableStateBuilder(
	shard shard.Context,
	logger log.Logger,
	domainEntry *cache.DomainCacheEntry,
) *mutableStateBuilder {
	s := &mutableStateBuilder{
		updateActivityInfos:        make(map[int64]*persistence.ActivityInfo),
		pendingActivityInfoIDs:     make(map[int64]*persistence.ActivityInfo),
		pendingActivityIDToEventID: make(map[string]int64),
		deleteActivityInfos:        make(map[int64]struct{}),
		syncActivityTasks:          make(map[int64]struct{}),

		pendingTimerInfoIDs:     make(map[string]*persistence.TimerInfo),
		pendingTimerEventIDToID: make(map[int64]string),
		updateTimerInfos:        make(map[string]*persistence.TimerInfo),
		deleteTimerInfos:        make(map[string]struct{}),

		updateChildExecutionInfos:    make(map[int64]*persistence.ChildExecutionInfo),
		pendingChildExecutionInfoIDs: make(map[int64]*persistence.ChildExecutionInfo),
		deleteChildExecutionInfos:    make(map[int64]struct{}),

		updateRequestCancelInfos:    make(map[int64]*persistence.RequestCancelInfo),
		pendingRequestCancelInfoIDs: make(map[int64]*persistence.RequestCancelInfo),
		deleteRequestCancelInfos:    make(map[int64]struct{}),

		updateSignalInfos:    make(map[int64]*persistence.SignalInfo),
		pendingSignalInfoIDs: make(map[int64]*persistence.SignalInfo),
		deleteSignalInfos:    make(map[int64]struct{}),

		updateSignalRequestedIDs:  make(map[string]struct{}),
		pendingSignalRequestedIDs: make(map[string]struct{}),
		deleteSignalRequestedIDs:  make(map[string]struct{}),

		currentVersion:        domainEntry.GetFailoverVersion(),
		hasBufferedEventsInDB: false,
		stateInDB:             persistence.WorkflowStateVoid,
		nextEventIDInDB:       0,
		domainEntry:           domainEntry,
		appliedEvents:         make(map[string]struct{}),

		queryRegistry: query.NewRegistry(),

		shard:           shard,
		clusterMetadata: shard.GetClusterMetadata(),
		eventsCache:     shard.GetEventsCache(),
		config:          shard.GetConfig(),
		timeSource:      shard.GetTimeSource(),
		logger:          logger,
		metricsClient:   shard.GetMetricsClient(),

		workflowRequests: make(map[persistence.WorkflowRequest]struct{}),
	}
	s.executionInfo = &persistence.WorkflowExecutionInfo{
		DecisionVersion:    constants.EmptyVersion,
		DecisionScheduleID: constants.EmptyEventID,
		DecisionStartedID:  constants.EmptyEventID,
		DecisionRequestID:  constants.EmptyUUID,
		DecisionTimeout:    0,

		NextEventID:        constants.FirstEventID,
		State:              persistence.WorkflowStateCreated,
		CloseStatus:        persistence.WorkflowCloseStatusNone,
		LastProcessedEvent: constants.EmptyEventID,
	}
	s.hBuilder = NewHistoryBuilder(s)
	s.taskGenerator = NewMutableStateTaskGenerator(shard.GetLogger(), shard.GetClusterMetadata(), shard.GetDomainCache(), s)
	s.decisionTaskManager = newMutableStateDecisionTaskManager(s)
	s.executionStats = &persistence.ExecutionStats{}
	return s
}

// NewMutableStateBuilderWithVersionHistories creates mutable state builder with version history initialized
func NewMutableStateBuilderWithVersionHistories(
	shard shard.Context,
	logger log.Logger,
	domainEntry *cache.DomainCacheEntry,
) MutableState {

	s := newMutableStateBuilder(shard, logger, domainEntry)
	s.versionHistories = persistence.NewVersionHistories(&persistence.VersionHistory{})
	return s
}

// NewMutableStateBuilderWithEventV2 is used only in test
func NewMutableStateBuilderWithEventV2(
	shard shard.Context,
	logger log.Logger,
	runID string,
	domainEntry *cache.DomainCacheEntry,
) MutableState {

	msBuilder := NewMutableStateBuilder(shard, logger, domainEntry)
	_ = msBuilder.SetHistoryTree(runID)

	return msBuilder
}

// NewMutableStateBuilderWithVersionHistoriesWithEventV2 is used only in test
func NewMutableStateBuilderWithVersionHistoriesWithEventV2(
	shard shard.Context,
	logger log.Logger,
	version int64,
	runID string,
	domainEntry *cache.DomainCacheEntry,
) MutableState {

	msBuilder := NewMutableStateBuilderWithVersionHistories(shard, logger, domainEntry)
	err := msBuilder.UpdateCurrentVersion(version, false)
	if err != nil {
		logger.Error("update current version error", tag.Error(err))
	}
	_ = msBuilder.SetHistoryTree(runID)

	return msBuilder
}

// Creates a shallow copy, not safe to use if the copied struct is mutated
func (e *mutableStateBuilder) CopyToPersistence() *persistence.WorkflowMutableState {
	state := &persistence.WorkflowMutableState{}

	state.ActivityInfos = e.pendingActivityInfoIDs
	state.TimerInfos = e.pendingTimerInfoIDs
	state.ChildExecutionInfos = e.pendingChildExecutionInfoIDs
	state.RequestCancelInfos = e.pendingRequestCancelInfoIDs
	state.SignalInfos = e.pendingSignalInfoIDs
	state.SignalRequestedIDs = e.pendingSignalRequestedIDs
	state.ExecutionInfo = e.executionInfo
	state.BufferedEvents = e.bufferedEvents
	state.VersionHistories = e.versionHistories
	state.Checksum = e.checksum
	state.ReplicationState = e.replicationState
	state.ExecutionStats = e.executionStats

	return state
}

func (e *mutableStateBuilder) Load(
	ctx context.Context,
	state *persistence.WorkflowMutableState,
) error {

	e.pendingActivityInfoIDs = state.ActivityInfos
	for _, activityInfo := range state.ActivityInfos {
		e.pendingActivityIDToEventID[activityInfo.ActivityID] = activityInfo.ScheduleID
	}
	e.pendingTimerInfoIDs = state.TimerInfos
	for _, timerInfo := range state.TimerInfos {
		e.pendingTimerEventIDToID[timerInfo.StartedID] = timerInfo.TimerID
	}
	e.pendingChildExecutionInfoIDs = state.ChildExecutionInfos
	e.pendingRequestCancelInfoIDs = state.RequestCancelInfos
	e.pendingSignalInfoIDs = state.SignalInfos
	e.pendingSignalRequestedIDs = state.SignalRequestedIDs
	e.executionInfo = state.ExecutionInfo
	e.bufferedEvents = e.reorderAndFilterDuplicateEvents(state.BufferedEvents, "load")

	e.currentVersion = constants.EmptyVersion
	e.hasBufferedEventsInDB = len(e.bufferedEvents) > 0
	e.stateInDB = state.ExecutionInfo.State
	e.nextEventIDInDB = state.ExecutionInfo.NextEventID
	e.versionHistories = state.VersionHistories
	// TODO: remove this after all 2DC workflows complete
	e.replicationState = state.ReplicationState
	e.checksum = state.Checksum
	e.executionStats = state.ExecutionStats

	e.fillForBackwardsCompatibility()

	if len(state.Checksum.Value) > 0 {
		switch {
		case e.shouldInvalidateChecksum():
			e.checksum = checksum.Checksum{}
			e.metricsClient.IncCounter(metrics.WorkflowContextScope, metrics.MutableStateChecksumInvalidated)
		case e.shouldVerifyChecksum():
			if err := verifyMutableStateChecksum(e, state.Checksum); err != nil {
				// we ignore checksum verification errors for now until this
				// feature is tested and/or we have mechanisms in place to deal
				// with these types of errors
				e.metricsClient.IncCounter(metrics.WorkflowContextScope, metrics.MutableStateChecksumMismatch)
				e.logError("mutable state checksum mismatch",
					tag.WorkflowNextEventID(e.executionInfo.NextEventID),
					tag.WorkflowScheduleID(e.executionInfo.DecisionScheduleID),
					tag.WorkflowStartedID(e.executionInfo.DecisionStartedID),
					tag.Dynamic("timerIDs", maps.Keys(e.pendingTimerInfoIDs)),
					tag.Dynamic("activityIDs", maps.Keys(e.pendingActivityInfoIDs)),
					tag.Dynamic("childIDs", maps.Keys(e.pendingChildExecutionInfoIDs)),
					tag.Dynamic("signalIDs", maps.Keys(e.pendingSignalInfoIDs)),
					tag.Dynamic("cancelIDs", maps.Keys(e.pendingRequestCancelInfoIDs)),
					tag.Error(err))
				if e.enableChecksumFailureRetry() {
					return err
				}
			}
		}
	}

	if e.domainEntry.GetReplicationConfig().IsActiveActive() {
		res, err := e.shard.GetActiveClusterManager().LookupWorkflow(ctx, e.executionInfo.DomainID, e.executionInfo.WorkflowID, e.executionInfo.RunID)
		if err != nil {
			return err
		}
		e.currentVersion = res.FailoverVersion
	}

	return nil
}

func (e *mutableStateBuilder) fillForBackwardsCompatibility() {
	// With https://github.com/uber/cadence/pull/4601 newly introduced DomainID may not be set for older workflows.
	// Here we will fill its value based on previously used domain name.
	for _, info := range e.pendingChildExecutionInfoIDs {
		if info.DomainID == "" && info.DomainNameDEPRECATED != "" {
			domainID, err := e.shard.GetDomainCache().GetDomainID(info.DomainNameDEPRECATED)
			if err != nil {
				e.logError("failed to fill domainId for pending child executions", tag.Error(err))
			}

			info.DomainID = domainID
		}
	}
}

func (e *mutableStateBuilder) GetCurrentBranchToken() ([]byte, error) {
	if e.versionHistories != nil {
		currentVersionHistory, err := e.versionHistories.GetCurrentVersionHistory()
		if err != nil {
			return nil, err
		}
		return currentVersionHistory.GetBranchToken(), nil
	}
	return e.executionInfo.BranchToken, nil
}

func (e *mutableStateBuilder) GetVersionHistories() *persistence.VersionHistories {
	return e.versionHistories
}

// set treeID/historyBranches
func (e *mutableStateBuilder) SetHistoryTree(
	treeID string,
) error {

	initialBranchToken, err := persistence.NewHistoryBranchToken(treeID)
	if err != nil {
		return err
	}
	return e.SetCurrentBranchToken(initialBranchToken)
}

func (e *mutableStateBuilder) SetCurrentBranchToken(
	branchToken []byte,
) error {

	exeInfo := e.GetExecutionInfo()
	if e.versionHistories == nil {
		exeInfo.BranchToken = branchToken
		return nil
	}

	currentVersionHistory, err := e.versionHistories.GetCurrentVersionHistory()
	if err != nil {
		return err
	}
	return currentVersionHistory.SetBranchToken(branchToken)
}

func (e *mutableStateBuilder) SetVersionHistories(
	versionHistories *persistence.VersionHistories,
) error {

	e.versionHistories = versionHistories
	return nil
}

func (e *mutableStateBuilder) GetHistoryBuilder() *HistoryBuilder {
	return e.hBuilder
}

func (e *mutableStateBuilder) SetHistoryBuilder(hBuilder *HistoryBuilder) {
	e.hBuilder = hBuilder
}

func (e *mutableStateBuilder) GetExecutionInfo() *persistence.WorkflowExecutionInfo {
	return e.executionInfo
}

func (e *mutableStateBuilder) FlushBufferedEvents() error {
	// put new events into 2 buckets:
	//  1) if the event was added while there was in-flight decision, then put it in buffered bucket
	//  2) otherwise, put it in committed bucket
	var newBufferedEvents []*types.HistoryEvent
	var newCommittedEvents []*types.HistoryEvent
	for _, event := range e.hBuilder.history {
		if event.ID == constants.BufferedEventID {
			newBufferedEvents = append(newBufferedEvents, event)
		} else {
			newCommittedEvents = append(newCommittedEvents, event)
		}
	}

	// no decision in-flight, flush all buffered events to committed bucket
	if !e.HasInFlightDecision() {
		var allBufferedEvents []*types.HistoryEvent

		// flush persisted buffered events
		if len(e.bufferedEvents) > 0 {
			allBufferedEvents = append(allBufferedEvents, e.bufferedEvents...)
			e.bufferedEvents = nil
		}
		if e.hasBufferedEventsInDB {
			e.clearBufferedEvents = true
		}
		// flush pending buffered events
		allBufferedEvents = append(allBufferedEvents, e.updateBufferedEvents...)
		e.updateBufferedEvents = nil

		// Resolve issues with persistence duplicating or reordering buffered events
		reorderedEvents := e.reorderAndFilterDuplicateEvents(allBufferedEvents, "flush")

		// Put back all the reordered buffer events at the end
		if len(reorderedEvents) > 0 {
			newCommittedEvents = append(newCommittedEvents, reorderedEvents...)
		}

		// flush new buffered events
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
	if e.HasInFlightDecision() && len(newBufferedEvents) > 0 {
		e.updateBufferedEvents = newBufferedEvents
	}

	return nil
}

func (e *mutableStateBuilder) UpdateCurrentVersion(
	version int64,
	forceUpdate bool,
) error {
	before := e.currentVersion
	defer func() {
		e.logger.Debugf("UpdateCurrentVersion for domain %s, wfID %v, version before: %v, version after: %v, forceUpdate: %v",
			e.executionInfo.DomainID, e.executionInfo.WorkflowID, before, e.currentVersion, forceUpdate)
	}()

	if state, _ := e.GetWorkflowStateCloseStatus(); state == persistence.WorkflowStateCompleted {
		// always set current version to last write version when workflow is completed
		lastWriteVersion, err := e.GetLastWriteVersion()
		if err != nil {
			return err
		}
		e.currentVersion = lastWriteVersion
		return nil
	}

	if e.versionHistories != nil {
		versionHistory, err := e.versionHistories.GetCurrentVersionHistory()
		if err != nil {
			return err
		}

		if !versionHistory.IsEmpty() {
			// this make sure current version >= last write version
			versionHistoryItem, err := versionHistory.GetLastItem()
			if err != nil {
				return err
			}
			e.currentVersion = versionHistoryItem.Version
		}

		if version > e.currentVersion || forceUpdate {
			e.currentVersion = version
		}

		return nil
	}
	e.currentVersion = constants.EmptyVersion
	return nil
}

// GetCurrentVersion indicates which cluster this workflow is considered active.
func (e *mutableStateBuilder) GetCurrentVersion() int64 {
	// Legacy TODO: remove this after all 2DC workflows complete
	if e.replicationState != nil {
		e.logger.Debugf("GetCurrentVersion replicationState.CurrentVersion=%v", e.replicationState.CurrentVersion)
		return e.replicationState.CurrentVersion
	}

	if e.versionHistories != nil {
		e.logger.Debugf("GetCurrentVersion versionHistories.CurrentVersion=%v", e.currentVersion)
		return e.currentVersion
	}

	e.logger.Debugf("GetCurrentVersion returning empty version=%v", constants.EmptyVersion)
	return constants.EmptyVersion
}

// TODO: Check all usages of this method and address active-active case if needed.
func (e *mutableStateBuilder) GetStartVersion() (int64, error) {

	if e.versionHistories != nil {
		versionHistory, err := e.versionHistories.GetCurrentVersionHistory()
		if err != nil {
			return 0, err
		}
		firstItem, err := versionHistory.GetFirstItem()
		if err != nil {
			return 0, err
		}
		return firstItem.Version, nil
	}

	return constants.EmptyVersion, nil
}

func (e *mutableStateBuilder) GetLastWriteVersion() (int64, error) {

	// TODO: remove this after all 2DC workflows complete
	if e.replicationState != nil {
		return e.replicationState.LastWriteVersion, nil
	}

	if e.versionHistories != nil {
		versionHistory, err := e.versionHistories.GetCurrentVersionHistory()
		if err != nil {
			return 0, err
		}
		lastItem, err := versionHistory.GetLastItem()
		if err != nil {
			return 0, err
		}
		return lastItem.Version, nil
	}

	return constants.EmptyVersion, nil
}

func (e *mutableStateBuilder) checkAndClearTimerFiredEvent(
	timerID string,
) *types.HistoryEvent {

	var timerEvent *types.HistoryEvent

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

func (e *mutableStateBuilder) trimEventsAfterWorkflowClose(
	input []*types.HistoryEvent,
) []*types.HistoryEvent {

	if len(input) == 0 {
		return input
	}

	nextIndex := 0

loop:
	for _, event := range input {
		nextIndex++

		switch event.GetEventType() {
		case types.EventTypeWorkflowExecutionCompleted,
			types.EventTypeWorkflowExecutionFailed,
			types.EventTypeWorkflowExecutionTimedOut,
			types.EventTypeWorkflowExecutionTerminated,
			types.EventTypeWorkflowExecutionContinuedAsNew,
			types.EventTypeWorkflowExecutionCanceled:

			break loop
		}
	}

	return input[0:nextIndex]
}

func (e *mutableStateBuilder) assignEventIDToBufferedEvents() {
	newCommittedEvents := e.hBuilder.history

	scheduledIDToStartedID := make(map[int64]int64)
	for _, event := range newCommittedEvents {
		if event.ID != constants.BufferedEventID {
			continue
		}

		eventID := e.executionInfo.NextEventID
		event.ID = eventID
		e.executionInfo.IncreaseNextEventID()

		switch event.GetEventType() {
		case types.EventTypeActivityTaskStarted:
			attributes := event.ActivityTaskStartedEventAttributes
			scheduledID := attributes.GetScheduledEventID()
			scheduledIDToStartedID[scheduledID] = eventID
			if ai, ok := e.GetActivityInfo(scheduledID); ok {
				ai.StartedID = eventID
				e.updateActivityInfos[ai.ScheduleID] = ai
			}
		case types.EventTypeChildWorkflowExecutionStarted:
			attributes := event.ChildWorkflowExecutionStartedEventAttributes
			initiatedID := attributes.GetInitiatedEventID()
			scheduledIDToStartedID[initiatedID] = eventID
			if ci, ok := e.GetChildExecutionInfo(initiatedID); ok {
				ci.StartedID = eventID
				e.updateChildExecutionInfos[ci.InitiatedID] = ci
			}
		case types.EventTypeActivityTaskCompleted:
			attributes := event.ActivityTaskCompletedEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetScheduledEventID()]; ok {
				attributes.StartedEventID = startedID
			}
		case types.EventTypeActivityTaskFailed:
			attributes := event.ActivityTaskFailedEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetScheduledEventID()]; ok {
				attributes.StartedEventID = startedID
			}
		case types.EventTypeActivityTaskTimedOut:
			attributes := event.ActivityTaskTimedOutEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetScheduledEventID()]; ok {
				attributes.StartedEventID = startedID
			}
		case types.EventTypeActivityTaskCanceled:
			attributes := event.ActivityTaskCanceledEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetScheduledEventID()]; ok {
				attributes.StartedEventID = startedID
			}
		case types.EventTypeChildWorkflowExecutionCompleted:
			attributes := event.ChildWorkflowExecutionCompletedEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetInitiatedEventID()]; ok {
				attributes.StartedEventID = startedID
			}
		case types.EventTypeChildWorkflowExecutionFailed:
			attributes := event.ChildWorkflowExecutionFailedEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetInitiatedEventID()]; ok {
				attributes.StartedEventID = startedID
			}
		case types.EventTypeChildWorkflowExecutionTimedOut:
			attributes := event.ChildWorkflowExecutionTimedOutEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetInitiatedEventID()]; ok {
				attributes.StartedEventID = startedID
			}
		case types.EventTypeChildWorkflowExecutionCanceled:
			attributes := event.ChildWorkflowExecutionCanceledEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetInitiatedEventID()]; ok {
				attributes.StartedEventID = startedID
			}
		case types.EventTypeChildWorkflowExecutionTerminated:
			attributes := event.ChildWorkflowExecutionTerminatedEventAttributes
			if startedID, ok := scheduledIDToStartedID[attributes.GetInitiatedEventID()]; ok {
				attributes.StartedEventID = startedID
			}
		}
	}
}

func (e *mutableStateBuilder) assignTaskIDToEvents() error {

	// assign task IDs to all history events
	// first transient events
	numTaskIDs := len(e.hBuilder.transientHistory)
	if numTaskIDs > 0 {
		taskIDs, err := e.shard.GenerateTaskIDs(numTaskIDs)
		if err != nil {
			return err
		}

		for index, event := range e.hBuilder.transientHistory {
			if event.TaskID == constants.EmptyEventTaskID {
				taskID := taskIDs[index]
				event.TaskID = taskID
				e.executionInfo.LastEventTaskID = taskID
			}
		}
	}

	// then normal events
	numTaskIDs = len(e.hBuilder.history)
	if numTaskIDs > 0 {
		taskIDs, err := e.shard.GenerateTaskIDs(numTaskIDs)
		if err != nil {
			return err
		}

		for index, event := range e.hBuilder.history {
			if event.TaskID == constants.EmptyEventTaskID {
				taskID := taskIDs[index]
				event.TaskID = taskID
				e.executionInfo.LastEventTaskID = taskID
			}
		}
	}

	return nil
}

func (e *mutableStateBuilder) IsCurrentWorkflowGuaranteed() bool {
	// stateInDB is used like a bloom filter:
	//
	// 1. stateInDB being created / running meaning that this workflow must be the current
	//  workflow (assuming there is no rebuild of mutable state).
	// 2. stateInDB being completed does not guarantee this workflow being the current workflow
	// 3. stateInDB being zombie guarantees this workflow not being the current workflow
	// 4. stateInDB cannot be void, void is only possible when mutable state is just initialized

	switch e.stateInDB {
	case persistence.WorkflowStateVoid:
		return false
	case persistence.WorkflowStateCreated:
		return true
	case persistence.WorkflowStateRunning:
		return true
	case persistence.WorkflowStateCompleted:
		return false
	case persistence.WorkflowStateZombie:
		return false
	case persistence.WorkflowStateCorrupted:
		return false
	default:
		panic(fmt.Sprintf("unknown workflow state: %v", e.executionInfo.State))
	}
}

func (e *mutableStateBuilder) GetDomainEntry() *cache.DomainCacheEntry {
	return e.domainEntry
}

func (e *mutableStateBuilder) IsStickyTaskListEnabled() bool {
	if e.executionInfo.StickyTaskList == "" {
		return false
	}
	ttl := e.config.StickyTTL(e.GetDomainEntry().GetInfo().Name)
	return !e.timeSource.Now().After(e.executionInfo.LastUpdatedTimestamp.Add(ttl))
}

func (e *mutableStateBuilder) CreateNewHistoryEvent(
	eventType types.EventType,
) *types.HistoryEvent {

	return e.CreateNewHistoryEventWithTimestamp(eventType, e.timeSource.Now().UnixNano())
}

func (e *mutableStateBuilder) CreateNewHistoryEventWithTimestamp(
	eventType types.EventType,
	timestamp int64,
) *types.HistoryEvent {
	eventID := e.executionInfo.NextEventID
	if e.shouldBufferEvent(eventType) {
		eventID = constants.BufferedEventID
	} else {
		// only increase NextEventID if event is not buffered
		e.executionInfo.IncreaseNextEventID()
	}

	ts := common.Int64Ptr(timestamp)
	historyEvent := &types.HistoryEvent{}
	historyEvent.ID = eventID
	historyEvent.Timestamp = ts
	historyEvent.EventType = &eventType
	historyEvent.Version = e.GetCurrentVersion()
	historyEvent.TaskID = constants.EmptyEventTaskID

	return historyEvent
}

func (e *mutableStateBuilder) shouldBufferEvent(
	eventType types.EventType,
) bool {

	switch eventType {
	case // do not buffer for workflow state change
		types.EventTypeWorkflowExecutionStarted,
		types.EventTypeWorkflowExecutionCompleted,
		types.EventTypeWorkflowExecutionFailed,
		types.EventTypeWorkflowExecutionTimedOut,
		types.EventTypeWorkflowExecutionTerminated,
		types.EventTypeWorkflowExecutionContinuedAsNew,
		types.EventTypeWorkflowExecutionCanceled:
		return false
	case // decision event should not be buffered
		types.EventTypeDecisionTaskScheduled,
		types.EventTypeDecisionTaskStarted,
		types.EventTypeDecisionTaskCompleted,
		types.EventTypeDecisionTaskFailed,
		types.EventTypeDecisionTaskTimedOut:
		return false
	case // events generated directly from decisions should not be buffered
		// workflow complete, failed, cancelled and continue-as-new events are duplication of above
		// just put is here for reference
		// types.EventTypeWorkflowExecutionCompleted,
		// types.EventTypeWorkflowExecutionFailed,
		// types.EventTypeWorkflowExecutionCanceled,
		// types.EventTypeWorkflowExecutionContinuedAsNew,
		types.EventTypeActivityTaskScheduled,
		types.EventTypeActivityTaskCancelRequested,
		types.EventTypeTimerStarted,
		// DecisionTypeCancelTimer is an exception. This decision will be mapped
		// to either types.EventTypeTimerCanceled, or types.EventTypeCancelTimerFailed.
		// So both should not be buffered. Ref: historyEngine, search for "types.DecisionTypeCancelTimer"
		types.EventTypeTimerCanceled,
		types.EventTypeCancelTimerFailed,
		types.EventTypeRequestCancelExternalWorkflowExecutionInitiated,
		types.EventTypeMarkerRecorded,
		types.EventTypeStartChildWorkflowExecutionInitiated,
		types.EventTypeSignalExternalWorkflowExecutionInitiated,
		types.EventTypeUpsertWorkflowSearchAttributes:
		// do not buffer event if event is directly generated from a corresponding decision

		// sanity check there is no decision on the fly
		if e.HasInFlightDecision() {
			msg := fmt.Sprintf("history mutable state is processing event: %v while there is decision pending. "+
				"domainID: %v, workflow ID: %v, run ID: %v.", eventType, e.executionInfo.DomainID, e.executionInfo.WorkflowID, e.executionInfo.RunID)
			panic(msg)
		}
		return false
	default:
		return true
	}
}

func (e *mutableStateBuilder) GetWorkflowType() *types.WorkflowType {
	wType := &types.WorkflowType{}
	wType.Name = e.executionInfo.WorkflowTypeName

	return wType
}

func (e *mutableStateBuilder) GetQueryRegistry() query.Registry {
	return e.queryRegistry
}

func (e *mutableStateBuilder) SetQueryRegistry(queryRegistry query.Registry) {
	e.queryRegistry = queryRegistry
}

func (e *mutableStateBuilder) GetRetryBackoffDuration(
	errReason string,
) time.Duration {

	info := e.executionInfo
	if !info.HasRetryPolicy {
		return backoff.NoBackoff
	}

	return getBackoffInterval(
		e.timeSource.Now(),
		info.ExpirationTime,
		info.Attempt,
		info.MaximumAttempts,
		info.InitialInterval,
		info.MaximumInterval,
		info.BackoffCoefficient,
		errReason,
		info.NonRetriableErrors,
	)
}

func (e *mutableStateBuilder) GetCronBackoffDuration(
	ctx context.Context,
) (time.Duration, error) {
	info := e.executionInfo
	if len(info.CronSchedule) == 0 {
		return backoff.NoBackoff, nil
	}
	sched, err := backoff.ValidateSchedule(info.CronSchedule)
	if err != nil {
		return backoff.NoBackoff, err
	}
	// TODO: decide if we can add execution time in execution info.
	executionTime := e.executionInfo.StartTimestamp
	// This only call when doing ContinueAsNew. At this point, the workflow should have a start event
	workflowStartEvent, err := e.GetStartEvent(ctx)
	if err != nil {
		e.logError("unable to find workflow start event", tag.ErrorTypeInvalidHistoryAction)
		return backoff.NoBackoff, err
	}
	firstDecisionTaskBackoff :=
		time.Duration(workflowStartEvent.GetWorkflowExecutionStartedEventAttributes().GetFirstDecisionTaskBackoffSeconds()) * time.Second
	executionTime = executionTime.Add(firstDecisionTaskBackoff)
	jitterStartSeconds := workflowStartEvent.GetWorkflowExecutionStartedEventAttributes().GetJitterStartSeconds()
	return backoff.GetBackoffForNextSchedule(sched, executionTime, e.timeSource.Now(), jitterStartSeconds)
}

// GetStartEvent retrieves the workflow start event from mutable state
func (e *mutableStateBuilder) GetStartEvent(
	ctx context.Context,
) (*types.HistoryEvent, error) {

	currentBranchToken, err := e.GetCurrentBranchToken()
	if err != nil {
		return nil, err
	}

	startEvent, err := e.eventsCache.GetEvent(
		ctx,
		e.shard.GetShardID(),
		e.executionInfo.DomainID,
		e.executionInfo.WorkflowID,
		e.executionInfo.RunID,
		constants.FirstEventID,
		constants.FirstEventID,
		currentBranchToken,
	)
	if err != nil {
		// do not return the original error
		// since original error can be of type entity not exists
		// which can cause task processing side to fail silently
		// However, if the error is a persistence transient error,
		// we return the original error, because we fail to get
		// the event because of failure from database
		if persistence.IsTransientError(err) {
			return nil, err
		}
		return nil, ErrMissingWorkflowStartEvent
	}
	return startEvent, nil
}

// DeletePendingRequestCancel deletes details about a RequestCancelInfo.
func (e *mutableStateBuilder) DeletePendingRequestCancel(
	initiatedEventID int64,
) error {

	if _, ok := e.pendingRequestCancelInfoIDs[initiatedEventID]; ok {
		delete(e.pendingRequestCancelInfoIDs, initiatedEventID)
	} else {
		e.logError(
			fmt.Sprintf("unable to find request cancel external workflow event ID: %v in mutable state", initiatedEventID),
			tag.ErrorTypeInvalidMutableStateAction,
		)
		// log data inconsistency instead of returning an error
		e.logDataInconsistency()
	}

	delete(e.updateRequestCancelInfos, initiatedEventID)
	e.deleteRequestCancelInfos[initiatedEventID] = struct{}{}
	return nil
}

func (e *mutableStateBuilder) writeEventToCache(
	event *types.HistoryEvent,
) {

	// For start event: store it within events cache so the recordWorkflowStarted transfer task doesn't need to
	// load it from database
	// For completion event: store it within events cache so we can communicate the result to parent execution
	// during the processing of DeleteTransferTask without loading this event from database
	e.eventsCache.PutEvent(
		e.executionInfo.DomainID,
		e.executionInfo.WorkflowID,
		e.executionInfo.RunID,
		event.ID,
		event,
	)
}

func (e *mutableStateBuilder) HasParentExecution() bool {
	return e.executionInfo.ParentDomainID != "" && e.executionInfo.ParentWorkflowID != ""
}

// GetUserTimerInfoByEventID gives details about a user timer.
func (e *mutableStateBuilder) GetUserTimerInfoByEventID(
	startEventID int64,
) (*persistence.TimerInfo, bool) {

	timerID, ok := e.pendingTimerEventIDToID[startEventID]
	if !ok {
		return nil, false
	}
	return e.GetUserTimerInfo(timerID)
}

func (e *mutableStateBuilder) GetPendingRequestCancelExternalInfos() map[int64]*persistence.RequestCancelInfo {
	return e.pendingRequestCancelInfoIDs
}

func (e *mutableStateBuilder) HasBufferedEvents() bool {
	if len(e.bufferedEvents) > 0 || len(e.updateBufferedEvents) > 0 {
		return true
	}

	for _, event := range e.hBuilder.history {
		if event.ID == constants.BufferedEventID {
			return true
		}
	}

	return false
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
	return e.executionInfo.IsRunning()
}

func (e *mutableStateBuilder) AddUpsertWorkflowSearchAttributesEvent(
	decisionCompletedEventID int64,
	request *types.UpsertWorkflowSearchAttributesDecisionAttributes,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionUpsertWorkflowSearchAttributes
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	event := e.hBuilder.AddUpsertWorkflowSearchAttributesEvent(decisionCompletedEventID, request)
	if err := e.ReplicateUpsertWorkflowSearchAttributesEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *mutableStateBuilder) ReplicateUpsertWorkflowSearchAttributesEvent(
	event *types.HistoryEvent,
) error {

	upsertSearchAttr := event.UpsertWorkflowSearchAttributesEventAttributes.GetSearchAttributes().GetIndexedFields()
	currentSearchAttr := e.GetExecutionInfo().SearchAttributes

	e.executionInfo.SearchAttributes = mergeMapOfByteArray(currentSearchAttr, upsertSearchAttr)

	return e.taskGenerator.GenerateWorkflowSearchAttrTasks()
}

func (e *mutableStateBuilder) AddRecordMarkerEvent(
	decisionCompletedEventID int64,
	attributes *types.RecordMarkerDecisionAttributes,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowRecordMarker
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	return e.hBuilder.AddMarkerRecordedEvent(decisionCompletedEventID, attributes), nil
}

func (e *mutableStateBuilder) AddWorkflowExecutionTerminatedEvent(
	firstEventID int64,
	reason string,
	details []byte,
	identity string,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionWorkflowTerminated
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	event := e.hBuilder.AddWorkflowExecutionTerminatedEvent(reason, details, identity)
	if err := e.ReplicateWorkflowExecutionTerminatedEvent(firstEventID, event); err != nil {
		return nil, err
	}

	domainName := e.GetDomainEntry().GetInfo().Name

	e.logger.Info(
		"Workflow execution terminated.",
		tag.WorkflowDomainName(domainName),
		tag.WorkflowID(e.GetExecutionInfo().WorkflowID),
		tag.WorkflowRunID(e.GetExecutionInfo().RunID),
		tag.WorkflowTerminationReason(reason),
	)

	scopeWithDomainTag := e.metricsClient.Scope(metrics.HistoryTerminateWorkflowExecutionScope).
		Tagged(metrics.DomainTag(domainName)).
		Tagged(metrics.WorkflowTerminationReasonTag(reason))
	scopeWithDomainTag.IncCounter(metrics.WorkflowTerminateCounterPerDomain)

	return event, nil
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionTerminatedEvent(
	firstEventID int64,
	event *types.HistoryEvent,
) error {

	if err := e.UpdateWorkflowStateCloseStatus(
		persistence.WorkflowStateCompleted,
		persistence.WorkflowCloseStatusTerminated,
	); err != nil {
		return err
	}
	e.executionInfo.CompletionEventBatchID = firstEventID // Used when completion event needs to be loaded from database
	e.ClearStickyness()
	e.writeEventToCache(event)

	return e.taskGenerator.GenerateWorkflowCloseTasks(event, e.config.WorkflowDeletionJitterRange(e.domainEntry.GetInfo().Name))
}

func (e *mutableStateBuilder) AddContinueAsNewEvent(
	ctx context.Context,
	firstEventID int64,
	decisionCompletedEventID int64,
	parentDomainName string,
	attributes *types.ContinueAsNewWorkflowExecutionDecisionAttributes,
) (*types.HistoryEvent, MutableState, error) {

	opTag := tag.WorkflowActionWorkflowContinueAsNew
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, err
	}

	var err error
	newRunID := uuid.New()
	newExecution := types.WorkflowExecution{
		WorkflowID: e.executionInfo.WorkflowID,
		RunID:      newRunID,
	}

	// Extract ParentExecutionInfo from current run so it can be passed down to the next
	var parentInfo *types.ParentExecutionInfo
	if e.HasParentExecution() {
		parentInfo = &types.ParentExecutionInfo{
			DomainUUID: e.executionInfo.ParentDomainID,
			Domain:     parentDomainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: e.executionInfo.ParentWorkflowID,
				RunID:      e.executionInfo.ParentRunID,
			},
			InitiatedID: e.executionInfo.InitiatedID,
		}
	}

	continueAsNewEvent := e.hBuilder.AddContinuedAsNewEvent(decisionCompletedEventID, newRunID, attributes)
	currentStartEvent, err := e.GetStartEvent(ctx)
	if err != nil {
		return nil, nil, err
	}
	firstRunID := e.executionInfo.FirstExecutionRunID
	// This is needed for backwards compatibility.  Workflow execution create with Cadence release v0.25.0 or earlier
	// does not have FirstExecutionRunID stored as part of mutable state.  If this is not set then load it from
	// workflow execution started event.
	if len(firstRunID) == 0 {
		firstRunID = currentStartEvent.GetWorkflowExecutionStartedEventAttributes().GetFirstExecutionRunID()
	}
	firstScheduleTime := currentStartEvent.GetWorkflowExecutionStartedEventAttributes().GetFirstScheduledTime()
	domainID := e.domainEntry.GetInfo().ID
	newStateBuilder := NewMutableStateBuilderWithVersionHistories(
		e.shard,
		e.logger,
		e.domainEntry,
	).(*mutableStateBuilder)

	if _, err = newStateBuilder.addWorkflowExecutionStartedEventForContinueAsNew(
		parentInfo,
		newExecution,
		e,
		attributes,
		firstRunID,
		firstScheduleTime,
	); err != nil {
		return nil, nil, &types.InternalServiceError{Message: "Failed to add workflow execution started event."}
	}

	if err = e.ReplicateWorkflowExecutionContinuedAsNewEvent(
		firstEventID,
		domainID,
		continueAsNewEvent,
	); err != nil {
		return nil, nil, err
	}

	return continueAsNewEvent, newStateBuilder, nil
}

func rolloverAutoResetPointsWithExpiringTime(
	resetPoints *types.ResetPoints,
	prevRunID string,
	nowNano int64,
	domainRetentionDays int32,
) *types.ResetPoints {

	if resetPoints == nil || resetPoints.Points == nil {
		return resetPoints
	}
	newPoints := make([]*types.ResetPointInfo, 0, len(resetPoints.Points))
	expiringTimeNano := nowNano + int64(time.Duration(domainRetentionDays)*time.Hour*24)
	for _, rp := range resetPoints.Points {
		if rp.GetRunID() == prevRunID {
			rp.ExpiringTimeNano = common.Int64Ptr(expiringTimeNano)
		}
		newPoints = append(newPoints, rp)
	}
	return &types.ResetPoints{
		Points: newPoints,
	}
}

func (e *mutableStateBuilder) ReplicateWorkflowExecutionContinuedAsNewEvent(
	firstEventID int64,
	domainID string,
	continueAsNewEvent *types.HistoryEvent,
) error {

	if err := e.UpdateWorkflowStateCloseStatus(
		persistence.WorkflowStateCompleted,
		persistence.WorkflowCloseStatusContinuedAsNew,
	); err != nil {
		return err
	}
	e.executionInfo.CompletionEventBatchID = firstEventID // Used when completion event needs to be loaded from database
	e.ClearStickyness()
	e.writeEventToCache(continueAsNewEvent)

	return e.taskGenerator.GenerateWorkflowCloseTasks(continueAsNewEvent, e.config.WorkflowDeletionJitterRange(e.domainEntry.GetInfo().Name))
}

// TODO mutable state should generate corresponding transfer / timer tasks according to
//  updates accumulated, while currently all transfer / timer tasks are managed manually

// TODO convert AddTransferTasks to prepareTransferTasks
func (e *mutableStateBuilder) AddTransferTasks(
	transferTasks ...persistence.Task,
) {

	e.insertTransferTasks = append(e.insertTransferTasks, transferTasks...)
}

// TODO convert AddTimerTasks to prepareTimerTasks
func (e *mutableStateBuilder) AddTimerTasks(
	timerTasks ...persistence.Task,
) {

	e.insertTimerTasks = append(e.insertTimerTasks, timerTasks...)
}

func (e *mutableStateBuilder) GetTransferTasks() []persistence.Task {
	return e.insertTransferTasks
}

func (e *mutableStateBuilder) GetTimerTasks() []persistence.Task {
	return e.insertTimerTasks
}

func (e *mutableStateBuilder) DeleteTransferTasks() {
	e.insertTransferTasks = nil
}

func (e *mutableStateBuilder) DeleteTimerTasks() {
	e.insertTimerTasks = nil
}

func (e *mutableStateBuilder) SetUpdateCondition(
	nextEventIDInDB int64,
) {

	e.nextEventIDInDB = nextEventIDInDB
}

func (e *mutableStateBuilder) GetUpdateCondition() int64 {
	return e.nextEventIDInDB
}

func (e *mutableStateBuilder) GetWorkflowStateCloseStatus() (int, int) {

	executionInfo := e.executionInfo
	return executionInfo.State, executionInfo.CloseStatus
}

func (e *mutableStateBuilder) UpdateWorkflowStateCloseStatus(
	state int,
	closeStatus int,
) error {

	return e.executionInfo.UpdateWorkflowStateCloseStatus(state, closeStatus)
}

func (e *mutableStateBuilder) StartTransaction(
	ctx context.Context,
	domainEntry *cache.DomainCacheEntry,
	incomingTaskVersion int64,
) (bool, error) {
	e.domainEntry = domainEntry
	version := domainEntry.GetFailoverVersion()
	if e.domainEntry.GetReplicationConfig().IsActiveActive() {
		res, err := e.shard.GetActiveClusterManager().LookupWorkflow(ctx, e.executionInfo.DomainID, e.executionInfo.WorkflowID, e.executionInfo.RunID)
		if err != nil {
			return false, err
		}
		version = res.FailoverVersion
	}

	if e.logger.DebugOn() {
		e.logger.Debugf("StartTransaction calling UpdateCurrentVersion for domain %s, wfID %v, incomingTaskVersion %v, version %v, stacktrace %v",
			domainEntry.GetInfo().Name, e.executionInfo.WorkflowID, incomingTaskVersion, version, string(debug.Stack()))
	}
	if err := e.UpdateCurrentVersion(version, false); err != nil {
		return false, err
	}

	flushBeforeReady, err := e.startTransactionHandleDecisionFailover(incomingTaskVersion)
	if err != nil {
		return false, err
	}

	return flushBeforeReady, nil
}

func (e *mutableStateBuilder) CloseTransactionAsMutation(
	now time.Time,
	transactionPolicy TransactionPolicy,
) (*persistence.WorkflowMutation, []*persistence.WorkflowEvents, error) {

	if err := e.prepareCloseTransaction(
		transactionPolicy,
	); err != nil {
		return nil, nil, err
	}

	workflowEventsSeq, err := e.prepareEventsAndReplicationTasks(transactionPolicy)
	if err != nil {
		return nil, nil, err
	}

	if len(workflowEventsSeq) > 0 {
		lastEvents := workflowEventsSeq[len(workflowEventsSeq)-1].Events
		firstEvent := lastEvents[0]
		lastEvent := lastEvents[len(lastEvents)-1]
		e.updateWithLastFirstEvent(firstEvent)
		if err := e.updateWithLastWriteEvent(
			lastEvent,
			transactionPolicy,
		); err != nil {
			return nil, nil, err
		}
	}

	// update last update time
	e.executionInfo.LastUpdatedTimestamp = now

	// we generate checksum here based on the assumption that the returned
	// snapshot object is considered immutable. As of this writing, the only
	// code that modifies the returned object lives inside workflowExecutionContext.resetWorkflowExecution
	// currently, the updates done inside workflowExecutionContext.resetWorkflowExecution doesn't
	// impact the checksum calculation
	checksum := e.generateChecksum()

	workflowMutation := &persistence.WorkflowMutation{
		ExecutionInfo:    e.executionInfo,
		VersionHistories: e.versionHistories,

		UpsertActivityInfos:       maps.Values(e.updateActivityInfos),
		DeleteActivityInfos:       maps.Keys(e.deleteActivityInfos),
		UpsertTimerInfos:          maps.Values(e.updateTimerInfos),
		DeleteTimerInfos:          maps.Keys(e.deleteTimerInfos),
		UpsertChildExecutionInfos: maps.Values(e.updateChildExecutionInfos),
		DeleteChildExecutionInfos: maps.Keys(e.deleteChildExecutionInfos),
		UpsertRequestCancelInfos:  maps.Values(e.updateRequestCancelInfos),
		DeleteRequestCancelInfos:  maps.Keys(e.deleteRequestCancelInfos),
		UpsertSignalInfos:         maps.Values(e.updateSignalInfos),
		DeleteSignalInfos:         maps.Keys(e.deleteSignalInfos),
		UpsertSignalRequestedIDs:  maps.Keys(e.updateSignalRequestedIDs),
		DeleteSignalRequestedIDs:  maps.Keys(e.deleteSignalRequestedIDs),
		NewBufferedEvents:         e.updateBufferedEvents,
		ClearBufferedEvents:       e.clearBufferedEvents,

		TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
			persistence.HistoryTaskCategoryTransfer:    e.insertTransferTasks,
			persistence.HistoryTaskCategoryReplication: e.insertReplicationTasks,
			persistence.HistoryTaskCategoryTimer:       e.insertTimerTasks,
		},

		WorkflowRequests: convertWorkflowRequests(e.workflowRequests),

		Condition: e.nextEventIDInDB,
		Checksum:  checksum,
	}

	e.checksum = checksum
	if err := e.cleanupTransaction(); err != nil {
		return nil, nil, err
	}
	return workflowMutation, workflowEventsSeq, nil
}

func (e *mutableStateBuilder) CloseTransactionAsSnapshot(
	now time.Time,
	transactionPolicy TransactionPolicy,
) (*persistence.WorkflowSnapshot, []*persistence.WorkflowEvents, error) {

	if err := e.prepareCloseTransaction(
		transactionPolicy,
	); err != nil {
		return nil, nil, err
	}

	workflowEventsSeq, err := e.prepareEventsAndReplicationTasks(transactionPolicy)
	if err != nil {
		return nil, nil, err
	}

	if len(workflowEventsSeq) > 1 {
		return nil, nil, &types.InternalServiceError{
			Message: "cannot generate workflow snapshot with transient events",
		}
	}
	if len(e.bufferedEvents) > 0 {
		// TODO do we need the functionality to generate snapshot with buffered events?
		return nil, nil, &types.InternalServiceError{
			Message: "cannot generate workflow snapshot with buffered events",
		}
	}

	if len(workflowEventsSeq) > 0 {
		lastEvents := workflowEventsSeq[len(workflowEventsSeq)-1].Events
		firstEvent := lastEvents[0]
		lastEvent := lastEvents[len(lastEvents)-1]
		e.updateWithLastFirstEvent(firstEvent)
		if err := e.updateWithLastWriteEvent(
			lastEvent,
			transactionPolicy,
		); err != nil {
			return nil, nil, err
		}
	}

	// update last update time
	e.executionInfo.LastUpdatedTimestamp = now

	// we generate checksum here based on the assumption that the returned
	// snapshot object is considered immutable. As of this writing, the only
	// code that modifies the returned object lives inside workflowExecutionContext.resetWorkflowExecution
	// currently, the updates done inside workflowExecutionContext.resetWorkflowExecution doesn't
	// impact the checksum calculation
	checksum := e.generateChecksum()

	workflowSnapshot := &persistence.WorkflowSnapshot{
		ExecutionInfo:    e.executionInfo,
		VersionHistories: e.versionHistories,

		ActivityInfos:       maps.Values(e.pendingActivityInfoIDs),
		TimerInfos:          maps.Values(e.pendingTimerInfoIDs),
		ChildExecutionInfos: maps.Values(e.pendingChildExecutionInfoIDs),
		RequestCancelInfos:  maps.Values(e.pendingRequestCancelInfoIDs),
		SignalInfos:         maps.Values(e.pendingSignalInfoIDs),
		SignalRequestedIDs:  maps.Keys(e.pendingSignalRequestedIDs),

		TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
			persistence.HistoryTaskCategoryTransfer:    e.insertTransferTasks,
			persistence.HistoryTaskCategoryReplication: e.insertReplicationTasks,
			persistence.HistoryTaskCategoryTimer:       e.insertTimerTasks,
		},

		WorkflowRequests: convertWorkflowRequests(e.workflowRequests),

		Condition: e.nextEventIDInDB,
		Checksum:  checksum,
	}

	e.checksum = checksum
	if err := e.cleanupTransaction(); err != nil {
		return nil, nil, err
	}
	return workflowSnapshot, workflowEventsSeq, nil
}

func (e *mutableStateBuilder) IsResourceDuplicated(
	resourceDedupKey definition.DeduplicationID,
) bool {
	id := definition.GenerateDeduplicationKey(resourceDedupKey)
	_, duplicated := e.appliedEvents[id]
	return duplicated
}

func (e *mutableStateBuilder) UpdateDuplicatedResource(
	resourceDedupKey definition.DeduplicationID,
) {
	id := definition.GenerateDeduplicationKey(resourceDedupKey)
	e.appliedEvents[id] = struct{}{}
}

func (e *mutableStateBuilder) GetHistorySize() int64 {
	return e.executionStats.HistorySize
}

func (e *mutableStateBuilder) SetHistorySize(size int64) {
	e.executionStats.HistorySize = size
}

func (e *mutableStateBuilder) ByteSize() uint64 {
	// TODO: To be implemented
	return 0
}

func (e *mutableStateBuilder) prepareCloseTransaction(
	transactionPolicy TransactionPolicy,
) error {

	if err := e.closeTransactionWithPolicyCheck(
		transactionPolicy,
	); err != nil {
		return err
	}

	if err := e.closeTransactionHandleBufferedEventsLimit(
		transactionPolicy,
	); err != nil {
		return err
	}

	if err := e.closeTransactionHandleWorkflowReset(
		transactionPolicy,
	); err != nil {
		return err
	}

	// flushing buffered events should happen at very last
	if transactionPolicy == TransactionPolicyActive {
		if err := e.FlushBufferedEvents(); err != nil {
			return err
		}
	}

	// NOTE: this function must be the last call
	//  since we only generate at most one activity & user timer,
	//  regardless of how many activity & user timer created
	//  so the calculation must be at the very end
	return e.closeTransactionHandleActivityUserTimerTasks()
}

func (e *mutableStateBuilder) cleanupTransaction() error {

	// Clear all updates to prepare for the next session
	e.hBuilder = NewHistoryBuilder(e)

	e.updateActivityInfos = make(map[int64]*persistence.ActivityInfo)
	e.deleteActivityInfos = make(map[int64]struct{})
	e.syncActivityTasks = make(map[int64]struct{})

	e.updateTimerInfos = make(map[string]*persistence.TimerInfo)
	e.deleteTimerInfos = make(map[string]struct{})

	e.updateChildExecutionInfos = make(map[int64]*persistence.ChildExecutionInfo)
	e.deleteChildExecutionInfos = make(map[int64]struct{})

	e.updateRequestCancelInfos = make(map[int64]*persistence.RequestCancelInfo)
	e.deleteRequestCancelInfos = make(map[int64]struct{})

	e.updateSignalInfos = make(map[int64]*persistence.SignalInfo)
	e.deleteSignalInfos = make(map[int64]struct{})

	e.updateSignalRequestedIDs = make(map[string]struct{})
	e.deleteSignalRequestedIDs = make(map[string]struct{})

	e.clearBufferedEvents = false
	if e.updateBufferedEvents != nil {
		e.bufferedEvents = append(e.bufferedEvents, e.updateBufferedEvents...)
		e.updateBufferedEvents = nil
	}

	e.hasBufferedEventsInDB = len(e.bufferedEvents) > 0
	e.stateInDB = e.executionInfo.State
	e.nextEventIDInDB = e.GetNextEventID()

	e.insertTransferTasks = nil
	e.insertReplicationTasks = nil
	e.insertTimerTasks = nil

	e.workflowRequests = make(map[persistence.WorkflowRequest]struct{})
	return nil
}

func (e *mutableStateBuilder) prepareEventsAndReplicationTasks(
	transactionPolicy TransactionPolicy,
) ([]*persistence.WorkflowEvents, error) {

	currentBranchToken, err := e.GetCurrentBranchToken()
	if err != nil {
		return nil, err
	}
	var workflowEventsSeq []*persistence.WorkflowEvents
	if len(e.hBuilder.transientHistory) != 0 {
		workflowEventsSeq = append(workflowEventsSeq, &persistence.WorkflowEvents{
			DomainID:    e.executionInfo.DomainID,
			WorkflowID:  e.executionInfo.WorkflowID,
			RunID:       e.executionInfo.RunID,
			BranchToken: currentBranchToken,
			Events:      e.hBuilder.transientHistory,
		})
	}
	if len(e.hBuilder.history) != 0 {
		workflowEventsSeq = append(workflowEventsSeq, &persistence.WorkflowEvents{
			DomainID:    e.executionInfo.DomainID,
			WorkflowID:  e.executionInfo.WorkflowID,
			RunID:       e.executionInfo.RunID,
			BranchToken: currentBranchToken,
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
		replicationTasks, err := e.eventsToReplicationTask(transactionPolicy, workflowEvents.Events)
		if err != nil {
			return nil, err
		}
		e.insertReplicationTasks = append(
			e.insertReplicationTasks,
			replicationTasks...,
		)
	}

	e.insertReplicationTasks = append(
		e.insertReplicationTasks,
		e.syncActivityToReplicationTask(transactionPolicy)...,
	)

	if transactionPolicy == TransactionPolicyPassive && len(e.insertReplicationTasks) > 0 {
		return nil, &types.InternalServiceError{
			Message: "should not generate replication task when close transaction as passive",
		}
	}

	return workflowEventsSeq, nil
}

func (e *mutableStateBuilder) eventsToReplicationTask(
	transactionPolicy TransactionPolicy,
	events []*types.HistoryEvent,
) ([]persistence.Task, error) {

	if transactionPolicy == TransactionPolicyPassive ||
		!e.canReplicateEvents() ||
		len(events) == 0 {
		return emptyTasks, nil
	}

	firstEvent := events[0]
	lastEvent := events[len(events)-1]
	version := firstEvent.Version

	// Check all the events in the transaction belongs to the same cluster
	sourceCluster, err := e.shard.GetActiveClusterManager().ClusterNameForFailoverVersion(version, e.executionInfo.DomainID)
	if err != nil {
		return nil, err
	}
	currentCluster := e.clusterMetadata.GetCurrentClusterName()

	if currentCluster != sourceCluster {
		return nil, &types.InternalServiceError{
			Message: "mutableStateBuilder encounter contradicting version & transaction policy",
		}
	}

	currentBranchToken, err := e.GetCurrentBranchToken()
	if err != nil {
		return nil, err
	}

	// the visibility timestamp will be set in shard context
	replicationTask := &persistence.HistoryReplicationTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   e.executionInfo.DomainID,
			WorkflowID: e.executionInfo.WorkflowID,
			RunID:      e.executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			Version: firstEvent.Version,
		},
		FirstEventID:      firstEvent.ID,
		NextEventID:       lastEvent.ID + 1,
		BranchToken:       currentBranchToken,
		NewRunBranchToken: nil,
	}

	e.logger.Debugf("eventsToReplicationTask returning replicationTask. Version: %v, FirstEventID: %v, NextEventID: %v, SourceCluster: %v, CurrentCluster: %v",
		replicationTask.Version,
		replicationTask.FirstEventID,
		replicationTask.NextEventID,
		sourceCluster,
		currentCluster,
	)

	return []persistence.Task{replicationTask}, nil
}

func (e *mutableStateBuilder) syncActivityToReplicationTask(
	transactionPolicy TransactionPolicy,
) []persistence.Task {

	if transactionPolicy == TransactionPolicyPassive ||
		!e.canReplicateEvents() {
		return emptyTasks
	}

	return convertSyncActivityInfos(
		e.executionInfo,
		e.pendingActivityInfoIDs,
		e.syncActivityTasks,
	)
}

func (e *mutableStateBuilder) updateWithLastWriteEvent(
	lastEvent *types.HistoryEvent,
	transactionPolicy TransactionPolicy,
) error {

	if transactionPolicy == TransactionPolicyPassive {
		// already handled in state builder
		return nil
	}

	e.GetExecutionInfo().LastEventTaskID = lastEvent.TaskID

	if e.versionHistories != nil {
		currentVersionHistory, err := e.versionHistories.GetCurrentVersionHistory()
		if err != nil {
			return err
		}
		if err := currentVersionHistory.AddOrUpdateItem(persistence.NewVersionHistoryItem(
			lastEvent.ID, lastEvent.Version,
		)); err != nil {
			return err
		}
	}
	return nil
}

func (e *mutableStateBuilder) updateWithLastFirstEvent(
	lastFirstEvent *types.HistoryEvent,
) {
	e.GetExecutionInfo().SetLastFirstEventID(lastFirstEvent.ID)
}

func (e *mutableStateBuilder) canReplicateEvents() bool {
	if e.domainEntry.GetReplicationPolicy() == cache.ReplicationPolicyOneCluster {
		return false
	}
	// ReplicationPolicyMultiCluster
	domainID := e.domainEntry.GetInfo().ID
	workflowID := e.GetExecutionInfo().WorkflowID
	return e.shard.GetConfig().EnableReplicationTaskGeneration(domainID, workflowID)
}

// validateNoEventsAfterWorkflowFinish perform check on history event batch
// NOTE: do not apply this check on every batch, since transient
// decision && workflow finish will be broken (the first batch)
func (e *mutableStateBuilder) validateNoEventsAfterWorkflowFinish(
	transactionPolicy TransactionPolicy,
	events []*types.HistoryEvent,
) error {

	if transactionPolicy == TransactionPolicyPassive ||
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
	case types.EventTypeWorkflowExecutionCompleted,
		types.EventTypeWorkflowExecutionFailed,
		types.EventTypeWorkflowExecutionTimedOut,
		types.EventTypeWorkflowExecutionTerminated,
		types.EventTypeWorkflowExecutionContinuedAsNew,
		types.EventTypeWorkflowExecutionCanceled:
		return nil

	default:
		executionInfo := e.GetExecutionInfo()
		e.logError(
			"encounter case where events appears after workflow finish.",
			tag.WorkflowDomainID(executionInfo.DomainID),
			tag.WorkflowID(executionInfo.WorkflowID),
			tag.WorkflowRunID(executionInfo.RunID),
		)
		return ErrEventsAfterWorkflowFinish
	}
}

func (e *mutableStateBuilder) startTransactionHandleDecisionFailover(
	incomingTaskVersion int64,
) (bool, error) {

	if !e.IsWorkflowExecutionRunning() ||
		!e.canReplicateEvents() {
		return false, nil
	}

	// NOTE:
	// the main idea here is to guarantee that once there is a decision task started
	// all events ending in the buffer should have the same version

	// Handling mutable state turn from standby to active, while having a decision on the fly
	decision, ok := e.GetInFlightDecision()
	if !ok || decision.Version >= e.GetCurrentVersion() {
		// no pending decision, no buffered events
		// or decision has higher / equal version
		return false, nil
	}

	lastWriteVersion, err := e.GetLastWriteVersion()
	if err != nil {
		return false, err
	}
	if lastWriteVersion != decision.Version {
		return false, &types.InternalServiceError{Message: fmt.Sprintf(
			"mutableStateBuilder encounter mismatch version, decision: %v, last write version %v",
			decision.Version,
			lastWriteVersion,
		)}
	}

	lastWriteSourceCluster, err := e.shard.GetActiveClusterManager().ClusterNameForFailoverVersion(lastWriteVersion, e.executionInfo.DomainID)
	if err != nil {
		return false, err
	}

	currentVersion := e.GetCurrentVersion()
	currentVersionCluster, err := e.shard.GetActiveClusterManager().ClusterNameForFailoverVersion(currentVersion, e.executionInfo.DomainID)
	if err != nil {
		return false, err
	}
	currentCluster := e.clusterMetadata.GetCurrentClusterName()

	// there are 4 cases for version changes (based on version from domain cache)
	// NOTE: domain cache version change may occur after seeing events with higher version
	//  meaning that the flush buffer logic in NDC branch manager should be kept.
	//
	// 1. active -> passive => fail decision & flush buffer using last write version
	// 2. active -> active => fail decision & flush buffer using last write version
	// 3. passive -> active => fail decision using current version, no buffered events
	// 4. passive -> passive => no buffered events, since always passive, nothing to be done
	// 5. special case: current cluster is passive. Due to some reason, the history generated by the current cluster
	// is missing and the missing history replicate back from remote cluster via resending approach => nothing to do

	e.logger.Debugf("startTransactionHandleDecisionFailover incomingTaskVersion %v, lastWriteVersion %v, currentVersion %v, currentCluster %v, lastWriteSourceCluster %v, currentVersionCluster %v",
		incomingTaskVersion,
		lastWriteVersion,
		currentVersion,
		currentCluster,
		lastWriteSourceCluster,
		currentVersionCluster,
	)
	// handle case 5
	incomingTaskSourceCluster, err := e.shard.GetActiveClusterManager().ClusterNameForFailoverVersion(incomingTaskVersion, e.executionInfo.DomainID)
	if err != nil {
		return false, err
	}
	if incomingTaskVersion != constants.EmptyVersion &&
		currentVersionCluster != currentCluster &&
		incomingTaskSourceCluster == currentCluster {
		return false, nil
	}

	// handle case 4
	if lastWriteSourceCluster != currentCluster && currentVersionCluster != currentCluster {
		// do a sanity check on buffered events
		if e.HasBufferedEvents() {
			return false, &types.InternalServiceError{
				Message: "mutableStateBuilder encounter previous passive workflow with buffered events",
			}
		}
		return false, nil
	}

	// handle case 1 & 2
	var flushBufferVersion = lastWriteVersion

	// handle case 3
	if lastWriteSourceCluster != currentCluster && currentVersionCluster == currentCluster {
		// do a sanity check on buffered events
		if e.HasBufferedEvents() {
			return false, &types.InternalServiceError{
				Message: "mutableStateBuilder encounter previous passive workflow with buffered events",
			}
		}
		flushBufferVersion = currentVersion
	}

	// this workflow was previous active (whether it has buffered events or not),
	// the in flight decision must be failed to guarantee all events within same
	// event batch shard the same version
	e.logger.Debugf("startTransactionHandleDecisionFailover calling UpdateCurrentVersion for domain %s, wfID %v, flushBufferVersion %v",
		e.executionInfo.DomainID, e.executionInfo.WorkflowID, flushBufferVersion)
	if err := e.UpdateCurrentVersion(flushBufferVersion, true); err != nil {
		return false, err
	}

	// we have a decision with buffered events on the fly with a lower version, fail it
	if err := FailDecision(
		e,
		decision,
		types.DecisionTaskFailedCauseFailoverCloseDecision,
	); err != nil {
		return false, err
	}

	err = ScheduleDecision(e)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (e *mutableStateBuilder) closeTransactionWithPolicyCheck(
	transactionPolicy TransactionPolicy,
) error {

	if transactionPolicy == TransactionPolicyPassive ||
		!e.canReplicateEvents() {
		return nil
	}

	activeCluster, err := e.shard.GetActiveClusterManager().ClusterNameForFailoverVersion(e.GetCurrentVersion(), e.executionInfo.DomainID)
	if err != nil {
		return err
	}
	currentCluster := e.clusterMetadata.GetCurrentClusterName()

	if activeCluster != currentCluster {
		e.logger.Debugf("closeTransactionWithPolicyCheck activeCluster != currentCluster, activeCluster=%v, currentCluster=%v, e.GetCurrentVersion()=%v", activeCluster, currentCluster, e.GetCurrentVersion())
		domainID := e.GetExecutionInfo().DomainID
		return errors.NewDomainNotActiveError(domainID, currentCluster, activeCluster)
	}
	return nil
}

func (e *mutableStateBuilder) closeTransactionHandleBufferedEventsLimit(
	transactionPolicy TransactionPolicy,
) error {

	if transactionPolicy == TransactionPolicyPassive ||
		!e.IsWorkflowExecutionRunning() {
		return nil
	}

	if len(e.bufferedEvents) < e.config.MaximumBufferedEventsBatch() {
		return nil
	}

	// Handling buffered events size issue
	if decision, ok := e.GetInFlightDecision(); ok {
		// we have a decision on the fly with a lower version, fail it
		if err := FailDecision(
			e,
			decision,
			types.DecisionTaskFailedCauseForceCloseDecision,
		); err != nil {
			return err
		}

		err := ScheduleDecision(e)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *mutableStateBuilder) closeTransactionHandleWorkflowReset(
	transactionPolicy TransactionPolicy,
) error {

	if transactionPolicy == TransactionPolicyPassive ||
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
		if err := e.taskGenerator.GenerateWorkflowResetTasks(); err != nil {
			return err
		}
		e.logInfo("Auto-Reset task is scheduled",
			tag.WorkflowDomainName(domainEntry.GetInfo().Name),
			tag.WorkflowID(executionInfo.WorkflowID),
			tag.WorkflowRunID(executionInfo.RunID),
			tag.WorkflowResetBaseRunID(pt.GetRunID()),
			tag.WorkflowEventID(pt.GetFirstDecisionCompletedID()),
			tag.WorkflowBinaryChecksum(pt.GetBinaryChecksum()),
		)
	}
	return nil
}

func (e *mutableStateBuilder) closeTransactionHandleActivityUserTimerTasks() error {
	if !e.IsWorkflowExecutionRunning() {
		return nil
	}

	if err := e.taskGenerator.GenerateActivityTimerTasks(); err != nil {
		return err
	}

	return e.taskGenerator.GenerateUserTimerTasks()
}

func (e *mutableStateBuilder) checkMutability(
	actionTag tag.Tag,
) error {

	if !e.IsWorkflowExecutionRunning() {
		e.logWarn(
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

func (e *mutableStateBuilder) insertWorkflowRequest(request persistence.WorkflowRequest) {
	if e.domainEntry != nil && e.config.EnableStrongIdempotency(e.domainEntry.GetInfo().Name) && request.RequestID != "" {
		if _, ok := e.workflowRequests[request]; ok {
			e.logWarn("error encountering duplicate request", tag.WorkflowRequestID(request.RequestID))
		}
		e.workflowRequests[request] = struct{}{}
	}
}

func (e *mutableStateBuilder) generateChecksum() checksum.Checksum {
	if !e.shouldGenerateChecksum() {
		return checksum.Checksum{}
	}
	csum, err := generateMutableStateChecksum(e)
	if err != nil {
		e.logWarn("error generating mutableState checksum", tag.Error(err))
		return checksum.Checksum{}
	}
	return csum
}

func (e *mutableStateBuilder) shouldGenerateChecksum() bool {
	if e.domainEntry == nil {
		return false
	}
	return rand.Intn(100) < e.config.MutableStateChecksumGenProbability(e.domainEntry.GetInfo().Name)
}

func (e *mutableStateBuilder) shouldVerifyChecksum() bool {
	if e.domainEntry == nil {
		return false
	}
	return rand.Intn(100) < e.config.MutableStateChecksumVerifyProbability(e.domainEntry.GetInfo().Name)
}

func (e *mutableStateBuilder) enableChecksumFailureRetry() bool {
	if e.domainEntry == nil {
		return false
	}
	return e.config.EnableRetryForChecksumFailure(e.domainEntry.GetInfo().Name)
}

func (e *mutableStateBuilder) shouldInvalidateChecksum() bool {
	invalidateBeforeEpochSecs := int64(e.config.MutableStateChecksumInvalidateBefore())
	if invalidateBeforeEpochSecs > 0 {
		invalidateBefore := time.Unix(invalidateBeforeEpochSecs, 0)
		return e.executionInfo.LastUpdatedTimestamp.Before(invalidateBefore)
	}
	return false
}

func (e *mutableStateBuilder) createInternalServerError(
	actionTag tag.Tag,
) error {

	return &types.InternalServiceError{Message: actionTag.Field().String + " operation failed"}
}

func (e *mutableStateBuilder) createCallerError(
	actionTag tag.Tag,
) error {

	return &types.BadRequestError{
		Message: fmt.Sprintf(mutableStateInvalidHistoryActionMsgTemplate, actionTag.Field().String),
	}
}

func (e *mutableStateBuilder) unixNanoToTime(
	timestampNanos int64,
) time.Time {

	return time.Unix(0, timestampNanos)
}

func (e *mutableStateBuilder) logInfo(msg string, tags ...tag.Tag) {
	if e == nil {
		return
	}
	if e.executionInfo != nil {
		tags = append(tags, tag.WorkflowID(e.executionInfo.WorkflowID))
		tags = append(tags, tag.WorkflowRunID(e.executionInfo.RunID))
		tags = append(tags, tag.WorkflowDomainID(e.executionInfo.DomainID))
	}
	e.logger.Info(msg, tags...)
}

func (e *mutableStateBuilder) logWarn(msg string, tags ...tag.Tag) {
	if e == nil {
		return
	}
	if e.executionInfo != nil {
		tags = append(tags, tag.WorkflowID(e.executionInfo.WorkflowID))
		tags = append(tags, tag.WorkflowRunID(e.executionInfo.RunID))
		tags = append(tags, tag.WorkflowDomainID(e.executionInfo.DomainID))
	}
	e.logger.Warn(msg, tags...)
}

func (e *mutableStateBuilder) logError(msg string, tags ...tag.Tag) {
	if e == nil {
		return
	}
	if e.executionInfo != nil {
		tags = append(tags, tag.WorkflowID(e.executionInfo.WorkflowID))
		tags = append(tags, tag.WorkflowRunID(e.executionInfo.RunID))
		tags = append(tags, tag.WorkflowDomainID(e.executionInfo.DomainID))
	}
	e.logger.Error(msg, tags...)
}

func (e *mutableStateBuilder) logDataInconsistency() {
	domainID := e.executionInfo.DomainID
	workflowID := e.executionInfo.WorkflowID
	runID := e.executionInfo.RunID

	e.metricsClient.Scope(metrics.WorkflowContextScope).IncCounter(metrics.DataInconsistentCounter)
	e.logger.Error("encounter mutable state data inconsistency",
		tag.WorkflowDomainID(domainID),
		tag.WorkflowID(workflowID),
		tag.WorkflowRunID(runID),
	)
}
func (e *mutableStateBuilder) reorderAndFilterDuplicateEvents(events []*types.HistoryEvent, source string) []*types.HistoryEvent {
	type activityTaskUniqueEventParams struct {
		eventType        types.EventType
		scheduledEventID int64
		attempt          int32
		startedEventID   int64
	}

	activityTaskUniqueEvents := make(map[activityTaskUniqueEventParams]struct{})

	checkActivityTaskEventUniqueness := func(event *types.HistoryEvent) bool {
		var uniqueEventParams activityTaskUniqueEventParams

		var scheduledEventID int64

		switch event.GetEventType() {
		case types.EventTypeActivityTaskStarted:
			scheduledEventID = event.ActivityTaskStartedEventAttributes.GetScheduledEventID()
			uniqueEventParams = activityTaskUniqueEventParams{
				eventType:        event.GetEventType(),
				scheduledEventID: scheduledEventID,
				attempt:          event.ActivityTaskStartedEventAttributes.Attempt,
			}
		case types.EventTypeActivityTaskCompleted:
			scheduledEventID = event.ActivityTaskCompletedEventAttributes.GetScheduledEventID()
			uniqueEventParams = activityTaskUniqueEventParams{
				eventType:        event.GetEventType(),
				scheduledEventID: scheduledEventID,
				startedEventID:   event.ActivityTaskCompletedEventAttributes.GetStartedEventID(),
			}
		case types.EventTypeActivityTaskFailed:
			scheduledEventID = event.ActivityTaskFailedEventAttributes.GetScheduledEventID()
			uniqueEventParams = activityTaskUniqueEventParams{
				eventType:        event.GetEventType(),
				scheduledEventID: scheduledEventID,
				startedEventID:   event.ActivityTaskFailedEventAttributes.GetStartedEventID(),
			}
		case types.EventTypeActivityTaskCanceled:
			scheduledEventID = event.ActivityTaskCanceledEventAttributes.GetScheduledEventID()
			uniqueEventParams = activityTaskUniqueEventParams{
				eventType:        event.GetEventType(),
				scheduledEventID: scheduledEventID,
				startedEventID:   event.ActivityTaskCanceledEventAttributes.StartedEventID,
			}
		case types.EventTypeActivityTaskTimedOut:
			scheduledEventID = event.ActivityTaskTimedOutEventAttributes.GetScheduledEventID()
			uniqueEventParams = activityTaskUniqueEventParams{
				eventType:        event.GetEventType(),
				scheduledEventID: scheduledEventID,
				startedEventID:   event.ActivityTaskTimedOutEventAttributes.StartedEventID,
			}
		default:
			return true
		}

		if _, ok := activityTaskUniqueEvents[uniqueEventParams]; ok {
			e.logger.Error("Duplicate activity task event found",
				tag.WorkflowDomainName(e.GetDomainEntry().GetInfo().Name),
				tag.WorkflowID(e.GetExecutionInfo().WorkflowID),
				tag.WorkflowRunID(e.GetExecutionInfo().RunID),
				tag.WorkflowScheduleID(scheduledEventID),
				tag.WorkflowEventType(event.GetEventType().String()),
				tag.Dynamic("duplication-source", source),
			)

			e.metricsClient.IncCounter(metrics.HistoryFlushBufferedEventsScope, metrics.DuplicateActivityTaskEventCounter)
			return false
		}
		activityTaskUniqueEvents[uniqueEventParams] = struct{}{}
		return true
	}

	var headEvents []*types.HistoryEvent
	var tailEvents []*types.HistoryEvent

	// Sometimes we see buffered events are out of order when read back from database.  This is mostly not an issue
	// except in the Activity case where ActivityStarted and ActivityCompleted gets out of order.  The following code
	// is added to reorder buffered events to guarantee all activity completion events will always be processed at the end.
	for _, event := range events {
		// We sometimes see duplicate events
		if unique := checkActivityTaskEventUniqueness(event); !unique {
			continue
		}
		switch event.GetEventType() {
		case types.EventTypeActivityTaskCompleted,
			types.EventTypeActivityTaskFailed,
			types.EventTypeActivityTaskCanceled,
			types.EventTypeActivityTaskTimedOut:
			tailEvents = append(tailEvents, event)
		case types.EventTypeChildWorkflowExecutionCompleted,
			types.EventTypeChildWorkflowExecutionFailed,
			types.EventTypeChildWorkflowExecutionCanceled,
			types.EventTypeChildWorkflowExecutionTimedOut,
			types.EventTypeChildWorkflowExecutionTerminated:
			tailEvents = append(tailEvents, event)
		default:
			headEvents = append(headEvents, event)
		}
	}
	return append(headEvents, tailEvents...)
}

func mergeMapOfByteArray(
	current map[string][]byte,
	upsert map[string][]byte,
) map[string][]byte {

	if current == nil {
		current = make(map[string][]byte)
	}
	for k, v := range upsert {
		current[k] = v
	}
	return current
}
