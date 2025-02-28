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

package testdata

import (
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

type WFExecRequestOption func(*nosqlplugin.WorkflowExecutionRequest)

func WFExecRequestWithMapsWriteMode(mode nosqlplugin.WorkflowExecutionMapsWriteMode) WFExecRequestOption {
	return func(request *nosqlplugin.WorkflowExecutionRequest) {
		request.MapsWriteMode = mode
	}
}

func WFExecRequestWithEventBufferWriteMode(mode nosqlplugin.EventBufferWriteMode) WFExecRequestOption {
	return func(request *nosqlplugin.WorkflowExecutionRequest) {
		request.EventBufferWriteMode = mode
	}
}

func WFExecRequest(opts ...WFExecRequestOption) *nosqlplugin.WorkflowExecutionRequest {
	ts := time.Now()
	req := &nosqlplugin.WorkflowExecutionRequest{
		InternalWorkflowExecutionInfo: persistence.InternalWorkflowExecutionInfo{
			DomainID:   "test-domain-id",
			WorkflowID: "test-workflow-id",
			CompletionEvent: &persistence.DataBlob{
				Encoding: common.EncodingTypeThriftRW,
				Data:     []byte("test-completion-event"),
			},
			AutoResetPoints: &persistence.DataBlob{
				Encoding: common.EncodingTypeThriftRW,
				Data:     []byte("test-auto-reset-points"),
			},
		},
		VersionHistories: &persistence.DataBlob{
			Encoding: common.EncodingTypeThriftRW,
			Data:     []byte("test-version-histories"),
		},
		Checksums: &checksum.Checksum{
			Version: 1,
			Flavor:  checksum.FlavorIEEECRC32OverThriftBinary,
			Value:   []byte("test-checksum"),
		},
		PreviousNextEventIDCondition: common.Int64Ptr(123),
		CurrentTimeStamp:             ts,
	}

	for _, opt := range opts {
		opt(req)
	}

	return req
}
