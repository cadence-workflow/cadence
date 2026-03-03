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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination timer_sequence_mock.go -self_package github.com/uber/cadence/service/history/execution

package execution

import (
	"fmt"
	"sort"
	"time"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	// TimerType indicates timer type
	TimerType int32
)

const (
	// TimerTypeStartToClose is the timer type for activity startToClose timer
	TimerTypeStartToClose = TimerType(types.TimeoutTypeStartToClose)
	// TimerTypeScheduleToStart is the timer type for activity scheduleToStart timer
	TimerTypeScheduleToStart = TimerType(types.TimeoutTypeScheduleToStart)
	// TimerTypeScheduleToClose is the timer type for activity scheduleToClose timer
	TimerTypeScheduleToClose = TimerType(types.TimeoutTypeScheduleToClose)
	// TimerTypeHeartbeat is the timer type for activity heartbeat timer
	TimerTypeHeartbeat = TimerType(types.TimeoutTypeHeartbeat)
)

const (
	// TimerTaskStatusNone indicates activity / user timer task has not been created
	TimerTaskStatusNone = iota
	// TimerTaskStatusCreated indicates user timer task has been created
	TimerTaskStatusCreated
)

const (
	// TimerTaskStatusCreatedStartToClose indicates activity startToClose timer has been created
	TimerTaskStatusCreatedStartToClose = 1 << iota
	// TimerTaskStatusCreatedScheduleToStart indicates activity scheduleToStart timer has been created
	TimerTaskStatusCreatedScheduleToStart
	// TimerTaskStatusCreatedScheduleToClose indicates activity scheduleToClose timer has been created
	TimerTaskStatusCreatedScheduleToClose
	// TimerTaskStatusCreatedHeartbeat indicates activity heartbeat timer has been created
	TimerTaskStatusCreatedHeartbeat
)

type (
	// TimerSequenceID describes user / activity timer and defines an order among timers
	TimerSequenceID struct {
		EventID      int64
		Timestamp    time.Time
		TimerType    TimerType
		TimerCreated bool
		Attempt      int32
	}

	// TimerSequenceIDs is a list of TimerSequenceID
	TimerSequenceIDs []TimerSequenceID

	// TimerSequence manages user / activity timer
	TimerSequence interface {
		IsExpired(referenceTime time.Time, TimerSequenceID TimerSequenceID) (time.Duration, bool)

		CreateNextUserTimer() (bool, error)
		CreateNextActivityTimer() (bool, error)

		LoadAndSortUserTimers() []TimerSequenceID
		LoadAndSortActivityTimers() []TimerSequenceID
	}

	timerSequenceImpl struct {
		mutableState MutableState
	}
)

var _ TimerSequence = (*timerSequenceImpl)(nil)

// NewTimerSequence creates a new timer sequence
func NewTimerSequence(
	mutableState MutableState,
) TimerSequence {
	return &timerSequenceImpl{
		mutableState: mutableState,
	}
}

func (t *timerSequenceImpl) IsExpired(
	referenceTime time.Time,
	TimerSequenceID TimerSequenceID,
) (time.Duration, bool) {

	// Cassandra timestamp resolution is in millisecond
	// here we do the check in terms of second resolution.

	timerFireTimeInSecond := TimerSequenceID.Timestamp.Unix()
	referenceTimeInSecond := referenceTime.Unix()

	if timerFireTimeInSecond <= referenceTimeInSecond {
		return time.Duration(referenceTimeInSecond-timerFireTimeInSecond) * time.Second, true
	}

	return 0, false
}

