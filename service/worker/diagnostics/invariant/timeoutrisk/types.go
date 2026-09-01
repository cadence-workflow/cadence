package timeoutrisk

import (
	"time"
)

type TimeoutRiskType string

const (
	ActivityStartToCloseAtWorkflowTimeoutCap TimeoutRiskType = "Activity StartToClose timeout at the workflow execution timeout cap"
	ActivityMissingHeartbeatTimeout          TimeoutRiskType = "Long-running activity missing heartbeat timeout"
)

func (t TimeoutRiskType) String() string {
	return string(t)
}

type IssueType string

const (
	StartToCloseAtWorkflowTimeoutCap       IssueType = "Activity StartToClose timeout may have been silently capped at the workflow execution timeout by the server, leaving no headroom for retrying."
	MissingHeartbeatTimeoutForLongActivity IssueType = "Long-running activity without a HeartbeatTimeout will leave worker failures undetected until the activity times out."
)

func (i IssueType) String() string {
	return string(i)
}

const (
	// longRunningActivityThresholdSeconds is the StartToCloseTimeout above which an activity is
	// considered long-running and should be configured with a HeartbeatTimeout.
	longRunningActivityThresholdSeconds int32 = 10 * 60
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

// TimeoutRiskIssuesMetadata is a discriminated union of the metadata for each timeout risk check,
// with exactly one field populated per issue.
type TimeoutRiskIssuesMetadata struct {
	ActivityStartToCloseAtWorkflowTimeoutCap *ActivityStartToCloseAtWorkflowTimeoutCapMetadata
	ActivityMissingHeartbeatTimeout          *ActivityMissingHeartbeatTimeoutMetadata
}
