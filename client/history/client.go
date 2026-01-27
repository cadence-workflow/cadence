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

package history

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"go.uber.org/yarpc"
	"golang.org/x/sync/errgroup"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/future"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/rpc"
	"github.com/uber/cadence/common/types"
)

var _ Client = (*clientImpl)(nil)

type (
	clientImpl struct {
		numberOfShards    int
		rpcMaxSizeInBytes int // This value currently only used in GetReplicationMessage API
		tokenSerializer   common.TaskTokenSerializer
		client            Client
		peerResolver      PeerResolver
		logger            log.Logger
	}

	getReplicationMessagesWithSize struct {
		response *types.GetReplicationMessagesResponse
		size     int
		peer     string

		// earliestCreationTime of replication tasks of response
		earliestCreationTime *int64
	}
)

// NewClient creates a new history service TChannel client
func NewClient(
	numberOfShards int,
	rpcMaxSizeInBytes int,
	client Client,
	peerResolver PeerResolver,
	logger log.Logger,
) Client {
	return &clientImpl{
		numberOfShards:    numberOfShards,
		rpcMaxSizeInBytes: rpcMaxSizeInBytes,
		tokenSerializer:   common.NewJSONTaskTokenSerializer(),
		client:            client,
		peerResolver:      peerResolver,
		logger:            logger,
	}
}

