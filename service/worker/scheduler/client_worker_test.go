// Copyright (c) 2026 Uber Technologies, Inc.
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

package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
)

func TestRefreshWorkers(t *testing.T) {
	selfHost := membership.NewDetailedHostInfo("10.0.0.1:7933", "self", nil)
	otherHost := membership.NewDetailedHostInfo("10.0.0.2:7933", "other", nil)
	thirdHost := membership.NewDetailedHostInfo("10.0.0.3:7933", "third", nil)

	makeDomainEntry := func(name string) *cache.DomainCacheEntry {
		return cache.NewDomainCacheEntryForTest(
			&persistence.DomainInfo{Name: name},
			nil, false, nil, 0, nil, 0, 0, 0,
		)
	}

	tests := []struct {
		name               string
		domains            map[string]*cache.DomainCacheEntry
		lookupNResults     map[string][]membership.HostInfo
		lookupNErrors      map[string]error
		existingWorkers    []string
		wantActiveWorkers  []string
		wantStoppedWorkers []string
		wantStartedWorkers []string
	}{
		{
			name: "starts workers for owned domains",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-a": makeDomainEntry("domain-a"),
				"domain-b": makeDomainEntry("domain-b"),
			},
			lookupNResults: map[string][]membership.HostInfo{
				"domain-a": {selfHost, otherHost},
				"domain-b": {selfHost, otherHost},
			},
			wantActiveWorkers:  []string{"domain-a", "domain-b"},
			wantStartedWorkers: []string{"domain-a", "domain-b"},
		},
		{
			name: "skips domains where this host is not among the owners",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-a": makeDomainEntry("domain-a"),
				"domain-b": makeDomainEntry("domain-b"),
			},
			lookupNResults: map[string][]membership.HostInfo{
				"domain-a": {selfHost, otherHost},
				"domain-b": {otherHost, thirdHost},
			},
			wantActiveWorkers:  []string{"domain-a"},
			wantStartedWorkers: []string{"domain-a"},
		},
		{
			name: "starts worker when self is secondary redundancy owner",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-a": makeDomainEntry("domain-a"),
			},
			lookupNResults: map[string][]membership.HostInfo{
				"domain-a": {otherHost, selfHost},
			},
			wantActiveWorkers:  []string{"domain-a"},
			wantStartedWorkers: []string{"domain-a"},
		},
		{
			name: "stops workers for domains no longer owned",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-a": makeDomainEntry("domain-a"),
			},
			lookupNResults: map[string][]membership.HostInfo{
				"domain-a": {otherHost, thirdHost},
			},
			existingWorkers:    []string{"domain-a"},
			wantActiveWorkers:  []string{},
			wantStoppedWorkers: []string{"domain-a"},
		},
		{
			name: "stops workers for domains that disappeared from cache",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-b": makeDomainEntry("domain-b"),
			},
			lookupNResults: map[string][]membership.HostInfo{
				"domain-b": {selfHost, otherHost},
			},
			existingWorkers:    []string{"domain-a"},
			wantActiveWorkers:  []string{"domain-b"},
			wantStoppedWorkers: []string{"domain-a"},
			wantStartedWorkers: []string{"domain-b"},
		},
		{
			name:              "no domains means no workers",
			domains:           map[string]*cache.DomainCacheEntry{},
			wantActiveWorkers: []string{},
		},
		{
			name: "lookup error skips domain without stopping existing worker",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-a": makeDomainEntry("domain-a"),
			},
			lookupNErrors: map[string]error{
				"domain-a": fmt.Errorf("ring not ready"),
			},
			existingWorkers:   []string{"domain-a"},
			wantActiveWorkers: []string{"domain-a"},
		},
		{
			name: "does not restart already running worker",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-a": makeDomainEntry("domain-a"),
			},
			lookupNResults: map[string][]membership.HostInfo{
				"domain-a": {selfHost, otherHost},
			},
			existingWorkers:   []string{"domain-a"},
			wantActiveWorkers: []string{"domain-a"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockDomainCache := cache.NewMockDomainCache(ctrl)
			mockDomainCache.EXPECT().GetAllDomain().Return(tc.domains)

			mockResolver := membership.NewMockResolver(ctrl)
			for domainName, hosts := range tc.lookupNResults {
				mockResolver.EXPECT().LookupN(service.Worker, domainName, workerRedundancyFactor).Return(hosts, nil)
			}
			for domainName, err := range tc.lookupNErrors {
				mockResolver.EXPECT().LookupN(service.Worker, domainName, workerRedundancyFactor).Return(nil, err)
			}

			stopped := make(map[string]bool)
			started := make(map[string]bool)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			wm := &WorkerManager{
				enabledFn:          dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
				metricsClient:      metrics.NewNoopMetricsClient(),
				logger:             testlogger.New(t),
				domainCache:        mockDomainCache,
				membershipResolver: mockResolver,
				hostInfo:           selfHost,
				activeWorkers:      make(map[string]workerHandle),
				ctx:                ctx,
				createWorker: func(domainName string) (workerHandle, error) {
					started[domainName] = true
					return &fakeWorker{
						stopFn: func() { stopped[domainName] = true },
					}, nil
				},
			}

			for _, d := range tc.existingWorkers {
				domain := d
				wm.activeWorkers[d] = &fakeWorker{
					stopFn: func() { stopped[domain] = true },
				}
			}

			wm.refreshWorkers()

			assert.Equal(t, len(tc.wantActiveWorkers), len(wm.activeWorkers),
				"active worker count mismatch")
			for _, d := range tc.wantActiveWorkers {
				_, exists := wm.activeWorkers[d]
				assert.True(t, exists, "expected active worker for domain %s", d)
			}

			for _, d := range tc.wantStoppedWorkers {
				assert.True(t, stopped[d], "expected worker for domain %s to be stopped", d)
			}

			for _, d := range tc.wantStartedWorkers {
				assert.True(t, started[d], "expected worker for domain %s to be started", d)
			}
		})
	}
}

