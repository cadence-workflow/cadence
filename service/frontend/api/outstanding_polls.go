package api

import (
	"maps"
	"sync"

	"github.com/uber/cadence/common/types"
)

type (
	outstandingPoll struct {
		domainID     string
		taskListType int32
		taskList     *types.TaskList
	}

	// outstandingPolls tracks long-polls this frontend has sent to matching, so they can be cancelled on shutdown
	outstandingPolls struct {
		mu     sync.Mutex
		closed bool
		polls  map[string]outstandingPoll // pollerID -> poll
	}
)

func newOutstandingPolls() *outstandingPolls {
	return &outstandingPolls{polls: make(map[string]outstandingPoll)}
}

func (p *outstandingPolls) add(pollerID string, poll outstandingPoll) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.polls[pollerID] = poll
}

func (p *outstandingPolls) remove(pollerID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.polls, pollerID)
}

func (p *outstandingPolls) isClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

// close marks the registry as closed, so no new polls should be sent to matching, and returns the outstanding ones
func (p *outstandingPolls) close() map[string]outstandingPoll {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return maps.Clone(p.polls)
}
