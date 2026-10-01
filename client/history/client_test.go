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

package history

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
)

func TestNewClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	numberOfShards := 10
	rpcMaxSizeInBytes := 1024
	client := NewMockClient(ctrl)
	peerResolver := NewMockPeerResolver(ctrl)
	logger := log.NewNoop()

	c := NewClient(numberOfShards, rpcMaxSizeInBytes, client, peerResolver, logger)
	assert.NotNil(t, c)
}

func TestClient_withResponse(t *testing.T) {
	tests := []struct {
		name      string
		op        func(Client) (any, error)
		mock      func(*MockPeerResolver, *MockClient)
		want      any
		wantError bool
	}{
		{
			name: "StartWorkflowExecution",
			op: func(c Client) (any, error) {
				return c.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
					StartRequest: &types.StartWorkflowExecutionRequest{
						WorkflowID: "test-workflow",
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
			},
			want: &types.StartWorkflowExecutionResponse{},
		},
		{
			name: "GetMutableState",
			op: func(c Client) (any, error) {
				return c.GetMutableState(context.Background(), &types.GetMutableStateRequest{
					Execution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().GetMutableState(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.GetMutableStateResponse{}, nil).Times(1)
			},
			want: &types.GetMutableStateResponse{},
		},
		{
			name: "PollMutableState",
			op: func(c Client) (any, error) {
				return c.PollMutableState(context.Background(), &types.PollMutableStateRequest{
					Execution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().PollMutableState(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.PollMutableStateResponse{}, nil).Times(1)
			},
			want: &types.PollMutableStateResponse{},
		},
		{
			name: "ResetWorkflowExecution",
			op: func(c Client) (any, error) {
				return c.ResetWorkflowExecution(context.Background(), &types.HistoryResetWorkflowExecutionRequest{
					ResetRequest: &types.ResetWorkflowExecutionRequest{
						WorkflowExecution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().ResetWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.ResetWorkflowExecutionResponse{}, nil).Times(1)
			},
			want: &types.ResetWorkflowExecutionResponse{},
		},
		{
			name: "DescribeWorkflowExecution",
			op: func(c Client) (any, error) {
				return c.DescribeWorkflowExecution(context.Background(), &types.HistoryDescribeWorkflowExecutionRequest{
					Request: &types.DescribeWorkflowExecutionRequest{
						Execution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.DescribeWorkflowExecutionResponse{}, nil).Times(1)
			},
			want: &types.DescribeWorkflowExecutionResponse{},
		},
		{
			name: "RecordActivityTaskHeartbeat",
			op: func(c Client) (any, error) {
				return c.RecordActivityTaskHeartbeat(context.Background(), &types.HistoryRecordActivityTaskHeartbeatRequest{
					HeartbeatRequest: &types.RecordActivityTaskHeartbeatRequest{
						TaskToken: []byte(`{"workflowId": "test-workflow"}`),
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RecordActivityTaskHeartbeat(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.RecordActivityTaskHeartbeatResponse{}, nil).Times(1)
			},
			want: &types.RecordActivityTaskHeartbeatResponse{},
		},
		{
			name: "RecordActivityTaskStarted",
			op: func(c Client) (any, error) {
				return c.RecordActivityTaskStarted(context.Background(), &types.RecordActivityTaskStartedRequest{
					WorkflowExecution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RecordActivityTaskStarted(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.RecordActivityTaskStartedResponse{}, nil).Times(1)
			},
			want: &types.RecordActivityTaskStartedResponse{},
		},
		{
			name: "RecordDecisionTaskStarted",
			op: func(c Client) (any, error) {
				return c.RecordDecisionTaskStarted(context.Background(), &types.RecordDecisionTaskStartedRequest{
					WorkflowExecution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RecordDecisionTaskStarted(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.RecordDecisionTaskStartedResponse{}, nil).Times(1)
			},
			want: &types.RecordDecisionTaskStartedResponse{},
		},
		{
			name: "GetReplicationMessages",
			op: func(c Client) (any, error) {
				return c.GetReplicationMessages(context.Background(), &types.GetReplicationMessagesRequest{
					Tokens: []*types.ReplicationToken{
						{
							ShardID: 100,
						},
						{
							ShardID: 101,
						},
						{
							ShardID: 102,
						},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromShardID(100).Return("test-peer-0", nil).Times(1)
				p.EXPECT().FromShardID(101).Return("test-peer-1", nil).Times(1)
				p.EXPECT().FromShardID(102).Return("test-peer-2", nil).Times(1)
				c.EXPECT().GetReplicationMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-0")}).
					Return(&types.GetReplicationMessagesResponse{
						MessagesByShard: map[int32]*types.ReplicationMessages{100: {}},
					}, nil).Times(1)
				c.EXPECT().GetReplicationMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-1")}).
					Return(&types.GetReplicationMessagesResponse{
						MessagesByShard: map[int32]*types.ReplicationMessages{101: {}},
					}, nil).Times(1)
				c.EXPECT().GetReplicationMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-2")}).
					Return(&types.GetReplicationMessagesResponse{
						MessagesByShard: map[int32]*types.ReplicationMessages{102: {}},
					}, nil).Times(1)
			},
			want: &types.GetReplicationMessagesResponse{
				MessagesByShard: map[int32]*types.ReplicationMessages{
					100: {},
					101: {},
					102: {},
				},
			},
		},
		{
			name: "GetDLQReplicationMessages",
			op: func(c Client) (any, error) {
				return c.GetDLQReplicationMessages(context.Background(), &types.GetDLQReplicationMessagesRequest{
					TaskInfos: []*types.ReplicationTaskInfo{
						{WorkflowID: "test-workflow"},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().GetDLQReplicationMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.GetDLQReplicationMessagesResponse{}, nil).Times(1)
			},
			want: &types.GetDLQReplicationMessagesResponse{},
		},
		{
			name: "ReadDLQMessages",
			op: func(c Client) (any, error) {
				return c.ReadDLQMessages(context.Background(), &types.ReadDLQMessagesRequest{
					ShardID: 123,
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromShardID(123).Return("test-peer", nil).Times(1)
				c.EXPECT().ReadDLQMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.ReadDLQMessagesResponse{}, nil).Times(1)
			},
			want: &types.ReadDLQMessagesResponse{},
		},
		{
			name: "MergeDLQMessages",
			op: func(c Client) (any, error) {
				return c.MergeDLQMessages(context.Background(), &types.MergeDLQMessagesRequest{
					ShardID: 123,
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromShardID(123).Return("test-peer", nil).Times(1)
				c.EXPECT().MergeDLQMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.MergeDLQMessagesResponse{}, nil).Times(1)
			},
			want: &types.MergeDLQMessagesResponse{},
		},
		{
			name: "GetFailoverInfo",
			op: func(c Client) (any, error) {
				return c.GetFailoverInfo(context.Background(), &types.GetFailoverInfoRequest{
					DomainID: "test-domain",
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromDomainID("test-domain").Return("test-peer", nil).Times(1)
				c.EXPECT().GetFailoverInfo(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.GetFailoverInfoResponse{}, nil).Times(1)
			},
			want: &types.GetFailoverInfoResponse{},
		},
		{
			name: "DescribeHistoryHost by host address",
			op: func(c Client) (any, error) {
				return c.DescribeHistoryHost(context.Background(), &types.DescribeHistoryHostRequest{
					HostAddress: common.StringPtr("test-host"),
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromHostAddress("test-host").Return("test-peer", nil).Times(1)
				c.EXPECT().DescribeHistoryHost(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.DescribeHistoryHostResponse{}, nil).Times(1)
			},
			want: &types.DescribeHistoryHostResponse{},
		},
		{
			name: "DescribeHistoryHost by workflow id",
			op: func(c Client) (any, error) {
				return c.DescribeHistoryHost(context.Background(), &types.DescribeHistoryHostRequest{
					ExecutionForHost: &types.WorkflowExecution{WorkflowID: "test-workflow"},
					HostAddress:      common.StringPtr("test-host"),
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().DescribeHistoryHost(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.DescribeHistoryHostResponse{}, nil).Times(1)
			},
			want: &types.DescribeHistoryHostResponse{},
		},
		{
			name: "DescribeHistoryHost by shard id",
			op: func(c Client) (any, error) {
				return c.DescribeHistoryHost(context.Background(), &types.DescribeHistoryHostRequest{
					ShardIDForHost:   common.Int32Ptr(123),
					ExecutionForHost: &types.WorkflowExecution{WorkflowID: "test-workflow"},
					HostAddress:      common.StringPtr("test-host"),
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromShardID(123).Return("test-peer", nil).Times(1)
				c.EXPECT().DescribeHistoryHost(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.DescribeHistoryHostResponse{}, nil).Times(1)
			},
			want: &types.DescribeHistoryHostResponse{},
		},
		{
			name: "DescribeMutableState",
			op: func(c Client) (any, error) {
				return c.DescribeMutableState(context.Background(), &types.DescribeMutableStateRequest{
					Execution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().DescribeMutableState(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.DescribeMutableStateResponse{}, nil).Times(1)
			},
			want: &types.DescribeMutableStateResponse{},
		},
		{
			name: "DescribeQueue",
			op: func(c Client) (any, error) {
				return c.DescribeQueue(context.Background(), &types.DescribeQueueRequest{
					ShardID: 123,
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromShardID(123).Return("test-peer", nil).Times(1)
				c.EXPECT().DescribeQueue(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.DescribeQueueResponse{}, nil).Times(1)
			},
			want: &types.DescribeQueueResponse{},
		},
		{
			name: "CountDLQMessages",
			op: func(c Client) (any, error) {
				return c.CountDLQMessages(context.Background(), &types.CountDLQMessagesRequest{})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().GetAllPeers().Return([]string{"test-peer-0", "test-peer-1"}, nil).Times(1)
				c.EXPECT().CountDLQMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-0")}).
					Return(&types.HistoryCountDLQMessagesResponse{
						Entries: map[types.HistoryDLQCountKey]int64{
							{ShardID: 1}: 1,
						},
					}, nil).Times(1)
				c.EXPECT().CountDLQMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-1")}).
					Return(&types.HistoryCountDLQMessagesResponse{
						Entries: map[types.HistoryDLQCountKey]int64{
							{ShardID: 2}: 2,
						},
					}, nil).Times(1)
			},
			want: &types.HistoryCountDLQMessagesResponse{
				Entries: map[types.HistoryDLQCountKey]int64{
					{ShardID: 1}: 1,
					{ShardID: 2}: 2,
				},
			},
		},
		{
			name: "QueryWorkflow",
			op: func(c Client) (any, error) {
				return c.QueryWorkflow(context.Background(), &types.HistoryQueryWorkflowRequest{
					Request: &types.QueryWorkflowRequest{
						Execution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.HistoryQueryWorkflowResponse{}, nil).Times(1)
			},
			want: &types.HistoryQueryWorkflowResponse{},
		},
		{
			name: "ResetStickyTaskList",
			op: func(c Client) (any, error) {
				return c.ResetStickyTaskList(context.Background(), &types.HistoryResetStickyTaskListRequest{
					Execution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().ResetStickyTaskList(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.HistoryResetStickyTaskListResponse{}, nil).Times(1)
			},
			want: &types.HistoryResetStickyTaskListResponse{},
		},
		{
			name: "RespondDecisionTaskCompleted",
			op: func(c Client) (any, error) {
				return c.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
					CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
						TaskToken: []byte(`{"workflowId": "test-workflow"}`),
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RespondDecisionTaskCompleted(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.HistoryRespondDecisionTaskCompletedResponse{}, nil).Times(1)
			},
			want: &types.HistoryRespondDecisionTaskCompletedResponse{},
		},
		{
			name: "RespondDecisionTaskCompleted",
			op: func(c Client) (any, error) {
				return c.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
					CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
						TaskToken: []byte(`{"workflowId": "test-workflow"}`),
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RespondDecisionTaskCompleted(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.HistoryRespondDecisionTaskCompletedResponse{}, nil).Times(1)
			},
			want: &types.HistoryRespondDecisionTaskCompletedResponse{},
		},
		{
			name: "SignalWithStartWorkflowExecution",
			op: func(c Client) (any, error) {
				return c.SignalWithStartWorkflowExecution(context.Background(), &types.HistorySignalWithStartWorkflowExecutionRequest{
					SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
						WorkflowID: "test-workflow",
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
			},
			want: &types.StartWorkflowExecutionResponse{},
		},
		{
			name: "RatelimitUpdate",
			op: func(c Client) (any, error) {
				return c.RatelimitUpdate(context.Background(), &types.RatelimitUpdateRequest{
					Any: &types.Any{
						ValueType: "something",
						Value:     []byte("data"),
					},
				}, yarpc.WithShardKey("test-peer"))
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				c.EXPECT().RatelimitUpdate(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(&types.RatelimitUpdateResponse{}, nil).Times(1)
			},
			want: &types.RatelimitUpdateResponse{},
		},
		{
			name: "StartWorkflowExecution peer resolve failure",
			op: func(c Client) (any, error) {
				return c.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
					StartRequest: &types.StartWorkflowExecutionRequest{
						WorkflowID: "test-workflow",
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("", fmt.Errorf("some error")).Times(1)
			},
			wantError: true,
		},
		{
			name: "StartWorkflowExecution failure",
			op: func(c Client) (any, error) {
				return c.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
					StartRequest: &types.StartWorkflowExecutionRequest{
						WorkflowID: "test-workflow",
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("StartWorkflowExecution failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "StartWorkflowExecution redirected success with host lost error",
			op: func(c Client) (any, error) {
				return c.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
					StartRequest: &types.StartWorkflowExecutionRequest{
						WorkflowID: "test-workflow",
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer-1", nil).Times(1)
				p.EXPECT().FromHostAddress("host-test-peer-2").Return("test-peer-2", nil).Times(1)
				c.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-1")}).
					Return(nil, &types.ShardOwnershipLostError{
						Message: "test-peer-1 lost the shard",
						Owner:   "host-test-peer-2",
					}).Times(1)
				c.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-2")}).
					Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
			},
			want: &types.StartWorkflowExecutionResponse{},
		},
		{
			name: "StartWorkflowExecution redirected failed again with peer resolve",
			op: func(c Client) (any, error) {
				return c.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
					StartRequest: &types.StartWorkflowExecutionRequest{
						WorkflowID: "test-workflow",
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer-1", nil).Times(1)
				p.EXPECT().FromHostAddress("host-test-peer-2").Return("", fmt.Errorf("not found")).Times(1)
				c.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-1")}).
					Return(nil, &types.ShardOwnershipLostError{
						Message: "test-peer-1 lost the shard",
						Owner:   "host-test-peer-2",
					}).Times(1)
			},
			wantError: true,
		},
		{
			name: "StartWorkflowExecution redirected failed again with error",
			op: func(c Client) (any, error) {
				return c.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
					StartRequest: &types.StartWorkflowExecutionRequest{
						WorkflowID: "test-workflow",
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer-1", nil).Times(1)
				p.EXPECT().FromHostAddress("host-test-peer-2").Return("test-peer-2", nil).Times(1)
				c.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-1")}).
					Return(nil, &types.ShardOwnershipLostError{
						Message: "test-peer-1 lost the shard",
						Owner:   "host-test-peer-2",
					}).Times(1)
				c.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-2")}).
					Return(nil, fmt.Errorf("some error")).Times(1)
			},
			wantError: true,
		},
		{
			name: "GetMutableState fail",
			op: func(c Client) (any, error) {
				return c.GetMutableState(context.Background(), &types.GetMutableStateRequest{
					Execution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().GetMutableState(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("GetMutableState failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "PollMutableState fail",
			op: func(c Client) (any, error) {
				return c.PollMutableState(context.Background(), &types.PollMutableStateRequest{
					Execution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().PollMutableState(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("PollMutableState failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "ResetWorkflowExecution fail",
			op: func(c Client) (any, error) {
				return c.ResetWorkflowExecution(context.Background(), &types.HistoryResetWorkflowExecutionRequest{
					ResetRequest: &types.ResetWorkflowExecutionRequest{
						WorkflowExecution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().ResetWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("ResetWorkflowExecution failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "DescribeWorkflowExecution fail",
			op: func(c Client) (any, error) {
				return c.DescribeWorkflowExecution(context.Background(), &types.HistoryDescribeWorkflowExecutionRequest{
					Request: &types.DescribeWorkflowExecutionRequest{
						Execution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("DescribeWorkflowExecution failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "RecordActivityTaskHeartbeat fail",
			op: func(c Client) (any, error) {
				return c.RecordActivityTaskHeartbeat(context.Background(), &types.HistoryRecordActivityTaskHeartbeatRequest{
					HeartbeatRequest: &types.RecordActivityTaskHeartbeatRequest{
						TaskToken: []byte(`{"workflowId": "test-workflow"}`),
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RecordActivityTaskHeartbeat(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("RecordActivityTaskHeartbeat failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "RecordActivityTaskStarted fail",
			op: func(c Client) (any, error) {
				return c.RecordActivityTaskStarted(context.Background(), &types.RecordActivityTaskStartedRequest{
					WorkflowExecution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RecordActivityTaskStarted(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("RecordActivityTaskStarted failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "RecordDecisionTaskStarted fail",
			op: func(c Client) (any, error) {
				return c.RecordDecisionTaskStarted(context.Background(), &types.RecordDecisionTaskStartedRequest{
					WorkflowExecution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RecordDecisionTaskStarted(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("RecordDecisionTaskStarted failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "GetReplicationMessages fail open on unknow error",
			op: func(c Client) (any, error) {
				return c.GetReplicationMessages(context.Background(), &types.GetReplicationMessagesRequest{
					Tokens: []*types.ReplicationToken{
						{
							ShardID: 100,
						},
						{
							ShardID: 101,
						},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromShardID(100).Return("test-peer-0", nil).Times(1)
				p.EXPECT().FromShardID(101).Return("test-peer-1", nil).Times(1)
				c.EXPECT().GetReplicationMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-0")}).
					Return(&types.GetReplicationMessagesResponse{
						MessagesByShard: map[int32]*types.ReplicationMessages{100: {}},
					}, nil).Times(1)
				c.EXPECT().GetReplicationMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-1")}).
					Return(nil, fmt.Errorf("GetReplicationMessages failed")).Times(1)
			},
			want: &types.GetReplicationMessagesResponse{
				MessagesByShard: map[int32]*types.ReplicationMessages{
					100: {},
				},
			},
			wantError: false,
		},
		{
			name: "GetReplicationMessages fail open on unknow error",
			op: func(c Client) (any, error) {
				return c.GetReplicationMessages(context.Background(), &types.GetReplicationMessagesRequest{
					Tokens: []*types.ReplicationToken{
						{
							ShardID: 100,
						},
						{
							ShardID: 101,
						},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromShardID(100).Return("test-peer-0", nil).Times(1)
				p.EXPECT().FromShardID(101).Return("test-peer-1", nil).Times(1)
				c.EXPECT().GetReplicationMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-0")}).
					Return(&types.GetReplicationMessagesResponse{
						MessagesByShard: map[int32]*types.ReplicationMessages{100: {}},
					}, nil).Times(1)
				c.EXPECT().GetReplicationMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-1")}).
					Return(nil, &types.ServiceBusyError{}).Times(1)
			},
			wantError: true,
		},
		{
			name: "GetDLQReplicationMessages fail",
			op: func(c Client) (any, error) {
				return c.GetDLQReplicationMessages(context.Background(), &types.GetDLQReplicationMessagesRequest{
					TaskInfos: []*types.ReplicationTaskInfo{
						{WorkflowID: "test-workflow"},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().GetDLQReplicationMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("GetDLQReplicationMessages failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "ReadDLQMessages fail",
			op: func(c Client) (any, error) {
				return c.ReadDLQMessages(context.Background(), &types.ReadDLQMessagesRequest{
					ShardID: 123,
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromShardID(123).Return("test-peer", nil).Times(1)
				c.EXPECT().ReadDLQMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("ReadDLQMessages failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "MergeDLQMessages fail",
			op: func(c Client) (any, error) {
				return c.MergeDLQMessages(context.Background(), &types.MergeDLQMessagesRequest{
					ShardID: 123,
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromShardID(123).Return("test-peer", nil).Times(1)
				c.EXPECT().MergeDLQMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("MergeDLQMessages failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "GetFailoverInfo fail",
			op: func(c Client) (any, error) {
				return c.GetFailoverInfo(context.Background(), &types.GetFailoverInfoRequest{
					DomainID: "test-domain",
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromDomainID("test-domain").Return("test-peer", nil).Times(1)
				c.EXPECT().GetFailoverInfo(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("GetFailoverInfo failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "DescribeMutableState fail",
			op: func(c Client) (any, error) {
				return c.DescribeMutableState(context.Background(), &types.DescribeMutableStateRequest{
					Execution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromWorkflowID(gomock.Any()).Return("test-peer", nil).Times(1)
				c.EXPECT().DescribeMutableState(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("DescribeMutableState failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "DescribeQueue fail",
			op: func(c Client) (any, error) {
				return c.DescribeQueue(context.Background(), &types.DescribeQueueRequest{
					ShardID: 123,
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromShardID(123).Return("test-peer", nil).Times(1)
				c.EXPECT().DescribeQueue(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("DescribeQueue failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "CountDLQMessages fail",
			op: func(c Client) (any, error) {
				return c.CountDLQMessages(context.Background(), &types.CountDLQMessagesRequest{})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().GetAllPeers().Return([]string{"test-peer"}, nil).Times(1)
				c.EXPECT().CountDLQMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("CountDLQMessages failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "QueryWorkflow fail",
			op: func(c Client) (any, error) {
				return c.QueryWorkflow(context.Background(), &types.HistoryQueryWorkflowRequest{
					Request: &types.QueryWorkflowRequest{
						Execution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromWorkflowID(gomock.Any()).Return("test-peer", nil).Times(1)
				c.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("QueryWorkflow failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "ResetStickyTaskList fail",
			op: func(c Client) (any, error) {
				return c.ResetStickyTaskList(context.Background(), &types.HistoryResetStickyTaskListRequest{
					Execution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().ResetStickyTaskList(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("ResetStickyTaskList failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "RespondDecisionTaskCompleted fail",
			op: func(c Client) (any, error) {
				return c.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
					CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
						TaskToken: []byte(`{"workflowId": "test-workflow"}`),
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RespondDecisionTaskCompleted(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("RespondDecisionTaskCompleted failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "RespondDecisionTaskCompleted fail",
			op: func(c Client) (any, error) {
				return c.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
					CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
						TaskToken: []byte(`{"workflowId": "test-workflow"}`),
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RespondDecisionTaskCompleted(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("RespondDecisionTaskCompleted failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "SignalWithStartWorkflowExecution fail",
			op: func(c Client) (any, error) {
				return c.SignalWithStartWorkflowExecution(context.Background(), &types.HistorySignalWithStartWorkflowExecutionRequest{
					SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
						WorkflowID: "test-workflow",
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// Add your mock expectations here
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil, fmt.Errorf("SignalWithStartWorkflowExecution failed")).Times(1)
			},
			wantError: true,
		},
		{
			name: "RatelimitUpdate requires explicit shard key arg",
			op: func(c Client) (any, error) {
				// same as successful call...
				return c.RatelimitUpdate(context.Background(), &types.RatelimitUpdateRequest{
					// Peer: "", // intentionally the zero value
					Any: &types.Any{
						ValueType: "something",
						Value:     []byte("data"),
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				// no calls expected
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockClient(ctrl)
			mockPeerResolver := NewMockPeerResolver(ctrl)
			c := NewClient(10, 1024, mockClient, mockPeerResolver, log.NewNoop())
			tt.mock(mockPeerResolver, mockClient)
			res, err := tt.op(c)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, res)
			}
		})
	}
}

func TestClient_withNoResponse(t *testing.T) {
	tests := []struct {
		name      string
		op        func(Client) error
		mock      func(*MockPeerResolver, *MockClient)
		wantError bool
	}{
		{
			name: "RefreshWorkflowTasks",
			op: func(c Client) error {
				return c.RefreshWorkflowTasks(context.Background(), &types.HistoryRefreshWorkflowTasksRequest{
					Request: &types.RefreshWorkflowTasksRequest{
						Execution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RefreshWorkflowTasks(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "PurgeDLQMessages",
			op: func(c Client) error {
				return c.PurgeDLQMessages(context.Background(), &types.PurgeDLQMessagesRequest{
					ShardID: 123,
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromShardID(123).Return("test-peer", nil).Times(1)
				c.EXPECT().PurgeDLQMessages(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "ReapplyEvents",
			op: func(c Client) error {
				return c.ReapplyEvents(context.Background(), &types.HistoryReapplyEventsRequest{
					Request: &types.ReapplyEventsRequest{
						WorkflowExecution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().ReapplyEvents(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "TerminateWorkflowExecution",
			op: func(c Client) error {
				return c.TerminateWorkflowExecution(context.Background(), &types.HistoryTerminateWorkflowExecutionRequest{
					TerminateRequest: &types.TerminateWorkflowExecutionRequest{
						WorkflowExecution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "NotifyFailoverMarkers",
			op: func(c Client) error {
				return c.NotifyFailoverMarkers(context.Background(), &types.NotifyFailoverMarkersRequest{
					FailoverMarkerTokens: []*types.FailoverMarkerToken{
						{
							FailoverMarker: &types.FailoverMarkerAttributes{
								DomainID: "test-domain-0",
							},
						},
						{
							FailoverMarker: &types.FailoverMarkerAttributes{
								DomainID: "test-domain-1",
							},
						},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromDomainID("test-domain-0").Return("test-peer-0", nil).Times(1)
				p.EXPECT().FromDomainID("test-domain-1").Return("test-peer-1", nil).Times(1)
				c.EXPECT().NotifyFailoverMarkers(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-0")}).
					Return(nil).Times(1)
				c.EXPECT().NotifyFailoverMarkers(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer-1")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "CloseShard",
			op: func(c Client) error {
				return c.CloseShard(context.Background(), &types.CloseShardRequest{
					ShardID: 123,
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromShardID(123).Return("test-peer", nil).Times(1)
				c.EXPECT().CloseShard(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "RemoveTask",
			op: func(c Client) error {
				return c.RemoveTask(context.Background(), &types.RemoveTaskRequest{
					ShardID: 123,
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromShardID(123).Return("test-peer", nil).Times(1)
				c.EXPECT().RemoveTask(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "RecordChildExecutionCompleted",
			op: func(c Client) error {
				return c.RecordChildExecutionCompleted(context.Background(), &types.RecordChildExecutionCompletedRequest{
					WorkflowExecution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RecordChildExecutionCompleted(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "RefreshWorkflowTasks",
			op: func(c Client) error {
				return c.RefreshWorkflowTasks(context.Background(), &types.HistoryRefreshWorkflowTasksRequest{
					Request: &types.RefreshWorkflowTasksRequest{
						Execution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RefreshWorkflowTasks(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "RemoveSignalMutableState",
			op: func(c Client) error {
				return c.RemoveSignalMutableState(context.Background(), &types.RemoveSignalMutableStateRequest{
					WorkflowExecution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RemoveSignalMutableState(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "ReplicateEventsV2",
			op: func(c Client) error {
				return c.ReplicateEventsV2(context.Background(), &types.ReplicateEventsV2Request{
					WorkflowExecution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().ReplicateEventsV2(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "RequestCancelWorkflowExecution",
			op: func(c Client) error {
				return c.RequestCancelWorkflowExecution(context.Background(), &types.HistoryRequestCancelWorkflowExecutionRequest{
					CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
						WorkflowExecution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "ResetQueue",
			op: func(c Client) error {
				return c.ResetQueue(context.Background(), &types.ResetQueueRequest{
					ShardID: 123,
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromShardID(123).Return("test-peer", nil).Times(1)
				c.EXPECT().ResetQueue(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "RespondActivityTaskCanceled",
			op: func(c Client) error {
				return c.RespondActivityTaskCanceled(context.Background(), &types.HistoryRespondActivityTaskCanceledRequest{
					CancelRequest: &types.RespondActivityTaskCanceledRequest{
						TaskToken: []byte(`{"workflowId": "test-workflow"}`),
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RespondActivityTaskCanceled(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "RespondActivityTaskCompleted",
			op: func(c Client) error {
				return c.RespondActivityTaskCompleted(context.Background(), &types.HistoryRespondActivityTaskCompletedRequest{
					CompleteRequest: &types.RespondActivityTaskCompletedRequest{
						TaskToken: []byte(`{"workflowId": "test-workflow"}`),
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RespondActivityTaskCompleted(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "RespondActivityTaskFailed",
			op: func(c Client) error {
				return c.RespondActivityTaskFailed(context.Background(), &types.HistoryRespondActivityTaskFailedRequest{
					FailedRequest: &types.RespondActivityTaskFailedRequest{
						TaskToken: []byte(`{"workflowId": "test-workflow"}`),
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RespondActivityTaskFailed(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "RespondDecisionTaskFailed",
			op: func(c Client) error {
				return c.RespondDecisionTaskFailed(context.Background(), &types.HistoryRespondDecisionTaskFailedRequest{
					FailedRequest: &types.RespondDecisionTaskFailedRequest{
						TaskToken: []byte(`{"workflowId": "test-workflow"}`),
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().RespondDecisionTaskFailed(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "ScheduleDecisionTask",
			op: func(c Client) error {
				return c.ScheduleDecisionTask(context.Background(), &types.ScheduleDecisionTaskRequest{
					WorkflowExecution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().ScheduleDecisionTask(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "SignalWorkflowExecution",
			op: func(c Client) error {
				return c.SignalWorkflowExecution(context.Background(), &types.HistorySignalWorkflowExecutionRequest{
					SignalRequest: &types.SignalWorkflowExecutionRequest{
						WorkflowExecution: &types.WorkflowExecution{WorkflowID: "test-workflow"},
					},
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "SyncActivity",
			op: func(c Client) error {
				return c.SyncActivity(context.Background(), &types.SyncActivityRequest{
					WorkflowID: "test-workflow",
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromWorkflowID("test-workflow").Return("test-peer", nil).Times(1)
				c.EXPECT().SyncActivity(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
		{
			name: "SyncShardStatus",
			op: func(c Client) error {
				return c.SyncShardStatus(context.Background(), &types.SyncShardStatusRequest{
					ShardID: 123,
				})
			},
			mock: func(p *MockPeerResolver, c *MockClient) {
				p.EXPECT().FromShardID(123).Return("test-peer", nil).Times(1)
				c.EXPECT().SyncShardStatus(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("test-peer")}).
					Return(nil).Times(1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := NewMockClient(ctrl)
			mockPeerResolver := NewMockPeerResolver(ctrl)
			c := NewClient(10, 1024, mockClient, mockPeerResolver, log.NewNoop())
			tt.mock(mockPeerResolver, mockClient)
			err := tt.op(c)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_cmpGetReplicationMessagesWithSize(t *testing.T) {
	for name, c := range map[string]struct {
		a, b *getReplicationMessagesWithSize
		want int
	}{
		"both nil": {
			a: nil, b: nil, want: 1,
		},
		"a time is nil, b is nil": {
			a: &getReplicationMessagesWithSize{earliestCreationTime: nil}, b: nil, want: 1,
		},
		"a time is not nil, b is nil": {
			a: &getReplicationMessagesWithSize{earliestCreationTime: common.Int64Ptr(10)}, b: nil, want: -1,
		},
		"a time is not nil, b time is nil": {
			a:    &getReplicationMessagesWithSize{earliestCreationTime: common.Int64Ptr(10)},
			b:    &getReplicationMessagesWithSize{earliestCreationTime: nil},
			want: -1,
		},
		"a time less b time": {
			a:    &getReplicationMessagesWithSize{earliestCreationTime: common.Int64Ptr(10)},
			b:    &getReplicationMessagesWithSize{earliestCreationTime: common.Int64Ptr(20)},
			want: -1,
		},
		"a time greater b time": {
			a:    &getReplicationMessagesWithSize{earliestCreationTime: common.Int64Ptr(20)},
			b:    &getReplicationMessagesWithSize{earliestCreationTime: common.Int64Ptr(10)},
			want: 1,
		},
		"a size less b size": {
			a:    &getReplicationMessagesWithSize{earliestCreationTime: common.Int64Ptr(10), size: 10},
			b:    &getReplicationMessagesWithSize{earliestCreationTime: common.Int64Ptr(10), size: 20},
			want: -1,
		},
		"a size greater b size": {
			a:    &getReplicationMessagesWithSize{earliestCreationTime: common.Int64Ptr(10), size: 20},
			b:    &getReplicationMessagesWithSize{earliestCreationTime: common.Int64Ptr(10), size: 10},
			want: 1,
		},
		"a equal b": {
			a:    &getReplicationMessagesWithSize{earliestCreationTime: common.Int64Ptr(10)},
			b:    &getReplicationMessagesWithSize{earliestCreationTime: common.Int64Ptr(10)},
			want: 0,
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, cmpGetReplicationMessagesWithSize(c.a, c.b))
		})
	}
}

func Test_sortGetReplicationMessageWithSize(t *testing.T) {
	for name, c := range map[string]struct {
		responses []*getReplicationMessagesWithSize
		want      []*getReplicationMessagesWithSize
	}{
		"empty": {},
		"multiple nil, non nil earliestCreationTime": {
			responses: []*getReplicationMessagesWithSize{
				{earliestCreationTime: nil},
				{earliestCreationTime: nil},
				{earliestCreationTime: common.Int64Ptr(20)},
				{earliestCreationTime: common.Int64Ptr(10)},
			},
			want: []*getReplicationMessagesWithSize{
				{earliestCreationTime: common.Int64Ptr(10)},
				{earliestCreationTime: common.Int64Ptr(20)},
				{earliestCreationTime: nil},
				{earliestCreationTime: nil},
			},
		},
		"multiple nil, non nil same earliestCreationTime, different size": {
			responses: []*getReplicationMessagesWithSize{
				{earliestCreationTime: nil},
				{earliestCreationTime: nil},
				{earliestCreationTime: common.Int64Ptr(100), size: 50},
				{earliestCreationTime: common.Int64Ptr(100), size: 30},
				{earliestCreationTime: common.Int64Ptr(20)},
			},
			want: []*getReplicationMessagesWithSize{
				{earliestCreationTime: common.Int64Ptr(20)},
				{earliestCreationTime: common.Int64Ptr(100), size: 30},
				{earliestCreationTime: common.Int64Ptr(100), size: 50},
				{earliestCreationTime: nil},
				{earliestCreationTime: nil},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			sortGetReplicationMessageWithSize(c.responses)
			assert.Equal(t, c.want, c.responses)
		})
	}
}
