package execution

import (
	"time"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

// IsImportedRun returns true when the start event is the start event of the imported run. A
// reset run of an imported run copies the start event, but it has a new run ID: the import sets
// the original execution run ID of the start event to the imported run ID.
func IsImportedRun(
	startAttributes *types.WorkflowExecutionStartedEventAttributes,
	runID string,
) bool {
	return startAttributes.GetImportSourceExecution() != nil &&
		startAttributes.OriginalExecutionRunID == runID
}

// SetImportInProgress makes the new mutable state of an imported run an import in progress, with
// the import fields of its start event. The state is set directly: after the start events, the
// new mutable state is Created or Running, and no transition goes from these states to Preparing.
func SetImportInProgress(
	mutableState MutableState,
	startAttributes *types.WorkflowExecutionStartedEventAttributes,
) {
	executionInfo := mutableState.GetExecutionInfo()
	executionInfo.State = persistence.WorkflowStatePreparing
	executionInfo.ImportSourceExecution = startAttributes.GetImportSourceExecution()
	executionInfo.ImportTimestamp = time.Unix(0, startAttributes.GetImportTimestamp())
	executionInfo.ImportInProgress = true
}

// IsRunningOrImporting returns true when the workflow is running or is an import in progress.
// Conflict resolution handles an import in progress like a running workflow: it is the current
// run of its workflow ID, and its history is not complete.
func IsRunningOrImporting(mutableState MutableState) bool {
	return mutableState.IsWorkflowExecutionRunning() ||
		mutableState.GetExecutionInfo().State == persistence.WorkflowStatePreparing
}
