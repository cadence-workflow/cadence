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

package asyncworkflowqueue

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/asyncworkflow/queue/consumer"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/task"
	"github.com/uber/cadence/service/history/config"
)

func newTestTaskScheduler(t *testing.T, workerCount int) TaskScheduler {
	t.Helper()

	cfg := config.NewForTest()
	cfg.AsyncWorkflowTaskWorkerCount = dynamicproperties.GetIntPropertyFn(workerCount)
	cfg.AsyncWorkflowTaskSchedulerBufferSize = dynamicproperties.GetIntPropertyFn(100)
	cfg.AsyncWorkflowConsumerDomainRPS = dynamicproperties.GetIntPropertyFilteredByDomain(1000)
	cfg.AsyncWorkflowConsumerDomainWeight = dynamicproperties.GetIntPropertyFilteredByDomain(100)

	scheduler, err := NewTaskScheduler(cfg, testlogger.New(t), metrics.NewNoopMetricsClient(), clock.NewRealTimeSource())
	require.NoError(t, err)
	return scheduler
}

// newSchedulerTestTask builds a consumerTask whose Invoke records the execution
// and whose ack manager tracks completion. The mock queue manager has no DLQ
// expectations, so any DLQ write fails the test.
func newSchedulerTestTask(
	t *testing.T,
	ctrl *gomock.Controller,
	domain string,
	messageID int64,
	ackMgr messaging.AckManager,
	invoke func(ctx context.Context) (string, error),
) *consumerTask {
	t.Helper()
	require.NoError(t, ackMgr.ReadItem(messageID))
	return newConsumerTask(
		context.Background(),
		&consumer.PreparedRequest{
			RequestType: "StartWorkflowExecutionAsyncRequest",
			Domain:      domain,
			WorkflowID:  fmt.Sprintf("wf-%s-%d", domain, messageID),
			Invoke:      invoke,
		},
		&persistence.AsyncWorkflowMessage{
			ShardID:   testShardID,
			MessageID: messageID,
			Payload:   []byte("payload"),
		},
		persistence.NewMockAsyncWorkflowQueueManager(ctrl),
		ackMgr,
		clock.NewMockedTimeSource(),
		testlogger.New(t),
		metrics.NewNoopMetricsClient().Scope(metrics.AsyncWorkflowConsumerScope),
	)
}

func TestTaskSchedulerProcessesTasksAcrossDomains(t *testing.T) {
	ctrl := gomock.NewController(t)
	scheduler := newTestTaskScheduler(t, 2)
	scheduler.Start()
	defer scheduler.Stop()

	const perDomain = 5
	domains := []string{"domain-a", "domain-b"}

	var mu sync.Mutex
	executed := make(map[string]int)
	done := make(chan struct{}, len(domains)*perDomain)
	invoke := func(domain string) func(ctx context.Context) (string, error) {
		return func(ctx context.Context) (string, error) {
			mu.Lock()
			executed[domain]++
			mu.Unlock()
			done <- struct{}{}
			return "run-id", nil
		}
	}

	ackMgrs := make(map[string]messaging.AckManager)
	for _, domain := range domains {
		ackMgrs[domain] = messaging.NewContinuousAckManager(testlogger.New(t))
		for i := 0; i < perDomain; i++ {
			ct := newSchedulerTestTask(t, ctrl, domain, int64(i+1), ackMgrs[domain], invoke(domain))
			require.NoError(t, scheduler.Submit(ct))
		}
	}

	for i := 0; i < len(domains)*perDomain; i++ {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for tasks to execute")
		}
	}

	mu.Lock()
	defer mu.Unlock()
	for _, domain := range domains {
		assert.Equal(t, perDomain, executed[domain], "all tasks for %s should have executed", domain)
	}

	// All tasks succeeded, so every ack level must have advanced to the last
	// message; poll briefly since Ack happens after Invoke returns.
	assert.Eventually(t, func() bool {
		for _, domain := range domains {
			if ackMgrs[domain].GetAckLevel() != int64(perDomain) {
				return false
			}
		}
		return true
	}, 10*time.Second, 10*time.Millisecond, "ack levels should reach the last message ID")
}

func TestTaskSchedulerStopDrainsWithoutDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	scheduler := newTestTaskScheduler(t, 1)
	scheduler.Start()

	// Occupy the single worker so subsequently submitted tasks stay queued.
	blockerStarted := make(chan struct{})
	blockerRelease := make(chan struct{})
	blockerAckMgr := messaging.NewContinuousAckManager(testlogger.New(t))
	blocker := newSchedulerTestTask(t, ctrl, "domain-a", 1, blockerAckMgr, func(ctx context.Context) (string, error) {
		close(blockerStarted)
		<-blockerRelease
		return "run-id", nil
	})
	require.NoError(t, scheduler.Submit(blocker))

	select {
	case <-blockerStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for blocker task to start")
	}

	// These stay queued behind the blocked worker. No DLQ expectations are set
	// on their mock managers: a DLQ write during drain would fail the test.
	queuedAckMgr := messaging.NewContinuousAckManager(testlogger.New(t))
	queued := make([]*consumerTask, 0, 3)
	for i := 0; i < 3; i++ {
		ct := newSchedulerTestTask(t, ctrl, "domain-b", int64(i+1), queuedAckMgr, func(ctx context.Context) (string, error) {
			return "run-id", nil
		})
		require.NoError(t, scheduler.Submit(ct))
		queued = append(queued, ct)
	}

	close(blockerRelease)
	scheduler.Stop()

	// Which queued tasks executed before the drain is timing-dependent, but the
	// invariants are not: every task must be resolved (executed-and-acked or
	// dropped for redelivery), and none may have been written to the DLQ (the
	// strict mock manager has no EnqueueToDLQ expectation).
	for _, ct := range queued {
		assert.NotEqual(t, task.TaskStatePending, ct.State(), "queued tasks should be resolved on stop")
	}
}
