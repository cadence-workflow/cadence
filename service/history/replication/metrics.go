package replication

import (
	"math"
	"time"

	"github.com/uber-go/tally"
)

type histally struct {
	base tally.Scope
}

// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/sdk.md#base2-exponential-bucket-histogram-aggregation
const defaultScale = 3

// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#exponential-scale
// targets 160 buckets from 1ms to 100s (plus a zero) with <5% error in each bucket.
// this is implied to be "a decent default" and I generally agree.
//
// this is the "official" definition, but it allows floating point inequality to compound,
// which seems worse.  so I'm going to fake it.
var official1ms100sBuckets = append(
	tally.DurationBuckets{0},
	tally.MustMakeExponentialDurationBuckets(
		time.Millisecond,
		math.Pow(2, math.Pow(2, -defaultScale)),
		160,
	)...,
)

func appendall(slices ...tally.DurationBuckets) tally.DurationBuckets {
	total := 0
	for _, slice := range slices {
		total += len(slice)
	}
	result := make(tally.DurationBuckets, 0, total)
	for _, slice := range slices {
		result = append(result, slice...)
	}
	return result
}

// equivalent to official1ms100msBuckets but reduces floating point error for
// more accurate (and more readable) numbers.
var default1ms100sBuckets = appendall(
	tally.DurationBuckets{0},
	// note this is easy to calculate: scale factor 3 == 1/2**3 growth, and 2**3 per precision reset,
	// and just keep going up to 160 or whatever.
	tally.MustMakeExponentialDurationBuckets(time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(2*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(4*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(8*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(16*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(32*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(64*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(128*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(256*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(512*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(1024*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(1024*2*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(1024*4*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(1024*8*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(1024*16*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(1024*32*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(1024*64*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(1024*128*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(1024*256*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
	tally.MustMakeExponentialDurationBuckets(1024*512*time.Millisecond, math.Pow(2, 1.0/8.0), 8),
)

func (h histally) Count(name string, inc int) {
	h.base.Counter(name).Inc(int64(inc))
}
func (h histally) Gauge(name string, val float64) {
	h.base.Gauge(name).Update(val)
}
func (h histally) Durationgram(name string, dur time.Duration) {
	h.base.Histogram(name, default1ms100sBuckets).RecordDuration(dur)
}
func (h histally) Floatogram(name string, val float64) {

}

// Metrics covers all "core" metrics, i.e. stuff we expect to be semantically
// consistent and we have probably built alerts and dashboards on it.
//
// ad-hoc or experimental metrics can use a looser API for simplicity, or
// document temporary stuff it in the interface if preferred.
//
// TODO: an interface is nice because we can replace it... but do we care?  ignore it for now.

// TODO: switch to migrating metrics_emitter.go, it's the actual latency one and it's much simpler.
// TODO: still likely need some way to incrementally build up a context

type Metrics struct {
	base tally.Scope
}

// set up the initial metrics with any constant tags.
// all other tags are added in methods.
//
// the passed scope should be the service-level base tally scope, which
// currently comes from fx and only contains cadence_service:cadence-frontend.
// all others are added internally, to keep things explicit.
func newMetrics(scope tally.Scope) *Metrics {
	return &Metrics{
		base: scope,
	}
}

func (m *Metrics) TaskCleanupFailure() {
	// p.metricsClient.Scope(metrics.ReplicationTaskCleanupScope).IncCounter(metrics.ReplicationTaskCleanupFailure)
}

func (m *Metrics) TaskCleanupSuccess() {
	// p.metricsClient.Scope(metrics.ReplicationTaskCleanupScope).IncCounter(metrics.ReplicationTaskCleanupCount)
}

func (m *Metrics) TaskLag(targetCluster string, lag time.Duration) {
	// p.metricsClient.Scope(metrics.ReplicationTaskFetcherScope,
	// 	metrics.TargetClusterTag(p.currentCluster),
	// ).RecordTimer(
	// 	metrics.ReplicationTasksLag,
	// 	time.Duration(p.shard.UpdateIfNeededAndGetQueueMaxReadLevel(persistence.HistoryTaskCategoryReplication, p.currentCluster).GetTaskID()-minAckLevel),
	// )
}

func (m *Metrics) ShardSynced() {
	// p.metricsClient.Scope(metrics.HistorySyncShardStatusScope).IncCounter(metrics.SyncShardFromRemoteCounter)
}

func (m *Metrics) TaskApplyLatency(latency time.Duration) {
	// scope := p.metricsClient.Scope(metrics.ReplicationTaskFetcherScope, metrics.TargetClusterTag(p.sourceCluster))
	// scope.RecordTimer(metrics.ReplicationTasksAppliedLatency, time.Since(batchRequestStartTime))
}

func (m *Metrics) LastRetrievedMessageID(lastRetrievedMessageID int64) {
	// scope := p.metricsClient.Scope(metrics.ReplicationTaskFetcherScope, metrics.TargetClusterTag(p.sourceCluster))
	// scope.UpdateGauge(metrics.LastRetrievedMessageID, float64(p.lastRetrievedMessageID))
}

// next three share source/domain because they share a scope.
// might be a fitting shared object.

func (m *Metrics) TaskProcessingLatency(sourceCluster string, domainName string, latency time.Duration) {
	// mScope := p.metricsClient.Scope(scope, metrics.TargetClusterTag(p.sourceCluster))
	// mScope = mScope.Tagged(metrics.DomainTag(domainName))
	// mScope.RecordTimer(metrics.TaskProcessingLatency, now.Sub(startTime))
}
func (m *Metrics) TaskReplicationLatency(sourceCluster string, domainName string, latency time.Duration) {
	// mScope := p.metricsClient.Scope(scope, metrics.TargetClusterTag(p.sourceCluster))
	// mScope = mScope.Tagged(metrics.DomainTag(domainName))
	// mScope.RecordTimer(
	// 	metrics.ReplicationTaskLatency,
	// 	now.Sub(time.Unix(0, replicationTask.GetCreationTime())),
	// )
}
func (m *Metrics) TasksAppliedPerDomain(sourceCluster string, domainName string) {
	// mScope := p.metricsClient.Scope(scope, metrics.TargetClusterTag(p.sourceCluster))
	// mScope = mScope.Tagged(metrics.DomainTag(domainName))
	// mScope.IncCounter(metrics.ReplicationTasksAppliedPerDomain)
}

func (m *Metrics) TasksAppliedPerShard(sourceCluster string, shard int) {
	// shardScope := p.metricsClient.Scope(scope, metrics.TargetClusterTag(p.sourceCluster), metrics.InstanceTag(strconv.Itoa(p.shard.GetShardID())))
	// shardScope.IncCounter(metrics.ReplicationTasksApplied)
}

func (m *Metrics) ReplicationDLQLevel(sourceCluster string, shard int, taskid int) {
	// p.metricsClient.Scope(
	// 	metrics.ReplicationDLQStatsScope,
	// 	metrics.TargetClusterTag(p.sourceCluster),
	// 	metrics.InstanceTag(strconv.Itoa(p.shard.GetShardID())),
	// ).UpdateGauge(
	// 	metrics.ReplicationDLQMaxLevelGauge,
	// 	float64(request.TaskInfo.GetTaskID()),
	// )
}

func (m *Metrics) ReplicationDLQPutFailed() {
	// p.metricsClient.IncCounter(metrics.ReplicationTaskFetcherScope, metrics.ReplicationDLQFailed)
}

func (m *Metrics) SyncShardFromRemoteFailure() {
	// p.metricsClient.Scope(metrics.HistorySyncShardStatusScope).IncCounter(metrics.SyncShardFromRemoteFailure)
}

func (m *Metrics) RecordFailure(scope int, shard int, err error) {
	// TODO: scope is complicated, comes from task-executor.execute :(
	// probably needs to be carried through similarly, maybe a unique type for ease

	// func (e *taskExecutorImpl) execute(
	// 	replicationTask *types.ReplicationTask,
	// forceApply bool,
	// ) (int, error) {
	//
	// 	var err error
	// 	var scope int
	// 	switch replicationTask.GetTaskType() {
	// 	case types.ReplicationTaskTypeSyncActivity:
	// 		scope = metrics.SyncActivityTaskScope
	// 		err = e.handleActivityTask(replicationTask, forceApply)
	// 	case types.ReplicationTaskTypeHistoryV2:
	// 		scope = metrics.HistoryReplicationV2TaskScope
	// 		err = e.handleHistoryReplicationTaskV2(replicationTask, forceApply)
	// 	case types.ReplicationTaskTypeFailoverMarker:
	// 		scope = metrics.FailoverMarkerScope
	// 		err = e.handleFailoverReplicationTask(replicationTask)
	// 	default:
	// 		e.logger.Error("Unknown task type.")
	// 		scope = metrics.ReplicatorScope
	// 		err = ErrUnknownReplicationTask
	// 	}
	//
	// 	return scope, err
	// }

	// but then it's used as:

	// shardScope := p.metricsClient.Scope(scope, metrics.InstanceTag(strconv.Itoa(shardID)))
	// shardScope.IncCounter(metrics.ReplicatorFailures)
	//
	// // Also update counter to distinguish between type of failures
	// switch err := err.(type) {
	// case *types.ShardOwnershipLostError:
	// 	shardScope.IncCounter(metrics.CadenceErrShardOwnershipLostCounter)
	// case *types.BadRequestError:
	// 	shardScope.IncCounter(metrics.CadenceErrBadRequestCounter)
	// case *types.DomainNotActiveError:
	// 	shardScope.IncCounter(metrics.CadenceErrDomainNotActiveCounter)
	// case *types.WorkflowExecutionAlreadyStartedError:
	// 	shardScope.IncCounter(metrics.CadenceErrExecutionAlreadyStartedCounter)
	// case *types.EntityNotExistsError:
	// 	shardScope.IncCounter(metrics.CadenceErrEntityNotExistsCounter)
	// case *types.WorkflowExecutionAlreadyCompletedError:
	// 	shardScope.IncCounter(metrics.CadenceErrWorkflowExecutionAlreadyCompletedCounter)
	// case *types.LimitExceededError:
	// 	shardScope.IncCounter(metrics.CadenceErrLimitExceededCounter)
	// case *yarpcerrors.Status:
	// 	if err.Code() == yarpcerrors.CodeDeadlineExceeded {
	// 		shardScope.IncCounter(metrics.CadenceErrContextTimeoutCounter)
	// 	}
	// }
}
