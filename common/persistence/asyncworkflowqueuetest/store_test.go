package asyncworkflowqueuetest_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/asyncworkflowqueuetest"
	"github.com/uber/cadence/common/types"
)

func enqueueReq(shardID int, source string, id int64) *persistence.EnqueueAsyncWorkflowMessageRequest {
	return &persistence.EnqueueAsyncWorkflowMessageRequest{
		ShardID:         shardID,
		SourceCluster:   source,
		MessageID:       id,
		DomainName:      "test-domain",
		WorkflowID:      fmt.Sprintf("wf-%d", id),
		RequestID:       fmt.Sprintf("req-%d", id),
		RequestType:     persistence.AsyncWorkflowRequestTypeStartWorkflow,
		Payload:         []byte{byte(id), 0x01, 0x02},
		PayloadEncoding: constants.EncodingTypeThriftRW,
	}
}

func mustEnqueue(t *testing.T, m persistence.AsyncWorkflowQueueManager, shardID int, source string, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		require.NoError(t, m.EnqueueAsyncWorkflowMessage(context.Background(), enqueueReq(shardID, source, id)))
	}
}

func messageIDs(msgs []*persistence.AsyncWorkflowMessage) []int64 {
	ids := make([]int64, 0, len(msgs))
	for _, msg := range msgs {
		ids = append(ids, msg.MessageID)
	}
	return ids
}

func dlqMessageIDs(msgs []*persistence.AsyncWorkflowDLQMessage) []int64 {
	ids := make([]int64, 0, len(msgs))
	for _, msg := range msgs {
		ids = append(ids, msg.MessageID)
	}
	return ids
}

