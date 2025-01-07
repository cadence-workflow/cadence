// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
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

package invariant

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	c "github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/types"
)

func TestConcreteExecutionCheckAndFix(t *testing.T) {
	notExistsError := types.EntityNotExistsError{}
	unknownError := types.BadRequestError{}
	testCases := []struct {
		desc             string
		execution        any
		ctx              context.Context
		getConcreteResp  *persistence.IsWorkflowExecutionExistsResponse
		getConcreteErr   error
		getCurrentResp   *persistence.GetCurrentExecutionResponse
		getCurrentErr    error
		getDomainNameErr error
		wantCheckResult  CheckResult
		wantFixResult    FixResult
	}{
		{
			desc:            "closed execution with concrete execution",
			execution:       getClosedCurrentExecution(),
			getConcreteResp: &persistence.IsWorkflowExecutionExistsResponse{Exists: true},
			getCurrentResp: &persistence.GetCurrentExecutionResponse{
				RunID: getClosedCurrentExecution().CurrentRunID,
			},
			wantCheckResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   ConcreteExecutionExists,
			},
			wantFixResult: FixResult{
				FixResultType: FixResultTypeSkipped,
				InvariantName: ConcreteExecutionExists,
				Info:          "skipped fix because execution was healthy",
			},
		},
		{
			desc:           "failed to get concrete execution",
			execution:      getOpenCurrentExecution(),
			getConcreteErr: errors.New("error getting concrete execution"),
			getCurrentResp: &persistence.GetCurrentExecutionResponse{
				RunID: getOpenCurrentExecution().CurrentRunID,
			},
			wantCheckResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   ConcreteExecutionExists,
				Info:            "failed to check if concrete execution exists",
				InfoDetails:     "error getting concrete execution",
			},
			wantFixResult: FixResult{
				FixResultType: FixResultTypeFailed,
				InvariantName: ConcreteExecutionExists,
				Info:          "failed fix because check failed",
			},
		},
		{
			desc:            "open execution without concrete execution",
			execution:       getOpenCurrentExecution(),
			getConcreteResp: &persistence.IsWorkflowExecutionExistsResponse{Exists: false},
			getCurrentResp: &persistence.GetCurrentExecutionResponse{
				RunID: getOpenCurrentExecution().CurrentRunID,
			},
			wantCheckResult: CheckResult{
				CheckResultType: CheckResultTypeCorrupted,
				InvariantName:   ConcreteExecutionExists,
				Info:            "execution is open without having concrete execution",
				InfoDetails:     fmt.Sprintf("concrete execution not found. WorkflowId: %v, RunId: %v", workflowID, currentRunID),
			},
			wantFixResult: FixResult{
				FixResultType: FixResultTypeFixed,
				InvariantName: ConcreteExecutionExists,
			},
		},
		{
			desc:            "mismatching current runid and concrete execution doesn't exist",
			execution:       getOpenCurrentExecution(),
			getConcreteResp: &persistence.IsWorkflowExecutionExistsResponse{Exists: false},
			getCurrentResp: &persistence.GetCurrentExecutionResponse{
				RunID: uuid.New(),
			},
			wantCheckResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   ConcreteExecutionExists,
			},
			wantFixResult: FixResult{
				FixResultType: FixResultTypeSkipped,
				InvariantName: ConcreteExecutionExists,
				Info:          "skipped fix because execution was healthy",
			},
		},
		{
			desc:            "open execution with concrete execution",
			execution:       getOpenCurrentExecution(),
			getConcreteErr:  nil,
			getConcreteResp: &persistence.IsWorkflowExecutionExistsResponse{Exists: true},
			getCurrentResp: &persistence.GetCurrentExecutionResponse{
				RunID: getOpenCurrentExecution().CurrentRunID,
			},
			wantCheckResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   ConcreteExecutionExists,
			},
			wantFixResult: FixResult{
				FixResultType: FixResultTypeSkipped,
				InvariantName: ConcreteExecutionExists,
				Info:          "skipped fix because execution was healthy",
			},
		},
		{
			desc:            "open execution that is not current",
			execution:       getOpenCurrentExecution(),
			getConcreteErr:  nil,
			getConcreteResp: &persistence.IsWorkflowExecutionExistsResponse{Exists: true},
			getCurrentResp: &persistence.GetCurrentExecutionResponse{
				RunID: uuid.New(),
			},
			wantCheckResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   ConcreteExecutionExists,
			},
			wantFixResult: FixResult{
				FixResultType: FixResultTypeSkipped,
				InvariantName: ConcreteExecutionExists,
				Info:          "skipped fix because execution was healthy",
			},
		},
		{
			desc:            "concrete exists but current doesn't",
			execution:       getOpenCurrentExecution(),
			getConcreteErr:  nil,
			getConcreteResp: &persistence.IsWorkflowExecutionExistsResponse{Exists: true},
			getCurrentResp:  nil,
			getCurrentErr:   &notExistsError,
			wantCheckResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   ConcreteExecutionExists,
			},
			wantFixResult: FixResult{
				FixResultType: FixResultTypeSkipped,
				InvariantName: ConcreteExecutionExists,
				Info:          "skipped fix because execution was healthy",
			},
		},
		{
			desc:            "concrete exists but failed to get current",
			execution:       getOpenCurrentExecution(),
			getConcreteErr:  nil,
			getConcreteResp: &persistence.IsWorkflowExecutionExistsResponse{Exists: false},
			getCurrentResp:  nil,
			getCurrentErr:   &unknownError,
			wantCheckResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   ConcreteExecutionExists,
				Info:            "failed to get current execution.",
				InfoDetails:     unknownError.Error(),
			},
			wantFixResult: FixResult{
				FixResultType: FixResultTypeFailed,
				InvariantName: ConcreteExecutionExists,
				Info:          "failed fix because check failed",
			},
		},
		{
			desc: "canceled context",
			ctx:  canceledCtx(),
			wantCheckResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   ConcreteExecutionExists,
				Info:            "failed to check: context expired or cancelled",
				InfoDetails:     "context canceled",
			},
			wantFixResult: FixResult{
				FixResultType: FixResultTypeFailed,
				InvariantName: ConcreteExecutionExists,
				Info:          "failed to check: context expired or cancelled",
				InfoDetails:   "context canceled",
			},
		},
		{
			desc:      "invalid execution object",
			execution: &entity.ConcreteExecution{}, // invalid type
			wantCheckResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   ConcreteExecutionExists,
				Info:            "failed to check: expected current execution",
			},
			wantFixResult: FixResult{
				FixResultType: FixResultTypeFailed,
				InvariantName: ConcreteExecutionExists,
				Info:          "failed to fix: expected current execution",
			},
		},
		{
			desc: "empty run id - found after current lookup",
			execution: func() *entity.CurrentExecution {
				e := getOpenCurrentExecution()
				e.CurrentRunID = ""
				return e
			}(),
			getConcreteResp: &persistence.IsWorkflowExecutionExistsResponse{Exists: true},
			getCurrentResp: &persistence.GetCurrentExecutionResponse{
				RunID: currentRunID,
			},
			wantCheckResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   ConcreteExecutionExists,
			},
			wantFixResult: FixResult{
				FixResultType: FixResultTypeSkipped,
				InvariantName: ConcreteExecutionExists,
				Info:          "skipped fix because execution was healthy",
			},
		},
		{
			desc: "empty run id - current lookup returns not found",
			execution: func() *entity.CurrentExecution {
				e := getOpenCurrentExecution()
				e.CurrentRunID = ""
				return e
			}(),
			getCurrentErr: &notExistsError,
			wantCheckResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   ConcreteExecutionExists,
				Info:            "current execution does not exist.",
			},
			wantFixResult: FixResult{
				FixResultType: FixResultTypeSkipped,
				InvariantName: ConcreteExecutionExists,
			},
		},
		{
			desc: "empty run id - domain name lookup failed",
			execution: func() *entity.CurrentExecution {
				e := getOpenCurrentExecution()
				e.CurrentRunID = ""
				return e
			}(),
			getDomainNameErr: errors.New("error getting domain name"),
			wantCheckResult: CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   ConcreteExecutionExists,
				Info:            "failed to fetch domainName",
				InfoDetails:     "error getting domain name",
			},
			wantFixResult: FixResult{
				FixResultType: FixResultTypeSkipped,
				InvariantName: ConcreteExecutionExists,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDomainCache := cache.NewMockDomainCache(ctrl)
			execManager := &mocks.ExecutionManager{}
			execManager.On("IsWorkflowExecutionExists", mock.Anything, mock.Anything).Return(tc.getConcreteResp, tc.getConcreteErr)
			execManager.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(tc.getCurrentResp, tc.getCurrentErr)
			execManager.On("DeleteCurrentWorkflowExecution", mock.Anything, mock.Anything).Return(nil)
			mockDomainCache.EXPECT().GetDomainName(domainID).Return(domainName, tc.getDomainNameErr).AnyTimes()
			o := NewConcreteExecutionExists(persistence.NewPersistenceRetryer(execManager, nil, c.CreatePersistenceRetryPolicy()), mockDomainCache)
			ctx := tc.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			require.Equal(t, tc.wantCheckResult, o.Check(ctx, tc.execution))

			gotFixResult := o.Fix(ctx, tc.execution)
			gotFixResult.CheckResult = CheckResult{}
			require.Equal(t, tc.wantFixResult, gotFixResult)
		})
	}
}

func canceledCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}
