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

	"go.uber.org/yarpc"

	"github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/future"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

var _ Client = (*clientImpl)(nil)

type clientImpl struct {
	client       Client
	peerResolver PeerResolver
	loadBalancer LoadBalancer
	provider     PartitionConfigProvider
}

// NewClient creates a new history service TChannel client
func NewClient(
	client Client,
	peerResolver PeerResolver,
	lb LoadBalancer,
	provider PartitionConfigProvider,
) Client {
	return &clientImpl{
		client:       client,
		peerResolver: peerResolver,
		loadBalancer: lb,
		provider:     provider,
	}
}

func (c *clientImpl) AddActivityTask(
	ctx context.Context,
	request *types.AddActivityTaskRequest,
	opts ...yarpc.CallOption,
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
	resp, err := c.client.AddActivityTask(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	if err != nil {
		// ReadOnlyPartitionError indicates the partition is being drained - invalidate cache to force next request to route to root partition
		if _, ok := err.(*types.ReadOnlyPartitionError); ok {
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

func (c *clientImpl) AddDecisionTask(
	ctx context.Context,
	request *types.AddDecisionTaskRequest,
	opts ...yarpc.CallOption,
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
	resp, err := c.client.AddDecisionTask(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	if err != nil {
		// ReadOnlyPartitionError indicates the partition is being drained - invalidate cache to force next request to route to root partition
		if _, ok := err.(*types.ReadOnlyPartitionError); ok {
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

func (c *clientImpl) PollForActivityTask(
	ctx context.Context,
	request *types.MatchingPollForActivityTaskRequest,
	opts ...yarpc.CallOption,
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
	resp, err := c.client.PollForActivityTask(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	if err != nil {
		return nil, errors.NewPeerHostnameError(err, peer)
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

func (c *clientImpl) PollForDecisionTask(
	ctx context.Context,
	request *types.MatchingPollForDecisionTaskRequest,
	opts ...yarpc.CallOption,
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
	resp, err := c.client.PollForDecisionTask(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	if err != nil {
		return nil, errors.NewPeerHostnameError(err, peer)
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

func (c *clientImpl) QueryWorkflow(
	ctx context.Context,
	request *types.MatchingQueryWorkflowRequest,
	opts ...yarpc.CallOption,
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
	resp, err := c.client.QueryWorkflow(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
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

func (c *clientImpl) RespondQueryTaskCompleted(
	ctx context.Context,
	request *types.MatchingRespondQueryTaskCompletedRequest,
	opts ...yarpc.CallOption,
) error {
	peer, err := c.peerResolver.FromTaskList(request.TaskList.GetName())
	if err != nil {
		return err
	}
	err = c.client.RespondQueryTaskCompleted(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	if err != nil {
		return err
	}
	return nil
}

func (c *clientImpl) CancelOutstandingPoll(
	ctx context.Context,
	request *types.CancelOutstandingPollRequest,
	opts ...yarpc.CallOption,
) error {
	peer, err := c.peerResolver.FromTaskList(request.TaskList.GetName())
	if err != nil {
		return err
	}
	err = c.client.CancelOutstandingPoll(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	if err != nil {
		return err
	}
	return nil
}

func (c *clientImpl) DescribeTaskList(
	ctx context.Context,
	request *types.MatchingDescribeTaskListRequest,
	opts ...yarpc.CallOption,
) (*types.DescribeTaskListResponse, error) {
	peer, err := c.peerResolver.FromTaskList(request.DescRequest.TaskList.GetName())
	if err != nil {
		return nil, err
	}
	resp, err := c.client.DescribeTaskList(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *clientImpl) ListTaskListPartitions(
	ctx context.Context,
	request *types.MatchingListTaskListPartitionsRequest,
	opts ...yarpc.CallOption,
) (*types.ListTaskListPartitionsResponse, error) {
	peer, err := c.peerResolver.FromTaskList(request.TaskList.GetName())
	if err != nil {
		return nil, err
	}
	resp, err := c.client.ListTaskListPartitions(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *clientImpl) GetTaskListsByDomain(
	ctx context.Context,
	request *types.GetTaskListsByDomainRequest,
	opts ...yarpc.CallOption,
) (*types.GetTaskListsByDomainResponse, error) {
	peers, err := c.peerResolver.GetAllPeers()
	if err != nil {
		return nil, err
	}

	var futures []future.Future
	for _, peer := range peers {
		future, settable := future.NewFuture()
		settable.Set(c.client.GetTaskListsByDomain(ctx, request, append(opts, yarpc.WithShardKey(peer))...))
		futures = append(futures, future)
	}

	decisionTaskListMap := make(map[string]*types.DescribeTaskListResponse)
	activityTaskListMap := make(map[string]*types.DescribeTaskListResponse)
	for i, future := range futures {
		var resp *types.GetTaskListsByDomainResponse
		if err = future.Get(ctx, &resp); err != nil {
			return nil, errors.NewPeerHostnameError(err, peers[i])
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

func (c *clientImpl) UpdateTaskListPartitionConfig(
	ctx context.Context,
	request *types.MatchingUpdateTaskListPartitionConfigRequest,
	opts ...yarpc.CallOption,
) (*types.MatchingUpdateTaskListPartitionConfigResponse, error) {
	peer, err := c.peerResolver.FromTaskList(request.TaskList.GetName())
	if err != nil {
		return nil, err
	}
	resp, err := c.client.UpdateTaskListPartitionConfig(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *clientImpl) RefreshTaskListPartitionConfig(
	ctx context.Context,
	request *types.MatchingRefreshTaskListPartitionConfigRequest,
	opts ...yarpc.CallOption,
) (*types.MatchingRefreshTaskListPartitionConfigResponse, error) {
	peer, err := c.peerResolver.FromTaskList(request.TaskList.GetName())
	if err != nil {
		return nil, err
	}
	resp, err := c.client.RefreshTaskListPartitionConfig(ctx, request, append(opts, yarpc.WithShardKey(peer))...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
