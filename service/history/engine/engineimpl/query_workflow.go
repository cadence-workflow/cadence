// Copyright (c) 2017-2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2021 Temporal Technologies Inc.
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

package engineimpl

import (
	"context"
	"time"

	"go.uber.org/yarpc/yarpcerrors"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/query"
	"github.com/uber/cadence/service/history/workflow"
)

func (e *historyEngineImpl) QueryWorkflow(
	ctx context.Context,
	request *types.HistoryQueryWorkflowRequest,
) (retResp *types.HistoryQueryWorkflowResponse, retErr error) {

	scope := e.metricsClient.Scope(metrics.HistoryQueryWorkflowScope).Tagged(metrics.DomainTag(request.GetRequest().GetDomain()))
	shardMetricScope := e.metricsClient.Scope(metrics.HistoryQueryWorkflowScope, metrics.ShardIDTag(e.shard.GetShardID()))

	consistentQueryEnabled := e.config.EnableConsistentQuery() && e.config.EnableConsistentQueryByDomain(request.GetRequest().GetDomain())
	if request.GetRequest().GetQueryConsistencyLevel() == types.QueryConsistencyLevelStrong {
		if !consistentQueryEnabled {
			return nil, workflow.ErrConsistentQueryNotEnabled
		}
		shardMetricScope.IncCounter(metrics.ConsistentQueryPerShard)
		e.logger.SampleInfo("History QueryWorkflow called with QueryConsistencyLevelStrong", e.config.SampleLoggingRate(), tag.ShardID(e.shard.GetShardID()), tag.WorkflowID(request.GetRequest().Execution.WorkflowID), tag.WorkflowDomainName(request.GetRequest().Domain))
	}

	execution := *request.GetRequest().GetExecution()

	mutableStateResp, err := e.getMutableState(ctx, request.GetDomainUUID(), execution)
	if err != nil {
		return nil, err
	}
	req := request.GetRequest()
	if !mutableStateResp.GetIsWorkflowRunning() && req.QueryRejectCondition != nil {
		notOpenReject := req.GetQueryRejectCondition() == types.QueryRejectConditionNotOpen
		closeStatus := mutableStateResp.GetWorkflowCloseState()
		notCompletedCleanlyReject := req.GetQueryRejectCondition() == types.QueryRejectConditionNotCompletedCleanly && closeStatus != persistence.WorkflowCloseStatusCompleted
		if notOpenReject || notCompletedCleanlyReject {
			return &types.HistoryQueryWorkflowResponse{
				Response: &types.QueryWorkflowResponse{
					QueryRejected: &types.QueryRejected{
						CloseStatus: persistence.ToInternalWorkflowExecutionCloseStatus(int(closeStatus)),
					},
				},
			}, nil
		}
	}

	// query cannot be processed unless at least one decision task has finished
	// if first decision task has not finished wait for up to a second for it to complete
	queryFirstDecisionTaskWaitTime := defaultQueryFirstDecisionTaskWaitTime
	ctxDeadline, ok := ctx.Deadline()
	if ok {
		ctxWaitTime := time.Until(ctxDeadline) - time.Second
		if ctxWaitTime > queryFirstDecisionTaskWaitTime {
			queryFirstDecisionTaskWaitTime = ctxWaitTime
		}
	}
	deadline := time.Now().Add(queryFirstDecisionTaskWaitTime)
	for mutableStateResp.GetPreviousStartedEventID() <= 0 && time.Now().Before(deadline) {
		time.Sleep(queryFirstDecisionTaskCheckInterval)
		mutableStateResp, err = e.getMutableState(ctx, request.GetDomainUUID(), execution)
		if err != nil {
			return nil, err
		}
	}

	if mutableStateResp.GetPreviousStartedEventID() <= 0 {
		scope.IncCounter(metrics.QueryBeforeFirstDecisionCount)
		return nil, workflow.ErrQueryWorkflowBeforeFirstDecision
	}

	de, err := e.shard.GetDomainCache().GetDomainByID(request.GetDomainUUID())
	if err != nil {
		return nil, err
	}

	wfContext, release, err := e.executionCache.GetOrCreateWorkflowExecution(ctx, request.GetDomainUUID(), execution)
	if err != nil {
		return nil, err
	}
	defer func() { release(retErr) }()
	mutableState, err := wfContext.LoadWorkflowExecution(ctx)
	if err != nil {
		return nil, err
	}
	// If history is corrupted, query will be rejected
	if corrupted, err := e.checkForHistoryCorruptions(ctx, mutableState); err != nil {
		return nil, err
	} else if corrupted {
		return nil, &types.EntityNotExistsError{Message: "Workflow execution corrupted."}
	}

	// There are two ways in which queries get dispatched to decider. First, queries can be dispatched on decision tasks.
	// These decision tasks potentially contain new events and queries. The events are treated as coming before the query in time.
	// The second way in which queries are dispatched to decider is directly through matching; in this approach queries can be
	// dispatched to decider immediately even if there are outstanding events that came before the query. The following logic
	// is used to determine if a query can be safely dispatched directly through matching or if given the desired consistency
	// level must be dispatched on a decision task. There are four cases in which a query can be dispatched directly through
	// matching safely, without violating the desired consistency level:
	// 1. the domain is not active, in this case history is immutable so a query dispatched at any time is consistent
	// 2. the workflow is not running, whenever a workflow is not running dispatching query directly is consistent
	// 3. the client requested eventual consistency, in this case there are no consistency requirements so dispatching directly through matching is safe
	// 4. if there is no pending or started decision it means no events came before query arrived, so its safe to dispatch directly
	isActive, _ := de.IsActiveIn(e.clusterMetadata.GetCurrentClusterName())
	safeToDispatchDirectly := !isActive ||
		!mutableState.IsWorkflowExecutionRunning() ||
		req.GetQueryConsistencyLevel() == types.QueryConsistencyLevelEventual ||
		(!mutableState.HasPendingDecision() && !mutableState.HasInFlightDecision())
	if safeToDispatchDirectly {
		release(nil)
		msResp, err := e.getMutableState(ctx, request.GetDomainUUID(), execution)
		if err != nil {
			return nil, err
		}
		req.Execution.RunID = msResp.Execution.RunID
		return e.queryDirectlyThroughMatching(ctx, msResp, request.GetDomainUUID(), req, scope)
	}

	// If we get here it means query could not be dispatched through matching directly, so it must block
	// until either an result has been obtained on a decision task response or until it is safe to dispatch directly through matching.
	sw := scope.StartTimer(metrics.DecisionTaskQueryLatency)
	defer sw.Stop()
	queryReg := mutableState.GetQueryRegistry()
	if len(queryReg.GetBufferedIDs()) >= e.config.MaxBufferedQueryCount() {
		scope.IncCounter(metrics.QueryBufferExceededCount)
		return nil, workflow.ErrConsistentQueryBufferExceeded
	}
	queryID, termCh := queryReg.BufferQuery(req.GetQuery())
	defer queryReg.RemoveQuery(queryID)
	release(nil)
	select {
	case <-termCh:
		state, err := queryReg.GetTerminationState(queryID)
		if err != nil {
			scope.IncCounter(metrics.QueryRegistryInvalidStateCount)
			return nil, err
		}
		switch state.TerminationType {
		case query.TerminationTypeCompleted:
			result := state.QueryResult
			switch result.GetResultType() {
			case types.QueryResultTypeAnswered:
				return &types.HistoryQueryWorkflowResponse{
					Response: &types.QueryWorkflowResponse{
						QueryResult: result.GetAnswer(),
					},
				}, nil
			case types.QueryResultTypeFailed:
				return nil, &types.QueryFailedError{Message: result.GetErrorMessage()}
			default:
				scope.IncCounter(metrics.QueryRegistryInvalidStateCount)
				return nil, workflow.ErrQueryEnteredInvalidState
			}
		case query.TerminationTypeUnblocked:
			msResp, err := e.getMutableState(ctx, request.GetDomainUUID(), execution)
			if err != nil {
				return nil, err
			}
			req.Execution.RunID = msResp.Execution.RunID
			return e.queryDirectlyThroughMatching(ctx, msResp, request.GetDomainUUID(), req, scope)
		case query.TerminationTypeFailed:
			return nil, state.Failure
		default:
			scope.IncCounter(metrics.QueryRegistryInvalidStateCount)
			return nil, workflow.ErrQueryEnteredInvalidState
		}
	case <-ctx.Done():
		scope.IncCounter(metrics.ConsistentQueryTimeoutCount)
		return nil, ctx.Err()
	}
}

