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

package timeractivityloop

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/simulation/replication/types"
)

func Workflow(ctx workflow.Context, input types.WorkflowInput) (types.WorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Sugar().Infof("timer-activity-loop-workflow started with input: %+v", input)

	endTime := workflow.Now(ctx).Add(input.Duration)
	count := 0
	for {
		logger.Sugar().Infof("timer-activity-loop-workflow iteration %d", count)
		selector := workflow.NewSelector(ctx)
		activityFuture := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			TaskList:               types.TasklistName,
			ScheduleToStartTimeout: 10 * time.Second,
			StartToCloseTimeout:    10 * time.Second,
		}), FormatStringActivity, "World")
		selector.AddFuture(activityFuture, func(f workflow.Future) {
			logger.Info("timer-activity-loop-workflow completed activity")
		})

		// use timer future to send notification email if processing takes too long
		timerFuture := workflow.NewTimer(ctx, types.TimerInterval)
		selector.AddFuture(timerFuture, func(f workflow.Future) {
			logger.Info("timer-activity-loop-workflow timer fired")
		})

		// wait for both activity and timer to complete
		selector.Select(ctx)
		selector.Select(ctx)
		count++

		now := workflow.Now(ctx)
		if now.Before(endTime) {
			logger.Sugar().Infof("timer-activity-loop-workflow will continue iteration because [now %v] < [endTime %v]", now, endTime)
		} else {
			logger.Sugar().Infof("timer-activity-loop-workflow will exit because [now %v] >= [endTime %v]", now, endTime)
			break
		}
	}

	logger.Info("timer-activity-loop-workflow completed")
	return types.WorkflowOutput{Count: count}, nil
}

func FormatStringActivity(ctx context.Context, input string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("timer-activity-loop-workflow format-string-activity started")
	return fmt.Sprintf("Hello, %s!", input), nil
}
