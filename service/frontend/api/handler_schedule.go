// Copyright (c) 2026 Uber Technologies, Inc.
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

package api

import (
	"context"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/validate"
)

var errScheduleNotImplemented = &types.BadRequestError{Message: "Schedule APIs are not yet implemented."}

func (wh *WorkflowHandler) CreateSchedule(
	ctx context.Context,
	request *types.CreateScheduleRequest,
) (*types.CreateScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}
	return nil, errScheduleNotImplemented
}

func (wh *WorkflowHandler) DescribeSchedule(
	ctx context.Context,
	request *types.DescribeScheduleRequest,
) (*types.DescribeScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}
	return nil, errScheduleNotImplemented
}

func (wh *WorkflowHandler) UpdateSchedule(
	ctx context.Context,
	request *types.UpdateScheduleRequest,
) (*types.UpdateScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}
	return nil, errScheduleNotImplemented
}

func (wh *WorkflowHandler) DeleteSchedule(
	ctx context.Context,
	request *types.DeleteScheduleRequest,
) (*types.DeleteScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}
	return nil, errScheduleNotImplemented
}

func (wh *WorkflowHandler) PauseSchedule(
	ctx context.Context,
	request *types.PauseScheduleRequest,
) (*types.PauseScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}
	return nil, errScheduleNotImplemented
}

func (wh *WorkflowHandler) UnpauseSchedule(
	ctx context.Context,
	request *types.UnpauseScheduleRequest,
) (*types.UnpauseScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}
	return nil, errScheduleNotImplemented
}

func (wh *WorkflowHandler) BackfillSchedule(
	ctx context.Context,
	request *types.BackfillScheduleRequest,
) (*types.BackfillScheduleResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}
	return nil, errScheduleNotImplemented
}

func (wh *WorkflowHandler) ListSchedules(
	ctx context.Context,
	request *types.ListSchedulesRequest,
) (*types.ListSchedulesResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if request == nil {
		return nil, validate.ErrRequestNotSet
	}
	return nil, errScheduleNotImplemented
}
