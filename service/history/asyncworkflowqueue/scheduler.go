package asyncworkflowqueue

import (
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/task"
)

// Scheduler is the host-level FIFO executor of async workflow queue tasks.
type Scheduler interface {
	common.Daemon
	Submit(t task.Task) error // blocks until accepted or stopped
}

// NewScheduler creates a Scheduler backed by a parallel task processor. Tasks never ask the
// processor to retry (see Task.RetryErr), so the retry policy only needs to be non-nil;
// all retrying of transient errors happens inside Task.Execute.
func NewScheduler(
	logger log.Logger,
	metricsClient metrics.Client,
	workerCount dynamicproperties.IntPropertyFn,
	queueSize dynamicproperties.IntPropertyFn,
) Scheduler {
	policy := backoff.NewExponentialRetryPolicy(time.Millisecond)
	policy.SetMaximumAttempts(1)

	return task.NewParallelTaskProcessor(logger, metricsClient, &task.ParallelTaskProcessorOptions{
		QueueSize:   queueSize(),
		WorkerCount: workerCount,
		RetryPolicy: policy,
	})
}
