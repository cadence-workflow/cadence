// Copyright (c) 2017-2020 Uber Technologies Inc.
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

package resource

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas/global/algorithm"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/worker/archiver"
)

type (
	// Test is the test implementation used for testing
	Test struct {
		*resource.Test
		EventCache           *events.MockCache
		ratelimiterAlgorithm algorithm.RequestWeighted
		archiverClient       archiver.Client
	}
)

var _ Resource = (*Test)(nil)

// NewTest returns a new test resource instance
func NewTest(
	t *testing.T,
	controller *gomock.Controller,
	serviceMetricsIndex metrics.ServiceIdx,
) *Test {
	return &Test{
		Test:           resource.NewTest(t, controller, serviceMetricsIndex),
		EventCache:     events.NewMockCache(controller),
		archiverClient: &archiver.ClientMock{},
	}
}

// GetEventCache for testing
func (s *Test) GetEventCache() events.Cache {
	return s.EventCache
}

func (s *Test) GetRatelimiterAlgorithm() algorithm.RequestWeighted {
	return s.ratelimiterAlgorithm
}

func (s *Test) GetArchiverClient() archiver.Client {
	return s.archiverClient
}
