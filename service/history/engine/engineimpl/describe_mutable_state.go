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
	"encoding/json"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/validation"
	"github.com/uber/cadence/service/history/execution"
)

func (e *historyEngineImpl) DescribeMutableState(
	ctx context.Context,
	request *types.DescribeMutableStateRequest,
) (response *types.DescribeMutableStateResponse, retError error) {

	if err := validation.ValidateDomainUUID(request.DomainUUID); err != nil {
		return nil, err
	}

	domainID := request.DomainUUID
	execution := types.WorkflowExecution{
		WorkflowID: request.Execution.WorkflowID,
		RunID:      request.Execution.RunID,
	}

	cacheCtx, dbCtx, release, cacheHit, err := e.executionCache.GetAndCreateWorkflowExecution(
		ctx, domainID, execution,
	)
	if err != nil {
		return nil, err
	}
	defer func() { release(retError) }()

	response = &types.DescribeMutableStateResponse{}

	if cacheHit {
		if msb := cacheCtx.GetWorkflowExecution(); msb != nil {
			response.MutableStateInCache, err = e.toMutableStateJSON(msb)
			if err != nil {
				return nil, err
			}
		}
	}

	msb, err := dbCtx.LoadWorkflowExecution(ctx)
	if err != nil {
		return nil, err
	}
	response.MutableStateInDatabase, err = e.toMutableStateJSON(msb)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (e *historyEngineImpl) toMutableStateJSON(msb execution.MutableState) (string, error) {
	ms := msb.CopyToPersistence()

	jsonBytes, err := json.Marshal(ms)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}
