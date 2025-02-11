// Copyright (c) 2018 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/uber/cadence/simulation/replication/types"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"
)

func TestWorkflow(ctx workflow.Context, input types.WorkflowInput) (types.WorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Sugar().Infof("testWorkflow started with input: %+v", input)

	endTime := workflow.Now(ctx).Add(input.Duration)
	count := 0
	for {
		logger.Sugar().Infof("testWorkflow iteration %d", count)
		selector := workflow.NewSelector(ctx)
		activityFuture := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			TaskList:               types.TasklistName,
			ScheduleToStartTimeout: 10 * time.Second,
			StartToCloseTimeout:    10 * time.Second,
		}), types.ActivityName, "World")
		selector.AddFuture(activityFuture, func(f workflow.Future) {
			logger.Info("testWorkflow completed activity")
		})

		// use timer future to send notification email if processing takes too long
		timerFuture := workflow.NewTimer(ctx, types.TimerInterval)
		selector.AddFuture(timerFuture, func(f workflow.Future) {
			logger.Info("testWorkflow timer fired")
		})

		// wait for both activity and timer to complete
		selector.Select(ctx)
		selector.Select(ctx)
		count++

		now := workflow.Now(ctx)
		if now.Before(endTime) {
			logger.Sugar().Infof("testWorkflow will continue iteration because [now %v] < [endTime %v]", now, endTime)
		} else {
			logger.Sugar().Infof("testWorkflow will exit because [now %v] >= [endTime %v]", now, endTime)
			break
		}
	}

	logger.Info("testWorkflow completed")
	return types.WorkflowOutput{Count: count}, nil
}

func TestActivity(ctx context.Context, input string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("testActivity started")
	return fmt.Sprintf("Hello, %s!", input), nil
}
