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

package query

import (
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/simulation/replication/types"
)

const (
	signalName       = "custom-signal"
	clusterNameQuery = "cluster-name"
	signalCountQuery = "signal-count"
)

type Runner struct {
	ClusterName string
}

func (r *Runner) Workflow(ctx workflow.Context, input types.WorkflowInput) (types.WorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Sugar().Infof("query workflow started with input: %+v", input)

	err := workflow.SetQueryHandler(ctx, clusterNameQuery, func() (string, error) {
		logger.Sugar().Infof("query handler called. returning cluster name: %s", r.ClusterName)
		return r.ClusterName, nil
	})
	if err != nil {
		logger.Sugar().Errorf("failed to set query handler: %v", err)
		return types.WorkflowOutput{}, err
	}

	signalCount := 0
	err = workflow.SetQueryHandler(ctx, signalCountQuery, func() (int, error) {
		logger.Sugar().Infof("query handler called. returning signal count: %d", signalCount)
		return signalCount, nil
	})
	if err != nil {
		logger.Sugar().Errorf("failed to set query handler: %v", err)
		return types.WorkflowOutput{}, err
	}

	endTime := workflow.Now(ctx).Add(input.Duration)
	signalCh := workflow.GetSignalChannel(ctx, signalName)
	done := false
	for {
		selector := workflow.NewSelector(ctx)

		// timer
		timerCtx, timerCancel := workflow.WithCancel(ctx)
		waitTimer := workflow.NewTimer(timerCtx, endTime.Sub(workflow.Now(ctx)))
		selector.AddFuture(waitTimer, func(f workflow.Future) {
			done = true
		})

		// signal
		selector.AddReceive(signalCh, func(c workflow.Channel, more bool) {
			var signal string
			for c.ReceiveAsync(&signal) {
				signalCount++
				logger.Sugar().Infof("signal received: %s", signal)
			}
		})

		selector.Select(ctx)
		timerCancel()
		if done {
			break
		}
	}

	logger.Info("query workflow completed")
	return types.WorkflowOutput{}, nil
}
