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

package matching

import (
	"sync"
	"testing"

	"github.com/uber/cadence/common/mocks"

	log "github.com/sirupsen/logrus"
	"github.com/uber-common/bark"
	"github.com/uber/cadence/common/persistence"
)

func TestDeliverBufferTasks(t *testing.T) {
	tests := []func(tlm *taskListManagerImpl){
		func(tlm *taskListManagerImpl) { close(tlm.taskBuffer) },
		func(tlm *taskListManagerImpl) { close(tlm.deliverBufferShutdownCh) },
		func(tlm *taskListManagerImpl) { tlm.cancelFunc() },
	}
	for _, test := range tests {
		tlm := createTestTaskListManager()
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			tlm.deliverBufferTasksForPoll()
		}()
		test(tlm)
		// deliverBufferTasksForPoll should stop after invokation of the test function
		wg.Wait()
	}
}

func createTestTaskListManager() *taskListManagerImpl {
	logger := bark.NewLoggerFromLogrus(log.New())
	tm := newTestTaskManager(logger)
	cfg := defaultTestConfig()
	me := newMatchingEngine(
		cfg, tm, &mocks.HistoryClient{}, logger,
	)
	tl := "tl"
	dID := "domain"
	tlID := &taskListID{domainID: dID, taskListName: tl, taskType: persistence.TaskListTypeActivity}
	return newTaskListManager(me, tlID, cfg).(*taskListManagerImpl)
}
