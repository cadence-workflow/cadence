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

type mismatchedRecords struct {
	pr persistence.Retryer
	dc cache.DomainCache
}

// NewMismatchedRecords returns a new invariant that checks whether
// current and concrete execution records agree on close status.
func NewMismatchedRecords(
	pr persistence.Retryer, dc cache.DomainCache,
) Invariant {
	return &mismatchedRecords{
		pr: pr,
		dc: dc,
	}
}

func (m *mismatchedRecords) Check(
	ctx context.Context,
	execution interface{},
) CheckResult {
	if checkResult := validateCheckContext(ctx, m.Name()); checkResult != nil {
		return *checkResult
	}

	concreteExecution, ok := execution.(*entity.ConcreteExecution)
	if !ok {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   m.Name(),
			Info:            "failed to check: expected concrete execution",
		}
	}

	domainID := concreteExecution.GetDomainID()
	domainName, err := m.dc.GetDomainName(domainID)
	if err != nil {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   m.Name(),
			Info:            "failed to check: expected DomainName",
			InfoDetails:     err.Error(),
		}
	}

	currentExec, err := m.pr.GetCurrentExecution(ctx, &persistence.GetCurrentExecutionRequest{
		DomainID:   domainID,
		WorkflowID: concreteExecution.WorkflowID,
		DomainName: domainName,
	})
	if err != nil {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   m.Name(),
			Info:            "failed to get current execution record",
			InfoDetails:     err.Error(),
		}
	}

	concreteWorkflow, err := m.pr.GetWorkflowExecution(ctx, &persistence.GetWorkflowExecutionRequest{
		DomainID: domainID,
		Execution: types.WorkflowExecution{
			WorkflowID: concreteExecution.WorkflowID,
			RunID:      concreteExecution.RunID,
		},
		DomainName: domainName,
	})
	if err != nil {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   m.Name(),
			Info:            "failed to get workflow execution",
			InfoDetails:     err.Error(),
		}
	}

	if concreteWorkflow.State.ExecutionInfo.CloseStatus != currentExec.CloseStatus {
		return CheckResult{
			CheckResultType: CheckResultTypeCorrupted,
			InvariantName:   m.Name(),
			Info:            "current record does not agree with concrete record about workflow being closed",
			InfoDetails:     fmt.Sprintf("current: %d concrete: %d", currentExec.CloseStatus, concreteWorkflow.State.ExecutionInfo.CloseStatus),
		}
	}

	return CheckResult{
		CheckResultType: CheckResultTypeHealthy,
		InvariantName:   m.Name(),
	}
}

func (m *mismatchedRecords) Fix(
	ctx context.Context,
	execution interface{},
) FixResult {
	if fixResult := validateFixContext(ctx, m.Name()); fixResult != nil {
		return *fixResult
	}

	fixResult, checkResult := checkBeforeFix(ctx, m, execution)
	if fixResult != nil {
		return *fixResult
	}
	fixResult = DeleteExecution(ctx, execution, m.pr, m.dc)
	fixResult.CheckResult = *checkResult
	fixResult.InvariantName = m.Name()
	return *fixResult
}

func (m *mismatchedRecords) Name() Name {
	return MismatchedRecords
}
