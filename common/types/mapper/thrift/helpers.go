// Copyright (c) 2017-2022 Uber Technologies Inc.

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

package thrift

import (
	"time"

	"github.com/uber/cadence/common"
)

func timeToNano(t *time.Time) *int64 {
	if t == nil {
		return nil
	}

	return common.Int64Ptr(t.UnixNano())
}

func nanoToTime(nanos *int64) *time.Time {
	if nanos == nil {
		return nil
	}

	result := time.Unix(0, *nanos)
	return &result
}

func timeValToNano(t time.Time) *int64 {
	if t.IsZero() {
		return nil
	}
	return common.Int64Ptr(t.UnixNano())
}

func nanoToTimeVal(n *int64) time.Time {
	if n == nil {
		return time.Time{}
	}
	return time.Unix(0, *n)
}

func durationToSeconds(d time.Duration) *int32 {
	if d == 0 {
		return nil
	}
	s := int32(d / time.Second)
	return &s
}

func secondsToDuration(s *int32) time.Duration {
	if s == nil {
		return 0
	}
	return time.Duration(*s) * time.Second
}
