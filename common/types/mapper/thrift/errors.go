// Copyright (c) 2020 Uber Technologies Inc.
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

package thrift

import (
	"github.com/uber/cadence/common/types/mapper/errorutils"
)

// FromError convert error to Thrift type if it comes as its internal equivalent
func FromError(err error) error {
	if err == nil {
		return nil
	}

	var (
		ok       bool
		typedErr error
	)
	if ok, typedErr = errorutils.ConvertError(err, FromAccessDeniedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromBadRequestError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromCancellationAlreadyRequestedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromClientVersionNotSupportedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromFeatureNotEnabledError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromCurrentBranchChangedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromDomainAlreadyExistsError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromDomainNotActiveError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromEntityNotExistsError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromWorkflowExecutionAlreadyCompletedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromInternalDataInconsistencyError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromInternalServiceError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromLimitExceededError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromQueryFailedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromRemoteSyncMatchedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromRetryTaskV2Error); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromServiceBusyError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromWorkflowExecutionAlreadyStartedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromShardOwnershipLostError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromEventAlreadyStartedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromStickyWorkerUnavailableError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, FromTaskListNotOwnedByHostError); ok {
		return typedErr
	}

	return err
}

// ToError convert error to internal type if it comes as its thrift equivalent
func ToError(err error) error {
	if err == nil {
		return nil
	}

	var (
		ok       bool
		typedErr error
	)
	if ok, typedErr = errorutils.ConvertError(err, ToAccessDeniedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToBadRequestError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToCancellationAlreadyRequestedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToClientVersionNotSupportedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToFeatureNotEnabledError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToCurrentBranchChangedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToDomainAlreadyExistsError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToDomainNotActiveError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToEntityNotExistsError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToWorkflowExecutionAlreadyCompletedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToInternalDataInconsistencyError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToInternalServiceError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToLimitExceededError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToQueryFailedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToRemoteSyncMatchedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToRetryTaskV2Error); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToServiceBusyError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToWorkflowExecutionAlreadyStartedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToShardOwnershipLostError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToEventAlreadyStartedError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToStickyWorkerUnavailableError); ok {
		return typedErr
	} else if ok, typedErr = errorutils.ConvertError(err, ToTaskListNotOwnedByHostError); ok {
		return typedErr
	}

	return err
}
