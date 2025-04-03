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

package validation

import (
	"context"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

// IsValidIDLength checks if id is valid according to its length
func IsValidIDLength(
	id string,
	scope metrics.Scope,
	warnLimit int,
	errorLimit int,
	metricsCounter int,
	domainName string,
	logger log.Logger,
	idTypeViolationTag tag.Tag,
) bool {
	if len(id) > warnLimit {
		scope.IncCounter(metricsCounter)
		logger.Warn("ID length exceeds limit.",
			tag.WorkflowDomainName(domainName),
			tag.Name(id),
			idTypeViolationTag)
	}
	return len(id) <= errorLimit
}

// CheckDecisionResultLimit checks if decision result count exceeds limits.
func CheckDecisionResultLimit(
	actualSize int,
	limit int,
	scope metrics.Scope,
) error {
	scope.RecordTimer(metrics.DecisionResultCount, time.Duration(actualSize))
	if limit > 0 && actualSize > limit {
		return common.ErrDecisionResultCountTooLarge
	}
	return nil
}

// ValidateRetryPolicy validates a retry policy
func ValidateRetryPolicy(policy *types.RetryPolicy) error {
	if policy == nil {
		// nil policy is valid which means no retry
		return nil
	}
	if policy.GetInitialIntervalInSeconds() <= 0 {
		return &types.BadRequestError{Message: "InitialIntervalInSeconds must be greater than 0 on retry policy."}
	}
	if policy.GetBackoffCoefficient() < 1 {
		return &types.BadRequestError{Message: "BackoffCoefficient cannot be less than 1 on retry policy."}
	}
	if policy.GetMaximumIntervalInSeconds() < 0 {
		return &types.BadRequestError{Message: "MaximumIntervalInSeconds cannot be less than 0 on retry policy."}
	}
	if policy.GetMaximumIntervalInSeconds() > 0 && policy.GetMaximumIntervalInSeconds() < policy.GetInitialIntervalInSeconds() {
		return &types.BadRequestError{Message: "MaximumIntervalInSeconds cannot be less than InitialIntervalInSeconds on retry policy."}
	}
	if policy.GetMaximumAttempts() < 0 {
		return &types.BadRequestError{Message: "MaximumAttempts cannot be less than 0 on retry policy."}
	}
	if policy.GetExpirationIntervalInSeconds() < 0 {
		return &types.BadRequestError{Message: "ExpirationIntervalInSeconds cannot be less than 0 on retry policy."}
	}
	if policy.GetMaximumAttempts() == 0 && policy.GetExpirationIntervalInSeconds() == 0 {
		return &types.BadRequestError{Message: "MaximumAttempts and ExpirationIntervalInSeconds are both 0. At least one of them must be specified."}
	}
	return nil
}

// ValidateLongPollContextTimeout check if the context timeout for a long poll handler is too short or below a normal value.
// If the timeout is not set or too short, it logs an error, and return ErrContextTimeoutNotSet or ErrContextTimeoutTooShort
// accordingly. If the timeout is only below a normal value, it just logs an info and return nil.
func ValidateLongPollContextTimeout(
	ctx context.Context,
	handlerName string,
	logger log.Logger,
) error {

	deadline, err := ValidateLongPollContextTimeoutIsSet(ctx, handlerName, logger)
	if err != nil {
		return err
	}
	timeout := time.Until(deadline)
	if timeout < constants.MinLongPollTimeout {
		err := common.ErrContextTimeoutTooShort
		logger.Error("Context timeout is too short for long poll API.",
			tag.WorkflowHandlerName(handlerName), tag.Error(err), tag.WorkflowPollContextTimeout(timeout))
		return err
	}
	if timeout < constants.CriticalLongPollTimeout {
		logger.Debug("Context timeout is lower than critical value for long poll API.",
			tag.WorkflowHandlerName(handlerName), tag.WorkflowPollContextTimeout(timeout))
	}
	return nil
}

// ValidateLongPollContextTimeoutIsSet checks if the context timeout is set for long poll requests.
func ValidateLongPollContextTimeoutIsSet(
	ctx context.Context,
	handlerName string,
	logger log.Logger,
) (time.Time, error) {

	deadline, ok := ctx.Deadline()
	if !ok {
		err := common.ErrContextTimeoutNotSet
		logger.Error("Context timeout not set for long poll API.",
			tag.WorkflowHandlerName(handlerName), tag.Error(err))
		return deadline, err
	}
	return deadline, nil
}

// ValidateDomainUUID checks if the given domainID string is a valid UUID
func ValidateDomainUUID(
	domainUUID string,
) error {

	if domainUUID == "" {
		return &types.BadRequestError{Message: "Missing domain UUID."}
	} else if uuid.Parse(domainUUID) == nil {
		return &types.BadRequestError{Message: "Invalid domain UUID."}
	}
	return nil
}

// CheckEventBlobSizeLimit checks if a blob data exceeds limits. It logs a warning if it exceeds warnLimit,
// and return ErrBlobSizeExceedsLimit if it exceeds errorLimit.
func CheckEventBlobSizeLimit(
	actualSize int,
	warnLimit int,
	errorLimit int,
	domainID string,
	domainName string,
	workflowID string,
	runID string,
	scope metrics.Scope,
	logger log.Logger,
	blobSizeViolationOperationTag tag.Tag,
) error {

	scope.RecordTimer(metrics.EventBlobSize, time.Duration(actualSize))

	if errorLimit < warnLimit {
		logger.Warn("Error limit is less than warn limit.", tag.WorkflowDomainName(domainName), tag.WorkflowDomainID(domainID))

		warnLimit = errorLimit
	}

	if actualSize <= warnLimit {
		return nil
	}

	tags := []tag.Tag{
		tag.WorkflowDomainName(domainName),
		tag.WorkflowDomainID(domainID),
		tag.WorkflowID(workflowID),
		tag.WorkflowRunID(runID),
		tag.WorkflowSize(int64(actualSize)),
		blobSizeViolationOperationTag,
	}

	if actualSize <= errorLimit {
		logger.Warn("Blob size close to the limit.", tags...)

		return nil
	}

	scope.Tagged(metrics.DomainTag(domainName)).IncCounter(metrics.EventBlobSizeExceedLimit)

	logger.Error("Blob size exceeds limit.", tags...)

	return common.ErrBlobSizeExceedsLimit
}
