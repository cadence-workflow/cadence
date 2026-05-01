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

package sampled

import (
	"sync"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/tokenbucket"
)

type RateLimiterFactoryFunc func(timeSource clock.TimeSource, numOfPriority int, qpsConfig dynamicproperties.IntPropertyFnWithDomainFilter) RateLimiterFactory

type RateLimiterFactory interface {
	GetRateLimiter(domain string) tokenbucket.PriorityTokenBucket
}

type domainToBucketMap struct {
	sync.RWMutex
	timeSource    clock.TimeSource
	qpsConfig     dynamicproperties.IntPropertyFnWithDomainFilter
	numOfPriority int
	mappings      map[string]tokenbucket.PriorityTokenBucket
}

// NewDomainToBucketMap returns a rate limiter factory.
func NewDomainToBucketMap(timeSource clock.TimeSource, numOfPriority int, qpsConfig dynamicproperties.IntPropertyFnWithDomainFilter) RateLimiterFactory {
	return &domainToBucketMap{
		timeSource:    timeSource,
		qpsConfig:     qpsConfig,
		numOfPriority: numOfPriority,
		mappings:      make(map[string]tokenbucket.PriorityTokenBucket),
	}
}

func (m *domainToBucketMap) GetRateLimiter(domain string) tokenbucket.PriorityTokenBucket {
	m.RLock()
	rateLimiter, exist := m.mappings[domain]
	m.RUnlock()

	if exist {
		return rateLimiter
	}

	m.Lock()
	if rateLimiter, ok := m.mappings[domain]; ok { // read again to ensure no duplicate create
		m.Unlock()
		return rateLimiter
	}
	rateLimiter = tokenbucket.NewFullPriorityTokenBucket(m.numOfPriority, m.qpsConfig(domain), m.timeSource)
	m.mappings[domain] = rateLimiter
	m.Unlock()
	return rateLimiter
}
