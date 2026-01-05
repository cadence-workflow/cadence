package ratelimited

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/api"
)

const testDomain = "test-domain"

func setupHandler(t *testing.T) *apiHandler {
	ctrl := gomock.NewController(t)
	mockHandler := api.NewMockHandler(ctrl)
	mockDomainCache := cache.NewMockDomainCache(ctrl)

	return NewAPIHandler(
		mockHandler,
		mockDomainCache,
		&mockPolicy{}, // userRateLimiter
		&mockPolicy{}, // workerRateLimiter
		&mockPolicy{}, // visibilityRateLimiter
		&mockPolicy{}, // asyncRateLimiter
	).(*apiHandler)
}

func TestAllowDomainRequestRouting(t *testing.T) {
	cases := []struct {
		name        string
		requestType ratelimitType
		setupMock   func(*apiHandler)
		expectedErr error
	}{
		{
			name:        "User request type uses user limiter with Allow",
			requestType: ratelimitTypeUser,
			setupMock: func(h *apiHandler) {
				h.userRateLimiter.(*mockPolicy).On("Allow", quotas.Info{Domain: testDomain}).Return(true).Once()
			},
			expectedErr: nil,
		},
		{
			name:        "Worker request type uses worker limiter with Allow",
			requestType: ratelimitTypeWorker,
			setupMock: func(h *apiHandler) {
				h.workerRateLimiter.(*mockPolicy).On("Allow", quotas.Info{Domain: testDomain}).Return(true).Once()
			},
			expectedErr: nil,
		},
		{
			name:        "Visibility request type uses visibility limiter with Allow",
			requestType: ratelimitTypeVisibility,
			setupMock: func(h *apiHandler) {
				h.visibilityRateLimiter.(*mockPolicy).On("Allow", quotas.Info{Domain: testDomain}).Return(true).Once()
			},
			expectedErr: nil,
		},
		{
			name:        "WorkerPoll request with zero delay uses worker limiter with Allow",
			requestType: ratelimitTypeWorkerPoll,
			setupMock: func(h *apiHandler) {
				h.maxWorkerPollDelay = func(domain string) time.Duration { return 0 }
				h.workerRateLimiter.(*mockPolicy).On("Allow", quotas.Info{Domain: testDomain}).Return(true).Once()
			},
			expectedErr: nil,
		},
		{
			name:        "WorkerPoll request with delay uses worker limiter with Wait",
			requestType: ratelimitTypeWorkerPoll,
			setupMock: func(h *apiHandler) {
				h.maxWorkerPollDelay = func(domain string) time.Duration { return 5 * time.Second }
				h.workerRateLimiter.(*mockPolicy).On("Wait", mock.Anything, quotas.Info{Domain: testDomain}).Return(nil).Once()
			},
			expectedErr: nil,
		},
		{
			name:        "Allow blocking returns rate limit error",
			requestType: ratelimitTypeUser,
			setupMock: func(h *apiHandler) {
				h.userRateLimiter.(*mockPolicy).On("Allow", quotas.Info{Domain: testDomain}).Return(false).Once()
			},
			expectedErr: newErrRateLimited(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := setupHandler(t)
			tc.setupMock(handler)

			err := handler.allowDomain(context.Background(), tc.requestType, testDomain)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				if tc.expectedErr != nil {
					assert.Equal(t, tc.expectedErr, err)
				}
			} else {
				assert.NoError(t, err)
			}
			handler.userRateLimiter.(*mockPolicy).AssertExpectations(t)
			handler.workerRateLimiter.(*mockPolicy).AssertExpectations(t)
			handler.visibilityRateLimiter.(*mockPolicy).AssertExpectations(t)
		})
	}
}

func TestHandlerRpcRouting(t *testing.T) {
	cases := []struct {
		name         string
		operation    func(*apiHandler) (interface{}, error)
		limiterSetup func(*apiHandler)
	}{
		{
			name: "PollForActivityTask uses worker limiter with Allow when no delay is configured",
			operation: func(h *apiHandler) (interface{}, error) {
				return h.PollForActivityTask(context.Background(), &types.PollForActivityTaskRequest{Domain: testDomain})
			},
			limiterSetup: func(h *apiHandler) {
				h.maxWorkerPollDelay = func(domain string) time.Duration { return 0 }
				h.workerRateLimiter.(*mockPolicy).On("Allow", quotas.Info{Domain: testDomain}).Return(true).Once()
				h.wrapped.(*api.MockHandler).EXPECT().PollForActivityTask(gomock.Any(), gomock.Any()).Return(&types.PollForActivityTaskResponse{}, nil).Times(1)
			},
		},
		{
			name: "PollForActivityTask uses worker limiter with Wait when delay is configured",
			operation: func(h *apiHandler) (interface{}, error) {
				return h.PollForActivityTask(context.Background(), &types.PollForActivityTaskRequest{Domain: testDomain})
			},
			limiterSetup: func(h *apiHandler) {
				h.maxWorkerPollDelay = func(domain string) time.Duration { return 1 * time.Millisecond }
				h.workerRateLimiter.(*mockPolicy).On("Wait", mock.Anything, quotas.Info{Domain: testDomain}).Return(nil).Once()
				h.wrapped.(*api.MockHandler).EXPECT().PollForActivityTask(gomock.Any(), gomock.Any()).Return(&types.PollForActivityTaskResponse{}, nil).Times(1)
			},
		},
		{
			name: "CountWorkflowExecutions uses visibility limiter with Allow",
			operation: func(h *apiHandler) (interface{}, error) {
				return h.CountWorkflowExecutions(context.Background(), &types.CountWorkflowExecutionsRequest{Domain: testDomain})
			},
			limiterSetup: func(h *apiHandler) {
				h.visibilityRateLimiter.(*mockPolicy).On("Allow", quotas.Info{Domain: testDomain}).Return(true).Once()
				h.wrapped.(*api.MockHandler).EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&types.CountWorkflowExecutionsResponse{}, nil).Times(1)
			},
		},
		{
			name: "DescribeTaskList uses user limiter with Allow",
			operation: func(h *apiHandler) (interface{}, error) {
				return h.DescribeTaskList(context.Background(), &types.DescribeTaskListRequest{
					Domain:   testDomain,
					TaskList: &types.TaskList{Name: "test-tasklist"},
				})
			},
			limiterSetup: func(h *apiHandler) {
				h.userRateLimiter.(*mockPolicy).On("Allow", quotas.Info{Domain: testDomain}).Return(true).Once()
				h.wrapped.(*api.MockHandler).EXPECT().DescribeTaskList(gomock.Any(), gomock.Any()).Return(&types.DescribeTaskListResponse{}, nil).Times(1)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := setupHandler(t)
			tc.limiterSetup(handler)

			resp, err := tc.operation(handler)

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			handler.userRateLimiter.(*mockPolicy).AssertExpectations(t)
			handler.workerRateLimiter.(*mockPolicy).AssertExpectations(t)
			handler.visibilityRateLimiter.(*mockPolicy).AssertExpectations(t)
		})
	}
}

type mockPolicy struct {
	mock.Mock
}

func (m *mockPolicy) Allow(info quotas.Info) bool {
	args := m.Called(info)
	return args.Bool(0)
}

func (m *mockPolicy) Wait(ctx context.Context, info quotas.Info) error {
	args := m.Called(ctx, info)
	return args.Error(0)
}
