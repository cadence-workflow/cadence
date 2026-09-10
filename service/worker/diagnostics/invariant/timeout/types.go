package timeout

import (
	"time"

	"github.com/uber/cadence/common/types"
)

type TimeoutType string

const (
	TimeoutTypeExecution     TimeoutType = "The Workflow Execution has timed out"
	TimeoutTypeActivity      TimeoutType = "Activity task has timed out"
	TimeoutTypeDecision      TimeoutType = "Decision task has timed out"
	TimeoutTypeChildWorkflow TimeoutType = "Child Workflow Execution has timed out"
)

func (tt TimeoutType) String() string {
	return string(tt)
}

type ExecutionTimeoutMetadata struct {
	ExecutionTime    time.Duration
	Tasklist         *types.TaskList
	LastOngoingEvent *types.HistoryEvent
}

type ChildWfTimeoutMetadata struct {
	ExecutionTime time.Duration
	Execution     *types.WorkflowExecution
}

type ActivityTimeoutMetadata struct {
	TimeoutType      *types.TimeoutType
	TimeElapsed      time.Duration
	RetryPolicy      *types.RetryPolicy
	HeartBeatTimeout time.Duration
	Tasklist         *types.TaskList
}

type PollersMetadata struct {
	TaskListName    string
	TaskListBacklog int64
}

type HeartbeatingMetadata struct {
	TimeElapsed time.Duration
	RetryPolicy *types.RetryPolicy
}

// TimeoutIssuesMetadata is the metadata for every timeout issue. EventID is the timed-out event.
type TimeoutIssuesMetadata struct {
	EventID           int64
	ConfiguredTimeout time.Duration

	ExecutionTimeout *ExecutionTimeoutMetadata `json:",omitempty"`
	ActivityTimeout  *ActivityTimeoutMetadata  `json:",omitempty"`
	ChildWfTimeout   *ChildWfTimeoutMetadata   `json:",omitempty"`
}

type TimeoutRootcauseMetadata struct {
	PollersMetadata      *PollersMetadata
	HeartBeatingMetadata *HeartbeatingMetadata
}
