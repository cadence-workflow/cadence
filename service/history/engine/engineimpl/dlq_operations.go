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
)

func (e *historyEngineImpl) CountDLQMessages(ctx context.Context, forceFetch bool) (map[string]int64, error) {
	return e.replicationDLQHandler.GetMessageCount(ctx, forceFetch)
}

func (e *historyEngineImpl) ReadDLQMessages(
	ctx context.Context,
	request *types.ReadDLQMessagesRequest,
) (*types.ReadDLQMessagesResponse, error) {

	tasks, taskInfo, token, err := e.replicationDLQHandler.ReadMessages(
		ctx,
		request.GetSourceCluster(),
		request.GetInclusiveEndMessageID(),
		int(request.GetMaximumPageSize()),
		request.GetNextPageToken(),
	)
	if err != nil {
		return nil, err
	}

	// ReadMessages returns nil entries for tasks that could not be hydrated (e.g. the
	// source workflow was deleted). A nil *ReplicationTask cannot be serialized into the
	// response: the generated protobuf MarshalToSizedBuffer dereferences each repeated
	// element without a nil check, so a nil entry panics the history service while
	// marshaling the RPC response. Callers correlate tasks with their metadata by
	// SourceTaskID and the full set is still represented in ReplicationTasksInfo, so
	// dropping the unhydrated tasks here is safe.
	hydratedTasks := make([]*types.ReplicationTask, 0, len(tasks))
	for _, task := range tasks {
		if task != nil {
			hydratedTasks = append(hydratedTasks, task)
		}
	}

	return &types.ReadDLQMessagesResponse{
		Type:                 request.GetType().Ptr(),
		ReplicationTasks:     hydratedTasks,
		ReplicationTasksInfo: taskInfo,
		NextPageToken:        token,
	}, nil
}

func (e *historyEngineImpl) PurgeDLQMessages(
	ctx context.Context,
	request *types.PurgeDLQMessagesRequest,
) error {

	return e.replicationDLQHandler.PurgeMessages(
		ctx,
		request.GetSourceCluster(),
		request.GetInclusiveEndMessageID(),
	)
}

func (e *historyEngineImpl) MergeDLQMessages(
	ctx context.Context,
	request *types.MergeDLQMessagesRequest,
) (*types.MergeDLQMessagesResponse, error) {

	token, err := e.replicationDLQHandler.MergeMessages(
		ctx,
		request.GetSourceCluster(),
		request.GetInclusiveEndMessageID(),
		int(request.GetMaximumPageSize()),
		request.GetNextPageToken(),
	)
	if err != nil {
		return nil, err
	}
	return &types.MergeDLQMessagesResponse{
		NextPageToken: token,
	}, nil
}
