package main

import (
	"context"
	"time"

	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"
)

// ActivityID is the unique identifier for our hello world activity
const ActivityID = "hello-world-activity"

// SayHello is the implementation for our hello world activity
func SayHello(ctx context.Context) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Activity executing", zap.String("activity", ActivityID))
	return "Hello World!", nil
}

// HelloWorldWorkflow is the implementation for our hello world workflow
func HelloWorldWorkflow(ctx workflow.Context) (string, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Workflow executing")

	var result string
	err := workflow.ExecuteActivity(ctx, SayHello).Get(ctx, &result)
	if err != nil {
		logger.Error("Activity failed.", zap.Error(err))
		return "", err
	}

	logger.Info("Workflow completed.", zap.String("result", result))
	return result, nil
}

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Initialize worker
	worker := worker.New(
		"localhost:7933",
		"samples-domain",
		"hello-world-taskList",
		worker.Options{
			Logger: logger,
		},
	)

	// Register workflow and activity
	worker.RegisterWorkflow(HelloWorldWorkflow)
	worker.RegisterActivity(SayHello)

	// Start listening to the task list
	err := worker.Run()
	if err != nil {
		logger.Fatal("Unable to start worker", zap.Error(err))
	}
}