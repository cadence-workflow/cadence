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

package poller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/types"
)

var NoopFunc = func() {}

func TestManager_HistoryCallback(t *testing.T) {
	mockTime := clock.NewMockedTimeSource()
	counter := 0
	m := NewPollerManager(func() {
		counter++
	}, mockTime)
	m.StartPoll("a", NoopFunc, &Info{Identity: "a"})

	assert.Equal(t, 1, counter)

	m.EndPoll("a")
	assert.Equal(t, 2, counter)

	// Require identity
	counter = 0
	m.StartPoll("b", NoopFunc, &Info{})

	assert.Equal(t, 0, counter)
}

func TestManager_CancelPoll(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		mockTime := clock.NewMockedTimeSource()
		m := NewPollerManager(NoopFunc, mockTime)
		counter := 0
		m.StartPoll("a", func() {
			counter++
		}, &Info{})

		res := m.CancelPoll("a")

		assert.Equal(t, true, res)
		assert.Equal(t, 1, counter)
	})
	t.Run("repeated", func(t *testing.T) {
		mockTime := clock.NewMockedTimeSource()
		m := NewPollerManager(NoopFunc, mockTime)
		counter := 0
		m.StartPoll("a", func() {
			counter++
		}, &Info{})

		// Cancel doesn't remove it from the outstanding
		res := m.CancelPoll("a")
		res2 := m.CancelPoll("a")

		assert.Equal(t, true, res)
		assert.Equal(t, true, res2)
		assert.Equal(t, 2, counter)
	})
	t.Run("unknown", func(t *testing.T) {
		mockTime := clock.NewMockedTimeSource()
		m := NewPollerManager(NoopFunc, mockTime)
		counter := 0
		m.StartPoll("b", func() {
			counter++
		}, &Info{})

		res := m.CancelPoll("a")

		assert.Equal(t, false, res)
		assert.Equal(t, 0, counter)
	})
}

func TestManager_HasPollerFromIsolationGroupAfter(t *testing.T) {
	startTime := time.Date(2024, time.October, 28, 0, 0, 0, 0, time.UTC)
	group := "the group"
	cases := []struct {
		name   string
		fn     func(mockTime clock.MockedTimeSource, m Manager)
		after  time.Time
		result bool
	}{
		{
			name: "outstanding poller",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("a", NoopFunc, &Info{IsolationGroup: group})
			},
			after:  startTime,
			result: true,
		},
		{
			name: "recent poller",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("a", NoopFunc, &Info{IsolationGroup: group})
				mockTime.Advance(time.Second)
				m.EndPoll("a")
			},
			after:  startTime,
			result: true,
		},
		{
			name: "boundary condition poller",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("a", NoopFunc, &Info{IsolationGroup: group})
				mockTime.Advance(time.Nanosecond)
				m.EndPoll("a")
			},
			// Needs to after this time, not at this time
			after:  startTime.Add(time.Nanosecond),
			result: false,
		},
		{
			name: "expired poller",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("a", NoopFunc, &Info{IsolationGroup: group})
				m.EndPoll("a")
			},
			after:  startTime.Add(time.Minute),
			result: false,
		},
		{
			name:   "never polled",
			fn:     func(mockTime clock.MockedTimeSource, m Manager) {},
			after:  startTime,
			result: false,
		},
		{
			name: "unrelated poller",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("a", NoopFunc, &Info{IsolationGroup: "other"})
				mockTime.Advance(time.Second)
				m.EndPoll("a")
			},
			after:  startTime,
			result: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockTime := clock.NewMockedTimeSourceAt(startTime)
			m := NewPollerManager(NoopFunc, mockTime)
			tc.fn(mockTime, m)
			assert.Equal(t, tc.result, m.HasPollerFromIsolationGroupAfter(group, tc.after))
		})
	}
}

func TestManager_HasPollerAfter(t *testing.T) {
	startTime := time.Date(2024, time.October, 28, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		fn     func(mockTime clock.MockedTimeSource, m Manager)
		after  time.Time
		result bool
	}{
		{
			name: "outstanding poller",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("a", NoopFunc, &Info{})
			},
			after:  startTime,
			result: true,
		},
		{
			name: "recent poller",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("a", NoopFunc, &Info{})
				mockTime.Advance(time.Second)
				m.EndPoll("a")
			},
			after:  startTime,
			result: true,
		},
		{
			name: "boundary condition poller",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("a", NoopFunc, &Info{})
				mockTime.Advance(time.Nanosecond)
				m.EndPoll("a")
			},
			// Needs to after this time, not at this time
			after:  startTime.Add(time.Nanosecond),
			result: false,
		},
		{
			name: "expired poller",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("a", NoopFunc, &Info{})
				m.EndPoll("a")
			},
			after:  startTime.Add(time.Minute),
			result: false,
		},
		{
			name:   "never polled",
			fn:     func(mockTime clock.MockedTimeSource, m Manager) {},
			after:  startTime,
			result: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockTime := clock.NewMockedTimeSourceAt(startTime)
			m := NewPollerManager(NoopFunc, mockTime)
			tc.fn(mockTime, m)
			assert.Equal(t, tc.result, m.HasPollerAfter(tc.after))
		})
	}
}

