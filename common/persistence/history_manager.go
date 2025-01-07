// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package persistence

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pborman/uuid"

	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

type (
	// historyV2PagingToken is used to serialize/deserialize pagination token for ReadHistoryBranchRequest
	historyV2PagingToken struct {
		// TODO remove LastEventVersion once 3+DC is enabled for all workflow
		LastEventVersion int64
		LastEventID      int64
		// the pagination token passing to persistence
		StoreToken []byte
		// recording which branchRange it is reading
		CurrentRangeIndex int
		FinalRangeIndex   int

		// LastNodeID is the last known node ID attached to a history node
		LastNodeID int64
		// LastTransactionID is the last known transaction ID attached to a history node
		LastTransactionID int64
	}
	// historyManagerImpl implements HistoryManager based on HistoryStore and PayloadSerializer
	historyV2ManagerImpl struct {
		historySerializer      PayloadSerializer
		persistence            HistoryStore
		logger                 log.Logger
		thriftEncoder          codec.BinaryEncoder
		transactionSizeLimit   dynamicconfig.IntPropertyFn
		serializeTokenFn       func(*historyV2PagingToken) ([]byte, error)
		deserializeTokenFn     func([]byte, int64) (*historyV2PagingToken, error)
		readRawHistoryBranchFn func(context.Context, *ReadHistoryBranchRequest) ([]*DataBlob, *historyV2PagingToken, int, log.Logger, error)
		readHistoryBranchFn    func(context.Context, bool, *ReadHistoryBranchRequest) ([]*types.HistoryEvent, []*types.History, []byte, int, int64, error)
	}
)

const (
	notStartedIndex          = -1
	defaultLastNodeID        = common.FirstEventID - 1
	defaultLastTransactionID = int64(0)
)

var (
	ErrCorruptedHistory = &types.InternalDataInconsistencyError{Message: "corrupted history event batch, eventID is not continuous"}
)

var _ HistoryManager = (*historyV2ManagerImpl)(nil)

// NewHistoryV2ManagerImpl returns new HistoryManager
func NewHistoryV2ManagerImpl(
	persistence HistoryStore,
	logger log.Logger,
	historySerializer PayloadSerializer,
	binaryEncoder codec.BinaryEncoder,
	transactionSizeLimit dynamicconfig.IntPropertyFn,
) HistoryManager {
	hm := &historyV2ManagerImpl{
		historySerializer:    historySerializer,
		persistence:          persistence,
		logger:               logger,
		thriftEncoder:        binaryEncoder,
		transactionSizeLimit: transactionSizeLimit,
		serializeTokenFn:     serializeToken,
		deserializeTokenFn:   deserializeToken,
	}
	hm.readRawHistoryBranchFn = hm.readRawHistoryBranch
	hm.readHistoryBranchFn = hm.readHistoryBranch
	return hm
}

func (m *historyV2ManagerImpl) GetName() string {
	return m.persistence.GetName()
}

// ForkHistoryBranch forks a new branch from a old branch
func (m *historyV2ManagerImpl) ForkHistoryBranch(
	ctx context.Context,
	request *ForkHistoryBranchRequest,
) (*ForkHistoryBranchResponse, error) {
	if request.ForkNodeID <= 1 {
		return nil, &InvalidPersistenceRequestError{
			Msg: "ForkNodeID must be > 1",
		}
	}
	shardID, err := getShardID(request.ShardID)
	if err != nil {
		return nil, &types.InternalServiceError{
			Message: err.Error(),
		}
	}

	var forkBranch workflow.HistoryBranch
	err = m.thriftEncoder.Decode(request.ForkBranchToken, &forkBranch)
	if err != nil {
		return nil, err
	}
	req := &InternalForkHistoryBranchRequest{
		ForkBranchInfo: *thrift.ToHistoryBranch(&forkBranch),
		ForkNodeID:     request.ForkNodeID,
		NewBranchID:    uuid.New(),
		Info:           request.Info,
		ShardID:        shardID,
	}

	resp, err := m.persistence.ForkHistoryBranch(ctx, req)
	if err != nil {
		return nil, err
	}

	token, err := m.thriftEncoder.Encode(thrift.FromHistoryBranch(&resp.NewBranchInfo))
	if err != nil {
		return nil, err
	}

	return &ForkHistoryBranchResponse{
		NewBranchToken: token,
	}, nil
}