func (t *timerSequenceImpl) CreateNextUserTimer() (bool, error) {

	sequenceIDs := t.LoadAndSortUserTimers()
	if len(sequenceIDs) == 0 {
		return false, nil
	}

	firstTimerTask := sequenceIDs[0]

	// timer has already been created
	if firstTimerTask.TimerCreated {
		return false, nil
	}

	timerInfo, ok := t.mutableState.GetUserTimerInfoByEventID(firstTimerTask.EventID)
	if !ok {
		return false, &types.InternalServiceError{
			Message: fmt.Sprintf("unable to load activity info %v", firstTimerTask.EventID),
		}
	}
	// mark timer task mask as indication that timer task is generated
	// here TaskID is misleading attr, should be called timer created flag or something
	timerInfo.TaskStatus = TimerTaskStatusCreated
	if err := t.mutableState.UpdateUserTimer(timerInfo); err != nil {
		return false, err
	}
	executionInfo := t.mutableState.GetExecutionInfo()
	t.mutableState.AddTimerTasks(&persistence.UserTimerTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID is set by shard
			VisibilityTimestamp: firstTimerTask.Timestamp,
			Version:             t.mutableState.GetCurrentVersion(),
		},
		EventID:  firstTimerTask.EventID,
		TaskList: executionInfo.TaskList,
	})
	return true, nil
}

func (t *timerSequenceImpl) CreateNextActivityTimer() (bool, error) {

	sequenceIDs := t.LoadAndSortActivityTimers()
	if len(sequenceIDs) == 0 {
		return false, nil
	}

	firstTimerTask := sequenceIDs[0]

	// timer has already been created
	if firstTimerTask.TimerCreated {
		return false, nil
	}

	activityInfo, ok := t.mutableState.GetActivityInfo(firstTimerTask.EventID)
	if !ok {
		return false, &types.InternalServiceError{
			Message: fmt.Sprintf("unable to load activity info %v", firstTimerTask.EventID),
		}
	}
	// mark timer task mask as indication that timer task is generated
	activityInfo.TimerTaskStatus |= TimerTypeToTimerMask(firstTimerTask.TimerType)
	if firstTimerTask.TimerType == TimerTypeHeartbeat {
		activityInfo.LastHeartbeatTimeoutVisibilityInSeconds = firstTimerTask.Timestamp.Unix()
	}
	if err := t.mutableState.UpdateActivity(activityInfo); err != nil {
		return false, err
	}
	executionInfo := t.mutableState.GetExecutionInfo()
	t.mutableState.AddTimerTasks(&persistence.ActivityTimeoutTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID is set by shard
			VisibilityTimestamp: firstTimerTask.Timestamp,
			Version:             t.mutableState.GetCurrentVersion(),
		},
		TimeoutType: int(firstTimerTask.TimerType),
		EventID:     firstTimerTask.EventID,
		Attempt:     int64(firstTimerTask.Attempt),
		TaskList:    activityInfo.TaskList,
	})
	return true, nil
}

func (t *timerSequenceImpl) LoadAndSortUserTimers() []TimerSequenceID {

	pendingTimers := t.mutableState.GetPendingTimerInfos()
	timers := make(TimerSequenceIDs, 0, len(pendingTimers))

	for _, timerInfo := range pendingTimers {

		if sequenceID := t.getUserTimerTimeout(
			timerInfo,
		); sequenceID != nil {
			timers = append(timers, *sequenceID)
		}
	}

	sort.Sort(timers)
	return timers
}

func (t *timerSequenceImpl) LoadAndSortActivityTimers() []TimerSequenceID {
	// there can be 4 timer per activity
	// see TimerType
	pendingActivities := t.mutableState.GetPendingActivityInfos()
	activityTimers := make(TimerSequenceIDs, 0, len(pendingActivities)*4)

	for _, activityInfo := range pendingActivities {

		if sequenceID := t.getActivityScheduleToCloseTimeout(
			activityInfo,
		); sequenceID != nil {
			activityTimers = append(activityTimers, *sequenceID)
		}

		if sequenceID := t.getActivityScheduleToStartTimeout(
			activityInfo,
		); sequenceID != nil {
			activityTimers = append(activityTimers, *sequenceID)
		}

		if sequenceID := t.getActivityStartToCloseTimeout(
			activityInfo,
		); sequenceID != nil {
			activityTimers = append(activityTimers, *sequenceID)
		}

		if sequenceID := t.getActivityHeartbeatTimeout(
			activityInfo,
		); sequenceID != nil {
			activityTimers = append(activityTimers, *sequenceID)
		}
	}

	sort.Sort(activityTimers)
	return activityTimers
}

