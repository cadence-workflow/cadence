//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination poll_tracker_mock.go -package matching github.com/uber/cadence/client/matching PollTracker

package matching

import "github.com/uber/cadence/common/types"

// PollTracker is told about every poll in flight, with the request which cancels it
type PollTracker interface {
	// PollStarted is called before the poll is sent to matching; the returned function is called when the poll ends
	PollStarted(cancelRequest *types.CancelOutstandingPollRequest) (done func())
}

type noopPollTracker struct{}

// NewNoopPollTracker creates a PollTracker which tracks nothing
func NewNoopPollTracker() PollTracker {
	return noopPollTracker{}
}

func (noopPollTracker) PollStarted(*types.CancelOutstandingPollRequest) func() { return func() {} }
