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

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig"
	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log/loggerimpl"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
	"github.com/uber/cadence/service/matching/tasklist"
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
			logger := loggerimpl.NewNopLogger()

			mockTimeSource := clock.NewMockedTimeSourceAt(time.Now())
			taskManager := tasklist.NewTestTaskManager(t, logger, mockTimeSource)
			mockHistoryClient := history.NewMockClient(ctrl)
			mockDomainCache := cache.NewMockDomainCache(ctrl)
			resolverMock := membership.NewMockResolver(ctrl)
			resolverMock.EXPECT().Subscribe(service.Matching, "matching-engine", gomock.Any()).AnyTimes()

			// this is only if the call goes through
			mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(cache.CreateDomainCacheEntry(matchingTestDomainName), nil).AnyTimes()
			mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.CreateDomainCacheEntry(matchingTestDomainName), nil).AnyTimes()
			mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(matchingTestDomainName, nil).AnyTimes()

			config := defaultTestConfig()
			taskListEnabled := tc.tasklistGuardEnabled
			config.EnableTasklistOwnershipGuard = func(opts ...dynamicconfig.FilterOption) bool {
				return taskListEnabled
			}

			matchingEngine := NewEngine(
				taskManager,
				cluster.GetTestClusterMetadata(true),
				mockHistoryClient,
				nil,
				config,
				logger,
				metrics.NewClient(tally.NoopScope, metrics.Matching),
				mockDomainCache,
				resolverMock,
				nil,
				mockTimeSource,
			).(*matchingEngineImpl)

			resolverMock.EXPECT().Lookup(gomock.Any(), gomock.Any()).Return(
				membership.NewDetailedHostInfo("", tc.lookUpResult, make(membership.PortMap)), tc.lookUpErr,
			).AnyTimes()
			resolverMock.EXPECT().WhoAmI().Return(
				membership.NewDetailedHostInfo("", tc.whoAmIResult, make(membership.PortMap)), tc.whoAmIErr,
			).AnyTimes()

			taskListKind := types.TaskListKindNormal

			_, err := matchingEngine.getTaskListManager(
				tasklist.NewTestTaskListID(t, "domain", "tasklist", persistence.TaskListTypeActivity),
				&taskListKind,
			)
			if tc.expectedError != nil {
				assert.ErrorAs(t, err, &tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMembershipSubscriptionShutdown(t *testing.T) {
	assert.NotPanics(t, func() {
		ctrl := gomock.NewController(t)
		m := membership.NewMockResolver(ctrl)

		m.EXPECT().Subscribe(service.Matching, "matching-engine", gomock.Any()).Times(1)

		e := matchingEngineImpl{
			membershipResolver: m,
			config: &config.Config{
				EnableTasklistOwnershipGuard: func(opts ...dynamicconfig.FilterOption) bool { return true },
			},
			shutdown: make(chan struct{}),
			logger:   loggerimpl.NewNopLogger(),
		}

		go func() {
			time.Sleep(time.Second)
			close(e.shutdown)
		}()
		e.subscribeToMembershipChanges()
	})
}

func TestMembershipSubscriptionPanicHandling(t *testing.T) {
	assert.NotPanics(t, func() {
		ctrl := gomock.NewController(t)

		r := resource.NewTest(t, ctrl, 0)
		r.MembershipResolver.EXPECT().Subscribe(service.Matching, "matching-engine", gomock.Any()).DoAndReturn(func(_, _, _ any) {
			panic("a panic has occurred")
		})

		e := matchingEngineImpl{
			membershipResolver: r.MembershipResolver,
			config: &config.Config{
				EnableTasklistOwnershipGuard: func(opts ...dynamicconfig.FilterOption) bool { return true },
			},
			logger:   loggerimpl.NewNopLogger(),
			shutdown: make(chan struct{}),
		}

		e.subscribeToMembershipChanges()
	})
}

func TestSubscriptionAndShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := membership.NewMockResolver(ctrl)

	shutdownWG := &sync.WaitGroup{}
	shutdownWG.Add(1)

	e := matchingEngineImpl{
		shutdownCompletion: shutdownWG,
		membershipResolver: m,
		config: &config.Config{
			EnableTasklistOwnershipGuard: func(opts ...dynamicconfig.FilterOption) bool { return true },
		},
		shutdown: make(chan struct{}),
		logger:   loggerimpl.NewNopLogger(),
	}

	// anytimes here because this is quite a racy test and the actual assertions for the unsubscription logic will be separated out
	m.EXPECT().WhoAmI().Return(membership.NewDetailedHostInfo("host2", "host2", nil), nil).AnyTimes()
	m.EXPECT().Subscribe(service.Matching, "matching-engine", gomock.Any()).Do(
		func(service string, name string, inc chan<- *membership.ChangedEvent) {
			m := membership.ChangedEvent{
				HostsAdded:   nil,
				HostsUpdated: nil,
				HostsRemoved: []string{"host123"},
			}
			inc <- &m
		})

	go func() {
		// then call stop so the test can finish
		time.Sleep(time.Second)
		e.Stop()
	}()

	e.subscribeToMembershipChanges()
}

func TestSubscriptionAndErrorReturned(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := membership.NewMockResolver(ctrl)

	shutdownWG := sync.WaitGroup{}
	shutdownWG.Add(1)

	e := matchingEngineImpl{
		shutdownCompletion: &shutdownWG,
		membershipResolver: m,
		config: &config.Config{
			EnableTasklistOwnershipGuard: func(opts ...dynamicconfig.FilterOption) bool { return true },
		},
		shutdown: make(chan struct{}),
		logger:   loggerimpl.NewNopLogger(),
	}

	// this should trigger the error case on a membership event
	m.EXPECT().WhoAmI().Return(membership.HostInfo{}, assert.AnError).AnyTimes()

	m.EXPECT().Subscribe(service.Matching, "matching-engine", gomock.Any()).Do(
		func(service string, name string, inc chan<- *membership.ChangedEvent) {
			m := membership.ChangedEvent{
				HostsAdded:   nil,
				HostsUpdated: nil,
				HostsRemoved: []string{"host123"},
			}
			inc <- &m
		})

	go func() {
		// then call stop so the test can finish
		time.Sleep(time.Second)
		e.Stop()
	}()

	e.subscribeToMembershipChanges()
}

func TestSubscribeToMembershipChangesQuitsIfSubscribeFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := membership.NewMockResolver(ctrl)

	logger, logs := testlogger.NewObserved(t)

	shutdownWG := sync.WaitGroup{}
	shutdownWG.Add(1)

	e := matchingEngineImpl{
		shutdownCompletion: &shutdownWG,
		membershipResolver: m,
		config: &config.Config{
			EnableTasklistOwnershipGuard: func(opts ...dynamicconfig.FilterOption) bool { return true },
		},
		shutdown: make(chan struct{}),
		logger:   logger,
	}

	// this should trigger the error case on a membership event
	m.EXPECT().WhoAmI().Return(membership.HostInfo{}, assert.AnError).AnyTimes()

	m.EXPECT().Subscribe(service.Matching, "matching-engine", gomock.Any()).
		Return(errors.New("matching-engine is already subscribed to updates"))

	go func() {
		// then call stop so the test can finish
		time.Sleep(time.Second)
		e.Stop()
	}()

	e.subscribeToMembershipChanges()
	// check we emitted error-message
	filteredLogs := logs.FilterMessage("Failed to subscribe to membership updates")
	assert.Equal(t, 1, filteredLogs.Len(), "error-message should be produced")

	assert.True(
		t,
		common.AwaitWaitGroup(&shutdownWG, 10*time.Second),
		"subscribeToMembershipChanges should immediately shut down because of critical error",
	)
}

func TestGetTasklistManagerShutdownScenario(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := membership.NewMockResolver(ctrl)

	self := membership.NewDetailedHostInfo("self", "self", nil)

	m.EXPECT().WhoAmI().Return(self, nil).AnyTimes()

	shutdownWG := sync.WaitGroup{}
	shutdownWG.Add(0)

	e := matchingEngineImpl{
		shutdownCompletion: &shutdownWG,
		membershipResolver: m,
		config: &config.Config{
			EnableTasklistOwnershipGuard: func(opts ...dynamicconfig.FilterOption) bool { return true },
		},
		shutdown: make(chan struct{}),
		logger:   loggerimpl.NewNopLogger(),
	}

	// set this engine to be shutting down so as to trigger the tasklistGetTasklistByID guard
	e.Stop()

	tl, _ := tasklist.NewIdentifier("domainid", "tl", 0)
	kind := types.TaskListKindNormal
	res, err := e.getTaskListManager(tl, &kind)
	assertErr := &cadence_errors.TaskListNotOwnedByHostError{}
	assert.ErrorAs(t, err, &assertErr)
	assert.Nil(t, res)
}
