package retryactivitystress

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/cadence"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/simulation/replication/types"
)

// failUntilAttempt is the first activity attempt that succeeds; attempts 0..failUntilAttempt-1
// fail with a retriable error after running for attemptDuration. Together with the 8s flat
// retry backoff this keeps every activity in a continuous fail-retry cycle until ~t+150, so
// that every domain failover in the scenario (t+30/60/90/120) lands while retries are pending
// or attempts are in flight. The short attempt relative to the backoff (~1s vs 8s) maximizes
// the probability that any given failover catches a chain mid-backoff — the state in which a
// failover strands the retry (the retry timer lives only in the cluster that recorded the
// failure and is silently dropped there once that cluster is standby).
const (
	failUntilAttempt = 15
	attemptDuration  = time.Second
)

// Workflow runs input.ActivityCount copies of FailUntilAttemptActivity in parallel and
// completes when all of them have succeeded. A single activity retry lost across a failover
// strands that activity in SCHEDULED state (its ScheduleToStart timeout is capped at the
// workflow timeout, far beyond the simulation's assertion window), so the workflow cannot
// complete — which is what the scenario's final validations detect.
func Workflow(ctx workflow.Context, input types.WorkflowInput) (types.WorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Sugar().Infof("activity-retry-stress-workflow started with input: %+v", input)

	activityCount := input.ActivityCount
	if activityCount <= 0 {
		activityCount = 1
	}

	aCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskList:               types.TasklistName,
		ScheduleToStartTimeout: 10 * time.Minute,
		// Short StartToClose (attempts run ~1s) so that attempts interrupted by a
		// failover are recovered quickly by the new active cluster's timeout timer,
		// keeping the post-fix (green) completion time well inside the scenario's
		// assertion window.
		StartToCloseTimeout:    15 * time.Second,
		ScheduleToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &cadence.RetryPolicy{
			InitialInterval:    8 * time.Second,
			BackoffCoefficient: 1.0,
			MaximumInterval:    8 * time.Second,
			ExpirationInterval: 15 * time.Minute,
		},
	})

	futures := make([]workflow.Future, 0, activityCount)
	for i := 0; i < activityCount; i++ {
		futures = append(futures, workflow.ExecuteActivity(aCtx, FailUntilAttemptActivity, fmt.Sprintf("input-%d", i)))
	}

	completed := 0
	for i, f := range futures {
		var result string
		if err := f.Get(ctx, &result); err != nil {
			logger.Sugar().Errorf("activity-retry-stress-workflow activity %d failed: %v", i, err)
			return types.WorkflowOutput{Count: completed}, err
		}
		completed++
	}

	logger.Sugar().Infof("activity-retry-stress-workflow completed all %d activities", completed)
	return types.WorkflowOutput{Count: completed}, nil
}

// FailUntilAttemptActivity runs for attemptDuration and then fails with a retriable error
// on attempts 0..failUntilAttempt-1; it succeeds at attempt failUntilAttempt or later. The
// per-attempt run time keeps attempts in flight across failovers, so their
// RespondActivityTaskFailed calls land in the just-demoted cluster — the domain-cache
// staleness race surface — and it also lets the standby cluster observe STARTED states via
// SyncActivity and ack its activity transfer tasks (see the activityretryfailover scenario
// for why that matters).
func FailUntilAttemptActivity(ctx context.Context, input string) (string, error) {
	logger := activity.GetLogger(ctx)
	attempt := activity.GetInfo(ctx).Attempt
	time.Sleep(attemptDuration)
	if attempt < failUntilAttempt {
		logger.Sugar().Infof("fail-until-attempt-activity %s failing attempt %d with retriable error", input, attempt)
		return "", fmt.Errorf("retriable failure on attempt %d", attempt)
	}
	logger.Sugar().Infof("fail-until-attempt-activity %s succeeding on attempt %d", input, attempt)
	return fmt.Sprintf("Hello, %s!", input), nil
}