func (c *clientImpl) StartWorkflowExecution(
	ctx context.Context,
	request *types.HistoryStartWorkflowExecutionRequest,
	opts ...yarpc.CallOption,
) (*types.StartWorkflowExecutionResponse, error) {
	peer, err := c.peerResolver.FromWorkflowID(request.StartRequest.WorkflowID)
	if err != nil {
		return nil, err
	}
	var response *types.StartWorkflowExecutionResponse
	op := func(ctx context.Context, peer string) error {
		var err error
		response, err = c.client.StartWorkflowExecution(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}
	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *clientImpl) GetMutableState(
	ctx context.Context,
	request *types.GetMutableStateRequest,
	opts ...yarpc.CallOption,
) (*types.GetMutableStateResponse, error) {
	peer, err := c.peerResolver.FromWorkflowID(request.Execution.WorkflowID)
	if err != nil {
		return nil, err
	}
	var response *types.GetMutableStateResponse
	op := func(ctx context.Context, peer string) error {
		var err error
		response, err = c.client.GetMutableState(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}
	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *clientImpl) PollMutableState(
	ctx context.Context,
	request *types.PollMutableStateRequest,
	opts ...yarpc.CallOption,
) (*types.PollMutableStateResponse, error) {
	peer, err := c.peerResolver.FromWorkflowID(request.Execution.WorkflowID)
	if err != nil {
		return nil, err
	}
	var response *types.PollMutableStateResponse
	op := func(ctx context.Context, peer string) error {
		var err error
		response, err = c.client.PollMutableState(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}
	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *clientImpl) DescribeHistoryHost(
	ctx context.Context,
	request *types.DescribeHistoryHostRequest,
	opts ...yarpc.CallOption,
) (*types.DescribeHistoryHostResponse, error) {

	var err error
	var peer string

	if request.ShardIDForHost != nil {
		peer, err = c.peerResolver.FromShardID(int(request.GetShardIDForHost()))
		if err != nil {
			c.logger.Error("peer could not be resolved for host.", tag.Error(err), tag.ShardID(int(request.GetShardIDForHost())))
			return nil, err
		}

	} else if request.ExecutionForHost != nil {
		peer, err = c.peerResolver.FromWorkflowID(request.ExecutionForHost.GetWorkflowID())
		if err != nil {
			c.logger.Error("peer could not be resolved for workflow.", tag.Error(err), tag.WorkflowID(request.ExecutionForHost.GetWorkflowID()))
			return nil, err
		}

	} else {
		peer, err = c.peerResolver.FromHostAddress(request.GetHostAddress())
		if err != nil {
			c.logger.Error("peer could not be resolved for address.", tag.Error(err), tag.Address(request.GetHostAddress()))
			return nil, err
		}

	}
	var response *types.DescribeHistoryHostResponse
	op := func(ctx context.Context, peer string) error {
		var err error
		response, err = c.client.DescribeHistoryHost(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}
	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *clientImpl) RemoveTask(
	ctx context.Context,
	request *types.RemoveTaskRequest,
	opts ...yarpc.CallOption,
) error {
	peer, err := c.peerResolver.FromShardID(int(request.GetShardID()))
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		var err error
		err = c.client.RemoveTask(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}

	err = c.executeWithRedirect(ctx, peer, op)
	return err
}

func (c *clientImpl) CloseShard(
	ctx context.Context,
	request *types.CloseShardRequest,
	opts ...yarpc.CallOption,
) error {
	peer, err := c.peerResolver.FromShardID(int(request.GetShardID()))
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		var err error
		err = c.client.CloseShard(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}

	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return err
	}
	return nil
}

func (c *clientImpl) ResetQueue(
	ctx context.Context,
	request *types.ResetQueueRequest,
	opts ...yarpc.CallOption,
) error {
	peer, err := c.peerResolver.FromShardID(int(request.GetShardID()))
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		var err error
		err = c.client.ResetQueue(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}

	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return err
	}
	return nil
}

func (c *clientImpl) DescribeQueue(
	ctx context.Context,
	request *types.DescribeQueueRequest,
	opts ...yarpc.CallOption,
) (*types.DescribeQueueResponse, error) {
	peer, err := c.peerResolver.FromShardID(int(request.GetShardID()))
	if err != nil {
		return nil, err
	}
	var response *types.DescribeQueueResponse
	op := func(ctx context.Context, peer string) error {
		var err error
		response, err = c.client.DescribeQueue(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}

	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *clientImpl) DescribeMutableState(
	ctx context.Context,
	request *types.DescribeMutableStateRequest,
	opts ...yarpc.CallOption,
) (*types.DescribeMutableStateResponse, error) {
	peer, err := c.peerResolver.FromWorkflowID(request.Execution.WorkflowID)
	if err != nil {
		return nil, err
	}
	var response *types.DescribeMutableStateResponse
	op := func(ctx context.Context, peer string) error {
		var err error
		response, err = c.client.DescribeMutableState(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}
	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *clientImpl) ResetStickyTaskList(
	ctx context.Context,
	request *types.HistoryResetStickyTaskListRequest,
	opts ...yarpc.CallOption,
) (*types.HistoryResetStickyTaskListResponse, error) {
	peer, err := c.peerResolver.FromWorkflowID(request.Execution.WorkflowID)
	if err != nil {
		return nil, err
	}
	var response *types.HistoryResetStickyTaskListResponse
	op := func(ctx context.Context, peer string) error {
		var err error
		response, err = c.client.ResetStickyTaskList(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}
	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *clientImpl) DescribeWorkflowExecution(
	ctx context.Context,
	request *types.HistoryDescribeWorkflowExecutionRequest,
	opts ...yarpc.CallOption,
) (*types.DescribeWorkflowExecutionResponse, error) {
	peer, err := c.peerResolver.FromWorkflowID(request.Request.Execution.WorkflowID)
	if err != nil {
		return nil, err
	}
	var response *types.DescribeWorkflowExecutionResponse
	op := func(ctx context.Context, peer string) error {
		var err error
		response, err = c.client.DescribeWorkflowExecution(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}
	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *clientImpl) RecordDecisionTaskStarted(
	ctx context.Context,
	request *types.RecordDecisionTaskStartedRequest,
	opts ...yarpc.CallOption,
) (*types.RecordDecisionTaskStartedResponse, error) {
	peer, err := c.peerResolver.FromWorkflowID(request.WorkflowExecution.WorkflowID)
	if err != nil {
		return nil, err
	}
	var response *types.RecordDecisionTaskStartedResponse
	op := func(ctx context.Context, peer string) error {
		var err error
		response, err = c.client.RecordDecisionTaskStarted(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}
	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *clientImpl) RecordActivityTaskStarted(
	ctx context.Context,
	request *types.RecordActivityTaskStartedRequest,
	opts ...yarpc.CallOption,
) (*types.RecordActivityTaskStartedResponse, error) {
	peer, err := c.peerResolver.FromWorkflowID(request.WorkflowExecution.WorkflowID)
	if err != nil {
		return nil, err
	}
	var response *types.RecordActivityTaskStartedResponse
	op := func(ctx context.Context, peer string) error {
		var err error
		response, err = c.client.RecordActivityTaskStarted(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}
	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *clientImpl) RespondDecisionTaskCompleted(
	ctx context.Context,
	request *types.HistoryRespondDecisionTaskCompletedRequest,
	opts ...yarpc.CallOption,
) (*types.HistoryRespondDecisionTaskCompletedResponse, error) {
	taskToken, err := c.tokenSerializer.Deserialize(request.CompleteRequest.TaskToken)
	if err != nil {
		return nil, err
	}
	peer, err := c.peerResolver.FromWorkflowID(taskToken.WorkflowID)
	if err != nil {
		return nil, err
	}
	var response *types.HistoryRespondDecisionTaskCompletedResponse
	op := func(ctx context.Context, peer string) error {
		response, err = c.client.RespondDecisionTaskCompleted(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}
	err = c.executeWithRedirect(ctx, peer, op)
	return response, err
}

func (c *clientImpl) RespondDecisionTaskFailed(
	ctx context.Context,
	request *types.HistoryRespondDecisionTaskFailedRequest,
	opts ...yarpc.CallOption,
) error {
	taskToken, err := c.tokenSerializer.Deserialize(request.FailedRequest.TaskToken)
	if err != nil {
		return err
	}
	peer, err := c.peerResolver.FromWorkflowID(taskToken.WorkflowID)
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		return c.client.RespondDecisionTaskFailed(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	}
	err = c.executeWithRedirect(ctx, peer, op)
	return err
}

func (c *clientImpl) RespondActivityTaskCompleted(
	ctx context.Context,
	request *types.HistoryRespondActivityTaskCompletedRequest,
	opts ...yarpc.CallOption,
) error {
	taskToken, err := c.tokenSerializer.Deserialize(request.CompleteRequest.TaskToken)
	if err != nil {
		return err
	}
	peer, err := c.peerResolver.FromWorkflowID(taskToken.WorkflowID)
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		return c.client.RespondActivityTaskCompleted(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	}
	err = c.executeWithRedirect(ctx, peer, op)
	return err
}

func (c *clientImpl) RespondActivityTaskFailed(
	ctx context.Context,
	request *types.HistoryRespondActivityTaskFailedRequest,
	opts ...yarpc.CallOption,
) error {
	taskToken, err := c.tokenSerializer.Deserialize(request.FailedRequest.TaskToken)
	if err != nil {
		return err
	}
	peer, err := c.peerResolver.FromWorkflowID(taskToken.WorkflowID)
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		return c.client.RespondActivityTaskFailed(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	}
	err = c.executeWithRedirect(ctx, peer, op)
	return err
}

func (c *clientImpl) RespondActivityTaskCanceled(
	ctx context.Context,
	request *types.HistoryRespondActivityTaskCanceledRequest,
	opts ...yarpc.CallOption,
) error {
	taskToken, err := c.tokenSerializer.Deserialize(request.CancelRequest.TaskToken)
	if err != nil {
		return err
	}
	peer, err := c.peerResolver.FromWorkflowID(taskToken.WorkflowID)
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		return c.client.RespondActivityTaskCanceled(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	}
	err = c.executeWithRedirect(ctx, peer, op)
	return err
}

func (c *clientImpl) RecordActivityTaskHeartbeat(
	ctx context.Context,
	request *types.HistoryRecordActivityTaskHeartbeatRequest,
	opts ...yarpc.CallOption,
) (*types.RecordActivityTaskHeartbeatResponse, error) {
	taskToken, err := c.tokenSerializer.Deserialize(request.HeartbeatRequest.TaskToken)
	if err != nil {
		return nil, err
	}
	peer, err := c.peerResolver.FromWorkflowID(taskToken.WorkflowID)
	if err != nil {
		return nil, err
	}
	var response *types.RecordActivityTaskHeartbeatResponse
	op := func(ctx context.Context, peer string) error {
		var err error
		response, err = c.client.RecordActivityTaskHeartbeat(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}
	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *clientImpl) RequestCancelWorkflowExecution(
	ctx context.Context,
	request *types.HistoryRequestCancelWorkflowExecutionRequest,
	opts ...yarpc.CallOption,
) error {
	peer, err := c.peerResolver.FromWorkflowID(request.CancelRequest.WorkflowExecution.WorkflowID)
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		return c.client.RequestCancelWorkflowExecution(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	}
	return c.executeWithRedirect(ctx, peer, op)
}

func (c *clientImpl) SignalWorkflowExecution(
	ctx context.Context,
	request *types.HistorySignalWorkflowExecutionRequest,
	opts ...yarpc.CallOption,
) error {
	peer, err := c.peerResolver.FromWorkflowID(request.SignalRequest.WorkflowExecution.WorkflowID)
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		return c.client.SignalWorkflowExecution(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	}
	err = c.executeWithRedirect(ctx, peer, op)

	return err
}

func (c *clientImpl) SignalWithStartWorkflowExecution(
	ctx context.Context,
	request *types.HistorySignalWithStartWorkflowExecutionRequest,
	opts ...yarpc.CallOption,
) (*types.StartWorkflowExecutionResponse, error) {
	peer, err := c.peerResolver.FromWorkflowID(request.SignalWithStartRequest.WorkflowID)
	if err != nil {
		return nil, err
	}
	var response *types.StartWorkflowExecutionResponse
	op := func(ctx context.Context, peer string) error {
		var err error
		response, err = c.client.SignalWithStartWorkflowExecution(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}
	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}

	return response, err
}

func (c *clientImpl) RemoveSignalMutableState(
	ctx context.Context,
	request *types.RemoveSignalMutableStateRequest,
	opts ...yarpc.CallOption,
) error {
	peer, err := c.peerResolver.FromWorkflowID(request.WorkflowExecution.WorkflowID)
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		return c.client.RemoveSignalMutableState(ctx, request, yarpc.WithShardKey(peer))
	}
	err = c.executeWithRedirect(ctx, peer, op)

	return err
}

func (c *clientImpl) TerminateWorkflowExecution(
	ctx context.Context,
	request *types.HistoryTerminateWorkflowExecutionRequest,
	opts ...yarpc.CallOption,
) error {
	peer, err := c.peerResolver.FromWorkflowID(request.TerminateRequest.WorkflowExecution.WorkflowID)
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		return c.client.TerminateWorkflowExecution(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	}
	err = c.executeWithRedirect(ctx, peer, op)
	return err
}

func (c *clientImpl) ResetWorkflowExecution(
	ctx context.Context,
	request *types.HistoryResetWorkflowExecutionRequest,
	opts ...yarpc.CallOption,
) (*types.ResetWorkflowExecutionResponse, error) {
	peer, err := c.peerResolver.FromWorkflowID(request.ResetRequest.WorkflowExecution.WorkflowID)
	if err != nil {
		return nil, err
	}
	var response *types.ResetWorkflowExecutionResponse
	op := func(ctx context.Context, peer string) error {
		response, err = c.client.ResetWorkflowExecution(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}
	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}
	return response, err
}

func (c *clientImpl) ScheduleDecisionTask(
	ctx context.Context,
	request *types.ScheduleDecisionTaskRequest,
	opts ...yarpc.CallOption,
) error {
	peer, err := c.peerResolver.FromWorkflowID(request.WorkflowExecution.WorkflowID)
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		return c.client.ScheduleDecisionTask(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	}
	err = c.executeWithRedirect(ctx, peer, op)
	return err
}

func (c *clientImpl) RecordChildExecutionCompleted(
	ctx context.Context,
	request *types.RecordChildExecutionCompletedRequest,
	opts ...yarpc.CallOption,
) error {
	peer, err := c.peerResolver.FromWorkflowID(request.WorkflowExecution.WorkflowID)
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		return c.client.RecordChildExecutionCompleted(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	}
	err = c.executeWithRedirect(ctx, peer, op)
	return err
}

func (c *clientImpl) ReplicateEventsV2(
	ctx context.Context,
	request *types.ReplicateEventsV2Request,
	opts ...yarpc.CallOption,
) error {
	peer, err := c.peerResolver.FromWorkflowID(request.WorkflowExecution.GetWorkflowID())
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		return c.client.ReplicateEventsV2(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	}
	err = c.executeWithRedirect(ctx, peer, op)
	return err
}

func (c *clientImpl) SyncShardStatus(
	ctx context.Context,
	request *types.SyncShardStatusRequest,
	opts ...yarpc.CallOption,
) error {

	// we do not have a workflow ID here, instead, we have something even better
	peer, err := c.peerResolver.FromShardID(int(request.GetShardID()))
	if err != nil {
		return err
	}

	op := func(ctx context.Context, peer string) error {
		return c.client.SyncShardStatus(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	}
	err = c.executeWithRedirect(ctx, peer, op)
	return err
}

func (c *clientImpl) SyncActivity(
	ctx context.Context,
	request *types.SyncActivityRequest,
	opts ...yarpc.CallOption,
) error {

	peer, err := c.peerResolver.FromWorkflowID(request.GetWorkflowID())
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		return c.client.SyncActivity(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	}
	err = c.executeWithRedirect(ctx, peer, op)
	return err
}

func (c *clientImpl) QueryWorkflow(
	ctx context.Context,
	request *types.HistoryQueryWorkflowRequest,
	opts ...yarpc.CallOption,
) (*types.HistoryQueryWorkflowResponse, error) {
	peer, err := c.peerResolver.FromWorkflowID(request.GetRequest().GetExecution().GetWorkflowID())
	if err != nil {
		return nil, err
	}
	var response *types.HistoryQueryWorkflowResponse
	op := func(ctx context.Context, peer string) error {
		var err error
		response, err = c.client.QueryWorkflow(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}
	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *clientImpl) GetReplicationMessages(
	ctx context.Context,
	request *types.GetReplicationMessagesRequest,
	opts ...yarpc.CallOption,
) (*types.GetReplicationMessagesResponse, error) {
	requestsByPeer := make(map[string]*types.GetReplicationMessagesRequest)

	for _, token := range request.Tokens {
		peer, err := c.peerResolver.FromShardID(int(token.GetShardID()))
		if err != nil {
			return nil, err
		}

		if _, ok := requestsByPeer[peer]; !ok {
			requestsByPeer[peer] = &types.GetReplicationMessagesRequest{
				ClusterName: request.ClusterName,
			}
		}

		req := requestsByPeer[peer]
		req.Tokens = append(req.Tokens, token)
	}

	g := &errgroup.Group{}
	var responseMutex sync.Mutex
	peerResponses := make([]*getReplicationMessagesWithSize, 0, len(requestsByPeer))

	for peer, req := range requestsByPeer {
		g.Go(func() (e error) {
			defer func() { log.CapturePanic(recover(), c.logger, &e) }()

			requestContext, cancel := common.CreateChildContext(ctx, 0.05)
			defer cancel()

			requestContext, responseInfo := rpc.ContextWithResponseInfo(requestContext)
			resp, err := c.client.GetReplicationMessages(requestContext, req, append(opts, yarpc.WithShardKey(peer))...)
			if err != nil {
				c.logger.Warn("Failed to get replication tasks from client",
					tag.Error(err),
					tag.ShardReplicationToken(req),
				)
				// Returns service busy error to notify replication
				if _, ok := err.(*types.ServiceBusyError); ok {
					return err
				}
				return nil
			}
			responseMutex.Lock()
			peerResponses = append(peerResponses, &getReplicationMessagesWithSize{
				response:             resp,
				size:                 responseInfo.Size,
				peer:                 peer,
				earliestCreationTime: resp.GetEarliestCreationTime(),
			})
			responseMutex.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return c.buildGetReplicationMessagesResponse(peerResponses), nil
}

// buildGetReplicationMessagesResponse builds a new GetReplicationMessagesResponse from peer responses
// The response can be partial if the total size of the response exceeds the max size.
// In this case, responses with oldest replication tasks will be returned
func (c *clientImpl) buildGetReplicationMessagesResponse(peerResponses []*getReplicationMessagesWithSize) *types.GetReplicationMessagesResponse {
	// Peers with large response can cause the response to exceed the max size.
	// In this case, we need to skip some peer responses to make sure the result response size is within the limit.
	// To prevent a replication lag in the future, we should return the response with the oldest replication task.
	// So we sort the peer responses by the earliest creation time of the replication task.
	// If the earliest creation time is the same, we compare the size of the response.
	// This will sure that shards with the oldest replication tasks will be processed first.
	sortGetReplicationMessageWithSize(peerResponses)

	response := &types.GetReplicationMessagesResponse{MessagesByShard: make(map[int32]*types.ReplicationMessages)}
	responseTotalSize := 0
	rpcMaxResponseSize := c.rpcMaxSizeInBytes
	for _, resp := range peerResponses {
		if (responseTotalSize + resp.size) >= rpcMaxResponseSize {
			// Log shards that did not fit for debugging purposes
			for shardID := range resp.response.GetMessagesByShard() {
				c.logger.Warn("Replication messages did not fit in the response",
					tag.ShardID(int(shardID)),
					tag.Address(resp.peer),
					tag.ResponseSize(resp.size),
					tag.ResponseTotalSize(responseTotalSize),
					tag.ResponseMaxSize(rpcMaxResponseSize),
				)
			}

			// return partial response if the response size exceeded supported max size
			// but continue with next peer response, as it may still fit
			continue
		}

		responseTotalSize += resp.size

		for shardID, tasks := range resp.response.GetMessagesByShard() {
			response.MessagesByShard[shardID] = tasks
		}
	}

	return response
}

// sortGetReplicationMessageWithSize sorts the peer responses by the earliest creation time of the replication tasks
func sortGetReplicationMessageWithSize(peerResponses []*getReplicationMessagesWithSize) {
	slices.SortStableFunc(peerResponses, cmpGetReplicationMessagesWithSize)
}

// cmpGetReplicationMessagesWithSize compares
// two getReplicationMessagesWithSize objects by earliest creation time
// it can be used as a comparison func for slices.SortStableFunc
// if a, b, or their earliestCreationTime is nil, slices.SortStableFunc will put them to the end of a slice
// otherwise it will compare the earliestCreationTime of the replication tasks
// if earliestCreationTime is equal, it will compare the size of the response
func cmpGetReplicationMessagesWithSize(a, b *getReplicationMessagesWithSize) int {
	// a > b
	if a == nil || a.earliestCreationTime == nil {
		return 1
	}
	// a < b
	if b == nil || b.earliestCreationTime == nil {
		return -1
	}

	// if both are not nil, compare the creation time
	if *a.earliestCreationTime < *b.earliestCreationTime {
		return -1
	}

	if *a.earliestCreationTime > *b.earliestCreationTime {
		return 1
	}

	// if both equal, compare the size
	if a.size < b.size {
		return -1
	}

	if a.size > b.size {
		return 1
	}

	return 0
}

func (c *clientImpl) GetDLQReplicationMessages(
	ctx context.Context,
	request *types.GetDLQReplicationMessagesRequest,
	opts ...yarpc.CallOption,
) (*types.GetDLQReplicationMessagesResponse, error) {
	// All workflow IDs are in the same shard per request
	workflowID := request.GetTaskInfos()[0].GetWorkflowID()
	peer, err := c.peerResolver.FromWorkflowID(workflowID)
	if err != nil {
		return nil, err
	}

	return c.client.GetDLQReplicationMessages(
		ctx,
		request,
		append(opts, yarpc.WithShardKey(peer))...,
	)
}

func (c *clientImpl) ReapplyEvents(
	ctx context.Context,
	request *types.HistoryReapplyEventsRequest,
	opts ...yarpc.CallOption,
) error {
	peer, err := c.peerResolver.FromWorkflowID(request.GetRequest().GetWorkflowExecution().GetWorkflowID())
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		return c.client.ReapplyEvents(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	}
	err = c.executeWithRedirect(ctx, peer, op)
	return err
}

func (c *clientImpl) CountDLQMessages(
	ctx context.Context,
	request *types.CountDLQMessagesRequest,
	opts ...yarpc.CallOption,
) (*types.HistoryCountDLQMessagesResponse, error) {

	peers, err := c.peerResolver.GetAllPeers()
	if err != nil {
		return nil, err
	}

	var mu sync.Mutex
	responses := make([]*types.HistoryCountDLQMessagesResponse, 0, len(peers))

	g := &errgroup.Group{}
	for _, peer := range peers {
		g.Go(func() (e error) {
			defer func() { log.CapturePanic(recover(), c.logger, &e) }()

			response, err := c.client.CountDLQMessages(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
			if err == nil {
				mu.Lock()
				responses = append(responses, response)
				mu.Unlock()
			}

			return err
		})
	}

	err = g.Wait()

	entries := map[types.HistoryDLQCountKey]int64{}
	for _, response := range responses {
		for key, count := range response.Entries {
			entries[key] = count
		}
	}
	return &types.HistoryCountDLQMessagesResponse{Entries: entries}, err
}

func (c *clientImpl) ReadDLQMessages(
	ctx context.Context,
	request *types.ReadDLQMessagesRequest,
	opts ...yarpc.CallOption,
) (*types.ReadDLQMessagesResponse, error) {

	peer, err := c.peerResolver.FromShardID(int(request.GetShardID()))
	if err != nil {
		return nil, err
	}
	return c.client.ReadDLQMessages(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
}

func (c *clientImpl) PurgeDLQMessages(
	ctx context.Context,
	request *types.PurgeDLQMessagesRequest,
	opts ...yarpc.CallOption,
) error {

	peer, err := c.peerResolver.FromShardID(int(request.GetShardID()))
	if err != nil {
		return err
	}
	return c.client.PurgeDLQMessages(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
}

func (c *clientImpl) MergeDLQMessages(
	ctx context.Context,
	request *types.MergeDLQMessagesRequest,
	opts ...yarpc.CallOption,
) (*types.MergeDLQMessagesResponse, error) {

	peer, err := c.peerResolver.FromShardID(int(request.GetShardID()))
	if err != nil {
		return nil, err
	}
	return c.client.MergeDLQMessages(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
}

func (c *clientImpl) RefreshWorkflowTasks(
	ctx context.Context,
	request *types.HistoryRefreshWorkflowTasksRequest,
	opts ...yarpc.CallOption,
) error {
	peer, err := c.peerResolver.FromWorkflowID(request.GetRequest().GetExecution().GetWorkflowID())
	if err != nil {
		return err
	}
	op := func(ctx context.Context, peer string) error {
		return c.client.RefreshWorkflowTasks(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	}
	err = c.executeWithRedirect(ctx, peer, op)
	return err
}

func (c *clientImpl) NotifyFailoverMarkers(
	ctx context.Context,
	request *types.NotifyFailoverMarkersRequest,
	opts ...yarpc.CallOption,
) error {
	requestsByPeer := make(map[string]*types.NotifyFailoverMarkersRequest)

	for _, token := range request.GetFailoverMarkerTokens() {
		marker := token.GetFailoverMarker()
		peer, err := c.peerResolver.FromDomainID(marker.GetDomainID())
		if err != nil {
			return err
		}
		if _, ok := requestsByPeer[peer]; !ok {
			requestsByPeer[peer] = &types.NotifyFailoverMarkersRequest{
				FailoverMarkerTokens: []*types.FailoverMarkerToken{},
			}
		}

		req := requestsByPeer[peer]
		req.FailoverMarkerTokens = append(req.FailoverMarkerTokens, token)
	}

	g := &errgroup.Group{}
	for peer, req := range requestsByPeer {
		g.Go(func() (e error) {
			defer func() { log.CapturePanic(recover(), c.logger, &e) }()
			return c.client.NotifyFailoverMarkers(ctx, req, append(opts, yarpc.WithShardKey(peer))...)
		})
	}

	return g.Wait()
}

func (c *clientImpl) GetCrossClusterTasks(
	ctx context.Context,
	request *types.GetCrossClusterTasksRequest,
	opts ...yarpc.CallOption,
) (*types.GetCrossClusterTasksResponse, error) {
	requestByPeer := make(map[string]*types.GetCrossClusterTasksRequest)
	for _, shardID := range request.GetShardIDs() {
		peer, err := c.peerResolver.FromShardID(int(shardID))
		if err != nil {
			return nil, err
		}

		if _, ok := requestByPeer[peer]; !ok {
			requestByPeer[peer] = &types.GetCrossClusterTasksRequest{
				TargetCluster: request.TargetCluster,
			}
		}
		requestByPeer[peer].ShardIDs = append(requestByPeer[peer].ShardIDs, shardID)
	}

	// preserve 5% timeout to return partial of the result if context is timing out
	ctx, cancel := common.CreateChildContext(ctx, 0.05)
	defer cancel()

	futureByPeer := make(map[string]future.Future, len(requestByPeer))
	for peer, req := range requestByPeer {
		future, settable := future.NewFuture()
		go func(ctx context.Context, peer string, req *types.GetCrossClusterTasksRequest) {
			settable.Set(c.client.GetCrossClusterTasks(ctx, req, yarpc.WithShardKey(peer)))
		}(ctx, peer, req)

		futureByPeer[peer] = future
	}

	response := &types.GetCrossClusterTasksResponse{
		TasksByShard:       make(map[int32][]*types.CrossClusterTaskRequest),
		FailedCauseByShard: make(map[int32]types.GetTaskFailedCause),
	}
	for peer, future := range futureByPeer {
		var resp *types.GetCrossClusterTasksResponse
		if futureErr := future.Get(ctx, &resp); futureErr != nil {
			c.logger.Error("Failed to get cross cluster tasks", tag.Error(futureErr))
			for _, failedShardID := range requestByPeer[peer].ShardIDs {
				response.FailedCauseByShard[failedShardID] = common.ConvertErrToGetTaskFailedCause(futureErr)
			}
		} else {
			for shardID, tasks := range resp.TasksByShard {
				response.TasksByShard[shardID] = tasks
			}
			for shardID, failedCause := range resp.FailedCauseByShard {
				response.FailedCauseByShard[shardID] = failedCause
			}
		}
	}
	// not using a waitGroup for created goroutines as once all futures are unblocked,
	// those goroutines will eventually be completed

	return response, nil
}

func (c *clientImpl) RespondCrossClusterTasksCompleted(
	ctx context.Context,
	request *types.RespondCrossClusterTasksCompletedRequest,
	opts ...yarpc.CallOption,
) (*types.RespondCrossClusterTasksCompletedResponse, error) {
	peer, err := c.peerResolver.FromShardID(int(request.GetShardID()))
	if err != nil {
		return nil, err
	}

	var response *types.RespondCrossClusterTasksCompletedResponse
	op := func(ctx context.Context, peer string) error {
		var err error
		response, err = c.client.RespondCrossClusterTasksCompleted(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
		return err
	}

	err = c.executeWithRedirect(ctx, peer, op)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *clientImpl) GetFailoverInfo(
	ctx context.Context,
	request *types.GetFailoverInfoRequest,
	opts ...yarpc.CallOption,
) (*types.GetFailoverInfoResponse, error) {
	peer, err := c.peerResolver.FromDomainID(request.GetDomainID())
	if err != nil {
		return nil, err
	}
	return c.client.GetFailoverInfo(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
}

func (c *clientImpl) RatelimitUpdate(ctx context.Context, request *types.RatelimitUpdateRequest, opts ...yarpc.CallOption) (*types.RatelimitUpdateResponse, error) {
	if len(opts) == 0 {
		// unfortunately there is not really any way to ensure "must have a shard key option"
		// due to the closed nature of yarpc.CallOption's implementation, outside private-field-reading reflection.
		//
		// there are a few options to work around this, but they are currently rather high effort or
		// run into import cycles or similar.  risk is low for now as there is only one caller, and likely
		// never will be others.
		return nil, fmt.Errorf("invalid arguments, missing yarpc.WithShardKey(peer) at a minimum")
	}

	// intentionally does not use peer-redirecting retries, as keys in this request
	// could end up on multiple different hosts after a peer change.
	return c.client.RatelimitUpdate(ctx, request, opts...)
}

func (c *clientImpl) executeWithRedirect(
	ctx context.Context,
	peer string,
	op func(ctx context.Context, peer string) error,
) error {
	var err error
	if ctx == nil {
		ctx = context.Background()
	}
redirectLoop:
	for {
		err = common.IsValidContext(ctx)
		if err != nil {
			break redirectLoop
		}
		err = op(ctx, peer)
		if err != nil {
			if s, ok := err.(*types.ShardOwnershipLostError); ok {
				// TODO: consider emitting a metric for number of redirects
				peer, err = c.peerResolver.FromHostAddress(s.GetOwner())
				if err != nil {
					return err
				}
				continue redirectLoop
			}
		}
		break redirectLoop
	}
	return err
}
