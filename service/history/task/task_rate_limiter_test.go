// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package task

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/quotas"
	ctask "github.com/uber/cadence/common/task"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
)

type (
	noopTask struct {
		sync.RWMutex
		persistence.Task
		queueType QueueType
		shard     shard.Context
		attempt   int
		priority  int
		state     ctask.State
	}
)

func (s *noopTask) Execute() error {
	return nil
}

func (s *noopTask) HandleErr(err error) error {
	return nil
}

func (s *noopTask) RetryErr(err error) bool {
	return false
}

func (s *noopTask) Ack() {
	s.Lock()
	defer s.Unlock()
	if s.state != ctask.TaskStatePending {
		return
	}
	s.state = ctask.TaskStateAcked
}

func (s *noopTask) Nack() {
	s.Lock()
	defer s.Unlock()
	if s.state != ctask.TaskStatePending {
		return
	}
}

func (s *noopTask) Cancel() {
	s.Lock()
	defer s.Unlock()
	if s.state == ctask.TaskStatePending {
		s.state = ctask.TaskStateCanceled
	}
}

func (s *noopTask) State() ctask.State {
	s.RLock()
	defer s.RUnlock()
	return s.state
}

func (s *noopTask) Priority() int {
	s.RLock()
	defer s.RUnlock()
	return s.priority
}

func (s *noopTask) SetPriority(p int) {
	s.priority = p
}

func (s *noopTask) GetShard() shard.Context {
	return s.shard
}

func (s *noopTask) GetQueueType() QueueType {
	return s.queueType
}

func (s *noopTask) GetAttempt() int {
	return s.attempt
}

func (s *noopTask) GetInfo() persistence.Task {
	return s.Task
}

func (s *noopTask) SetInitialSubmitTime(submitTime time.Time) {
}

type taskRateLimiterMockDeps struct {
	ctrl                *gomock.Controller
	mockDomainCache     *cache.MockDomainCache
	mockShardController *shard.MockController
	mockICollection     *quotas.MockICollection
	dynamicClient       dynamicconfig.Client
}

func setupMocksForTaskRateLimiter(t *testing.T, mockQuotasCollection bool) (*taskRateLimiterImpl, *taskRateLimiterMockDeps) {
	ctrl := gomock.NewController(t)
	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockShardController := shard.NewMockController(ctrl)
	dynamicClient := dynamicconfig.NewInMemoryClient()

	deps := &taskRateLimiterMockDeps{
		ctrl:                ctrl,
		mockDomainCache:     mockDomainCache,
		mockShardController: mockShardController,
		dynamicClient:       dynamicClient,
	}

	config := config.New(
		dynamicconfig.NewCollection(
			dynamicClient,
			testlogger.New(t),
		),
		16,
		1024,
		false,
		"hostname",
	)

	rateLimiter := NewRateLimiter(
		testlogger.New(t),
		metrics.NewNoopMetricsClient(),
		deps.mockDomainCache,
		config,
		deps.mockShardController,
	)
	r, ok := rateLimiter.(*taskRateLimiterImpl)
	require.True(t, ok, "rate limiter type assertion failure")
	if mockQuotasCollection {
		deps.mockICollection = quotas.NewMockICollection(ctrl)
		r.limiters = deps.mockICollection
	}
	return r, deps
}

func TestRateLimiterRPS(t *testing.T) {
	r, deps := setupMocksForTaskRateLimiter(t, false)

	deps.mockShardController.EXPECT().NumShards().Return(8).AnyTimes()
	require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerGlobalDomainRPS, 100))

	l := r.limiters.For("test-domain").Limit()
	assert.Equal(t, 50, int(l))
}