func (t *timerSequenceImpl) getUserTimerTimeout(
	timerInfo *persistence.TimerInfo,
) *TimerSequenceID {

	return &TimerSequenceID{
		EventID:      timerInfo.StartedID,
		Timestamp:    timerInfo.ExpiryTime,
		TimerType:    TimerTypeStartToClose,
		TimerCreated: timerInfo.TaskStatus == TimerTaskStatusCreated,
		Attempt:      0,
	}
}

func (t *timerSequenceImpl) getActivityScheduleToStartTimeout(
	activityInfo *persistence.ActivityInfo,
) *TimerSequenceID {

	// activity is not scheduled yet, probably due to retry & backoff
	if activityInfo.ScheduleID == constants.EmptyEventID {
		return nil
	}

	// activity is already started
	if activityInfo.StartedID != constants.EmptyEventID {
		return nil
	}

	startTimeout := activityInfo.ScheduledTime.Add(
		time.Duration(activityInfo.ScheduleToStartTimeout) * time.Second,
	)

	return &TimerSequenceID{
		EventID:      activityInfo.ScheduleID,
		Timestamp:    startTimeout,
		TimerType:    TimerTypeScheduleToStart,
		TimerCreated: (activityInfo.TimerTaskStatus & TimerTaskStatusCreatedScheduleToStart) > 0,
		Attempt:      activityInfo.Attempt,
	}
}

func (t *timerSequenceImpl) getActivityScheduleToCloseTimeout(
	activityInfo *persistence.ActivityInfo,
) *TimerSequenceID {

	// activity is not scheduled yet, probably due to retry & backoff
	if activityInfo.ScheduleID == constants.EmptyEventID {
		return nil
	}

	closeTimeout := activityInfo.ScheduledTime.Add(
		time.Duration(activityInfo.ScheduleToCloseTimeout) * time.Second,
	)

	return &TimerSequenceID{
		EventID:      activityInfo.ScheduleID,
		Timestamp:    closeTimeout,
		TimerType:    TimerTypeScheduleToClose,
		TimerCreated: (activityInfo.TimerTaskStatus & TimerTaskStatusCreatedScheduleToClose) > 0,
		Attempt:      activityInfo.Attempt,
	}
}

func (t *timerSequenceImpl) getActivityStartToCloseTimeout(
	activityInfo *persistence.ActivityInfo,
) *TimerSequenceID {

	// activity is not scheduled yet, probably due to retry & backoff
	if activityInfo.ScheduleID == constants.EmptyEventID {
		return nil
	}

	// activity is not started yet
	if activityInfo.StartedID == constants.EmptyEventID {
		return nil
	}

	closeTimeout := activityInfo.StartedTime.Add(
		time.Duration(activityInfo.StartToCloseTimeout) * time.Second,
	)

	return &TimerSequenceID{
		EventID:      activityInfo.ScheduleID,
		Timestamp:    closeTimeout,
		TimerType:    TimerTypeStartToClose,
		TimerCreated: (activityInfo.TimerTaskStatus & TimerTaskStatusCreatedStartToClose) > 0,
		Attempt:      activityInfo.Attempt,
	}
}

