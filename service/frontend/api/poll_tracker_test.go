package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zapcore"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestPollTrackerCancelsEveryPollInFlight(t *testing.T) {
	client := matching.NewMockClient(gomock.NewController(t))
	tracker := NewPollTracker(log.NewNoop())
	failedPoll := newCancelRequest("poller-1")
	cancelledPoll := newCancelRequest("poller-2")
	tracker.PollStarted(failedPoll)
	tracker.PollStarted(cancelledPoll)

	client.EXPECT().CancelOutstandingPoll(gomock.Any(), failedPoll).Return(errors.New("matching is restarting"))
	client.EXPECT().CancelOutstandingPoll(gomock.Any(), cancelledPoll).Return(nil)

	tracker.cancelAll(context.Background(), client)
}

func TestPollTrackerIgnoresPollWithoutPollerID(t *testing.T) {
	logger, logs := testlogger.NewObserved(t)
	tracker := NewPollTracker(logger)

	tracker.PollStarted(newCancelRequest(""))

	assert.Empty(t, tracker.polls)
	assert.Equal(t, 1, logs.FilterLevelExact(zapcore.WarnLevel).Len())
}

func newCancelRequest(pollerID string) *types.CancelOutstandingPollRequest {
	return &types.CancelOutstandingPollRequest{
		DomainUUID:   "domain-id",
		TaskListType: common.Int32Ptr(persistence.TaskListTypeDecision),
		TaskList:     &types.TaskList{Name: "/__cadence_sys/task-list/1", Kind: types.TaskListKindNormal.Ptr()},
		PollerID:     pollerID,
	}
}