func TestRefreshWorkers_StopsWorkerWhenDomainDisabled(t *testing.T) {
	selfHost := membership.NewDetailedHostInfo("10.0.0.1:7933", "self", nil)
	otherHost := membership.NewDetailedHostInfo("10.0.0.2:7933", "other", nil)
	ctrl := gomock.NewController(t)

	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockDomainCache.EXPECT().GetAllDomain().Return(map[string]*cache.DomainCacheEntry{
		"domain-a": cache.NewDomainCacheEntryForTest(
			&persistence.DomainInfo{Name: "domain-a"},
			nil, false, nil, 0, nil, 0, 0, 0,
		),
	})

	mockResolver := membership.NewMockResolver(ctrl)
	mockResolver.EXPECT().LookupN(service.Worker, "domain-a", workerRedundancyFactor).Return(
		[]membership.HostInfo{selfHost, otherHost}, nil,
	)

	stopped := make(map[string]bool)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wm := &WorkerManager{
		enabledFn:          func(domain string) bool { return false },
		metricsClient:      metrics.NewNoopMetricsClient(),
		logger:             testlogger.New(t),
		domainCache:        mockDomainCache,
		membershipResolver: mockResolver,
		hostInfo:           selfHost,
		activeWorkers:      make(map[string]workerHandle),
		ctx:                ctx,
		createWorker: func(domainName string) (workerHandle, error) {
			t.Fatal("should not start a worker for a disabled domain")
			return nil, nil
		},
	}

	wm.activeWorkers["domain-a"] = &fakeWorker{
		stopFn: func() { stopped["domain-a"] = true },
	}

	wm.refreshWorkers()

	assert.Empty(t, wm.activeWorkers, "worker should be removed for disabled domain")
	assert.True(t, stopped["domain-a"], "worker for disabled domain should have been stopped")
}

func TestRefreshWorkersHandlesCreateWorkerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	selfHost := membership.NewDetailedHostInfo("10.0.0.1:7933", "self", nil)

	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockDomainCache.EXPECT().GetAllDomain().Return(map[string]*cache.DomainCacheEntry{
		"domain-a": cache.NewDomainCacheEntryForTest(
			&persistence.DomainInfo{Name: "domain-a"},
			nil, false, nil, 0, nil, 0, 0, 0,
		),
	})

	mockResolver := membership.NewMockResolver(ctrl)
	mockResolver.EXPECT().LookupN(service.Worker, "domain-a", workerRedundancyFactor).Return([]membership.HostInfo{selfHost}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wm := &WorkerManager{
		enabledFn:          dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
		metricsClient:      metrics.NewNoopMetricsClient(),
		logger:             testlogger.New(t),
		domainCache:        mockDomainCache,
		membershipResolver: mockResolver,
		hostInfo:           selfHost,
		activeWorkers:      make(map[string]workerHandle),
		ctx:                ctx,
		createWorker: func(domainName string) (workerHandle, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	wm.refreshWorkers()

	assert.Empty(t, wm.activeWorkers, "worker should not be added on creation error")
}

func TestStopAllWorkers(t *testing.T) {
	wm := &WorkerManager{
		logger:        testlogger.New(t),
		activeWorkers: make(map[string]workerHandle),
	}

	stoppedDomains := make(map[string]bool)
	for _, d := range []string{"domain-a", "domain-b", "domain-c"} {
		domain := d
		wm.activeWorkers[d] = &fakeWorker{
			stopFn: func() { stoppedDomains[domain] = true },
		}
	}

	wm.stopAllWorkers()

	require.Empty(t, wm.activeWorkers)
	assert.True(t, stoppedDomains["domain-a"])
	assert.True(t, stoppedDomains["domain-b"])
	assert.True(t, stoppedDomains["domain-c"])
}

// TestMembershipChangeTriggersRefresh verifies that a membership change event
// causes an immediate call to refreshWorkers without waiting for the next tick.
func TestMembershipChangeTriggersRefresh(t *testing.T) {
	ctrl := gomock.NewController(t)
	selfHost := membership.NewDetailedHostInfo("10.0.0.1:7933", "self", nil)
	otherHost := membership.NewDetailedHostInfo("10.0.0.2:7933", "other", nil)

	domainEntry := cache.NewDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: "domain-a"},
		nil, false, nil, 0, nil, 0, 0, 0,
	)

	// refreshed is closed after GetAllDomain is called a second time, which
	// proves that the event-driven path invoked refreshWorkers().
	refreshed := make(chan struct{})
	getCount := 0

	mockDomainCache := cache.NewMockDomainCache(ctrl)
	mockDomainCache.EXPECT().GetAllDomain().DoAndReturn(func() map[string]*cache.DomainCacheEntry {
		getCount++
		if getCount == 2 {
			close(refreshed)
		}
		return map[string]*cache.DomainCacheEntry{"domain-a": domainEntry}
	}).AnyTimes()

	mockResolver := membership.NewMockResolver(ctrl)
	mockResolver.EXPECT().Subscribe(service.Worker, membershipSubscriberName, gomock.Any()).Return(nil)
	mockResolver.EXPECT().Unsubscribe(service.Worker, membershipSubscriberName).Return(nil)
	// First refresh: this host is not an owner, so no worker is started.
	// Second refresh (event-triggered): this host becomes an owner.
	mockResolver.EXPECT().LookupN(service.Worker, "domain-a", workerRedundancyFactor).Return(
		[]membership.HostInfo{otherHost}, nil,
	).Return(
		[]membership.HostInfo{selfHost}, nil,
	).AnyTimes()

	wm := NewWorkerManager(&BootstrapParams{
		Logger:             testlogger.New(t),
		MetricsClient:      metrics.NewNoopMetricsClient(),
		DomainCache:        mockDomainCache,
		MembershipResolver: mockResolver,
		HostInfo:           selfHost,
	}, dynamicproperties.GetBoolPropertyFnFilteredByDomain(true))

	wm.createWorker = func(domainName string) (workerHandle, error) {
		return &fakeWorker{}, nil
	}

	wm.Start()

	// Simulate a membership ring change (e.g. a new host joined).
	wm.membershipChangeCh <- &membership.ChangedEvent{HostsAdded: []string{"10.0.0.3:7933"}}

	// Wait for the event-driven refresh to complete.
	select {
	case <-refreshed:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for membership change to trigger refresh")
	}

	wm.Stop()
}

