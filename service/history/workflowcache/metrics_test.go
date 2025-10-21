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

package workflowcache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/metrics"
)

func TestUpdatePerDomainMaxWFRequestCount(t *testing.T) {
	domainName := "some domain name"
	metric := metrics.WorkflowIDCacheRequestsInternalMaxRequestsPerSecondsTimer

	cases := []struct {
		name                             string
		updatePerDomainMaxWFRequestCount func(metricsClient metrics.Client, source clock.MockedTimeSource)
		expecetMetrics                   []time.Duration
	}{
		{
			name: "Single workflowID",
			updatePerDomainMaxWFRequestCount: func(metricsClient metrics.Client, timeSource clock.MockedTimeSource) {
				workflowID1 := &workflowIDCountMetric{}
				workflowID1.updatePerDomainMaxWFRequestCount(domainName, timeSource, metricsClient, metric) // Emits 1
				workflowID1.updatePerDomainMaxWFRequestCount(domainName, timeSource, metricsClient, metric) // Emits 2
			},
			expecetMetrics: []time.Duration{1, 2},
		},
		{
			name: "Separate workflowIDs",
			updatePerDomainMaxWFRequestCount: func(metricsClient metrics.Client, timeSource clock.MockedTimeSource) {
				workflowID1 := &workflowIDCountMetric{}
				workflowID1.updatePerDomainMaxWFRequestCount(domainName, timeSource, metricsClient, metric) // Emits 1

				workflowID2 := &workflowIDCountMetric{}
				workflowID2.updatePerDomainMaxWFRequestCount(domainName, timeSource, metricsClient, metric) // Emits 1
				workflowID2.updatePerDomainMaxWFRequestCount(domainName, timeSource, metricsClient, metric) // Emits 2
				workflowID2.updatePerDomainMaxWFRequestCount(domainName, timeSource, metricsClient, metric) // Emits 3

				workflowID1.updatePerDomainMaxWFRequestCount(domainName, timeSource, metricsClient, metric) // Emits 2

			},
			expecetMetrics: []time.Duration{1, 1, 2, 3, 2},
		},
		{
			name: "Reset",
			updatePerDomainMaxWFRequestCount: func(metricsClient metrics.Client, timeSource clock.MockedTimeSource) {
				workflowID1 := &workflowIDCountMetric{}
				workflowID1.updatePerDomainMaxWFRequestCount(domainName, timeSource, metricsClient, metric) // Emits 1
				workflowID1.updatePerDomainMaxWFRequestCount(domainName, timeSource, metricsClient, metric) // Emits 2

				timeSource.Advance(1100 * time.Millisecond)
				workflowID1.updatePerDomainMaxWFRequestCount(domainName, timeSource, metricsClient, metric) // Emits 1
			},
			expecetMetrics: []time.Duration{1, 2, 1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testScope := tally.NewTestScope("", make(map[string]string))
			timeSource := clock.NewMockedTimeSourceAt(time.Unix(123, 456))
			metricsClient := metrics.NewClient(testScope, metrics.History, metrics.HistogramMigration{})

			tc.updatePerDomainMaxWFRequestCount(metricsClient, timeSource)

			// We expect the domain tag to be set to "all" and "some domain name", we don't know the order, so use a set
			expectedDomainTags := map[string]struct{}{"all": {}, "some domain name": {}}
			actualDomainTags := map[string]struct{}{}

			timers := testScope.Snapshot().Timers()
			assert.Equal(t, 2, len(timers))
			for _, v := range timers {
				actualDomainTags[v.Tags()["domain"]] = struct{}{}
				assert.Equal(t, tc.expecetMetrics, v.Values())
			}

			assert.Equal(t, expectedDomainTags, actualDomainTags)
		})
	}

}
