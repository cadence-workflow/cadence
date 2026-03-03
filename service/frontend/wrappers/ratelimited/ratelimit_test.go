package ratelimited

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/api"
)

const testDomain = "test-domain"

func setupHandler(t *testing.T) *apiHandler {
	return setupHandlerWithDC(t, nil)
}

func setupHandlerWithDC(t *testing.T, dc *dynamicconfig.Collection) *apiHandler {
	ctrl := gomock.NewController(t)
	mockHandler := api.NewMockHandler(ctrl)
	mockDomainCache := cache.NewMockDomainCache(ctrl)

	var callerBypass quotas.CallerBypass
	if dc != nil {
		callerBypass = quotas.NewCallerBypass(dc.GetListProperty(dynamicproperties.RateLimiterBypassCallerTypes))
	} else {
		callerBypass = quotas.NewCallerBypass(nil)
	}

	return NewAPIHandler(
		mockHandler,
		mockDomainCache,
		&mockPolicy{}, // userRateLimiter
		&mockPolicy{}, // workerRateLimiter
		&mockPolicy{}, // visibilityRateLimiter
		&mockPolicy{}, // asyncRateLimiter
		func(domain string) time.Duration { return 0 }, // maxWorkerPollDelay
		callerBypass,
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

func TestCallerTypeBypass(t *testing.T) {
	tests := []struct {
		name              string
		bypassCallerTypes []interface{}
		callerType        types.CallerType
		rateLimitBlocks   bool
		expectBypass      bool
	}{
		{
			name:              "Bypass CLI caller when configured",
			bypassCallerTypes: []interface{}{"cli"},
			callerType:        types.CallerTypeCLI,
			rateLimitBlocks:   true,
			expectBypass:      true,
		},
		{
			name:              "Bypass UI caller when configured",
			bypassCallerTypes: []interface{}{"ui"},
			callerType:        types.CallerTypeUI,
			rateLimitBlocks:   true,
			expectBypass:      true,
		},
		{
			name:              "Don't bypass when caller type not in list",
			bypassCallerTypes: []interface{}{"cli"},
			callerType:        types.CallerTypeUI,
			rateLimitBlocks:   true,
			expectBypass:      false,
		},
		{
			name:              "Don't bypass with empty list",
			bypassCallerTypes: []interface{}{},
			callerType:        types.CallerTypeCLI,
			rateLimitBlocks:   true,
			expectBypass:      false,
		},
		{
			name:              "Multiple caller types in bypass list",
			bypassCallerTypes: []interface{}{"cli", "ui"},
			callerType:        types.CallerTypeUI,
			rateLimitBlocks:   true,
			expectBypass:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := dynamicconfig.NewInMemoryClient()
			client.UpdateValue(dynamicproperties.RateLimiterBypassCallerTypes, tt.bypassCallerTypes)
			dc := dynamicconfig.NewCollection(client, testlogger.New(t))

			handler := setupHandlerWithDC(t, dc)
			ctx := types.ContextWithCallerInfo(context.Background(), types.NewCallerInfo(tt.callerType))

			if tt.rateLimitBlocks {
				handler.userRateLimiter.(*mockPolicy).On("Allow", quotas.Info{Domain: testDomain}).Return(false).Once()
			} else {
				handler.userRateLimiter.(*mockPolicy).On("Allow", quotas.Info{Domain: testDomain}).Return(true).Once()
			}

			err := handler.allowDomain(ctx, ratelimitTypeUser, testDomain)

			if tt.expectBypass {
				assert.NoError(t, err, "Expected bypass to allow request")
			} else {
				if tt.rateLimitBlocks {
					assert.Error(t, err, "Expected rate limit error")
					assert.Equal(t, newErrRateLimited(), err)
				} else {
					assert.NoError(t, err)
				}
			}

			handler.userRateLimiter.(*mockPolicy).AssertExpectations(t)
		})
	}
}

func TestCallerTypeBypassWithWait(t *testing.T) {
	tests := []struct {
		name              string
		bypassCallerTypes []interface{}
		callerType        types.CallerType
		expectBypass      bool
	}{
		{
			name:              "Bypass in Wait nil error path when configured",
			bypassCallerTypes: []interface{}{"cli"},
			callerType:        types.CallerTypeCLI,
			expectBypass:      true,
		},
		{
			name:              "Don't bypass in Wait nil error path when not configured",
			bypassCallerTypes: []interface{}{},
			callerType:        types.CallerTypeCLI,
			expectBypass:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := dynamicconfig.NewInMemoryClient()
			client.UpdateValue(dynamicproperties.RateLimiterBypassCallerTypes, tt.bypassCallerTypes)
			dc := dynamicconfig.NewCollection(client, testlogger.New(t))

			handler := setupHandlerWithDC(t, dc)
			handler.maxWorkerPollDelay = func(domain string) time.Duration { return 1 * time.Millisecond }
			ctx := types.ContextWithCallerInfo(context.Background(), types.NewCallerInfo(tt.callerType))

			// Wait returns an error, but waitCtx.Err() is nil (case nil path)
			handler.workerRateLimiter.(*mockPolicy).On("Wait", mock.Anything, quotas.Info{Domain: testDomain}).Return(assert.AnError).Once()

			err := handler.allowDomain(ctx, ratelimitTypeWorkerPoll, testDomain)

			if tt.expectBypass {
				assert.NoError(t, err, "Expected bypass to allow request")
			} else {
				assert.Error(t, err, "Expected rate limit error")
				assert.Equal(t, newErrRateLimited(), err)
			}

			handler.workerRateLimiter.(*mockPolicy).AssertExpectations(t)
		})
	}
}
