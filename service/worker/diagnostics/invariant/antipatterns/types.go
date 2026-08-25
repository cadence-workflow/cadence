package antipatterns

import (
	"time"
)

type AntipatternType string

const (
	ActivityScheduleBurst       AntipatternType = "Activities scheduled in quick succession"
	ContinueAsNewInCronWorkflow AntipatternType = "Continue-As-New used in cron workflow"
)

func (a AntipatternType) String() string {
	return string(a)
}

type IssueType string

const (
	ActivityScheduleBurstDetected                 IssueType = "A burst of activities was scheduled within a short time window, which can be an early-warning signal of hot-shard contention."
	ContinueAsNewInitiatedByDeciderInCronWorkflow IssueType = "The workflow continued-as-new from workflow code while running under a cron schedule, which can interfere with server-managed cron scheduling."
)

func (i IssueType) String() string {
	return string(i)
}

const (
	// activityBurstWindowInSeconds is the width of the sliding window used to look for a burst of
	// scheduled activities.
	activityBurstWindowInSeconds int64 = 10

	// activityBurstCountThreshold is the minimum number of ActivityTaskScheduled events within
	// activityBurstWindowInSeconds for a burst to be flagged as a hot-shard risk.
	activityBurstCountThreshold = 50
)

// ActivityScheduleBurstMetadata describes the densest window of scheduled activities found in the
// history. Only the peak window is reported, not every window that crossed the threshold.
// FirstEventID anchors the issue to the first ActivityTaskScheduled event of that window.
type ActivityScheduleBurstMetadata struct {
	FirstEventID    int64
	LastEventID     int64
	EventCount      int
	WindowStart     time.Time
	WindowEnd       time.Time
	WindowInSeconds int64
	Threshold       int
}

// ContinueAsNewInCronWorkflowMetadata identifies the started event carrying the cron schedule and
// the continue-as-new event that was initiated by workflow code.
type ContinueAsNewInCronWorkflowMetadata struct {
	StartedEventID        int64
	CronSchedule          string
	ContinuedAsNewEventID int64
}

// AntipatternIssuesMetadata is a discriminated union of the metadata for each antipattern check,
// with exactly one field populated per issue.
type AntipatternIssuesMetadata struct {
	ActivityScheduleBurst       *ActivityScheduleBurstMetadata
	ContinueAsNewInCronWorkflow *ContinueAsNewInCronWorkflowMetadata
}
