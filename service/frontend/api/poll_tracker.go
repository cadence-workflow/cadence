package api

import (
	"context"
	"maps"
	"sync"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

const cancelPollsConcurrency = 100

var _ matching.PollTracker = (*PollTracker)(nil)

// PollTracker keeps the polls frontend has sent to matching and not finished yet, by their poller ID
type PollTracker struct {
	mu    sync.Mutex
	polls map[string]*types.CancelOutstandingPollRequest
}

// NewPollTracker creates an empty PollTracker
func NewPollTracker() *PollTracker {
	return &PollTracker{polls: make(map[string]*types.CancelOutstandingPollRequest)}
}

// PollStarted tracks the poll until the returned function is called
func (t *PollTracker) PollStarted(cancel *types.CancelOutstandingPollRequest) func() {
	pollerID := cancel.GetPollerID()
	if pollerID == "" {
		return func() {}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.polls[pollerID] = cancel
	return func() { t.remove(pollerID) }
}

func (t *PollTracker) remove(pollerID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.polls, pollerID)
}

// cancelAll cancels every poll in flight. It is best-effort:
// a poll which fails to cancel is logged and finishes on its own timeout.
func (t *PollTracker) cancelAll(ctx context.Context, client matching.Client, logger log.Logger) {
	t.mu.Lock()
	polls := maps.Clone(t.polls)
	t.mu.Unlock()

	var wg sync.WaitGroup
	limiter := make(chan struct{}, cancelPollsConcurrency)
	for _, poll := range polls {
		limiter <- struct{}{}
		wg.Go(func() {
			defer func() { <-limiter }()
			if err := client.CancelOutstandingPoll(ctx, poll); err != nil {
				logger.Warn("Failed to cancel outstanding poll",
					tag.WorkflowDomainID(poll.GetDomainUUID()),
					tag.WorkflowTaskListName(poll.GetTaskList().GetName()),
					tag.Error(err),
				)
			}
		})
	}
	wg.Wait()
}
