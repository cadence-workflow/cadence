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

package activecluster

import (
	"sync/atomic"
	"time"
)

var (
	wf1FailoverTime = 60 * time.Second

	cluster0Result = &LookupResult{
		Region:          "region0",
		ClusterName:     "cluster0",
		FailoverVersion: 1,
	}

	cluster1Result = &LookupResult{
		Region:          "region1",
		ClusterName:     "cluster1",
		FailoverVersion: 2,
	}
)

// TODO(active-active): Remove below fake implementation and implement properly
// - lookup active region given <domain id, wf id, run id> from executions table RowType=ActiveCluster.
// - cache this info
// - add metrics for cache hit/miss
// - return cluster name

// Fake logic:
//   - wf1 is active in cluster0 for first 60 seconds, then active in cluster1.
//     Note: Simulation sleeps for 30s in the beginning and runs wf1 for 60s. So wf1 should start in cluster0 and complete in cluster1.
//   - other workflows are always active in cluster1
func (m *manager) fakeLookupWorkflow(wfID string) (*LookupResult, error) {
	if wfID == "wf1" && (m.wf1StartTime.IsZero() || atomic.LoadInt32(&m.wf1FailedOver) == 0) {
		if m.wf1StartTime.IsZero() {
			m.logger.Debug("Initializing wf1 failover timer")
			m.wf1StartTime = time.Now()
			go m.fakeEntityMapChange()
		}
		m.logger.Debug("Returning cluster0 for wf1")
		return cluster0Result, nil
	}

	if wfID == "wf1" {
		m.logger.Debug("Returning cluster1 for wf1")
	}

	return cluster1Result, nil
}

func (m *manager) fakeEntityMapChange() {
	// Based on the fake logic, wf1 will failover to cluster1 after 60 seconds.
	t := time.NewTimer(wf1FailoverTime)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			m.logger.Debugf("Faking wf1 failover")
			atomic.CompareAndSwapInt32(&m.wf1FailedOver, 0, 1)
			m.notifyChangeCallbacks(ChangeTypeEntityMap)
		case <-m.ctx.Done():
			return
		}
	}
}
