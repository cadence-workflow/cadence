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
	"github.com/uber/cadence/service/history/queue"
)

func (e *historyEngineImpl) ResetTransferQueue(
	ctx context.Context,
	clusterName string,
) error {
	transferProcessor, ok := e.queueProcessors[persistence.HistoryTaskCategoryTransfer]
	if !ok {
		return fmt.Errorf("transfer processor not found")
	}
	_, err := transferProcessor.HandleAction(ctx, clusterName, queue.NewResetAction())
	return err
}

func (e *historyEngineImpl) ResetTimerQueue(
	ctx context.Context,
	clusterName string,
) error {
	timerProcessor, ok := e.queueProcessors[persistence.HistoryTaskCategoryTimer]
	if !ok {
		return fmt.Errorf("timer processor not found")
	}
	_, err := timerProcessor.HandleAction(ctx, clusterName, queue.NewResetAction())
	return err
}