func TestRateLimiterAllow(t *testing.T) {
	testCases := []struct {
		name       string
		task       Task
		setupMocks func(*taskRateLimiterMockDeps)
		expected   bool
	}{
		{
			name: "Not rate limited",
			task: &noopTask{
				Task: &persistence.DecisionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID: "test-domain-id",
					},
				},
			},
			setupMocks: func(deps *taskRateLimiterMockDeps) {
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerGlobalDomainRPS, 100))
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerEnableRateLimiter, true))
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerEnableRateLimiterShadowMode, false))
				deps.mockDomainCache.EXPECT().GetDomainName("test-domain-id").Return("test-domain", nil)
				limiter := quotas.NewMockLimiter(deps.ctrl)
				deps.mockICollection.EXPECT().For("test-domain").Return(limiter)
				limiter.EXPECT().Allow().Return(true)
			},
			expected: true,
		},
		{
			name: "Not rate limited - domain cache error ignored",
			task: &noopTask{
				Task: &persistence.DecisionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID: "test-domain-id",
					},
				},
			},
			setupMocks: func(deps *taskRateLimiterMockDeps) {
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerGlobalDomainRPS, 100))
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerEnableRateLimiter, true))
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerEnableRateLimiterShadowMode, false))
				deps.mockDomainCache.EXPECT().GetDomainName("test-domain-id").Return("", errors.New("cache error"))
				limiter := quotas.NewMockLimiter(deps.ctrl)
				deps.mockICollection.EXPECT().For("").Return(limiter)
				limiter.EXPECT().Allow().Return(true)
			},
			expected: true,
		},
		{
			name: "Rate limited - shadow mode",
			task: &noopTask{
				Task: &persistence.DecisionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID: "test-domain-id",
					},
				},
			},
			setupMocks: func(deps *taskRateLimiterMockDeps) {
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerGlobalDomainRPS, 100))
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerEnableRateLimiter, true))
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerEnableRateLimiterShadowMode, true))
				deps.mockDomainCache.EXPECT().GetDomainName("test-domain-id").Return("test-domain", nil)
				limiter := quotas.NewMockLimiter(deps.ctrl)
				deps.mockICollection.EXPECT().For("test-domain").Return(limiter)
				limiter.EXPECT().Allow().Return(false)
			},
			expected: true,
		},
		{
			name: "Rate limited",
			task: &noopTask{
				Task: &persistence.DecisionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID: "test-domain-id",
					},
				},
			},
			setupMocks: func(deps *taskRateLimiterMockDeps) {
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerGlobalDomainRPS, 100))
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerEnableRateLimiter, true))
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerEnableRateLimiterShadowMode, false))
				deps.mockDomainCache.EXPECT().GetDomainName("test-domain-id").Return("test-domain", nil)
				limiter := quotas.NewMockLimiter(deps.ctrl)
				deps.mockICollection.EXPECT().For("test-domain").Return(limiter)
				limiter.EXPECT().Allow().Return(false)
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r, deps := setupMocksForTaskRateLimiter(t, true)

			tc.setupMocks(deps)

			allow := r.Allow(tc.task)
			assert.Equal(t, tc.expected, allow)
		})
	}
}

func TestRateLimiterWait(t *testing.T) {
	testCases := []struct {
		name        string
		task        Task
		setupMocks  func(*taskRateLimiterMockDeps)
		expectErr   bool
		expectedErr string
	}{
		{
			name: "Not rate limited",
			task: &noopTask{
				Task: &persistence.DecisionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID: "test-domain-id",
					},
				},
			},
			setupMocks: func(deps *taskRateLimiterMockDeps) {
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerGlobalDomainRPS, 100))
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerEnableRateLimiter, true))
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerEnableRateLimiterShadowMode, false))
				deps.mockDomainCache.EXPECT().GetDomainName("test-domain-id").Return("test-domain", nil)
				limiter := quotas.NewMockLimiter(deps.ctrl)
				deps.mockICollection.EXPECT().For("test-domain").Return(limiter)
				limiter.EXPECT().Wait(gomock.Any()).Return(nil)
			},
			expectErr: false,
		},
		{
			name: "Not rate limited - domain cache error ignored",
			task: &noopTask{
				Task: &persistence.DecisionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID: "test-domain-id",
					},
				},
			},
			setupMocks: func(deps *taskRateLimiterMockDeps) {
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerGlobalDomainRPS, 100))
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerEnableRateLimiter, true))
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerEnableRateLimiterShadowMode, false))
				deps.mockDomainCache.EXPECT().GetDomainName("test-domain-id").Return("", errors.New("cache error"))
				limiter := quotas.NewMockLimiter(deps.ctrl)
				deps.mockICollection.EXPECT().For("").Return(limiter)
				limiter.EXPECT().Wait(gomock.Any()).Return(nil)
			},
			expectErr: false,
		},
		{
			name: "Rate limited - error",
			task: &noopTask{
				Task: &persistence.DecisionTask{
					WorkflowIdentifier: persistence.WorkflowIdentifier{
						DomainID: "test-domain-id",
					},
				},
			},
			setupMocks: func(deps *taskRateLimiterMockDeps) {
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerGlobalDomainRPS, 100))
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerEnableRateLimiter, true))
				require.NoError(t, deps.dynamicClient.UpdateValue(dynamicproperties.TaskSchedulerEnableRateLimiterShadowMode, false))
				deps.mockDomainCache.EXPECT().GetDomainName("test-domain-id").Return("test-domain", nil)
				limiter := quotas.NewMockLimiter(deps.ctrl)
				deps.mockICollection.EXPECT().For("test-domain").Return(limiter)
				limiter.EXPECT().Wait(gomock.Any()).Return(errors.New("wait error"))
			},
			expectErr:   true,
			expectedErr: "wait error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r, deps := setupMocksForTaskRateLimiter(t, true)

			tc.setupMocks(deps)

			err := r.Wait(context.Background(), tc.task)
			if tc.expectErr {
				assert.ErrorContains(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
