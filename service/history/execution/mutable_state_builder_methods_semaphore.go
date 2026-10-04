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

	si, ok := e.pendingSemaphoreInfoIDs[initiatedEventID]
	return si, ok
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
// waitTimeoutSeconds is the timeout the acquire actually uses, already resolved by the caller.
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
	si, err := e.ReplicateSemaphoreAcquireInitiatedEvent(event)
	if err != nil {
		return nil, nil, err
	}
	return event, si, nil
}

// ReplicateSemaphoreAcquireInitiatedEvent starts the hold for an acquire event. Everything it
// stores comes from the event and the run, so replay rebuilds the same hold.
func (e *mutableStateBuilder) ReplicateSemaphoreAcquireInitiatedEvent(
	event *types.HistoryEvent,
) (*persistence.SemaphoreInfo, error) {

	attributes := event.SemaphoreAcquireInitiatedEventAttributes
	si := &persistence.SemaphoreInfo{
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

	e.UpsertSemaphoreInfo(si)
	return si, nil
}

// AddSemaphoreAcquiredEvent records that the acquire started by initiatedEventID was granted
// tokenID.
//
// It returns an error if there is no hold for initiatedEventID, or if the hold already has
// tokenID. If the hold has a different token, the new token replaces it.
func (e *mutableStateBuilder) AddSemaphoreAcquiredEvent(
	initiatedEventID int64,
	tokenID int32,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionSemaphoreAcquired
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	si, ok := e.GetSemaphoreInfo(initiatedEventID)
	if !ok || si.TokenID == int(tokenID) {
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

	si, ok := e.GetSemaphoreInfo(initiatedEventID)
	if !ok {
		e.logError(
			"Unable to find semaphore hold",
			tag.ErrorTypeInvalidMutableStateAction,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.WorkflowInitiatedID(initiatedEventID),
		)
		return ErrMissingSemaphoreInfo
	}
	si.Version = event.Version
	si.TokenID = int(attributes.GetTokenID())
	e.UpsertSemaphoreInfo(si)

	return nil
}

// ReplicateSemaphoreReleasedEvent ends the hold the released token belonged to.
func (e *mutableStateBuilder) ReplicateSemaphoreReleasedEvent(
	event *types.HistoryEvent,
) error {

	return e.DeleteSemaphoreInfo(event.SemaphoreReleasedEventAttributes.GetInitiatedEventID())
}
