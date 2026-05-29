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

package types

import (
	"go.uber.org/zap/zapcore"
)

// EventAlreadyStartedError is an internal type (TBD...)
type EventAlreadyStartedError struct {
	Message string `json:"message,required"`
}

func (err AccessDeniedError) Error() string {
	return err.Message
}

func (err BadRequestError) Error() string {
	return err.Message
}

func (err CancellationAlreadyRequestedError) Error() string {
	return err.Message
}

func (err ClientVersionNotSupportedError) Error() string {
	return "client version not supported"
}

func (err ClientVersionNotSupportedError) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("feature-version", err.FeatureVersion)
	enc.AddString("client-implementation", err.ClientImpl)
	enc.AddString("supported-versions", err.SupportedVersions)
	return nil
}

func (err FeatureNotEnabledError) Error() string {
	return "feature not enabled"
}

func (err FeatureNotEnabledError) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("feature-flag", err.FeatureFlag)
	return nil
}

func (err CurrentBranchChangedError) Error() string {
	return err.Message
}

func (err CurrentBranchChangedError) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("wf-branch-token", string(err.CurrentBranchToken))
	return nil
}

func (err DomainAlreadyExistsError) Error() string {
	return err.Message
}

func (err DomainNotActiveError) Error() string {
	return err.Message
}

func (err DomainNotActiveError) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("domain-name", err.DomainName)
	enc.AddString("current-cluster", err.CurrentCluster)
	enc.AddString("active-cluster", err.ActiveCluster)
	return nil
}

func (err EntityNotExistsError) Error() string {
	return err.Message
}

func (err WorkflowExecutionAlreadyCompletedError) Error() string {
	return err.Message
}

func (err InternalDataInconsistencyError) Error() string {
	return err.Message
}

func (err InternalServiceError) Error() string {
	return err.Message
}

func (err LimitExceededError) Error() string {
	return err.Message
}

func (err QueryFailedError) Error() string {
	return err.Message
}

func (err RemoteSyncMatchedError) Error() string {
	return err.Message
}

func (err RetryTaskV2Error) Error() string {
	return err.Message
}

func (err RetryTaskV2Error) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("domain-id", err.DomainID)
	enc.AddString("workflow-id", err.WorkflowID)
	enc.AddString("run-id", err.RunID)
	if err.StartEventID != nil {
		enc.AddInt64("start-event-id", *err.StartEventID)
	}
	if err.StartEventVersion != nil {
		enc.AddInt64("start-event-version", *err.StartEventVersion)
	}
	if err.EndEventID != nil {
		enc.AddInt64("end-event-id", *err.EndEventID)
	}
	if err.EndEventVersion != nil {
		enc.AddInt64("end-event-version", *err.EndEventVersion)
	}
	return nil
}

func (err ServiceBusyError) Error() string {
	return err.Message
}

func (err WorkflowExecutionAlreadyStartedError) Error() string {
	return err.Message
}

func (err WorkflowExecutionAlreadyStartedError) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("start-request-id", err.StartRequestID)
	enc.AddString("run-id", err.RunID)
	return nil
}

func (err ShardOwnershipLostError) Error() string {
	return err.Message
}

func (err ShardOwnershipLostError) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("shard-owner", err.Owner)
	return nil
}

func (err EventAlreadyStartedError) Error() string {
	return err.Message
}

func (err StickyWorkerUnavailableError) Error() string {
	return err.Message
}

func (err ReadOnlyPartitionError) Error() string {
	return err.Message
}
