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

package stats

import (
	"math"
	"testing"
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/service/matching/event"
)

const floatResolution = 1e-6

func TestEmaFixedWindowQPSTracker(t *testing.T) {
	timeSource := clock.NewMockedTimeSourceAt(time.Now())
	exp := 0.4
	bucketInterval := time.Second

	r := NewEmaFixedWindowQPSTracker(timeSource, exp, bucketInterval, event.E{})
	r.Start()
	defer r.Stop()

	// Test ReportCounter
	r.ReportCounter(10)
	r.ReportCounter(20)

	qps := r.QPS()
	if qps != 0 {
		t.Errorf("QPS mismatch, expected: 0, got: %v", qps)
	}

	timeSource.BlockUntil(1)
	timeSource.Advance(bucketInterval)
	time.Sleep(10 * time.Millisecond)
	// Test QPS
	qps = r.QPS()
	expectedQPS := float64(30) / (float64(bucketInterval) / float64(time.Second))
	if math.Abs(qps-expectedQPS) > floatResolution {
		t.Errorf("QPS mismatch, expected: %v, got: %v", expectedQPS, qps)
	}

	r.ReportCounter(10)
	timeSource.BlockUntil(1)
	timeSource.Advance(bucketInterval)
	time.Sleep(10 * time.Millisecond)
	// Test QPS
	qps = r.QPS()
	expectedQPS = float64(22) / (float64(bucketInterval) / float64(time.Second))
	if math.Abs(qps-expectedQPS) > floatResolution {
		t.Errorf("QPS mismatch, expected: %v, got: %v", expectedQPS, qps)
	}
}

func TestRollingWindowQPSTracker(t *testing.T) {
	timeSource := clock.NewMockedTimeSourceAt(time.Now())
	bucketInterval := time.Second

	r := NewRollingWindowQPSTracker(timeSource, bucketInterval, 10)
	r.Start()
	defer r.Stop()

	r.ReportCounter(10)
	qps := r.QPS()
	if qps != 0 {
		t.Errorf("QPS mismatch, expected: 0, got: %v", qps)
	}

	timeSource.Advance(bucketInterval)

	qps = r.QPS()
	expectedQPS := float64(10) / (float64(bucketInterval) * 10 / float64(time.Second))
	if math.Abs(qps-expectedQPS) > floatResolution {
		t.Errorf("QPS mismatch, expected: %v, got: %v", expectedQPS, qps)
	}

	r.ReportCounter(20)
	qps = r.QPS()
	if qps != expectedQPS {
		t.Errorf("QPS mismatch, expected: %v, got: %v", expectedQPS, qps)
	}

	timeSource.Advance(bucketInterval)

	qps = r.QPS()
	expectedQPS = float64(30) / (float64(bucketInterval) * 10 / float64(time.Second))
	if math.Abs(qps-expectedQPS) > floatResolution {
		t.Errorf("QPS mismatch, expected: %v, got: %v", expectedQPS, qps)
	}

	r.ReportCounter(100)

	timeSource.Advance(8 * bucketInterval)

	qps = r.QPS()
	expectedQPS = float64(130) / (float64(bucketInterval) * 10 / float64(time.Second))
	if math.Abs(qps-expectedQPS) > floatResolution {
		t.Errorf("QPS mismatch, expected: %v, got: %v", expectedQPS, qps)
	}

	timeSource.Advance(bucketInterval)
	qps = r.QPS()
	expectedQPS = float64(120) / (float64(bucketInterval) * 10 / float64(time.Second))
	if math.Abs(qps-expectedQPS) > floatResolution {
		t.Errorf("QPS mismatch, expected: %v, got: %v", expectedQPS, qps)
	}

	timeSource.Advance(bucketInterval)
	qps = r.QPS()
	expectedQPS = float64(100) / (float64(bucketInterval) * 10 / float64(time.Second))
	if math.Abs(qps-expectedQPS) > floatResolution {
		t.Errorf("QPS mismatch, expected: %v, got: %v", expectedQPS, qps)
	}
}
