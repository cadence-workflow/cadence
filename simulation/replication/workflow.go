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

package replication

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/workflow"
)

type WorkflowInput struct {
	Duration time.Duration
}

type WorkflowOutput struct {
	Count int
}

func testWorkflow(ctx workflow.Context, input WorkflowInput) (WorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("testWorkflow started")

	endTime := workflow.Now(ctx).Add(input.Duration)
	count := 0
	for workflow.Now(ctx).Before(endTime) {
		selector := workflow.NewSelector(ctx)
		activityFuture := workflow.ExecuteActivity(ctx, activityName, "World")
		selector.AddFuture(activityFuture, func(f workflow.Future) {
			logger.Info("activity completed")
		})

		// use timer future to send notification email if processing takes too long
		timerFuture := workflow.NewTimer(ctx, timerInterval)
		selector.AddFuture(timerFuture, func(f workflow.Future) {
			logger.Info("timer fired")
		})

		// wait for both activity and timer to complete
		selector.Select(ctx)
		selector.Select(ctx)
		count++
	}

	logger.Info("testWorkflow completed")
	return WorkflowOutput{Count: count}, nil
}

func testActivity(ctx context.Context, input string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("testActivity started")
	return fmt.Sprintf("Hello, %s!", input), nil
}
