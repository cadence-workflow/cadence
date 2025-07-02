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

package activityloop

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
	logger.Sugar().Infof("single-serial-activity-workflow started with input: %+v", input)

	count := 0

	for count < input.ActivityCount {
		logger.Sugar().Infof("single-serial-activity-workflow iteration %d", count)
		selector := workflow.NewSelector(ctx)
		activityFuture := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			TaskList:               types.TasklistName,
			ScheduleToStartTimeout: 10 * time.Second,
			StartToCloseTimeout:    10 * time.Second,
		}), FormatStringActivity, "World")
		selector.AddFuture(activityFuture, func(f workflow.Future) {
			logger.Info("single-serial-activity-workflow completed activity")
		})

		selector.Select(ctx)
		count++
	}

	logger.Info("single-serial-activity-workflow completed")
	return types.WorkflowOutput{Count: count}, nil
}

func FormatStringActivity(ctx context.Context, input string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("single-serial-activity-workflow format-string-activity started")

	time.Sleep(3 * time.Second)

	return fmt.Sprintf("Hello, %s!", input), nil
}
