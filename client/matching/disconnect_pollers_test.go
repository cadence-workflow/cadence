package matching_test

import (
	"context"
	"errors"
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

func TestDisconnectPollersCancelsPollsInFlight(t *testing.T) {
	cancelErr := errors.New("cancel failed")

	tests := []struct {
		name      string
		cancelErr error
		wantErr   error
	}{
		{name: "poll is cancelled"},
		{name: "cancel failure is reported", cancelErr: cancelErr, wantErr: cancelErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawClient, client := newTestClient(t)
			pollStarted, pollReleased := make(chan struct{}), make(chan struct{})
			rawClient.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
				func(context.Context, *types.MatchingPollForDecisionTaskRequest, ...yarpc.CallOption) (*types.MatchingPollForDecisionTaskResponse, error) {
					close(pollStarted)
					<-pollReleased
					return &types.MatchingPollForDecisionTaskResponse{}, nil
				})
			rawClient.EXPECT().CancelOutstandingPoll(gomock.Any(), &types.CancelOutstandingPollRequest{
				DomainUUID:   testDomainID,
				TaskListType: common.Int32Ptr(persistence.TaskListTypeDecision),
				TaskList:     &types.TaskList{Name: testPartition, Kind: types.TaskListKindNormal.Ptr()},
				PollerID:     testPollerID,
			}, gomock.Any()).DoAndReturn(
				func(context.Context, *types.CancelOutstandingPollRequest, ...yarpc.CallOption) error {
					close(pollReleased)
					return tt.cancelErr
				})

			pollErr := make(chan error)
			go func() {
				_, err := client.PollForDecisionTask(context.Background(), newDecisionPollRequest())
				pollErr <- err
			}()
			<-pollStarted

			err := client.DisconnectPollers(context.Background())
			assert.ErrorIs(t, err, tt.wantErr)
			assert.NoError(t, <-pollErr)
		})
	}
}

func TestPollAfterDisconnectIsNotSentToMatching(t *testing.T) {
	_, client := newTestClient(t)
	require.NoError(t, client.DisconnectPollers(context.Background()))

	activityResp, err := client.PollForActivityTask(context.Background(), &types.MatchingPollForActivityTaskRequest{
		DomainUUID:  testDomainID,
		PollerID:    testPollerID,
		PollRequest: &types.PollForActivityTaskRequest{TaskList: newRootTaskList()},
	})
	assert.NoError(t, err)
	assert.Equal(t, &types.MatchingPollForActivityTaskResponse{}, activityResp)

	decisionResp, err := client.PollForDecisionTask(context.Background(), newDecisionPollRequest())
	assert.NoError(t, err)
	assert.Equal(t, &types.MatchingPollForDecisionTaskResponse{}, decisionResp)
}

// newTestClient returns the raw RPC client mock and the matching client sending polls through it
func newTestClient(t *testing.T) (*matching.MockClient, matching.ClientWithPollerDisconnector) {
	ctrl := gomock.NewController(t)
	rawClient := matching.NewMockClient(ctrl)

	peerResolver := matching.NewMockPeerResolver(ctrl)
	peerResolver.EXPECT().FromTaskList(testPartition).Return("peer", nil).AnyTimes()
	loadBalancer := matching.NewMockLoadBalancer(ctrl)
	loadBalancer.EXPECT().PickReadPartition(gomock.Any(), gomock.Any(), gomock.Any()).Return(testPartition).AnyTimes()
	loadBalancer.EXPECT().UpdateWeight(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	provider := matching.NewMockPartitionConfigProvider(ctrl)
	provider.EXPECT().UpdatePartitionConfig(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	return rawClient, matching.NewClient(rawClient, peerResolver, loadBalancer, provider)
}

func newDecisionPollRequest() *types.MatchingPollForDecisionTaskRequest {
	return &types.MatchingPollForDecisionTaskRequest{
		DomainUUID:  testDomainID,
		PollerID:    testPollerID,
		PollRequest: &types.PollForDecisionTaskRequest{TaskList: newRootTaskList()},
	}
}

func newRootTaskList() *types.TaskList {
	return &types.TaskList{Name: "task-list", Kind: types.TaskListKindNormal.Ptr()}
}
