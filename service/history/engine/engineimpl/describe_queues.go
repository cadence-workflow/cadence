// Copyright (c) 2017-2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2021 Temporal Technologies Inc.
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

package engineimpl

import (
	"context"
	"fmt"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/queue"
)

func (e *historyEngineImpl) DescribeTransferQueue(
	ctx context.Context,
	clusterName string,
) (*types.DescribeQueueResponse, error) {
	return e.describeQueue(ctx, persistence.HistoryTaskCategoryTransfer, clusterName)
}

func (e *historyEngineImpl) DescribeTimerQueue(
	ctx context.Context,
	clusterName string,
) (*types.DescribeQueueResponse, error) {
	return e.describeQueue(ctx, persistence.HistoryTaskCategoryTimer, clusterName)
}

func (e *historyEngineImpl) describeQueue(
	ctx context.Context,
	category persistence.HistoryTaskCategory,
	clusterName string,
) (*types.DescribeQueueResponse, error) {
	queueProcessor, ok := e.queueProcessors[category]
	if !ok {
		return nil, fmt.Errorf("queue processor not found for category %v", category)
	}
	resp, err := queueProcessor.HandleAction(ctx, clusterName, queue.NewGetStateAction())
	if err != nil {
		return nil, err
	}

	serializedStates := make([]string, 0, len(resp.GetStateActionResult.States))
	for _, state := range resp.GetStateActionResult.States {
		serializedStates = append(serializedStates, e.serializeQueueState(state))
	}
	return &types.DescribeQueueResponse{
		ProcessingQueueStates: serializedStates,
	}, nil
}

func (e *historyEngineImpl) serializeQueueState(
	state queue.ProcessingQueueState,
) string {
	return fmt.Sprintf("%v", state)
}
