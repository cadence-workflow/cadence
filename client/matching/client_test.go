// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package matching

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

const (
	_testDomainUUID = "123"
	_testDomain     = "test-domain"
	_testTaskList   = "test-tasklist"
	_testPartition  = "test-partition"
)

func TestNewClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := NewMockClient(ctrl)
	peerResolver := NewMockPeerResolver(ctrl)
	loadbalancer := NewMockLoadBalancer(ctrl)
	provider := NewMockPartitionConfigProvider(ctrl)

	c := NewClient(client, peerResolver, loadbalancer, provider)
	assert.NotNil(t, c)
}

func TestClient_withoutResponse(t *testing.T) {
	tests := []struct {
		name      string
		op        func(Client) error
		mock      func(*MockPeerResolver, *MockLoadBalancer, *MockClient)
		wantError bool
	}{
		{
			name: "RespondQueryTaskCompleted",
			op: func(c Client) error {
				return c.RespondQueryTaskCompleted(context.Background(), testRespondQueryTaskCompletedRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", nil)
				c.EXPECT().RespondQueryTaskCompleted(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(nil)
			},
		},
		{
			name: "RespondQueryTaskCompleted - Error in resolving peer",
			op: func(c Client) error {
				return c.RespondQueryTaskCompleted(context.Background(), testRespondQueryTaskCompletedRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", assert.AnError)
			},
			wantError: true,
		},
		{
			name: "RespondQueryTaskCompleted - Error while responding to completion of QueryTask",
			op: func(c Client) error {
				return c.RespondQueryTaskCompleted(context.Background(), testRespondQueryTaskCompletedRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", nil)
				c.EXPECT().RespondQueryTaskCompleted(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(assert.AnError)
			},
			wantError: true,
		},
		{
			name: "CancelOutstandingPoll",
			op: func(c Client) error {
				return c.CancelOutstandingPoll(context.Background(), testCancelOutstandingPollRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", nil)
				c.EXPECT().CancelOutstandingPoll(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(nil)
			},
		},
		{
			name: "CancelOutstandingPoll - Error in resolving peer",
			op: func(c Client) error {
				return c.CancelOutstandingPoll(context.Background(), testCancelOutstandingPollRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", assert.AnError)
			},
			wantError: true,
		},
		{
			name: "CancelOutstandingPoll - Error while cancelling outstanding poll",
			op: func(c Client) error {
				return c.CancelOutstandingPoll(context.Background(), testCancelOutstandingPollRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", nil)
				c.EXPECT().CancelOutstandingPoll(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(assert.AnError)
			},
			wantError: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			// setting up client
			ctrl := gomock.NewController(t)
			client := NewMockClient(ctrl)
			peerResolverMock := NewMockPeerResolver(ctrl)
			loadbalancerMock := NewMockLoadBalancer(ctrl)
			providerMock := NewMockPartitionConfigProvider(ctrl)
			tt.mock(peerResolverMock, loadbalancerMock, client)
			c := NewClient(client, peerResolverMock, loadbalancerMock, providerMock)

			err := tt.op(c)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

		})
	}
}

func TestClient_withResponse(t *testing.T) {
	tests := []struct {
		name      string
		op        func(Client) (any, error)
		mock      func(*MockPeerResolver, *MockLoadBalancer, *MockClient, *MockPartitionConfigProvider)
		want      any
		wantError bool
	}{
		{
			name: "AddActivityTask",
			op: func(c Client) (any, error) {
				return c.AddActivityTask(context.Background(), testAddActivityTaskRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickWritePartition(persistence.TaskListTypeActivity, testAddActivityTaskRequest()).Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", nil)
				c.EXPECT().AddActivityTask(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(&types.AddActivityTaskResponse{}, nil)
				mp.EXPECT().UpdatePartitionConfig(_testDomainUUID, types.TaskList{Name: _testTaskList}, persistence.TaskListTypeActivity, nil)
			},
			want: &types.AddActivityTaskResponse{},
		},
		{
			name: "AddActivityTask - Error in resolving peer",
			op: func(c Client) (any, error) {
				return c.AddActivityTask(context.Background(), testAddActivityTaskRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickWritePartition(persistence.TaskListTypeActivity, testAddActivityTaskRequest()).Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", assert.AnError)
			},
			wantError: true,
		},
		{
			name: "AddActivityTask - Error while adding activity task",
			op: func(c Client) (any, error) {
				return c.AddActivityTask(context.Background(), testAddActivityTaskRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickWritePartition(persistence.TaskListTypeActivity, testAddActivityTaskRequest()).Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", nil)
				c.EXPECT().AddActivityTask(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(nil, assert.AnError)
			},
			wantError: true,
		},
		{
			name: "AddDecisionTask",
			op: func(c Client) (any, error) {
				return c.AddDecisionTask(context.Background(), testAddDecisionTaskRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickWritePartition(persistence.TaskListTypeDecision, testAddDecisionTaskRequest()).Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", nil)
				c.EXPECT().AddDecisionTask(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(&types.AddDecisionTaskResponse{}, nil)
				mp.EXPECT().UpdatePartitionConfig(_testDomainUUID, types.TaskList{Name: _testTaskList}, persistence.TaskListTypeDecision, nil)
			},
			want: &types.AddDecisionTaskResponse{},
		},
		{
			name: "AddDecisionTask - Error in resolving peer",
			op: func(c Client) (any, error) {
				return c.AddDecisionTask(context.Background(), testAddDecisionTaskRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickWritePartition(persistence.TaskListTypeDecision, testAddDecisionTaskRequest()).Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", assert.AnError)
			},
			wantError: true,
		},
		{
			name: "AddDecisionTask - Error while adding decision task",
			op: func(c Client) (any, error) {
				return c.AddDecisionTask(context.Background(), testAddDecisionTaskRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickWritePartition(persistence.TaskListTypeDecision, testAddDecisionTaskRequest()).Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", nil)
				c.EXPECT().AddDecisionTask(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(nil, assert.AnError)
			},
			wantError: true,
		},
		{
			name: "PollForActivityTask",
			op: func(c Client) (any, error) {
				return c.PollForActivityTask(context.Background(), testMatchingPollForActivityTaskRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickReadPartition(persistence.TaskListTypeActivity, testMatchingPollForActivityTaskRequest(), "").Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", nil)
				c.EXPECT().PollForActivityTask(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(&types.MatchingPollForActivityTaskResponse{}, nil)
				mp.EXPECT().UpdatePartitionConfig(_testDomainUUID, types.TaskList{Name: _testTaskList}, persistence.TaskListTypeActivity, nil)
				balancer.EXPECT().UpdateWeight(persistence.TaskListTypeActivity, testMatchingPollForActivityTaskRequest(), _testPartition, nil)
			},
			want: &types.MatchingPollForActivityTaskResponse{},
		},
		{
			name: "PollForActivityTask - Error in resolving peer",
			op: func(c Client) (any, error) {
				return c.PollForActivityTask(context.Background(), testMatchingPollForActivityTaskRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickReadPartition(persistence.TaskListTypeActivity, testMatchingPollForActivityTaskRequest(), "").Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "PollForActivityTask - Error while polling for ActivityTask",
			op: func(c Client) (any, error) {
				return c.PollForActivityTask(context.Background(), testMatchingPollForActivityTaskRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickReadPartition(persistence.TaskListTypeActivity, testMatchingPollForActivityTaskRequest(), "").Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", nil)
				c.EXPECT().PollForActivityTask(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(nil, assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "PollForDecisionTask",
			op: func(c Client) (any, error) {
				return c.PollForDecisionTask(context.Background(), testMatchingPollForDecisionTaskRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickReadPartition(persistence.TaskListTypeDecision, testMatchingPollForDecisionTaskRequest(), "").Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", nil)
				c.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(&types.MatchingPollForDecisionTaskResponse{}, nil)
				mp.EXPECT().UpdatePartitionConfig(_testDomainUUID, types.TaskList{Name: _testTaskList}, persistence.TaskListTypeDecision, nil)
				balancer.EXPECT().UpdateWeight(persistence.TaskListTypeDecision, testMatchingPollForDecisionTaskRequest(), _testPartition, nil)
			},
			want: &types.MatchingPollForDecisionTaskResponse{},
		},
		{
			name: "PollForDecisionTask - Error in resolving peer",
			op: func(c Client) (any, error) {
				return c.PollForDecisionTask(context.Background(), testMatchingPollForDecisionTaskRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickReadPartition(persistence.TaskListTypeDecision, testMatchingPollForDecisionTaskRequest(), "").Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "PollForDecisionTask - Error while polling for DecisionTask",
			op: func(c Client) (any, error) {
				return c.PollForDecisionTask(context.Background(), testMatchingPollForDecisionTaskRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickReadPartition(persistence.TaskListTypeDecision, testMatchingPollForDecisionTaskRequest(), "").Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", nil)
				c.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(nil, assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "QueryWorkflow",
			op: func(c Client) (any, error) {
				return c.QueryWorkflow(context.Background(), testMatchingQueryWorkflowRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickReadPartition(persistence.TaskListTypeDecision, testMatchingQueryWorkflowRequest(), "").Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", nil)
				c.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(&types.QueryWorkflowResponse{}, nil)
			},
			want: &types.QueryWorkflowResponse{},
		},
		{
			name: "QueryWorkflow - Error in resolving peer",
			op: func(c Client) (any, error) {
				return c.QueryWorkflow(context.Background(), testMatchingQueryWorkflowRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickReadPartition(persistence.TaskListTypeDecision, testMatchingQueryWorkflowRequest(), "").Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "QueryWorkflow - Error while querying workflow",
			op: func(c Client) (any, error) {
				return c.QueryWorkflow(context.Background(), testMatchingQueryWorkflowRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				balancer.EXPECT().PickReadPartition(persistence.TaskListTypeDecision, testMatchingQueryWorkflowRequest(), "").Return(_testPartition)
				p.EXPECT().FromTaskList(_testPartition).Return("peer0", nil)
				c.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(nil, assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "DescribeTaskList",
			op: func(c Client) (any, error) {
				return c.DescribeTaskList(context.Background(), testMatchingDescribeTaskListRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", nil)
				c.EXPECT().DescribeTaskList(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(&types.DescribeTaskListResponse{}, nil)
			},
			want: &types.DescribeTaskListResponse{},
		},
		{
			name: "DescribeTaskList - Error in resolving peer",
			op: func(c Client) (any, error) {
				return c.DescribeTaskList(context.Background(), testMatchingDescribeTaskListRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "DescribeTaskList - Error while describing tasklist",
			op: func(c Client) (any, error) {
				return c.DescribeTaskList(context.Background(), testMatchingDescribeTaskListRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", nil)
				c.EXPECT().DescribeTaskList(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(nil, assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "ListTaskListPartitions",
			op: func(c Client) (any, error) {
				return c.ListTaskListPartitions(context.Background(), testMatchingListTaskListPartitionsRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", nil)
				c.EXPECT().ListTaskListPartitions(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(&types.ListTaskListPartitionsResponse{}, nil)
			},
			want: &types.ListTaskListPartitionsResponse{},
		},
		{
			name: "ListTaskListPartitions - Error in resolving peer",
			op: func(c Client) (any, error) {
				return c.ListTaskListPartitions(context.Background(), testMatchingListTaskListPartitionsRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "ListTaskListPartitions - Error while listing tasklist partitions",
			op: func(c Client) (any, error) {
				return c.ListTaskListPartitions(context.Background(), testMatchingListTaskListPartitionsRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", nil)
				c.EXPECT().ListTaskListPartitions(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(nil, assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "GetTaskListsByDomain",
			op: func(c Client) (any, error) {
				return c.GetTaskListsByDomain(context.Background(), testGetTaskListsByDomainRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				p.EXPECT().GetAllPeers().Return([]string{"peer0", "peer1"}, nil)
				c.EXPECT().GetTaskListsByDomain(gomock.Any(), testGetTaskListsByDomainRequest(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(&types.GetTaskListsByDomainResponse{}, nil)
				c.EXPECT().GetTaskListsByDomain(gomock.Any(), testGetTaskListsByDomainRequest(), []yarpc.CallOption{yarpc.WithShardKey("peer1")}).Return(&types.GetTaskListsByDomainResponse{}, nil)
			},
			want: &types.GetTaskListsByDomainResponse{
				DecisionTaskListMap: make(map[string]*types.DescribeTaskListResponse),
				ActivityTaskListMap: make(map[string]*types.DescribeTaskListResponse),
			},
		},
		{
			name: "GetTaskListsByDomain - Error in resolving peer",
			op: func(c Client) (any, error) {
				return c.GetTaskListsByDomain(context.Background(), testGetTaskListsByDomainRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				p.EXPECT().GetAllPeers().Return([]string{"peer0", "peer1"}, assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "UpdateTaskListPartitionConfig",
			op: func(c Client) (any, error) {
				return c.UpdateTaskListPartitionConfig(context.Background(), testMatchingUpdateTaskListPartitionConfigRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", nil)
				c.EXPECT().UpdateTaskListPartitionConfig(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(&types.MatchingUpdateTaskListPartitionConfigResponse{}, nil)
			},
			want: &types.MatchingUpdateTaskListPartitionConfigResponse{},
		},
		{
			name: "UpdateTaskListPartitionConfig - Error in resolving peer",
			op: func(c Client) (any, error) {
				return c.UpdateTaskListPartitionConfig(context.Background(), testMatchingUpdateTaskListPartitionConfigRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "UpdateTaskListPartitionConfig - Error while listing tasklist partitions",
			op: func(c Client) (any, error) {
				return c.UpdateTaskListPartitionConfig(context.Background(), testMatchingUpdateTaskListPartitionConfigRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", nil)
				c.EXPECT().UpdateTaskListPartitionConfig(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(nil, assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "RefreshTaskListPartitionConfig",
			op: func(c Client) (any, error) {
				return c.RefreshTaskListPartitionConfig(context.Background(), testMatchingRefreshTaskListPartitionConfigRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", nil)
				c.EXPECT().RefreshTaskListPartitionConfig(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(&types.MatchingRefreshTaskListPartitionConfigResponse{}, nil)
			},
			want: &types.MatchingRefreshTaskListPartitionConfigResponse{},
		},
		{
			name: "RefreshTaskListPartitionConfig - Error in resolving peer",
			op: func(c Client) (any, error) {
				return c.RefreshTaskListPartitionConfig(context.Background(), testMatchingRefreshTaskListPartitionConfigRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "RefreshTaskListPartitionConfig - Error while listing tasklist partitions",
			op: func(c Client) (any, error) {
				return c.RefreshTaskListPartitionConfig(context.Background(), testMatchingRefreshTaskListPartitionConfigRequest())
			},
			mock: func(p *MockPeerResolver, balancer *MockLoadBalancer, c *MockClient, mp *MockPartitionConfigProvider) {
				p.EXPECT().FromTaskList(_testTaskList).Return("peer0", nil)
				c.EXPECT().RefreshTaskListPartitionConfig(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).Return(nil, assert.AnError)
			},
			want:      nil,
			wantError: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			// setting up client
			ctrl := gomock.NewController(t)
			client := NewMockClient(ctrl)
			peerResolverMock := NewMockPeerResolver(ctrl)
			loadbalancerMock := NewMockLoadBalancer(ctrl)
			providerMock := NewMockPartitionConfigProvider(ctrl)
			tt.mock(peerResolverMock, loadbalancerMock, client, providerMock)
			c := NewClient(client, peerResolverMock, loadbalancerMock, providerMock)

			res, err := tt.op(c)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, res)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, res)
			}
		})
	}
}

func testAddActivityTaskRequest() *types.AddActivityTaskRequest {
	return &types.AddActivityTaskRequest{
		DomainUUID: _testDomainUUID,
		TaskList:   &types.TaskList{Name: _testTaskList},
	}
}

func testAddDecisionTaskRequest() *types.AddDecisionTaskRequest {
	return &types.AddDecisionTaskRequest{
		DomainUUID: _testDomainUUID,
		TaskList:   &types.TaskList{Name: _testTaskList},
	}
}

func testRespondQueryTaskCompletedRequest() *types.MatchingRespondQueryTaskCompletedRequest {
	return &types.MatchingRespondQueryTaskCompletedRequest{
		DomainUUID: _testDomainUUID,
		TaskList:   &types.TaskList{Name: _testTaskList},
	}
}

func testCancelOutstandingPollRequest() *types.CancelOutstandingPollRequest {
	return &types.CancelOutstandingPollRequest{
		DomainUUID: _testDomainUUID,
		TaskList:   &types.TaskList{Name: _testTaskList},
	}
}

func testMatchingPollForActivityTaskRequest() *types.MatchingPollForActivityTaskRequest {
	return &types.MatchingPollForActivityTaskRequest{
		DomainUUID:  _testDomainUUID,
		PollRequest: &types.PollForActivityTaskRequest{TaskList: &types.TaskList{Name: _testTaskList}},
	}
}

func testMatchingPollForDecisionTaskRequest() *types.MatchingPollForDecisionTaskRequest {
	return &types.MatchingPollForDecisionTaskRequest{
		DomainUUID:  _testDomainUUID,
		PollRequest: &types.PollForDecisionTaskRequest{TaskList: &types.TaskList{Name: _testTaskList}},
	}
}

func testMatchingQueryWorkflowRequest() *types.MatchingQueryWorkflowRequest {
	return &types.MatchingQueryWorkflowRequest{
		DomainUUID:   _testDomainUUID,
		TaskList:     &types.TaskList{Name: _testTaskList},
		QueryRequest: &types.QueryWorkflowRequest{Domain: _testDomain},
	}
}

func testMatchingDescribeTaskListRequest() *types.MatchingDescribeTaskListRequest {
	return &types.MatchingDescribeTaskListRequest{
		DomainUUID:  _testDomainUUID,
		DescRequest: &types.DescribeTaskListRequest{TaskList: &types.TaskList{Name: _testTaskList}},
	}
}

func testMatchingListTaskListPartitionsRequest() *types.MatchingListTaskListPartitionsRequest {
	return &types.MatchingListTaskListPartitionsRequest{
		Domain:   _testDomainUUID,
		TaskList: &types.TaskList{Name: _testTaskList},
	}
}

func testGetTaskListsByDomainRequest() *types.GetTaskListsByDomainRequest {
	return &types.GetTaskListsByDomainRequest{
		Domain: _testDomain,
	}
}

func testMatchingUpdateTaskListPartitionConfigRequest() *types.MatchingUpdateTaskListPartitionConfigRequest {
	return &types.MatchingUpdateTaskListPartitionConfigRequest{
		DomainUUID: _testDomainUUID,
		TaskList:   &types.TaskList{Name: _testTaskList},
		PartitionConfig: &types.TaskListPartitionConfig{
			Version: 1,
			ReadPartitions: map[int]*types.TaskListPartition{
				0: {},
				1: {},
				2: {
					IsolationGroups: []string{"foo"},
				},
			},
			WritePartitions: map[int]*types.TaskListPartition{
				0: {},
				1: {
					IsolationGroups: []string{"bar"},
				},
			},
		},
	}
}

func testMatchingRefreshTaskListPartitionConfigRequest() *types.MatchingRefreshTaskListPartitionConfigRequest {
	return &types.MatchingRefreshTaskListPartitionConfigRequest{
		DomainUUID: _testDomainUUID,
		TaskList:   &types.TaskList{Name: _testTaskList},
		PartitionConfig: &types.TaskListPartitionConfig{
			Version: 1,
			ReadPartitions: map[int]*types.TaskListPartition{
				0: {},
				1: {},
				2: {
					IsolationGroups: []string{"foo"},
				},
			},
			WritePartitions: map[int]*types.TaskListPartition{
				0: {},
				1: {
					IsolationGroups: []string{"bar"},
				},
			},
		},
	}
}
