package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"

	"github.com/uber/cadence/common/types"
)

func TestEmitWorkflowAgeDays(t *testing.T) {
	tests := map[string]struct {
		closeStatus    types.WorkflowExecutionCloseStatus
		age            time.Duration
		expectedStatus string
		expectedBucket float64
	}{
		"completed is success":        {types.WorkflowExecutionCloseStatusCompleted, 71 * time.Hour, "success", 3},
		"continued as new is success": {types.WorkflowExecutionCloseStatusContinuedAsNew, 23 * time.Hour, "success", 1},
		"failed is failure":           {types.WorkflowExecutionCloseStatusFailed, 23 * time.Hour, "failure", 1},
		"timed out is failure":        {types.WorkflowExecutionCloseStatusTimedOut, 1 * time.Hour, "failure", 1},
		"canceled is failure":         {types.WorkflowExecutionCloseStatusCanceled, 1 * time.Hour, "failure", 1},
		"terminated is failure":       {types.WorkflowExecutionCloseStatusTerminated, 1 * time.Hour, "failure", 1},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			scope := tally.NewTestScope("", nil)
			client := NewClient(scope, History, MigrationConfig{})

			EmitWorkflowAgeDays(client, HistoryQueryWorkflowScope, "test-domain", time.Now().Add(-td.age).UnixNano(), td.closeStatus)

			var found bool
			for _, h := range scope.Snapshot().Histograms() {
				if h.Name() != "workflow_query_age_days" {
					continue
				}
				found = true
				assert.Equal(t, td.expectedStatus, h.Tags()["workflow_close_status"])
				assert.Equal(t, "test-domain", h.Tags()["domain"])
				assert.Equal(t, "QueryWorkflow", h.Tags()["operation"])
				assert.Equal(t, int64(1), h.Values()[td.expectedBucket])
			}
			assert.True(t, found)
		})
	}
}