// DeleteHistoryBranch removes a branch
func (m *historyV2ManagerImpl) DeleteHistoryBranch(
	ctx context.Context,
	request *DeleteHistoryBranchRequest,
) error {
	shardID, err := getShardID(request.ShardID)
	if err != nil {
		m.logger.Error("shardID is not set in delete history operation", tag.Error(err))
		return &types.InternalServiceError{
			Message: err.Error(),
		}
	}

	var branch workflow.HistoryBranch
	err = m.thriftEncoder.Decode(request.BranchToken, &branch)
	if err != nil {
		return err
	}

	req := &InternalDeleteHistoryBranchRequest{
		BranchInfo: *thrift.ToHistoryBranch(&branch),
		ShardID:    shardID,
	}
	return m.persistence.DeleteHistoryBranch(ctx, req)
}

// GetHistoryTree returns all branch information of a tree
func (m *historyV2ManagerImpl) GetHistoryTree(
	ctx context.Context,
	request *GetHistoryTreeRequest,
) (*GetHistoryTreeResponse, error) {
	if len(request.TreeID) == 0 {
		var branch workflow.HistoryBranch
		err := m.thriftEncoder.Decode(request.BranchToken, &branch)
		if err != nil {
			return nil, err
		}
		request.TreeID = branch.GetTreeID()
	}
	internalRequest := &InternalGetHistoryTreeRequest{
		TreeID:      request.TreeID,
		ShardID:     request.ShardID,
		BranchToken: request.BranchToken,
	}
	resp, err := m.persistence.GetHistoryTree(ctx, internalRequest)
	if err != nil {
		return nil, err
	}
	var branches []*workflow.HistoryBranch
	for _, b := range resp.Branches {
		branches = append(branches, thrift.FromHistoryBranch(b))
	}
	return &GetHistoryTreeResponse{
		Branches: branches,
	}, nil
}

// AppendHistoryNodes add(or override) a node to a history branch
func (m *historyV2ManagerImpl) AppendHistoryNodes(
	ctx context.Context,
	request *AppendHistoryNodesRequest,
) (*AppendHistoryNodesResponse, error) {
	shardID, err := getShardID(request.ShardID)
	if err != nil {
		m.logger.Error("shardID is not set in append history nodes operation", tag.Error(err))
		return nil, &types.InternalServiceError{
			Message: err.Error(),
		}
	}
	if len(request.Events) == 0 {
		return nil, &InvalidPersistenceRequestError{
			Msg: "events to be appended cannot be empty",
		}
	}
	version := request.Events[0].Version
	nodeID := request.Events[0].ID
	lastID := nodeID - 1
	if nodeID <= 0 {
		return nil, &InvalidPersistenceRequestError{
			Msg: "eventID cannot be less than 1",
		}
	}
	for _, e := range request.Events {
		if e.Version != version {
			return nil, &InvalidPersistenceRequestError{
				Msg: "event version must be the same inside a batch",
			}
		}
		if e.ID != lastID+1 {
			return nil, &InvalidPersistenceRequestError{
				Msg: "event ID must be continuous",
			}
		}
		lastID++
	}
	var branch workflow.HistoryBranch
	err = m.thriftEncoder.Decode(request.BranchToken, &branch)
	if err != nil {
		return nil, err
	}

	// nodeID will be the first eventID
	blob, err := m.historySerializer.SerializeBatchEvents(request.Events, request.Encoding)
	if err != nil {
		return nil, err
	}
	size := len(blob.Data)
	sizeLimit := m.transactionSizeLimit()
	if size > sizeLimit {
		return nil, &TransactionSizeLimitError{
			Msg: fmt.Sprintf("transaction size of %v bytes exceeds limit of %v bytes", size, sizeLimit),
		}
	}
	req := &InternalAppendHistoryNodesRequest{
		IsNewBranch:   request.IsNewBranch,
		Info:          request.Info,
		BranchInfo:    *thrift.ToHistoryBranch(&branch),
		NodeID:        nodeID,
		Events:        blob,
		TransactionID: request.TransactionID,
		ShardID:       shardID,
	}

	err = m.persistence.AppendHistoryNodes(ctx, req)

	return &AppendHistoryNodesResponse{
		DataBlob: *blob,
	}, err
}

