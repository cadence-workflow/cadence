package replication

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type stubAckLevels struct {
	ackLevel     int64
	maxReadLevel int64
}

func (s *stubAckLevels) UpdateIfNeededAndGetQueueMaxReadLevel(_ persistence.HistoryTaskCategory, _ string) persistence.HistoryTaskKey {
	return persistence.NewImmediateTaskKey(s.maxReadLevel)
}
func (s *stubAckLevels) GetQueueClusterAckLevel(_ persistence.HistoryTaskCategory, _ string) persistence.HistoryTaskKey {
	return persistence.NewImmediateTaskKey(s.ackLevel)
}
func (s *stubAckLevels) UpdateQueueClusterAckLevel(_ persistence.HistoryTaskCategory, _ string, _ persistence.HistoryTaskKey) error {
	return nil
}

type stubBatchSizer struct{ v int }

func (s stubBatchSizer) analyse(_ error, _ *getTasksResult) {}
func (s stubBatchSizer) value() int                         { return s.v }

type errReader struct {
	ts clock.MockedTimeSource
}

func (r errReader) Read(_ context.Context, _, _ int64, _ int) ([]persistence.Task, bool, error) {
	// Advance mocked time so deferred histogram records a non-zero duration.
	r.ts.Advance(10 * time.Millisecond)
	return nil, false, errors.New("read failed")
}

func findHistogramTotal(t *testing.T, snap tally.Snapshot, name string) int64 {
	t.Helper()
	for _, h := range snap.Histograms() {
		if h.Name() != name {
			continue
		}
		var total int64
		for _, v := range h.Durations() {
			total += v
		}
		for _, v := range h.Values() {
			total += v
		}
		return total
	}
	return 0
}

type noopReservation struct{}

func (noopReservation) Allow() bool { return true }
func (noopReservation) Used(_ bool) {}

type allowLimiter struct{}

func (allowLimiter) Allow() bool                  { return true }
func (allowLimiter) Wait(_ context.Context) error { return nil }
func (allowLimiter) Reserve() clock.Reservation   { return noopReservation{} }
func (allowLimiter) Limit() rate.Limit            { return rate.Inf }

type stubDomainCache struct {
	entry *cache.DomainCacheEntry
}

func (s stubDomainCache) GetDomainByID(_ string) (*cache.DomainCacheEntry, error) {
	return s.entry, nil
}

type stubAckCache struct{}

func (stubAckCache) Put(_ *types.ReplicationTask, _ uint64) error { return nil }
func (stubAckCache) Get(_ int64) *types.ReplicationTask           { return nil }
func (stubAckCache) Ack(_ int64) (uint64, int)                    { return 0, 0 }
func (stubAckCache) Size() uint64                                 { return 0 }
func (stubAckCache) Count() int                                   { return 0 }

type errHydrator struct{}

func (errHydrator) Hydrate(_ context.Context, _ persistence.Task) (*types.ReplicationTask, error) {
	time.Sleep(2 * time.Millisecond)
	return nil, errors.New("hydrate failed")
}

type fakeTask struct {
	domainID string
	taskID   int64
}

func (t *fakeTask) GetTaskCategory() persistence.HistoryTaskCategory {
	return persistence.HistoryTaskCategoryReplication
}
func (t *fakeTask) GetTaskKey() persistence.HistoryTaskKey {
	return persistence.NewImmediateTaskKey(t.taskID)
}
func (t *fakeTask) GetTaskType() int                   { return persistence.ReplicationTaskTypeHistory }
func (t *fakeTask) GetDomainID() string                { return t.domainID }
func (t *fakeTask) GetWorkflowID() string              { return "" }
func (t *fakeTask) GetRunID() string                   { return "" }
func (t *fakeTask) GetVersion() int64                  { return 0 }
func (t *fakeTask) SetVersion(_ int64)                 {}
func (t *fakeTask) GetTaskID() int64                   { return t.taskID }
func (t *fakeTask) SetTaskID(id int64)                 { t.taskID = id }
func (t *fakeTask) GetVisibilityTimestamp() time.Time  { return time.Unix(0, 0) }
func (t *fakeTask) SetVisibilityTimestamp(_ time.Time) {}
func (t *fakeTask) ByteSize() uint64                   { return 1 }
func (t *fakeTask) ToTransferTaskInfo() (*persistence.TransferTaskInfo, error) {
	return nil, errors.New("not transfer")
}
func (t *fakeTask) ToTimerTaskInfo() (*persistence.TimerTaskInfo, error) {
	return nil, errors.New("not timer")
}
func (t *fakeTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, errors.New("not internal replication task info")
}

func TestTaskAckManager_EmitsExponentialTaskLatencyHistogram(t *testing.T) {
	ts := tally.NewTestScope("", nil)
	mc := metrics.NewClient(ts, metrics.History, metrics.HistogramMigration{})

	mockedTS := clock.NewMockedTimeSourceAt(time.Unix(0, 0))
	am := TaskAckManager{
		ackLevels:             &stubAckLevels{ackLevel: 1, maxReadLevel: 100},
		scope:                 mc.Scope(metrics.ReplicatorQueueProcessorScope, metrics.InstanceTag("1")),
		logger:                log.NewNoop(),
		reader:                errReader{ts: mockedTS},
		store:                 nil, // not used in this test (we error before hydration)
		timeSource:            mockedTS,
		dynamicTaskBatchSizer: stubBatchSizer{v: 1},
	}

	_, err := am.getTasks(context.Background(), "cluster-a", constants.EmptyMessageID)
	require.Error(t, err)

	snap := ts.Snapshot()
	total := findHistogramTotal(t, snap, "task_latency_ns")
	require.Greater(t, total, int64(0), "expected task_latency_ns histogram to be emitted")
}

func TestTaskStore_EmitsExponentialCacheLatencyHistogram(t *testing.T) {
	ts := tally.NewTestScope("", nil)
	mc := metrics.NewClient(ts, metrics.History, metrics.HistogramMigration{})

	cluster := "cluster-a"
	entry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: "domain-id", Name: "domain"},
		&persistence.DomainConfig{},
		cluster,
	)

	retryPolicy := backoff.NewExponentialRetryPolicy(time.Millisecond)
	retryPolicy.SetMaximumAttempts(1)

	store := &TaskStore{
		clusters: map[string]cache.AckCache[*types.ReplicationTask]{
			cluster: stubAckCache{},
		},
		domains:       stubDomainCache{entry: entry},
		hydrator:      errHydrator{},
		rateLimiter:   allowLimiter{},
		throttleRetry: backoff.NewThrottleRetry(backoff.WithRetryPolicy(retryPolicy)),
		scope:         mc.Scope(metrics.ReplicatorCacheManagerScope),
		logger:        log.NewNoop(),
	}

	_, err := store.Get(context.Background(), cluster, &fakeTask{domainID: "domain-id", taskID: 123})
	require.Error(t, err)

	snap := ts.Snapshot()
	total := findHistogramTotal(t, snap, "cache_latency_ns")
	require.Greater(t, total, int64(0), "expected cache_latency_ns histogram to be emitted")
}
