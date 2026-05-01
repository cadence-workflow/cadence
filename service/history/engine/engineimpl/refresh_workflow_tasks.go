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

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
)

func (e *historyEngineImpl) RefreshWorkflowTasks(
	ctx context.Context,
	domainUUID string,
	workflowExecution types.WorkflowExecution,
) (retError error) {
	domainEntry, err := e.shard.GetDomainCache().GetDomainByID(domainUUID)
	if err != nil {
		return err
	}
	domainID := domainEntry.GetInfo().ID

	wfContext, release, err := e.executionCache.GetOrCreateWorkflowExecution(ctx, domainID, workflowExecution)
	if err != nil {
		return err
	}
	defer func() { release(retError) }()

	mutableState, err := wfContext.LoadWorkflowExecution(ctx)
	if err != nil {
		return err
	}

	mutableStateTaskRefresher := execution.NewMutableStateTaskRefresher(
		e.shard.GetConfig(),
		e.shard.GetClusterMetadata(),
		e.shard.GetDomainCache(),
		e.shard.GetEventsCache(),
		e.shard.GetShardID(),
		e.logger,
	)

	err = mutableStateTaskRefresher.RefreshTasks(ctx, mutableState.GetExecutionInfo().StartTimestamp, mutableState)
	if err != nil {
		return err
	}

	err = wfContext.UpdateWorkflowExecutionTasks(ctx, e.shard.GetTimeSource().Now())
	if err != nil {
		return err
	}
	return nil
}
