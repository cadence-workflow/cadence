package metrics

import (
	"time"

	"github.com/uber/cadence/common/types"
)

// EmitWorkflowAgeDays records the age in days of a closed workflow that was just read, into WorkflowQueryAgeDays.
func EmitWorkflowAgeDays(client Client, scopeIdx ScopeIdx, domainName string, closeTimeNanos int64, closeStatus types.WorkflowExecutionCloseStatus) {
	ageDays := time.Since(time.Unix(0, closeTimeNanos)).Hours() / 24

	// Collapse close statuses to "success" or "failure" to limit tag cardinality.
	statusTag := "failure"
	switch closeStatus {
	case types.WorkflowExecutionCloseStatusCompleted, types.WorkflowExecutionCloseStatusContinuedAsNew:
		statusTag = "success"
	}

	client.Scope(scopeIdx, DomainTag(domainName), WorkflowCloseStatusTag(statusTag)).RecordHistogramValue(WorkflowQueryAgeDays, ageDays)
}
