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

package engineimpl

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/execution"
)

func TestDescribeMutableState_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	cachedExecutionInfo := getWorkflowMutableState()
	dbExecutionInfo := getWorkflowMutableState()

	// Make sure the db is different from the cache, so we can verify the correct value
	// goes to the correct field in the response
	dbExecutionInfo.ExecutionInfo.DomainID = constants.TestDomainID + "other"

	// Cache context mock
	mutableStateFromCacheMock := execution.NewMockMutableState(ctrl)
	mutableStateFromCacheMock.EXPECT().CopyToPersistence().Return(cachedExecutionInfo)
	cacheContextMock := execution.NewMockContext(ctrl)
	cacheContextMock.EXPECT().GetWorkflowExecution().Return(mutableStateFromCacheMock)

	// DB context mock
	mutableStateFromDBMock := execution.NewMockMutableState(ctrl)
	mutableStateFromDBMock.EXPECT().CopyToPersistence().Return(dbExecutionInfo)
	dbContextMock := execution.NewMockContext(ctrl)
	dbContextMock.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(mutableStateFromDBMock, nil)

	releaseFunctionCalled := false

	cacheMock := execution.NewMockCache(ctrl)
	cacheMock.EXPECT().
		GetAndCreateWorkflowExecution(gomock.Any(), constants.TestDomainID, *getExpectedWFExecution()).
		Return(cacheContextMock, dbContextMock, func(err error) {
			releaseFunctionCalled = true
			assert.NoError(t, err)
		}, true, nil)

	engine := &historyEngineImpl{
		executionCache: cacheMock,
	}

	resp, err := engine.DescribeMutableState(context.TODO(), &types.DescribeMutableStateRequest{
		DomainUUID: constants.TestDomainID,
		Execution:  getExpectedWFExecution(),
	})
	assert.NoError(t, err)

	expectedCachedExecutionInfo, err := json.Marshal(cachedExecutionInfo)
	require.NoError(t, err)
	expectedDBExecutionInfo, err := json.Marshal(dbExecutionInfo)
	require.NoError(t, err)

	// The response should contain the json representation of the execution info
	assert.Equal(t, string(expectedCachedExecutionInfo), resp.MutableStateInCache)
	assert.Equal(t, string(expectedDBExecutionInfo), resp.MutableStateInDatabase)

	// The release function should have been called
	assert.True(t, releaseFunctionCalled)
}

func TestDescribeMutableState_Error_UnknownDomain(t *testing.T) {
	engine := &historyEngineImpl{}

	_, err := engine.DescribeMutableState(context.TODO(), &types.DescribeMutableStateRequest{
		DomainUUID: "This is not a uuid",
	})
	assert.Error(t, err)
	assert.ErrorAs(t, err, new(*types.BadRequestError))
	assert.ErrorContains(t, err, "Invalid domain UUID")
}

func TestDescribeMutableState_Error_GetAndCreateError(t *testing.T) {
	ctrl := gomock.NewController(t)

	cacheMock := execution.NewMockCache(ctrl)
	cacheMock.EXPECT().
		GetAndCreateWorkflowExecution(gomock.Any(), constants.TestDomainID, *getExpectedWFExecution()).
		Return(nil, nil, nil, true, assert.AnError)

	engine := &historyEngineImpl{
		executionCache: cacheMock,
	}

	_, err := engine.DescribeMutableState(context.TODO(), &types.DescribeMutableStateRequest{
		DomainUUID: constants.TestDomainID,
		Execution:  getExpectedWFExecution(),
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestDescribeMutableState_Error_LoadFromDBError(t *testing.T) {
	ctrl := gomock.NewController(t)

	// DB context mock returns error
	dbContextMock := execution.NewMockContext(ctrl)
	dbContextMock.EXPECT().LoadWorkflowExecution(gomock.Any()).Return(nil, assert.AnError)

	releaseFunctionCalled := false
	cacheMock := execution.NewMockCache(ctrl)
	cacheMock.EXPECT().
		GetAndCreateWorkflowExecution(gomock.Any(), constants.TestDomainID, *getExpectedWFExecution()).
		Return(nil, dbContextMock, func(err error) {
			releaseFunctionCalled = true
			assert.ErrorIs(t, err, assert.AnError)
		}, false, nil)

	engine := &historyEngineImpl{
		executionCache: cacheMock,
	}

	_, err := engine.DescribeMutableState(context.TODO(), &types.DescribeMutableStateRequest{
		DomainUUID: constants.TestDomainID,
		Execution:  getExpectedWFExecution(),
	})
	assert.ErrorIs(t, err, assert.AnError)

	// release function should have been called
	assert.True(t, releaseFunctionCalled)
}

// The content of this struct is not important for the purpose of this test,
// we just want to check that the method returns the json representation of the struct
func getWorkflowMutableState() *persistence.WorkflowMutableState {
	return &persistence.WorkflowMutableState{
		ActivityInfos:       map[int64]*persistence.ActivityInfo{},
		TimerInfos:          nil,
		ChildExecutionInfos: nil,
		RequestCancelInfos:  nil,
		SignalInfos:         nil,
		SignalRequestedIDs:  nil,
		ExecutionInfo: &persistence.WorkflowExecutionInfo{
			DomainID:   constants.TestDomainID,
			WorkflowID: constants.TestWorkflowID,
			RunID:      constants.TestRunID,
		},
		ExecutionStats:   nil,
		BufferedEvents:   nil,
		VersionHistories: nil,
		ReplicationState: nil,
		Checksum:         checksum.Checksum{},
	}
}

func getExpectedWFExecution() *types.WorkflowExecution {
	return &types.WorkflowExecution{
		WorkflowID: constants.TestWorkflowID,
		RunID:      constants.TestRunID,
	}
}
