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

package collection

import (
	"math"
	"testing"

	"golang.org/x/time/rate"
)

func FuzzBoostRPS(f *testing.F) {
	f.Fuzz(func(t *testing.T, target, fallback, weight, used float64) {
		target = math.Abs(target)
		fallback = math.Abs(fallback)
		used = math.Abs(used)
		weight = weight - math.Floor(weight) // trim to 0..1

		if anyInvalid(target, fallback, used, weight) {
			t.Skip("bad numbers")
		}

		if target < fallback {
			// fallback is always equal or below target, as it's `target / num hosts`.
			target, fallback = fallback, target
		}

		boosted := boostRPS(rate.Limit(target), rate.Limit(fallback), weight, used)

		if boosted > rate.Limit(target) {
			// should never exceed whole-cluster target
			t.Error("boosted beyond configured limit")
		}
		if boosted < 0 {
			// should never become negative.
			//
			// the ratelimiter treats negatives as zero, so this is "fine",
			// but it's likely a sign of flawed logic.
			t.Error("boosted is negative")
		}
		if math.IsNaN(float64(boosted)) {
			t.Error("boosted is NaN")
		}
		if math.IsInf(float64(boosted), 0) {
			t.Error("boosted is inf")
		}
	})
}

func anyInvalid(f ...float64) bool {
	for _, v := range f {
		if math.IsNaN(v) {
			return true
		}
		if math.IsInf(v, 0) {
			return true
		}
	}
	return false
}
