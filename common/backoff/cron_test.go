// Copyright (c) 2019 Uber Technologies, Inc.
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

package backoff

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/types"
)

func TestCron(t *testing.T) {
	var crontests = []struct {
		cron      string
		startTime string
		endTime   string
		result    time.Duration
	}{
		{"0 10 * * *", "2018-12-17T08:00:00-08:00", "", time.Hour * 18},
		{"0 10 * * *", "2018-12-18T02:00:00-08:00", "", time.Hour * 24},
		{"0 * * * *", "2018-12-17T08:08:00+00:00", "", time.Minute * 52},
		{"0 * * * *", "2018-12-17T09:00:00+00:00", "", time.Hour},
		{"* * * * *", "2018-12-17T08:08:18+00:00", "", time.Second * 42},
		{"0 * * * *", "2018-12-17T09:00:00+00:00", "", time.Minute * 60},
		{"0 10 * * *", "2018-12-17T08:00:00+00:00", "2018-12-20T00:00:00+00:00", time.Hour * 10},
		{"0 10 * * *", "2018-12-17T08:00:00+00:00", "2018-12-17T09:00:00+00:00", time.Hour},
		{"*/10 * * * *", "2018-12-17T00:04:00+00:00", "2018-12-17T01:02:00+00:00", time.Minute * 8},
		{"invalid-cron-spec", "2018-12-17T00:04:00+00:00", "2018-12-17T01:02:00+00:00", NoBackoff},
		{"@every 5h", "2018-12-17T08:00:00+00:00", "2018-12-17T09:00:00+00:00", time.Hour * 4},
		{"@every 5h", "2018-12-17T08:00:00+00:00", "2018-12-18T00:00:00+00:00", time.Hour * 4},
		{"0 3 * * 0-6", "2018-12-17T08:00:00-08:00", "", time.Hour * 11},
	}
	for idx, tt := range crontests {
		t.Run(strconv.Itoa(idx), func(t *testing.T) {
			start, _ := time.Parse(time.RFC3339, tt.startTime)
			end := start
			if tt.endTime != "" {
				end, _ = time.Parse(time.RFC3339, tt.endTime)
			}
			sched, err := ValidateSchedule(tt.cron)
			if tt.result == NoBackoff {
				// no backoff == error, simplifies the test-struct a bit
				require.ErrorContains(t, err, "Invalid CronSchedule")
			} else {
				require.NoError(t, err)
				backoff, err := GetBackoffForNextSchedule(sched, start, end, 0, types.CronOverlapPolicySkipped)
				require.NoError(t, err)
				assert.Equal(t, tt.result, backoff, "The cron spec is %s and the expected result is %s", tt.cron, tt.result)
			}
		})
	}
}

func TestCronWithJitterStart(t *testing.T) {
	var cronWithJitterStartTests = []struct {
		cron                   string
		startTime              string
		jitterStartSeconds     int32
		expectedResultSeconds  time.Duration
		expectedResultSeconds2 time.Duration
	}{
		// Note that the cron scheduler we use (robfig) schedules differently depending on the syntax:
		// 1) * * * syntax : next run is scheduled on the first second of each minute, starting at next minute
		// 2) @every X syntax: next run is scheduled X seconds from the time passed in to Next() call.
		{"* * * * *", "2018-12-17T08:00:00+00:00", 10, time.Second * 60, time.Second * 60},
		{"* * * * *", "2018-12-17T08:00:10+00:00", 30, time.Second * 50, time.Second * 60},
		{"* * * * *", "2018-12-17T08:00:25+00:00", 15, time.Second * 35, time.Second * 60},
		{"* * * * *", "2018-12-17T08:00:45+00:00", 0, time.Second * 15, time.Second * 60},
		{"@every 60s", "2018-12-17T08:00:45+00:00", 0, time.Second * 60, time.Second * 60},
		{"* * * * *", "2018-12-17T08:00:45+00:00", 45, time.Second * 15, time.Second * 60},
		{"@every 60s", "2018-12-17T08:00:45+00:00", 45, time.Second * 60, time.Second * 60},
		{"* * * * *", "2018-12-17T08:00:00+00:00", 70, time.Second * 60, time.Second * 60},
		{"@every 20s", "2018-12-17T08:00:00+00:00", 15, time.Second * 20, time.Second * 20},
		{"@every 10s", "2018-12-17T08:00:09+00:00", 0, time.Second * 10, time.Second * 10},
		{"@every 20s", "2018-12-17T08:00:09+00:00", 15, time.Second * 20, time.Second * 20},
		{"* * * * *", "0001-01-01T00:00:00+00:00", 0, time.Second * 60, time.Second * 60},
		{"@every 20s", "0001-01-01T00:00:00+00:00", 0, time.Second * 20, time.Second * 20},
	}

	rand.Seed(int64(time.Now().Nanosecond()))
	for idx, tt := range cronWithJitterStartTests {
		t.Run(strconv.Itoa(idx), func(t *testing.T) {
			exactCount := 0

			start, _ := time.Parse(time.RFC3339, tt.startTime)
			end := start
			sched, err := ValidateSchedule(tt.cron)
			if tt.expectedResultSeconds != NoBackoff {
				assert.NoError(t, err)
			}
			backoff, err := GetBackoffForNextSchedule(sched, start, end, tt.jitterStartSeconds, types.CronOverlapPolicySkipped)
			require.NoError(t, err)
			fmt.Printf("Backoff time for test %d = %v\n", idx, backoff)
			delta := time.Duration(tt.jitterStartSeconds) * time.Second
			expectedResultTime := start.Add(tt.expectedResultSeconds)
			backoffTime := start.Add(backoff)
			assert.WithinDuration(t, expectedResultTime, backoffTime, delta,
				"The test specs are %v and the expected result in seconds is between %s and %s",
				tt, tt.expectedResultSeconds, tt.expectedResultSeconds+delta)
			if expectedResultTime == backoffTime {
				exactCount++
			}

			// Also check next X cron times
			caseCount := 5
			for i := 1; i < caseCount; i++ {
				startTime := expectedResultTime

				backoff, err := GetBackoffForNextSchedule(sched, startTime, startTime, tt.jitterStartSeconds, types.CronOverlapPolicySkipped)
				require.NoError(t, err)
				expectedResultTime := startTime.Add(tt.expectedResultSeconds2)
				backoffTime := startTime.Add(backoff)
				assert.WithinDuration(t, expectedResultTime, backoffTime, delta,
					"Iteration %d: The test specs are %v and the expected result in seconds is between %s and %s",
					i, tt, tt.expectedResultSeconds, tt.expectedResultSeconds+delta)
				if expectedResultTime == backoffTime {
					exactCount++
				}

			}

			// If jitter is > 0, we want to detect whether jitter is being applied - BUT we don't want the test
			// to be flaky if the code randomly chooses a jitter of 0, so we try to have enough data points by
			// checking the next X cron times AND by choosing a jitter thats not super low.

			if tt.jitterStartSeconds > 0 && exactCount == caseCount {
				// Test to make sure a jitter test case sometimes doesn't get exact values
				t.Fatalf("FAILED to jitter properly? Test specs = %v\n", tt)
			} else if tt.jitterStartSeconds == 0 && exactCount != caseCount {
				// Test to make sure a non-jitter test case always gets exact values
				t.Fatalf("Jittered when we weren't supposed to? Test specs = %v\n", tt)
			}

		})
	}
}

