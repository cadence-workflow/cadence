package childactivityloop

import (
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/simulation/replication/types"
)

func Workflow(ctx workflow.Context, input types.WorkflowInput) (types.WorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Sugar().Infof("child-activity-loop-workflow started with input: %+v", input)

	cwo := workflow.ChildWorkflowOptions{
		WorkflowID:                   input.ChildWorkflowID,
		ExecutionStartToCloseTimeout: input.ChildWorkflowTimeout,
	}
	ctx = workflow.WithChildOptions(ctx, cwo)
	var output types.WorkflowOutput
	err := workflow.ExecuteChildWorkflow(ctx, "timer-activity-loop-workflow", input).Get(ctx, &output)
	if err != nil {
		logger.Sugar().Errorf("failed to execute child workflow: %v", err)
		return types.WorkflowOutput{}, err
	}

	return output, nil
}