func (t *timerSequenceImpl) getActivityHeartbeatTimeout(
	activityInfo *persistence.ActivityInfo,
) *TimerSequenceID {

	// activity is not scheduled yet, probably due to retry & backoff
	if activityInfo.ScheduleID == constants.EmptyEventID {
		return nil
	}

	// activity is not started yet
	if activityInfo.StartedID == constants.EmptyEventID {
		return nil
	}

	// not heartbeat timeout configured
	if activityInfo.HeartbeatTimeout <= 0 {
		return nil
	}

	// use the latest time as last heartbeat time
	lastHeartbeat := activityInfo.StartedTime
	if activityInfo.LastHeartBeatUpdatedTime.After(lastHeartbeat) {
		lastHeartbeat = activityInfo.LastHeartBeatUpdatedTime
	}

	heartbeatTimeout := lastHeartbeat.Add(
		time.Duration(activityInfo.HeartbeatTimeout) * time.Second,
	)

	return &TimerSequenceID{
		EventID:      activityInfo.ScheduleID,
		Timestamp:    heartbeatTimeout,
		TimerType:    TimerTypeHeartbeat,
		TimerCreated: (activityInfo.TimerTaskStatus & TimerTaskStatusCreatedHeartbeat) > 0,
		Attempt:      activityInfo.Attempt,
	}
}

// TimerTypeToTimerMask converts TimerType into the TimerTaskStatus flag
func TimerTypeToTimerMask(
	TimerType TimerType,
) int32 {

	switch TimerType {
	case TimerTypeStartToClose:
		return TimerTaskStatusCreatedStartToClose
	case TimerTypeScheduleToStart:
		return TimerTaskStatusCreatedScheduleToStart
	case TimerTypeScheduleToClose:
		return TimerTaskStatusCreatedScheduleToClose
	case TimerTypeHeartbeat:
		return TimerTaskStatusCreatedHeartbeat
	default:
		panic("invalid timeout type")
	}
}

// TimerTypeToInternal converts TimeType to its internal representation
func TimerTypeToInternal(
	TimerType TimerType,
) types.TimeoutType {

	switch TimerType {
	case TimerTypeStartToClose:
		return types.TimeoutTypeStartToClose
	case TimerTypeScheduleToStart:
		return types.TimeoutTypeScheduleToStart
	case TimerTypeScheduleToClose:
		return types.TimeoutTypeScheduleToClose
	case TimerTypeHeartbeat:
		return types.TimeoutTypeHeartbeat
	default:
		panic(fmt.Sprintf("invalid timer type: %v", TimerType))
	}
}

// TimerTypeFromInternal gets TimerType from internal type
func TimerTypeFromInternal(
	TimerType types.TimeoutType,
) TimerType {

	switch TimerType {
	case types.TimeoutTypeStartToClose:
		return TimerTypeStartToClose
	case types.TimeoutTypeScheduleToStart:
		return TimerTypeScheduleToStart
	case types.TimeoutTypeScheduleToClose:
		return TimerTypeScheduleToClose
	case types.TimeoutTypeHeartbeat:
		return TimerTypeHeartbeat
	default:
		panic(fmt.Sprintf("invalid timeout type: %v", TimerType))
	}
}

// TimerTypeToReason creates timeout reason based on the TimeType
func TimerTypeToReason(
	timerType TimerType,
) string {
	return fmt.Sprintf("cadenceInternal:Timeout %v", TimerTypeToInternal(timerType))
}

// Len implements sort.Interface
func (s TimerSequenceIDs) Len() int {
	return len(s)
}

// Swap implements sort.Interface.
func (s TimerSequenceIDs) Swap(
	this int,
	that int,
) {
	s[this], s[that] = s[that], s[this]
}

// Less implements sort.Interface
func (s TimerSequenceIDs) Less(
	this int,
	that int,
) bool {

	thisSequenceID := s[this]
	thatSequenceID := s[that]

	// order: timeout time, event ID, timeout type

	if thisSequenceID.Timestamp.Before(thatSequenceID.Timestamp) {
		return true
	} else if thisSequenceID.Timestamp.After(thatSequenceID.Timestamp) {
		return false
	}

	// timeout time are the same
	if thisSequenceID.EventID < thatSequenceID.EventID {
		return true
	} else if thisSequenceID.EventID > thatSequenceID.EventID {
		return false
	}

	// timeout time & event ID are the same
	if thisSequenceID.TimerType < thatSequenceID.TimerType {
		return true
	} else if thisSequenceID.TimerType > thatSequenceID.TimerType {
		return false
	}

	// thisSequenceID && thatSequenceID are the same
	return true
}
