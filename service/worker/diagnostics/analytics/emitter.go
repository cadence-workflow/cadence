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

package analytics

import (
	"context"
	"encoding/json"

	"github.com/uber/cadence/.gen/go/indexer"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/messaging"
)

type Emitter DataEmitter

type emitter struct {
	producer messaging.Producer
}

type EmitterParams struct {
	Producer messaging.Producer
}

func NewEmitter(p EmitterParams) DataEmitter {
	return &emitter{
		producer: p.Producer,
	}
}

func (et *emitter) EmitUsageData(ctx context.Context, data WfDiagnosticsUsageData) error {
	msg := make(map[string]interface{})
	msg[Domain] = data.Domain
	msg[WorkflowID] = data.WorkflowID
	msg[RunID] = data.RunID
	msg[Identity] = data.Identity
	msg[SatisfactionFeedback] = data.SatisfactionFeedback
	msg[IssueType] = data.IssueType
	msg[DiagnosticsWfID] = data.DiagnosticsWorkflowID
	msg[DiagnosticsWfRunID] = data.DiagnosticsRunID
	msg[Environment] = data.Environment
	msg[DiagnosticsStartTime] = data.DiagnosticsStartTime
	msg[DiagnosticsEndTime] = data.DiagnosticsEndTime

	serializedMsg, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	pinotMsg := &indexer.PinotMessage{
		WorkflowID: common.StringPtr(data.DiagnosticsWorkflowID),
		Payload:    serializedMsg,
	}
	return et.producer.Publish(ctx, pinotMsg)
}
