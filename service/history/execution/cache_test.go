// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package execution

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/shard"
)

type (
	historyCacheSuite struct {
		suite.Suite
		*require.Assertions

		controller *gomock.Controller
		mockShard  *shard.TestContext

		cache Cache
	}
)

func TestHistoryCacheSuite(t *testing.T) {
	s := new(historyCacheSuite)
	suite.Run(t, s)
}

func (s *historyCacheSuite) SetupSuite() {
}

func (s *historyCacheSuite) TearDownSuite() {
}

func (s *historyCacheSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockShard = shard.NewTestContext(
		s.T(),
		s.controller,
		&persistence.ShardInfo{
			ShardID:          0,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		config.NewForTest(),
	)
}

func (s *historyCacheSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *historyCacheSuite) TestHistoryCacheBasic() {
	s.cache = NewCache(s.mockShard)

	domainID := "test_domain_id"
	domainName := "test_domain_name"
	execution1 := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      uuid.New(),
	}
	s.mockShard.Resource.DomainCache.EXPECT().GetDomainName(gomock.Any()).Return(domainName, nil).AnyTimes()
	mockMS1 := NewMockMutableState(s.controller)
	ctx, release, err := s.cache.GetOrCreateWorkflowExecutionForBackground(domainID, execution1)
	s.Nil(err)
	ctx.(*contextImpl).mutableState = mockMS1
	release(nil)
	ctx, release, err = s.cache.GetOrCreateWorkflowExecutionForBackground(domainID, execution1)
	s.Nil(err)
	s.Equal(mockMS1, ctx.(*contextImpl).mutableState)
	release(nil)

	execution2 := types.WorkflowExecution{
		WorkflowID: "some random workflow ID",
		RunID:      uuid.New(),
	}
	ctx, release, err = s.cache.GetOrCreateWorkflowExecutionForBackground(domainID, execution2)
	s.Nil(err)
	s.NotEqual(mockMS1, ctx.(*contextImpl).mutableState)
	release(nil)
}

func (s *historyCacheSuite) TestHistoryCachePinning() {
	s.mockShard.GetConfig().HistoryCacheMaxSize = dynamicconfig.GetIntPropertyFn(2)
	domainID := "test_domain_id"
	s.cache = NewCache(s.mockShard)
	we := types.WorkflowExecution{
		WorkflowID: "wf-cache-test-pinning",
		RunID:      uuid.New(),
	}
	s.mockShard.Resource.DomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test_domain_name", nil).AnyTimes()
	ctx, release, err := s.cache.GetOrCreateWorkflowExecutionForBackground(domainID, we)
	s.Nil(err)

	we2 := types.WorkflowExecution{
		WorkflowID: "wf-cache-test-pinning",
		RunID:      uuid.New(),
	}

	// Cache is full because ctx is pinned, should get an error now
	_, _, err2 := s.cache.GetOrCreateWorkflowExecutionForBackground(domainID, we2)
	s.NotNil(err2)

	// Now release the ctx, this should unpin it.
	release(err2)

	_, release2, err3 := s.cache.GetOrCreateWorkflowExecutionForBackground(domainID, we2)
	s.Nil(err3)
	release2(err3)

	// Old ctx should be evicted.
	newContext, release, err4 := s.cache.GetOrCreateWorkflowExecutionForBackground(domainID, we)
	s.Nil(err4)
	s.False(ctx == newContext)
	release(err4)
}

func (s *historyCacheSuite) TestHistoryCacheClear() {
	s.mockShard.GetConfig().HistoryCacheMaxSize = dynamicconfig.GetIntPropertyFn(20)
	domainID := "test_domain_id"
	s.cache = NewCache(s.mockShard)
	we := types.WorkflowExecution{
		WorkflowID: "wf-cache-test-clear",
		RunID:      uuid.New(),
	}
	s.mockShard.Resource.DomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test_domain_name", nil).AnyTimes()
	ctx, release, err := s.cache.GetOrCreateWorkflowExecutionForBackground(domainID, we)
	s.Nil(err)
	// since we are just testing whether the release function will clear the cache
	// all we need is a fake msBuilder
	ctx.(*contextImpl).mutableState = &mutableStateBuilder{}
	release(nil)

	// since last time, the release function receive a nil error
	// the ms builder will not be cleared
	ctx, release, err = s.cache.GetOrCreateWorkflowExecutionForBackground(domainID, we)
	s.Nil(err)
	s.NotNil(ctx.(*contextImpl).mutableState)
	release(errors.New("some random error message"))

	// since last time, the release function receive a non-nil error
	// the ms builder will be cleared
	ctx, release, err = s.cache.GetOrCreateWorkflowExecutionForBackground(domainID, we)
	s.Nil(err)
	s.Nil(ctx.(*contextImpl).mutableState)
	release(nil)
}

