package workflows

import (
	"github.com/uber/cadence/simulation/replication/workflows/activityloop"
	"github.com/uber/cadence/simulation/replication/workflows/childactivityloop"
	"github.com/uber/cadence/simulation/replication/workflows/query"
	"github.com/uber/cadence/simulation/replication/workflows/timeractivityloop"
)

// Add workflows and activities to this map to register them with the worker.
var (
	Workflows = func(clusterName string) map[string]any {
		queryWFRunner := &query.Runner{ClusterName: clusterName}
		return map[string]any{
			"timer-activity-loop-workflow": timeractivityloop.Workflow,
			"activity-loop-workflow":       activityloop.Workflow,
			"query-workflow":               queryWFRunner.Workflow,
			"child-activity-loop-workflow": childactivityloop.Workflow,
		}
	}
	Activities = map[string]any{
		"timer-activity-loop-format-string-activity": timeractivityloop.FormatStringActivity,
		"activity-loop-format-string-activity":       activityloop.FormatStringActivity,
	}
)
