package matching_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

const (
	testDomainID  = "domain-id"
	testPollerID  = "poller-id"
	testPartition = "/__cadence_sys/task-list/1"
)

func TestPollIsTrackedWithItsPartition(t *testing.T) {
	tests := []struct {
		name         string
		taskListType int32
		poll         func(matching.Client) error
		mockPoll     func(raw *matching.MockClient, onPoll func())
	}{
		{
			name:         "activity",
			taskListType: persistence.TaskListTypeActivity,
			poll: func(c matching.Client) error {
				_, err := c.PollForActivityTask(context.Background(), &types.MatchingPollForActivityTaskRequest{
					DomainUUID:  testDomainID,
					PollerID:    testPollerID,
					PollRequest: &types.PollForActivityTaskRequest{TaskList: newRootTaskList()},
				})
				return err
			},
			mockPoll: func(raw *matching.MockClient, onPoll func()) {
				raw.EXPECT().PollForActivityTask(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(context.Context, *types.MatchingPollForActivityTaskRequest, ...yarpc.CallOption) (*types.MatchingPollForActivityTaskResponse, error) {
						onPoll()
						return &types.MatchingPollForActivityTaskResponse{}, nil
					})
			},
		},
		{
			name:         "decision",
			taskListType: persistence.TaskListTypeDecision,
			poll: func(c matching.Client) error {
				_, err := c.PollForDecisionTask(context.Background(), &types.MatchingPollForDecisionTaskRequest{
					DomainUUID:  testDomainID,
					PollerID:    testPollerID,
					PollRequest: &types.PollForDecisionTaskRequest{TaskList: newRootTaskList()},
				})
				return err
			},
			mockPoll: func(raw *matching.MockClient, onPoll func()) {
				raw.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(context.Context, *types.MatchingPollForDecisionTaskRequest, ...yarpc.CallOption) (*types.MatchingPollForDecisionTaskResponse, error) {
						onPoll()
						return &types.MatchingPollForDecisionTaskResponse{}, nil
					})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			rawClient := matching.NewMockClient(ctrl)
			tracker := matching.NewMockPollTracker(ctrl)
			pollDone := false
			tracker.EXPECT().PollStarted(&types.CancelOutstandingPollRequest{
				DomainUUID:   testDomainID,
				TaskListType: common.Int32Ptr(tt.taskListType),
				TaskList:     &types.TaskList{Name: testPartition, Kind: types.TaskListKindNormal.Ptr()},
				PollerID:     testPollerID,
			}).Return(func() { pollDone = true })
			tt.mockPoll(rawClient, func() { assert.False(t, pollDone, "poll is done before it was sent") })

			require.NoError(t, tt.poll(newTestClient(ctrl, rawClient, tracker)))
			assert.True(t, pollDone)
		})
	}
}

func newTestClient(ctrl *gomock.Controller, rawClient matching.Client, tracker matching.PollTracker) matching.Client {
	peerResolver := matching.NewMockPeerResolver(ctrl)
	peerResolver.EXPECT().FromTaskList(testPartition).Return("peer", nil).AnyTimes()
	loadBalancer := matching.NewMockLoadBalancer(ctrl)
	loadBalancer.EXPECT().PickReadPartition(gomock.Any(), gomock.Any(), gomock.Any()).Return(testPartition).AnyTimes()
	loadBalancer.EXPECT().UpdateWeight(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	provider := matching.NewMockPartitionConfigProvider(ctrl)
	provider.EXPECT().UpdatePartitionConfig(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	return matching.NewClient(rawClient, peerResolver, loadBalancer, provider, tracker)
}

func newRootTaskList() *types.TaskList {
	return &types.TaskList{Name: "task-list", Kind: types.TaskListKindNormal.Ptr()}
}
