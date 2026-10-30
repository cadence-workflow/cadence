package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestCancelOutstandingPoll(t *testing.T) {
	partition := &types.TaskList{Name: "/__cadence_sys/task-list/1"}
	matchingErr := &types.InternalServiceError{Message: "matching error"}

	tests := map[string]struct {
		clientGone     bool
		err            error
		expectCancelTo *types.TaskList
		expectedErr    error
	}{
		"client gone: cancels the poll on the partition it was sent to": {
			clientGone:     true,
			err:            &matching.PollError{TaskList: partition, Err: context.Canceled},
			expectCancelTo: partition,
		},
		"client gone before the poll was sent: nothing to cancel": {
			clientGone: true,
			err:        context.Canceled,
		},
		"matching error: returned without the partition wrapper": {
			err:         &matching.PollError{TaskList: partition, Err: matchingErr},
			expectedErr: matchingErr,
		},
		"error before the poll was sent: returned as is": {
			err:         matchingErr,
			expectedErr: matchingErr,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			if tc.expectCancelTo != nil {
				deps.mockMatchingClient.EXPECT().CancelOutstandingPoll(gomock.Any(), &types.CancelOutstandingPollRequest{
					DomainUUID:   "domain-id",
					TaskListType: common.Int32Ptr(persistence.TaskListTypeDecision),
					TaskList:     tc.expectCancelTo,
					PollerID:     "poller-id",
				}).Return(nil)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.clientGone {
				cancel()
			}

			err := wh.cancelOutstandingPoll(ctx, tc.err, "domain-id", persistence.TaskListTypeDecision, "poller-id")

			assert.Equal(t, tc.expectedErr, err)
		})
	}
}
