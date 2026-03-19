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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
)

func TestRefreshWorkers(t *testing.T) {
	selfHost := membership.NewDetailedHostInfo("10.0.0.1:7933", "self", nil)
	otherHost := membership.NewDetailedHostInfo("10.0.0.2:7933", "other", nil)

	makeDomainEntry := func(name string) *cache.DomainCacheEntry {
		return cache.NewDomainCacheEntryForTest(
			&persistence.DomainInfo{Name: name},
			nil, false, nil, 0, nil, 0, 0, 0,
		)
	}

	tests := []struct {
		name               string
		domains            map[string]*cache.DomainCacheEntry
		lookupResults      map[string]membership.HostInfo
		lookupErrors       map[string]error
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
			lookupResults: map[string]membership.HostInfo{
				"domain-a": selfHost,
				"domain-b": selfHost,
			},
			wantActiveWorkers:  []string{"domain-a", "domain-b"},
			wantStartedWorkers: []string{"domain-a", "domain-b"},
		},
		{
			name: "skips domains owned by other hosts",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-a": makeDomainEntry("domain-a"),
				"domain-b": makeDomainEntry("domain-b"),
			},
			lookupResults: map[string]membership.HostInfo{
				"domain-a": selfHost,
				"domain-b": otherHost,
			},
			wantActiveWorkers:  []string{"domain-a"},
			wantStartedWorkers: []string{"domain-a"},
		},
		{
			name: "stops workers for domains no longer owned",
			domains: map[string]*cache.DomainCacheEntry{
				"domain-a": makeDomainEntry("domain-a"),
			},
			lookupResults: map[string]membership.HostInfo{
				"domain-a": otherHost,
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
			lookupResults: map[string]membership.HostInfo{
				"domain-b": selfHost,
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
			lookupErrors: map[string]error{
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
			lookupResults: map[string]membership.HostInfo{
				"domain-a": selfHost,
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
			for domainName, host := range tc.lookupResults {
				mockResolver.EXPECT().Lookup(service.Worker, domainName).Return(host, nil)
			}
			for domainName, err := range tc.lookupErrors {
				mockResolver.EXPECT().Lookup(service.Worker, domainName).Return(membership.HostInfo{}, err)
			}

			stopped := make(map[string]bool)
			started := make(map[string]bool)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			wm := &WorkerManager{
				enabledFn:          dynamicproperties.GetBoolPropertyFn(true),
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
	mockResolver.EXPECT().Lookup(service.Worker, "domain-a").Return(selfHost, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wm := &WorkerManager{
		enabledFn:          dynamicproperties.GetBoolPropertyFn(true),
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

type fakeWorker struct {
	stopFn func()
}

func (f *fakeWorker) Stop() {
	if f.stopFn != nil {
		f.stopFn()
	}
}
