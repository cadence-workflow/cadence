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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/log/testlogger"
)

func TestSanityChecks(t *testing.T) {
	tests := map[string]struct {
		lower, actual, upper float64
		expected             string
	}{
		"nan":      {lower: 0, actual: math.NaN(), upper: 0, expected: "is NaN"},
		"pos inf":  {lower: 0, actual: math.Inf(1), upper: 0, expected: "is Inf"},
		"neg inf":  {lower: 0, actual: math.Inf(-1), upper: 0, expected: "is Inf"},
		"too low":  {lower: 5, actual: 1, upper: 10, expected: "below lower bound"},
		"too high": {lower: 0, actual: 10, upper: 5, expected: "above upper bound"},

		"inclusive lower": {lower: 0, actual: 0, upper: 10, expected: ""},
		"valid":           {lower: 0, actual: 5, upper: 10, expected: ""},
		"inclusive upper": {lower: 0, actual: 10, upper: 10, expected: ""},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			msg := SanityCheckFloat(test.lower, test.actual, test.upper, "test")
			if test.expected == "" {
				assert.Empty(t, msg, "should not have failed validation")
			} else {
				assert.Containsf(t, msg, test.expected, "wrong or missing error case")
			}
		})
	}

	t.Run("logs", func(t *testing.T) {
		l, obs := testlogger.NewObserved(t)
		_ = SanityLogFloat[float64](5, 1, 10, "test", l)
		logs := obs.TakeAll()
		require.Len(t, logs, 1, "should have made an error log")
		assert.Contains(t, logs[0].Message, "below lower bound", "should log the issue")
		assert.Contains(t, logs[0].Message, "test", "should mention the 'what'")
		assert.NotZero(t, logs[0].ContextMap()["actual"], "should log the actual value")
	})
	t.Run("assert helper", func(t *testing.T) {
		l, obs := testlogger.NewObserved(t)
		_ = SanityLogFloat[float64](5, 1, 10, "test", l)
		logs := obs.TakeAll()
		require.Len(t, logs, 1, "should have made an error log")

		errobs := &errorfobserver{}
		AssertNoSanityCheckFailures(errobs, logs)
		require.Len(t, errobs.logs, 1, "should Errorf that there was a log")
		assert.Contains(t, errobs.logs[0], "sanity check log detected")
		assert.Contains(t, errobs.logs[0], "below lower bound", "should contain the original message")

		errobs = &errorfobserver{}                  // clear the logs
		tripleLog := append(logs, logs[0], logs[0]) // fake having three logs, details don't matter
		AssertNoSanityCheckFailures(errobs, tripleLog)
		require.Len(t, errobs.logs, 3, "should have two Errorfs and one giving up")
		assert.Contains(t, errobs.logs[0], "sanity check log detected")
		assert.Contains(t, errobs.logs[1], "sanity check log detected")
		assert.Contains(t, errobs.logs[2], "ignoring any remaining sanity check failures")
	})
}

type errorfobserver struct {
	logs []string
}

func (o *errorfobserver) Errorf(format string, args ...interface{}) {
	o.logs = append(o.logs, fmt.Sprintf(format, args...))
}
