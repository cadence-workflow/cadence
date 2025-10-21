package main

import (
	"context"
	"time"

	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"
)

// longRunningWorkflow demonstrates a workflow that stays open for a while
func longRunningWorkflow(ctx workflow.Context, name string) (*string, error) {
	ao := workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute,
		StartToCloseTimeout:    time.Minute,
		HeartbeatTimeout:       time.Second * 20,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	logger := workflow.GetLogger(ctx)
	logger.Info("Long-running workflow started - will sleep for 2 minutes")

	// Execute the hello activity
	var helloworldResult string
	err := workflow.ExecuteActivity(ctx, helloWorldActivity, name).Get(ctx, &helloworldResult)
	if err != nil {
		logger.Error("Activity failed.", zap.Error(err))
		return nil, err
	}

	logger.Info("Activity completed, now sleeping for 2 minutes...")
	
	// Sleep for 2 minutes so you can see it in the UI
	err = workflow.Sleep(ctx, 2*time.Minute)
	if err != nil {
		logger.Error("Sleep failed.", zap.Error(err))
		return nil, err
	}

	logger.Info("Workflow completed after sleep.", zap.String("Result", helloworldResult))

	return &helloworldResult, nil
}

// longRunningActivity demonstrates an activity that takes some time
func longRunningActivity(ctx context.Context, seconds int) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Long-running activity started", zap.Int("duration_seconds", seconds))
	
	// Sleep for the specified duration
	time.Sleep(time.Duration(seconds) * time.Second)
	
	logger.Info("Long-running activity completed")
	return "Activity completed successfully!", nil
}

