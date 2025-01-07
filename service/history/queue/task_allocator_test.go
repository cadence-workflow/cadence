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

package queue

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/shard"
	htask "github.com/uber/cadence/service/history/task"
)

// TaskAllocatorSuite defines the suite for TaskAllocator tests
type TaskAllocatorSuite struct {
	suite.Suite
	controller      *gomock.Controller
	mockShard       *shard.MockContext
	mockDomainCache *cache.MockDomainCache
	mockLogger      *log.Logger
	allocator       *taskAllocatorImpl
	taskDomainID    string
	task            interface{}
}

func (s *TaskAllocatorSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())

	// Create mocks
	s.mockShard = shard.NewMockContext(s.controller)
	s.mockDomainCache = cache.NewMockDomainCache(s.controller)

	// Setup mock shard to return mock domain cache and logger
	s.mockShard.EXPECT().GetDomainCache().Return(s.mockDomainCache).AnyTimes()
	s.mockShard.EXPECT().GetService().Return(nil).AnyTimes() // Adjust based on your implementation

	// Create the task allocator
	s.allocator = &taskAllocatorImpl{
		currentClusterName: "currentCluster",
		shard:              s.mockShard,
		domainCache:        s.mockDomainCache,
		logger:             log.NewNoop(),
	}

	s.taskDomainID = "testDomainID"
	s.task = "testTask"
	s.allocator.Lock()
	s.allocator.Unlock()
}

func (s *TaskAllocatorSuite) TearDownTest() {
	s.controller.Finish()
}

// Run the test suite
func TestTaskAllocatorSuite(t *testing.T) {
	suite.Run(t, new(TaskAllocatorSuite))
}

func (s *TaskAllocatorSuite) TestVerifyActiveTask() {
	tests := []struct {
		name                string
		setupMocks          func()
		expectedResult      bool
		expectedErrorString string
	}{
		{
			name: "Domain not found, non-EntityNotExistsError",
			setupMocks: func() {
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(nil, errors.New("some error"))
			},
			expectedResult:      false,
			expectedErrorString: "some error",
		},
		{
			name: "Domain not found, EntityNotExistsError",
			setupMocks: func() {
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(nil, &types.EntityNotExistsError{})
			},
			expectedResult: true,
		},
		{
			name: "Domain is global and not active in current cluster",
			setupMocks: func() {
				domainEntry := cache.NewGlobalDomainCacheEntryForTest(nil, nil, &persistence.DomainReplicationConfig{
					ActiveClusterName: "otherCluster",
				}, 1)
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(domainEntry, nil)
			},
			expectedResult: false,
		},
		{
			name: "Domain is global and pending active in current cluster",
			setupMocks: func() {
				endtime := int64(1)
				domainEntry := cache.NewDomainCacheEntryForTest(nil, nil, true, &persistence.DomainReplicationConfig{
					ActiveClusterName: "currentCluster",
				}, 1, &endtime, 1, 1, 1)
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(domainEntry, nil)
			},
			expectedResult:      false,
			expectedErrorString: "the domain is pending-active",
		},
		{
			name: "Domain is global and active in current cluster",
			setupMocks: func() {
				domainEntry := cache.NewGlobalDomainCacheEntryForTest(nil, nil, &persistence.DomainReplicationConfig{
					ActiveClusterName: "currentCluster",
				}, 1)
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(domainEntry, nil)
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMocks()
			result, err := s.allocator.VerifyActiveTask(s.taskDomainID, s.task)
			assert.Equal(s.T(), tt.expectedResult, result)
			if tt.expectedErrorString != "" {
				assert.Contains(s.T(), err.Error(), tt.expectedErrorString)
			} else {
				assert.NoError(s.T(), err)
			}
		})
	}
}

func (s *TaskAllocatorSuite) TestVerifyFailoverActiveTask() {
	tests := []struct {
		name                string
		targetDomainIDs     map[string]struct{}
		setupMocks          func()
		expectedResult      bool
		expectedError       error
		expectedErrorString string
	}{
		{
			name: "Domain not in targetDomainIDs",
			targetDomainIDs: map[string]struct{}{
				"someOtherDomainID": {},
			},
			setupMocks: func() {
				// No mocks needed since the domain is not in targetDomainIDs
			},
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "Domain in targetDomainIDs, GetDomainByID returns non-EntityNotExistsError",
			targetDomainIDs: map[string]struct{}{
				s.taskDomainID: {},
			},
			setupMocks: func() {
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(nil, errors.New("some error"))
			},
			expectedResult:      false,
			expectedError:       errors.New("some error"),
			expectedErrorString: "some error",
		},
		{
			name: "Domain in targetDomainIDs, GetDomainByID returns EntityNotExistsError",
			targetDomainIDs: map[string]struct{}{
				s.taskDomainID: {},
			},
			setupMocks: func() {
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(nil, &types.EntityNotExistsError{})
			},
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "Domain in targetDomainIDs, domain is pending active",
			targetDomainIDs: map[string]struct{}{
				s.taskDomainID: {},
			},
			setupMocks: func() {
				// Set up a domainEntry that is global and has a non-nil FailoverEndTime
				endtime := int64(1)
				domainEntry := cache.NewDomainCacheEntryForTest(
					nil,
					nil,
					true, // IsGlobalDomain
					&persistence.DomainReplicationConfig{},
					1,
					&endtime, // FailoverEndTime is non-nil
					1,
					1,
					1,
				)
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(domainEntry, nil)
			},
			expectedResult: false,
			expectedError:  htask.ErrTaskPendingActive,
		},
		{
			name: "Domain in targetDomainIDs, checkDomainPendingActive returns nil",
			targetDomainIDs: map[string]struct{}{
				s.taskDomainID: {},
			},
			setupMocks: func() {
				// Set up a domainEntry that is global and has a nil FailoverEndTime
				domainEntry := cache.NewDomainCacheEntryForTest(
					nil,
					nil,
					true, // IsGlobalDomain
					&persistence.DomainReplicationConfig{},
					1,
					nil, // FailoverEndTime is nil
					1,
					1,
					1,
				)
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(domainEntry, nil)
			},
			expectedResult: true,
			expectedError:  nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMocks()
			result, err := s.allocator.VerifyFailoverActiveTask(tt.targetDomainIDs, s.taskDomainID, s.task)
			assert.Equal(s.T(), tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.Equal(s.T(), tt.expectedError, err)
			} else if tt.expectedErrorString != "" {
				assert.Contains(s.T(), err.Error(), tt.expectedErrorString)
			} else {
				assert.NoError(s.T(), err)
			}
		})
	}
}

