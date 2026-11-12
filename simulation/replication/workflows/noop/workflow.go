package noop

import (
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/simulation/replication/types"
)

// Workflow completes in its first decision task. It is used as a child that closes soon after it starts.
func Workflow(ctx workflow.Context, input types.WorkflowInput) (types.WorkflowOutput, error) {
	workflow.GetLogger(ctx).Info("noop-workflow completed")
	return types.WorkflowOutput{}, nil
}
