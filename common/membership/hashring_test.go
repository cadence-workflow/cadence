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

package membership

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest/observer"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyz")

const maxTestDuration = 5 * time.Second

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func randomHostInfo(n int) []HostInfo {
	res := make([]HostInfo, 0, n)
	for i := 0; i < n; i++ {
		res = append(res, NewDetailedHostInfo(randSeq(5), randSeq(12), PortMap{randSeq(3): 666}))
	}
	return res
}

func TestDiffMemberMakesCorrectDiff(t *testing.T) {
	tests := []struct {
		name           string
		curr           []HostInfo
		new            []HostInfo
		expectedChange ChangedEvent
	}{
		{
			name:           "empty and one added",
			curr:           []HostInfo{},
			new:            []HostInfo{NewHostInfo("a")},
			expectedChange: ChangedEvent{HostsAdded: []string{"a"}},
		},
		{
			name:           "non-empty and added",
			curr:           []HostInfo{NewHostInfo("a")},
			new:            []HostInfo{NewHostInfo("a"), NewHostInfo("b")},
			expectedChange: ChangedEvent{HostsAdded: []string{"b"}},
		},
		{
			name:           "empty and nothing has changed",
			curr:           []HostInfo{},
			new:            []HostInfo{},
			expectedChange: ChangedEvent{},
		},
		{
			name:           "multiple hosts, but no change",
			curr:           []HostInfo{NewHostInfo("a"), NewHostInfo("b"), NewHostInfo("c")},
			new:            []HostInfo{NewHostInfo("c"), NewHostInfo("b"), NewHostInfo("a")},
			expectedChange: ChangedEvent{},
		},

		{
			name:           "multiple hosts, add/delete",
			curr:           []HostInfo{NewHostInfo("a"), NewHostInfo("b"), NewHostInfo("c")},
			new:            []HostInfo{NewHostInfo("b"), NewHostInfo("e"), NewHostInfo("f")},
			expectedChange: ChangedEvent{HostsRemoved: []string{"a", "c"}, HostsAdded: []string{"e", "f"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ring{}
			currMembers := r.makeMembersMap(tt.curr)
			r.members.keys = currMembers

			combinedChange := r.diffMembers(r.makeMembersMap(tt.new))
			assert.Equal(t, tt.expectedChange, combinedChange)
		})
	}
}

type hashringTestData struct {
	t                *testing.T
	mockPeerProvider *MockPeerProvider
	mockTimeSource   clock.MockedTimeSource
	hashRing         *ring
	observedLogs     *observer.ObservedLogs
}

func newHashringTestData(t *testing.T) *hashringTestData {
	var td hashringTestData

	ctrl := gomock.NewController(t)
	td.t = t
	td.mockPeerProvider = NewMockPeerProvider(ctrl)
	td.mockTimeSource = clock.NewMockedTimeSourceAt(time.Now())

	logger, observedLogs := testlogger.NewObserved(t)
	td.observedLogs = observedLogs

	td.hashRing = newHashring(
		"test-service",
		td.mockPeerProvider,
		td.mockTimeSource,
		logger,
		metrics.NoopScope(0),
	)

	return &td
}

// starts hashring' background work and verifies all the goroutines closed at the end
func (td *hashringTestData) startHashRing() {
	td.mockPeerProvider.EXPECT().Stop()

	td.t.Cleanup(func() {
		td.hashRing.Stop()
		goleak.VerifyNone(td.t)
	})

	td.hashRing.Start()
}

func (td *hashringTestData) bypassRefreshRatelimiter() {
	td.hashRing.members.refreshed = time.Now().AddDate(0, 0, -1)
}

func TestFailedLookupWillAskProvider(t *testing.T) {
	td := newHashringTestData(t)

	var wg sync.WaitGroup
	wg.Add(2)
	td.mockPeerProvider.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Times(1)
	td.mockPeerProvider.EXPECT().GetMembers("test-service").
		Do(func(string) {
			// we expect first call on hashring creation
			// the second call should be initiated by failed Lookup
			wg.Done()
		}).Times(2)

	td.startHashRing()
	_, err := td.hashRing.Lookup("a")
	assert.Error(t, err)

	require.True(t, common.AwaitWaitGroup(&wg, maxTestDuration), "Failed Lookup should lead to refresh")
}

func TestFailingToSubscribeIsFatal(t *testing.T) {
	defer goleak.VerifyNone(t)
	td := newHashringTestData(t)

	// we need to intercept logger calls, use mock
	mockLogger := &log.MockLogger{}
	td.hashRing.logger = mockLogger

	mockLogger.On("Fatal", mock.Anything, mock.Anything).Run(
		func(arguments mock.Arguments) {
			// we need to stop goroutine like log.Fatal() does with an entire program
			runtime.Goexit()
		},
	).Times(1)

	td.mockPeerProvider.EXPECT().
		Subscribe(gomock.Any(), gomock.Any()).
		Return(errors.New("can't subscribe"))

	// because we use runtime.Goexit() we need to call .Start in a separate goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		td.hashRing.Start()
	}()

	require.True(t, common.AwaitWaitGroup(&wg, maxTestDuration), "must be finished - failed to subscribe")
	require.True(t, mockLogger.AssertExpectations(t), "log.Fatal must be called")
}