func TestContainsHost(t *testing.T) {
	h1 := membership.NewDetailedHostInfo("10.0.0.1:7933", "h1", nil)
	h2 := membership.NewDetailedHostInfo("10.0.0.2:7933", "h2", nil)
	h3 := membership.NewDetailedHostInfo("10.0.0.3:7933", "h3", nil)

	assert.True(t, containsHost([]membership.HostInfo{h1, h2}, h1))
	assert.True(t, containsHost([]membership.HostInfo{h1, h2}, h2))
	assert.False(t, containsHost([]membership.HostInfo{h1, h2}, h3))
	assert.False(t, containsHost(nil, h1))
}

func TestRefreshWorkersMetrics(t *testing.T) {
	selfHost := membership.NewDetailedHostInfo("10.0.0.1:7933", "self", nil)
	otherHost := membership.NewDetailedHostInfo("10.0.0.2:7933", "other", nil)

	makeDomainEntry := func(name string) *cache.DomainCacheEntry {
		return cache.NewDomainCacheEntryForTest(
			&persistence.DomainInfo{Name: name},
			nil, false, nil, 0, nil, 0, 0, 0,
		)
	}

	tests := []struct {
		name            string
		domains         map[string]*cache.DomainCacheEntry
		lookupNResults  map[string][]membership.HostInfo
		lookupNErrors   map[string]error
		existingWorkers []string
		workerStartErr  error
		assertMetrics   func(t *testing.T, snap tally.Snapshot)
	}{
		{
			name: "started counter and active gauge reflect new workers",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-a": makeDomainEntry("domain-a"),
				"domain-b": makeDomainEntry("domain-b"),
			},
			lookupNResults: map[string][]membership.HostInfo{
				"domain-a": {selfHost, otherHost},
				"domain-b": {selfHost, otherHost},
			},
			assertMetrics: func(t *testing.T, snap tally.Snapshot) {
				assertCounter(t, snap, "scheduler_worker_started_count", nil, 2)
				assertGauge(t, snap, "scheduler_worker_active_gauge", nil, 2)
				assertHistogramRecorded(t, snap, "scheduler_worker_refresh_latency_ns")
			},
		},
		{
			name: "stopped counter incremented for domain no longer owned",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-a": makeDomainEntry("domain-a"),
			},
			lookupNResults: map[string][]membership.HostInfo{
				"domain-a": {otherHost},
			},
			existingWorkers: []string{"domain-a"},
			assertMetrics: func(t *testing.T, snap tally.Snapshot) {
				assertCounter(t, snap, "scheduler_worker_stopped_count", nil, 1)
			},
		},
		{
			name: "lookup failure increments lookup failures counter",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-a": makeDomainEntry("domain-a"),
			},
			lookupNErrors: map[string]error{
				"domain-a": fmt.Errorf("ring not ready"),
			},
			assertMetrics: func(t *testing.T, snap tally.Snapshot) {
				assertCounter(t, snap, "scheduler_worker_lookup_failures_count", nil, 1)
			},
		},
		{
			name: "worker start error increments per-domain error counter",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-a": makeDomainEntry("domain-a"),
			},
			lookupNResults: map[string][]membership.HostInfo{
				"domain-a": {selfHost, otherHost},
			},
			workerStartErr: fmt.Errorf("connection refused"),
			assertMetrics: func(t *testing.T, snap tally.Snapshot) {
				assertCounter(t, snap, "scheduler_worker_start_errors_count_per_domain", map[string]string{"domain": "domain-a"}, 1)
			},
		},
		{
			name: "domain coverage counter incremented for each active worker",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-a": makeDomainEntry("domain-a"),
				"domain-b": makeDomainEntry("domain-b"),
			},
			lookupNResults: map[string][]membership.HostInfo{
				"domain-a": {selfHost, otherHost},
				"domain-b": {otherHost},
			},
			existingWorkers: []string{"domain-a"},
			assertMetrics: func(t *testing.T, snap tally.Snapshot) {
				assertCounter(t, snap, "scheduler_worker_domain_coverage_count", map[string]string{"domain": "domain-a"}, 1)
				// domain-b is not owned by this host, so no coverage counter
				for _, c := range snap.Counters() {
					if c.Name() == "scheduler_worker_domain_coverage_count" && c.Tags()["domain"] == "domain-b" {
						t.Errorf("unexpected domain coverage counter for domain-b")
					}
				}
			},
		},
		{
			name: "domain coverage counter skipped for lookup-failed domains",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-a": makeDomainEntry("domain-a"),
			},
			lookupNErrors: map[string]error{
				"domain-a": fmt.Errorf("ring not ready"),
			},
			existingWorkers: []string{"domain-a"},
			assertMetrics: func(t *testing.T, snap tally.Snapshot) {
				for _, c := range snap.Counters() {
					if c.Name() == "scheduler_worker_domain_coverage_count" {
						t.Errorf("domain coverage counter should not be emitted when lookup fails")
					}
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockDomainCache := cache.NewMockDomainCache(ctrl)
			mockDomainCache.EXPECT().GetAllDomain().Return(tc.domains)

			mockResolver := membership.NewMockResolver(ctrl)
			for domainName, hosts := range tc.lookupNResults {
				mockResolver.EXPECT().LookupN(service.Worker, domainName, workerRedundancyFactor).Return(hosts, nil)
			}
			for domainName, err := range tc.lookupNErrors {
				mockResolver.EXPECT().LookupN(service.Worker, domainName, workerRedundancyFactor).Return(nil, err)
			}

			ts := tally.NewTestScope("", nil)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			wm := &WorkerManager{
				enabledFn:          dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
				metricsClient:      metrics.NewClient(ts, metrics.Worker, metrics.MigrationConfig{}),
				logger:             testlogger.New(t),
				domainCache:        mockDomainCache,
				membershipResolver: mockResolver,
				hostInfo:           selfHost,
				activeWorkers:      make(map[string]workerHandle),
				ctx:                ctx,
				createWorker: func(domainName string) (workerHandle, error) {
					if tc.workerStartErr != nil {
						return nil, tc.workerStartErr
					}
					return &fakeWorker{}, nil
				},
			}
			for _, d := range tc.existingWorkers {
				wm.activeWorkers[d] = &fakeWorker{}
			}

			wm.refreshWorkers()

			tc.assertMetrics(t, ts.Snapshot())
		})
	}
}