func TestCronWithoutBufferOneCronWorkflow(t *testing.T) {
	var bufferOneCronWorkflowTests = []struct {
		startTime       string
		closeTime       string
		expectedBackoff time.Duration
		description     string
	}{
		{
			startTime:       "2018-12-17T10:00:00+00:00",
			closeTime:       "2018-12-17T11:00:00+00:00",
			expectedBackoff: time.Hour * 23,
			description:     "Next schedule is next day at 10:00",
		},
		{
			startTime:       "2018-12-17T10:00:00+00:00",
			closeTime:       "2018-12-18T10:45:00+00:00",
			expectedBackoff: time.Hour*23 + time.Minute*15,
			description:     "Skip that run and schedule at next possible time",
		},
	}

	for idx, tt := range bufferOneCronWorkflowTests {
		t.Run(fmt.Sprintf("%d_%s", idx, tt.description), func(t *testing.T) {
			start, err := time.Parse(time.RFC3339, tt.startTime)
			require.NoError(t, err)
			close, err := time.Parse(time.RFC3339, tt.closeTime)
			require.NoError(t, err)

			sched, err := ValidateSchedule("0 10 * * *")
			require.NoError(t, err)

			backoff, err := GetBackoffForNextSchedule(sched, start, close, 0, types.CronOverlapPolicySkipped)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBackoff, backoff,
				"Test case %d failed: %s\nStart: %s\nClose: %s",
				idx, tt.description, tt.startTime, tt.closeTime)
		})
	}
}

func TestCronWithBufferOneCronWorkflow(t *testing.T) {
	var bufferOneCronWorkflowTests = []struct {
		startTime       string
		closeTime       string
		expectedBackoff time.Duration
		description     string
	}{
		{
			startTime:       "2018-12-17T10:00:00+00:00",
			closeTime:       "2018-12-17T11:00:00+00:00",
			expectedBackoff: time.Hour * 23,
			description:     "Next schedule is next day at 10:00",
		},
		{
			startTime:       "2018-12-17T10:00:00+00:00",
			closeTime:       "2018-12-18T10:45:00+00:00",
			expectedBackoff: 0,
			description:     "Start immediately after previous close time",
		},
		{
			startTime:       "2018-12-17T10:00:00+00:00",
			closeTime:       "2018-12-17T10:30:00+00:00",
			expectedBackoff: time.Hour*23 + time.Minute*30,
			description:     "Close time is after schedule time",
		},
		{
			startTime:       "2018-12-17T10:00:00+00:00",
			closeTime:       "2018-12-30T10:45:00+00:00",
			expectedBackoff: 0,
			description:     "Start immediately after previous close time even if previous one skipped multiple runs",
		},
	}

	for idx, tt := range bufferOneCronWorkflowTests {
		t.Run(fmt.Sprintf("%d_%s", idx, tt.description), func(t *testing.T) {
			start, err := time.Parse(time.RFC3339, tt.startTime)
			require.NoError(t, err)
			close, err := time.Parse(time.RFC3339, tt.closeTime)
			require.NoError(t, err)

			sched, err := ValidateSchedule("0 10 * * *")
			require.NoError(t, err)

			backoff, err := GetBackoffForNextSchedule(sched, start, close, 0, types.CronOverlapPolicyBufferOne)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBackoff, backoff,
				"Test case %d failed: %s\nStart: %s\nClose: %s",
				idx, tt.description, tt.startTime, tt.closeTime)
		})
	}
}
