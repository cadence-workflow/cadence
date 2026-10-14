// Package asyncworkflowqueuetest provides an in-memory persistence.AsyncWorkflowQueueStore for tests.
// Only _test.go files and test helpers may import it; production code must not.
//
// Duplicate message ids return *types.InternalServiceError because the real stores do not define a
// dedicated error yet.
package asyncworkflowqueuetest

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type queueKey struct {
	shardID int
	source  string
}

// Store is an in-memory persistence.AsyncWorkflowQueueStore.
type Store struct {
	mu   sync.Mutex
	rows map[queueKey][]*persistence.AsyncWorkflowMessage
	acks map[queueKey]int64
	dlq  map[queueKey][]*persistence.AsyncWorkflowDLQMessage
}

var _ persistence.AsyncWorkflowQueueStore = (*Store)(nil)

// NewStore returns an empty in-memory store.
func NewStore() *Store {
	return &Store{
		rows: make(map[queueKey][]*persistence.AsyncWorkflowMessage),
		acks: make(map[queueKey]int64),
		dlq:  make(map[queueKey][]*persistence.AsyncWorkflowDLQMessage),
	}
}

// NewManager wraps the store in the production AsyncWorkflowQueueManager so requests are validated.
func NewManager(s *Store) persistence.AsyncWorkflowQueueManager {
	return persistence.NewAsyncWorkflowQueueManager(s, log.NewNoop())
}

// GetName returns the store name.
func (s *Store) GetName() string { return "asyncworkflowqueuetest" }

// Close is a no-op.
func (s *Store) Close() {}

func copyMessage(m *persistence.AsyncWorkflowMessage) *persistence.AsyncWorkflowMessage {
	c := *m
	c.Payload = append([]byte(nil), m.Payload...)
	return &c
}

func copyDLQMessage(m *persistence.AsyncWorkflowDLQMessage) *persistence.AsyncWorkflowDLQMessage {
	c := *m
	c.Payload = append([]byte(nil), m.Payload...)
	return &c
}

func msgID(m *persistence.AsyncWorkflowMessage) int64    { return m.MessageID }
func dlqID(m *persistence.AsyncWorkflowDLQMessage) int64 { return m.MessageID }

// upsertSorted inserts item into items keeping ascending id order. If the id already exists the item
// is replaced when replace is true, otherwise items is returned unchanged with false.
func upsertSorted[T any](items []T, item T, idOf func(T) int64, replace bool) ([]T, bool) {
	id := idOf(item)
	i := sort.Search(len(items), func(i int) bool { return idOf(items[i]) >= id })
	if i < len(items) && idOf(items[i]) == id {
		if !replace {
			return items, false
		}
		items[i] = item
		return items, true
	}
	items = append(items, item)
	copy(items[i+1:], items[i:])
	items[i] = item
	return items, true
}

// pageAfter returns up to pageSize converted items with id > after.
func pageAfter[T any, R any](items []T, after int64, pageSize int, idOf func(T) int64, convert func(T) R) []R {
	i := sort.Search(len(items), func(i int) bool { return idOf(items[i]) > after })
	out := []R{}
	for ; i < len(items) && len(out) < pageSize; i++ {
		out = append(out, convert(items[i]))
	}
	return out
}

// deleteUpTo drops items with id <= maxID.
func deleteUpTo[T any](items []T, maxID int64, idOf func(T) int64) []T {
	i := sort.Search(len(items), func(i int) bool { return idOf(items[i]) > maxID })
	return append([]T(nil), items[i:]...)
}

// EnqueueAsyncWorkflowMessage inserts a message, rejecting duplicate message ids.
func (s *Store) EnqueueAsyncWorkflowMessage(ctx context.Context, request *persistence.InternalEnqueueAsyncWorkflowMessageRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	r := request.EnqueueAsyncWorkflowMessageRequest
	key := queueKey{r.ShardID, r.SourceCluster}
	msg := &persistence.AsyncWorkflowMessage{
		ShardID:         r.ShardID,
		SourceCluster:   r.SourceCluster,
		MessageID:       r.MessageID,
		DomainName:      r.DomainName,
		WorkflowID:      r.WorkflowID,
		RequestID:       r.RequestID,
		RequestType:     r.RequestType,
		Payload:         append([]byte(nil), r.Payload...),
		PayloadEncoding: r.PayloadEncoding,
		CreatedTime:     request.CurrentTimeStamp,
	}
	rows, ok := upsertSorted(s.rows[key], msg, msgID, false)
	if !ok {
		return &types.InternalServiceError{Message: fmt.Sprintf("async workflow message %d already exists for shard %d source %q", r.MessageID, r.ShardID, r.SourceCluster)}
	}
	s.rows[key] = rows
	return nil
}

