package semaphore

import (
	"context"
	"errors"
	"time"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	commonsemaphore "github.com/uber/cadence/common/semaphore"
	"github.com/uber/cadence/common/types"
)

const (
	taskCallTimeout             = 10 * time.Second
	taskRetryInitialInterval    = 100 * time.Millisecond
	taskRetryMaxInterval        = 10 * time.Second
	taskHistoryRetryMaxInterval = time.Minute
	// A retry loop logs its failures as critical after the first taskCriticalRetryCount.
	taskCriticalRetryCount = 10
	// taskReleaseTimeout is how long releaseTaskToken retries a release before it gives up.
	taskReleaseTimeout = 5 * time.Minute
	// historyCallConcurrency is the most History calls one bucket makes at once.
	historyCallConcurrency = 10
)

// taskRetrier backs off one retry loop of the grant loop and counts its failures.
type taskRetrier struct {
	retrier  backoff.Retrier
	attempts int
}

// signalGrantLoop wakes dispatchBufferedTasks when it waits for a free token. It never blocks: if a
// signal is already pending, this one is dropped.
func (m *semaphoreManagerImpl) signalGrantLoop() {
	select {
	case m.grantNotifyC <- struct{}{}:
	default: // a signal is already waiting
	}
}

// ownerIDFromTask returns the owner id of task's hold.
func ownerIDFromTask(task *persistence.SemaphoreTask) string {
	return commonsemaphore.Owner{WorkflowID: task.WorkflowID, RunID: task.RunID, HoldID: task.HoldID}.String()
}

// grantTokenToTask grants a waiting task a token once one is free. It returns NoSemaphoreTokenID if
// the task expired and holds none, and false only if ctx ends first. Each pass:
//   - The task expired: check whether it was already granted a token.
//   - Otherwise: try to grant a token. If none is free, wait for a signal or the deadline, then
//     try again.
//
// A failed call backs off and tries again.
func (m *semaphoreManagerImpl) grantTokenToTask(ctx context.Context, task *persistence.SemaphoreTask) (int, bool) {
	ownerID := ownerIDFromTask(task)
	retrier := m.newTaskRetrier(taskRetryMaxInterval)
	for {
		if m.isTaskExpired(task) {
			// An expired task gets no new token. But its owner may hold one granted before the
			// deadline, if the bucket stopped before History answered. Return it, so History keeps
			// it if it recorded the grant, or refuses it and the token is released.
			tokenID, err := m.findHeldToken(ctx, ownerID)
			if err == nil {
				return tokenID, true
			}
			if !m.waitToRetry(ctx, retrier, task, "Semaphore grant loop failed to look up an expired task's token", err) {
				return constants.NoSemaphoreTokenID, false
			}
			continue
		}

		callCtx, cancel := context.WithTimeout(ctx, taskCallTimeout)
		tokenID, err := m.grant(callCtx, ownerID)
		cancel()
		if err == nil && tokenID != constants.NoSemaphoreTokenID {
			return tokenID, true
		}
		// No token yet. What to wait for depends on why.
		var waited bool
		switch {
		case err == nil:
			// No free token: wait for one to free, or for the deadline.
			waited = m.waitForGrantSignal(ctx, task.AcquireDeadline)
		case errors.Is(err, errStaleCache):
			// Wait until the bucket can reload again.
			reloadAt := m.timeSource.Now().Add(minBucketReloadInterval)
			waited = m.waitForGrantSignal(ctx, &reloadAt)
		default:
			waited = m.waitToRetry(ctx, retrier, task, "Semaphore grant loop failed to grant a task", err)
		}
		if !waited {
			return constants.NoSemaphoreTokenID, false
		}
	}
}

func (m *semaphoreManagerImpl) isTaskExpired(task *persistence.SemaphoreTask) bool {
	return task.AcquireDeadline != nil && !m.timeSource.Now().Before(*task.AcquireDeadline)
}

