package matching_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

const (
	testDomainID  = "domain-id"
	testPartition = "/__cadence_sys/task-list/1"
)

func TestDisconnectPollersCancelsEveryPollInFlight(t *testing.T) {
	rawClient, client := newTestClient(t)
	cancelErrs := map[string]error{
		"poller-1": errors.New("matching is restarting"),
		"poller-2": nil,
	}
	released := map[string]chan struct{}{
		"poller-1": make(chan struct{}),
		"poller-2": make(chan struct{}),
	}
	var pollsStarted sync.WaitGroup
	pollsStarted.Add(len(released))

	rawClient.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any(), gomock.Any()).Times(len(released)).DoAndReturn(
		func(_ context.Context, request *types.MatchingPollForDecisionTaskRequest, _ ...yarpc.CallOption) (*types.MatchingPollForDecisionTaskResponse, error) {
			pollsStarted.Done()
			<-released[request.GetPollerID()]
			return &types.MatchingPollForDecisionTaskResponse{}, nil
		})
	for pollerID, cancelErr := range cancelErrs {
		rawClient.EXPECT().CancelOutstandingPoll(gomock.Any(), &types.CancelOutstandingPollRequest{
			DomainUUID:   testDomainID,
			TaskListType: common.Int32Ptr(persistence.TaskListTypeDecision),
			TaskList:     &types.TaskList{Name: testPartition, Kind: types.TaskListKindNormal.Ptr()},
			PollerID:     pollerID,
		}, gomock.Any()).DoAndReturn(
			func(context.Context, *types.CancelOutstandingPollRequest, ...yarpc.CallOption) error {
				close(released[pollerID])
				return cancelErr
			})
	}

	pollErrs := make(chan error, len(released))
	for pollerID := range released {
		go func() {
			_, err := client.PollForDecisionTask(context.Background(), newDecisionPollRequest(pollerID))
			pollErrs <- err
		}()
	}
	pollsStarted.Wait()

	client.DisconnectPollers(context.Background())

	for range released {
		assert.NoError(t, <-pollErrs)
	}
}

func TestPollAfterDisconnectIsNotSentToMatching(t *testing.T) {
	_, client := newTestClient(t)
	client.DisconnectPollers(context.Background())

	activityResp, err := client.PollForActivityTask(context.Background(), &types.MatchingPollForActivityTaskRequest{
		DomainUUID:  testDomainID,
		PollerID:    "poller-1",
		PollRequest: &types.PollForActivityTaskRequest{TaskList: newRootTaskList()},
	})
	assert.NoError(t, err)
	assert.Equal(t, &types.MatchingPollForActivityTaskResponse{}, activityResp)

	decisionResp, err := client.PollForDecisionTask(context.Background(), newDecisionPollRequest("poller-1"))
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

	return rawClient, matching.NewClient(rawClient, peerResolver, loadBalancer, provider, log.NewNoop())
}

func newDecisionPollRequest(pollerID string) *types.MatchingPollForDecisionTaskRequest {
	return &types.MatchingPollForDecisionTaskRequest{
		DomainUUID:  testDomainID,
		PollerID:    pollerID,
		PollRequest: &types.PollForDecisionTaskRequest{TaskList: newRootTaskList()},
	}
}

func newRootTaskList() *types.TaskList {
	return &types.TaskList{Name: "task-list", Kind: types.TaskListKindNormal.Ptr()}
}
