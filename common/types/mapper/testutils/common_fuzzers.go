package testutils

import (
	fuzz "github.com/google/gofuzz"

	"github.com/uber/cadence/common/types"
)

// WithCommonEnumFuzzers adds fuzzers for common Cadence enum types
// to ensure only valid enum values are generated
func WithCommonEnumFuzzers() FuzzOption {
	return WithCustomFuncs(
		func(e *types.WorkflowExecutionCloseStatus, c fuzz.Continue) {
			*e = types.WorkflowExecutionCloseStatus(c.Intn(6)) // 0-5
		},
		func(e *types.TaskListKind, c fuzz.Continue) {
			*e = types.TaskListKind(c.Intn(3)) // 0-2: Normal, Sticky, Ephemeral
		},
		func(e *types.TaskListType, c fuzz.Continue) {
			*e = types.TaskListType(c.Intn(2)) // 0-1: Decision, Activity
		},
		func(e *types.TimeoutType, c fuzz.Continue) {
			*e = types.TimeoutType(c.Intn(4)) // 0-3: StartToClose, ScheduleToStart, ScheduleToClose, Heartbeat
		},
		func(e *types.ParentClosePolicy, c fuzz.Continue) {
			*e = types.ParentClosePolicy(c.Intn(3)) // 0-2
		},
		func(e *types.PendingActivityState, c fuzz.Continue) {
			*e = types.PendingActivityState(c.Intn(3)) // 0-2
		},
		func(e *types.PendingDecisionState, c fuzz.Continue) {
			*e = types.PendingDecisionState(c.Intn(2)) // 0-1
		},
		func(e *types.QueryTaskCompletedType, c fuzz.Continue) {
			*e = types.QueryTaskCompletedType(c.Intn(3)) // 0-2
		},
		func(e *types.QueryResultType, c fuzz.Continue) {
			*e = types.QueryResultType(c.Intn(2)) // 0-1: Answered, Failed
		},
		func(e *types.IndexedValueType, c fuzz.Continue) {
			*e = types.IndexedValueType(c.Intn(6)) // 0-5: String, Keyword, Int, Double, Bool, Datetime
		},
		func(e *types.CronOverlapPolicy, c fuzz.Continue) {
			*e = types.CronOverlapPolicy(c.Intn(2)) // 0-1: Skipped, BufferOne
		},
		func(e *types.WorkflowExecutionStatus, c fuzz.Continue) {
			*e = types.WorkflowExecutionStatus(c.Intn(8)) // 0-7: Pending, Started, Completed, Failed, Canceled, Terminated, ContinuedAsNew, TimedOut
		},
		func(e *types.ActiveClusterSelectionStrategy, c fuzz.Continue) {
			*e = types.ActiveClusterSelectionStrategy(c.Intn(2)) // 0-1: RegionSticky, ExternalEntity
		},
	)
}

func EncodingTypeFuzzer(e *types.EncodingType, c fuzz.Continue) {
	*e = types.EncodingType(c.Intn(2)) // 0-1: ThriftRW, JSON
}

func CrossClusterTaskFailedCauseFuzzer(e *types.CrossClusterTaskFailedCause, c fuzz.Continue) {
	*e = types.CrossClusterTaskFailedCause(c.Intn(6)) // 0-5: DomainNotActive, DomainNotExists, WorkflowAlreadyRunning, WorkflowNotExists, WorkflowAlreadyCompleted, Uncategorized
}

func CrossClusterTaskTypeFuzzer(e *types.CrossClusterTaskType, c fuzz.Continue) {
	*e = types.CrossClusterTaskType(c.Intn(5)) // 0-4: StartChildExecution, CancelExecution, SignalExecution, RecordChildWorkflowExecutionComplete, ApplyParentClosePolicy
}

func DLQTypeFuzzer(e *types.DLQType, c fuzz.Continue) {
	*e = types.DLQType(c.Intn(2)) // 0-1: Replication, Domain
}

func DomainOperationFuzzer(e *types.DomainOperation, c fuzz.Continue) {
	*e = types.DomainOperation(c.Intn(3)) // 0-2: Create, Update, Delete
}

func ReplicationTaskTypeFuzzer(e *types.ReplicationTaskType, c fuzz.Continue) {
	*e = types.ReplicationTaskType(c.Intn(7)) // 0-6: Domain, History, SyncShardStatus, SyncActivity, HistoryMetadata, HistoryV2, FailoverMarker
}

func WorkflowStateFuzzer(e *int32, c fuzz.Continue) {
	// WorkflowState values: 0-2 (Created, Running, Completed)
	// Note: This operates on *int32 because the types package uses *int32 for WorkflowState
	if e != nil {
		*e = int32(c.Intn(3))
	}
}

func GetTaskFailedCauseFuzzer(e *types.GetTaskFailedCause, c fuzz.Continue) {
	*e = types.GetTaskFailedCause(c.Intn(4)) // 0-3: ServiceBusy, Timeout, ShardOwnershipLost, Uncategorized
}

func IsolationGroupStateFuzzer(e *types.IsolationGroupState, c fuzz.Continue) {
	*e = types.IsolationGroupState(c.Intn(3)) // 0-2: Invalid, Healthy, Drained
}

func TaskTypeFuzzer(e *int32, c fuzz.Continue) {
	// TaskType internal constant values (from common/constants/constants.go):
	// 2: TaskTypeTransfer
	// 3: TaskTypeTimer
	// 4: TaskTypeReplication
	// 6: TaskTypeCrossCluster (deprecated but still valid)
	// Note: Value 0 maps to nil/INVALID, so we always generate a valid value
	if e != nil {
		// Generate one of the valid constant values: 2, 3, 4, or 6
		validValues := []int32{2, 3, 4, 6}
		*e = validValues[c.Intn(len(validValues))]
	}
}

func DomainStatusFuzzer(e *types.DomainStatus, c fuzz.Continue) {
	*e = types.DomainStatus(c.Intn(3)) // 0-2: Registered, Deprecated, Deleted
}
