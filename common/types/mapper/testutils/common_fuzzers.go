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
		func(e *types.EventType, c fuzz.Continue) {
			*e = types.EventType(c.Intn(50)) // 0-49: various event types
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