func (s *historyCacheSuite) TestHistoryCacheConcurrentAccess() {
	s.mockShard.GetConfig().HistoryCacheMaxSize = dynamicconfig.GetIntPropertyFn(20)
	domainID := "test_domain_id"
	s.cache = NewCache(s.mockShard)
	we := types.WorkflowExecution{
		WorkflowID: "wf-cache-test-pinning",
		RunID:      uuid.New(),
	}

	coroutineCount := 50
	waitGroup := &sync.WaitGroup{}
	stopChan := make(chan struct{})
	testFn := func() {
		<-stopChan
		ctx, release, err := s.cache.GetOrCreateWorkflowExecutionForBackground(domainID, we)
		s.Nil(err)
		// since each time the builder is reset to nil
		s.Nil(ctx.(*contextImpl).mutableState)
		// since we are just testing whether the release function will clear the cache
		// all we need is a fake msBuilder
		ctx.(*contextImpl).mutableState = &mutableStateBuilder{}
		release(errors.New("some random error message"))
		waitGroup.Done()
	}
	s.mockShard.Resource.DomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain-name", nil).AnyTimes()
	for i := 0; i < coroutineCount; i++ {
		waitGroup.Add(1)
		go testFn()
	}
	close(stopChan)
	waitGroup.Wait()

	ctx, release, err := s.cache.GetOrCreateWorkflowExecutionForBackground(domainID, we)
	s.Nil(err)
	// since we are just testing whether the release function will clear the cache
	// all we need is a fake msBuilder
	s.Nil(ctx.(*contextImpl).mutableState)
	release(nil)
}

func (s *historyCacheSuite) TestGetOrCreateCurrentWorkflowExecution() {
	tests := []struct {
		name          string
		mockSetup     func(c *cacheImpl, mockContext *MockContext, ctx context.Context)
		disabled      bool
		cacheCtxNil   bool
		noOpReleaseFn bool
		err           error
	}{
		{
			name:      "success - cache enabled - cache miss",
			mockSetup: func(_ *cacheImpl, _ *MockContext, _ context.Context) {},
		},
		{
			name:          "success - cache disabled",
			mockSetup:     func(_ *cacheImpl, _ *MockContext, _ context.Context) {},
			disabled:      true,
			noOpReleaseFn: true,
		},
		{
			name: "success - cache enabled - cache hit",
			mockSetup: func(c *cacheImpl, mockContext *MockContext, ctx context.Context) {
				key := definition.NewWorkflowIdentifier(constants.TestDomainID, constants.TestWorkflowID, "")
				_, _ = c.Cache.PutIfNotExist(key, mockContext)
				mockContext.EXPECT().Lock(ctx).Return(nil).Times(1)
			},
		},
		{
			name: "error - cache enabled - cache hit error on lock",
			mockSetup: func(cache *cacheImpl, mockContext *MockContext, ctx context.Context) {
				key := definition.NewWorkflowIdentifier(constants.TestDomainID, constants.TestWorkflowID, "")
				_, _ = cache.Cache.PutIfNotExist(key, mockContext)
				mockContext.EXPECT().Lock(ctx).Return(errors.New("test-error")).Times(1)
			},
			cacheCtxNil: true,
			err:         errors.New("test-error"),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			c := &cacheImpl{
				Cache: cache.New(&cache.Options{
					InitialCapacity: s.mockShard.GetConfig().HistoryCacheInitialSize(),
					TTL:             s.mockShard.GetConfig().HistoryCacheTTL(),
					Pin:             true,
					MaxCount:        s.mockShard.GetConfig().HistoryCacheMaxSize(),
				}),
				shard:            s.mockShard,
				executionManager: s.mockShard.GetExecutionManager(),
				logger:           s.mockShard.GetLogger().WithTags(tag.ComponentHistoryCache),
				metricsClient:    s.mockShard.GetMetricsClient(),
				config:           s.mockShard.GetConfig(),
			}
			mockContext := NewMockContext(s.controller)

			c.disabled = tt.disabled
			ctx := context.Background()
			tt.mockSetup(c, mockContext, ctx)

			cacheCtx, releaseFunc, err := c.GetOrCreateCurrentWorkflowExecution(ctx, constants.TestDomainID, constants.TestWorkflowID)

			if tt.cacheCtxNil {
				s.Nil(cacheCtx)
			} else {
				s.NotNil(cacheCtx)
			}

			if tt.noOpReleaseFn {
				s.True(reflect.ValueOf(NoopReleaseFn).Pointer() == reflect.ValueOf(releaseFunc).Pointer())
			}

			if tt.err != nil {
				s.Error(err)
				s.Equal(tt.err, err)
				s.Nil(releaseFunc)
			} else {
				s.NoError(err)
				s.NotNil(releaseFunc)
			}
		})
	}
}