func (s *TaskAllocatorSuite) TestVerifyStandbyTask() {
	tests := []struct {
		name                string
		setupMocks          func()
		standbyCluster      string
		expectedResult      bool
		expectedError       error
		expectedErrorString string
	}{
		{
			name: "GetDomainByID returns non-EntityNotExistsError",
			setupMocks: func() {
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(nil, errors.New("some error"))
			},
			standbyCluster:      "standbyCluster",
			expectedResult:      false,
			expectedErrorString: "some error",
		},
		{
			name: "GetDomainByID returns EntityNotExistsError",
			setupMocks: func() {
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(nil, &types.EntityNotExistsError{})
			},
			standbyCluster: "standbyCluster",
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "Domain is not global",
			setupMocks: func() {
				domainEntry := cache.NewLocalDomainCacheEntryForTest(
					&persistence.DomainInfo{},
					&persistence.DomainConfig{},
					"",
				)
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(domainEntry, nil)
			},
			standbyCluster: "standbyCluster",
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "Domain is global but not standby (active cluster name does not match standbyCluster)",
			setupMocks: func() {
				domainEntry := cache.NewGlobalDomainCacheEntryForTest(
					&persistence.DomainInfo{},
					&persistence.DomainConfig{},
					&persistence.DomainReplicationConfig{
						ActiveClusterName: "anotherCluster",
					},
					0,
				)
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(domainEntry, nil)
			},
			standbyCluster: "standbyCluster",
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "Domain is global and standby, but checkDomainPendingActive returns error",
			setupMocks: func() {
				endTime := time.Now().Add(time.Hour).UnixNano()
				domainEntry := cache.NewDomainCacheEntryForTest(nil, nil, true, &persistence.DomainReplicationConfig{
					ActiveClusterName: "currentCluster",
				}, 1, &endTime, 1, 1, 1)
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(domainEntry, nil)
			},
			standbyCluster:      "currentCluster",
			expectedResult:      false,
			expectedErrorString: htask.ErrTaskPendingActive.Error(),
		},
		{
			name: "Domain is global and standby, checkDomainPendingActive returns nil",
			setupMocks: func() {
				domainEntry := cache.NewGlobalDomainCacheEntryForTest(
					&persistence.DomainInfo{},
					&persistence.DomainConfig{},
					&persistence.DomainReplicationConfig{
						ActiveClusterName: "standbyCluster",
					},
					0, // FailoverEndTime is zero
				)
				s.mockDomainCache.EXPECT().GetDomainByID(s.taskDomainID).Return(domainEntry, nil)
			},
			standbyCluster: "standbyCluster",
			expectedResult: true,
			expectedError:  nil,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.setupMocks()
			result, err := s.allocator.VerifyStandbyTask(tt.standbyCluster, s.taskDomainID, s.task)
			assert.Equal(s.T(), tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.Equal(s.T(), tt.expectedError, err)
			} else if tt.expectedErrorString != "" {
				assert.Contains(s.T(), err.Error(), tt.expectedErrorString)
			} else {
				assert.NoError(s.T(), err)
			}
		})
	}
}

func (s *TaskAllocatorSuite) TestIsDomainNotRegistered() {
	tests := []struct {
		name                string
		domainID            string
		mockFn              func()
		expectedErrorString string
	}{
		{
			name: "domainID return error",
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainByID("").Return(nil, fmt.Errorf("testError"))
			},
			domainID:            "",
			expectedErrorString: "testError",
		},
		{
			name:     "cannot get info",
			domainID: "testDomainID",
			mockFn: func() {
				domainEntry := cache.NewDomainCacheEntryForTest(nil, nil, false, nil, 0, nil, 0, 0, 0)
				s.mockDomainCache.EXPECT().GetDomainByID("testDomainID").Return(domainEntry, nil)
			},
			expectedErrorString: "domain info is nil in cache",
		},
		{
			name:     "domain is deprecated",
			domainID: "testDomainID",
			mockFn: func() {
				domainEntry := cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{Status: persistence.DomainStatusDeprecated}, nil, false, nil, 0, nil, 0, 0, 0)
				s.mockDomainCache.EXPECT().GetDomainByID("testDomainID").Return(domainEntry, nil)
			},
			expectedErrorString: "",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			tt.mockFn()
			res, err := isDomainNotRegistered(s.mockShard, tt.domainID)
			if tt.expectedErrorString != "" {
				assert.ErrorContains(s.T(), err, tt.expectedErrorString)
				assert.False(s.T(), res)
			} else {
				assert.NoError(s.T(), err)
				assert.True(s.T(), res)
			}
		})
	}
}
