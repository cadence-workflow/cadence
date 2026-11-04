package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestPollTrackerCancelsEveryPollInFlight(t *testing.T) {
	client := matching.NewMockClient(gomock.NewController(t))
	tracker := NewPollTracker()
	failedPoll := newCancelRequest("poller-1")
	cancelledPoll := newCancelRequest("poller-2")
	tracker.PollStarted(failedPoll)
	tracker.PollStarted(cancelledPoll)

	client.EXPECT().CancelOutstandingPoll(gomock.Any(), failedPoll).Return(errors.New("matching is restarting"))
	client.EXPECT().CancelOutstandingPoll(gomock.Any(), cancelledPoll).Return(nil)

	tracker.cancelAll(context.Background(), client, log.NewNoop())
}

func TestPollTrackerForgetsFinishedPolls(t *testing.T) {
	client := matching.NewMockClient(gomock.NewController(t))
	tracker := NewPollTracker()
	done := tracker.PollStarted(newCancelRequest("poller-1"))
	tracker.PollStarted(newCancelRequest(""))

	done()

	tracker.cancelAll(context.Background(), client, log.NewNoop())
	assert.Empty(t, tracker.polls)
}

func newCancelRequest(pollerID string) *types.CancelOutstandingPollRequest {
	return &types.CancelOutstandingPollRequest{
		DomainUUID:   "domain-id",
		TaskListType: common.Int32Ptr(persistence.TaskListTypeDecision),
		TaskList:     &types.TaskList{Name: "/__cadence_sys/task-list/1", Kind: types.TaskListKindNormal.Ptr()},
		PollerID:     pollerID,
	}
}
