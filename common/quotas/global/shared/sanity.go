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

package shared

import (
	"fmt"
	"math"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
)

// SanityCheckFloat checks bounds and numerically-valid-ness, and returns an error string if any check fails.
//
// This is the "lower level" of SanityLogFloat, when invalid means behavior needs to change,
// and invalid data is an unavoidable possibility.
//
// If invalid data should *always* be avoided by earlier code, use SanityLogFloat instead to
// do a paranoid log of violations without affecting behavior.
func SanityCheckFloat[T ~float64](lower, actual, upper T, what string) (msg string) {
	if math.IsNaN(float64(actual)) {
		return what + " is NaN, should be impossible"
	} else if math.IsInf(float64(actual), 0) {
		return what + " is Inf, should be impossible"
	} else if actual < lower {
		return what + " is below lower bound, should be impossible"
	} else if upper < actual {
		return what + " is above upper bound, should be impossible"
	}
	return ""
}

// SanityLogFloat ensures values are within sane bounds (inclusive) and not NaN or Inf,
// and logs an error if these expectations are violated.
// This should only be used when a value is believed to *always* be valid, not
// when it needs to be clamped to a reasonable range.
//
// This largely exists because [math.Max] and similar retain NaN/Inf instead of
// treating them as error states, and that (and other issues) caused problems.
// There are a lot of potential gotchas with floats, and without good fuzzing
// you're kinda unlikely to trigger them all, so this helps us be as paranoid as
// we like with small enough cost to ignore (while also documenting expectations).
//
// The "actual" value is returned no matter what, to make this easier to use in
// a logic chain without temp vars.
//
// Violations are intentionally NOT fatal, nor is a "reasonable" value returned,
// as they are not expected to occur and code paths involved are not built with
// that assumption.  Consider it "undefined behavior", this log exists only to
// help troubleshoot the cause later if it happens.
//
// ---
//
// In tests, always assert that this is not ever triggered, by using the following:
//
//	logger, obs := logtest.NewObserved(t)
//	t.Cleanup(func() { AssertNoSanityCheckFailures(t, obs) })
//	// your test
func SanityLogFloat[T ~float64](lower, actual, upper T, what string, logger log.Logger) T {
	msg := SanityCheckFloat(lower, actual, upper, what)
	if msg != "" {
		logger.Error(msg,
			tag.Dynamic("lower", lower),
			tag.Dynamic("actual", actual),
			tag.Dynamic("upper", upper),
			tag.Dynamic("actual_type", fmt.Sprintf("%T", actual)))
	}

	return actual
}

// small and easier to mock in tests, as no other methods are needed
type testingt interface {
	Errorf(string, ...any)
}

func AssertNoSanityCheckFailures(t testingt, entries []observer.LoggedEntry) {
	logged := false
	for _, entry := range entries {
		if entry.Level.Enabled(zap.ErrorLevel) && strings.Contains(entry.Message, "impossible") {
			t.Errorf("sanity check log detected: %v: %v", entry.Message, entry.Context)
			if logged {
				t.Errorf("ignoring any remaining sanity check failures, comment this out to get all logs")
				// produces 2 logs and then gives up, as large or fuzz tests can trigger thousands and that's very slow
				break
			}
			logged = true
		}
	}
}
