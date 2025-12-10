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

package handler

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
	"github.com/uber/cadence/service/matching/tasklist"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

func TestGetTaskListManager_OwnerShip(t *testing.T) {

	testCases := []struct {
		name                 string
		lookUpResult         string
		lookUpErr            error
		whoAmIResult         string
		whoAmIErr            error
		tasklistGuardEnabled bool

		expectedError error
	}{
		{
			name:                 "Not owned by current host",
			lookUpResult:         "A",
			whoAmIResult:         "B",
			tasklistGuardEnabled: true,

			expectedError: new(cadence_errors.TaskListNotOwnedByHostError),
		},
		{
			name:                 "LookupError",
			lookUpErr:            assert.AnError,
			tasklistGuardEnabled: true,
			expectedError:        assert.AnError,
		},
		{
			name:                 "WhoAmIError",
			whoAmIErr:            assert.AnError,
			tasklistGuardEnabled: true,
			expectedError:        assert.AnError,
		},
		{
			name:                 "when feature is not enabled, expect previous behaviour to continue",
			lookUpResult:         "A",
			whoAmIResult:         "B",
			tasklistGuardEnabled: false,

			expectedError: nil,
		},
	}

	for _, tc := range testCases {

		t.Run(tc.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			logger := log.NewNoop()

			mockTimeSource := clock.NewMockedTimeSourceAt(time.Now())
			taskManager := tasklist.NewTestTaskManager(t, logger, mockTimeSource)
			mockHistoryClient := history.NewMockClient(ctrl)
			mockMatchingClient := matching.NewMockClient(ctrl)
			mockDomainCache := cache.NewMockDomainCache(ctrl)
			resolverMock := membership.NewMockResolver(ctrl)
			resolverMock.EXPECT().Subscribe(service.Matching, "matching-engine", gomock.Any()).AnyTimes()
			mockShardDistributorExecutorClient := executorclient.NewMockClient(ctrl)

			// this is only if the call goes through
			mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(cache.CreateDomainCacheEntry(matchingTestDomainName), nil).AnyTimes()
			mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.CreateDomainCacheEntry(matchingTestDomainName), nil).AnyTimes()
			mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(matchingTestDomainName, nil).AnyTimes()

			config := defaultTestConfig()
			taskListEnabled := tc.tasklistGuardEnabled
			config.EnableTasklistOwnershipGuard = func(opts ...dynamicproperties.FilterOption) bool {
				return taskListEnabled
			}

			matchingEngine := NewEngine(
				taskManager,
				cluster.GetTestClusterMetadata(true),
				mockHistoryClient,
				mockMatchingClient,
				config,
				logger,
				metrics.NewClient(tally.NoopScope, metrics.Matching, metrics.HistogramMigration{}),
				mockDomainCache,
				resolverMock,
				isolationgroup.NewMockState(ctrl),
				mockTimeSource,
				mockShardDistributorExecutorClient,
			).(*matchingEngineImpl)

			resolverMock.EXPECT().Lookup(gomock.Any(), gomock.Any()).Return(
				membership.NewDetailedHostInfo("", tc.lookUpResult, make(membership.PortMap)), tc.lookUpErr,
			).AnyTimes()
			resolverMock.EXPECT().WhoAmI().Return(
				membership.NewDetailedHostInfo("", tc.whoAmIResult, make(membership.PortMap)), tc.whoAmIErr,
			).AnyTimes()

			_, err := matchingEngine.getTaskListManager(
				tasklist.NewTestTaskListID(t, "domain", "tasklist", persistence.TaskListTypeActivity),
				types.TaskListKindNormal,
			)
			if tc.expectedError != nil {
				assert.ErrorAs(t, err, &tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMembershipSubscriptionRecoversAfterPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		ctrl := gomock.NewController(t)

		r := resource.NewTest(t, ctrl, 0)
		r.MembershipResolver.EXPECT().Subscribe(service.Matching, "matching-engine", gomock.Any()).DoAndReturn(func(_, _, _ any) {
			panic("a panic has occurred")
		})

		engine := matchingEngineImpl{
			membershipResolver: r.MembershipResolver,
			config: &config.Config{
				EnableTasklistOwnershipGuard: func(opts ...dynamicproperties.FilterOption) bool { return true },
			},
			logger:   log.NewNoop(),
			shutdown: make(chan struct{}),
		}

		engine.runMembershipChangeLoop()
	})
}

func TestSubscriptionAndShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockResolver := membership.NewMockResolver(ctrl)

	shutdownWG := sync.WaitGroup{}
	shutdownWG.Add(1)

	mockDomainCache := cache.NewMockDomainCache(ctrl)

	engine := matchingEngineImpl{
		shutdownCompletion: &shutdownWG,
		membershipResolver: mockResolver,
		config: &config.Config{
			EnableTasklistOwnershipGuard: func(opts ...dynamicproperties.FilterOption) bool { return true },
		},
		shutdown:    make(chan struct{}),
		logger:      log.NewNoop(),
		domainCache: mockDomainCache,
	}

	mockResolver.EXPECT().WhoAmI().Return(membership.NewDetailedHostInfo("host2", "host2", nil), nil).AnyTimes()
	mockResolver.EXPECT().Subscribe(service.Matching, "matching-engine", gomock.Any())
	mockDomainCache.EXPECT().UnregisterDomainChangeCallback(service.Matching).Times(1)

	go engine.runMembershipChangeLoop()

	engine.Stop()
	assert.True(t, common.AwaitWaitGroup(&shutdownWG, 10*time.Second), "runMembershipChangeLoop has to be shut down")
}

