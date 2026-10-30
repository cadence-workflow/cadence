package matching

import "github.com/uber/cadence/common/types"

// PollError is returned by poll calls which were sent to matching.
// It reports the task list partition the poll was sent to, so the caller can cancel the poll.
type PollError struct {
	TaskList *types.TaskList
	Err      error
}

func (e *PollError) Error() string {
	return e.Err.Error()
}

func (e *PollError) Unwrap() error {
	return e.Err
}
