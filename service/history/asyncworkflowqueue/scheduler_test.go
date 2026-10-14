package asyncworkflowqueue

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	ctask "github.com/uber/cadence/common/task"
)

type fakeTask struct {
	mu        sync.Mutex
	state     ctask.State
	executeFn func() error
	acked     chan struct{}
	nacked    chan error
}

func newFakeTask(executeFn func() error) *fakeTask {
	return &fakeTask{
		state:     ctask.TaskStatePending,
		executeFn: executeFn,
		acked:     make(chan struct{}, 1),
		nacked:    make(chan error, 1),
	}
}

func (f *fakeTask) Execute() error            { return f.executeFn() }
func (f *fakeTask) HandleErr(err error) error { return err }
func (f *fakeTask) RetryErr(err error) bool   { return false }
func (f *fakeTask) Ack()                      { f.set(ctask.TaskStateAcked); f.acked <- struct{}{} }
func (f *fakeTask) Nack(err error)            { f.set(ctask.TaskStateCanceled); f.nacked <- err }
func (f *fakeTask) Cancel()                   { f.set(ctask.TaskStateCanceled) }
func (f *fakeTask) State() ctask.State {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}
func (f *fakeTask) set(s ctask.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = s
}

func newTestScheduler(t *testing.T, workers, queueSize int) Scheduler {
	return NewScheduler(
		testlogger.New(t),
		metrics.NewNoopMetricsClient(),
		dynamicproperties.GetIntPropertyFn(workers),
		dynamicproperties.GetIntPropertyFn(queueSize),
	)
}

func TestScheduler_SubmitExecutesAndAcks(t *testing.T) {
	defer goleak.VerifyNone(t)

	s := newTestScheduler(t, 2, 10)
	s.Start()
	defer s.Stop()

	executed := make(chan struct{})
	tk := newFakeTask(func() error { close(executed); return nil })
	require.NoError(t, s.Submit(tk))

	select {
	case <-tk.acked:
	case <-time.After(5 * time.Second):
		t.Fatal("task was not acked")
	}
	select {
	case <-executed:
	default:
		t.Fatal("Ack observed before Execute")
	}
	assert.Equal(t, ctask.TaskStateAcked, tk.State())
}

func TestScheduler_ExecuteErrorNacksWithError(t *testing.T) {
	defer goleak.VerifyNone(t)

	s := newTestScheduler(t, 1, 10)
	s.Start()
	defer s.Stop()

	boom := errors.New("boom")
	tk := newFakeTask(func() error { return boom })
	require.NoError(t, s.Submit(tk))

	select {
	case err := <-tk.nacked:
		assert.Equal(t, boom, err)
	case <-time.After(5 * time.Second):
		t.Fatal("task was not nacked")
	}
}

func TestScheduler_SubmitAfterStopReturnsError(t *testing.T) {
	defer goleak.VerifyNone(t)

	s := newTestScheduler(t, 1, 10)
	s.Start()
	s.Stop()

	err := s.Submit(newFakeTask(func() error { return nil }))
	assert.ErrorIs(t, err, ctask.ErrTaskProcessorClosed)
}

func TestScheduler_StopDrainsQueuedTasksWithNackNil(t *testing.T) {
	defer goleak.VerifyNone(t)

	s := newTestScheduler(t, 1, 10)
	s.Start()

	started := make(chan struct{})
	release := make(chan struct{})
	blocker := newFakeTask(func() error { close(started); <-release; return nil })
	require.NoError(t, s.Submit(blocker))
	<-started // the only worker is now busy

	queued := newFakeTask(func() error { t.Error("queued task must not execute"); return nil })
	require.NoError(t, s.Submit(queued))

	stopped := make(chan struct{})
	go func() { s.Stop(); close(stopped) }()

	select {
	case err := <-queued.nacked:
		assert.NoError(t, err, "drained tasks are Nack(nil) so they stay unacked and are redelivered")
	case <-time.After(5 * time.Second):
		t.Fatal("queued task was not drained on Stop")
	}

	close(release)
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("Stop did not return")
	}
	assert.Equal(t, ctask.TaskStateCanceled, queued.State())
}