func TestSubscriptionAndErrorReturned(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockResolver := membership.NewMockResolver(ctrl)

	mockDomainCache := cache.NewMockDomainCache(ctrl)

	shutdownWG := sync.WaitGroup{}
	shutdownWG.Add(1)

	membershipChangeHandledWG := sync.WaitGroup{}
	membershipChangeHandledWG.Add(1)

	engine := matchingEngineImpl{
		shutdownCompletion: &shutdownWG,
		membershipResolver: mockResolver,
		config: &config.Config{
			EnableTasklistOwnershipGuard: func(opts ...dynamicproperties.FilterOption) bool { return true },
		},
		shutdown:    make(chan struct{}),
		logger:      log.NewNoop(),
		domainCache: mockDomainCache,
	}

	// this should trigger the error case on a membership event
	// unfortunately, this is purely for code-coverage, no checks are involved
	mockResolver.EXPECT().WhoAmI().DoAndReturn(func() (membership.HostInfo, error) {
		membershipChangeHandledWG.Done()
		return membership.HostInfo{}, errors.New("failure")
	}).MinTimes(1)

	mockResolver.EXPECT().Subscribe(service.Matching, "matching-engine", gomock.Any()).Do(
		func(service string, name string, inc chan<- *membership.ChangedEvent) {
			m := membership.ChangedEvent{
				HostsAdded:   nil,
				HostsUpdated: nil,
				HostsRemoved: []string{"host123"},
			}
			inc <- &m
		})

	mockDomainCache.EXPECT().UnregisterDomainChangeCallback(service.Matching).Times(1)

	go engine.runMembershipChangeLoop()

	assert.True(t,
		common.AwaitWaitGroup(&membershipChangeHandledWG, 10*time.Second),
		"membership event is not handled",
	)

	engine.Stop()
	assert.True(t,
		common.AwaitWaitGroup(&shutdownWG, 10*time.Second),
		"runMembershipChangeLoop has to be shut down",
	)
}

func TestSubscribeToMembershipChangesQuitsIfSubscribeFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockResolver := membership.NewMockResolver(ctrl)
	mockDomainCache := cache.NewMockDomainCache(ctrl)

	logger, logs := testlogger.NewObserved(t)

	shutdownWG := sync.WaitGroup{}
	shutdownWG.Add(1)

	engine := matchingEngineImpl{
		shutdownCompletion: &shutdownWG,
		membershipResolver: mockResolver,
		config: &config.Config{
			EnableTasklistOwnershipGuard: func(opts ...dynamicproperties.FilterOption) bool { return true },
		},
		shutdown:    make(chan struct{}),
		logger:      logger,
		domainCache: mockDomainCache,
	}

	mockResolver.EXPECT().Subscribe(service.Matching, "matching-engine", gomock.Any()).
		Return(errors.New("failed to subscribe"))

	mockDomainCache.EXPECT().UnregisterDomainChangeCallback(service.Matching).AnyTimes()

	go engine.runMembershipChangeLoop()
	// we do not stop `engine` here - it has to shut down after failing to Subscribe

	assert.True(
		t,
		common.AwaitWaitGroup(&shutdownWG, 10*time.Second),
		"runMembershipChangeLoop should immediately shut down because of critical error",
	)

	// check we emitted error-message
	filteredLogs := logs.FilterMessage("Failed to subscribe to membership updates")
	assert.Equal(t, 1, filteredLogs.Len(), "error-message should be produced")
}

func TestGetTasklistManagerShutdownScenario(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockResolver := membership.NewMockResolver(ctrl)

	mockDomainCache := cache.NewMockDomainCache(ctrl)

	self := membership.NewDetailedHostInfo("self", "self", nil)

	mockResolver.EXPECT().WhoAmI().Return(self, nil).AnyTimes()
	mockDomainCache.EXPECT().UnregisterDomainChangeCallback(service.Matching).Times(1)

	mockExecutor := executorclient.NewMockExecutor[tasklist.ShardProcessor](ctrl)
	mockExecutor.EXPECT().GetShardProcess(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mockExecutor.EXPECT().IsOnboardedToSD().Return(false).AnyTimes()

	shutdownWG := sync.WaitGroup{}
	shutdownWG.Add(0)

	engine := matchingEngineImpl{
		shutdownCompletion: &shutdownWG,
		membershipResolver: mockResolver,
		config: &config.Config{
			EnableTasklistOwnershipGuard: func(opts ...dynamicproperties.FilterOption) bool { return true },
		},
		shutdown:    make(chan struct{}),
		logger:      log.NewNoop(),
		domainCache: mockDomainCache,
		executor:    mockExecutor,
	}

	// set this engine to be shutting down to trigger the tasklistGetTasklistByID guard
	engine.Stop()

	tl, _ := tasklist.NewIdentifier("domainid", "tl", 0)
	res, err := engine.getTaskListManager(tl, types.TaskListKindNormal)
	assertErr := &cadence_errors.TaskListNotOwnedByHostError{}
	assert.ErrorAs(t, err, &assertErr)
	assert.Nil(t, res)
}
