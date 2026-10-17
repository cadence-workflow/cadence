package api

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/validate"
)

const exportVersion = 1

type exportContinuationToken struct {
	BranchToken []byte                   `json:"branchToken"`
	MaxEventID  int64                    `json:"maxEventID"`
	ShardID     int                      `json:"shardID"`
	Execution   *types.WorkflowExecution `json:"execution"`
	DomainID    string                   `json:"domainID"`
	DomainName  string                   `json:"domainName"`
	// ClusterName is the cluster that serves the export. The branch token and the persistence
	// token are local to it.
	ClusterName string `json:"clusterName"`
	// PersistenceToken is the internal pagination token from ReadRawHistoryBranch
	PersistenceToken []byte `json:"persistenceToken"`
}

var errExportTokenFromOtherCluster = &types.BadRequestError{
	Message: "NextPageToken is from a different cluster. Start the export again without a NextPageToken.",
}

func (wh *WorkflowHandler) ExportWorkflowExecution(
	ctx context.Context,
	request *types.ExportWorkflowExecutionRequest,
) (*types.ExportWorkflowExecutionResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}

	domainName := request.GetDomain()
	if domainName == "" {
		return nil, validate.ErrDomainNotSet
	}
	if err := validate.CheckExecution(request.GetExecution()); err != nil {
		return nil, err
	}

	domainID, err := wh.GetDomainCache().GetDomainID(domainName)
	if err != nil {
		return nil, err
	}

	pageSize := int(request.GetPageSize())
	if pageSize <= 0 || pageSize > constants.GetHistoryMaxPageSize {
		pageSize = constants.GetHistoryMaxPageSize
	}

	var token *exportContinuationToken
	if len(request.GetNextPageToken()) > 0 {
		token = &exportContinuationToken{}
		if err := json.Unmarshal(request.GetNextPageToken(), token); err != nil {
			return nil, &types.BadRequestError{Message: "invalid NextPageToken"}
		}
		if token.DomainID != domainID {
			return nil, &types.BadRequestError{Message: "NextPageToken domain does not match request domain"}
		}
		if token.ClusterName != wh.GetClusterMetadata().GetCurrentClusterName() {
			return nil, errExportTokenFromOtherCluster
		}
	} else {
		token, err = wh.newExportContinuationToken(ctx, request, domainID)
		if err != nil {
			return nil, err
		}
	}

	rawResp, err := wh.GetHistoryManager().ReadRawHistoryBranch(ctx, &persistence.ReadHistoryBranchRequest{
		BranchToken:   token.BranchToken,
		MinEventID:    constants.FirstEventID,
		MaxEventID:    token.MaxEventID,
		PageSize:      pageSize,
		NextPageToken: token.PersistenceToken,
		ShardID:       common.IntPtr(token.ShardID),
		DomainName:    token.DomainName,
	})
	if err != nil {
		return nil, err
	}

	// The read returns whole batches, so the batch that has the last event can have later events.
	if count := len(rawResp.HistoryEventBlobs); count > 0 {
		cutBlob, err := cutHistoryBlob(rawResp.HistoryEventBlobs[count-1], token.MaxEventID)
		if err != nil {
			return nil, err
		}
		rawResp.HistoryEventBlobs[count-1] = cutBlob
	}

	blobs := make([]*types.DataBlob, 0, len(rawResp.HistoryEventBlobs))
	for _, blob := range rawResp.HistoryEventBlobs {
		b := blob.ToInternal()
		b.Checksum = make([]byte, 4)
		binary.LittleEndian.PutUint32(b.Checksum, crc32.ChecksumIEEE(b.Data))
		blobs = append(blobs, b)
	}

	var nextPageToken []byte
	if len(rawResp.NextPageToken) > 0 {
		token.PersistenceToken = rawResp.NextPageToken
		nextPageToken, err = json.Marshal(token)
		if err != nil {
			return nil, err
		}
	}

	return &types.ExportWorkflowExecutionResponse{
		Execution:      token.Execution,
		HistoryBatches: blobs,
		NextPageToken:  nextPageToken,
		Version:        exportVersion,
	}, nil
}

