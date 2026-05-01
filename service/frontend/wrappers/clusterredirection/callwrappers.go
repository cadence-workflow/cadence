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

package clusterredirection

import (
	"time"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

type (
	domainIDGetter interface {
		GetDomainID() string
	}
)

func (handler *clusterRedirectionHandler) beforeCall(
	scope metrics.ScopeIdx,
) (metrics.Scope, time.Time) {
	return handler.GetMetricsClient().Scope(scope), handler.GetTimeSource().Now()
}

func (handler *clusterRedirectionHandler) afterCall(
	recovered interface{},
	scope metrics.Scope,
	startTime time.Time,
	domainEntry *cache.DomainCacheEntry,
	cluster string,
	retError *error,
) {
	var extraTags []tag.Tag
	if domainEntry != nil {
		extraTags = append(extraTags, tag.WorkflowDomainName(domainEntry.GetInfo().Name))
		extraTags = append(extraTags, tag.WorkflowDomainID(domainEntry.GetInfo().ID))
	}
	log.CapturePanic(recovered, handler.GetLogger().WithTags(extraTags...), retError)

	scope = scope.Tagged(metrics.TargetClusterTag(cluster))
	scope.IncCounter(metrics.CadenceDcRedirectionClientRequests)
	scope.RecordTimer(metrics.CadenceDcRedirectionClientLatency, handler.GetTimeSource().Now().Sub(startTime))
	if *retError != nil {
		scope.IncCounter(metrics.CadenceDcRedirectionClientFailures)
	}
}
