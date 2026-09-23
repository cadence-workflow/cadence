package execution

import (
	"fmt"

	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
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

// DeletePendingSemaphore deletes details about a SemaphoreInfo
func (e *mutableStateBuilder) DeletePendingSemaphore(
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