func (s *historyCacheSuite) TestGetOrCreateWorkflowExecution() {
	tests := []struct {
		name        string
		workflowID  string
		runID       string
		cacheCtxNil bool
		mockSetup   func(mockShard *shard.TestContext)
		err         error
	}{
		{
			name:        "error - empty workflow ID",
			workflowID:  "",
			runID:       constants.TestRunID,
			mockSetup:   func(_ *shard.TestContext) {},
			cacheCtxNil: true,
			err:         &types.BadRequestError{Message: "Can't load workflow execution.  WorkflowId not set."},
		},
		{
			name:       "error - get domain cache error",
			workflowID: constants.TestWorkflowID,
			runID:      constants.TestRunID,
			mockSetup: func(mockShard *shard.TestContext) {
				mockShard.GetDomainCache().(*cache.MockDomainCache).EXPECT().GetDomainName(constants.TestDomainID).Return("", errors.New("test-error")).Times(1)
			},
			cacheCtxNil: true,
			err:         errors.New("test-error"),
		},
		{
			name: "error - not valid runID",
			mockSetup: func(mockShard *shard.TestContext) {
				mockShard.GetDomainCache().(*cache.MockDomainCache).EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).Times(1)
			},
			workflowID:  constants.TestWorkflowID,
			runID:       "invalid-run-id",
			cacheCtxNil: true,
			err:         &types.BadRequestError{Message: "RunID is not valid UUID."},
		},
		{
			name: "error - empty runID retry failed",
			mockSetup: func(mockShard *shard.TestContext) {
				mockShard.GetDomainCache().(*cache.MockDomainCache).EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).Times(1)
				req := &persistence.GetCurrentExecutionRequest{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					DomainName: constants.TestDomainName,
				}
				mockShard.GetExecutionManager().(*mocks.ExecutionManager).On("GetCurrentExecution", mock.Anything, req).Return(nil, errors.New("test-error")).Times(1)
			},
			workflowID:  constants.TestWorkflowID,
			runID:       "",
			cacheCtxNil: true,
			err:         errors.New("test-error"),
		},
		{
			name: "success - runID provided",
			mockSetup: func(mockShard *shard.TestContext) {
				mockShard.GetDomainCache().(*cache.MockDomainCache).EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).Times(1)
			},
			workflowID: constants.TestWorkflowID,
			runID:      constants.TestRunID,
			err:        nil,
		},
		{
			name: "success - runID not provided",
			mockSetup: func(mockShard *shard.TestContext) {
				mockShard.GetDomainCache().(*cache.MockDomainCache).EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).Times(1)
				req := &persistence.GetCurrentExecutionRequest{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					DomainName: constants.TestDomainName,
				}
				resp := &persistence.GetCurrentExecutionResponse{
					RunID: constants.TestRunID,
				}
				mockShard.GetExecutionManager().(*mocks.ExecutionManager).On("GetCurrentExecution", mock.Anything, req).Return(resp, nil).Times(1)
			},
			workflowID: constants.TestWorkflowID,
			runID:      "",
			err:        nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.cache = NewCache(s.mockShard)

			ctx := context.Background()
			tt.mockSetup(s.mockShard)

			cacheCtx, releaseFunc, err := s.cache.GetOrCreateWorkflowExecution(ctx, constants.TestDomainID, types.WorkflowExecution{WorkflowID: tt.workflowID, RunID: tt.runID})

			if tt.cacheCtxNil {
				s.Nil(cacheCtx)
			} else {
				s.NotNil(cacheCtx)
			}

			if tt.err != nil {
				s.Error(err)
				s.Equal(tt.err, err)
				s.Nil(releaseFunc)
			} else {
				s.NoError(err)
				s.NotNil(releaseFunc)
			}
		})
	}
}

