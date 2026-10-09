package execution

import (
	"fmt"
	"time"

	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	commonsemaphore "github.com/uber/cadence/common/semaphore"
	"github.com/uber/cadence/common/types"
)

// GetSemaphoreInfo gets details about one hold this run has, granted or still waiting.
func (e *mutableStateBuilder) GetSemaphoreInfo(
	initiatedEventID int64,
) (*persistence.SemaphoreInfo, bool) {

	semaphoreInfo, ok := e.pendingSemaphoreInfoIDs[initiatedEventID]
	return semaphoreInfo, ok
}

func (e *mutableStateBuilder) GetPendingSemaphoreInfos() map[int64]*persistence.SemaphoreInfo {
	return e.pendingSemaphoreInfoIDs
}

// UpsertSemaphoreInfo records a hold, replacing any entry already under the same initiated id.
// Callers use it both to start a hold and to fill in the token once one is granted.
func (e *mutableStateBuilder) UpsertSemaphoreInfo(
	info *persistence.SemaphoreInfo,
) {

	// Load installs whatever persistence returned, which is nil on a backend that does not
	// store holds. The other two maps are only ever set by the constructor and by the flush.
	if e.pendingSemaphoreInfoIDs == nil {
		e.pendingSemaphoreInfoIDs = make(map[int64]*persistence.SemaphoreInfo)
	}
	e.pendingSemaphoreInfoIDs[info.InitiatedID] = info
	e.updateSemaphoreInfos[info.InitiatedID] = info
}

// DeleteSemaphoreInfo removes the record of the hold started by initiatedEventID. It does not
// touch the semaphore's definition or release the slot in Matching.
func (e *mutableStateBuilder) DeleteSemaphoreInfo(
	initiatedEventID int64,
) error {

	if _, ok := e.pendingSemaphoreInfoIDs[initiatedEventID]; ok {
		delete(e.pendingSemaphoreInfoIDs, initiatedEventID)
	} else {
		e.logError(
			fmt.Sprintf("unable to find semaphore hold event ID: %v in mutable state", initiatedEventID),
			tag.ErrorTypeInvalidMutableStateAction,
		)
		// log data inconsistency instead of returning an error
		e.logDataInconsistency()
	}

	delete(e.updateSemaphoreInfos, initiatedEventID)
	e.deleteSemaphoreInfos[initiatedEventID] = struct{}{}
	return nil
}

// AddSemaphoreAcquireInitiatedEvent records an acquire and starts its hold with no token yet.
// waitTimeoutSeconds must be positive; the caller fills in the default.
func (e *mutableStateBuilder) AddSemaphoreAcquireInitiatedEvent(
	decisionCompletedEventID int64,
	semaphoreName string,
	waitTimeoutSeconds int32,
) (*types.HistoryEvent, *persistence.SemaphoreInfo, error) {

	opTag := tag.WorkflowActionSemaphoreAcquireInitiated
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, err
	}

	event := e.hBuilder.AddSemaphoreAcquireInitiatedEvent(decisionCompletedEventID, semaphoreName, waitTimeoutSeconds)
	semaphoreInfo, err := e.ReplicateSemaphoreAcquireInitiatedEvent(event)
	if err != nil {
		return nil, nil, err
	}
	return event, semaphoreInfo, nil
}

// ReplicateSemaphoreAcquireInitiatedEvent starts the hold for an acquire event. Everything it
// stores comes from the event and the run, so replay rebuilds the same hold.
func (e *mutableStateBuilder) ReplicateSemaphoreAcquireInitiatedEvent(
	event *types.HistoryEvent,
) (*persistence.SemaphoreInfo, error) {

	attributes := event.SemaphoreAcquireInitiatedEventAttributes
	semaphoreInfo := &persistence.SemaphoreInfo{
		Version:       event.Version,
		InitiatedID:   event.ID,
		SemaphoreName: attributes.GetSemaphoreName(),
		OwnerID: commonsemaphore.Owner{
			WorkflowID: e.executionInfo.WorkflowID,
			RunID:      e.executionInfo.RunID,
			HoldID:     event.ID,
		}.String(),
		// Measured from the event's own timestamp, not the clock, so replay gets the same deadline.
		AcquireDeadline: time.Unix(0, event.GetTimestamp()).
			Add(time.Duration(attributes.GetWaitTimeoutSeconds()) * time.Second),
	}

	e.UpsertSemaphoreInfo(semaphoreInfo)
	return semaphoreInfo, nil
}

// AddSemaphoreAcquiredEvent records that the acquire started by initiatedEventID was granted
// tokenID, replacing any different token the hold has. It refuses a missing hold or a repeat of
// the hold's token.
func (e *mutableStateBuilder) AddSemaphoreAcquiredEvent(
	initiatedEventID int64,
	tokenID int32,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionSemaphoreAcquired
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	semaphoreInfo, ok := e.GetSemaphoreInfo(initiatedEventID)
	// Callers handle a missing hold and a repeated grant first, so reaching this is a caller bug.
	// Refuse it rather than write a grant with no hold, or a duplicate grant, to history.
	if !ok || semaphoreInfo.TokenID == int(tokenID) {
		e.logWarn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowInitiatedID(initiatedEventID))
		return nil, e.createInternalServerError(opTag)
	}

	event := e.hBuilder.AddSemaphoreAcquiredEvent(initiatedEventID, tokenID)
	if err := e.ReplicateSemaphoreAcquiredEvent(event); err != nil {
		return nil, err
	}
	return event, nil
}

// ReplicateSemaphoreAcquiredEvent fills in the granted token, which moves the hold to held.
func (e *mutableStateBuilder) ReplicateSemaphoreAcquiredEvent(
	event *types.HistoryEvent,
) error {

	attributes := event.SemaphoreAcquiredEventAttributes
	initiatedEventID := attributes.GetInitiatedEventID()

	semaphoreInfo, ok := e.GetSemaphoreInfo(initiatedEventID)
	if !ok {
		e.logError(
			"Unable to find semaphore hold",
			tag.ErrorTypeInvalidMutableStateAction,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.WorkflowInitiatedID(initiatedEventID),
		)
		return ErrMissingSemaphoreInfo
	}
	semaphoreInfo.Version = event.Version
	semaphoreInfo.TokenID = int(attributes.GetTokenID())
	e.UpsertSemaphoreInfo(semaphoreInfo)

	return nil
}

// ReplicateSemaphoreReleasedEvent ends the hold the released token belonged to.
func (e *mutableStateBuilder) ReplicateSemaphoreReleasedEvent(
	event *types.HistoryEvent,
) error {

	return e.DeleteSemaphoreInfo(event.SemaphoreReleasedEventAttributes.GetInitiatedEventID())
}
