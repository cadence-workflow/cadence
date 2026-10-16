package api

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
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
	if err := validate.CheckExecution(request.GetWorkflowExecution()); err != nil {
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
	if len(request.GetNextPageToken()) == 0 {
		// First page: there is no token yet, so make a new one.
		token, err = wh.newExportContinuationToken(ctx, request, domainID)
		if err != nil {
			return nil, err
		}
	} else {
		// Next page: check that the token belongs to this request.
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
		if token.Execution.GetWorkflowID() != request.GetWorkflowExecution().GetWorkflowID() {
			return nil, &types.BadRequestError{Message: "NextPageToken workflow does not match request workflow"}
		}
		if request.GetWorkflowExecution().GetRunID() != "" && request.GetWorkflowExecution().GetRunID() != token.Execution.GetRunID() {
			return nil, validate.ErrNextPageTokenRunIDMismatch
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
	lastPage := len(rawResp.NextPageToken) == 0

	// Check if the last batch of the page has the last event requested for the export. If it has it,
	// complete the export.
	found, err := checkExportLastBatch(rawResp.HistoryEventBlobs, token.MaxEventID-1)
	if err != nil {
		return nil, err
	}
	if lastPage && !found {
		return nil, &types.BadRequestError{Message: fmt.Sprintf("LastEventID %d not found in the history", token.MaxEventID-1)}
	}
	lastPage = lastPage || found

	blobs := make([]*types.DataBlob, 0, len(rawResp.HistoryEventBlobs))
	for _, blob := range rawResp.HistoryEventBlobs {
		b := blob.ToInternal()
		b.Checksum = make([]byte, 4)
		binary.LittleEndian.PutUint32(b.Checksum, crc32.ChecksumIEEE(b.Data))
		blobs = append(blobs, b)
	}

	var nextPageToken []byte
	if !lastPage {
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
// run, and checks that the requested last event is in range or chooses the last event of a running
// workflow.
func (wh *WorkflowHandler) newExportContinuationToken(
	ctx context.Context,
	request *types.ExportWorkflowExecutionRequest,
	domainID string,
) (*exportContinuationToken, error) {

	resp, err := wh.GetHistoryClient().GetMutableState(ctx, &types.GetMutableStateRequest{
		DomainUUID: domainID,
		Execution:  request.GetWorkflowExecution(),
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
	shardID := common.WorkflowIDToHistoryShard(execution.GetWorkflowID(), wh.config.NumHistoryShards)

	maxEventID := lastItem.EventID + 1
	if request.LastEventID != nil {
		lastEventID := *request.LastEventID
		if lastEventID <= 0 || lastEventID > lastItem.EventID {
			return nil, &types.BadRequestError{Message: "LastEventID is out of range"}
		}
		maxEventID = lastEventID + 1
	} else if resp.GetIsWorkflowRunning() {
		// A running workflow is exported up to its last event that an export can stop at, so the
		// imported workflow can schedule its next decision.
		lastEventID, err := wh.findExportStopEvent(
			ctx, currentVersionHistory.GetBranchToken(), resp.GetPreviousStartedEventID(), lastItem.EventID, shardID, request.GetDomain(),
		)
		if err != nil {
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

// findExportStopEvent returns the last event of a running workflow that an export can stop at. It
// first reads only the batches from the last completed decision, and reads the whole history when
// that read cannot give the event.
func (wh *WorkflowHandler) findExportStopEvent(
	ctx context.Context,
	branchToken []byte,
	previousStartedEventID int64,
	lastEventID int64,
	shardID int,
	domainName string,
) (int64, error) {
	if previousStartedEventID > constants.FirstEventID {
		stopEventID, err := wh.readExportStopEvent(ctx, branchToken, previousStartedEventID, lastEventID, shardID, domainName)
		// The read returns EntityNotExistsError when no batch starts at or after its first event.
		// This happens when the decision started in a batch that starts before it, and no batch
		// came after that batch.
		var notExistsErr *types.EntityNotExistsError
		if errors.As(err, &notExistsErr) {
			err = nil
		}
		if err != nil || stopEventID != 0 {
			return stopEventID, err
		}
	}
	stopEventID, err := wh.readExportStopEvent(ctx, branchToken, constants.FirstEventID, lastEventID, shardID, domainName)
	if err != nil {
		return 0, err
	}
	if stopEventID == 0 {
		return 0, &types.BadRequestError{Message: "The running workflow has no event that the export can stop at."}
	}
	return stopEventID, nil
}

// readExportStopEvent reads the batches from minEventID to lastEventID, and returns the last event
// that ends its batch and that an export can stop at. It returns 0 when there is no such event.
func (wh *WorkflowHandler) readExportStopEvent(
	ctx context.Context,
	branchToken []byte,
	minEventID int64,
	lastEventID int64,
	shardID int,
	domainName string,
) (int64, error) {
	serializer := persistence.NewPayloadSerializer()
	var stopEventID int64
	var pageToken []byte
	for {
		resp, err := wh.GetHistoryManager().ReadRawHistoryBranch(ctx, &persistence.ReadHistoryBranchRequest{
			BranchToken:   branchToken,
			MinEventID:    minEventID,
			MaxEventID:    lastEventID + 1,
			PageSize:      constants.GetHistoryMaxPageSize,
			NextPageToken: pageToken,
			ShardID:       common.IntPtr(shardID),
			DomainName:    domainName,
		})
		if err != nil {
			return 0, err
		}
		for _, blob := range resp.HistoryEventBlobs {
			events, err := serializer.DeserializeBatchEvents(blob)
			if err != nil {
				return 0, err
			}
			if len(events) > 0 && isExportStopEvent(events[len(events)-1]) {
				stopEventID = events[len(events)-1].ID
			}
		}
		if len(resp.NextPageToken) == 0 {
			return stopEventID, nil
		}
		pageToken = resp.NextPageToken
	}
}

// checkExportLastBatch returns true when the last batch of a page has the last event of the export.
// It returns an error when the batch has the event but the event does not end the batch, or when an
// export cannot stop at the event. The last event must be one of the events listed in isExportStopEvent.
func checkExportLastBatch(blobs []*persistence.DataBlob, lastEventID int64) (bool, error) {
	if len(blobs) > 0 {
		events, err := persistence.NewPayloadSerializer().DeserializeBatchEvents(blobs[len(blobs)-1])
		if err != nil {
			return false, err
		}
		if len(events) > 0 && events[len(events)-1].ID >= lastEventID {
			if batchLastEventID := events[len(events)-1].ID; batchLastEventID != lastEventID {
				return false, &types.BadRequestError{Message: fmt.Sprintf(
					"LastEventID %d must be the last event of a history batch, its batch ends at event %d", lastEventID, batchLastEventID,
				)}
			}
			return true, checkExportLastEvent(events[len(events)-1])
		}
	}
	return false, nil
}

// isExportStopEvent returns true when an export can stop at the event: it ends a decision task, so
// the imported workflow can schedule its next decision, or it closes the workflow. A close event is
// always the last event of the history, so a stop at it exports the whole run.
func isExportStopEvent(event *types.HistoryEvent) bool {
	switch event.GetEventType() {
	case types.EventTypeDecisionTaskStarted,
		types.EventTypeDecisionTaskTimedOut,
		types.EventTypeDecisionTaskFailed,
		types.EventTypeWorkflowExecutionCompleted,
		types.EventTypeWorkflowExecutionFailed,
		types.EventTypeWorkflowExecutionTimedOut,
		types.EventTypeWorkflowExecutionCanceled,
		types.EventTypeWorkflowExecutionTerminated,
		types.EventTypeWorkflowExecutionContinuedAsNew:
		return true
	default:
		return false
	}
}

// checkExportLastEvent checks that an export can stop at its last event.
func checkExportLastEvent(event *types.HistoryEvent) error {
	if isExportStopEvent(event) {
		return nil
	}
	switch event.GetEventType() {
	case types.EventTypeDecisionTaskCompleted:
		return &types.BadRequestError{Message: fmt.Sprintf(
			"LastEventID cannot be a DecisionTaskCompleted event: use the DecisionTaskStarted event ID %d", event.ID-1,
		)}
	default:
		return &types.BadRequestError{Message: fmt.Sprintf(
			"LastEventID %d must be a DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed or workflow close event, got: %v",
			event.ID, event.GetEventType(),
		)}
	}
}
