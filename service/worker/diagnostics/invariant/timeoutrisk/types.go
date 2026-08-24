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

package timeoutrisk

import (
	"time"

	"github.com/uber/cadence/common/types"
)

type TimeoutRiskType string

const (
	ActivityStartToCloseAtWorkflowTimeoutCap TimeoutRiskType = "Activity StartToClose timeout at the workflow execution timeout cap"
	ActivityMissingHeartbeatTimeout          TimeoutRiskType = "Long-running activity missing heartbeat timeout"
	ActivityLongScheduleToStartWithRetries   TimeoutRiskType = "Long ScheduleToStart timeout with retry policy allowing multiple attempts"
)

func (t TimeoutRiskType) String() string {
	return string(t)
}

type IssueType string

const (
	StartToCloseAtWorkflowTimeoutCap       IssueType = "Activity StartToClose timeout may have been silently capped at the workflow execution timeout by the server, leaving no headroom for retrying."
	MissingHeartbeatTimeoutForLongActivity IssueType = "Long-running activity without a HeartbeatTimeout will leave worker failures undetected until the activity times out."
	LongScheduleToStartWithMultipleRetries IssueType = "Long ScheduleToStartTimeout with a retry policy allowing multiple attempts can multiply recovery time during a failover or poller outage."
)

func (i IssueType) String() string {
	return string(i)
}

const (
	// longRunningActivityThresholdSeconds is the StartToCloseTimeout above which an activity is
	// considered long-running and should be configured with a HeartbeatTimeout.
	longRunningActivityThresholdSeconds int32 = 10 * 60

	// longScheduleToStartThresholdSeconds is the ScheduleToStartTimeout above which, combined with a
	// retry policy allowing multiple attempts, a failover or poller outage can significantly delay recovery.
	longScheduleToStartThresholdSeconds int32 = 5 * 60

	// minRetryAttemptsForFailoverRisk is the minimum number of configured retry attempts (or unlimited,
	// represented by MaximumAttempts == 0) that is considered risky when combined with a long ScheduleToStartTimeout.
	minRetryAttemptsForFailoverRisk int32 = 3
)

// ActivityStartToCloseAtWorkflowTimeoutCapMetadata is the metadata for an activity whose StartToCloseTimeout
// sits exactly at the workflow's ExecutionStartToCloseTimeout. The server caps an activity's StartToClose at
// the workflow timeout when validating ActivityTaskScheduled attributes (see validateActivityScheduleAttributes
// in service/history/decision/checker.go), so this equality is the fingerprint of a client-configured value that
// was silently capped -- or, if genuinely configured this way, an activity with zero headroom before the
// workflow itself times out.
type ActivityStartToCloseAtWorkflowTimeoutCapMetadata struct {
	EventID             int64
	ActivityID          string
	ActivityType        string
	StartToCloseTimeout time.Duration
	WorkflowTimeout     time.Duration
}

// ActivityMissingHeartbeatTimeoutMetadata is the metadata for a long-running activity that has no
// HeartbeatTimeout configured, meaning a dead worker would go undetected until the activity times out.
type ActivityMissingHeartbeatTimeoutMetadata struct {
	EventID             int64
	ActivityID          string
	ActivityType        string
	StartToCloseTimeout time.Duration
	Threshold           time.Duration
}

// ActivityLongScheduleToStartWithRetriesMetadata is the metadata for an activity with a long
// ScheduleToStartTimeout combined with a retry policy that allows multiple attempts, which can
// multiply recovery time during a failover or poller outage.
type ActivityLongScheduleToStartWithRetriesMetadata struct {
	EventID                int64
	ActivityID             string
	ActivityType           string
	ScheduleToStartTimeout time.Duration
	Threshold              time.Duration
	RetryPolicy            *types.RetryPolicy
}

// TimeoutRiskIssuesMetadata is a discriminated union of the metadata for each timeout risk check,
// with exactly one field populated per issue.
type TimeoutRiskIssuesMetadata struct {
	ActivityStartToCloseAtWorkflowTimeoutCap *ActivityStartToCloseAtWorkflowTimeoutCapMetadata
	ActivityMissingHeartbeatTimeout          *ActivityMissingHeartbeatTimeoutMetadata
	ActivityLongScheduleToStartWithRetries   *ActivityLongScheduleToStartWithRetriesMetadata
}