// findHeldToken returns the token ownerID holds, or NoSemaphoreTokenID. It first checks the cached
// token against its token row. If that finds none, it reads the owner row, since the cache can miss
// a grant, such as a failed write that landed.
func (m *semaphoreManagerImpl) findHeldToken(ctx context.Context, ownerID string) (int, error) {
	callCtx, cancel := context.WithTimeout(ctx, taskCallTimeout)
	defer cancel()
	tokenID, err := m.confirmHeldToken(callCtx, ownerID)
	if err != nil {
		return constants.NoSemaphoreTokenID, err
	}
	if tokenID != constants.NoSemaphoreTokenID {
		return tokenID, nil
	}
	return m.readOwnerRowToken(callCtx, m.currentCache(), ownerID)
}

// recordSemaphoreAcquired records in History that task was granted tokenID. If History refuses the
// grant, the token is released; any other error is retried. It returns true once the task is done,
// and false if ctx ends first.
func (m *semaphoreManagerImpl) recordSemaphoreAcquired(ctx context.Context, task *persistence.SemaphoreTask, tokenID int) bool {
	ownerID := ownerIDFromTask(task)
	request := &types.RecordSemaphoreAcquiredRequest{
		DomainUUID:        m.id.DomainID,
		WorkflowExecution: &types.WorkflowExecution{WorkflowID: task.WorkflowID, RunID: task.RunID},
		InitiatedID:       task.HoldID,
		TokenID:           int32(tokenID),
		SemaphoreName:     m.id.SemaphoreName,
	}
	retrier := m.newTaskRetrier(taskHistoryRetryMaxInterval)
	for {
		m.liveness.MarkAlive()
		err := m.callHistory(ctx, func(callCtx context.Context) error {
			_, err := m.historyService.RecordSemaphoreAcquired(callCtx, request)
			return err
		})
		switch {
		case err == nil:
			return true
		case errors.As(err, new(*types.EntityNotExistsError)),
			errors.As(err, new(*types.WorkflowExecutionAlreadyCompletedError)),
			errors.As(err, new(*types.EventAlreadyStartedError)):
			// The hold is gone, the run closed, or the hold has another token.
			return m.releaseTaskToken(ctx, task, ownerID, tokenID)
		case errors.As(err, new(*types.DomainNotActiveError)):
			// The domain failed over. Free the token, then refresh the run's tasks so this cluster
			// acquires again if the domain fails back. Freeing comes first: otherwise the refreshed
			// acquire could be given this token and then lose it to the release.
			if !m.releaseTaskToken(ctx, task, ownerID, tokenID) {
				return false
			}
			return m.refreshWorkflowTasks(ctx, task)
		}
		if !m.waitToRetry(ctx, retrier, task, "Semaphore grant loop failed to record a grant", err) {
			return false
		}
	}
}

// refreshWorkflowTasks asks History to rebuild the run's tasks, retrying until it succeeds or the
// run is gone. It returns false only if ctx ends first.
func (m *semaphoreManagerImpl) refreshWorkflowTasks(ctx context.Context, task *persistence.SemaphoreTask) bool {
	request := &types.HistoryRefreshWorkflowTasksRequest{
		DomainUIID: m.id.DomainID,
		Request: &types.RefreshWorkflowTasksRequest{
			Execution: &types.WorkflowExecution{WorkflowID: task.WorkflowID, RunID: task.RunID},
		},
	}
	// No retry limit: giving up would leave this cluster with no acquire task for the run if the
	// domain fails back.
	retrier := m.newTaskRetrier(taskHistoryRetryMaxInterval)
	for {
		err := m.callHistory(ctx, func(callCtx context.Context) error {
			return m.historyService.RefreshWorkflowTasks(callCtx, request)
		})
		if err == nil || errors.As(err, new(*types.EntityNotExistsError)) {
			return true
		}
		if !m.waitToRetry(ctx, retrier, task, "Semaphore grant loop failed to refresh a run's tasks", err) {
			return false
		}
	}
}