func TestHandleUpdatesNeverBlocks(t *testing.T) {
	td := newHashringTestData(t)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			td.hashRing.handleUpdates(ChangedEvent{})
			wg.Done()
		}()
	}

	require.True(t, common.AwaitWaitGroup(&wg, maxTestDuration), "handleUpdates should never block")
}

func TestHandlerSchedulesUpdates(t *testing.T) {
	td := newHashringTestData(t)

	var wg sync.WaitGroup
	td.mockPeerProvider.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Times(1)
	td.mockPeerProvider.EXPECT().GetMembers("test-service").DoAndReturn(func(service string) ([]HostInfo, error) {
		wg.Done()
		fmt.Println("GetMembers called")
		return randomHostInfo(5), nil
	}).Times(2)
	td.mockPeerProvider.EXPECT().WhoAmI().AnyTimes()

	wg.Add(1) // we expect 1st GetMembers to be called during hashring start
	td.startHashRing()
	require.True(t, common.AwaitWaitGroup(&wg, maxTestDuration), "GetMembers must be called")

	wg.Add(1) // another call to GetMembers should happen because of handleUpdate
	td.bypassRefreshRatelimiter()
	td.hashRing.handleUpdates(ChangedEvent{})

	require.True(t, common.AwaitWaitGroup(&wg, maxTestDuration), "GetMembers must be called again")
}

func TestFailedRefreshLogsError(t *testing.T) {
	td := newHashringTestData(t)

	var wg sync.WaitGroup
	td.mockPeerProvider.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Times(1)
	td.mockPeerProvider.EXPECT().GetMembers("test-service").DoAndReturn(func(service string) ([]HostInfo, error) {
		wg.Done()
		return randomHostInfo(5), nil
	}).Times(1)
	td.mockPeerProvider.EXPECT().WhoAmI().AnyTimes()

	wg.Add(1) // we expect 1st GetMembers to be called during hashring start
	td.startHashRing()
	require.True(t, common.AwaitWaitGroup(&wg, maxTestDuration), "GetMembers must be called")

	td.mockPeerProvider.EXPECT().GetMembers("test-service").DoAndReturn(func(service string) ([]HostInfo, error) {
		wg.Done()
		return nil, errors.New("GetMembers failed")
	}).Times(1)

	wg.Add(1) // another call to GetMembers should happen because of handleUpdate
	td.bypassRefreshRatelimiter()
	td.hashRing.handleUpdates(ChangedEvent{})

	require.True(t, common.AwaitWaitGroup(&wg, maxTestDuration), "GetMembers must be called again (and fail)")
	td.hashRing.Stop()
	assert.Equal(t, 1, td.observedLogs.FilterMessageSnippet("failed to refresh ring").Len())
}

func TestRefreshUpdatesRingOnlyWhenRingHasChanged(t *testing.T) {
	td := newHashringTestData(t)

	td.mockPeerProvider.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Times(1)
	td.mockPeerProvider.EXPECT().GetMembers("test-service").Times(1).Return(randomHostInfo(3), nil)
	td.mockPeerProvider.EXPECT().WhoAmI().AnyTimes()

	// Start will also call .refresh()
	td.startHashRing()
	updatedAt := td.hashRing.members.refreshed

	assert.NoError(t, td.hashRing.refresh())
	assert.Equal(t, updatedAt, td.hashRing.members.refreshed)
}

