package childworkflowloop

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/simulation/replication/types"
)

type Runner struct {
	ClusterName string
}

func (r *Runner) Workflow(ctx workflow.Context, input types.WorkflowInput) (types.WorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Sugar().Infof("child-workflow-loop started, spawning %d children with prefix %s", input.ChildWorkflowCount, input.ChildWorkflowID)

	if input.DecisionHoldDuration > 0 {
		// a local activity keeps the decision task in flight, the worker heartbeats it until the local activity completes
		logger.Sugar().Infof("holding the decision task for %v before spawning children", input.DecisionHoldDuration)
		laCtx := workflow.WithLocalActivityOptions(ctx, workflow.LocalActivityOptions{
			ScheduleToCloseTimeout: input.DecisionHoldDuration + 10*time.Second,
		})
		if err := workflow.ExecuteLocalActivity(laCtx, r.holdActivity, input.DecisionHoldDuration, input.DecisionHoldCluster).Get(ctx, nil); err != nil {
			logger.Sugar().Errorf("decision hold failed: %v", err)
			return types.WorkflowOutput{}, err
		}
	}

	childWorkflowType := input.ChildWorkflowType
	if childWorkflowType == "" {
		childWorkflowType = "timer-activity-loop-workflow"
	}

	var futures []workflow.ChildWorkflowFuture
	for i := 0; i < input.ChildWorkflowCount; i++ {
		childID := fmt.Sprintf("%s-%d", input.ChildWorkflowID, i)
		cwo := workflow.ChildWorkflowOptions{
			WorkflowID:                   childID,
			ExecutionStartToCloseTimeout: input.ChildWorkflowTimeout,
		}
		childCtx := workflow.WithChildOptions(ctx, cwo)
		childInput := types.WorkflowInput{Duration: input.ChildWorkflowDuration}
		futures = append(futures, workflow.ExecuteChildWorkflow(childCtx, childWorkflowType, childInput))
	}

	for i, f := range futures {
		var output types.WorkflowOutput
		if err := f.Get(ctx, &output); err != nil {
			logger.Sugar().Errorf("child %d failed: %v", i, err)
			return types.WorkflowOutput{}, err
		}
	}

	logger.Sugar().Infof("child-workflow-loop completed, all %d children finished", input.ChildWorkflowCount)
	return types.WorkflowOutput{Count: input.ChildWorkflowCount}, nil
}

// holdActivity holds only on the worker of holdCluster, or on all workers when holdCluster is empty
func (r *Runner) holdActivity(ctx context.Context, duration time.Duration, holdCluster string) error {
	if holdCluster != "" && holdCluster != r.ClusterName {
		return nil
	}
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