func TestManager_GetCount(t *testing.T) {
	mockTime := clock.NewMockedTimeSource()
	m := NewPollerManager(NoopFunc, mockTime)
	m.StartPoll("a", NoopFunc, &Info{Identity: "aIdent"})
	m.EndPoll("a")
	m.StartPoll("b", NoopFunc, &Info{Identity: "bIdent"})
	mockTime.Advance(4 * time.Minute) // t = 4m
	m.EndPoll("b")
	mockTime.Advance(2 * time.Minute) // t = 6m
	m.StartPoll("c", NoopFunc, &Info{Identity: "cIdent"})
	m.EndPoll("c")
	m.StartPoll("d", NoopFunc, &Info{Identity: "dIdent"})

	// Since the cache doesn't actively evict, we still see a in the results
	assert.Equal(t, 4, m.GetCount())
}

func TestManager_ListInfo(t *testing.T) {
	startTime := time.Date(2024, time.October, 28, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		fn     func(mockTime clock.MockedTimeSource, m Manager)
		result []*types.PollerInfo
	}{
		{
			name: "happy path",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("a", NoopFunc, &Info{Identity: "aIdent"})
				m.EndPoll("a")
				m.StartPoll("b", NoopFunc, &Info{Identity: "bIdent"})
				mockTime.Advance(time.Minute) // t = 1m
				m.EndPoll("b")
				mockTime.Advance(time.Minute) // t = 2m
				m.StartPoll("c", NoopFunc, &Info{Identity: "cIdent", RatePerSecond: 1.0})
				m.EndPoll("c")
				m.StartPoll("d", NoopFunc, &Info{Identity: "dIdent"})
			},
			result: []*types.PollerInfo{
				{
					LastAccessTime: common.Int64Ptr(startTime.Add(2 * time.Minute).UnixNano()),
					Identity:       "dIdent",
					RatePerSecond:  0,
				},
				{
					LastAccessTime: common.Int64Ptr(startTime.Add(2 * time.Minute).UnixNano()),
					Identity:       "cIdent",
					RatePerSecond:  1.0,
				},
				{
					LastAccessTime: common.Int64Ptr(startTime.Add(time.Minute).UnixNano()),
					Identity:       "bIdent",
					RatePerSecond:  0,
				},
				{
					LastAccessTime: common.Int64Ptr(startTime.UnixNano()),
					Identity:       "aIdent",
					RatePerSecond:  0,
				},
			},
		},
		{
			name: "exclude pollers with no identity",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("a", NoopFunc, &Info{Identity: ""})
				m.EndPoll("a")
			},
			result: []*types.PollerInfo{},
		},
		{
			name: "include pollers with no pollerID",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("", NoopFunc, &Info{Identity: "aIdent"})
			},
			result: []*types.PollerInfo{
				{
					LastAccessTime: common.Int64Ptr(startTime.UnixNano()),
					Identity:       "aIdent",
					RatePerSecond:  0,
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockTime := clock.NewMockedTimeSourceAt(startTime)
			m := NewPollerManager(NoopFunc, mockTime)
			tc.fn(mockTime, m)
			assert.Equal(t, tc.result, m.ListInfo())
		})
	}
}

func TestManager_GetCountByIsolationGroup(t *testing.T) {
	startTime := time.Date(2024, time.October, 28, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		fn     func(mockTime clock.MockedTimeSource, m Manager)
		after  time.Time
		result map[string]int
	}{
		{
			name: "happy path",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("a", NoopFunc, &Info{Identity: "aIdent", IsolationGroup: "groupA"})
				m.EndPoll("a")
				m.StartPoll("b", NoopFunc, &Info{Identity: "bIdent", IsolationGroup: "groupA"})
				mockTime.Advance(time.Minute) // t = 1m
				m.EndPoll("b")
				mockTime.Advance(time.Minute) // t = 2m
				m.StartPoll("c", NoopFunc, &Info{Identity: "cIdent", RatePerSecond: 1.0, IsolationGroup: "groupB"})
				m.EndPoll("c")
				m.StartPoll("d", NoopFunc, &Info{Identity: "dIdent"})
			},
			result: map[string]int{
				"groupA": 2,
				"groupB": 1,
			},
		},
		{
			name: "some expired",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("a", NoopFunc, &Info{Identity: "aIdent", IsolationGroup: "groupA"})
				m.EndPoll("a")
				m.StartPoll("b", NoopFunc, &Info{Identity: "bIdent", IsolationGroup: "groupA"})
				mockTime.Advance(time.Minute) // t = 1m
				m.EndPoll("b")
				mockTime.Advance(time.Minute) // t = 2m
				m.StartPoll("c", NoopFunc, &Info{Identity: "cIdent", RatePerSecond: 1.0, IsolationGroup: "groupB"})
				m.EndPoll("c")
				m.StartPoll("d", NoopFunc, &Info{Identity: "dIdent"})
			},
			after: startTime.Add(time.Minute - time.Nanosecond),
			result: map[string]int{
				"groupA": 1,
				"groupB": 1,
			},
		},
		{
			name: "exclude pollers with no identity",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("a", NoopFunc, &Info{Identity: ""})
				m.EndPoll("a")
			},
			after:  startTime.Add(-time.Nanosecond),
			result: map[string]int{},
		},
		{
			name: "include pollers with no pollerID",
			fn: func(mockTime clock.MockedTimeSource, m Manager) {
				m.StartPoll("", NoopFunc, &Info{Identity: "aIdent", IsolationGroup: "groupA"})
			},
			after: startTime.Add(-time.Nanosecond),
			result: map[string]int{
				"groupA": 1,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockTime := clock.NewMockedTimeSourceAt(startTime)
			m := NewPollerManager(NoopFunc, mockTime)
			tc.fn(mockTime, m)
			assert.Equal(t, tc.result, m.GetCountByIsolationGroup(tc.after))
		})
	}
}
