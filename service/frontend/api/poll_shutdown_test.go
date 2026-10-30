package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestCancelOutstandingPollsOnShutdown(t *testing.T) {
	taskList := &types.TaskList{Name: "task-list"}

	tests := map[string]struct {
		taskListType int32
		setupMocks   func(deps *mockDeps, matching *blockingMatching)
		poll         func(ctx context.Context, wh *WorkflowHandler) (any, error)
		expected     any
	}{
		"activity poll returns task matched before cancellation": {
			taskListType: persistence.TaskListTypeActivity,
			setupMocks: func(deps *mockDeps, matching *blockingMatching) {
				deps.mockDomainCache.EXPECT().GetDomainID("domain").Return("domain-id", nil)
				deps.mockMatchingClient.EXPECT().PollForActivityTask(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, req *types.MatchingPollForActivityTaskRequest, _ ...yarpc.CallOption) (*types.MatchingPollForActivityTaskResponse, error) {
						matching.poll(ctx, req.PollerID)
						return &types.MatchingPollForActivityTaskResponse{TaskToken: []byte("token")}, nil
					})
			},
			poll: func(ctx context.Context, wh *WorkflowHandler) (any, error) {
				return wh.PollForActivityTask(ctx, &types.PollForActivityTaskRequest{Domain: "domain", TaskList: taskList})
			},
			expected: &types.PollForActivityTaskResponse{TaskToken: []byte("token")},
		},
		"decision poll returns empty response from matching": {
			taskListType: persistence.TaskListTypeDecision,
			setupMocks: func(deps *mockDeps, matching *blockingMatching) {
				deps.mockDomainCache.EXPECT().GetDomain("domain").Return(testDomainCacheEntry, nil)
				deps.mockMatchingClient.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, req *types.MatchingPollForDecisionTaskRequest, _ ...yarpc.CallOption) (*types.MatchingPollForDecisionTaskResponse, error) {
						matching.poll(ctx, req.PollerID)
						return &types.MatchingPollForDecisionTaskResponse{AutoConfigHint: &types.AutoConfigHint{PollerWaitTimeInMs: 42}}, nil
					})
			},
			poll: func(ctx context.Context, wh *WorkflowHandler) (any, error) {
				return wh.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{Domain: "domain", TaskList: taskList})
			},
			expected: &types.PollForDecisionTaskResponse{AutoConfigHint: &types.AutoConfigHint{PollerWaitTimeInMs: 42}},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.FrontendCancelOutstandingPollsOnShutdown, true))
			matching := newBlockingMatching()
			tc.setupMocks(deps, matching)
			deps.mockMatchingClient.EXPECT().CancelOutstandingPoll(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, req *types.CancelOutstandingPollRequest, _ ...yarpc.CallOption) error {
					assert.Equal(t, &types.CancelOutstandingPollRequest{
						DomainUUID:   "domain-id",
						TaskListType: common.Int32Ptr(tc.taskListType),
						TaskList:     taskList,
						PollerID:     matching.pollerID,
					}, req)
					close(matching.cancelled)
					return nil
				})

			go func() {
				<-matching.started
				wh.Stop()
			}()
			resp, err := tc.poll(testPollContext(t), wh)

			assert.NoError(t, err)
			assert.Equal(t, tc.expected, resp)
		})
	}
}

func TestPollAfterShutdownIsNotSentToMatching(t *testing.T) {
	wh, deps := setupMocksForWorkflowHandler(t)
	require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.FrontendCancelOutstandingPollsOnShutdown, true))
	deps.mockDomainCache.EXPECT().GetDomainID("domain").Return("domain-id", nil)
	wh.outstandingPolls.close()

	resp, err := wh.PollForActivityTask(testPollContext(t), &types.PollForActivityTaskRequest{Domain: "domain", TaskList: &types.TaskList{Name: "task-list"}})

	assert.NoError(t, err)
	assert.Equal(t, &types.PollForActivityTaskResponse{}, resp)
}

func TestOutstandingPollsNotCancelledOnShutdownWhenDisabled(t *testing.T) {
	wh, _ := setupMocksForWorkflowHandler(t)
	wh.outstandingPolls.add("poller-id", outstandingPoll{domainID: "domain-id", taskList: &types.TaskList{Name: "task-list"}})

	// the matching mock fails the test on an unexpected CancelOutstandingPoll call
	wh.Stop()

	assert.False(t, wh.outstandingPolls.isClosed())
}

// blockingMatching holds a poll until it is cancelled, like matching does
type blockingMatching struct {
	pollerID  string
	started   chan struct{}
	cancelled chan struct{}
}

func newBlockingMatching() *blockingMatching {
	return &blockingMatching{started: make(chan struct{}), cancelled: make(chan struct{})}
}

func (m *blockingMatching) poll(ctx context.Context, pollerID string) {
	m.pollerID = pollerID
	close(m.started)
	select {
	case <-m.cancelled:
	case <-ctx.Done():
	}
}

func testPollContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}