func assertCounter(t *testing.T, snap tally.Snapshot, name string, tags map[string]string, want int64) {
	t.Helper()
	for _, c := range snap.Counters() {
		if c.Name() == name && tagsMatch(c.Tags(), tags) {
			assert.EqualValues(t, want, c.Value())
			return
		}
	}
	t.Errorf("counter %q with tags %v not found in snapshot", name, tags)
}

func assertGauge(t *testing.T, snap tally.Snapshot, name string, tags map[string]string, want float64) {
	t.Helper()
	for _, g := range snap.Gauges() {
		if g.Name() == name && tagsMatch(g.Tags(), tags) {
			assert.EqualValues(t, want, g.Value())
			return
		}
	}
	t.Errorf("gauge %q with tags %v not found in snapshot", name, tags)
}

func assertHistogramRecorded(t *testing.T, snap tally.Snapshot, name string) {
	t.Helper()
	for _, h := range snap.Histograms() {
		if h.Name() == name {
			return
		}
	}
	t.Errorf("histogram %q not found in snapshot", name)
}

func tagsMatch(actual, want map[string]string) bool {
	for k, v := range want {
		if actual[k] != v {
			return false
		}
	}
	return true
}

type fakeWorker struct {
	stopFn func()
}

func (f *fakeWorker) Stop() {
	if f.stopFn != nil {
		f.stopFn()
	}
}
