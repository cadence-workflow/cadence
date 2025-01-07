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

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/types"
)

const TimerInvalidName = "TimerInvalid"

type TimerInvalid struct {
	pr    persistence.Retryer
	cache cache.DomainCache
}

// NewTimerInvalid returns a new history exists invariant
func NewTimerInvalid(
	pr persistence.Retryer, cache cache.DomainCache,
) Invariant {
	return &TimerInvalid{
		pr:    pr,
		cache: cache,
	}
}

// Check checks if timer is scheduled for open execution
func (h *TimerInvalid) Check(
	ctx context.Context,
	e interface{},
) CheckResult {
	if checkResult := validateCheckContext(ctx, h.Name()); checkResult != nil {
		return *checkResult
	}

	timer, ok := e.(*entity.Timer)

	if !ok {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   h.Name(),
			Info:            "failed to check: expected timer entity",
		}
	}
	domainID := timer.DomainID
	domainName, err := h.cache.GetDomainName(timer.DomainID)
	if err != nil {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   h.Name(),
			Info:            "failed to check: expected Domain Name",
		}
	}
	req := &persistence.GetWorkflowExecutionRequest{
		DomainID: domainID,
		Execution: types.WorkflowExecution{
			WorkflowID: timer.WorkflowID,
			RunID:      timer.RunID,
		},
		DomainName: domainName,
	}

	resp, err := h.pr.GetWorkflowExecution(ctx, req)

	if err != nil {
		switch err.(type) {
		case *types.EntityNotExistsError:
			return CheckResult{
				CheckResultType: CheckResultTypeCorrupted,
				InvariantName:   h.Name(),
				Info:            "timer scheduled for non existing workflow",
			}
		default:
			return CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   h.Name(),
				Info:            "failed to get workflow for timer",
			}
		}
	}

	if !Open(resp.State.ExecutionInfo.State) {
		return CheckResult{
			CheckResultType: CheckResultTypeCorrupted,
			InvariantName:   h.Name(),
			Info:            "timer scheduled for closed workflow",
		}
	}

	return CheckResult{
		CheckResultType: CheckResultTypeHealthy,
		InvariantName:   h.Name(),
	}
}

// Fix will delete invalid timer
func (h *TimerInvalid) Fix(
	ctx context.Context,
	e interface{},
) FixResult {

	if fixResult := validateFixContext(ctx, h.Name()); fixResult != nil {
		return *fixResult
	}

	fixResult, checkResult := checkBeforeFix(ctx, h, e)
	if fixResult != nil {
		return *fixResult
	}

	timer, _ := e.(*entity.Timer)

	if timer.TaskType != persistence.TaskTypeUserTimer {
		return FixResult{
			FixResultType: FixResultTypeSkipped,
			InvariantName: h.Name(),
			Info:          "timer is not a TaskTypeUserTimer",
		}
	}

	req := persistence.CompleteTimerTaskRequest{
		VisibilityTimestamp: timer.VisibilityTimestamp,
		TaskID:              timer.TaskID,
	}

	if err := h.pr.CompleteTimerTask(ctx, &req); err != nil {
		return FixResult{
			FixResultType: FixResultTypeFailed,
			InvariantName: h.Name(),
			Info:          err.Error(),
		}
	}

	return FixResult{
		FixResultType: FixResultTypeFixed,
		InvariantName: h.Name(),
		CheckResult:   *checkResult,
	}
}

func (h *TimerInvalid) Name() Name {
	return TimerInvalidName
}