func TestRefreshWillNotifySubscribers(t *testing.T) {
	td := newHashringTestData(t)

	var hostsToReturn []HostInfo
	td.mockPeerProvider.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Times(1)
	td.mockPeerProvider.EXPECT().GetMembers("test-service").DoAndReturn(func(service string) ([]HostInfo, error) {
		hostsToReturn = randomHostInfo(5)
		return hostsToReturn, nil
	}).Times(2)
	td.mockPeerProvider.EXPECT().WhoAmI().AnyTimes()

	td.startHashRing()

	var changeCh = make(chan *ChangedEvent, 2)
	// Check if multiple subscribers will get notified
	assert.NoError(t, td.hashRing.Subscribe("subscriber1", changeCh))
	assert.NoError(t, td.hashRing.Subscribe("subscriber2", changeCh))

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		changedEvent := <-changeCh
		changedEvent2 := <-changeCh
		assert.NotEmpty(t, changedEvent, "changed event should never be empty")
		assert.NotEmpty(t, changedEvent2, "changed event should never be empty")
	}()

	td.bypassRefreshRatelimiter()
	td.hashRing.signalSelf()

	// wait until both subscribers will get notification
	require.True(t, common.AwaitWaitGroup(&wg, maxTestDuration))

	// Test if internal members are updated
	assert.ElementsMatch(t, td.hashRing.Members(), hostsToReturn, "members should contain just-added nodes")
}

func TestSubscribersAreNotifiedPeriodically(t *testing.T) {
	td := newHashringTestData(t)

	var hostsToReturn []HostInfo

	td.mockPeerProvider.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Times(1)
	td.mockPeerProvider.EXPECT().GetMembers("test-service").DoAndReturn(func(service string) ([]HostInfo, error) {
		// we have to change members since subscribers are only notified on change
		hostsToReturn = randomHostInfo(5)
		return hostsToReturn, nil
	}).Times(2)
	td.mockPeerProvider.EXPECT().WhoAmI().AnyTimes()

	td.startHashRing()

	var changeCh = make(chan *ChangedEvent, 1)
	assert.NoError(t, td.hashRing.Subscribe("subscriber1", changeCh))

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		event := <-changeCh
		assert.NotEmpty(t, event, "changed event should never be empty")
	}()

	td.mockTimeSource.BlockUntil(1)                   // we should wait until ticker(defaultRefreshInterval) is created
	td.mockTimeSource.Advance(defaultRefreshInterval) // and only then to advance time

	require.True(t, common.AwaitWaitGroup(&wg, maxTestDuration)) // wait until subscriber will get notification

	// Test if internal members are updated
	assert.ElementsMatch(t, td.hashRing.Members(), hostsToReturn, "members should contain just-added nodes")
}

func TestSubscribeIgnoresDuplicates(t *testing.T) {
	var changeCh = make(chan *ChangedEvent)
	td := newHashringTestData(t)

	assert.NoError(t, td.hashRing.Subscribe("test-service", changeCh))
	assert.Error(t, td.hashRing.Subscribe("test-service", changeCh))
	assert.Equal(t, 1, len(td.hashRing.subscribers.keys))
}

func TestUnsubcribeIgnoresDeletionOnEmpty(t *testing.T) {
	td := newHashringTestData(t)

	assert.Equal(t, 0, len(td.hashRing.subscribers.keys))
	assert.NoError(t, td.hashRing.Unsubscribe("test-service"))
	assert.NoError(t, td.hashRing.Unsubscribe("test-service"))
	assert.NoError(t, td.hashRing.Unsubscribe("test-service"))
}

func TestUnsubcribeDeletes(t *testing.T) {
	td := newHashringTestData(t)
	var changeCh = make(chan *ChangedEvent)

	assert.Equal(t, 0, len(td.hashRing.subscribers.keys))
	assert.NoError(t, td.hashRing.Subscribe("testservice1", changeCh))
	assert.Equal(t, 1, len(td.hashRing.subscribers.keys))
	assert.NoError(t, td.hashRing.Unsubscribe("test-service"))
	assert.Equal(t, 1, len(td.hashRing.subscribers.keys))
	assert.NoError(t, td.hashRing.Unsubscribe("testservice1"))
	assert.Equal(t, 0, len(td.hashRing.subscribers.keys))
}

