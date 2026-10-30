package matching

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestPollSendsRequestToPartitionWithoutMutatingIt(t *testing.T) {
	rootTaskList := types.TaskList{Name: _testTaskList, Kind: types.TaskListKindNormal.Ptr()}
	partitionTaskList := &types.TaskList{Name: _testPartition, Kind: types.TaskListKindNormal.Ptr()}

	tests := map[string]struct {
		taskListType int
		mockPoll     func(c *MockClient, pollErr error)
		poll         func(c Client) (request any, err error)
	}{
		"PollForActivityTask": {
			taskListType: persistence.TaskListTypeActivity,
			mockPoll: func(c *MockClient, pollErr error) {
				c.EXPECT().PollForActivityTask(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).
					DoAndReturn(func(_ context.Context, req *types.MatchingPollForActivityTaskRequest, _ ...yarpc.CallOption) (*types.MatchingPollForActivityTaskResponse, error) {
						assert.Equal(t, partitionTaskList, req.PollRequest.TaskList)
						if pollErr != nil {
							return nil, pollErr
						}
						return &types.MatchingPollForActivityTaskResponse{}, nil
					})
			},
			poll: func(c Client) (any, error) {
				taskList := rootTaskList
				req := &types.MatchingPollForActivityTaskRequest{
					DomainUUID:  _testDomainUUID,
					PollerID:    "poller-id",
					PollRequest: &types.PollForActivityTaskRequest{TaskList: &taskList},
				}
				_, err := c.PollForActivityTask(context.Background(), req)
				return req, err
			},
		},
		"PollForDecisionTask": {
			taskListType: persistence.TaskListTypeDecision,
			mockPoll: func(c *MockClient, pollErr error) {
				c.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any(), []yarpc.CallOption{yarpc.WithShardKey("peer0")}).
					DoAndReturn(func(_ context.Context, req *types.MatchingPollForDecisionTaskRequest, _ ...yarpc.CallOption) (*types.MatchingPollForDecisionTaskResponse, error) {
						assert.Equal(t, partitionTaskList, req.PollRequest.TaskList)
						if pollErr != nil {
							return nil, pollErr
						}
						return &types.MatchingPollForDecisionTaskResponse{}, nil
					})
			},
			poll: func(c Client) (any, error) {
				taskList := rootTaskList
				req := &types.MatchingPollForDecisionTaskRequest{
					DomainUUID:  _testDomainUUID,
					PollerID:    "poller-id",
					PollRequest: &types.PollForDecisionTaskRequest{TaskList: &taskList},
				}
				_, err := c.PollForDecisionTask(context.Background(), req)
				return req, err
			},
		},
	}

	for name, tc := range tests {
		t.Run(name+" success", func(t *testing.T) {
			c, deps := newTestPollClient(t)
			deps.peerResolver.EXPECT().FromTaskList(_testPartition).Return("peer0", nil)
			tc.mockPoll(deps.rawClient, nil)
			deps.provider.EXPECT().UpdatePartitionConfig(_testDomainUUID, rootTaskList, tc.taskListType, nil)
			deps.loadBalancer.EXPECT().UpdateWeight(tc.taskListType, gomock.Any(), _testPartition, nil).
				Do(func(_ int, req ReadRequest, _ string, _ *types.LoadBalancerHints) {
					assert.Equal(t, rootTaskList, *req.GetTaskList())
				})

			req, err := tc.poll(c)

			assert.NoError(t, err)
			assertRootTaskList(t, rootTaskList, req)
		})

		t.Run(name+" error reports partition", func(t *testing.T) {
			c, deps := newTestPollClient(t)
			deps.peerResolver.EXPECT().FromTaskList(_testPartition).Return("peer0", nil)
			tc.mockPoll(deps.rawClient, &types.InternalServiceError{Message: "test error"})

			req, err := tc.poll(c)

			var pollErr *PollError
			assert.True(t, errors.As(err, &pollErr))
			assert.Equal(t, partitionTaskList, pollErr.TaskList)
			peer, origErr := cadence_errors.ExtractPeerHostname(err)
			assert.Equal(t, "peer0", peer)
			assert.Equal(t, &types.InternalServiceError{Message: "test error"}, origErr)
			assertRootTaskList(t, rootTaskList, req)
		})

		t.Run(name+" error before sending is not a PollError", func(t *testing.T) {
			c, deps := newTestPollClient(t)
			deps.peerResolver.EXPECT().FromTaskList(_testPartition).Return("", assert.AnError)

			req, err := tc.poll(c)

			assert.ErrorIs(t, err, assert.AnError)
			assert.False(t, errors.As(err, new(*PollError)))
			assertRootTaskList(t, rootTaskList, req)
		})
	}
}

type testPollClientDeps struct {
	rawClient    *MockClient
	peerResolver *MockPeerResolver
	loadBalancer *MockLoadBalancer
	provider     *MockPartitionConfigProvider
}

func newTestPollClient(t *testing.T) (Client, testPollClientDeps) {
	ctrl := gomock.NewController(t)
	deps := testPollClientDeps{
		rawClient:    NewMockClient(ctrl),
		peerResolver: NewMockPeerResolver(ctrl),
		loadBalancer: NewMockLoadBalancer(ctrl),
		provider:     NewMockPartitionConfigProvider(ctrl),
	}
	deps.loadBalancer.EXPECT().PickReadPartition(gomock.Any(), gomock.Any(), gomock.Any()).Return(_testPartition)
	return NewClient(deps.rawClient, deps.peerResolver, deps.loadBalancer, deps.provider), deps
}

func assertRootTaskList(t *testing.T, expected types.TaskList, request any) {
	req, ok := request.(ReadRequest)
	if assert.True(t, ok) {
		assert.Equal(t, expected, *req.GetTaskList(), "caller's request must not be mutated")
	}
}