// callHistory makes one History call while holding one of historyCallConcurrency places, so a call
// that keeps failing holds none while it waits to retry.
func (m *semaphoreManagerImpl) callHistory(ctx context.Context, call func(ctx context.Context) error) error {
	select {
	case m.historyCallLimiter <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-m.historyCallLimiter }()
	callCtx, cancel := context.WithTimeout(ctx, m.config.RecordAcquiredTimeout())
	defer cancel()
	return call(callCtx)
}

// releaseTaskToken releases ownerID's tokenID, retrying until it succeeds or taskReleaseTimeout
// passes. It returns false only if ctx ends first.
func (m *semaphoreManagerImpl) releaseTaskToken(ctx context.Context, task *persistence.SemaphoreTask, ownerID string, tokenID int) bool {
	retrier := m.newTaskRetrier(taskRetryMaxInterval)
	start := m.timeSource.Now()
	for {
		callCtx, cancel := context.WithTimeout(ctx, taskCallTimeout)
		err := m.Release(callCtx, ownerID)
		cancel()
		if err == nil {
			return true
		}
		if m.timeSource.Since(start) >= taskReleaseTimeout {
			// Stop retrying after taskReleaseTimeout and finish the task. Reconciliation frees the
			// token later.
			m.logger.Error("Semaphore grant loop gave up releasing a refused grant",
				append(taskLogTags(task), tag.Error(err), tag.SemaphoreTokenID(tokenID), tag.OperationCritical)...)
			return true
		}
		if !m.waitToRetry(ctx, retrier, task, "Semaphore grant loop failed to release a refused grant", err) {
			return false
		}
	}
}

// waitForGrantSignal waits until the grant loop is signaled or deadline passes. A nil deadline
// never passes. It returns false only if ctx ends.
func (m *semaphoreManagerImpl) waitForGrantSignal(ctx context.Context, deadline *time.Time) bool {
	var deadlineC <-chan time.Time
	if deadline != nil {
		timer := m.timeSource.NewTimer(deadline.Sub(m.timeSource.Now()))
		defer timer.Stop()
		deadlineC = timer.Chan()
	}
	select {
	case <-m.grantNotifyC:
		return true
	case <-deadlineC:
		return true
	case <-ctx.Done():
		return false
	}
}

// newTaskRetrier returns a retrier whose backoff grows up to maxInterval.
func (m *semaphoreManagerImpl) newTaskRetrier(maxInterval time.Duration) *taskRetrier {
	policy := backoff.NewExponentialRetryPolicy(taskRetryInitialInterval)
	policy.SetMaximumInterval(maxInterval)
	// No expiry: retry until success or stop.
	policy.SetExpirationInterval(backoff.NoInterval)
	return &taskRetrier{retrier: backoff.NewRetrier(policy, m.timeSource)}
}

// waitToRetry logs err and waits before the next try. It returns false only if ctx ends.
func (m *semaphoreManagerImpl) waitToRetry(ctx context.Context, r *taskRetrier, task *persistence.SemaphoreTask, msg string, err error) bool {
	r.attempts++
	tags := append(taskLogTags(task), tag.Error(err), tag.AttemptCount(r.attempts))
	if r.attempts > taskCriticalRetryCount {
		// Too many failures in a row: mark the log critical so it stands out.
		tags = append(tags, tag.OperationCritical)
	}
	m.logger.Error(msg, tags...)

	timer := m.timeSource.NewTimer(r.retrier.NextBackOff())
	defer timer.Stop()
	select {
	case <-timer.Chan():
		return true
	case <-ctx.Done():
		return false
	}
}

// taskLogTags identifies task in logs.
func taskLogTags(task *persistence.SemaphoreTask) []tag.Tag {
	return []tag.Tag{
		tag.TaskID(task.TaskID),
		tag.WorkflowID(task.WorkflowID),
		tag.WorkflowRunID(task.RunID),
		tag.WorkflowInitiatedID(task.HoldID),
	}
}
