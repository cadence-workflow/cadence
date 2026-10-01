package engineimpl

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/engine/testdata"
)

// findWorkflowIDs returns one workflow id that hashes to ownShardID and one that hashes elsewhere.
func findWorkflowIDs(t *testing.T, numShards, ownShardID int) (owned, foreign string) {
	t.Helper()
	for i := 0; i < 10000 && (owned == "" || foreign == ""); i++ {
		candidate := "wf-" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)) + string(rune('a'+(i/676)%26))
		if common.WorkflowIDToHistoryShard(candidate, numShards) == ownShardID {
			if owned == "" {
				owned = candidate
			}
		} else if foreign == "" {
			foreign = candidate
		}
	}
	require.NotEmpty(t, owned)
	require.NotEmpty(t, foreign)
	return owned, foreign
}

func TestEnqueueAsyncWorkflowMessage(t *testing.T) {
	// The engine under test runs on shard 0 (NewEngineForTest builds a shard.TestContext with the zero ShardID).
	// The default test config has a single shard, which would make every workflow id belong to shard 0, so the
	// shard count is widened to let a workflow id that belongs to another shard exist.
	const (
		testNumShards = 16
		ownShardID    = 0
	)
	ownedWorkflowID, foreignWorkflowID := findWorkflowIDs(t, testNumShards, ownShardID)

	errManager := errors.New("boom")
	errUpdateShard := errors.New("update shard failed")

	thriftPayload := &types.DataBlob{EncodingType: types.EncodingTypeThriftRW.Ptr(), Data: []byte("envelope")}
	jsonPayload := &types.DataBlob{EncodingType: types.EncodingTypeJSON.Ptr(), Data: []byte(`{"k":"v"}`)}

	newRequest := func() *types.EnqueueAsyncWorkflowMessageRequest {
		return &types.EnqueueAsyncWorkflowMessageRequest{
			ShardID:     ownShardID,
			DomainName:  "test-domain",
			WorkflowID:  ownedWorkflowID,
			RequestID:   "req-1",
			RequestType: types.AsyncWorkflowRequestTypeSignalWithStartWorkflowExecution.Ptr(),
			Payload:     thriftPayload,
		}
	}
	withRequest := func(mutate func(r *types.EnqueueAsyncWorkflowMessageRequest)) *types.EnqueueAsyncWorkflowMessageRequest {
		r := newRequest()
		mutate(r)
		return r
	}

	tests := []struct {
		name string
		// request is the RPC request handed to the engine.
		request *types.EnqueueAsyncWorkflowMessageRequest
		// wantPersisted, when non-nil, is the request the queue manager must receive. SourceCluster and MessageID
		// are filled in by the test body: SourceCluster must equal the shard's current cluster name, and MessageID
		// must be positive and equal to the id returned to the caller. When nil, the manager must not be called.
		wantPersisted *persistence.EnqueueAsyncWorkflowMessageRequest
		managerErr    error
		// exhaustTaskIDs drains the shard's current task-id range so the next GenerateTaskID must renew the range,
		// which fails because the mocked shard manager returns errUpdateShard.
		exhaustTaskIDs bool
		wantBadRequest bool
		wantErrIs      error
	}{
		{
			name:    "success: signal-with-start, thriftrw payload, every field is persisted",
			request: newRequest(),
			wantPersisted: &persistence.EnqueueAsyncWorkflowMessageRequest{
				ShardID:         ownShardID,
				DomainName:      "test-domain",
				WorkflowID:      ownedWorkflowID,
				RequestID:       "req-1",
				RequestType:     persistence.AsyncWorkflowRequestTypeSignalWithStartWorkflow,
				Payload:         []byte("envelope"),
				PayloadEncoding: constants.EncodingTypeThriftRW,
			},
		},
		{
			name: "success: start-workflow request type maps to the persistence start type",
			request: withRequest(func(r *types.EnqueueAsyncWorkflowMessageRequest) {
				r.RequestType = types.AsyncWorkflowRequestTypeStartWorkflowExecution.Ptr()
			}),
			wantPersisted: &persistence.EnqueueAsyncWorkflowMessageRequest{
				ShardID:         ownShardID,
				DomainName:      "test-domain",
				WorkflowID:      ownedWorkflowID,
				RequestID:       "req-1",
				RequestType:     persistence.AsyncWorkflowRequestTypeStartWorkflow,
				Payload:         []byte("envelope"),
				PayloadEncoding: constants.EncodingTypeThriftRW,
			},
		},
		{
			name: "success: json payload encoding maps to the persistence json encoding",
			request: withRequest(func(r *types.EnqueueAsyncWorkflowMessageRequest) {
				r.Payload = jsonPayload
			}),
			wantPersisted: &persistence.EnqueueAsyncWorkflowMessageRequest{
				ShardID:         ownShardID,
				DomainName:      "test-domain",
				WorkflowID:      ownedWorkflowID,
				RequestID:       "req-1",
				RequestType:     persistence.AsyncWorkflowRequestTypeSignalWithStartWorkflow,
				Payload:         []byte(`{"k":"v"}`),
				PayloadEncoding: constants.EncodingTypeJSON,
			},
		},
		{
			name: "request shard id is not this shard: BadRequestError, nothing written",
			request: withRequest(func(r *types.EnqueueAsyncWorkflowMessageRequest) {
				r.ShardID = ownShardID + 1
			}),
			wantBadRequest: true,
		},
		{
			name: "workflow id hashes to another shard: BadRequestError, nothing written",
			request: withRequest(func(r *types.EnqueueAsyncWorkflowMessageRequest) {
				r.WorkflowID = foreignWorkflowID
			}),
			wantBadRequest: true,
		},
		{
			name: "nil payload: BadRequestError, nothing written",
			request: withRequest(func(r *types.EnqueueAsyncWorkflowMessageRequest) {
				r.Payload = nil
			}),
			wantBadRequest: true,
		},
		{
			name: "empty payload data: BadRequestError, nothing written",
			request: withRequest(func(r *types.EnqueueAsyncWorkflowMessageRequest) {
				r.Payload = &types.DataBlob{EncodingType: types.EncodingTypeThriftRW.Ptr(), Data: []byte{}}
			}),
			wantBadRequest: true,
		},
		{
			name: "unsupported payload encoding: BadRequestError, nothing written",
			request: withRequest(func(r *types.EnqueueAsyncWorkflowMessageRequest) {
				r.Payload = &types.DataBlob{EncodingType: types.EncodingType(99).Ptr(), Data: []byte("envelope")}
			}),
			wantBadRequest: true,
		},
		{
			name: "nil request type: BadRequestError, nothing written",
			request: withRequest(func(r *types.EnqueueAsyncWorkflowMessageRequest) {
				r.RequestType = nil
			}),
			wantBadRequest: true,
		},
		{
			name: "unknown request type: BadRequestError, nothing written",
			request: withRequest(func(r *types.EnqueueAsyncWorkflowMessageRequest) {
				unknown := types.AsyncWorkflowRequestType(99)
				r.RequestType = &unknown
			}),
			wantBadRequest: true,
		},
		{
			name:           "GenerateTaskID fails: error is returned unchanged, nothing written",
			request:        newRequest(),
			exhaustTaskIDs: true,
			wantErrIs:      errUpdateShard,
		},
		{
			name:    "queue manager error is returned unchanged",
			request: newRequest(),
			wantPersisted: &persistence.EnqueueAsyncWorkflowMessageRequest{
				ShardID:         ownShardID,
				DomainName:      "test-domain",
				WorkflowID:      ownedWorkflowID,
				RequestID:       "req-1",
				RequestType:     persistence.AsyncWorkflowRequestTypeSignalWithStartWorkflow,
				Payload:         []byte("envelope"),
				PayloadEncoding: constants.EncodingTypeThriftRW,
			},
			managerErr: errManager,
			wantErrIs:  errManager,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
			eft.ShardCtx.GetConfig().NumberOfShards = testNumShards
			require.Equal(t, ownShardID, eft.ShardCtx.GetShardID())
			// NewEngineForTest registers one-time start/stop expectations on the queue processors.
			eft.Engine.Start()
			defer eft.Engine.Stop()

			manager := persistence.NewMockAsyncWorkflowQueueManager(gomock.NewController(t))
			eft.ShardCtx.Resource.PersistenceBean.EXPECT().GetAsyncWorkflowQueueManager().Return(manager).AnyTimes()

			var persisted *persistence.EnqueueAsyncWorkflowMessageRequest
			if tc.wantPersisted != nil {
				manager.EXPECT().EnqueueAsyncWorkflowMessage(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, req *persistence.EnqueueAsyncWorkflowMessageRequest) error {
						persisted = req
						return tc.managerErr
					}).Times(1)
			}

			if tc.exhaustTaskIDs {
				eft.ShardCtx.Resource.ShardMgr.On("UpdateShard", mock.Anything, mock.Anything).Return(errUpdateShard)
				for i := 0; i < 1000000; i++ {
					if _, err := eft.ShardCtx.GenerateTaskID(); err != nil {
						break
					}
				}
			}

			resp, err := eft.Engine.EnqueueAsyncWorkflowMessage(context.Background(), tc.request)

			switch {
			case tc.wantBadRequest:
				var badRequest *types.BadRequestError
				require.ErrorAs(t, err, &badRequest)
				assert.Nil(t, resp)
			case tc.wantErrIs != nil:
				require.ErrorIs(t, err, tc.wantErrIs)
				assert.Nil(t, resp)
			default:
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Greater(t, resp.MessageID, int64(0), "message id must come from the shard task-id sequence")
			}

			if tc.wantPersisted == nil {
				return
			}
			require.NotNil(t, persisted, "queue manager must have been called")
			want := *tc.wantPersisted
			want.SourceCluster = eft.ShardCtx.GetClusterMetadata().GetCurrentClusterName()
			assert.NotEmpty(t, want.SourceCluster)
			assert.Greater(t, persisted.MessageID, int64(0))
			want.MessageID = persisted.MessageID
			assert.Equal(t, want, *persisted)
			if resp != nil {
				assert.Equal(t, persisted.MessageID, resp.MessageID, "returned message id must be the persisted one")
			}
		})
	}
}
