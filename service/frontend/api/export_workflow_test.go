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
		{ID: 5, EventType: types.EventTypeDecisionTaskFailed.Ptr()},
	}

	tests := map[string]struct {
		state              int
		lastEventID        *int64
		batches            [][]*types.HistoryEvent
		noRead             bool
		persistenceToken   []byte
		wantMaxEventID     int64
		wantBatchIDs       [][]int64
		wantNextPageToken  bool
		wantCheckLastEvent bool
		wantErr            string
	}{
		"whole history in one page": {
			wantMaxEventID: 6,
			wantBatchIDs:   [][]int64{{1, 2}, {3, 4, 5}},
		},
		"history with more pages returns a next page token": {
			persistenceToken:  []byte("persistence-token"),
			wantMaxEventID:    6,
			wantBatchIDs:      [][]int64{{1, 2}, {3, 4, 5}},
			wantNextPageToken: true,
		},
		"history cut at a DecisionTaskStarted event": {
			lastEventID:    common.Int64Ptr(3),
			wantMaxEventID: 4,
			wantBatchIDs:   [][]int64{{1, 2}, {3}},
		},
		"history cut at an event in the middle of a batch": {
			lastEventID:    common.Int64Ptr(5),
			wantMaxEventID: 6,
			wantBatchIDs:   [][]int64{{1, 2}, {3, 4, 5}},
		},
		"last event on a later page is checked on that page": {
			lastEventID:        common.Int64Ptr(5),
			batches:            [][]*types.HistoryEvent{firstBatch},
			persistenceToken:   []byte("persistence-token"),
			wantMaxEventID:     6,
			wantBatchIDs:       [][]int64{{1, 2}},
			wantNextPageToken:  true,
			wantCheckLastEvent: true,
		},
		"last event checked on this page is not checked again": {
			lastEventID:       common.Int64Ptr(3),
			persistenceToken:  []byte("persistence-token"),
			wantMaxEventID:    4,
			wantBatchIDs:      [][]int64{{1, 2}, {3}},
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
		"cut at a DecisionTaskCompleted event is refused": {
			lastEventID:    common.Int64Ptr(4),
			wantMaxEventID: 5,
			wantErr:        "LastEventID cannot be a DecisionTaskCompleted event: use the DecisionTaskStarted event ID 3",
		},
		"cut at an event that does not end a decision task is refused": {
			lastEventID:    common.Int64Ptr(2),
			batches:        [][]*types.HistoryEvent{firstBatch},
			wantMaxEventID: 3,
			wantErr:        "LastEventID 2 must be a DecisionTaskStarted, DecisionTaskTimedOut or DecisionTaskFailed event",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			shardID := common.WorkflowIDToHistoryShard(execution.WorkflowID, wh.config.NumHistoryShards)
			deps.mockDomainCache.EXPECT().GetDomainID(exportTestDomain).Return(exportTestDomainID, nil)
			state := int32(persistence.WorkflowStateRunning)
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
				CheckLastEvent:   tc.wantCheckLastEvent,
			}, token)
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
		checkLastEvent    bool
		emptyPage         bool
		wantIDs           []int64
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
		"last event is checked on the page that has it": {
			domainID:       exportTestDomainID,
			checkLastEvent: true,
			wantIDs:        []int64{6, 7},
		},
		"last event of a wrong type is refused on the page that has it": {
			domainID:       exportTestDomainID,
			maxEventID:     7,
			checkLastEvent: true,
			wantErr:        "LastEventID 6 must be a DecisionTaskStarted, DecisionTaskTimedOut or DecisionTaskFailed event, got: DecisionTaskScheduled",
		},
		"empty last page after the check is served": {
			domainID:  exportTestDomainID,
			emptyPage: true,
		},
		"last event not found by the last page is refused": {
			domainID:       exportTestDomainID,
			checkLastEvent: true,
			emptyPage:      true,
			wantErr:        "LastEventID 7 not found in the history",
		},
		"token for another workflow is refused": {
			domainID:          exportTestDomainID,
			requestWorkflowID: "wf-2",
			wantErr:           "NextPageToken workflow does not match request workflow",
		},
		"token for another run is refused": {
			domainID:     exportTestDomainID,
			requestRunID: uuid.NewString(),
			wantErr:      validate.ErrNextPageTokenRunIDMismatch.Message,
		},
		"invalid token is refused": {
			invalid: true,
			wantErr: "invalid NextPageToken",
		},
		"token from another domain is refused": {
			domainID: "other-domain-id",
			wantErr:  "NextPageToken domain does not match request domain",
		},
		"token from another cluster is refused": {
			domainID:     exportTestDomainID,
			otherCluster: true,
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
				CheckLastEvent:   tc.checkLastEvent,
			}
			nextPageToken, err := json.Marshal(token)
			require.NoError(t, err)
			if tc.invalid {
				nextPageToken = []byte("not json")
			}
			if tc.wantIDs != nil || tc.emptyPage || tc.checkLastEvent {
				var blobs []*persistence.DataBlob
				if !tc.emptyPage {
					blobs = []*persistence.DataBlob{serializeExportTestBatch(t, batch)}
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
			assert.Empty(t, resp.NextPageToken)
		})
	}
}