func (m *historyV2ManagerImpl) GetAllHistoryTreeBranches(
	ctx context.Context,
	request *GetAllHistoryTreeBranchesRequest,
) (*GetAllHistoryTreeBranchesResponse, error) {
	return m.persistence.GetAllHistoryTreeBranches(ctx, request)
}

// ReadHistoryBranchByBatch returns history node data for a branch by batch
// Pagination is implemented here, the actual minNodeID passing to persistence layer is calculated along with token's LastNodeID
func (m *historyV2ManagerImpl) ReadHistoryBranchByBatch(
	ctx context.Context,
	request *ReadHistoryBranchRequest,
) (*ReadHistoryBranchByBatchResponse, error) {

	resp := &ReadHistoryBranchByBatchResponse{}
	var err error
	_, resp.History, resp.NextPageToken, resp.Size, resp.LastFirstEventID, err = m.readHistoryBranchFn(ctx, true, request)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ReadHistoryBranch returns history node data for a branch
// Pagination is implemented here, the actual minNodeID passing to persistence layer is calculated along with token's LastNodeID
func (m *historyV2ManagerImpl) ReadHistoryBranch(
	ctx context.Context,
	request *ReadHistoryBranchRequest,
) (*ReadHistoryBranchResponse, error) {

	resp := &ReadHistoryBranchResponse{}
	var err error
	resp.HistoryEvents, _, resp.NextPageToken, resp.Size, resp.LastFirstEventID, err = m.readHistoryBranchFn(ctx, false, request)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ReadRawHistoryBranch returns raw history binary data for a branch
// Pagination is implemented here, the actual minNodeID passing to persistence layer is calculated along with token's LastNodeID
// NOTE: this API should only be used by 3+DC
func (m *historyV2ManagerImpl) ReadRawHistoryBranch(
	ctx context.Context,
	request *ReadHistoryBranchRequest,
) (*ReadRawHistoryBranchResponse, error) {

	dataBlobs, token, dataSize, _, err := m.readRawHistoryBranchFn(ctx, request)
	if err != nil {
		return nil, err
	}

	nextPageToken, err := m.serializeTokenFn(token)
	if err != nil {
		return nil, err
	}

	return &ReadRawHistoryBranchResponse{
		HistoryEventBlobs: dataBlobs,
		NextPageToken:     nextPageToken,
		Size:              dataSize,
	}, nil
}

func (m *historyV2ManagerImpl) readRawHistoryBranch(
	ctx context.Context,
	request *ReadHistoryBranchRequest,
) ([]*DataBlob, *historyV2PagingToken, int, log.Logger, error) {
	shardID, err := getShardID(request.ShardID)
	if err != nil {
		m.logger.Error("shardID is not set in read history branch operation", tag.Error(err))
		return nil, nil, 0, nil, &types.InternalServiceError{Message: err.Error()}
	}
	if request.PageSize <= 0 || request.MinEventID >= request.MaxEventID {
		return nil, nil, 0, nil, &InvalidPersistenceRequestError{
			Msg: fmt.Sprintf(
				"no events can be found for pageSize %v, minEventID %v, maxEventID: %v",
				request.PageSize,
				request.MinEventID,
				request.MaxEventID,
			),
		}
	}
	defaultLastEventID := request.MinEventID - 1
	token, err := m.deserializeTokenFn(
		request.NextPageToken,
		defaultLastEventID,
	)
	if err != nil {
		return nil, nil, 0, nil, err
	}

	var branch workflow.HistoryBranch
	err = m.thriftEncoder.Decode(request.BranchToken, &branch)
	if err != nil {
		return nil, nil, 0, nil, err
	}
	treeID := *branch.TreeID
	branchID := *branch.BranchID

	allBRs := branch.Ancestors
	// We may also query the current branch from beginNodeID
	beginNodeID := common.FirstEventID
	if len(branch.Ancestors) > 0 {
		beginNodeID = *branch.Ancestors[len(branch.Ancestors)-1].EndNodeID
	}
	allBRs = append(allBRs, &workflow.HistoryBranchRange{
		BranchID:    &branchID,
		BeginNodeID: common.Int64Ptr(beginNodeID),
		EndNodeID:   common.Int64Ptr(request.MaxEventID),
	})

	if token.CurrentRangeIndex == notStartedIndex {
		for idx, br := range allBRs {
			// this range won't contain any nodes needed
			if request.MinEventID >= *br.EndNodeID {
				continue
			}
			// similarly, the ranges and the rest won't contain any nodes needed,
			if request.MaxEventID <= *br.BeginNodeID {
				break
			}

			if token.CurrentRangeIndex == notStartedIndex {
				token.CurrentRangeIndex = idx
			}
			token.FinalRangeIndex = idx
		}

		if token.CurrentRangeIndex == notStartedIndex {
			return nil, nil, 0, nil, &types.InternalDataInconsistencyError{
				Message: "branchRange is corrupted",
			}
		}
	}

	minNodeID := request.MinEventID
	maxNodeID := *allBRs[token.CurrentRangeIndex].EndNodeID
	if request.MaxEventID < maxNodeID {
		maxNodeID = request.MaxEventID
	}
	pageSize := request.PageSize

	req := &InternalReadHistoryBranchRequest{
		TreeID:            treeID,
		BranchID:          *allBRs[token.CurrentRangeIndex].BranchID,
		MinNodeID:         minNodeID,
		MaxNodeID:         maxNodeID,
		NextPageToken:     token.StoreToken,
		LastNodeID:        token.LastNodeID,
		LastTransactionID: token.LastTransactionID,
		ShardID:           shardID,
		PageSize:          pageSize,
	}

	resp, err := m.persistence.ReadHistoryBranch(ctx, req)
	if err != nil {
		return nil, nil, 0, nil, err
	}
	// TODO: consider if it's possible to remove this branch
	if len(resp.History) == 0 && len(request.NextPageToken) == 0 {
		return nil, nil, 0, nil, &types.EntityNotExistsError{Message: "Workflow execution history not found."}
	}

	dataBlobs := resp.History
	dataSize := 0
	for _, dataBlob := range resp.History {
		dataSize += len(dataBlob.Data)
	}

	token.StoreToken = resp.NextPageToken
	token.LastNodeID = resp.LastNodeID
	token.LastTransactionID = resp.LastTransactionID

	// NOTE: in this method, we need to make sure eventVersion is NOT
	// decreasing(otherwise we skip the events), eventID should be continuous(otherwise return error)
	logger := m.logger.WithTags(tag.WorkflowBranchID(*branch.BranchID), tag.WorkflowTreeID(*branch.TreeID))

	return dataBlobs, token, dataSize, logger, nil
}

func (m *historyV2ManagerImpl) readHistoryBranch(
	ctx context.Context,
	byBatch bool,
	request *ReadHistoryBranchRequest,
) ([]*types.HistoryEvent, []*types.History, []byte, int, int64, error) {

	dataBlobs, token, dataSize, logger, err := m.readRawHistoryBranchFn(ctx, request)
	if err != nil {
		return nil, nil, nil, 0, 0, err
	}
	defaultLastEventID := request.MinEventID - 1

	historyEvents := make([]*types.HistoryEvent, 0, request.PageSize)
	historyEventBatches := make([]*types.History, 0, request.PageSize)
	// first_event_id of the last batch
	lastFirstEventID := common.EmptyEventID

	for _, batch := range dataBlobs {
		events, err := m.historySerializer.DeserializeBatchEvents(batch)
		if err != nil {
			return nil, nil, nil, 0, 0, err
		}
		if len(events) == 0 {
			logger.Error("Empty events in a batch")
			return nil, nil, nil, 0, 0, &types.InternalDataInconsistencyError{
				Message: "corrupted history event batch, empty events",
			}
		}

		firstEvent := events[0]           // first
		eventCount := len(events)         // length
		lastEvent := events[eventCount-1] // last

		if firstEvent.Version != lastEvent.Version || firstEvent.ID+int64(eventCount-1) != lastEvent.ID {
			// in a single batch, version should be the same, and ID should be continous
			logger.Error("Corrupted event batch",
				tag.FirstEventVersion(firstEvent.Version), tag.WorkflowFirstEventID(firstEvent.ID),
				tag.LastEventVersion(lastEvent.Version), tag.WorkflowNextEventID(lastEvent.ID),
				tag.Counter(eventCount))
			return nil, nil, nil, 0, 0, &types.InternalDataInconsistencyError{
				Message: "corrupted history event batch, wrong version and IDs",
			}
		}

		if firstEvent.Version < token.LastEventVersion {
			// version decrease means the this batch are all stale events, we should skip
			logger.Info("Stale event batch with smaller version", tag.FirstEventVersion(firstEvent.Version), tag.TokenLastEventVersion(token.LastEventVersion))
			continue
		}
		if firstEvent.ID <= token.LastEventID {
			// we could see it because first batch of next page has a smaller txn_id
			logger.Info("Stale event batch with eventID", tag.WorkflowFirstEventID(firstEvent.ID), tag.TokenLastEventID(token.LastEventID))
			continue
		}
		if firstEvent.ID != token.LastEventID+1 {
			// We assume application layer want to read from MinEventID(inclusive)
			// However, for getting history from remote cluster, there is scenario that we have to read from middle without knowing the firstEventID.
			// In that case we don't validate history continuousness for the first page
			// TODO: in this case, some events returned can be invalid(stale). application layer need to make sure it won't make any problems to XDC
			if defaultLastEventID == 0 || token.LastEventID != defaultLastEventID {
				logger.Error("Corrupted incontinouous event batch",
					tag.FirstEventVersion(firstEvent.Version), tag.WorkflowFirstEventID(firstEvent.ID),
					tag.LastEventVersion(lastEvent.Version), tag.WorkflowNextEventID(lastEvent.ID),
					tag.TokenLastEventVersion(token.LastEventVersion), tag.TokenLastEventID(token.LastEventID),
					tag.Counter(eventCount))
				return nil, nil, nil, 0, 0, ErrCorruptedHistory
			}
		}

		token.LastEventVersion = firstEvent.Version
		token.LastEventID = lastEvent.ID
		if byBatch {
			historyEventBatches = append(historyEventBatches, &types.History{Events: events})
		} else {
			historyEvents = append(historyEvents, events...)
		}
		lastFirstEventID = firstEvent.ID
	}

	nextPageToken, err := m.serializeTokenFn(token)
	if err != nil {
		return nil, nil, nil, 0, 0, err
	}

	return historyEvents, historyEventBatches, nextPageToken, dataSize, lastFirstEventID, nil
}

func deserializeToken(
	data []byte,
	defaultLastEventID int64,
) (*historyV2PagingToken, error) {
	if len(data) == 0 {
		return &historyV2PagingToken{
			LastEventID:       defaultLastEventID,
			LastEventVersion:  common.EmptyVersion,
			CurrentRangeIndex: notStartedIndex,
			LastNodeID:        defaultLastNodeID,
			LastTransactionID: defaultLastTransactionID,
		}, nil
	}

	token := historyV2PagingToken{}
	err := json.Unmarshal(data, &token)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func serializeToken(pagingToken *historyV2PagingToken) ([]byte, error) {
	if len(pagingToken.StoreToken) == 0 {
		if pagingToken.CurrentRangeIndex == pagingToken.FinalRangeIndex {
			// this means that we have reached the final page of final branchRange
			return nil, nil
		}
		pagingToken.CurrentRangeIndex++
	}
	return json.Marshal(pagingToken)
}

func (m *historyV2ManagerImpl) Close() {
	m.persistence.Close()
}

func getShardID(shardID *int) (int, error) {
	if shardID == nil {
		return 0, fmt.Errorf("shardID is not set for persistence operation")
	}
	return *shardID, nil
}
