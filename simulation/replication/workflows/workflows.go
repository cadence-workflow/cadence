package workflows

import (
	activityloop "github.com/uber/cadence/simulation/replication/workflows/activity_loop"
	timeractivityloop "github.com/uber/cadence/simulation/replication/workflows/timer_activity_loop"
)

// Add workflows and activities to this map to register them with the worker.
var (
	Workflows = map[string]any{
		"timer-activity-loop-workflow": timeractivityloop.Workflow,
		"activity-loop-workflow":       activityloop.Workflow,
	}
	Activities = map[string]any{
		"timer-activity-loop-format-string-activity": timeractivityloop.FormatStringActivity,
		"activity-loop-format-string-activity":       activityloop.FormatStringActivity,
	}
)
