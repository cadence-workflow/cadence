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

type concreteExecutionExists struct {
	pr    persistence.Retryer
	cache cache.DomainCache
}

// NewConcreteExecutionExists returns a new invariant for checking concrete execution
func NewConcreteExecutionExists(pr persistence.Retryer, cache cache.DomainCache) Invariant {
	return &concreteExecutionExists{
		pr:    pr,
		cache: cache,
	}
}

func (c *concreteExecutionExists) Check(ctx context.Context, execution interface{}) CheckResult {
	if checkResult := validateCheckContext(ctx, c.Name()); checkResult != nil {
		return *checkResult
	}

	currentExecution, ok := execution.(*entity.CurrentExecution)
	if !ok {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   c.Name(),
			Info:            "failed to check: expected current execution",
		}
	}

	if len(currentExecution.CurrentRunID) == 0 {
		// set the current run id
		var runIDCheckResult *CheckResult
		currentExecution, runIDCheckResult = c.validateCurrentRunID(ctx, currentExecution)
		if runIDCheckResult != nil {
			return *runIDCheckResult
		}
	}

	domainName, err := c.cache.GetDomainName(currentExecution.DomainID)
	if err != nil {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   c.Name(),
			Info:            "Failed to fetch Domain Name",
			InfoDetails:     err.Error(),
		}
	}
	concreteExecResp, concreteExecErr := c.pr.IsWorkflowExecutionExists(ctx, &persistence.IsWorkflowExecutionExistsRequest{
		DomainID:   currentExecution.DomainID,
		DomainName: domainName,
		WorkflowID: currentExecution.WorkflowID,
		RunID:      currentExecution.CurrentRunID,
	})
	if concreteExecErr != nil {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   c.Name(),
			Info:            "failed to check if concrete execution exists",
			InfoDetails:     concreteExecErr.Error(),
		}
	}
	if !concreteExecResp.Exists {
		// verify if the current execution exists
		_, checkResult := c.validateCurrentRunID(ctx, currentExecution)
		if checkResult != nil {
			return *checkResult
		}
		return CheckResult{
			CheckResultType: CheckResultTypeCorrupted,
			InvariantName:   c.Name(),
			Info:            "execution is open without having concrete execution",
			InfoDetails: fmt.Sprintf("concrete execution not found. WorkflowId: %v, RunId: %v",
				currentExecution.WorkflowID, currentExecution.CurrentRunID),
		}
	}
	return CheckResult{
		CheckResultType: CheckResultTypeHealthy,
		InvariantName:   c.Name(),
	}
}

func (c *concreteExecutionExists) Fix(ctx context.Context, execution interface{}) FixResult {
	if fixResult := validateFixContext(ctx, c.Name()); fixResult != nil {
		return *fixResult
	}

	currentExecution, ok := execution.(*entity.CurrentExecution)
	if !ok {
		return FixResult{
			FixResultType: FixResultTypeFailed,
			InvariantName: c.Name(),
			Info:          "failed to fix: expected current execution",
		}
	}

	var runIDCheckResult *CheckResult
	if len(currentExecution.CurrentRunID) == 0 {
		// this is to set the current run ID prior to the check and fix operations
		currentExecution, runIDCheckResult = c.validateCurrentRunID(ctx, currentExecution)
		if runIDCheckResult != nil {
			return FixResult{
				FixResultType: FixResultTypeSkipped,
				CheckResult:   *runIDCheckResult,
				InvariantName: c.Name(),
			}
		}
	}
	fixResult, checkResult := checkBeforeFix(ctx, c, currentExecution)
	if fixResult != nil {
		return *fixResult
	}
	domainName, errorDomain := c.cache.GetDomainName(currentExecution.DomainID)
	if errorDomain != nil {
		return FixResult{
			FixResultType: FixResultTypeFailed,
			InvariantName: c.Name(),
			Info:          "failed to fetch Domain Name",
			InfoDetails:   errorDomain.Error(),
		}
	}
	if err := c.pr.DeleteCurrentWorkflowExecution(ctx, &persistence.DeleteCurrentWorkflowExecutionRequest{
		DomainID:   currentExecution.DomainID,
		WorkflowID: currentExecution.WorkflowID,
		RunID:      currentExecution.CurrentRunID,
		DomainName: domainName,
	}); err != nil {
		return FixResult{
			FixResultType: FixResultTypeFailed,
			InvariantName: c.Name(),
			Info:          "failed to delete current workflow execution",
			InfoDetails:   err.Error(),
		}
	}
	return FixResult{
		FixResultType: FixResultTypeFixed,
		CheckResult:   *checkResult,
		InvariantName: c.Name(),
	}
}

func (c *concreteExecutionExists) Name() Name {
	return ConcreteExecutionExists
}

func (c *concreteExecutionExists) validateCurrentRunID(
	ctx context.Context,
	currentExecution *entity.CurrentExecution,
) (*entity.CurrentExecution, *CheckResult) {
	domainName, err := c.cache.GetDomainName(currentExecution.DomainID)
	if err != nil {
		return nil, &CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   c.Name(),
			Info:            "failed to fetch domainName",
			InfoDetails:     err.Error(),
		}
	}
	resp, err := c.pr.GetCurrentExecution(ctx, &persistence.GetCurrentExecutionRequest{
		DomainID:   currentExecution.DomainID,
		WorkflowID: currentExecution.WorkflowID,
		DomainName: domainName,
	})
	if err != nil {
		switch err.(type) {
		case *types.EntityNotExistsError:
			return nil, &CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   c.Name(),
				Info:            "current execution does not exist.",
				InfoDetails:     err.Error(),
			}
		default:
			return nil, &CheckResult{
				CheckResultType: CheckResultTypeFailed,
				InvariantName:   c.Name(),
				Info:            "failed to get current execution.",
				InfoDetails:     err.Error(),
			}
		}
	}

	if len(currentExecution.CurrentRunID) == 0 {
		currentExecution.CurrentRunID = resp.RunID
	}

	if currentExecution.CurrentRunID != resp.RunID {
		return nil, &CheckResult{
			CheckResultType: CheckResultTypeHealthy,
			InvariantName:   c.Name(),
		}
	}
	return currentExecution, nil
}