func readIDs(t *testing.T, m persistence.AsyncWorkflowQueueManager, shardID int, source string, after int64, pageSize int) []int64 {
	t.Helper()
	resp, err := m.ReadAsyncWorkflowMessages(context.Background(), &persistence.ReadAsyncWorkflowMessagesRequest{
		ShardID:               shardID,
		SourceCluster:         source,
		ExclusiveMinMessageID: after,
		PageSize:              pageSize,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	return messageIDs(resp.Messages)
}

func readDLQIDs(t *testing.T, m persistence.AsyncWorkflowQueueManager, shardID int, source string, after int64, pageSize int) []int64 {
	t.Helper()
	resp, err := m.ReadAsyncWorkflowMessagesFromDLQ(context.Background(), &persistence.ReadAsyncWorkflowMessagesFromDLQRequest{
		ShardID:               shardID,
		SourceCluster:         source,
		ExclusiveMinMessageID: after,
		PageSize:              pageSize,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	return dlqMessageIDs(resp.Messages)
}

func TestStoreRoundTrip(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		run  func(t *testing.T, s *asyncworkflowqueuetest.Store, m persistence.AsyncWorkflowQueueManager)
	}{
		{
			name: "enqueue preserves every field and reads ascending regardless of insert order",
			run: func(t *testing.T, s *asyncworkflowqueuetest.Store, m persistence.AsyncWorkflowQueueManager) {
				mustEnqueue(t, m, 0, "c1", 3, 1, 2)

				assert.Equal(t, []int64{1, 2, 3}, readIDs(t, m, 0, "c1", 0, 10))

				rows := s.Rows(0, "c1")
				require.Len(t, rows, 3)
				got := *rows[0]
				assert.False(t, got.CreatedTime.IsZero(), "CreatedTime must be stamped from the manager's write time")
				assert.WithinDuration(t, time.Now(), got.CreatedTime, time.Minute)
				got.CreatedTime = time.Time{}
				assert.Equal(t, persistence.AsyncWorkflowMessage{
					ShardID:         0,
					SourceCluster:   "c1",
					MessageID:       1,
					DomainName:      "test-domain",
					WorkflowID:      "wf-1",
					RequestID:       "req-1",
					RequestType:     persistence.AsyncWorkflowRequestTypeStartWorkflow,
					Payload:         []byte{1, 0x01, 0x02},
					PayloadEncoding: constants.EncodingTypeThriftRW,
				}, got)
			},
		},
		{
			name: "read honors exclusive min message id and page size",
			run: func(t *testing.T, _ *asyncworkflowqueuetest.Store, m persistence.AsyncWorkflowQueueManager) {
				mustEnqueue(t, m, 0, "c1", 1, 2, 3, 4, 5)

				assert.Equal(t, []int64{2, 3}, readIDs(t, m, 0, "c1", 1, 2))
				assert.Equal(t, []int64{4, 5}, readIDs(t, m, 0, "c1", 3, 10))
				assert.Empty(t, readIDs(t, m, 0, "c1", 5, 10))
			},
		},
		{
			name: "duplicate message id is rejected and leaves the original row",
			run: func(t *testing.T, s *asyncworkflowqueuetest.Store, m persistence.AsyncWorkflowQueueManager) {
				mustEnqueue(t, m, 0, "c1", 7)

				dup := enqueueReq(0, "c1", 7)
				dup.WorkflowID = "different-workflow"
				err := m.EnqueueAsyncWorkflowMessage(ctx, dup)

				var internal *types.InternalServiceError
				require.ErrorAs(t, err, &internal)
				rows := s.Rows(0, "c1")
				require.Len(t, rows, 1)
				assert.Equal(t, "wf-7", rows[0].WorkflowID)
			},
		},
		{
			name: "queues are isolated by shard and source cluster",
			run: func(t *testing.T, s *asyncworkflowqueuetest.Store, m persistence.AsyncWorkflowQueueManager) {
				mustEnqueue(t, m, 0, "c1", 1, 2)
				mustEnqueue(t, m, 0, "c2", 1)
				mustEnqueue(t, m, 1, "c1", 1, 2, 3)

				assert.Equal(t, []int64{1, 2}, messageIDs(s.Rows(0, "c1")))
				assert.Equal(t, []int64{1}, messageIDs(s.Rows(0, "c2")))
				assert.Equal(t, []int64{1, 2, 3}, messageIDs(s.Rows(1, "c1")))
				assert.Empty(t, s.Rows(2, "c1"))
				assert.Empty(t, readIDs(t, m, 0, "c3", 0, 10))
			},
		},
		{
			name: "ack level upsert, overwrite and per-shard get",
			run: func(t *testing.T, s *asyncworkflowqueuetest.Store, m persistence.AsyncWorkflowQueueManager) {
				levels, err := m.GetAsyncWorkflowAckLevels(ctx, &persistence.GetAsyncWorkflowAckLevelsRequest{ShardID: 0})
				require.NoError(t, err)
				assert.Empty(t, levels.AckLevels)
				_, ok := s.AckLevel(0, "c1")
				assert.False(t, ok)

				for _, u := range []persistence.UpdateAsyncWorkflowAckLevelRequest{
					{ShardID: 0, SourceCluster: "c1", AckLevel: 5},
					{ShardID: 0, SourceCluster: "c2", AckLevel: 7},
					{ShardID: 1, SourceCluster: "c1", AckLevel: 9},
					{ShardID: 0, SourceCluster: "c1", AckLevel: 8},
				} {
					u := u
					require.NoError(t, m.UpdateAsyncWorkflowAckLevel(ctx, &u))
				}

				levels, err = m.GetAsyncWorkflowAckLevels(ctx, &persistence.GetAsyncWorkflowAckLevelsRequest{ShardID: 0})
				require.NoError(t, err)
				assert.Equal(t, map[string]int64{"c1": 8, "c2": 7}, levels.AckLevels)

				level, ok := s.AckLevel(0, "c1")
				assert.True(t, ok)
				assert.Equal(t, int64(8), level)
				level, ok = s.AckLevel(1, "c1")
				assert.True(t, ok)
				assert.Equal(t, int64(9), level)
				level, ok = s.AckLevel(0, "missing")
				assert.False(t, ok)
				assert.Equal(t, int64(0), level)
			},
		},
		{
			name: "range delete is inclusive and scoped to one queue",
			run: func(t *testing.T, s *asyncworkflowqueuetest.Store, m persistence.AsyncWorkflowQueueManager) {
				mustEnqueue(t, m, 0, "c1", 1, 2, 3, 4, 5)
				mustEnqueue(t, m, 0, "c2", 1, 2)

				require.NoError(t, m.RangeDeleteAsyncWorkflowMessages(ctx, &persistence.RangeDeleteAsyncWorkflowMessagesRequest{
					ShardID: 0, SourceCluster: "c1", InclusiveMaxMessageID: 3,
				}))
				assert.Equal(t, []int64{4, 5}, messageIDs(s.Rows(0, "c1")))
				assert.Equal(t, []int64{1, 2}, messageIDs(s.Rows(0, "c2")))

				require.NoError(t, m.RangeDeleteAsyncWorkflowMessages(ctx, &persistence.RangeDeleteAsyncWorkflowMessagesRequest{
					ShardID: 0, SourceCluster: "c1", InclusiveMaxMessageID: 100,
				}))
				assert.Empty(t, s.Rows(0, "c1"))
				assert.Equal(t, []int64{1, 2}, messageIDs(s.Rows(0, "c2")))
			},
		},
		{
			name: "dlq enqueue, overwrite, ascending read, range delete; queue rows untouched",
			run: func(t *testing.T, s *asyncworkflowqueuetest.Store, m persistence.AsyncWorkflowQueueManager) {
				mustEnqueue(t, m, 0, "c1", 1, 2, 3)
				rows := s.Rows(0, "c1")
				require.Len(t, rows, 3)

				require.NoError(t, m.EnqueueAsyncWorkflowMessageToDLQ(ctx, &persistence.EnqueueAsyncWorkflowMessageToDLQRequest{Message: rows[2], Reason: "boom"}))
				require.NoError(t, m.EnqueueAsyncWorkflowMessageToDLQ(ctx, &persistence.EnqueueAsyncWorkflowMessageToDLQRequest{Message: rows[0], Reason: "first"}))

				dlq := s.DLQRows(0, "c1")
				require.Len(t, dlq, 2)
				assert.Equal(t, []int64{1, 3}, dlqMessageIDs(dlq))
				assert.Equal(t, "first", dlq[0].Reason)
				assert.Equal(t, "boom", dlq[1].Reason)
				assert.Equal(t, rows[0].Payload, dlq[0].Payload)
				assert.Equal(t, rows[0].WorkflowID, dlq[0].WorkflowID)
				assert.Equal(t, []int64{1, 2, 3}, messageIDs(s.Rows(0, "c1")), "DLQ write must not remove the source row")

				// a retry of the same source message overwrites the DLQ row
				require.NoError(t, m.EnqueueAsyncWorkflowMessageToDLQ(ctx, &persistence.EnqueueAsyncWorkflowMessageToDLQRequest{Message: rows[2], Reason: "again"}))
				dlq = s.DLQRows(0, "c1")
				require.Len(t, dlq, 2)
				assert.Equal(t, "again", dlq[1].Reason)

				assert.Equal(t, []int64{3}, readDLQIDs(t, m, 0, "c1", 1, 10))
				assert.Equal(t, []int64{1}, readDLQIDs(t, m, 0, "c1", 0, 1))
				assert.Empty(t, readDLQIDs(t, m, 0, "c2", 0, 10))

				require.NoError(t, m.RangeDeleteAsyncWorkflowMessagesFromDLQ(ctx, &persistence.RangeDeleteAsyncWorkflowMessagesFromDLQRequest{
					ShardID: 0, SourceCluster: "c1", InclusiveMaxMessageID: 1,
				}))
				assert.Equal(t, []int64{3}, dlqMessageIDs(s.DLQRows(0, "c1")))
				assert.Equal(t, []int64{1, 2, 3}, messageIDs(s.Rows(0, "c1")))
			},
		},
		{
			name: "dlq read pages through more rows than the page size until exhausted",
			run: func(t *testing.T, s *asyncworkflowqueuetest.Store, m persistence.AsyncWorkflowQueueManager) {
				mustEnqueue(t, m, 0, "c1", 1, 2, 3, 4, 5)
				for _, row := range s.Rows(0, "c1") {
					require.NoError(t, m.EnqueueAsyncWorkflowMessageToDLQ(ctx, &persistence.EnqueueAsyncWorkflowMessageToDLQRequest{Message: row, Reason: "r"}))
				}

				var got []int64
				after := int64(0)
				pages := 0
				for {
					page := readDLQIDs(t, m, 0, "c1", after, 2)
					if len(page) == 0 {
						break
					}
					assert.LessOrEqual(t, len(page), 2)
					got = append(got, page...)
					after = page[len(page)-1]
					pages++
					require.LessOrEqual(t, pages, 10, "paging did not end")
				}
				assert.Equal(t, []int64{1, 2, 3, 4, 5}, got)
				assert.Equal(t, 3, pages)
			},
		},
		{
			name: "dlq overwrite replaces payload and workflow id and keeps the row count",
			run: func(t *testing.T, s *asyncworkflowqueuetest.Store, m persistence.AsyncWorkflowQueueManager) {
				mustEnqueue(t, m, 0, "c1", 1, 2)
				rows := s.Rows(0, "c1")
				require.NoError(t, m.EnqueueAsyncWorkflowMessageToDLQ(ctx, &persistence.EnqueueAsyncWorkflowMessageToDLQRequest{Message: rows[0], Reason: "first"}))
				require.NoError(t, m.EnqueueAsyncWorkflowMessageToDLQ(ctx, &persistence.EnqueueAsyncWorkflowMessageToDLQRequest{Message: rows[1], Reason: "other"}))

				replacement := *rows[0]
				replacement.Payload = []byte{9, 9, 9}
				replacement.WorkflowID = "replaced-workflow"
				require.NoError(t, m.EnqueueAsyncWorkflowMessageToDLQ(ctx, &persistence.EnqueueAsyncWorkflowMessageToDLQRequest{Message: &replacement, Reason: "second"}))

				dlq := s.DLQRows(0, "c1")
				require.Len(t, dlq, 2)
				assert.Equal(t, []byte{9, 9, 9}, dlq[0].Payload)
				assert.Equal(t, "replaced-workflow", dlq[0].WorkflowID)
				assert.Equal(t, "second", dlq[0].Reason)
				assert.Equal(t, "other", dlq[1].Reason)
			},
		},
		{
			name: "dlq rows and dlq reads are copies and cannot mutate the store",
			run: func(t *testing.T, s *asyncworkflowqueuetest.Store, m persistence.AsyncWorkflowQueueManager) {
				mustEnqueue(t, m, 0, "c1", 1)
				rows := s.Rows(0, "c1")
				require.NoError(t, m.EnqueueAsyncWorkflowMessageToDLQ(ctx, &persistence.EnqueueAsyncWorkflowMessageToDLQRequest{Message: rows[0], Reason: "boom"}))

				// changing the source message after the DLQ write must not leak into the DLQ row
				rows[0].Payload[0] = 0xAA

				dlq := s.DLQRows(0, "c1")
				require.Len(t, dlq, 1)
				dlq[0].Payload[0] = 0xFF
				dlq[0].WorkflowID = "changed"
				dlq[0].Reason = "changed"

				resp, err := m.ReadAsyncWorkflowMessagesFromDLQ(ctx, &persistence.ReadAsyncWorkflowMessagesFromDLQRequest{ShardID: 0, SourceCluster: "c1", PageSize: 10})
				require.NoError(t, err)
				require.Len(t, resp.Messages, 1)
				assert.Equal(t, []byte{1, 0x01, 0x02}, resp.Messages[0].Payload)
				resp.Messages[0].Payload[1] = 0xEE
				resp.Messages[0].Reason = "changed-read"

				fresh := s.DLQRows(0, "c1")
				require.Len(t, fresh, 1)
				assert.Equal(t, []byte{1, 0x01, 0x02}, fresh[0].Payload)
				assert.Equal(t, "wf-1", fresh[0].WorkflowID)
				assert.Equal(t, "boom", fresh[0].Reason)
			},
		},
		{
			name: "returned rows are copies and cannot mutate the store",
			run: func(t *testing.T, s *asyncworkflowqueuetest.Store, m persistence.AsyncWorkflowQueueManager) {
				mustEnqueue(t, m, 0, "c1", 1)

				rows := s.Rows(0, "c1")
				require.Len(t, rows, 1)
				rows[0].Payload[0] = 0xFF
				rows[0].WorkflowID = "mutated"

				resp, err := m.ReadAsyncWorkflowMessages(ctx, &persistence.ReadAsyncWorkflowMessagesRequest{ShardID: 0, SourceCluster: "c1", PageSize: 10})
				require.NoError(t, err)
				require.Len(t, resp.Messages, 1)
				resp.Messages[0].Payload[1] = 0xEE

				fresh := s.Rows(0, "c1")
				assert.Equal(t, []byte{1, 0x01, 0x02}, fresh[0].Payload)
				assert.Equal(t, "wf-1", fresh[0].WorkflowID)
			},
		},
		{
			name: "cancelled context fails every operation",
			run: func(t *testing.T, _ *asyncworkflowqueuetest.Store, m persistence.AsyncWorkflowQueueManager) {
				cancelled, cancel := context.WithCancel(context.Background())
				cancel()

				assert.ErrorIs(t, m.EnqueueAsyncWorkflowMessage(cancelled, enqueueReq(0, "c1", 1)), context.Canceled)
				_, err := m.ReadAsyncWorkflowMessages(cancelled, &persistence.ReadAsyncWorkflowMessagesRequest{ShardID: 0, SourceCluster: "c1", PageSize: 1})
				assert.ErrorIs(t, err, context.Canceled)
				_, err = m.GetAsyncWorkflowAckLevels(cancelled, &persistence.GetAsyncWorkflowAckLevelsRequest{ShardID: 0})
				assert.ErrorIs(t, err, context.Canceled)
				assert.ErrorIs(t, m.UpdateAsyncWorkflowAckLevel(cancelled, &persistence.UpdateAsyncWorkflowAckLevelRequest{ShardID: 0, SourceCluster: "c1", AckLevel: 1}), context.Canceled)
				assert.ErrorIs(t, m.RangeDeleteAsyncWorkflowMessages(cancelled, &persistence.RangeDeleteAsyncWorkflowMessagesRequest{ShardID: 0, SourceCluster: "c1", InclusiveMaxMessageID: 1}), context.Canceled)
			},
		},
		{
			name: "manager validation rejects bad requests before they reach the store",
			run: func(t *testing.T, s *asyncworkflowqueuetest.Store, m persistence.AsyncWorkflowQueueManager) {
				bad := enqueueReq(0, "c1", 1)
				bad.PayloadEncoding = ""
				err := m.EnqueueAsyncWorkflowMessage(ctx, bad)

				var badRequest *types.BadRequestError
				require.ErrorAs(t, err, &badRequest)
				assert.Empty(t, s.Rows(0, "c1"))
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := asyncworkflowqueuetest.NewStore()
			tc.run(t, s, asyncworkflowqueuetest.NewManager(s))
		})
	}
}

func TestStoreConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	s := asyncworkflowqueuetest.NewStore()
	m := asyncworkflowqueuetest.NewManager(s)

	const writers = 4
	const perWriter = 50
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		w := w
		wg.Add(1)
		go func() {
			defer wg.Done()
			source := fmt.Sprintf("c%d", w%2)
			for i := 1; i <= perWriter; i++ {
				id := int64(w*perWriter + i)
				assert.NoError(t, m.EnqueueAsyncWorkflowMessage(ctx, enqueueReq(w, source, id)))
				_, err := m.ReadAsyncWorkflowMessages(ctx, &persistence.ReadAsyncWorkflowMessagesRequest{ShardID: w, SourceCluster: source, PageSize: 10})
				assert.NoError(t, err)
				assert.NoError(t, m.UpdateAsyncWorkflowAckLevel(ctx, &persistence.UpdateAsyncWorkflowAckLevelRequest{ShardID: w, SourceCluster: source, AckLevel: id}))
				if i%10 == 0 {
					assert.NoError(t, m.RangeDeleteAsyncWorkflowMessages(ctx, &persistence.RangeDeleteAsyncWorkflowMessagesRequest{ShardID: w, SourceCluster: source, InclusiveMaxMessageID: id - 5}))
				}
				_ = s.Rows(w, source)
			}
		}()
	}
	wg.Wait()

	for w := 0; w < writers; w++ {
		source := fmt.Sprintf("c%d", w%2)
		last := int64(w*perWriter + perWriter)
		level, ok := s.AckLevel(w, source)
		assert.True(t, ok)
		assert.Equal(t, last, level)
		rows := s.Rows(w, source)
		require.NotEmpty(t, rows)
		assert.Equal(t, last, rows[len(rows)-1].MessageID)
	}
}

func TestStoreNameAndClose(t *testing.T) {
	var s persistence.AsyncWorkflowQueueStore = asyncworkflowqueuetest.NewStore()
	assert.NotEmpty(t, s.GetName())
	assert.NotPanics(t, s.Close)
}