// newExportContinuationToken makes the token of the first page. It reads the current branch of the
// run and checks the requested last event.
func (wh *WorkflowHandler) newExportContinuationToken(
	ctx context.Context,
	request *types.ExportWorkflowExecutionRequest,
	domainID string,
) (*exportContinuationToken, error) {

	resp, err := wh.GetHistoryClient().GetMutableState(ctx, &types.GetMutableStateRequest{
		DomainUUID: domainID,
		Execution:  request.GetExecution(),
	})
	if err != nil {
		return nil, err
	}

	// A zombie lost a run conflict, so its history is not the history of the workflow.
	if resp.WorkflowState != nil && int(*resp.WorkflowState) == persistence.WorkflowStateZombie {
		return nil, &types.BadRequestError{Message: "Cannot export a zombie workflow run."}
	}

	versionHistories := persistence.NewVersionHistoriesFromInternalType(resp.GetVersionHistories())
	currentVersionHistory, err := versionHistories.GetCurrentVersionHistory()
	if err != nil {
		return nil, err
	}
	lastItem, err := currentVersionHistory.GetLastItem()
	if err != nil {
		return nil, err
	}

	execution := resp.Execution
	if execution == nil {
		execution = request.GetExecution()
	}
	shardID := common.WorkflowIDToHistoryShard(execution.GetWorkflowID(), wh.config.NumHistoryShards)

	maxEventID := lastItem.EventID + 1
	if request.LastEventID != nil {
		lastEventID := *request.LastEventID
		if lastEventID <= 0 || lastEventID > lastItem.EventID {
			return nil, &types.BadRequestError{Message: "LastEventID is out of range"}
		}

		events, err := wh.readExportLastEventPage(ctx, currentVersionHistory.GetBranchToken(), lastEventID, shardID, request.GetDomain())
		if err != nil {
			return nil, err
		}
		if err := checkExportLastEvent(events, lastEventID); err != nil {
			return nil, err
		}
		maxEventID = lastEventID + 1
	}

	return &exportContinuationToken{
		BranchToken: currentVersionHistory.GetBranchToken(),
		MaxEventID:  maxEventID,
		ShardID:     shardID,
		Execution:   execution,
		DomainID:    domainID,
		DomainName:  request.GetDomain(),
		ClusterName: wh.GetClusterMetadata().GetCurrentClusterName(),
	}, nil
}

// readExportLastEventPage returns the last page of events up to lastEventID. History nodes are
// stored by the ID of the first event of the batch, so a read that starts at lastEventID skips the
// batch when lastEventID is not its first event. The read starts at the first event instead.
func (wh *WorkflowHandler) readExportLastEventPage(
	ctx context.Context,
	branchToken []byte,
	lastEventID int64,
	shardID int,
	domainName string,
) ([]*types.HistoryEvent, error) {
	var events []*types.HistoryEvent
	var pageToken []byte
	for {
		resp, err := wh.GetHistoryManager().ReadHistoryBranch(ctx, &persistence.ReadHistoryBranchRequest{
			BranchToken:   branchToken,
			MinEventID:    constants.FirstEventID,
			MaxEventID:    lastEventID + 1,
			PageSize:      constants.GetHistoryMaxPageSize,
			NextPageToken: pageToken,
			ShardID:       common.IntPtr(shardID),
			DomainName:    domainName,
		})
		if err != nil {
			return nil, err
		}
		if len(resp.HistoryEvents) > 0 {
			events = resp.HistoryEvents
		}
		if len(resp.NextPageToken) == 0 {
			return events, nil
		}
		pageToken = resp.NextPageToken
	}
}

// checkExportLastEvent checks that the last event of a cut history ends a decision task, so the
// imported workflow can schedule its next decision. A cut after DecisionTaskCompleted drops the
// commands of the decision, and the replay of that decision then fails on every decision task.
func checkExportLastEvent(events []*types.HistoryEvent, lastEventID int64) error {
	for _, event := range events {
		if event.ID != lastEventID {
			continue
		}
		switch event.GetEventType() {
		case types.EventTypeDecisionTaskStarted,
			types.EventTypeDecisionTaskTimedOut,
			types.EventTypeDecisionTaskFailed:
			return nil
		case types.EventTypeDecisionTaskCompleted:
			return &types.BadRequestError{Message: fmt.Sprintf(
				"LastEventID cannot be a DecisionTaskCompleted event: use the DecisionTaskStarted event ID %d", lastEventID-1,
			)}
		default:
			return &types.BadRequestError{Message: fmt.Sprintf(
				"LastEventID %d must be a DecisionTaskStarted, DecisionTaskTimedOut or DecisionTaskFailed event, got: %v",
				lastEventID, event.GetEventType(),
			)}
		}
	}
	return &types.BadRequestError{Message: fmt.Sprintf("LastEventID %d not found in the history", lastEventID)}
}

// cutHistoryBlob removes the events with an ID >= maxEventID from the batch. The batch is written
// again with the same encoding. A batch with no such events is returned as it is.
func cutHistoryBlob(blob *persistence.DataBlob, maxEventID int64) (*persistence.DataBlob, error) {
	serializer := persistence.NewPayloadSerializer()
	events, err := serializer.DeserializeBatchEvents(blob)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 || events[len(events)-1].ID < maxEventID {
		return blob, nil
	}
	kept := events[:0]
	for _, event := range events {
		if event.ID < maxEventID {
			kept = append(kept, event)
		}
	}
	return serializer.SerializeBatchEvents(kept, blob.Encoding)
}
