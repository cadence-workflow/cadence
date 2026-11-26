// Copyright (c) 2017-2021 Uber Technologies Inc.
// Portions of the Software are attributed to Copyright (c) 2021 Temporal Technologies Inc.
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

package engineimpl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/engine/testdata"
)

func TestHistoryEngineStartStop(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)

	eft.Engine.Start()
	eft.Engine.Stop()
}

func TestNewEngineSkipsDisabledQueue(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })

	eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)

	queueProcessors := eft.Engine.(*historyEngineImpl).queueProcessors
	assert.Contains(t, queueProcessors, persistence.HistoryTaskCategoryTransfer)
	assert.Contains(t, queueProcessors, persistence.HistoryTaskCategoryTimer)
	assert.NotContains(t, queueProcessors, persistence.HistoryTaskCategoryVisibility)

	// start and stop only the queues that were created
	eft.Engine.Start()
	eft.Engine.Stop()
}
