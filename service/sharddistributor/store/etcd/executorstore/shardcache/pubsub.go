package shardcache

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/sharddistributor/store"
)

// executorStatePubSub manages subscriptions to executor state changes
type executorStatePubSub struct {
	mu          sync.RWMutex
	subscribers map[string]chan<- map[*store.ShardOwner][]string
	logger      log.Logger
	namespace   string
}

func newExecutorStatePubSub(logger log.Logger, namespace string) *executorStatePubSub {
	return &executorStatePubSub{
		subscribers: make(map[string]chan<- map[*store.ShardOwner][]string),
		logger:      logger,
		namespace:   namespace,
	}
}

// Subscribe returns a channel that receives executor state updates.
func (p *executorStatePubSub) subscribe(ctx context.Context) (chan map[*store.ShardOwner][]string, func()) {
	ch := make(chan map[*store.ShardOwner][]string)
	uniqueID := uuid.New().String()

	p.mu.Lock()
	defer p.mu.Unlock()
	p.subscribers[uniqueID] = ch

	unSub := func() {
		p.unSubscribe(uniqueID)
	}

	return ch, unSub
}

func (p *executorStatePubSub) unSubscribe(uniqueID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.subscribers, uniqueID)
}

// Publish sends the state to all subscribers (non-blocking)
func (p *executorStatePubSub) publish(state map[*store.ShardOwner][]string) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, sub := range p.subscribers {
		select {
		case sub <- state:
		default:
			// Subscriber is not reading fast enough, skip this update
			p.logger.Warn("Subscriber not keeping up with state updates, dropping update", tag.ShardNamespace(p.namespace))
		}
	}
}