func TestMemberCountReturnsNumber(t *testing.T) {
	td := newHashringTestData(t)

	assert.Equal(t, 0, td.hashRing.MemberCount())

	ring := emptyHashring()
	for _, addr := range []string{"127", "128"} {
		host := NewHostInfo(addr)
		ring.AddMembers(host)
	}
	td.hashRing.value.Store(ring)
	assert.Equal(t, 2, td.hashRing.MemberCount())
}

func TestErrorIsPropagatedWhenProviderFails(t *testing.T) {
	td := newHashringTestData(t)

	td.mockPeerProvider.EXPECT().GetMembers(gomock.Any()).Return(nil, errors.New("provider failure"))

	assert.ErrorContains(t, td.hashRing.refresh(), "provider failure")
}

func TestStopWillStopProvider(t *testing.T) {
	td := newHashringTestData(t)

	td.mockPeerProvider.EXPECT().Stop().Times(1)

	td.hashRing.status = common.DaemonStatusStarted
	td.hashRing.Stop()
}

func TestLookupAndRefreshRaceCondition(t *testing.T) {
	td := newHashringTestData(t)
	var wg sync.WaitGroup

	td.mockPeerProvider.EXPECT().Subscribe(gomock.Any(), gomock.Any()).Times(1)
	td.mockPeerProvider.EXPECT().GetMembers("test-service").AnyTimes().DoAndReturn(func(service string) ([]HostInfo, error) {
		return randomHostInfo(5), nil
	})
	td.mockPeerProvider.EXPECT().WhoAmI().AnyTimes()

	td.startHashRing()
	wg.Add(2)
	go func() {
		for i := 0; i < 50; i++ {
			_, _ = td.hashRing.Lookup("a")
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < 50; i++ {
			td.bypassRefreshRatelimiter()
			assert.NoError(t, td.hashRing.refresh())
		}
		wg.Done()
	}()

	wg.Wait()
}

func TestEmitHashringView(t *testing.T) {
	tests := map[string]struct {
		hosts          []HostInfo
		lookuperr      error
		selfInfo       HostInfo
		selfErr        error
		expectedResult float64
	}{
		"example one - sorted set 1 - the output should be some random hashed value": {
			hosts: []HostInfo{
				{addr: "10.0.0.1:1234", ip: "10.0.0.1", identity: "host1", portMap: nil},
				{addr: "10.0.0.2:1234", ip: "10.0.0.2", identity: "host2", portMap: nil},
				{addr: "10.0.0.3:1234", ip: "10.0.0.3", identity: "host3", portMap: nil},
			},
			selfInfo:       HostInfo{identity: "host123"},
			expectedResult: 835.0, // the number here is meaningless
		},
		"example one - unsorted set 1 - the order of the hosts should not matter": {
			hosts: []HostInfo{
				{addr: "10.0.0.1:1234", ip: "10.0.0.1", identity: "host1", portMap: nil},
				{addr: "10.0.0.3:1234", ip: "10.0.0.3", identity: "host3", portMap: nil},
				{addr: "10.0.0.2:1234", ip: "10.0.0.2", identity: "host2", portMap: nil},
			},
			selfInfo:       HostInfo{identity: "host123"},
			expectedResult: 835.0, // the test here is that it's the same as test 1
		},
		"example 2 - empty set": {
			hosts:          []HostInfo{},
			selfInfo:       HostInfo{identity: "host123"},
			expectedResult: 242.0, // meaningless hash value
		},
		"example 3 - nil set": {
			hosts:          nil,
			selfInfo:       HostInfo{identity: "host123"},
			expectedResult: 242.0, // meaningless hash value
		},
	}

	for testName, testInput := range tests {
		t.Run(testName, func(t *testing.T) {
			td := newHashringTestData(t)

			td.mockPeerProvider.EXPECT().GetMembers("test-service").DoAndReturn(func(service string) ([]HostInfo, error) {
				return testInput.hosts, testInput.lookuperr
			})
			td.mockPeerProvider.EXPECT().WhoAmI().DoAndReturn(func() (HostInfo, error) {
				return testInput.selfInfo, testInput.selfErr
			}).AnyTimes()

			require.NoError(t, td.hashRing.refresh())
			assert.Equal(t, testInput.expectedResult, td.hashRing.emitHashIdentifier())
		})
	}
}
