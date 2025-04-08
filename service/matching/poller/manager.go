// Copyright (c) 2017 Uber Technologies, Inc.
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

package poller

import (
	"context"
	"sync"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/types"
)

const (
	pollerHistoryInitSize    = 0
	pollerHistoryInitMaxSize = 5000
	pollerHistoryTTL         = 5 * time.Minute
)

type (
	Info struct {
		Identity       string
		RatePerSecond  float64
		IsolationGroup string
	}

	Manager interface {
		StartPoll(pollerID string, cancelFunc context.CancelFunc, info *Info)
		EndPoll(pollerID string)
		CancelPoll(pollerID string) bool
		HasPollerFromIsolationGroupAfter(isolationGroup string, after time.Time) bool
		HasPollerAfter(after time.Time) bool
		GetCount() int
		GetCountByIsolationGroup(after time.Time) map[string]int
		ListInfo() []*types.PollerInfo
	}

	historicalPoller struct {
		info        *Info
		outstanding bool
	}

	outstandingPoller struct {
		info   *Info
		cancel context.CancelFunc
	}

	manager struct {
		// identity -> historicalPoller
		historyCache cache.Cache
		timeSource   clock.TimeSource

		lock                     sync.RWMutex
		mostRecentPollEnd        time.Time
		mostRecentPollEndByGroup map[string]time.Time
		outstandingCountByGroup  map[string]int
		// pollerID -> outstandingPoller
		outstanding map[string]outstandingPoller

		// OnHistoryUpdatedFunc is a function called when the historyCache was updated
		onHistoryUpdatedFunc HistoryUpdatedFunc
	}

	// HistoryUpdatedFunc is a type for notifying applications when the poller historyCache was updated
	HistoryUpdatedFunc func()
)

func NewPollerManager(historyUpdatedFunc HistoryUpdatedFunc, timeSource clock.TimeSource) Manager {
	opts := &cache.Options{
		InitialCapacity: pollerHistoryInitSize,
		TTL:             pollerHistoryTTL,
		Pin:             false,
		MaxCount:        pollerHistoryInitMaxSize,
		TimeSource:      timeSource,
	}

	return &manager{
		historyCache:             cache.New(opts, nil),
		timeSource:               timeSource,
		onHistoryUpdatedFunc:     historyUpdatedFunc,
		mostRecentPollEndByGroup: make(map[string]time.Time),
		outstandingCountByGroup:  make(map[string]int),
		outstanding:              make(map[string]outstandingPoller),
	}
}

func (m *manager) StartPoll(pollerID string, cancelFunc context.CancelFunc, info *Info) {
	if info.Identity != "" {
		m.historyCache.Put(info.Identity, &historicalPoller{
			info: info,
			// If there's no PollerID then we'll never have a subsequent EndPoll. Treat it like it isn't outstanding
			// so that we don't keep returning it forever
			// It doesn't seem like there's a possible code path where this happens
			outstanding: pollerID != "",
		})
		if m.onHistoryUpdatedFunc != nil {
			m.onHistoryUpdatedFunc()
		}
	}
	if pollerID != "" {
		m.lock.Lock()
		defer m.lock.Unlock()
		m.outstanding[pollerID] = outstandingPoller{
			info:   info,
			cancel: cancelFunc,
		}
		if info.IsolationGroup != "" {
			m.outstandingCountByGroup[info.IsolationGroup]++
		}
	}
}

func (m *manager) EndPoll(pollerID string) {
	poller, ok := m.tryRemovePoller(pollerID)
	if ok && poller.info.Identity != "" {
		// Refresh the cache to update the timestamp and clear outstanding value
		m.historyCache.Put(poller.info.Identity, &historicalPoller{
			info:        poller.info,
			outstanding: false,
		})
		if m.onHistoryUpdatedFunc != nil {
			m.onHistoryUpdatedFunc()
		}
	}
}

func (m *manager) tryRemovePoller(pollerID string) (outstandingPoller, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	now := m.timeSource.Now()
	poller, ok := m.outstanding[pollerID]
	if ok {
		delete(m.outstanding, pollerID)
		if poller.info.IsolationGroup != "" {
			m.mostRecentPollEndByGroup[poller.info.IsolationGroup] = now
			m.outstandingCountByGroup[poller.info.IsolationGroup]--
		}
	}
	// reset the mostRecentPollEnd even if we didn't find the poller. They might not have specified a PollerID.
	// It doesn't seem possible outside of tests, but there's no harm in being safe
	m.mostRecentPollEnd = now
	return poller, ok
}

func (m *manager) CancelPoll(pollerID string) bool {
	poller, ok := m.tryGetPoller(pollerID)
	if ok {
		poller.cancel()
	}
	return ok
}

func (m *manager) tryGetPoller(pollerID string) (outstandingPoller, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	poller, ok := m.outstanding[pollerID]
	return poller, ok
}

func (m *manager) HasPollerFromIsolationGroupAfter(isolationGroup string, earliest time.Time) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.outstandingCountByGroup[isolationGroup] > 0 || m.mostRecentPollEndByGroup[isolationGroup].After(earliest)
}

func (m *manager) HasPollerAfter(earliestAccessTime time.Time) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return len(m.outstanding) > 0 || m.mostRecentPollEnd.After(earliestAccessTime)
}

func (m *manager) GetCount() int {
	return m.historyCache.Size()
}

func (m *manager) GetCountByIsolationGroup(after time.Time) map[string]int {
	groupSet := make(map[string]int)

	m.forEachPoller(after, func(identity string, info *Info, lastAccessTime time.Time) {
		if info.IsolationGroup != "" {
			groupSet[info.IsolationGroup]++
		}
	})

	return groupSet
}

func (m *manager) ListInfo() []*types.PollerInfo {
	var result []*types.PollerInfo
	// optimistic size get, it can change before Iterator call.
	size := m.historyCache.Size()
	result = make([]*types.PollerInfo, 0, size)

	m.forEachPoller(time.Time{}, func(identity string, info *Info, lastAccessTime time.Time) {
		result = append(result, &types.PollerInfo{
			Identity:       identity,
			LastAccessTime: common.Int64Ptr(lastAccessTime.UnixNano()),
			RatePerSecond:  info.RatePerSecond,
		})
	})

	return result
}

func (m *manager) forEachPoller(after time.Time, callback func(string, *Info, time.Time)) {
	ite := m.historyCache.Iterator()
	defer ite.Close()

	for ite.HasNext() {
		entry := ite.Next()
		key := entry.Key().(string)
		value := entry.Value().(*historicalPoller)
		lastAccessTime := entry.CreateTime()
		if after.IsZero() || value.outstanding || after.Before(lastAccessTime) {
			callback(key, value.info, lastAccessTime)
		}
	}
}
