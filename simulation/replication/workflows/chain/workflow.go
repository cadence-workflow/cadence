package chain

import (
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/simulation/replication/types"
)

// Workflow is a chain of runs. Each run sleeps for the first of RunDurations, then continues as new with the
// remaining durations. The last run completes.
func Workflow(ctx workflow.Context, input types.WorkflowInput) (types.WorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	if len(input.RunDurations) == 0 {
		logger.Info("chain-workflow completed")
		return types.WorkflowOutput{}, nil
	}

	if err := workflow.Sleep(ctx, input.RunDurations[0]); err != nil {
		return types.WorkflowOutput{}, err
	}

	remaining := input.RunDurations[1:]
	if len(remaining) == 0 {
		logger.Info("chain-workflow completed")
		return types.WorkflowOutput{}, nil
	}
	logger.Sugar().Infof("chain-workflow continues as new, %d runs remain", len(remaining))
	return types.WorkflowOutput{}, workflow.NewContinueAsNewError(ctx, "chain-workflow", types.WorkflowInput{RunDurations: remaining})
}
