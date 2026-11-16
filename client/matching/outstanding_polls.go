package matching

import (
	"maps"
	"sync"

	"github.com/uber/cadence/common/types"
)

type outstandingPoll struct {
	domainID     string
	taskListType int32
	// taskList is the partition the poll was sent to
	taskList types.TaskList
}

// outstandingPolls tracks polls in flight by their poller ID
type outstandingPolls struct {
	mu     sync.Mutex
	closed bool // set by close; new polls are rejected after that
	polls  map[string]outstandingPoll
}

func newOutstandingPolls() *outstandingPolls {
	return &outstandingPolls{polls: make(map[string]outstandingPoll)}
}

// add tracks the poll until the returned function is called.
// It returns false if no more polls are accepted.
func (p *outstandingPolls) add(pollerID string, poll outstandingPoll) (bool, func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return false, nil
	}
	if pollerID == "" {
		return true, func() {}
	}
	p.polls[pollerID] = poll
	return true, func() { p.remove(pollerID) }
}

func (p *outstandingPolls) remove(pollerID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.polls, pollerID)
}

// close stops accepting new polls and returns the ones in flight
func (p *outstandingPolls) close() map[string]outstandingPoll {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return maps.Clone(p.polls)
}
