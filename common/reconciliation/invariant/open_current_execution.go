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
	"fmt"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/types"
)

type (
	openCurrentExecution struct {
		pr persistence.Retryer
		dc cache.DomainCache
	}
)

// NewOpenCurrentExecution returns a new invariant for checking open current execution
func NewOpenCurrentExecution(
	pr persistence.Retryer, dc cache.DomainCache,
) Invariant {
	return &openCurrentExecution{
		pr: pr,
		dc: dc,
	}
}

func (o *openCurrentExecution) Check(
	ctx context.Context,
	execution interface{},
) CheckResult {
	if checkResult := validateCheckContext(ctx, o.Name()); checkResult != nil {
		return *checkResult
	}

	concreteExecution, ok := execution.(*entity.ConcreteExecution)
	if !ok {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   o.Name(),
			Info:            "failed to check: expected concrete execution",
		}
	}
	if !Open(concreteExecution.State) {
		return CheckResult{
			CheckResultType: CheckResultTypeHealthy,
			InvariantName:   o.Name(),
		}
	}
	domainName, err := o.dc.GetDomainName(concreteExecution.DomainID)
	if err != nil {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   o.Name(),
			Info:            "failed to fetch Domain Name",
			InfoDetails:     err.Error(),
		}
	}
	currentExecResp, currentExecErr := o.pr.GetCurrentExecution(ctx, &persistence.GetCurrentExecutionRequest{
		DomainID:   concreteExecution.DomainID,
		WorkflowID: concreteExecution.WorkflowID,
		DomainName: domainName,
	})

	stillOpen, stillOpenErr := ExecutionStillOpen(ctx, &concreteExecution.Execution, o.pr, o.dc)
	if stillOpenErr != nil {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   o.Name(),
			Info:            "failed to check if concrete execution is still open",
			InfoDetails:     stillOpenErr.Error(),
		}
	}
	if !stillOpen {
		return CheckResult{
			CheckResultType: CheckResultTypeHealthy,
			InvariantName:   o.Name(),
		}
	}
	if currentExecErr != nil {
		switch currentExecErr.(type) {
		case *types.EntityNotExistsError:
			return CheckResult{
				CheckResultType: CheckResultTypeCorrupted,
				InvariantName:   o.Name(),
				Info:            "execution is open without having current execution",
				InfoDetails:     currentExecErr.Error(),
			}
		default:
			return CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   o.Name(),
				Info:            "failed to check if current execution exists",
				InfoDetails:     currentExecErr.Error(),
			}
		}
	}
	if currentExecResp.RunID != concreteExecution.RunID {
		return CheckResult{
			CheckResultType: CheckResultTypeCorrupted,
			InvariantName:   o.Name(),
			Info:            "execution is open but current points at a different execution",
			InfoDetails:     fmt.Sprintf("current points at %v", currentExecResp.RunID),
		}
	}
	return CheckResult{
		CheckResultType: CheckResultTypeHealthy,
		InvariantName:   o.Name(),
	}
}

func (o *openCurrentExecution) Fix(
	ctx context.Context,
	execution interface{},
) FixResult {
	if fixResult := validateFixContext(ctx, o.Name()); fixResult != nil {
		return *fixResult
	}

	fixResult, checkResult := checkBeforeFix(ctx, o, execution)
	if fixResult != nil {
		return *fixResult
	}
	fixResult = DeleteExecution(ctx, execution, o.pr, o.dc)
	fixResult.CheckResult = *checkResult
	fixResult.InvariantName = o.Name()
	return *fixResult
}

func (o *openCurrentExecution) Name() Name {
	return OpenCurrentExecution
}

// ExecutionStillOpen returns true if execution in persistence exists and is open, false otherwise.
// Returns error on failure to confirm.
func ExecutionStillOpen(
	ctx context.Context,
	exec *entity.Execution,
	pr persistence.Retryer,
	dc cache.DomainCache,
) (bool, error) {
	domainName, err := dc.GetDomainName(exec.DomainID)
	if err != nil {
		return false, nil
	}
	req := &persistence.GetWorkflowExecutionRequest{
		DomainID: exec.DomainID,
		Execution: types.WorkflowExecution{
			WorkflowID: exec.WorkflowID,
			RunID:      exec.RunID,
		},
		DomainName: domainName,
	}
	resp, err := pr.GetWorkflowExecution(ctx, req)
	if err != nil {
		switch err.(type) {
		case *types.EntityNotExistsError:
			return false, nil
		default:
			return false, err
		}
	}
	return Open(resp.State.ExecutionInfo.State), nil
}
