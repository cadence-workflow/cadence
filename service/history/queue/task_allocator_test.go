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

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/shard"
	htask "github.com/uber/cadence/service/history/task"
)

func TestVerifyActiveTask(t *testing.T) {
	domainID := "testDomainID"
	task := "testTask"
	tests := []struct {
		name                string
		setupMocks          func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager)
		expectedResult      bool
		expectedErrorString string
	}{
		{
			name: "Failed to get domain from cache, non-EntityNotExistsError",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(nil, errors.New("some error"))
			},
			expectedResult:      false,
			expectedErrorString: "some error",
		},
		{
			name: "Domain not found, EntityNotExistsError",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(nil, &types.EntityNotExistsError{})
			},
			expectedResult: false,
		},
		{
			name: "Domain is global and not active in current cluster",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				domainEntry := cache.NewGlobalDomainCacheEntryForTest(
					&persistence.DomainInfo{Name: domainID + "name"},
					nil,
					&persistence.DomainReplicationConfig{
						ActiveClusterName: "otherCluster",
					},
					1,
				)
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
			},
			expectedResult: false,
		},
		{
			name: "Domain is global and pending active in current cluster",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				endtime := int64(1)
				domainEntry := cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{Name: domainID + "name"},
					nil,
					true,
					&persistence.DomainReplicationConfig{
						ActiveClusterName: "currentCluster",
					},
					1,
					&endtime, 1, 1, 1,
				)
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
			},
			expectedResult:      false,
			expectedErrorString: "the domain is pending-active",
		},
		{
			name: "Domain is global and active in current cluster",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				domainEntry := cache.NewGlobalDomainCacheEntryForTest(
					&persistence.DomainInfo{Name: domainID + "name"},
					nil,
					&persistence.DomainReplicationConfig{
						ActiveClusterName: "currentCluster",
					},
					1,
				)
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
			},
			expectedResult: true,
		},
		{
			name: "Domain is active-active mode, task is not active in current cluster so should be skipped",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				domainEntry := activeActiveDomainEntry()
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
				activeClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&types.ActiveClusterInfo{
						ActiveClusterName: "another-cluster",
					}, nil)
			},
			expectedResult: false,
		},
		{
			name: "Domain is active-active mode, task is active in current cluster so should be processed",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				domainEntry := activeActiveDomainEntry()
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
				activeClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&types.ActiveClusterInfo{
						ActiveClusterName: "currentCluster",
					}, nil)
			},
			expectedResult: true,
		},
		{
			name: "Domain is active-active mode, activeness lookup returns error",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				domainEntry := activeActiveDomainEntry()
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
				activeClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("some error"))
			},
			expectedResult:      false,
			expectedErrorString: "some error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			// Create mocks
			mockShard := shard.NewMockContext(ctrl)
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			// Setup mock shard to return mock domain cache and logger
			mockShard.EXPECT().GetDomainCache().Return(mockDomainCache).AnyTimes()
			mockShard.EXPECT().GetService().Return(nil).AnyTimes() // Adjust based on your implementation

			activeClusterMgr := activecluster.NewMockManager(ctrl)

			// Create the task allocator
			allocator := &taskAllocatorImpl{
				currentClusterName: "currentCluster",
				shard:              mockShard,
				domainCache:        mockDomainCache,
				logger:             log.NewNoop(),
				activeClusterMgr:   activeClusterMgr,
			}

			tt.setupMocks(mockDomainCache, activeClusterMgr)
			result, err := allocator.VerifyActiveTask(domainID, "wfid", "rid", task)
			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedErrorString != "" {
				assert.Contains(t, err.Error(), tt.expectedErrorString)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVerifyFailoverActiveTask(t *testing.T) {
	domainID := "testDomainID"
	task := "testTask"

	tests := []struct {
		name                string
		targetDomainIDs     map[string]struct{}
		setupMocks          func(mockDomainCache *cache.MockDomainCache)
		expectedResult      bool
		expectedError       error
		expectedErrorString string
	}{
		{
			name: "Domain not in targetDomainIDs",
			targetDomainIDs: map[string]struct{}{
				"someOtherDomainID": {},
			},
			setupMocks: func(mockDomainCache *cache.MockDomainCache) {
				// No mocks needed since the domain is not in targetDomainIDs
			},
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "Domain in targetDomainIDs, GetDomainByID returns non-EntityNotExistsError",
			targetDomainIDs: map[string]struct{}{
				domainID: {},
			},
			setupMocks: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(nil, errors.New("some error"))
			},
			expectedResult:      false,
			expectedError:       errors.New("some error"),
			expectedErrorString: "some error",
		},
		{
			name: "Domain in targetDomainIDs, GetDomainByID returns EntityNotExistsError",
			targetDomainIDs: map[string]struct{}{
				domainID: {},
			},
			setupMocks: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(nil, &types.EntityNotExistsError{})
			},
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "Domain in targetDomainIDs, domain is pending active",
			targetDomainIDs: map[string]struct{}{
				domainID: {},
			},
			setupMocks: func(mockDomainCache *cache.MockDomainCache) {
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
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
			},
			expectedResult: false,
			expectedError:  htask.ErrTaskPendingActive,
		},
		{
			name: "Domain in targetDomainIDs, checkDomainPendingActive returns nil",
			targetDomainIDs: map[string]struct{}{
				domainID: {},
			},
			setupMocks: func(mockDomainCache *cache.MockDomainCache) {
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
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "Domain is local, should be skipped",
			targetDomainIDs: map[string]struct{}{
				domainID: {},
			},
			setupMocks: func(mockDomainCache *cache.MockDomainCache) {
				// Set up a domainEntry that is global and has a nil FailoverEndTime
				domainEntry := cache.NewDomainCacheEntryForTest(
					nil,
					nil,
					false, // IsGlobalDomain
					&persistence.DomainReplicationConfig{},
					1,
					nil,
					1,
					1,
					1,
				)
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
			},
			expectedResult: false,
			expectedError:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			// Create mocks
			mockShard := shard.NewMockContext(ctrl)
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			// Setup mock shard to return mock domain cache and logger
			mockShard.EXPECT().GetDomainCache().Return(mockDomainCache).AnyTimes()
			mockShard.EXPECT().GetService().Return(nil).AnyTimes() // Adjust based on your implementation

			// Create the task allocator
			allocator := &taskAllocatorImpl{
				currentClusterName: "currentCluster",
				shard:              mockShard,
				domainCache:        mockDomainCache,
				logger:             log.NewNoop(),
			}

			tt.setupMocks(mockDomainCache)
			result, err := allocator.VerifyFailoverActiveTask(tt.targetDomainIDs, domainID, "wfid", "rid", task)
			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
			} else if tt.expectedErrorString != "" {
				assert.Contains(t, err.Error(), tt.expectedErrorString)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVerifyStandbyTask(t *testing.T) {
	domainID := "testDomainID"
	task := "testTask"
	tests := []struct {
		name                string
		setupMocks          func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager)
		standbyCluster      string
		expectedResult      bool
		expectedError       error
		expectedErrorString string
	}{
		{
			name: "GetDomainByID returns non-EntityNotExistsError",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(nil, errors.New("some error"))
			},
			standbyCluster:      "standbyCluster",
			expectedResult:      false,
			expectedErrorString: "some error",
		},
		{
			name: "GetDomainByID returns EntityNotExistsError",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(nil, &types.EntityNotExistsError{})
			},
			standbyCluster: "standbyCluster",
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "Domain is not global",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				domainEntry := cache.NewLocalDomainCacheEntryForTest(
					&persistence.DomainInfo{},
					&persistence.DomainConfig{},
					"",
				)
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
			},
			standbyCluster: "standbyCluster",
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "Domain is global but not standby (active cluster name does not match standbyCluster)",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				domainEntry := cache.NewGlobalDomainCacheEntryForTest(
					&persistence.DomainInfo{},
					&persistence.DomainConfig{},
					&persistence.DomainReplicationConfig{
						ActiveClusterName: "anotherCluster",
					},
					0,
				)
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
			},
			standbyCluster: "standbyCluster",
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "Domain is global and standby, but checkDomainPendingActive returns error",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				endTime := time.Now().Add(time.Hour).UnixNano()
				domainEntry := cache.NewDomainCacheEntryForTest(nil, nil, true, &persistence.DomainReplicationConfig{
					ActiveClusterName: "currentCluster",
				}, 1, &endTime, 1, 1, 1)
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
			},
			standbyCluster:      "currentCluster",
			expectedResult:      false,
			expectedErrorString: htask.ErrTaskPendingActive.Error(),
		},
		{
			name: "Domain is global and standby, checkDomainPendingActive returns nil",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				domainEntry := cache.NewGlobalDomainCacheEntryForTest(
					&persistence.DomainInfo{},
					&persistence.DomainConfig{},
					&persistence.DomainReplicationConfig{
						ActiveClusterName: "standbyCluster",
					},
					0, // FailoverEndTime is zero
				)
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
			},
			standbyCluster: "standbyCluster",
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "Domain is local, should be skipped",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				// Set up a domainEntry that is global and has a nil FailoverEndTime
				domainEntry := cache.NewDomainCacheEntryForTest(
					nil,
					nil,
					false, // IsGlobalDomain
					&persistence.DomainReplicationConfig{},
					1,
					nil,
					1,
					1,
					1,
				)
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
			},
			standbyCluster: "standbyCluster",
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "Domain is active-active mode, task is not active in standby cluster so should be skipped",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				domainEntry := activeActiveDomainEntry()
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
				activeClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&types.ActiveClusterInfo{
						ActiveClusterName: "another-cluster",
					}, nil)
			},
			standbyCluster: "standbyCluster",
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "Domain is active-active mode, task is active in standby cluster so should be processed",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				domainEntry := activeActiveDomainEntry()
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
				activeClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&types.ActiveClusterInfo{
						ActiveClusterName: "standbyCluster",
					}, nil)
			},
			standbyCluster: "standbyCluster",
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "Domain is active-active mode, activeness lookup returns error",
			setupMocks: func(mockDomainCache *cache.MockDomainCache, activeClusterMgr *activecluster.MockManager) {
				domainEntry := activeActiveDomainEntry()
				mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil)
				activeClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("some error"))
			},
			standbyCluster: "standbyCluster",
			expectedResult: false,
			expectedError:  errors.New("some error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			// Create mocks
			mockShard := shard.NewMockContext(ctrl)
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			// Setup mock shard to return mock domain cache and logger
			mockShard.EXPECT().GetDomainCache().Return(mockDomainCache).AnyTimes()
			mockShard.EXPECT().GetService().Return(nil).AnyTimes() // Adjust based on your implementation

			activeClusterMgr := activecluster.NewMockManager(ctrl)

			// Create the task allocator
			allocator := &taskAllocatorImpl{
				currentClusterName: "currentCluster",
				shard:              mockShard,
				domainCache:        mockDomainCache,
				logger:             log.NewNoop(),
				activeClusterMgr:   activeClusterMgr,
			}

			tt.setupMocks(mockDomainCache, activeClusterMgr)
			result, err := allocator.VerifyStandbyTask(tt.standbyCluster, domainID, "wfid", "rid", task)
			assert.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
			} else if tt.expectedErrorString != "" {
				assert.Contains(t, err.Error(), tt.expectedErrorString)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsDomainNotRegistered(t *testing.T) {
	tests := []struct {
		name                string
		domainID            string
		mockFn              func(mockDomainCache *cache.MockDomainCache)
		expectedErrorString string
	}{
		{
			name: "domainID return error",
			mockFn: func(mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainByID("").Return(nil, fmt.Errorf("testError"))
			},
			domainID:            "",
			expectedErrorString: "testError",
		},
		{
			name:     "cannot get info",
			domainID: "testDomainID",
			mockFn: func(mockDomainCache *cache.MockDomainCache) {
				domainEntry := cache.NewDomainCacheEntryForTest(nil, nil, false, nil, 0, nil, 0, 0, 0)
				mockDomainCache.EXPECT().GetDomainByID("testDomainID").Return(domainEntry, nil)
			},
			expectedErrorString: "domain info is nil in cache",
		},
		{
			name:     "domain is deprecated",
			domainID: "testDomainID",
			mockFn: func(mockDomainCache *cache.MockDomainCache) {
				domainEntry := cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{Status: persistence.DomainStatusDeprecated}, nil, false, nil, 0, nil, 0, 0, 0)
				mockDomainCache.EXPECT().GetDomainByID("testDomainID").Return(domainEntry, nil)
			},
			expectedErrorString: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			// Create mocks
			mockShard := shard.NewMockContext(ctrl)
			mockDomainCache := cache.NewMockDomainCache(ctrl)

			// Setup mock shard to return mock domain cache and logger
			mockShard.EXPECT().GetDomainCache().Return(mockDomainCache).AnyTimes()
			mockShard.EXPECT().GetService().Return(nil).AnyTimes() // Adjust based on your implementation

			tt.mockFn(mockDomainCache)
			res, err := isDomainNotRegistered(mockShard, tt.domainID)
			if tt.expectedErrorString != "" {
				assert.ErrorContains(t, err, tt.expectedErrorString)
				assert.False(t, res)
			} else {
				assert.NoError(t, err)
				assert.True(t, res)
			}
		})
	}
}

func TestLockUnlock(t *testing.T) {
	// basic validation to ensure lock/unlock doesn't panic and get blocked
	allocator := &taskAllocatorImpl{}
	allocator.Lock()
	defer allocator.Unlock()
}

func activeActiveDomainEntry() *cache.DomainCacheEntry {
	// The specific versions are not important, as long as it's active-active.
	// Having a DomainReplicationConfig with non-zero length ActiveClusters is enough to make it active-active.
	return cache.NewDomainCacheEntryForTest(
		nil,
		nil,
		true,
		&persistence.DomainReplicationConfig{
			// Populate ActiveClusters to make it active-active
			ActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   1,
							},
						},
					},
				},
			},
		},
		1,
		nil,
		1,
		1,
		1,
	)
}
