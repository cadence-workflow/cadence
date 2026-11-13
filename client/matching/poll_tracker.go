//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination poll_tracker_mock.go -package matching github.com/uber/cadence/client/matching PollTracker

package matching

import "github.com/uber/cadence/common/types"

// PollTracker is told about every poll in flight, with the request which cancels it
type PollTracker interface {
	// PollStarted is called before the poll is sent to matching; the returned function is called when the poll ends
	PollStarted(cancelRequest *types.CancelOutstandingPollRequest) (done func())
}

// NoopPollTracker is a PollTracker which tracks nothing
type NoopPollTracker struct{}

// PollStarted does nothing
func (NoopPollTracker) PollStarted(*types.CancelOutstandingPollRequest) func() { return func() {} }
