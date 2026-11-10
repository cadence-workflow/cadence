package retryactivityfailover

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/cadence"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/simulation/replication/types"
)

// Workflow executes a single activity with a retry policy. The activity fails
// with a retriable error on its first attempt (attempt 0) and succeeds on any
// subsequent attempt, so exactly one retry backoff (InitialInterval) separates
// the failure from the expected redispatch. The retry policy expiration and the
// activity timeouts are deliberately much longer than the simulation window so
// that no timeout can rescue a lost retry within the test's runtime.
func Workflow(ctx workflow.Context, input types.WorkflowInput) (types.WorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Sugar().Infof("activity-retry-failover-workflow started with input: %+v", input)

	aCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskList:               types.TasklistName,
		ScheduleToStartTimeout: 10 * time.Minute,
		StartToCloseTimeout:    90 * time.Second,
		ScheduleToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &cadence.RetryPolicy{
			InitialInterval:    45 * time.Second,
			BackoffCoefficient: 1.0,
			MaximumInterval:    45 * time.Second,
			ExpirationInterval: 15 * time.Minute,
		},
	})

	var result string
	if err := workflow.ExecuteActivity(aCtx, FailOnFirstAttemptActivity, "World").Get(ctx, &result); err != nil {
		logger.Sugar().Errorf("activity-retry-failover-workflow activity failed: %v", err)
		return types.WorkflowOutput{}, err
	}

	logger.Sugar().Infof("activity-retry-failover-workflow completed with result: %s", result)
	return types.WorkflowOutput{Count: 1}, nil
}

// FailOnFirstAttemptActivity runs for activeDuration and then fails with a retriable
// error on attempt 0; it succeeds immediately on any later attempt.
//
// The attempt-0 run time is load-bearing for the activityretryfailover scenario: it keeps
// the activity in STARTED state long enough for the standby cluster to observe the started
// state via SyncActivity and ack its pending ActivityTaskScheduled transfer task. The
// standby executor re-evaluates a pending task on a hardcoded ~30s backoff
// (standbyTaskRedispatchInitialInterval in service/history/task/task.go), so the window
// must comfortably cover one such re-evaluation. If the transfer task were still pending
// (activity never observed as started) at failover time, the queue-processor restart on
// failover would replay it as active and re-dispatch the activity, masking the
// lost-retry-timer bug under test.
func FailOnFirstAttemptActivity(ctx context.Context, input string) (string, error) {
	const activeDuration = 60 * time.Second

	logger := activity.GetLogger(ctx)
	attempt := activity.GetInfo(ctx).Attempt
	if attempt == 0 {
		logger.Sugar().Infof("fail-on-first-attempt-activity attempt %d running for %v before failing with retriable error", attempt, activeDuration)
		time.Sleep(activeDuration)
		return "", fmt.Errorf("retriable failure on attempt %d", attempt)
	}
	logger.Sugar().Infof("fail-on-first-attempt-activity succeeding on attempt %d", attempt)
	return fmt.Sprintf("Hello, %s!", input), nil
}
