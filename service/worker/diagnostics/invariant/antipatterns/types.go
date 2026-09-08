package antipatterns

import (
	"fmt"
	"time"
)

type AntipatternType int32

const (
	AntipatternTypeInvalid AntipatternType = iota
	ActivityScheduleBurst
	ContinueAsNewInCronWorkflow
)

func (a AntipatternType) String() string {
	switch a {
	case ActivityScheduleBurst:
		return "Activities scheduled in quick succession"
	case ContinueAsNewInCronWorkflow:
		return "Continue-As-New used in cron workflow"
	}
	return fmt.Sprintf("AntipatternType(%d)", int32(a))
}

type IssueType int32

const (
	IssueTypeInvalid IssueType = iota
	ActivityScheduleBurstDetected
	ContinueAsNewInitiatedByDeciderInCronWorkflow
)

func (i IssueType) String() string {
	switch i {
	case ActivityScheduleBurstDetected:
		return "A burst of activities was scheduled within a short time window, which can cause hot-shard contention."
	case ContinueAsNewInitiatedByDeciderInCronWorkflow:
		return "The workflow continued-as-new from workflow code while running under a cron schedule, which can interfere with server-managed cron scheduling."
	}
	return fmt.Sprintf("IssueType(%d)", int32(i))
}

const (
	// activityBurstWindowInSeconds is the width of the sliding window used to look for a burst of
	// scheduled activities.
	activityBurstWindowInSeconds int64 = 10

	// activityBurstCountThreshold is the minimum number of ActivityTaskScheduled events within
	// activityBurstWindowInSeconds for a burst to be flagged as a hot-shard risk.
	activityBurstCountThreshold = 50
)

// ActivityScheduleBurstMetadata describes one cluster of tightly-packed scheduled activities.
// Every cluster that crosses the threshold is reported as its own issue, and a single sustained
// burst spanning more than WindowInSeconds is still reported as one span rather than being split
// at a window boundary. The issue's EventID and LastEventID bound the full cluster, not just the
// densest sub-window within it.
type ActivityScheduleBurstMetadata struct {
	LastEventID     int64
	EventCount      int
	WindowStart     time.Time
	WindowEnd       time.Time
	WindowInSeconds int64
	Threshold       int
}

// ContinueAsNewInCronWorkflowMetadata identifies the started event carrying the cron schedule.
type ContinueAsNewInCronWorkflowMetadata struct {
	StartedEventID int64
	CronSchedule   string
}

// AntipatternIssuesMetadata is the metadata for every antipattern issue. EventID is the event the
// issue anchors on. Exactly one of the per-check fields is set, holding the details specific to that check.
type AntipatternIssuesMetadata struct {
	EventID int64

	ActivityScheduleBurst       *ActivityScheduleBurstMetadata       `json:",omitempty"`
	ContinueAsNewInCronWorkflow *ContinueAsNewInCronWorkflowMetadata `json:",omitempty"`
}
