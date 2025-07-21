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
	"math"
	"math/rand"
	"time"

	"github.com/robfig/cron"

	"github.com/uber/cadence/common/types"
)

// NoBackoff is used to represent backoff when no cron backoff is needed
const NoBackoff = time.Duration(-1)

// ValidateSchedule validates a cron schedule spec
func ValidateSchedule(cronSchedule string) (cron.Schedule, error) {
	sched, err := cron.ParseStandard(cronSchedule)
	if err != nil {
		return nil, &types.BadRequestError{
			Message: fmt.Sprintf("Invalid CronSchedule, failed to parse: %q, err: %v", cronSchedule, err),
		}
	}
	// schedule must parse and there must be a next-firing date (catches impossible dates like Feb 30)
	next := sched.Next(time.Now())
	if next.IsZero() {
		return nil, &types.BadRequestError{
			Message: fmt.Sprintf("Invalid CronSchedule, no next firing time found, maybe impossible date: %q", cronSchedule),
		}
	}
	return sched, nil
}

// GetBackoffForNextSchedule calculates the backoff time for the next run given
// a cronSchedule, workflow start time and workflow close time
func GetBackoffForNextSchedule(
	sched cron.Schedule,
	startTime time.Time,
	closeTime time.Time,
	jitterStartSeconds int32,
	cronOverlapPolicy types.CronOverlapPolicy,
) (time.Duration, error) {
	startUTCTime := startTime.In(time.UTC)
	closeUTCTime := closeTime.In(time.UTC)
	nextScheduleTime := sched.Next(startUTCTime)
	roundedInterval := time.Duration(0)
	if nextScheduleTime.IsZero() {
		// this should only occur for bad specs, e.g. impossible dates like Feb 30,
		// which should be prevented from being saved by the valid check.
		return NoBackoff, fmt.Errorf("invalid CronSchedule, no next firing time found")
	}

	if nextScheduleTime.Before(closeUTCTime) {
		// Cron overlap policy only applies if there were runs skipped
		switch cronOverlapPolicy {
		case types.CronOverlapPolicySkipped:
			for nextScheduleTime.Before(closeUTCTime) {
				nextScheduleTime = sched.Next(nextScheduleTime)
				if nextScheduleTime.IsZero() {
					// this should only occur for bad specs, e.g. impossible dates like Feb 30,
					// which should be prevented from being saved by the valid check.
					return NoBackoff, fmt.Errorf("invalid CronSchedule, no next firing time found")
				}
			}
			backoffInterval := nextScheduleTime.Sub(closeUTCTime)
			roundedInterval = time.Second * time.Duration(math.Ceil(backoffInterval.Seconds()))
		case types.CronOverlapPolicyBufferOne:
			// we want to start the next run as soon as possible, so we don't need to buffer
			roundedInterval = time.Duration(0)
		}
	} else {
		backoffInterval := nextScheduleTime.Sub(closeUTCTime)
		roundedInterval = time.Second * time.Duration(math.Ceil(backoffInterval.Seconds()))
	}

	var jitter time.Duration
	if jitterStartSeconds > 0 {
		jitter = time.Duration(rand.Int31n(jitterStartSeconds+1)) * time.Second
	}

	return roundedInterval + jitter, nil
}

// GetBackoffForNextScheduleInSeconds calculates the backoff time in seconds for the
// next run given a cronSchedule and current time
func GetBackoffForNextScheduleInSeconds(
	cronSchedule string,
	startTime time.Time,
	closeTime time.Time,
	jitterStartSeconds int32,
	overlapPolicy types.CronOverlapPolicy,
) (int32, error) {
	sched, err := ValidateSchedule(cronSchedule)
	if err != nil {
		return 0, err
	}
	backoffDuration, err := GetBackoffForNextSchedule(sched, startTime, closeTime, jitterStartSeconds, overlapPolicy)
	if err != nil {
		return 0, err
	}
	return int32(math.Ceil(backoffDuration.Seconds())), nil
}