func TestCheckExportLastEvent(t *testing.T) {
	events := []*types.HistoryEvent{
		{ID: 3, EventType: types.EventTypeDecisionTaskStarted.Ptr()},
		{ID: 4, EventType: types.EventTypeDecisionTaskTimedOut.Ptr()},
		{ID: 5, EventType: types.EventTypeDecisionTaskFailed.Ptr()},
		{ID: 6, EventType: types.EventTypeDecisionTaskCompleted.Ptr()},
		{ID: 7, EventType: types.EventTypeTimerStarted.Ptr()},
	}

	tests := map[string]struct {
		lastEventID int64
		wantErr     string
	}{
		"DecisionTaskStarted":   {lastEventID: 3},
		"DecisionTaskTimedOut":  {lastEventID: 4},
		"DecisionTaskFailed":    {lastEventID: 5},
		"DecisionTaskCompleted": {lastEventID: 6, wantErr: "LastEventID cannot be a DecisionTaskCompleted event: use the DecisionTaskStarted event ID 5"},
		"other event":           {lastEventID: 7, wantErr: "LastEventID 7 must be a DecisionTaskStarted, DecisionTaskTimedOut or DecisionTaskFailed event, got: TimerStarted"},
		"missing event":         {lastEventID: 8, wantErr: "LastEventID 8 not found in the history"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := checkExportLastEvent(events, tc.lastEventID)
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

func TestCutHistoryBlob(t *testing.T) {
	events := []*types.HistoryEvent{
		{ID: 5, EventType: types.EventTypeDecisionTaskScheduled.Ptr()},
		{ID: 6, EventType: types.EventTypeDecisionTaskStarted.Ptr()},
		{ID: 7, EventType: types.EventTypeDecisionTaskCompleted.Ptr()},
	}

	tests := map[string]struct {
		maxEventID  int64
		wantIDs     []int64
		wantSameRef bool
	}{
		"batch before the max event ID is not changed": {
			maxEventID:  8,
			wantIDs:     []int64{5, 6, 7},
			wantSameRef: true,
		},
		"events at and after the max event ID are removed": {
			maxEventID: 7,
			wantIDs:    []int64{5, 6},
		},
		"cut after the first event": {
			maxEventID: 6,
			wantIDs:    []int64{5},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			blob := serializeExportTestBatch(t, events)

			cutBlob, keptEvents, err := cutHistoryBlob(blob, tc.maxEventID)
			require.NoError(t, err)
			if tc.wantSameRef {
				assert.Same(t, blob, cutBlob)
			}
			assert.Equal(t, blob.Encoding, cutBlob.Encoding)
			assert.Equal(t, tc.wantIDs, exportTestBatchIDs(t, cutBlob.ToInternal()))
			keptIDs := make([]int64, 0, len(keptEvents))
			for _, event := range keptEvents {
				keptIDs = append(keptIDs, event.ID)
			}
			assert.Equal(t, tc.wantIDs, keptIDs)
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
