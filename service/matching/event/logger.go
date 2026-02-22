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

package event

import (
	"time"

	"go.uber.org/zap"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type E struct {
	persistence.TaskInfo
	TaskListName string
	TaskListKind *types.TaskListKind
	TaskListType int // persistence.TaskListTypeDecision or persistence.TaskListTypeActivity

	EventTime time.Time

	// EventName describes the event. It is used to query events in simulations so don't change existing event names.
	EventName string
	Host      string
	Payload   map[string]any
}

func Log(events ...E) {
	for _, e := range events {
		zap.L().Debug(e.EventName, zap.Any("event", e),
			zap.String("taskListName", e.TaskListName),
			zap.Stringer("taskListKind", e.TaskListKind),
			zap.Int("taskListType", e.TaskListType),
			zap.Time("eventTime", e.EventTime),
			zap.String("wf-domain-id", e.DomainID),
			zap.String("wf-id", e.WorkflowID),
			zap.String("wf-run-id", e.RunID),
			zap.Any("event", e))
	}
}
