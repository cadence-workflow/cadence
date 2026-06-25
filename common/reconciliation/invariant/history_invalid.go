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
	"math"

	c "github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/types"
)

type historyInvalid struct {
	pr persistence.Retryer
	dc cache.DomainCache
}

// NewHistoryInvalid returns a new history invalid invariant that checks
// whether the full history is readable and not corrupted.
func NewHistoryInvalid(
	pr persistence.Retryer, dc cache.DomainCache,
) Invariant {
	return &historyInvalid{
		pr: pr,
		dc: dc,
	}
}

func (h *historyInvalid) Check(
	ctx context.Context,
	execution interface{},
) CheckResult {
	if checkResult := validateCheckContext(ctx, h.Name()); checkResult != nil {
		return *checkResult
	}

	concreteExecution, ok := execution.(*entity.ConcreteExecution)
	if !ok {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   h.Name(),
			Info:            "failed to check: expected concrete execution",
		}
	}

	domainID := concreteExecution.GetDomainID()
	domainName, err := h.dc.GetDomainName(domainID)
	if err != nil {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   h.Name(),
			Info:            "failed to check: expected DomainName",
			InfoDetails:     err.Error(),
		}
	}

	var nextPageToken []byte
	var lastEventID int64

	for {
		resp, readErr := h.pr.ReadHistoryBranch(ctx, &persistence.ReadHistoryBranchRequest{
			BranchToken:   concreteExecution.BranchToken,
			MinEventID:    constants.FirstEventID,
			MaxEventID:    math.MaxInt64,
			PageSize:      100,
			NextPageToken: nextPageToken,
			ShardID:       c.IntPtr(concreteExecution.ShardID),
			DomainName:    domainName,
		})
		if readErr != nil {
			if errors.Is(readErr, persistence.ErrCorruptedHistory) {
				return CheckResult{
					CheckResultType: CheckResultTypeCorrupted,
					InvariantName:   h.Name(),
					Info:            "corrupt history",
					InfoDetails:     readErr.Error(),
				}
			}
			return CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   h.Name(),
				Info:            "failed to read history",
				InfoDetails:     readErr.Error(),
			}
		}
		if len(resp.HistoryEvents) > 0 {
			lastEventID = resp.HistoryEvents[len(resp.HistoryEvents)-1].ID
		}
		if resp.NextPageToken == nil {
			break
		}
		nextPageToken = resp.NextPageToken
	}

	if lastEventID == 0 {
		return CheckResult{
			CheckResultType: CheckResultTypeCorrupted,
			InvariantName:   h.Name(),
			Info:            "empty history",
			InfoDetails:     "no history events found for workflow",
		}
	}

	wfResp, wfErr := h.pr.GetWorkflowExecution(ctx, &persistence.GetWorkflowExecutionRequest{
		DomainID: domainID,
		Execution: types.WorkflowExecution{
			WorkflowID: concreteExecution.WorkflowID,
			RunID:      concreteExecution.RunID,
		},
		DomainName: domainName,
	})
	if wfErr != nil {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   h.Name(),
			Info:            "failed to get workflow execution",
			InfoDetails:     wfErr.Error(),
		}
	}

	expectedLastEventID := wfResp.State.ExecutionInfo.NextEventID - 1
	if !Open(wfResp.State.ExecutionInfo.State) && lastEventID != expectedLastEventID {
		return CheckResult{
			CheckResultType: CheckResultTypeCorrupted,
			InvariantName:   h.Name(),
			Info:            "event count mismatch",
			InfoDetails:     fmt.Sprintf("history has last event ID %d but execution record expects %d", lastEventID, expectedLastEventID),
		}
	}

	return CheckResult{
		CheckResultType: CheckResultTypeHealthy,
		InvariantName:   h.Name(),
	}
}

func (h *historyInvalid) Fix(
	ctx context.Context,
	execution interface{},
) FixResult {
	if fixResult := validateFixContext(ctx, h.Name()); fixResult != nil {
		return *fixResult
	}

	fixResult, checkResult := checkBeforeFix(ctx, h, execution)
	if fixResult != nil {
		return *fixResult
	}
	fixResult = DeleteExecution(ctx, execution, h.pr, h.dc)
	fixResult.CheckResult = *checkResult
	fixResult.InvariantName = h.Name()
	return *fixResult
}

func (h *historyInvalid) Name() Name {
	return HistoryInvalid
}
