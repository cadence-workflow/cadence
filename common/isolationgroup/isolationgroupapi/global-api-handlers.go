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

package isolationgroupapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
)

func (z *handlerImpl) UpdateGlobalState(ctx context.Context, in types.UpdateGlobalIsolationGroupsRequest) error {
	if z.globalIsolationGroupDrains == nil {
		return &types.BadRequestError{"global isolation group drain is not supported in this cluster"}
	}
	mappedInput, err := MapUpdateGlobalIsolationGroupsRequest(in.IsolationGroups)
	if err != nil {
		return err
	}
	return z.globalIsolationGroupDrains.UpdateValue(
		dynamicproperties.DefaultIsolationGroupConfigStoreManagerGlobalMapping,
		mappedInput,
	)
}

func (z *handlerImpl) GetGlobalState(ctx context.Context) (*types.GetGlobalIsolationGroupsResponse, error) {
	if z.globalIsolationGroupDrains == nil {
		return nil, &types.BadRequestError{"global isolation group drain is not supported in this cluster"}
	}
	res, err := z.globalIsolationGroupDrains.GetListValue(dynamicproperties.DefaultIsolationGroupConfigStoreManagerGlobalMapping, nil)
	if err != nil {
		var e types.EntityNotExistsError
		if errors.As(err, &e) {
			return &types.GetGlobalIsolationGroupsResponse{}, nil
		}
		return nil, fmt.Errorf("failed to get global isolation groups from datastore: %w", err)
	}
	resp, err := MapDynamicConfigResponse(res)
	if err != nil {
		return nil, fmt.Errorf("failed to get global isolation groups from datastore: %w", err)
	}
	return &types.GetGlobalIsolationGroupsResponse{IsolationGroups: resp}, nil
}