func (e *historyEngineImpl) queryDirectlyThroughMatching(
	ctx context.Context,
	msResp *types.GetMutableStateResponse,
	domainID string,
	queryRequest *types.QueryWorkflowRequest,
	scope metrics.Scope,
) (*types.HistoryQueryWorkflowResponse, error) {

	sw := scope.StartTimer(metrics.DirectQueryDispatchLatency)
	defer sw.Stop()

	// Sticky task list is not very useful in the standby cluster because the decider cache is
	// not updated by dispatching tasks to it (it is only updated in the case of query).
	// Additionally on the standby side we are not even able to clear sticky.
	// Stickiness might be outdated if the customer did a restart of their nodes causing a query
	// dispatched on the standby side on sticky to hang. We decided it made sense to simply not attempt
	// query on sticky task list at all on the passive side.
	de, err := e.shard.GetDomainCache().GetDomainByID(domainID)
	if err != nil {
		return nil, err
	}
	supportsStickyQuery := e.clientChecker.SupportsStickyQuery(msResp.GetClientImpl(), msResp.GetClientFeatureVersion()) == nil
	domainIsActive, _ := de.IsActiveIn(e.clusterMetadata.GetCurrentClusterName())
	if msResp.GetIsStickyTaskListEnabled() &&
		len(msResp.GetStickyTaskList().GetName()) != 0 &&
		supportsStickyQuery &&
		e.config.EnableStickyQuery(queryRequest.GetDomain()) &&
		domainIsActive {

		stickyMatchingRequest := &types.MatchingQueryWorkflowRequest{
			DomainUUID:   domainID,
			QueryRequest: queryRequest,
			TaskList:     msResp.GetStickyTaskList(),
		}

		// using a clean new context in case customer provide a context which has
		// a really short deadline, causing we clear the stickiness
		stickyContext, cancel := context.WithTimeout(context.Background(), time.Duration(msResp.GetStickyTaskListScheduleToStartTimeout())*time.Second)
		stickyStopWatch := scope.StartTimer(metrics.DirectQueryDispatchStickyLatency)
		matchingResp, err := e.rawMatchingClient.QueryWorkflow(stickyContext, stickyMatchingRequest)
		stickyStopWatch.Stop()
		cancel()
		if err == nil {
			scope.IncCounter(metrics.DirectQueryDispatchStickySuccessCount)
			return &types.HistoryQueryWorkflowResponse{Response: matchingResp}, nil
		}
		switch v := err.(type) {
		case *types.StickyWorkerUnavailableError:
		case *yarpcerrors.Status:
			if v.Code() != yarpcerrors.CodeDeadlineExceeded {
				e.logger.Error("query directly though matching on sticky failed, will not attempt query on non-sticky",
					tag.WorkflowDomainName(queryRequest.GetDomain()),
					tag.WorkflowID(queryRequest.Execution.GetWorkflowID()),
					tag.WorkflowRunID(queryRequest.Execution.GetRunID()),
					tag.WorkflowQueryType(queryRequest.Query.GetQueryType()),
					tag.Error(err))
				return nil, err
			}
		default:
			e.logger.Error("query directly though matching on sticky failed, will not attempt query on non-sticky",
				tag.WorkflowDomainName(queryRequest.GetDomain()),
				tag.WorkflowID(queryRequest.Execution.GetWorkflowID()),
				tag.WorkflowRunID(queryRequest.Execution.GetRunID()),
				tag.WorkflowQueryType(queryRequest.Query.GetQueryType()),
				tag.Error(err))
			return nil, err
		}
		if msResp.GetIsWorkflowRunning() {
			e.logger.Info("query direct through matching failed on sticky, clearing sticky before attempting on non-sticky",
				tag.WorkflowDomainName(queryRequest.GetDomain()),
				tag.WorkflowID(queryRequest.Execution.GetWorkflowID()),
				tag.WorkflowRunID(queryRequest.Execution.GetRunID()),
				tag.WorkflowQueryType(queryRequest.Query.GetQueryType()),
				tag.Error(err))
			resetContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			clearStickinessStopWatch := scope.StartTimer(metrics.DirectQueryDispatchClearStickinessLatency)
			_, err := e.ResetStickyTaskList(resetContext, &types.HistoryResetStickyTaskListRequest{
				DomainUUID: domainID,
				Execution:  queryRequest.GetExecution(),
			})
			clearStickinessStopWatch.Stop()
			cancel()
			if err != nil && err != workflow.ErrAlreadyCompleted && err != workflow.ErrNotExists {
				return nil, err
			}
			scope.IncCounter(metrics.DirectQueryDispatchClearStickinessSuccessCount)
		}
	}

	if err := common.IsValidContext(ctx); err != nil {
		e.logger.Info("query context timed out before query on non-sticky task list could be attempted",
			tag.WorkflowDomainName(queryRequest.GetDomain()),
			tag.WorkflowID(queryRequest.Execution.GetWorkflowID()),
			tag.WorkflowRunID(queryRequest.Execution.GetRunID()),
			tag.WorkflowQueryType(queryRequest.Query.GetQueryType()))
		scope.IncCounter(metrics.DirectQueryDispatchTimeoutBeforeNonStickyCount)
		return nil, err
	}

	e.logger.Debug("query directly through matching on sticky timed out, attempting to query on non-sticky",
		tag.WorkflowDomainName(queryRequest.GetDomain()),
		tag.WorkflowID(queryRequest.Execution.GetWorkflowID()),
		tag.WorkflowRunID(queryRequest.Execution.GetRunID()),
		tag.WorkflowQueryType(queryRequest.Query.GetQueryType()),
		tag.WorkflowTaskListName(msResp.GetStickyTaskList().GetName()),
		tag.WorkflowNextEventID(msResp.GetNextEventID()))

	nonStickyMatchingRequest := &types.MatchingQueryWorkflowRequest{
		DomainUUID:   domainID,
		QueryRequest: queryRequest,
		TaskList:     msResp.TaskList,
	}

	nonStickyStopWatch := scope.StartTimer(metrics.DirectQueryDispatchNonStickyLatency)
	matchingResp, err := e.matchingClient.QueryWorkflow(ctx, nonStickyMatchingRequest)
	nonStickyStopWatch.Stop()
	if err != nil {
		e.logger.Error("query directly though matching on non-sticky failed",
			tag.WorkflowDomainName(queryRequest.GetDomain()),
			tag.WorkflowID(queryRequest.Execution.GetWorkflowID()),
			tag.WorkflowRunID(queryRequest.Execution.GetRunID()),
			tag.WorkflowQueryType(queryRequest.Query.GetQueryType()),
			tag.Error(err))
		return nil, err
	}
	scope.IncCounter(metrics.DirectQueryDispatchNonStickySuccessCount)
	return &types.HistoryQueryWorkflowResponse{Response: matchingResp}, err
}
