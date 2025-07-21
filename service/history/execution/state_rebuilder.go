// Copyright (c) 2019 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination state_rebuilder_mock.go -self_package github.com/uber/cadence/service/history/execution

package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/collection"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	persistenceutils "github.com/uber/cadence/common/persistence/persistence-utils"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/shard"
)

const (
	// NDCDefaultPageSize is the default pagination size for ndc
	NDCDefaultPageSize = 100
)

type (
	// StateRebuilder is a mutable state builder to ndc state rebuild
	StateRebuilder interface {
		Rebuild(
			ctx context.Context,
			now time.Time,
			baseWorkflowIdentifier definition.WorkflowIdentifier,
			baseBranchToken []byte,
			baseLastEventID int64,
			baseLastEventVersion int64,
			targetWorkflowIdentifier definition.WorkflowIdentifier,
			targetBranchFn func() ([]byte, error),
			requestID string,
		) (MutableState, int64, error)
	}

	stateRebuilderImpl struct {
		shard              shard.Context
		domainCache        cache.DomainCache
		clusterMetadata    cluster.Metadata
		historyV2Mgr       persistence.HistoryManager
		taskRefresher      MutableStateTaskRefresher
		rebuiltHistorySize int64
		logger             log.Logger
	}
)

var _ StateRebuilder = (*stateRebuilderImpl)(nil)

// NewStateRebuilder creates a state rebuilder
func NewStateRebuilder(
	shard shard.Context,
	logger log.Logger,
) StateRebuilder {

	return &stateRebuilderImpl{
		shard:           shard,
		domainCache:     shard.GetDomainCache(),
		clusterMetadata: shard.GetService().GetClusterMetadata(),
		historyV2Mgr:    shard.GetHistoryManager(),
		taskRefresher: NewMutableStateTaskRefresher(
			shard.GetConfig(),
			shard.GetClusterMetadata(),
			shard.GetDomainCache(),
			shard.GetEventsCache(),
			shard.GetShardID(),
			logger,
		),
		rebuiltHistorySize: 0,
		logger:             logger,
	}
}

func (r *stateRebuilderImpl) Rebuild(
	ctx context.Context,
	now time.Time,
	baseWorkflowIdentifier definition.WorkflowIdentifier,
	baseBranchToken []byte,
	baseLastEventID int64,
	baseLastEventVersion int64,
	targetWorkflowIdentifier definition.WorkflowIdentifier,
	targetBranchFn func() ([]byte, error),
	requestID string,
) (MutableState, int64, error) {

	iter := collection.NewPagingIterator(r.getPaginationFn(
		ctx,
		constants.FirstEventID,
		baseLastEventID+1,
		baseBranchToken,
		targetWorkflowIdentifier.DomainID,
	))

	domainEntry, err := r.domainCache.GetDomainByID(targetWorkflowIdentifier.DomainID)
	if err != nil {
		return nil, 0, err
	}

	// Corrupt data handling
	if !iter.HasNext() {
		return nil, 0, fmt.Errorf("Attempting to build history state but the iterator has found no history")
	}
	// need to specially handling the first batch, to initialize mutable state & state builder
	batch, err := iter.Next()
	if err != nil {
		return nil, 0, err
	}
	firstEventBatch := batch.(*types.History).Events
	rebuiltMutableState, stateBuilder := r.initializeBuilders(
		domainEntry,
	)
	if err := r.applyEvents(targetWorkflowIdentifier, stateBuilder, firstEventBatch, requestID); err != nil {
		return nil, 0, err
	}

	for iter.HasNext() {
		batch, err := iter.Next()
		if err != nil {
			return nil, 0, err
		}
		events := batch.(*types.History).Events
		if err := r.applyEvents(targetWorkflowIdentifier, stateBuilder, events, requestID); err != nil {
			return nil, 0, err
		}
	}

	rebuildVersionHistories := rebuiltMutableState.GetVersionHistories()
	if rebuildVersionHistories != nil {
		currentVersionHistory, err := rebuildVersionHistories.GetCurrentVersionHistory()
		if err != nil {
			return nil, 0, err
		}
		lastItem, err := currentVersionHistory.GetLastItem()
		if err != nil {
			return nil, 0, err
		}
		if !lastItem.Equals(persistence.NewVersionHistoryItem(
			baseLastEventID,
			baseLastEventVersion,
		)) {
			return nil, 0, &types.BadRequestError{Message: fmt.Sprintf(
				"nDCStateRebuilder unable to rebuild mutable state to event ID: %v, version: %v, "+
					"baseLastEventID + baseLastEventVersion is not the same as the last event of the last "+
					"batch, event ID: %v, version :%v ,typically because of attemptting to rebuild to a middle of a batch",
				baseLastEventID,
				baseLastEventVersion,
				lastItem.EventID,
				lastItem.Version,
			)}
		}
	}

	targetBranchToken, err := targetBranchFn()
	if err != nil {
		return nil, 0, err
	}

	if err := rebuiltMutableState.SetCurrentBranchToken(targetBranchToken); err != nil {
		return nil, 0, err
	}

	// close rebuilt mutable state transaction clearing all generated tasks, workflow requests, etc.
	_, _, err = rebuiltMutableState.CloseTransactionAsSnapshot(now, TransactionPolicyPassive)
	if err != nil {
		return nil, 0, err
	}

	// refresh tasks to be generated
	if err := r.taskRefresher.RefreshTasks(ctx, now, rebuiltMutableState); err != nil {
		return nil, 0, err
	}

	// mutable state rebuild should use the same time stamp
	rebuiltMutableState.GetExecutionInfo().StartTimestamp = now
	return rebuiltMutableState, r.rebuiltHistorySize, nil
}