// ReadAsyncWorkflowMessages reads a page of messages in ascending id order.
func (s *Store) ReadAsyncWorkflowMessages(ctx context.Context, request *persistence.ReadAsyncWorkflowMessagesRequest) (*persistence.ReadAsyncWorkflowMessagesResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	msgs := pageAfter(s.rows[queueKey{request.ShardID, request.SourceCluster}], request.ExclusiveMinMessageID, request.PageSize, msgID, copyMessage)
	return &persistence.ReadAsyncWorkflowMessagesResponse{Messages: msgs}, nil
}

// GetAsyncWorkflowAckLevels returns the ack levels of every source cluster on the shard.
func (s *Store) GetAsyncWorkflowAckLevels(ctx context.Context, request *persistence.GetAsyncWorkflowAckLevelsRequest) (*persistence.GetAsyncWorkflowAckLevelsResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	levels := map[string]int64{}
	for k, v := range s.acks {
		if k.shardID == request.ShardID {
			levels[k.source] = v
		}
	}
	return &persistence.GetAsyncWorkflowAckLevelsResponse{AckLevels: levels}, nil
}

// UpdateAsyncWorkflowAckLevel upserts the ack level unconditionally.
func (s *Store) UpdateAsyncWorkflowAckLevel(ctx context.Context, request *persistence.InternalUpdateAsyncWorkflowAckLevelRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	r := request.UpdateAsyncWorkflowAckLevelRequest
	s.acks[queueKey{r.ShardID, r.SourceCluster}] = r.AckLevel
	return nil
}

// RangeDeleteAsyncWorkflowMessages deletes messages with id <= InclusiveMaxMessageID.
func (s *Store) RangeDeleteAsyncWorkflowMessages(ctx context.Context, request *persistence.RangeDeleteAsyncWorkflowMessagesRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := queueKey{request.ShardID, request.SourceCluster}
	s.rows[key] = deleteUpTo(s.rows[key], request.InclusiveMaxMessageID, msgID)
	return nil
}

// EnqueueAsyncWorkflowMessageToDLQ stores a DLQ row, overwriting any existing row for the same message.
func (s *Store) EnqueueAsyncWorkflowMessageToDLQ(ctx context.Context, request *persistence.InternalEnqueueAsyncWorkflowMessageToDLQRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	m := request.Message
	key := queueKey{m.ShardID, m.SourceCluster}
	row := &persistence.AsyncWorkflowDLQMessage{AsyncWorkflowMessage: *copyMessage(m), Reason: request.Reason}
	s.dlq[key], _ = upsertSorted(s.dlq[key], row, dlqID, true)
	return nil
}

// ReadAsyncWorkflowMessagesFromDLQ reads a page of DLQ messages in ascending id order.
func (s *Store) ReadAsyncWorkflowMessagesFromDLQ(ctx context.Context, request *persistence.ReadAsyncWorkflowMessagesFromDLQRequest) (*persistence.ReadAsyncWorkflowMessagesFromDLQResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	msgs := pageAfter(s.dlq[queueKey{request.ShardID, request.SourceCluster}], request.ExclusiveMinMessageID, request.PageSize, dlqID, copyDLQMessage)
	return &persistence.ReadAsyncWorkflowMessagesFromDLQResponse{Messages: msgs}, nil
}

// RangeDeleteAsyncWorkflowMessagesFromDLQ deletes DLQ messages with id <= InclusiveMaxMessageID.
func (s *Store) RangeDeleteAsyncWorkflowMessagesFromDLQ(ctx context.Context, request *persistence.RangeDeleteAsyncWorkflowMessagesFromDLQRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := queueKey{request.ShardID, request.SourceCluster}
	s.dlq[key] = deleteUpTo(s.dlq[key], request.InclusiveMaxMessageID, dlqID)
	return nil
}

// Rows returns copies of the queue rows for one shard and source cluster in ascending id order.
func (s *Store) Rows(shardID int, source string) []*persistence.AsyncWorkflowMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []*persistence.AsyncWorkflowMessage
	for _, m := range s.rows[queueKey{shardID, source}] {
		out = append(out, copyMessage(m))
	}
	return out
}

// DLQRows returns copies of the DLQ rows for one shard and source cluster in ascending id order.
func (s *Store) DLQRows(shardID int, source string) []*persistence.AsyncWorkflowDLQMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []*persistence.AsyncWorkflowDLQMessage
	for _, m := range s.dlq[queueKey{shardID, source}] {
		out = append(out, copyDLQMessage(m))
	}
	return out
}

// AckLevel returns the ack level and whether a control row has been written.
func (s *Store) AckLevel(shardID int, source string) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	level, ok := s.acks[queueKey{shardID, source}]
	return level, ok
}
