package history

import (
	"github.com/uber-common/bark"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
)

const (
	emptyUuid = "emptyUuid"
)

type (
	mutableStateBuilder struct {
		pendingActivityInfoIDs          map[int64]*persistence.ActivityInfo // Schedule Event ID -> Activity Info.
		pendingActivityInfoByActivityID map[string]int64                    // Activity ID -> Schedule Event ID of the activity.
		updateActivityInfos             []*persistence.ActivityInfo
		deleteActivityInfo              *int64
		pendingTimerInfoIDs             map[string]*persistence.TimerInfo // User Timer ID -> Timer Info.
		updateTimerInfos                []*persistence.TimerInfo
		deleteTimerInfos                []string
		pendingDecision                 *persistence.DecisionInfo // The decision info.
		updatedDecision                 *persistence.DecisionInfo // The decision info.
		logger                          bark.Logger
	}
)

func newMutableStateBuilder(logger bark.Logger) *mutableStateBuilder {
	return &mutableStateBuilder{
		updateActivityInfos:             []*persistence.ActivityInfo{},
		pendingActivityInfoIDs:          make(map[int64]*persistence.ActivityInfo),
		pendingActivityInfoByActivityID: make(map[string]int64),
		pendingTimerInfoIDs:             make(map[string]*persistence.TimerInfo),
		updateTimerInfos:                []*persistence.TimerInfo{},
		deleteTimerInfos:                []string{},
		logger:                          logger}
}

func (e *mutableStateBuilder) Load(
	activityInfos map[int64]*persistence.ActivityInfo,
	timerInfos map[string]*persistence.TimerInfo,
	decision *persistence.DecisionInfo) {
	e.pendingActivityInfoIDs = activityInfos
	e.pendingTimerInfoIDs = timerInfos
	e.pendingDecision = decision
	for _, ai := range activityInfos {
		e.pendingActivityInfoByActivityID[ai.ActivityID] = ai.ScheduleID
	}
}

// GetActivity gives details about an activity that is currently in progress.
func (e *mutableStateBuilder) GetActivity(scheduleEventID int64) (bool, *persistence.ActivityInfo) {
	a, ok := e.pendingActivityInfoIDs[scheduleEventID]
	return ok, a
}

// GetActivityByActivityID gives details about an activity that is currently in progress.
func (e *mutableStateBuilder) GetActivityByActivityID(activityID string) (bool, *persistence.ActivityInfo) {
	eventID, ok := e.pendingActivityInfoByActivityID[activityID]
	if !ok {
		return ok, nil
	}
	a, ok := e.pendingActivityInfoIDs[eventID]
	return ok, a
}

// UpdateActivity updates details about an activity that is in progress.
func (e *mutableStateBuilder) UpdateActivity(scheduleEventID int64, ai *persistence.ActivityInfo) {
	e.pendingActivityInfoIDs[scheduleEventID] = ai
	e.pendingActivityInfoByActivityID[ai.ActivityID] = scheduleEventID
	e.updateActivityInfos = append(e.updateActivityInfos, ai)
}

// DeleteActivity deletes details about an activity.
func (e *mutableStateBuilder) DeleteActivity(scheduleEventID int64) {
	e.deleteActivityInfo = common.Int64Ptr(scheduleEventID)
}

// GetUserTimer gives details about a user timer.
func (e *mutableStateBuilder) GetUserTimer(timerID string) (bool, *persistence.TimerInfo) {
	a, ok := e.pendingTimerInfoIDs[timerID]
	return ok, a
}

// UpdateUserTimer updates the user timer in progress.
func (e *mutableStateBuilder) UpdateUserTimer(timerID string, ti *persistence.TimerInfo) {
	e.pendingTimerInfoIDs[timerID] = ti
	e.updateTimerInfos = append(e.updateTimerInfos, ti)
}

// DeleteUserTimer deletes an user timer.
func (e *mutableStateBuilder) DeleteUserTimer(timerID string) {
	e.deleteTimerInfos = append(e.deleteTimerInfos, timerID)
}

// GetDecision returns details about the in-progress decision task
func (e *mutableStateBuilder) GetDecision(scheduleEventID int64) (bool, *persistence.DecisionInfo) {
	if e.updatedDecision != nil {
		return e.updatedDecision.ScheduleID == scheduleEventID, e.updatedDecision
	}
	if e.pendingDecision != nil {
		return e.pendingDecision.ScheduleID == scheduleEventID, e.pendingDecision
	}
	return false, nil
}

// UpdateDecision updates a decision task.
func (e *mutableStateBuilder) UpdateDecision(di *persistence.DecisionInfo) {
	e.updatedDecision = di
	e.pendingDecision = di
}

// DeleteDecision deletes a decision task.
func (e *mutableStateBuilder) DeleteDecision() {
	emptyDecisionInfo := &persistence.DecisionInfo{
		ScheduleID:          emptyEventID,
		StartedID:           emptyEventID,
		RequestID:           emptyUuid,
		StartToCloseTimeout: 0,
	}
	e.updatedDecision = emptyDecisionInfo
	e.pendingDecision = emptyDecisionInfo
}
