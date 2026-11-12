// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package batcher

import (
	"fmt"

	"github.com/uber/cadence/common"
)

func ValidateParams(params BatchParams) error {
	if params.BatchType == "" ||
		params.Reason == "" ||
		params.DomainName == "" ||
		params.Query == "" {
		return fmt.Errorf("must provide required parameters: BatchType/Reason/DomainName/Query")
	}
	switch params.BatchType {
	case BatchTypeSignal:
		if params.SignalParams.SignalName == "" {
			return fmt.Errorf("must provide signal name")
		}
		return nil
	case BatchTypeReplicate:
		if params.ReplicateParams.SourceCluster == "" {
			return fmt.Errorf("must provide source cluster")
		}
		if params.ReplicateParams.TargetCluster == "" {
			return fmt.Errorf("must provide target cluster")
		}
		return nil
	case BatchTypeCancel:
		fallthrough
	case BatchTypeTerminate:
		fallthrough
	case BatchTypeRefresh:
		return nil
	default:
		return fmt.Errorf("not supported batch type: %v", params.BatchType)
	}
}

// NormalizeOperation derives BatchType and the matching legacy Xxx Params
// field from params.Operation when set, so the rest of this package can keep
// switching on BatchType regardless of whether the caller is the old CLI
// flow (which already sets BatchType/Xxx Params directly) or the newer
// StartBatchOperation handler (which only sets Operation). Exported so the
// frontend handler can validate a request against the same rules the
// workflow itself enforces before starting it.
func NormalizeOperation(params BatchParams) BatchParams {
	if params.Operation == nil {
		return params
	}
	params.BatchType = BatchTypeFromOperation(params.Operation)
	switch params.BatchType {
	case BatchTypeTerminate:
		if t := params.Operation.GetTerminateBatchOperation(); t != nil {
			params.TerminateParams = TerminateParams{TerminateChildren: t.TerminateChildren}
		}
	case BatchTypeCancel:
		if c := params.Operation.GetCancelBatchOperation(); c != nil {
			params.CancelParams = CancelParams{CancelChildren: c.CancelChildren}
		}
	case BatchTypeSignal:
		if s := params.Operation.GetSignalBatchOperation(); s != nil {
			params.SignalParams = SignalParams{SignalName: s.SignalName, Input: string(s.Input)}
		}
	case BatchTypeReplicate:
		if r := params.Operation.GetReplicateBatchOperation(); r != nil {
			params.ReplicateParams = ReplicateParams{SourceCluster: r.SourceCluster, TargetCluster: r.TargetCluster}
		}
	case BatchTypeRefresh:
		// RefreshBatchOperation carries no fields; BatchType alone drives the workflow.
	}
	return params
}

func setDefaultParams(params BatchParams) BatchParams {
	params = NormalizeOperation(params)
	if params.RPS <= 0 {
		params.RPS = DefaultRPS
	}
	if params.Concurrency <= 0 {
		params.Concurrency = DefaultConcurrency
	}
	if params.PageSize <= 0 {
		params.PageSize = DefaultPageSize
	}
	if params.AttemptsOnRetryableError <= 0 {
		params.AttemptsOnRetryableError = DefaultAttemptsOnRetryableError
	}
	if params.ActivityHeartBeatTimeout <= 0 {
		params.ActivityHeartBeatTimeout = DefaultActivityHeartBeatTimeout
	}
	if len(params.NonRetryableErrors) > 0 {
		params._nonRetryableErrors = make(map[string]struct{}, len(params.NonRetryableErrors))
		for _, estr := range params.NonRetryableErrors {
			params._nonRetryableErrors[estr] = struct{}{}
		}
	}
	if params.TerminateParams.TerminateChildren == nil {
		params.TerminateParams.TerminateChildren = common.BoolPtr(true)
	}
	if params.MaxActivityRetries < 0 {
		params.MaxActivityRetries = DefaultMaxActivityRetries
	}
	return params
}
