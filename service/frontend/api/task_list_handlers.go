// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package api

import (
	"context"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/validate"
)

// DescribeTaskList returns information about the target tasklist, right now this API returns the
// pollers which polled this tasklist in last few minutes. If includeTaskListStatus field is true,
// it will also return status of tasklist's ackManager (readLevel, ackLevel, backlogCountHint and taskIDBlock).
func (wh *WorkflowHandler) DescribeTaskList(
	ctx context.Context,
	request *types.DescribeTaskListRequest,
) (resp *types.DescribeTaskListResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateDescribeTaskListRequest(ctx, request); err != nil {
		return nil, err
	}
	domainID, err := wh.GetDomainCache().GetDomainID(request.GetDomain())
	if err != nil {
		return nil, err
	}
	response, err := wh.GetMatchingClient().DescribeTaskList(ctx, &types.MatchingDescribeTaskListRequest{
		DomainUUID:  domainID,
		DescRequest: request,
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

// ListTaskListPartitions returns all the partition and host for a taskList
func (wh *WorkflowHandler) ListTaskListPartitions(
	ctx context.Context,
	request *types.ListTaskListPartitionsRequest,
) (resp *types.ListTaskListPartitionsResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateListTaskListPartitionsRequest(ctx, request); err != nil {
		return nil, err
	}
	resp, err := wh.GetMatchingClient().ListTaskListPartitions(ctx, &types.MatchingListTaskListPartitionsRequest{
		Domain:   request.Domain,
		TaskList: request.TaskList,
	})
	return resp, err
}

// GetTaskListsByDomain returns all the partition and host for a taskList
func (wh *WorkflowHandler) GetTaskListsByDomain(
	ctx context.Context,
	request *types.GetTaskListsByDomainRequest,
) (resp *types.GetTaskListsByDomainResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateGetTaskListsByDomainRequest(ctx, request); err != nil {
		return nil, err
	}
	resp, err := wh.GetMatchingClient().GetTaskListsByDomain(ctx, &types.GetTaskListsByDomainRequest{
		Domain: request.Domain,
	})
	return resp, err
}

// ResetStickyTaskList reset the volatile information in mutable state of a given workflow.
func (wh *WorkflowHandler) ResetStickyTaskList(
	ctx context.Context,
	resetRequest *types.ResetStickyTaskListRequest,
) (resp *types.ResetStickyTaskListResponse, retError error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateResetStickyTaskListRequest(ctx, resetRequest); err != nil {
		return nil, err
	}
	domainID, err := wh.GetDomainCache().GetDomainID(resetRequest.GetDomain())
	if err != nil {
		return nil, err
	}
	_, err = wh.GetHistoryClient().ResetStickyTaskList(ctx, &types.HistoryResetStickyTaskListRequest{
		DomainUUID: domainID,
		Execution:  resetRequest.Execution,
	})
	if err != nil {
		return nil, wh.normalizeVersionedErrors(ctx, err)
	}
	return &types.ResetStickyTaskListResponse{}, nil
}
