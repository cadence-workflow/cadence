package api

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/validate"
)

const (
	exportTestDomain   = "test-domain"
	exportTestDomainID = "domain-id"
)

func TestExportWorkflowExecution_Validation(t *testing.T) {
	tests := map[string]struct {
		req     *types.ExportWorkflowExecutionRequest
		wantErr string
	}{
		"nil request": {
			wantErr: "Request is nil",
		},
		"empty domain": {
			req:     &types.ExportWorkflowExecutionRequest{},
			wantErr: "Domain not set",
		},
		"missing workflow ID": {
			req: &types.ExportWorkflowExecutionRequest{
				Domain:    exportTestDomain,
				Execution: &types.WorkflowExecution{},
			},
			wantErr: "WorkflowId is not set",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			wh, _ := setupMocksForWorkflowHandler(t)
			_, err := wh.ExportWorkflowExecution(context.Background(), tc.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestExportWorkflowExecution_FirstPage(t *testing.T) {
	execution := &types.WorkflowExecution{WorkflowID: "wf-1", RunID: uuid.NewString()}
	branchToken := []byte("branch-token")
	firstBatch := []*types.HistoryEvent{
		{ID: 1, EventType: types.EventTypeWorkflowExecutionStarted.Ptr()},
		{ID: 2, EventType: types.EventTypeDecisionTaskScheduled.Ptr()},
	}
	lastBatch := []*types.HistoryEvent{
		{ID: 3, EventType: types.EventTypeDecisionTaskStarted.Ptr()},
		{ID: 4, EventType: types.EventTypeDecisionTaskCompleted.Ptr()},
		{ID: 5, EventType: types.EventTypeWorkflowExecutionCompleted.Ptr()},
	}

	tests := map[string]struct {
		state             int
		lastEventID       *int64
		batches           [][]*types.HistoryEvent
		noRead            bool
		persistenceToken  []byte
		wantMaxEventID    int64
		wantBatchIDs      [][]int64
		wantNextPageToken bool
		wantErr           string
	}{
		"whole history in one page": {
			wantMaxEventID: 6,
			wantBatchIDs:   [][]int64{{1, 2}, {3, 4, 5}},
		},
		"history with more pages returns a next page token": {
			batches:           [][]*types.HistoryEvent{firstBatch},
			persistenceToken:  []byte("persistence-token"),
			wantMaxEventID:    6,
			wantBatchIDs:      [][]int64{{1, 2}},
			wantNextPageToken: true,
		},
		"page that reaches the last event ends the export": {
			persistenceToken: []byte("persistence-token"),
			wantMaxEventID:   6,
			wantBatchIDs:     [][]int64{{1, 2}, {3, 4, 5}},
		},
		"history cut at the last event of a batch": {
			lastEventID:    common.Int64Ptr(5),
			wantMaxEventID: 6,
			wantBatchIDs:   [][]int64{{1, 2}, {3, 4, 5}},
		},
		"cut at an event in the middle of a batch is refused": {
			lastEventID:    common.Int64Ptr(3),
			wantMaxEventID: 4,
			wantErr:        "LastEventID 3 must be the last event of a history batch, its batch ends at event 5",
		},
		"last event on a later page is checked on that page": {
			lastEventID:       common.Int64Ptr(5),
			batches:           [][]*types.HistoryEvent{firstBatch},
			persistenceToken:  []byte("persistence-token"),
			wantMaxEventID:    6,
			wantBatchIDs:      [][]int64{{1, 2}},
			wantNextPageToken: true,
		},
		"zombie run is refused": {
			state:   persistence.WorkflowStateZombie,
			noRead:  true,
			wantErr: "Cannot export a zombie workflow run.",
		},
		"last event ID of zero is refused": {
			lastEventID: common.Int64Ptr(0),
			noRead:      true,
			wantErr:     "LastEventID is out of range",
		},
		"last event ID after the last event is refused": {
			lastEventID: common.Int64Ptr(6),
			noRead:      true,
			wantErr:     "LastEventID is out of range",
		},
		"cut at an event that does not end a decision task is refused": {
			lastEventID:    common.Int64Ptr(2),
			batches:        [][]*types.HistoryEvent{firstBatch},
			wantMaxEventID: 3,
			wantErr:        "LastEventID 2 must be a DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed or workflow close event",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			shardID := common.WorkflowIDToHistoryShard(execution.WorkflowID, wh.config.NumHistoryShards)
			deps.mockDomainCache.EXPECT().GetDomainID(exportTestDomain).Return(exportTestDomainID, nil)
			state := int32(persistence.WorkflowStateCompleted)
			if tc.state != 0 {
				state = int32(tc.state)
			}
			deps.mockHistoryClient.EXPECT().GetMutableState(gomock.Any(), &types.GetMutableStateRequest{
				DomainUUID: exportTestDomainID,
				Execution:  execution,
			}).Return(&types.GetMutableStateResponse{
				Execution:     execution,
				WorkflowState: &state,
				VersionHistories: &types.VersionHistories{
					Histories: []*types.VersionHistory{{
						BranchToken: branchToken,
						Items:       []*types.VersionHistoryItem{{EventID: 5, Version: 1}},
					}},
				},
			}, nil)
			if !tc.noRead {
				batches := tc.batches
				if batches == nil {
					batches = [][]*types.HistoryEvent{firstBatch, lastBatch}
				}
				blobs := make([]*persistence.DataBlob, 0, len(batches))
				for _, batch := range batches {
					blobs = append(blobs, serializeExportTestBatch(t, batch))
				}
				deps.mockHistoryV2Mgr.On("ReadRawHistoryBranch", mock.Anything, &persistence.ReadHistoryBranchRequest{
					BranchToken: branchToken,
					MinEventID:  constants.FirstEventID,
					MaxEventID:  tc.wantMaxEventID,
					PageSize:    constants.GetHistoryMaxPageSize,
					ShardID:     common.IntPtr(shardID),
					DomainName:  exportTestDomain,
				}).Return(&persistence.ReadRawHistoryBranchResponse{
					HistoryEventBlobs: blobs,
					NextPageToken:     tc.persistenceToken,
				}, nil).Once()
			}

			resp, err := wh.ExportWorkflowExecution(context.Background(), &types.ExportWorkflowExecutionRequest{
				Domain:      exportTestDomain,
				Execution:   execution,
				LastEventID: tc.lastEventID,
			})
			if tc.wantErr != "" {
				var badRequestErr *types.BadRequestError
				require.ErrorAs(t, err, &badRequestErr)
				assert.Contains(t, badRequestErr.Message, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, execution, resp.Execution)
			assert.Equal(t, int32(exportVersion), resp.Version)
			require.Len(t, resp.HistoryBatches, len(tc.wantBatchIDs))
			for i, blob := range resp.HistoryBatches {
				assert.Equal(t, tc.wantBatchIDs[i], exportTestBatchIDs(t, blob))
				assert.Equal(t, crc32.ChecksumIEEE(blob.Data), binary.LittleEndian.Uint32(blob.Checksum))
			}

			if !tc.wantNextPageToken {
				assert.Empty(t, resp.NextPageToken)
				return
			}
			var token exportContinuationToken
			require.NoError(t, json.Unmarshal(resp.NextPageToken, &token))
			assert.Equal(t, exportContinuationToken{
				BranchToken:      branchToken,
				MaxEventID:       tc.wantMaxEventID,
				ShardID:          shardID,
				Execution:        execution,
				DomainID:         exportTestDomainID,
				DomainName:       exportTestDomain,
				ClusterName:      wh.GetClusterMetadata().GetCurrentClusterName(),
				PersistenceToken: tc.persistenceToken,
			}, token)
		})
	}
}

func TestExportWorkflowExecution_RunningWorkflow(t *testing.T) {
	execution := &types.WorkflowExecution{WorkflowID: "wf-1", RunID: uuid.NewString()}
	branchToken := []byte("branch-token")
	batches := [][]*types.HistoryEvent{
		{{ID: 1, EventType: types.EventTypeWorkflowExecutionStarted.Ptr()}, {ID: 2, EventType: types.EventTypeDecisionTaskScheduled.Ptr()}},
		{{ID: 3, EventType: types.EventTypeDecisionTaskStarted.Ptr()}},
		{{ID: 4, EventType: types.EventTypeDecisionTaskCompleted.Ptr()}, {ID: 5, EventType: types.EventTypeTimerStarted.Ptr()}},
		{{ID: 6, EventType: types.EventTypeTimerFired.Ptr()}, {ID: 7, EventType: types.EventTypeDecisionTaskScheduled.Ptr()}},
		{{ID: 8, EventType: types.EventTypeDecisionTaskStarted.Ptr()}},
		{{ID: 9, EventType: types.EventTypeDecisionTaskCompleted.Ptr()}, {ID: 10, EventType: types.EventTypeActivityTaskScheduled.Ptr()}},
	}

	tests := map[string]struct {
		previousStartedEventID int64
		tailBatches            [][]*types.HistoryEvent
		tailErr                error
		fullBatches            [][]*types.HistoryEvent
		wantErr                string
	}{
		"empty tail reads the whole history": {
			previousStartedEventID: 8,
			tailErr:                &types.EntityNotExistsError{Message: "Workflow execution history not found."},
			fullBatches:            batches,
		},
		"tail read error is returned": {
			previousStartedEventID: 8,
			tailErr:                &types.BadRequestError{Message: "tail read failed"},
			wantErr:                "tail read failed",
		},
		"export stops at the stop event found in the tail": {
			previousStartedEventID: 8,
			tailBatches:            batches[4:],
		},
		"tail with no stop event reads the whole history": {
			previousStartedEventID: 9,
			tailBatches:            batches[5:],
			fullBatches:            batches,
		},
		"export stops at the stop event found in a tail that starts after the last completed decision": {
			previousStartedEventID: 7,
			tailBatches:            batches[4:],
		},
		"workflow with no completed decision reads the whole history": {
			fullBatches: batches,
		},
		"workflow with no stop event is refused": {
			fullBatches: batches[:1],
			wantErr:     "The running workflow has no event that the export can stop at.",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			shardID := common.WorkflowIDToHistoryShard(execution.WorkflowID, wh.config.NumHistoryShards)
			deps.mockDomainCache.EXPECT().GetDomainID(exportTestDomain).Return(exportTestDomainID, nil)
			state := int32(persistence.WorkflowStateRunning)
			var previousStartedEventID *int64
			if tc.previousStartedEventID != 0 {
				previousStartedEventID = common.Int64Ptr(tc.previousStartedEventID)
			}
			deps.mockHistoryClient.EXPECT().GetMutableState(gomock.Any(), &types.GetMutableStateRequest{
				DomainUUID: exportTestDomainID,
				Execution:  execution,
			}).Return(&types.GetMutableStateResponse{
				Execution:              execution,
				WorkflowState:          &state,
				IsWorkflowRunning:      true,
				PreviousStartedEventID: previousStartedEventID,
				VersionHistories: &types.VersionHistories{
					Histories: []*types.VersionHistory{{
						BranchToken: branchToken,
						Items:       []*types.VersionHistoryItem{{EventID: 10, Version: 1}},
					}},
				},
			}, nil)
			expectRead := func(minEventID, maxEventID int64, readBatches [][]*types.HistoryEvent) {
				blobs := make([]*persistence.DataBlob, 0, len(readBatches))
				for _, batch := range readBatches {
					blobs = append(blobs, serializeExportTestBatch(t, batch))
				}
				deps.mockHistoryV2Mgr.On("ReadRawHistoryBranch", mock.Anything, &persistence.ReadHistoryBranchRequest{
					BranchToken: branchToken,
					MinEventID:  minEventID,
					MaxEventID:  maxEventID,
					PageSize:    constants.GetHistoryMaxPageSize,
					ShardID:     common.IntPtr(shardID),
					DomainName:  exportTestDomain,
				}).Return(&persistence.ReadRawHistoryBranchResponse{HistoryEventBlobs: blobs}, nil).Once()
			}
			if tc.tailErr != nil {
				deps.mockHistoryV2Mgr.On("ReadRawHistoryBranch", mock.Anything, &persistence.ReadHistoryBranchRequest{
					BranchToken: branchToken,
					MinEventID:  tc.previousStartedEventID,
					MaxEventID:  11,
					PageSize:    constants.GetHistoryMaxPageSize,
					ShardID:     common.IntPtr(shardID),
					DomainName:  exportTestDomain,
				}).Return(nil, tc.tailErr).Once()
			}
			if tc.tailBatches != nil {
				expectRead(tc.previousStartedEventID, 11, tc.tailBatches)
			}
			if tc.fullBatches != nil {
				expectRead(constants.FirstEventID, 11, tc.fullBatches)
			}
			if tc.wantErr == "" {
				expectRead(constants.FirstEventID, 9, batches[:5])
			}

			resp, err := wh.ExportWorkflowExecution(context.Background(), &types.ExportWorkflowExecutionRequest{
				Domain:    exportTestDomain,
				Execution: execution,
			})
			if tc.wantErr != "" {
				var badRequestErr *types.BadRequestError
				require.ErrorAs(t, err, &badRequestErr)
				assert.Equal(t, tc.wantErr, badRequestErr.Message)
				return
			}
			require.NoError(t, err)
			var ids []int64
			for _, blob := range resp.HistoryBatches {
				ids = append(ids, exportTestBatchIDs(t, blob)...)
			}
			assert.Equal(t, []int64{1, 2, 3, 4, 5, 6, 7, 8}, ids)
			assert.Empty(t, resp.NextPageToken)
		})
	}
}

func TestExportWorkflowExecution_NextPage(t *testing.T) {
	execution := &types.WorkflowExecution{WorkflowID: "wf-1", RunID: uuid.NewString()}
	batch := []*types.HistoryEvent{
		{ID: 6, EventType: types.EventTypeDecisionTaskScheduled.Ptr()},
		{ID: 7, EventType: types.EventTypeDecisionTaskStarted.Ptr()},
	}

	tests := map[string]struct {
		domainID          string
		otherCluster      bool
		invalid           bool
		requestWorkflowID string
		requestRunID      string
		maxEventID        int64
		emptyPage         bool
		morePages         bool
		noRead            bool
		wantIDs           []int64
		wantNextPageToken bool
		wantErr           string
	}{
		"next page is read with the token": {
			domainID: exportTestDomainID,
			wantIDs:  []int64{6, 7},
		},
		"next page is read when the request has the run ID of the token": {
			domainID:     exportTestDomainID,
			requestRunID: execution.RunID,
			wantIDs:      []int64{6, 7},
		},
		"page that reaches the last event ends the export": {
			domainID:  exportTestDomainID,
			morePages: true,
			wantIDs:   []int64{6, 7},
		},
		"page before the last event returns a next page token": {
			domainID:          exportTestDomainID,
			maxEventID:        10,
			morePages:         true,
			wantIDs:           []int64{6, 7},
			wantNextPageToken: true,
		},
		"last event in the middle of a batch is refused on the page that has it": {
			domainID:   exportTestDomainID,
			maxEventID: 7,
			wantErr:    "LastEventID 6 must be the last event of a history batch, its batch ends at event 7",
		},
		"last event not found by the last page is refused": {
			domainID:   exportTestDomainID,
			maxEventID: 10,
			wantErr:    "LastEventID 9 not found in the history",
		},
		"empty last page is refused": {
			domainID:  exportTestDomainID,
			emptyPage: true,
			wantErr:   "LastEventID 7 not found in the history",
		},
		"token for another workflow is refused": {
			domainID:          exportTestDomainID,
			requestWorkflowID: "wf-2",
			noRead:            true,
			wantErr:           "NextPageToken workflow does not match request workflow",
		},
		"token for another run is refused": {
			domainID:     exportTestDomainID,
			requestRunID: uuid.NewString(),
			noRead:       true,
			wantErr:      validate.ErrNextPageTokenRunIDMismatch.Message,
		},
		"invalid token is refused": {
			invalid: true,
			noRead:  true,
			wantErr: "invalid NextPageToken",
		},
		"token from another domain is refused": {
			domainID: "other-domain-id",
			noRead:   true,
			wantErr:  "NextPageToken domain does not match request domain",
		},
		"token from another cluster is refused": {
			domainID:     exportTestDomainID,
			otherCluster: true,
			noRead:       true,
			wantErr:      errExportTokenFromOtherCluster.Message,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			deps.mockDomainCache.EXPECT().GetDomainID(exportTestDomain).Return(exportTestDomainID, nil)

			clusterName := wh.GetClusterMetadata().GetCurrentClusterName()
			if tc.otherCluster {
				clusterName = "other-cluster"
			}
			maxEventID := int64(8)
			if tc.maxEventID != 0 {
				maxEventID = tc.maxEventID
			}
			token := exportContinuationToken{
				BranchToken:      []byte("branch-token"),
				MaxEventID:       maxEventID,
				ShardID:          3,
				Execution:        execution,
				DomainID:         tc.domainID,
				DomainName:       exportTestDomain,
				ClusterName:      clusterName,
				PersistenceToken: []byte("persistence-token"),
			}
			nextPageToken, err := json.Marshal(token)
			require.NoError(t, err)
			if tc.invalid {
				nextPageToken = []byte("not json")
			}
			if !tc.noRead {
				var blobs []*persistence.DataBlob
				if !tc.emptyPage {
					blobs = []*persistence.DataBlob{serializeExportTestBatch(t, batch)}
				}
				var persistenceNextPageToken []byte
				if tc.morePages {
					persistenceNextPageToken = []byte("next-persistence-token")
				}
				deps.mockHistoryV2Mgr.On("ReadRawHistoryBranch", mock.Anything, &persistence.ReadHistoryBranchRequest{
					BranchToken:   token.BranchToken,
					MinEventID:    constants.FirstEventID,
					MaxEventID:    token.MaxEventID,
					PageSize:      10,
					NextPageToken: token.PersistenceToken,
					ShardID:       common.IntPtr(token.ShardID),
					DomainName:    exportTestDomain,
				}).Return(&persistence.ReadRawHistoryBranchResponse{
					HistoryEventBlobs: blobs,
					NextPageToken:     persistenceNextPageToken,
				}, nil).Once()
			}

			requestWorkflowID := execution.WorkflowID
			if tc.requestWorkflowID != "" {
				requestWorkflowID = tc.requestWorkflowID
			}
			resp, err := wh.ExportWorkflowExecution(context.Background(), &types.ExportWorkflowExecutionRequest{
				Domain:        exportTestDomain,
				Execution:     &types.WorkflowExecution{WorkflowID: requestWorkflowID, RunID: tc.requestRunID},
				PageSize:      10,
				NextPageToken: nextPageToken,
			})
			if tc.wantErr != "" {
				var badRequestErr *types.BadRequestError
				require.ErrorAs(t, err, &badRequestErr)
				assert.Equal(t, tc.wantErr, badRequestErr.Message)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, execution, resp.Execution)
			var ids []int64
			for _, blob := range resp.HistoryBatches {
				ids = append(ids, exportTestBatchIDs(t, blob)...)
			}
			assert.Equal(t, tc.wantIDs, ids)
			if tc.wantNextPageToken {
				assert.NotEmpty(t, resp.NextPageToken)
			} else {
				assert.Empty(t, resp.NextPageToken)
			}
		})
	}
}

func TestCheckExportLastEvent(t *testing.T) {
	tests := map[string]struct {
		event   *types.HistoryEvent
		wantErr string
	}{
		"DecisionTaskStarted":  {event: &types.HistoryEvent{ID: 3, EventType: types.EventTypeDecisionTaskStarted.Ptr()}},
		"DecisionTaskTimedOut": {event: &types.HistoryEvent{ID: 4, EventType: types.EventTypeDecisionTaskTimedOut.Ptr()}},
		"DecisionTaskFailed":   {event: &types.HistoryEvent{ID: 5, EventType: types.EventTypeDecisionTaskFailed.Ptr()}},
		"WorkflowExecutionCompleted": {
			event: &types.HistoryEvent{ID: 8, EventType: types.EventTypeWorkflowExecutionCompleted.Ptr()},
		},
		"WorkflowExecutionFailed": {
			event: &types.HistoryEvent{ID: 8, EventType: types.EventTypeWorkflowExecutionFailed.Ptr()},
		},
		"WorkflowExecutionTimedOut": {
			event: &types.HistoryEvent{ID: 8, EventType: types.EventTypeWorkflowExecutionTimedOut.Ptr()},
		},
		"WorkflowExecutionCanceled": {
			event: &types.HistoryEvent{ID: 8, EventType: types.EventTypeWorkflowExecutionCanceled.Ptr()},
		},
		"WorkflowExecutionTerminated": {
			event: &types.HistoryEvent{ID: 8, EventType: types.EventTypeWorkflowExecutionTerminated.Ptr()},
		},
		"WorkflowExecutionContinuedAsNew": {
			event: &types.HistoryEvent{ID: 8, EventType: types.EventTypeWorkflowExecutionContinuedAsNew.Ptr()},
		},
		"DecisionTaskCompleted": {
			event:   &types.HistoryEvent{ID: 6, EventType: types.EventTypeDecisionTaskCompleted.Ptr()},
			wantErr: "LastEventID cannot be a DecisionTaskCompleted event: use the DecisionTaskStarted event ID 5",
		},
		"other event": {
			event:   &types.HistoryEvent{ID: 7, EventType: types.EventTypeTimerStarted.Ptr()},
			wantErr: "LastEventID 7 must be a DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed or workflow close event, got: TimerStarted",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := checkExportLastEvent(tc.event)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			var badRequestErr *types.BadRequestError
			require.ErrorAs(t, err, &badRequestErr)
			assert.Equal(t, tc.wantErr, badRequestErr.Message)
		})
	}
}

func TestCheckExportLastBatch(t *testing.T) {
	events := []*types.HistoryEvent{
		{ID: 5, EventType: types.EventTypeDecisionTaskScheduled.Ptr()},
		{ID: 6, EventType: types.EventTypeDecisionTaskStarted.Ptr()},
	}

	tests := map[string]struct {
		emptyPage   bool
		lastEventID int64
		wantFound   bool
		wantErr     string
	}{
		"batch before the last event": {
			lastEventID: 7,
		},
		"batch ends at the last event": {
			lastEventID: 6,
			wantFound:   true,
		},
		"last event in the middle of the batch is refused": {
			lastEventID: 5,
			wantErr:     "LastEventID 5 must be the last event of a history batch, its batch ends at event 6",
		},
		"empty page": {
			emptyPage:   true,
			lastEventID: 6,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var blobs []*persistence.DataBlob
			if !tc.emptyPage {
				blobs = []*persistence.DataBlob{serializeExportTestBatch(t, events)}
			}
			found, err := checkExportLastBatch(blobs, tc.lastEventID)
			if tc.wantErr != "" {
				var badRequestErr *types.BadRequestError
				require.ErrorAs(t, err, &badRequestErr)
				assert.Equal(t, tc.wantErr, badRequestErr.Message)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantFound, found)
		})
	}
}

func serializeExportTestBatch(t *testing.T, events []*types.HistoryEvent) *persistence.DataBlob {
	blob, err := persistence.NewPayloadSerializer().SerializeBatchEvents(events, constants.EncodingTypeThriftRW)
	require.NoError(t, err)
	return blob
}

func exportTestBatchIDs(t *testing.T, blob *types.DataBlob) []int64 {
	events, err := persistence.NewPayloadSerializer().DeserializeBatchEvents(persistence.NewDataBlobFromInternal(blob))
	require.NoError(t, err)
	ids := make([]int64, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return ids
}
