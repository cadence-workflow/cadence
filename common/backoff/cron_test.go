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
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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

func TestCron(t *testing.T) {
	for idx, tt := range crontests {
		t.Run(strconv.Itoa(idx), func(t *testing.T) {
			start, _ := time.Parse(time.RFC3339, tt.startTime)
			end := start
			if tt.endTime != "" {
				end, _ = time.Parse(time.RFC3339, tt.endTime)
			}
			err := ValidateSchedule(tt.cron)
			if tt.result != NoBackoff {
				assert.NoError(t, err)
			}
			backoff := GetBackoffForNextSchedule(tt.cron, start, end, 0)
			assert.Equal(t, tt.result, backoff, "The cron spec is %s and the expected result is %s", tt.cron, tt.result)
		})
	}
}

var cronWithJitterStartTests = []struct {
	cron               string
	startTime          string
	jitterStartSeconds int32
	result             time.Duration
}{
	{"* * * * *", "2018-12-17T08:00:00+00:00", 10, time.Second * 60},
	{"* * * * *", "2018-12-17T08:00:10+00:00", 30, time.Second * 50},
	{"* * * * *", "2018-12-17T08:00:25+00:00", 5, time.Second * 35},
	{"* * * * *", "2018-12-17T08:00:59+00:00", 45, time.Second * 1},
}

func TestCronWithJitterStart(t *testing.T) {
	for idx, tt := range cronWithJitterStartTests {
		t.Run(strconv.Itoa(idx), func(t *testing.T) {
			start, _ := time.Parse(time.RFC3339, tt.startTime)
			end := start
			err := ValidateSchedule(tt.cron)
			if tt.result != NoBackoff {
				assert.NoError(t, err)
			}
			backoff := GetBackoffForNextSchedule(tt.cron, start, end, tt.jitterStartSeconds)
			delta := time.Duration(tt.jitterStartSeconds) * time.Second
			resultTime := start.Add(tt.result)
			backoffTime := start.Add(backoff)
			assert.WithinDuration(t, resultTime, backoffTime, delta, "The cron spec is %s and the expected result is between %s and %s",
				tt.cron, tt.result, tt.result+delta)
		})
	}
}
