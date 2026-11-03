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

package matching

import (
	"context"
	"errors"

	"go.uber.org/yarpc"

	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/future"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

var _ Gateway = (*gatewayImpl)(nil)

type gatewayImpl struct {
	client       Client
	peerResolver PeerResolver
	loadBalancer LoadBalancer
	provider     PartitionConfigProvider
}

// NewGateway creates a matching Gateway which sends requests through the given RPC client
func NewGateway(
	client Client,
	peerResolver PeerResolver,
	lb LoadBalancer,
	provider PartitionConfigProvider,
) Gateway {
	return &gatewayImpl{
		client:       client,
		peerResolver: peerResolver,
		loadBalancer: lb,
		provider:     provider,
	}
}

func (c *gatewayImpl) AddActivityTask(
	ctx context.Context,
	request *types.AddActivityTaskRequest,
) (*types.AddActivityTaskResponse, error) {
	partition := c.loadBalancer.PickWritePartition(
		persistence.TaskListTypeActivity,
		request,
	)
	originalTaskList := request.TaskList
	request.TaskList = &types.TaskList{
		Name: partition,
		Kind: originalTaskList.Kind,
	}
	peer, err := c.peerResolver.FromTaskList(request.TaskList.GetName())
	if err != nil {
		return nil, err
	}
	resp, err := c.client.AddActivityTask(ctx, request, yarpc.WithShardKey(peer))
	if err != nil {
		// ReadOnlyPartitionError indicates the partition is being drained - invalidate cache to force next request to route to root partition
		var readOnlyErr *types.ReadOnlyPartitionError
		if errors.As(err, &readOnlyErr) {
			c.provider.InvalidatePartitionCache(request.GetDomainUUID(), *originalTaskList, persistence.TaskListTypeActivity)
		}
		return nil, err
	}
	request.TaskList = originalTaskList
	c.provider.UpdatePartitionConfig(
		request.GetDomainUUID(),
		*request.TaskList,
		persistence.TaskListTypeActivity,
		resp.PartitionConfig,
	)
	return resp, nil
}

func (c *gatewayImpl) AddDecisionTask(
	ctx context.Context,
	request *types.AddDecisionTaskRequest,
) (*types.AddDecisionTaskResponse, error) {
	partition := c.loadBalancer.PickWritePartition(
		persistence.TaskListTypeDecision,
		request,
	)
	originalTaskList := request.TaskList
	request.TaskList = &types.TaskList{
		Name: partition,
		Kind: originalTaskList.Kind,
	}
	peer, err := c.peerResolver.FromTaskList(request.TaskList.GetName())
	if err != nil {
		return nil, err
	}
	resp, err := c.client.AddDecisionTask(ctx, request, yarpc.WithShardKey(peer))
	if err != nil {
		// ReadOnlyPartitionError indicates the partition is being drained - invalidate cache to force next request to route to root partition
		var readOnlyErr *types.ReadOnlyPartitionError
		if errors.As(err, &readOnlyErr) {
			c.provider.InvalidatePartitionCache(request.GetDomainUUID(), *originalTaskList, persistence.TaskListTypeDecision)
		}
		return nil, err
	}
	request.TaskList = originalTaskList
	c.provider.UpdatePartitionConfig(
		request.GetDomainUUID(),
		*request.TaskList,
		persistence.TaskListTypeDecision,
		resp.PartitionConfig,
	)
	return resp, nil
}

func (c *gatewayImpl) PollForActivityTask(
	ctx context.Context,
	request *types.MatchingPollForActivityTaskRequest,
) (*types.MatchingPollForActivityTaskResponse, error) {
	partition := c.loadBalancer.PickReadPartition(
		persistence.TaskListTypeActivity,
		request,
		request.GetIsolationGroup(),
	)
	originalTaskList := request.PollRequest.TaskList
	request.PollRequest.TaskList = &types.TaskList{
		Name: partition,
		Kind: originalTaskList.Kind,
	}
	peer, err := c.peerResolver.FromTaskList(request.PollRequest.TaskList.GetName())
	if err != nil {
		return nil, err
	}
	resp, err := c.client.PollForActivityTask(ctx, request, yarpc.WithShardKey(peer))
	if err != nil {
		return nil, cadence_errors.NewPeerHostnameError(err, peer)
	}

	request.PollRequest.TaskList = originalTaskList
	c.provider.UpdatePartitionConfig(
		request.GetDomainUUID(),
		*request.PollRequest.GetTaskList(),
		persistence.TaskListTypeActivity,
		resp.PartitionConfig,
	)
	c.loadBalancer.UpdateWeight(
		persistence.TaskListTypeActivity,
		request,
		partition,
		resp.LoadBalancerHints,
	)
	// caller needs to know the actual partition for cancelling long poll, so modify the request to pass this information
	request.PollRequest.TaskList.Name = partition
	return resp, nil
}

func (c *gatewayImpl) PollForDecisionTask(
	ctx context.Context,
	request *types.MatchingPollForDecisionTaskRequest,
) (*types.MatchingPollForDecisionTaskResponse, error) {
	partition := c.loadBalancer.PickReadPartition(
		persistence.TaskListTypeDecision,
		request,
		request.GetIsolationGroup(),
	)
	originalTaskList := request.PollRequest.TaskList
	request.PollRequest.TaskList = &types.TaskList{
		Name: partition,
		Kind: originalTaskList.Kind,
	}
	peer, err := c.peerResolver.FromTaskList(request.PollRequest.TaskList.GetName())
	if err != nil {
		return nil, err
	}
	resp, err := c.client.PollForDecisionTask(ctx, request, yarpc.WithShardKey(peer))
	if err != nil {
		return nil, cadence_errors.NewPeerHostnameError(err, peer)
	}
	request.PollRequest.TaskList = originalTaskList
	c.provider.UpdatePartitionConfig(
		request.GetDomainUUID(),
		*request.PollRequest.GetTaskList(),
		persistence.TaskListTypeDecision,
		resp.PartitionConfig,
	)
	c.loadBalancer.UpdateWeight(
		persistence.TaskListTypeDecision,
		request,
		partition,
		resp.LoadBalancerHints,
	)
	// caller needs to know the actual partition for cancelling long poll, so modify the request to pass this information
	request.PollRequest.TaskList.Name = partition
	return resp, nil
}

func (c *gatewayImpl) QueryWorkflow(
	ctx context.Context,
	request *types.MatchingQueryWorkflowRequest,
) (*types.MatchingQueryWorkflowResponse, error) {
	partition := c.loadBalancer.PickReadPartition(
		persistence.TaskListTypeDecision,
		request,
		"",
	)
	originalTaskList := request.TaskList
	request.TaskList = &types.TaskList{
		Name: partition,
		Kind: originalTaskList.Kind,
	}
	peer, err := c.peerResolver.FromTaskList(request.TaskList.GetName())
	if err != nil {
		return nil, err
	}
	resp, err := c.client.QueryWorkflow(ctx, request, yarpc.WithShardKey(peer))
	if err != nil {
		return nil, err
	}
	request.TaskList = originalTaskList
	c.provider.UpdatePartitionConfig(
		request.GetDomainUUID(),
		*request.TaskList,
		persistence.TaskListTypeDecision,
		resp.PartitionConfig,
	)
	return resp, nil
}

func (c *gatewayImpl) RespondQueryTaskCompleted(
	ctx context.Context,
	request *types.MatchingRespondQueryTaskCompletedRequest,
) error {
	peer, err := c.peerResolver.FromTaskList(request.TaskList.GetName())
	if err != nil {
		return err
	}
	err = c.client.RespondQueryTaskCompleted(ctx, request, yarpc.WithShardKey(peer))
	if err != nil {
		return err
	}
	return nil
}

func (c *gatewayImpl) CancelOutstandingPoll(
	ctx context.Context,
	request *types.CancelOutstandingPollRequest,
) error {
	peer, err := c.peerResolver.FromTaskList(request.TaskList.GetName())
	if err != nil {
		return err
	}
	err = c.client.CancelOutstandingPoll(ctx, request, yarpc.WithShardKey(peer))
	if err != nil {
		return err
	}
	return nil
}

func (c *gatewayImpl) DescribeTaskList(
	ctx context.Context,
	request *types.MatchingDescribeTaskListRequest,
) (*types.DescribeTaskListResponse, error) {
	peer, err := c.peerResolver.FromTaskList(request.DescRequest.TaskList.GetName())
	if err != nil {
		return nil, err
	}
	resp, err := c.client.DescribeTaskList(ctx, request, yarpc.WithShardKey(peer))
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *gatewayImpl) ListTaskListPartitions(
	ctx context.Context,
	request *types.MatchingListTaskListPartitionsRequest,
) (*types.ListTaskListPartitionsResponse, error) {
	peer, err := c.peerResolver.FromTaskList(request.TaskList.GetName())
	if err != nil {
		return nil, err
	}
	resp, err := c.client.ListTaskListPartitions(ctx, request, yarpc.WithShardKey(peer))
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *gatewayImpl) GetTaskListsByDomain(
	ctx context.Context,
	request *types.GetTaskListsByDomainRequest,
) (*types.GetTaskListsByDomainResponse, error) {
	peers, err := c.peerResolver.GetAllPeers()
	if err != nil {
		return nil, err
	}

	var futures []future.Future
	for _, peer := range peers {
		future, settable := future.NewFuture()
		settable.Set(c.client.GetTaskListsByDomain(ctx, request, yarpc.WithShardKey(peer)))
		futures = append(futures, future)
	}

	decisionTaskListMap := make(map[string]*types.DescribeTaskListResponse)
	activityTaskListMap := make(map[string]*types.DescribeTaskListResponse)
	for i, future := range futures {
		var resp *types.GetTaskListsByDomainResponse
		if err = future.Get(ctx, &resp); err != nil {
			return nil, cadence_errors.NewPeerHostnameError(err, peers[i])
		}
		for name, tl := range resp.GetDecisionTaskListMap() {
			if _, ok := decisionTaskListMap[name]; !ok {
				decisionTaskListMap[name] = tl
			} else {
				decisionTaskListMap[name].Pollers = append(decisionTaskListMap[name].Pollers, tl.GetPollers()...)
			}
		}
		for name, tl := range resp.GetActivityTaskListMap() {
			if _, ok := activityTaskListMap[name]; !ok {
				activityTaskListMap[name] = tl
			} else {
				activityTaskListMap[name].Pollers = append(activityTaskListMap[name].Pollers, tl.GetPollers()...)
			}
		}
	}

	return &types.GetTaskListsByDomainResponse{
		DecisionTaskListMap: decisionTaskListMap,
		ActivityTaskListMap: activityTaskListMap,
	}, nil
}

func (c *gatewayImpl) AcquireSemaphore(
	ctx context.Context,
	request *types.AcquireSemaphoreRequest,
) (*types.AcquireSemaphoreResponse, error) {
	peer, err := c.peerResolver.FromSemaphoreBucket(request.GetDomainUUID(), request.GetSemaphoreName(), int(request.GetBucket()))
	if err != nil {
		return nil, err
	}
	resp, err := c.client.AcquireSemaphore(ctx, request, yarpc.WithShardKey(peer))
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *gatewayImpl) ReleaseSemaphore(
	ctx context.Context,
	request *types.ReleaseSemaphoreRequest,
) (*types.ReleaseSemaphoreResponse, error) {
	peer, err := c.peerResolver.FromSemaphoreBucket(request.GetDomainUUID(), request.GetSemaphoreName(), int(request.GetBucket()))
	if err != nil {
		return nil, err
	}
	resp, err := c.client.ReleaseSemaphore(ctx, request, yarpc.WithShardKey(peer))
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *gatewayImpl) UpdateTaskListPartitionConfig(
	ctx context.Context,
	request *types.MatchingUpdateTaskListPartitionConfigRequest,
) (*types.MatchingUpdateTaskListPartitionConfigResponse, error) {
	peer, err := c.peerResolver.FromTaskList(request.TaskList.GetName())
	if err != nil {
		return nil, err
	}
	resp, err := c.client.UpdateTaskListPartitionConfig(ctx, request, yarpc.WithShardKey(peer))
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *gatewayImpl) RefreshTaskListPartitionConfig(
	ctx context.Context,
	request *types.MatchingRefreshTaskListPartitionConfigRequest,
) (*types.MatchingRefreshTaskListPartitionConfigResponse, error) {
	peer, err := c.peerResolver.FromTaskList(request.TaskList.GetName())
	if err != nil {
		return nil, err
	}
	resp, err := c.client.RefreshTaskListPartitionConfig(ctx, request, yarpc.WithShardKey(peer))
	if err != nil {
		return nil, err
	}
	return resp, nil
}
