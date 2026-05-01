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
)

type (
	inactiveDomainExists struct {
		pr persistence.Retryer
		dc cache.DomainCache
	}
)

func NewInactiveDomainExists(
	pr persistence.Retryer, dc cache.DomainCache,
) Invariant {
	return &inactiveDomainExists{
		pr: pr,
		dc: dc,
	}
}

func (idc *inactiveDomainExists) Check(
	ctx context.Context,
	execution interface{},
) CheckResult {
	if checkResult := validateCheckContext(ctx, idc.Name()); checkResult != nil {
		return *checkResult
	}

	concreteExecution, ok := execution.(*entity.ConcreteExecution)
	if !ok {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   idc.Name(),
			Info:            "failed to check: expected concrete execution",
		}
	}
	domainID := concreteExecution.GetDomainID()
	domain, err := idc.dc.GetDomainByID(domainID)
	if err != nil {
		return CheckResult{
			CheckResultType: CheckResultTypeFailed,
			InvariantName:   idc.Name(),
			Info:            "failed to check: expected Domain",
			InfoDetails:     err.Error(),
		}
	}
	if domain.GetInfo().Status == persistence.DomainStatusDeprecated || domain.GetInfo().Status == persistence.DomainStatusDeleted {
		return CheckResult{
			CheckResultType: CheckResultTypeCorrupted,
			InvariantName:   idc.Name(),
			Info:            "failed check: domain is not active",
			InfoDetails:     "The domain has been deprecated or deleted",
		}
	}

	return CheckResult{
		CheckResultType: CheckResultTypeHealthy,
		InvariantName:   idc.Name(),
		Info:            "workflow's domain is active",
		InfoDetails:     "workflow's domain is active",
	}
}

func (idc *inactiveDomainExists) Fix(
	ctx context.Context,
	execution interface{},
) FixResult {
	if fixResult := validateFixContext(ctx, idc.Name()); fixResult != nil {
		return *fixResult
	}

	fixResult, checkResult := checkBeforeFix(ctx, idc, execution)
	if fixResult != nil {
		return *fixResult
	}
	fixResult = DeleteExecution(ctx, execution, idc.pr, idc.dc)
	fixResult.CheckResult = *checkResult
	fixResult.InvariantName = idc.Name()
	return *fixResult
}

func (idc *inactiveDomainExists) Name() Name {
	return InactiveDomainExists
}
