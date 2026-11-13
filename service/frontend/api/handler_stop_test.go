package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/service/frontend/config"
)

func TestStop(t *testing.T) {
	t.Run("cancels outstanding polls when enabled", func(t *testing.T) {
		res := resource.NewTest(t, gomock.NewController(t), metrics.Frontend)
		wh := newHandlerWithPoll(res, true)

		res.MatchingClient.EXPECT().CancelOutstandingPoll(gomock.Any(), newCancelRequest("poller-1")).Return(nil)
		wh.Stop()

		assert.True(t, wh.isShuttingDown())
	})

	t.Run("keeps outstanding polls when disabled", func(t *testing.T) {
		res := resource.NewTest(t, gomock.NewController(t), metrics.Frontend)
		wh := newHandlerWithPoll(res, false)

		res.MatchingClient.EXPECT().CancelOutstandingPoll(gomock.Any(), gomock.Any()).Times(0)
		wh.Stop()

		assert.True(t, wh.isShuttingDown())
	})
}

func newHandlerWithPoll(res *resource.Test, cancelPollsOnShutdown bool) *WorkflowHandler {
	tracker := NewPollTracker(log.NewNoop())
	tracker.PollStarted(newCancelRequest("poller-1"))
	return &WorkflowHandler{
		Resource:    res,
		config:      &config.Config{CancelOutstandingPollsOnShutdown: dynamicproperties.GetBoolPropertyFn(cancelPollsOnShutdown)},
		pollTracker: tracker,
	}
}