func (r *stateRebuilderImpl) initializeBuilders(
	domainEntry *cache.DomainCacheEntry,
) (MutableState, StateBuilder) {
	resetMutableStateBuilder := NewMutableStateBuilderWithVersionHistories(
		r.shard,
		r.logger,
		domainEntry,
	)
	stateBuilder := NewStateBuilder(
		r.shard,
		r.logger,
		resetMutableStateBuilder,
	)
	return resetMutableStateBuilder, stateBuilder
}

func (r *stateRebuilderImpl) applyEvents(
	workflowIdentifier definition.WorkflowIdentifier,
	stateBuilder StateBuilder,
	events []*types.HistoryEvent,
	requestID string,
) error {

	_, err := stateBuilder.ApplyEvents(
		workflowIdentifier.DomainID,
		requestID,
		types.WorkflowExecution{
			WorkflowID: workflowIdentifier.WorkflowID,
			RunID:      workflowIdentifier.RunID,
		},
		events,
		nil, // no new run history when rebuilding mutable state
	)
	if err != nil {
		r.logger.Error("nDCStateRebuilder unable to rebuild mutable state.", tag.Error(err))
	}

	return err
}

func (r *stateRebuilderImpl) getPaginationFn(
	ctx context.Context,
	firstEventID int64,
	nextEventID int64,
	branchToken []byte,
	domainID string,
) collection.PaginationFn {

	return func(paginationToken []byte) ([]interface{}, []byte, error) {

		_, historyBatches, token, size, err := persistenceutils.PaginateHistory(
			ctx,
			r.historyV2Mgr,
			true,
			branchToken,
			firstEventID,
			nextEventID,
			paginationToken,
			NDCDefaultPageSize,
			common.IntPtr(r.shard.GetShardID()),
			domainID,
			r.domainCache,
		)
		if err != nil {
			return nil, nil, err
		}
		r.rebuiltHistorySize += int64(size)

		var paginateItems []interface{}
		for _, history := range historyBatches {
			paginateItems = append(paginateItems, history)
		}
		return paginateItems, token, nil
	}
}