func (s *historyCacheSuite) TestGetAndCreateWorkflowExecution() {
	ctx := context.Background()

	tests := []struct {
		name        string
		domainID    string
		execution   types.WorkflowExecution
		mockSetup   func(mockShard *shard.TestContext, mockContext *MockContext, c *cacheImpl)
		cacheHit    bool
		cacheCtxNil bool
		err         error
	}{
		{
			name:     "error - could not validate workflow execution info",
			domainID: constants.TestDomainID,
			execution: types.WorkflowExecution{
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
			},
			mockSetup: func(mockShard *shard.TestContext, _ *MockContext, _ *cacheImpl) {
				mockShard.GetDomainCache().(*cache.MockDomainCache).EXPECT().GetDomainName(constants.TestDomainID).Return("", errors.New("validate-error")).Times(1)
			},
			cacheHit:    false,
			cacheCtxNil: true,
			err:         errors.New("validate-error"),
		},
		{
			name:     "success - cache miss",
			domainID: constants.TestDomainID,
			execution: types.WorkflowExecution{
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
			},
			mockSetup: func(mockShard *shard.TestContext, _ *MockContext, _ *cacheImpl) {
				mockShard.GetDomainCache().(*cache.MockDomainCache).EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).Times(1)
			},
			cacheHit:    false,
			cacheCtxNil: true,
			err:         nil,
		},
		{
			name:     "error - cache hit error on lock",
			domainID: constants.TestDomainID,
			execution: types.WorkflowExecution{
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
			},
			mockSetup: func(mockShard *shard.TestContext, mockContext *MockContext, c *cacheImpl) {
				mockShard.GetDomainCache().(*cache.MockDomainCache).EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).Times(1)
				key := definition.NewWorkflowIdentifier(constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID)
				_, _ = c.Cache.PutIfNotExist(key, mockContext)
				mockContext.EXPECT().Lock(ctx).Return(errors.New("lock-error")).Times(1)
			},
			cacheHit:    false,
			cacheCtxNil: true,
			err:         errors.New("lock-error"),
		},
		{
			name:     "success - cache hit",
			domainID: constants.TestDomainID,
			execution: types.WorkflowExecution{
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
			},
			mockSetup: func(mockShard *shard.TestContext, mockContext *MockContext, c *cacheImpl) {
				mockShard.GetDomainCache().(*cache.MockDomainCache).EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).Times(1)
				key := definition.NewWorkflowIdentifier(constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID)
				_, _ = c.Cache.PutIfNotExist(key, mockContext)
				mockContext.EXPECT().Lock(ctx).Return(nil).Times(1)
			},
			cacheHit: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.cache = NewCache(s.mockShard)

			mockContext := NewMockContext(s.controller)

			tt.mockSetup(s.mockShard, mockContext, s.cache.(*cacheImpl))

			cacheCtx, dbCtx, release, cacheHit, err := s.cache.GetAndCreateWorkflowExecution(ctx, tt.domainID, tt.execution)

			s.Equal(tt.cacheHit, cacheHit)

			if tt.cacheCtxNil {
				s.Nil(cacheCtx)
			} else {
				s.NotNil(cacheCtx)
				s.Equal(mockContext, cacheCtx)
			}

			if tt.err != nil {
				s.Error(err)
				s.Equal(tt.err, err)
				s.Nil(dbCtx)
				s.Nil(release)
			} else {
				s.NoError(err)
				s.NotNil(dbCtx)
				s.NotNil(release)
			}
		})
	}
}

func (s *historyCacheSuite) TestGetOrCreateWorkflowExecutionWithTimeout() {
	s.cache = NewCache(s.mockShard)

	workflowExecution := types.WorkflowExecution{
		WorkflowID: constants.TestWorkflowID,
		RunID:      constants.TestRunID,
	}

	newCtx := NewContext(constants.TestDomainID, workflowExecution, s.mockShard, s.mockShard.GetExecutionManager(), s.mockShard.GetLogger())

	s.mockShard.GetDomainCache().(*cache.MockDomainCache).EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).Times(1)
	key := definition.NewWorkflowIdentifier(constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID)
	_, _ = s.cache.(*cacheImpl).Cache.PutIfNotExist(key, newCtx)

	// getting the lock to guarantee that the context will time out
	_ = newCtx.Lock(context.Background())
	defer newCtx.Unlock()

	cacheCtx, release, err := s.cache.GetOrCreateWorkflowExecutionWithTimeout(constants.TestDomainID, workflowExecution, 0)

	s.Nil(cacheCtx)
	s.Nil(release)
	s.Error(err)
	s.Equal(context.DeadlineExceeded, err)
}
