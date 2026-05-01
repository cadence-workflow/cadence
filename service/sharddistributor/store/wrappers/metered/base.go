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

package metered

import (
	"errors"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/service/sharddistributor/store"
)

type base struct {
	metricClient metrics.Client
	logger       log.Logger
	timeSource   clock.TimeSource
}

func (p *base) updateErrorMetricPerNamespace(err error, scopeWithNamespaceTags metrics.Scope) {
	if errors.Is(err, store.ErrExecutorNotFound) {
		scopeWithNamespaceTags.IncCounter(metrics.ShardDistributorStoreExecutorNotFound)
	}

	scopeWithNamespaceTags.IncCounter(metrics.ShardDistributorStoreFailuresPerNamespace)
}

func (p *base) call(scope metrics.ScopeIdx, op func() error, tags ...metrics.Tag) error {
	metricsScope := p.metricClient.Scope(scope, tags...)

	metricsScope.IncCounter(metrics.ShardDistributorStoreRequestsPerNamespace)
	before := p.timeSource.Now()
	err := op()
	duration := p.timeSource.Since(before)
	metricsScope.RecordHistogramDuration(metrics.ShardDistributorStoreLatencyHistogramPerNamespace, duration)

	if err != nil {
		p.updateErrorMetricPerNamespace(err, metricsScope)
	}
	return err
}
